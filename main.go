package main

import (
	"context"
	"github.com/ptonini/ingress-bot/config"
	"github.com/ptonini/ingress-bot/handler"
	"github.com/ptonini/ingress-bot/kube"
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

	ctx := context.Background()
	config.Load()

	// Create logger instance
	logLevel := config.LogLevels[viper.GetString(config.LogLevel)]
	logger := zap.New(ecszap.NewCore(ecszap.NewDefaultEncoderConfig(), os.Stdout, logLevel), zap.AddCaller())
	logger.Info("starting service")

	// Create kubernetes client set
	err := kube.GetClientSet(ctx, logger)
	if err != nil {
		logger.Fatal(err.Error())
	}

	// Create handler and start reconciliation loop
	h := handler.Factory(ctx, logger, viper.GetInt64(config.ClientTimeout))
	go h.ReconciliationLoop()

	<-shutdown
	logger.Info("stopping service")

}
