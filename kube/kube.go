package kube

import (
	"context"
	"fmt"
	"github.com/go-logr/zapr"
	"github.com/ptonini/ingress-bot/config"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sync"
)

var lock = &sync.Mutex{}

var ClientSet kubernetes.Interface

func createClientSet(ctx context.Context, logger *zap.Logger) (kubernetes.Interface, error) {
	var cs kubernetes.Interface
	var cfg *rest.Config
	var err error
	t := viper.GetString(config.ContextTestingKey)
	f := viper.GetString(config.ContextFakeObjectsKey)
	klog.SetLogger(zapr.NewLogger(logger))
	if ctx.Value(t) != nil && ctx.Value(t).(bool) {
		var objList []runtime.Object
		if ctx.Value(f) != nil {
			objList = ctx.Value(f).([]runtime.Object)
		}
		cs = fake.NewSimpleClientset(objList...)
	} else {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			cfg, err = clientcmd.BuildConfigFromFlags("", viper.GetString(config.KubeconfigPath))
		}
		if err != nil {
			return nil, fmt.Errorf("error loading kubernetes config: %v", err)
		}
		cs, err = kubernetes.NewForConfig(cfg)
	}
	return cs, err
}

func GetClientSet(ctx context.Context, logger *zap.Logger) error {
	var err error
	lock.Lock()
	defer lock.Unlock()
	ClientSet, err = createClientSet(ctx, logger)
	return err
}
