package config

import (
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
)

const (
	LogLevel               = "LOG_LEVEL"
	CheckInterval          = "CHECK_INTERVAL"
	DryRun                 = "DRY_RUN"
	ContextTestingKey      = "CONTEXT_TESTING_KEY"
	ContextFakeObjectsKey  = "CONTEXT_FAKE_OBJECTS_KEY"
	KubeconfigPath         = "KUBECONFIG_PATH"
	ResourceLabelKey       = "RESOURCE_LABEL_KEY"
	ResourceLabelValue     = "RESOURCE_LABEL_VALUE"
	ClientTimeout          = "CLIENT_TIMEOUT"
	IngressHostAnnotation  = "INGRESS_HOST_ANNOTATION"
	IngressClassAnnotation = "INGRESS_CLASS_ANNOTATION"
	IngressPathAnnotation  = "INGRESS_PATH_ANNOTATION"
	IngressEnableTLS       = "INGRESS_ENABLE_TLS"
	IngressAnnotations     = "INGRESS_ANNOTATIONS"
	IngressLabels          = "INGRESS_LABELS"
	IngressPathType        = "INGRESS_PATH_TYPE"
)

var defaults = map[string]string{
	LogLevel:               "info",
	CheckInterval:          "30",
	DryRun:                 "false",
	ContextTestingKey:      "testing",
	ContextFakeObjectsKey:  "fake_objects",
	ClientTimeout:          "60",
	ResourceLabelKey:       "ptonini.github.io/ingress-bot",
	ResourceLabelValue:     "true",
	IngressHostAnnotation:  "ptonini.github.io/ingress-host",
	IngressClassAnnotation: "ptonini.github.io/ingress-class",
	IngressPathAnnotation:  "ptonini.github.io/ingress-path",
	IngressEnableTLS:       "true",
	IngressPathType:        "ImplementationSpecific",
}

var LogLevels = map[string]zapcore.Level{
	"debug":  -1,
	"info":   0,
	"warn":   1,
	"error":  2,
	"dpanic": 3,
	"panic":  4,
	"fatal":  5,
}

func Load() {
	viper.New()
	for k, v := range defaults {
		viper.SetDefault(k, v)
	}
	viper.AutomaticEnv()
}
