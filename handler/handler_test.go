package handler

import (
	"context"
	"errors"
	"github.com/ptonini/ingress-bot/config"
	"github.com/ptonini/ingress-bot/kube"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	core "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	coreFake "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	networkingFake "k8s.io/client-go/kubernetes/typed/networking/v1/fake"
	k8sTesting "k8s.io/client-go/testing"
	"testing"
	"time"
)

var (
	service = &core.Service{
		ObjectMeta: meta.ObjectMeta{
			Name:        "service",
			Namespace:   "default",
			Annotations: map[string]string{},
			Labels:      map[string]string{},
		},
		Spec: core.ServiceSpec{
			Ports: []core.ServicePort{
				{Port: 8080},
			},
		},
	}
	ingress = &networking.Ingress{
		ObjectMeta: meta.ObjectMeta{
			Name:        "ingress",
			Namespace:   "default",
			Annotations: map[string]string{},
			Labels:      map[string]string{},
		},
		Spec: networking.IngressSpec{
			IngressClassName: nil,
			DefaultBackend:   nil,
			TLS:              nil,
			Rules: []networking.IngressRule{
				{
					Host: "www.example.com",
					IngressRuleValue: networking.IngressRuleValue{
						HTTP: &networking.HTTPIngressRuleValue{Paths: []networking.HTTPIngressPath{}},
					},
				},
			},
		},
	}
	httpIngressPath = &networking.HTTPIngressPath{
		Backend: networking.IngressBackend{
			Service: &networking.IngressServiceBackend{
				Name: service.Name,
				Port: networking.ServiceBackendPort{Number: service.Spec.Ports[0].Port},
			},
		},
	}
)

func serviceListErrorReactor(action k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
	return true, &core.ServiceList{}, errors.New("fake error")
}

func ingressListErrorReactor(action k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
	return true, &networking.IngressList{}, errors.New("fake error")
}

func ingressErrorReactor(action k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
	return true, &networking.Ingress{}, errors.New("fake error")
}

func resetCoreReactionChain() {
	kube.ClientSet.CoreV1().(*coreFake.FakeCoreV1).ReactionChain = kube.ClientSet.CoreV1().(*coreFake.FakeCoreV1).ReactionChain[1:]
}

func resetNetworkingReactionChain() {
	kube.ClientSet.NetworkingV1().(*networkingFake.FakeNetworkingV1).ReactionChain = kube.ClientSet.NetworkingV1().(*networkingFake.FakeNetworkingV1).ReactionChain[1:]
}

func Test_Handler(t *testing.T) {

	config.Load()
	viper.Set(config.IngressLabels, `{"test": "true"}`)
	viper.Set(config.IngressAnnotations, `{"test": "true"}`)
	viper.Set(config.DryRun, "true")

	service.Labels[viper.GetString(config.ResourceLabelKey)] = viper.GetString(config.ResourceLabelValue)
	service.Annotations[viper.GetString(config.IngressHostAnnotation)] = "www.example.com"
	service.Annotations[viper.GetString(config.IngressClassAnnotation)] = "default"
	ingress.Labels[viper.GetString(config.ResourceLabelKey)] = viper.GetString(config.ResourceLabelValue)

	ctx := context.Background()
	ctx = context.WithValue(ctx, viper.GetString(config.ContextTestingKey), true)
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(observedZapCore)

	h := Factory(ctx, logger, viper.GetInt64(config.ClientTimeout))

	t.Run("fetch services", func(t *testing.T) {
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service})
		_ = kube.GetClientSet(ctx, logger)
		l, err := h.fetchServices()
		assert.NoError(t, err)
		assert.Len(t, l, 1)
	})
	t.Run("fetch services with error", func(t *testing.T) {
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service})
		_ = kube.GetClientSet(ctx, logger)
		kube.ClientSet.CoreV1().(*coreFake.FakeCoreV1).PrependReactor("list", "services", serviceListErrorReactor)
		defer resetCoreReactionChain()
		_, err := h.fetchServices()
		assert.Error(t, err)
	})

	t.Run("fetch ingresses", func(t *testing.T) {
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{ingress})
		_ = kube.GetClientSet(ctx, logger)
		l, err := h.fetchIngresses()
		assert.NoError(t, err)
		assert.Len(t, l, 1)
	})
	t.Run("fetch ingresses with error", func(t *testing.T) {
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{ingress})
		_ = kube.GetClientSet(ctx, logger)
		kube.ClientSet.NetworkingV1().(*networkingFake.FakeNetworkingV1).PrependReactor("list", "ingresses", ingressListErrorReactor)
		defer resetNetworkingReactionChain()
		_, err := h.fetchIngresses()
		assert.Error(t, err)
	})

	t.Run("get service annotations", func(t *testing.T) {
		host, class, name := h.getServiceAnnotations(service)
		assert.NotEmpty(t, host)
		assert.NotEmpty(t, class)
		assert.NotEmpty(t, name)
	})

	t.Run("build ingress", func(t *testing.T) {
		className := "default"
		i := h.buildIngress("test", "default", []string{"www.example.com", "example.com"}, className)
		assert.Len(t, i.Annotations, 1)
		assert.Len(t, i.Labels, 2)
		assert.Len(t, i.Spec.TLS, 1)
		assert.Len(t, i.Spec.Rules, 2)
		assert.Equal(t, &className, i.Spec.IngressClassName)
	})
	t.Run("build classless ingress", func(t *testing.T) {
		i := h.buildIngress("test", "default", []string{"www.example.com"}, "")
		assert.Nil(t, i.Spec.IngressClassName)
	})

	t.Run("attach service to ingress", func(t *testing.T) {
		s := service.DeepCopy()
		i := ingress.DeepCopy()
		h.attachServiceToIngress(i, *s)
		assert.Len(t, i.Spec.Rules[0].HTTP.Paths, 1)
	})
	t.Run("reattach service to ingress", func(t *testing.T) {
		s := service.DeepCopy()
		s.Annotations[viper.GetString(config.IngressPathAnnotation)] = "/new_path"
		s.Spec.Ports[0].Port = 9090
		i := ingress.DeepCopy()
		p := httpIngressPath.DeepCopy()
		p.Path = "/path"
		i.Spec.Rules[0].HTTP.Paths = append(i.Spec.Rules[0].HTTP.Paths, *p)
		h.attachServiceToIngress(i, *s)
		assert.Len(t, i.Spec.Rules[0].HTTP.Paths, 1)
		assert.Equal(t, s.Spec.Ports[0].Port, i.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)
		assert.Equal(t, s.Annotations[viper.GetString(config.IngressPathAnnotation)], i.Spec.Rules[0].HTTP.Paths[0].Path)
	})

	t.Run("compare ingresses", func(t *testing.T) {
		cur := ingress.DeepCopy()
		des := ingress.DeepCopy()
		assert.True(t, h.compareIngresses(des, cur))
	})
	t.Run("compare ingresses with new namespace", func(t *testing.T) {
		cur := ingress.DeepCopy()
		des := ingress.DeepCopy()
		des.Namespace = "new"
		assert.False(t, h.compareIngresses(des, cur))
	})
	t.Run("compare ingresses with new annotation", func(t *testing.T) {
		cur := ingress.DeepCopy()
		des := ingress.DeepCopy()
		des.Annotations["new-annotation"] = "true"
		assert.False(t, h.compareIngresses(des, cur))
	})
	t.Run("compare ingresses with new label", func(t *testing.T) {
		cur := ingress.DeepCopy()
		des := ingress.DeepCopy()
		des.Labels["new-label"] = "true"
		assert.False(t, h.compareIngresses(des, cur))
	})
	t.Run("compare ingresses with new spec", func(t *testing.T) {
		cur := ingress.DeepCopy()
		des := ingress.DeepCopy()
		p := httpIngressPath.DeepCopy()
		p.Path = "/path"
		des.Spec.Rules[0].HTTP.Paths = append(des.Spec.Rules[0].HTTP.Paths, *p)
		assert.False(t, h.compareIngresses(des, cur))
	})
	t.Run("compare ingresses with inverted specs", func(t *testing.T) {
		cur := ingress.DeepCopy()
		des := ingress.DeepCopy()
		p1 := httpIngressPath.DeepCopy()
		p1.Path = "/path01"
		p1.Backend.Service.Name = "service01"
		p2 := httpIngressPath.DeepCopy()
		p2.Path = "/path02"
		p2.Backend.Service.Name = "service02"
		cur.Spec.Rules[0].HTTP.Paths = append(cur.Spec.Rules[0].HTTP.Paths, *p1.DeepCopy(), *p2.DeepCopy())
		des.Spec.Rules[0].HTTP.Paths = append(des.Spec.Rules[0].HTTP.Paths, *p2.DeepCopy(), *p1.DeepCopy())
		//assert.True(t, h.compareIngresses(des, cur))
	})

	t.Run("build desired ingresses", func(t *testing.T) {
		s2 := service.DeepCopy()
		s2.Name = "service2"
		s2.Annotations[viper.GetString(config.IngressPathAnnotation)] = "/path2"
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service.DeepCopy(), s2})
		_ = kube.GetClientSet(ctx, logger)
		h.services, _ = h.fetchServices()
		l, err := h.buildDesiredIngresses()
		assert.NoError(t, err)
		assert.Len(t, l, 1)
		assert.Len(t, l["www-example-com"].Spec.Rules[0].HTTP.Paths, 2)
	})
	t.Run("build desired ingress with multiple hosts", func(t *testing.T) {
		s := service.DeepCopy()
		s.Annotations[viper.GetString(config.IngressHostAnnotation)] = "www.example.com,www2.example.com"
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{s})
		_ = kube.GetClientSet(ctx, logger)
		h.services, _ = h.fetchServices()
		l, err := h.buildDesiredIngresses()
		assert.NoError(t, err)
		assert.Len(t, l, 1)
		assert.Len(t, l["www-example-com"].Spec.Rules, 2)
		assert.Len(t, l["www-example-com"].Spec.Rules[0].HTTP.Paths, 1)
		assert.Len(t, l["www-example-com"].Spec.Rules[1].HTTP.Paths, 1)
	})

	t.Run("build desired ingresses with namespace mismatch error", func(t *testing.T) {
		s2 := service.DeepCopy()
		s2.Name = "service2"
		s2.Namespace = "alternative"
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service.DeepCopy(), s2})
		_ = kube.GetClientSet(ctx, logger)
		h.services, _ = h.fetchServices()
		_, err := h.buildDesiredIngresses()
		assert.Error(t, err)
	})
	t.Run("build desired ingresses with ingress class mismatch error", func(t *testing.T) {
		s2 := service.DeepCopy()
		s2.Name = "service2"
		s2.Annotations[viper.GetString(config.IngressClassAnnotation)] = "alternative"
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service.DeepCopy(), s2})
		_ = kube.GetClientSet(ctx, logger)
		h.services, _ = h.fetchServices()
		_, err := h.buildDesiredIngresses()
		assert.Error(t, err)
	})

	t.Run("create ingress", func(t *testing.T) {
		_ = kube.GetClientSet(ctx, logger)
		i := ingress.DeepCopy()
		i.Name = "new-ingress"
		_, err := h.createIngress(i)
		assert.NoError(t, err)
	})
	t.Run("create ingress with error", func(t *testing.T) {
		_ = kube.GetClientSet(ctx, logger)
		kube.ClientSet.NetworkingV1().(*networkingFake.FakeNetworkingV1).PrependReactor("create", "ingresses", ingressErrorReactor)
		defer resetNetworkingReactionChain()
		i := ingress.DeepCopy()
		_, err := h.createIngress(i)
		assert.Error(t, err)
	})

	t.Run("update ingress", func(t *testing.T) {
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{ingress})
		_ = kube.GetClientSet(ctx, logger)
		i := ingress.DeepCopy()
		_, err := h.updateIngress(i)
		assert.NoError(t, err)
	})
	t.Run("update ingress with error", func(t *testing.T) {
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{ingress})
		_ = kube.GetClientSet(ctx, logger)
		kube.ClientSet.NetworkingV1().(*networkingFake.FakeNetworkingV1).PrependReactor("update", "ingresses", ingressErrorReactor)
		defer resetNetworkingReactionChain()
		i := ingress.DeepCopy()
		_, err := h.updateIngress(i)
		assert.Error(t, err)
	})

	t.Run("delete ingress", func(t *testing.T) {
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{ingress})
		_ = kube.GetClientSet(ctx, logger)
		i := ingress.DeepCopy()
		err := h.deleteIngress(i)
		assert.NoError(t, err)
	})
	t.Run("delete ingress with error", func(t *testing.T) {
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{ingress})
		_ = kube.GetClientSet(ctx, logger)
		kube.ClientSet.NetworkingV1().(*networkingFake.FakeNetworkingV1).PrependReactor("delete", "ingresses", ingressErrorReactor)
		defer resetNetworkingReactionChain()
		i := ingress.DeepCopy()
		err := h.deleteIngress(i)
		assert.Error(t, err)
	})

	t.Run("reconcile creating ingress", func(t *testing.T) {
		s2 := service.DeepCopy()
		s2.Name = "service2"
		s2.Annotations[viper.GetString(config.IngressPathAnnotation)] = "/path2"
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service.DeepCopy(), s2, ingress.DeepCopy()})
		_ = kube.GetClientSet(ctx, logger)
		assert.NoError(t, h.reconcile())
	})
	t.Run("reconcile updating ingress", func(t *testing.T) {
		s2 := service.DeepCopy()
		s2.Name = "service2"
		s2.Annotations[viper.GetString(config.IngressPathAnnotation)] = "/path2"
		i2 := ingress.DeepCopy()
		i2.Name = "www-example-com"
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service.DeepCopy(), s2, ingress.DeepCopy(), i2})
		_ = kube.GetClientSet(ctx, logger)
		assert.NoError(t, h.reconcile())

	})
	t.Run("reconcile with error fetching services", func(t *testing.T) {
		_ = kube.GetClientSet(ctx, logger)
		kube.ClientSet.CoreV1().(*coreFake.FakeCoreV1).PrependReactor("list", "services", serviceListErrorReactor)
		defer resetCoreReactionChain()
		assert.Error(t, h.reconcile())
	})
	t.Run("reconcile with error fetching ingresses", func(t *testing.T) {
		_ = kube.GetClientSet(ctx, logger)
		kube.ClientSet.CoreV1().(*coreFake.FakeCoreV1).PrependReactor("list", "ingresses", ingressListErrorReactor)
		defer resetNetworkingReactionChain()
		assert.Error(t, h.reconcile())
	})
	t.Run("reconcile with error building desired ingresses", func(t *testing.T) {
		s2 := service.DeepCopy()
		s2.Name = "service2"
		s2.Namespace = "alternative"
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service.DeepCopy(), s2})
		_ = kube.GetClientSet(ctx, logger)
		assert.Error(t, h.reconcile())
	})
	t.Run("reconcile with error deleting ingresses", func(t *testing.T) {
		s2 := service.DeepCopy()
		s2.Name = "service2"
		s2.Annotations[viper.GetString(config.IngressPathAnnotation)] = "/path2"
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service.DeepCopy(), s2, ingress.DeepCopy()})
		_ = kube.GetClientSet(ctx, logger)
		kube.ClientSet.NetworkingV1().(*networkingFake.FakeNetworkingV1).PrependReactor("delete", "ingresses", ingressErrorReactor)
		defer resetNetworkingReactionChain()
		assert.Error(t, h.reconcile())
	})
	t.Run("reconcile with error updating ingress", func(t *testing.T) {
		s2 := service.DeepCopy()
		s2.Name = "service2"
		s2.Annotations[viper.GetString(config.IngressPathAnnotation)] = "/path2"
		i2 := ingress.DeepCopy()
		i2.Name = "www-example-com"
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service.DeepCopy(), s2, ingress.DeepCopy(), i2})
		_ = kube.GetClientSet(ctx, logger)
		kube.ClientSet.NetworkingV1().(*networkingFake.FakeNetworkingV1).PrependReactor("update", "ingresses", ingressErrorReactor)
		defer resetNetworkingReactionChain()
		assert.Error(t, h.reconcile())

	})
	t.Run("reconcile with error creating ingresses", func(t *testing.T) {
		s2 := service.DeepCopy()
		s2.Name = "service2"
		s2.Annotations[viper.GetString(config.IngressPathAnnotation)] = "/path2"
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service.DeepCopy(), s2, ingress.DeepCopy()})
		_ = kube.GetClientSet(ctx, logger)
		kube.ClientSet.NetworkingV1().(*networkingFake.FakeNetworkingV1).PrependReactor("create", "ingresses", ingressErrorReactor)
		defer resetNetworkingReactionChain()
		assert.Error(t, h.reconcile())
	})

	t.Run("reconciliation loop", func(t *testing.T) {
		_ = kube.GetClientSet(ctx, logger)
		viper.Set(config.CheckInterval, 0)
		before := time.Now()
		h.ReconciliationLoop()
		errorLogs := observedLogs.FilterLevelExact(zapcore.Level(2)).Filter(func(e observer.LoggedEntry) bool { return e.Time.After(before) }).All()
		assert.Len(t, errorLogs, 0)
	})
	t.Run("reconciliation loop with error", func(t *testing.T) {
		s2 := service.DeepCopy()
		s2.Name = "service2"
		s2.Namespace = "alternative"
		s2.Annotations[viper.GetString(config.IngressPathAnnotation)] = "/path2"
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{service.DeepCopy(), s2, ingress.DeepCopy()})
		_ = kube.GetClientSet(ctx, logger)
		viper.Set(config.CheckInterval, 0)
		before := time.Now()
		h.ReconciliationLoop()
		errorLogs := observedLogs.FilterLevelExact(zapcore.Level(2)).Filter(func(e observer.LoggedEntry) bool { return e.Time.After(before) }).All()
		assert.GreaterOrEqual(t, len(errorLogs), 1)
	})

}
