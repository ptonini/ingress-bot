package config

import (
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
)

const (
	LogLevel = "LOG_LEVEL"
)

var defaults = map[string]string{
	LogLevel: "INFO",
}

var LogLevels = map[string]zapcore.Level{
	"DEBUG":  -1,
	"INFO":   0,
	"WARN":   1,
	"ERROR":  2,
	"DPANIC": 3,
	"PANIC":  4,
	"FATAL":  5,
}

func Load() {

	viper.New()

	for k, v := range defaults {
		viper.SetDefault(k, v)
	}

	_ = viper.BindEnv(LogLevel)

}
