package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"reward-service/config"
	natsAdminAdapter "reward-service/internal/adapter/inbound/event/admin"
	enterpriseCalc "reward-service/internal/service_logic/service/calculation"
	enterpriseObs "reward-service/pkg/observability"
	enterpriseOutbox "reward-service/pkg/outbox"
	enterpriseResilience "reward-service/pkg/resilience"

	eventadapter "reward-service/internal/adapter/inbound/event"
	httpadapter "reward-service/internal/adapter/inbound/http"
	natsAdapter "reward-service/internal/adapter/outbound/messaging/nats"
	repoAdapter "reward-service/internal/adapter/outbound/persistence/postgres"
	ruleDomain "reward-service/internal/domain"
	httpInfra "reward-service/internal/integration/http"
	natsInfra "reward-service/internal/integration/nats"
	dbInfra "reward-service/internal/integration/postgres"
	adminHandler "reward-service/internal/service_logic/handler/admin"
	historyHandler "reward-service/internal/service_logic/handler/history"
	redeemHandler "reward-service/internal/service_logic/handler/redeem"
	adminSvc "reward-service/internal/service_logic/service/admin"
	redeemSvc "reward-service/internal/service_logic/service/redeem"
	servicecore "reward-service/pkg/enginebundle"
	loggerwrapper "reward-service/pkg/logger"
	metricswrapper "reward-service/pkg/metrics"
	tracingwrapper "reward-service/pkg/tracing"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
)

func Run() {
	_ = servicecore.NewEngineUnifiedBundle(servicecore.LoadEngineUnifiedConfigFromEnv("reward-service"))
	appLogger := loggerwrapper.New("reward-service")
	_ = tracingwrapper.NewProvider("reward-service")
	businessMetrics := metricswrapper.NewBusinessMetrics()

	cfg := config.Load()
	feature := config.LoadFeature()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := dbInfra.NewPostgres(cfg.DB)
	if err != nil {
		appLogger.Fatalf("[DB] error: %v", err)
	}
	defer dbInfra.Close(db)

	natsClient, err := natsInfra.NewClient(cfg.NATS.URL)
	if err != nil {
		appLogger.Fatalf("nats connect error: %v", err)
	}
	defer natsClient.Close()

	if err := natsInfra.EnsureStream(
		natsClient.Js,
		cfg.NATS.StreamName,
		[]string{
			cfg.NATS.SubjectHistory,
			cfg.NATS.SubjectRewardEarn,
			cfg.NATS.SubjectRedeemRequest,
			cfg.NATS.SubjectRedeemResult,
			"admin.control.reward-service.command",
		},
	); err != nil {
		appLogger.Fatalf("ensure stream error: %v", err)
	}

	earnRepo := repoAdapter.NewEarnRepository(db)
	redeemRepo := repoAdapter.NewRewardRedeemRepository(db)
	rewardRule := ruleDomain.NewDefaultRewardRule()
	metrics := enterpriseObs.NewMetrics(prometheus.DefaultRegisterer)
	logger := enterpriseObs.NewLogger("reward-service", slog.LevelInfo)

	cbCfg := enterpriseResilience.DefaultCircuitBreakerConfig()
	cbCfg.OnStateChange = func(name string, from, to enterpriseResilience.State) {
		metrics.CircuitBreakerState.WithLabelValues(name).Set(stateToGauge(to))
		if to == enterpriseResilience.StateOpen {
			metrics.CircuitBreakerTripped.WithLabelValues(name).Inc()
		}
	}
	publishCircuitBreaker := enterpriseResilience.NewCircuitBreaker("reward_earn_publish", cbCfg)
	metrics.CircuitBreakerState.WithLabelValues("reward_earn_publish").Set(
		stateToGauge(enterpriseResilience.StateClosed),
	)

	jsPublisher, err := natsInfra.NewJetStreamPublisher(natsClient.Conn)
	if err != nil {
		appLogger.Fatalf("jetstream publisher error: %v", err)
	}

	outboxStore := enterpriseOutbox.NewStore(db)
	outboxRelay := enterpriseOutbox.NewRelay(
		outboxStore,
		func(ctx context.Context, subject string, payload []byte) error {
			return jsPublisher.Publish(ctx, subject, payload)
		},
		2*time.Second,
	)
	go outboxRelay.Start(ctx)

	rewardEarnProducer := natsAdapter.NewRewardEarnProducer(
		jsPublisher.Publish,
		cfg.NATS.SubjectRewardEarn,
	)

	rewardService := enterpriseCalc.NewEnterpriseRewardCalculationService(
		earnRepo,
		rewardRule,
		rewardEarnProducer,
		metrics,
		logger,
		publishCircuitBreaker,
	)
	historyFlow := historyHandler.NewHistoryHandler(rewardService)

	redeemProducer := natsAdapter.NewRedeemRequestProducer(
		natsClient.Conn,
		cfg.NATS.SubjectRedeemRequest,
	)

	redeemRequestService := redeemSvc.NewRedeemRequestService(
		redeemRepo,
		redeemProducer,
	)
	redeemResultService := redeemSvc.NewRedeemResultService(redeemRepo)
	redeemRequestFlow := redeemHandler.NewRedeemRequestHandler(redeemRequestService)
	redeemResultFlow := redeemHandler.NewRedeemResultHandler(redeemResultService)

	if feature.History.Consume.Enable {
		historyConsumer := eventadapter.NewHistoryConsumer(
			natsClient.Js,
			cfg.NATS.SubjectHistory,
			cfg.NATS.SubjectHistory,
			historyFlow,
		)
		go func() {
			if err := historyConsumer.Start(ctx); err != nil {
				appLogger.Errorf("[NATS] history consumer error: %v", err)
			}
		}()
	}

	if feature.Redeem.Consume.Enable {
		resultConsumer := eventadapter.NewRedeemResultConsumer(
			natsClient.Conn,
			cfg.NATS.SubjectRedeemResult,
			redeemResultFlow,
		)
		if err := resultConsumer.Start(ctx); err != nil {
			appLogger.Errorf("[NATS] redeem result consumer error: %v", err)
		}
	}

	adminCfg := AdminControlConfig{Subject: "admin.control.reward-service.command", Durable: "reward-admin-control-consumer"}
	commandService := adminSvc.NewCommandService(redeemRequestService, redeemResultService)
	commandHandler := adminHandler.NewCommandHandler(commandService)
	adminConsumer := natsAdminAdapter.NewCommandConsumer(commandHandler)
	_, err = natsClient.Js.Subscribe(
		adminCfg.Subject,
		func(msg *nats.Msg) {
			if err := adminConsumer.Handle(ctx, msg.Data); err != nil {
				appLogger.Errorf("admin command invalid payload: %v", err)
				_ = msg.Ack()
				return
			}
			appLogger.Infof("admin command accepted service=reward-service")
			_ = msg.Ack()
		},
		nats.Durable(adminCfg.Durable),
		nats.ManualAck(),
	)
	if err != nil {
		appLogger.Errorf("admin subscribe error: %v", err)
	}

	if feature.HTTP.Enable {
		router := gin.New()
		router.Use(
			enterpriseObs.PrometheusMiddleware(metrics),
			enterpriseObs.GinTracingMiddleware(logger),
			gin.Recovery(),
		)
		router.GET("/metrics/app", gin.WrapF(metricswrapper.Handler()))
		router.GET("/metrics/business", gin.WrapF(metricswrapper.BusinessHandler(businessMetrics)))
		enterpriseObs.RegisterMetricsRoute(router)

		httpadapter.RegisterRoutes(router, httpadapter.Handlers{
			Redeem: redeemRequestFlow,
		})

		var handler http.Handler = router
		middlewares := servicecore.DefaultHTTPMiddlewares()
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}

		server := httpInfra.NewServer(
			httpInfra.Config{Address: cfg.HTTP.Address},
			handler,
		)
		go server.Start()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	cancel()
	time.Sleep(2 * time.Second)

	appLogger.Infof("reward-service stopped")
}

func stateToGauge(state enterpriseResilience.State) float64 {
	switch state {
	case enterpriseResilience.StateOpen:
		return 1
	case enterpriseResilience.StateHalfOpen:
		return 2
	default:
		return 0
	}
}
