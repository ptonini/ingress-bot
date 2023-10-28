package main

import (
	"context"
	"github.com/ptonini/ingress-bot/config"
	"github.com/spf13/viper"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	// Catch shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGTERM, syscall.SIGINT)

	_ = context.Background()

	config.Load()

	// Create logger instance
	logLevel := config.LogLevels[viper.GetString(config.LogLevel)]
	logger := zap.New(ecszap.NewCore(ecszap.NewDefaultEncoderConfig(), os.Stdout, logLevel), zap.AddCaller())
	logger.Info("starting service")

	<-shutdown
	logger.Info("stopping service")

}
