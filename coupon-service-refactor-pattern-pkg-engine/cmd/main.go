package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"coupon-service/config"
	"coupon-service/internal/app"
	applogger "coupon-service/pkg/logger"
)

func main() {
	logger := applogger.New("coupon-service")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg := config.Load()
	features := config.LoadFeatureFlags()

	application, err := app.Bootstrap(ctx, cfg, features, logger)
	if err != nil {
		logger.Fatalf("bootstrap failed: %v", err)
	}

	if err := application.Start(ctx); err != nil {
		logger.Fatalf("start failed: %v", err)
	}

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	application.Shutdown(shutdownCtx)
}
