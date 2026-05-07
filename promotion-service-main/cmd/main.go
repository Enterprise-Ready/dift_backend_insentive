package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"promotion-service/config"
	"promotion-service/internal/app"
	"promotion-service/pkg/logger"
)

func main() {
	_ = logger.New("promotion-service")
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC | log.Lmicroseconds)
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	application, err := app.Bootstrap(ctx, cfg)
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	go func() {
		if err := application.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	}()
	<-ctx.Done()
	_ = application.Shutdown(context.Background())
	log.Printf("service=%s status=stopped", cfg.App.Name)
}
