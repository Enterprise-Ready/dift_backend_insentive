package app

import (
	"context"
	"net/http"
	"time"

	"github.com/enterprise/payment-gateway/internal/service_logic/service"
	"github.com/enterprise/payment-gateway/pkg/metrics"
	"github.com/jmoiron/sqlx"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type App struct {
	Server     *http.Server
	DB         *sqlx.DB
	Redis      goredis.UniversalClient
	Metrics    *metrics.Metrics
	WebhookSvc *service.WebhookService
	Logger     *zap.Logger
	Cancel     context.CancelFunc
}

func (a *App) Start() error {
	go func() {
		a.Logger.Info("🌐 HTTP server listening", zap.String("addr", a.Server.Addr))
		if err := a.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.Logger.Fatal("server error", zap.Error(err))
		}
	}()
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	if a.Cancel != nil {
		a.Cancel()
	}
	if a.Redis != nil {
		_ = a.Redis.Close()
	}
	if a.DB != nil {
		_ = a.DB.Close()
	}
	return a.Server.Shutdown(ctx)
}

func startBackgroundJobs(ctx context.Context, webhookSvc *service.WebhookService, m *metrics.Metrics, logger *zap.Logger) {
	webhookTicker := time.NewTicker(30 * time.Second)
	expiredTicker := time.NewTicker(5 * time.Minute)
	defer webhookTicker.Stop()
	defer expiredTicker.Stop()
	for {
		select {
		case <-webhookTicker.C:
			if err := webhookSvc.RetryPending(ctx); err != nil {
				logger.Error("webhook retry job failed", zap.Error(err))
			}
		case <-expiredTicker.C:
			logger.Debug("checking for expired payments")
		case <-ctx.Done():
			logger.Info("background jobs stopped")
			return
		}
	}
}

func startMetricsCollector(ctx context.Context, db *sqlx.DB, m *metrics.Metrics) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			stats := db.Stats()
			m.DBConnectionPool.WithLabelValues("open").Set(float64(stats.OpenConnections))
			m.DBConnectionPool.WithLabelValues("idle").Set(float64(stats.Idle))
			m.DBConnectionPool.WithLabelValues("in_use").Set(float64(stats.InUse))
		case <-ctx.Done():
			return
		}
	}
}
