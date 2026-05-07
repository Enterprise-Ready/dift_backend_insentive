package app

import (
	"context"
	"net/http"
)

type App struct {
	Infra    Infra
	Adapters Adapters
	logger   Logger
}

type Logger interface {
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

func (a *App) Start(ctx context.Context) error {
	if a.Adapters.OutboxWorker != nil {
		go a.Adapters.OutboxWorker.Start(ctx)
	}
	if a.Infra.HTTPServer != nil {
		go func() {
			if err := a.Infra.HTTPServer.Start(); err != nil && err != http.ErrServerClosed {
				a.logger.Errorf("http server error: %v", err)
			}
		}()
	}
	if a.Infra.GRPCServer != nil {
		go func() {
			if err := a.Infra.GRPCServer.Start(); err != nil {
				a.logger.Errorf("grpc server error: %v", err)
			}
		}()
	}
	if a.Adapters.AdminConsumer != nil {
		go func() {
			if err := a.Adapters.AdminConsumer.Start(ctx); err != nil {
				a.logger.Errorf("admin nats consumer error: %v", err)
			}
		}()
	}
	return nil
}

func (a *App) Shutdown(ctx context.Context) {
	if a.Infra.GRPCServer != nil {
		a.Infra.GRPCServer.Stop(ctx)
	}
	if a.Infra.HTTPServer != nil {
		if err := a.Infra.HTTPServer.Stop(ctx); err != nil {
			a.logger.Errorf("http shutdown error: %v", err)
		}
	}
	if a.Infra.NATSConn != nil {
		a.Infra.NATSConn.Close()
	}
	if a.Infra.Redis != nil {
		if err := a.Infra.Redis.Close(); err != nil {
			a.logger.Errorf("redis close error: %v", err)
		}
	}
	if a.Infra.DB != nil {
		if err := a.Infra.DB.Close(); err != nil {
			a.logger.Errorf("db close error: %v", err)
		}
	}
}
