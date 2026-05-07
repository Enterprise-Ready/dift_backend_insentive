package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	httpadapter "github.com/enterprise/payment-gateway/internal/adapter/inbound/http"
	postgresrepo "github.com/enterprise/payment-gateway/internal/adapter/outbound/persistence/postgres"
	"github.com/enterprise/payment-gateway/internal/config"
	pginfra "github.com/enterprise/payment-gateway/internal/integration/postgres"
	redisinfra "github.com/enterprise/payment-gateway/internal/integration/redis"
	"github.com/enterprise/payment-gateway/internal/service_logic/service"
	"github.com/enterprise/payment-gateway/internal/service_logic/service/providers"
	"github.com/enterprise/payment-gateway/pkg/audit"
	"github.com/enterprise/payment-gateway/pkg/circuit"
	"github.com/enterprise/payment-gateway/pkg/idempotency"
	"github.com/enterprise/payment-gateway/pkg/logger"
	"github.com/enterprise/payment-gateway/pkg/metrics"
	"github.com/enterprise/payment-gateway/pkg/ratelimit"
	"go.uber.org/zap"
)

func Bootstrap(parent context.Context) (*App, error) {
	log := logger.New()
	log.Info("🚀 Starting Enterprise Payment Gateway")
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	db, err := pginfra.Connect(ctx, &cfg.Database)
	if err != nil {
		cancel()
		return nil, err
	}
	redisClient, err := redisinfra.Connect(ctx, &cfg.Redis)
	if err != nil {
		cancel()
		_ = db.Close()
		return nil, err
	}

	paymentRepo := postgresrepo.NewPaymentRepo(db)
	merchantRepo := postgresrepo.NewMerchantRepo(db)
	auditRepo := postgresrepo.NewAuditRepo(db)
	webhookRepo := postgresrepo.NewWebhookRepo(db)

	m := metrics.NewMetrics(cfg.Metrics.Namespace)
	cb := circuit.NewManager(log)
	idempotencyStore := idempotency.NewStore(redisClient, 24*time.Hour)
	rateLimiter := ratelimit.NewLimiter(redisClient)
	auditLogger := audit.NewLogger(auditRepo, log)

	registry := service.NewRegistry()
	if cfg.Providers.Omise.Enabled {
		registry.Register(providers.NewOmiseProvider(&cfg.Providers.Omise))
		log.Info("✅ Omise provider registered")
	}
	if cfg.Providers.GBPrimePay.Enabled {
		registry.Register(providers.NewGBPrimePayProvider(&cfg.Providers.GBPrimePay))
		log.Info("✅ GBPrimePay provider registered")
	}
	if cfg.Providers.Stripe.Enabled {
		registry.Register(providers.NewStripeProvider(&cfg.Providers.Stripe))
		log.Info("✅ Stripe provider registered")
	}

	riskEngine := service.NewRiskEngine(&cfg.Risk, redisClient, log)
	webhookSvc := service.NewWebhookService(webhookRepo, &cfg.Webhook, m, log)
	paymentSvc := service.NewPaymentService(paymentRepo, merchantRepo, registry, riskEngine, cb, idempotencyStore, auditLogger, m, webhookSvc, cfg, log)
	paymentHandler := httpadapter.NewPaymentHandler(paymentSvc, log)
	router := httpadapter.NewRouter(paymentHandler, merchantRepo, rateLimiter, m, log, cfg.Server.Environment)

	go startBackgroundJobs(ctx, webhookSvc, m, log)
	go startMetricsCollector(ctx, db, m)

	srv := &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port), Handler: router.Handler(), ReadTimeout: cfg.Server.ReadTimeout, WriteTimeout: cfg.Server.WriteTimeout, IdleTimeout: 120 * time.Second}
	return &App{Server: srv, DB: db, Redis: redisClient, Metrics: m, WebhookSvc: webhookSvc, Logger: log, Cancel: cancel}, nil
}

func Run() {
	root := context.Background()
	app, err := Bootstrap(root)
	if err != nil {
		logger.New().Fatal("bootstrap failed", zap.Error(err))
	}
	if err := app.Start(); err != nil {
		app.Logger.Fatal("start failed", zap.Error(err))
	}
	WaitForShutdown(app)
}
