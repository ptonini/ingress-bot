package handler

import (
	"context"
	"fmt"
	"github.com/ptonini/ingress-bot/config"
	"github.com/ptonini/ingress-bot/kube"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	core "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"maps"
	"reflect"
	"strings"
	"time"
)

type Handler struct {
	ctx              context.Context
	logger           *zap.Logger
	listOpt          meta.ListOptions
	services         map[string]core.Service
	currentIngresses map[string]*networking.Ingress
	desiredIngresses map[string]*networking.Ingress
	dryRun           []string
}

func (h *Handler) fetchServices() (map[string]core.Service, error) {
	l, err := kube.ClientSet.CoreV1().Services("").List(h.ctx, h.listOpt)
	if err != nil {
		return nil, fmt.Errorf("error fetching services: %v", err)
	}
	list := map[string]core.Service{}
	for _, v := range l.Items {
		list[v.Name] = v
	}
	return list, nil
}

func (h *Handler) fetchIngresses() (map[string]*networking.Ingress, error) {
	l, err := kube.ClientSet.NetworkingV1().Ingresses("").List(h.ctx, h.listOpt)
	if err != nil {
		return nil, fmt.Errorf("error fetching ingresses: %v", err)
	}
	list := map[string]*networking.Ingress{}
	for _, v := range l.Items {
		list[v.Name] = &v
	}
	return list, nil
}

func (h *Handler) getServiceAnnotations(s *core.Service) ([]string, string, string) {
	hosts := strings.Split(s.Annotations[viper.GetString(config.IngressHostAnnotation)], ",")
	class := s.Annotations[viper.GetString(config.IngressClassAnnotation)]
	name := strings.Replace(hosts[0], ".", "-", -1)
	return hosts, class, name
}

func (h *Handler) buildIngress(name string, namespace string, hosts []string, class string) *networking.Ingress {

	var rules []networking.IngressRule
	var tls []networking.IngressTLS
	var ingressClassName *string

	// Set ingress rules
	for _, v := range hosts {
		rules = append(rules, networking.IngressRule{
			Host: v,
			IngressRuleValue: networking.IngressRuleValue{
				HTTP: &networking.HTTPIngressRuleValue{
					Paths: []networking.HTTPIngressPath{},
				},
			},
		})
	}

	// Set annotations
	annotations := map[string]string{}
	if viper.IsSet(config.IngressAnnotations) {
		maps.Copy(annotations, viper.GetStringMapString(config.IngressAnnotations))
	}

	// Set labels
	labels := map[string]string{
		viper.GetString(config.ResourceLabelKey): viper.GetString(config.ResourceLabelValue),
	}
	if viper.IsSet(config.IngressLabels) {
		maps.Copy(labels, viper.GetStringMapString(config.IngressLabels))
	}

	// Set TLS
	if viper.GetBool(config.IngressEnableTLS) {
		tls = []networking.IngressTLS{
			{
				Hosts:      hosts,
				SecretName: fmt.Sprintf("%s-tls", name),
			},
		}
	}

	if class == "" {
		ingressClassName = nil
	} else {
		ingressClassName = &class
	}

	// Return ingress object
	return &networking.Ingress{
		ObjectMeta: meta.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
			Labels:      labels,
		},
		Spec: networking.IngressSpec{
			IngressClassName: ingressClassName,
			TLS:              tls,
			Rules:            rules,
		},
	}

}

func (h *Handler) attachServiceToIngress(i *networking.Ingress, s core.Service) {
	for j, p := range i.Spec.Rules[0].HTTP.Paths {
		if p.Backend.Service.Name == s.Name {
			i.Spec.Rules[0].HTTP.Paths = append(i.Spec.Rules[0].HTTP.Paths[:j], i.Spec.Rules[0].HTTP.Paths[j+1:]...)
			break
		}
	}
	pathType := networking.PathType(viper.GetString(config.IngressPathType))
	i.Spec.Rules[0].HTTP.Paths = append(i.Spec.Rules[0].HTTP.Paths, networking.HTTPIngressPath{
		Path:     s.Annotations[viper.GetString(config.IngressPathAnnotation)],
		PathType: &pathType,
		Backend: networking.IngressBackend{
			Service: &networking.IngressServiceBackend{
				Name: s.Name,
				Port: networking.ServiceBackendPort{
					Number: s.Spec.Ports[0].Port,
				},
			},
		},
	})
}

func (h *Handler) buildDesiredIngresses() (ingresses map[string]*networking.Ingress, err error) {

	ingresses = map[string]*networking.Ingress{}
	for _, s := range h.services {
		hosts, class, name := h.getServiceAnnotations(&s)
		if _, ok := ingresses[name]; ok {
			if ingresses[name].Namespace != s.Namespace {
				return nil, fmt.Errorf("service %s/%s declaring host for ingress %s/%s",
					s.Namespace, s.Name, ingresses[name].Namespace, ingresses[name].Name)
			}
			if ingresses[name].Spec.IngressClassName != nil && *ingresses[name].Spec.IngressClassName != class {
				return nil, fmt.Errorf("service %s/%s declaring class %s for ingress %s/%s",
					s.Namespace, s.Name, class, ingresses[name].Namespace, ingresses[name].Name)
			}
		} else {
			h.logger.Debug(fmt.Sprintf("adding ingress %s/%s to desired list", s.Namespace, name))
			ingresses[name] = h.buildIngress(name, s.Namespace, hosts, class)
		}
		h.logger.Debug(fmt.Sprintf("adding service %s to ingress %s/%s", s.Name, s.Namespace, name))
		h.attachServiceToIngress(ingresses[name], s)
	}
	return
}

func (h *Handler) compareIngresses(d *networking.Ingress, c *networking.Ingress) (specsAreEqual bool) {

	if d.Namespace != c.Namespace {
		h.logger.Debug(fmt.Sprintf("new namespace on ingress %s", c.Name))
		return
	}

	for k, v := range d.Annotations {
		if v != c.Annotations[k] {
			h.logger.Debug(fmt.Sprintf("updated annotations on ingress %s", c.Name))
			return
		}
	}

	for k, v := range d.Labels {
		if v != c.Labels[k] {
			h.logger.Debug(fmt.Sprintf("updated labels on ingress %s", c.Name))
			return
		}
	}

	specsAreEqual = reflect.DeepEqual(d.Spec, c.Spec)
	if !specsAreEqual {
		h.logger.Debug(fmt.Sprintf("updated spec on ingress %s", c.Name))
	}
	return
}

func (h *Handler) createIngress(i *networking.Ingress) (*networking.Ingress, error) {
	h.logger.Info(fmt.Sprintf("creating ingress %s", i.Name))
	i, err := kube.ClientSet.NetworkingV1().Ingresses(i.Namespace).Create(h.ctx, i, meta.CreateOptions{
		DryRun: h.dryRun,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating ingress: %v", err)
	}
	return i, nil
}

func (h *Handler) updateIngress(i *networking.Ingress) (*networking.Ingress, error) {
	h.logger.Info(fmt.Sprintf("updating ingress %s", i.Name))
	i, err := kube.ClientSet.NetworkingV1().Ingresses(i.Namespace).Update(h.ctx, i, meta.UpdateOptions{
		DryRun: h.dryRun,
	})
	if err != nil {
		return nil, fmt.Errorf("error updating ingress: %v", err)
	}
	return i, nil
}

func (h *Handler) deleteIngress(i *networking.Ingress) error {
	h.logger.Info(fmt.Sprintf("deleting ingress %s", i.Name))
	err := kube.ClientSet.NetworkingV1().Ingresses(i.Namespace).Delete(h.ctx, i.Name, meta.DeleteOptions{
		DryRun: h.dryRun,
	})
	if err != nil {
		return fmt.Errorf("error deleting ingress %s/%s: %v", i.Namespace, i.Name, err)
	}
	return nil
}

func (h *Handler) reconcile() error {

	var err error
	var i *networking.Ingress

	h.services, err = h.fetchServices()
	if err != nil {
		return err
	}
	h.currentIngresses, err = h.fetchIngresses()
	if err != nil {
		return err
	}
	h.desiredIngresses, err = h.buildDesiredIngresses()
	if err != nil {
		return err
	}

	// Remove undesired ingresses
	for _, ingress := range h.currentIngresses {
		// Remove serviceless ingresses
		if _, ok := h.desiredIngresses[ingress.Name]; !ok {
			err = h.deleteIngress(ingress)
			if err != nil {
				return err
			}
		}
	}

	// Upsert desired ingresses
	for _, ingress := range h.desiredIngresses {
		if _, ok := h.currentIngresses[ingress.Name]; ok {
			// Update existing ingress
			if !h.compareIngresses(h.desiredIngresses[ingress.Name], h.currentIngresses[ingress.Name]) {
				i, err = h.updateIngress(ingress)
				h.currentIngresses[ingress.Name] = i
				if err != nil {
					return err
				}
			}
		} else {
			// Create new ingress
			i, err = h.createIngress(ingress)
			h.currentIngresses[ingress.Name] = i
			if err != nil {
				return err
			}
		}
	}
	return err
}

func (h *Handler) ReconciliationLoop() {
	for {
		err := h.reconcile()
		if err != nil {
			h.logger.Error(err.Error())
			break
		}
		time.Sleep(viper.GetDuration(config.CheckInterval) * time.Second)
		if viper.GetDuration(config.CheckInterval) == 0 {
			break
		}
	}
}

func Factory(ctx context.Context, logger *zap.Logger, timeout int64) *Handler {
	h := &Handler{
		ctx:              ctx,
		logger:           logger,
		services:         map[string]core.Service{},
		currentIngresses: map[string]*networking.Ingress{},
		desiredIngresses: map[string]*networking.Ingress{},
		listOpt: meta.ListOptions{
			TimeoutSeconds: &timeout,
			LabelSelector:  viper.GetString(config.ResourceLabelKey),
		},
	}
	if viper.GetBool(config.DryRun) {
		h.dryRun = []string{"All"}
	}
	return h
}
