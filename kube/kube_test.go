package kube

import (
	"context"
	"github.com/ptonini/ingress-bot/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"testing"
)

const kubeConfigContent = "apiVersion: v1\nkind: Config\nclusters: [{name: dummy, cluster: {server: https://dummy:443}}]\ncontexts: [{name: dummy, context: {cluster: dummy}}]\ncurrent-context: dummy"

func Test_Kube(t *testing.T) {

	kubeConfigFile, _ := os.CreateTemp("", "tmpfile-")
	defer func(f *os.File) {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}(kubeConfigFile)

	config.Load()
	ctx := context.Background()
	observedZapCore, _ := observer.New(zap.DebugLevel)
	logger := zap.New(observedZapCore)
	_, _ = kubeConfigFile.WriteString(kubeConfigContent)

	t.Run("create client set with no config", func(t *testing.T) {
		_, err := createClientSet(ctx, logger)
		assert.Error(t, err)
	})

	t.Run("create client set with invalid config", func(t *testing.T) {
		viper.Set(config.KubeconfigPath, "invalid")
		_, err := createClientSet(ctx, logger)
		assert.Error(t, err)
	})

	t.Run("create client set with kubeconfig", func(t *testing.T) {
		viper.Set(config.KubeconfigPath, kubeConfigFile.Name())
		_, err := createClientSet(ctx, logger)
		assert.NoError(t, err)
	})

	t.Run("create flake client set", func(t *testing.T) {
		ctx = context.WithValue(ctx, viper.GetString(config.ContextTestingKey), true)
		ctx = context.WithValue(ctx, viper.GetString(config.ContextFakeObjectsKey), []runtime.Object{})
		_, err := createClientSet(ctx, logger)
		assert.NoError(t, err)

	})

	t.Run("get client set", func(t *testing.T) {
		ctx = context.WithValue(ctx, viper.GetString(config.ContextTestingKey), true)
		assert.NoError(t, GetClientSet(ctx, logger))
	})

}
