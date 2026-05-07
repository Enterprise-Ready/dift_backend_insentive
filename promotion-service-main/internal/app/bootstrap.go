package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"promotion-service/config"
	admin "promotion-service/internal/adapter/inbound/event/admin"
	adapterhttp "promotion-service/internal/adapter/inbound/http"
	postgresrepo "promotion-service/internal/adapter/outbound/persistence/postgres"
	httpinfra "promotion-service/internal/integration/http"
	natsinfra "promotion-service/internal/integration/nats"
	"promotion-service/internal/integration/postgres"
	adminhandler "promotion-service/internal/service_logic/handler/admin"
	newshandler "promotion-service/internal/service_logic/handler/news"
	promotionhandler "promotion-service/internal/service_logic/handler/promotion"
	adminservice "promotion-service/internal/service_logic/service/admin"
	newsservice "promotion-service/internal/service_logic/service/news"
	promotionservice "promotion-service/internal/service_logic/service/promotion"
	"promotion-service/pkg/health"
	"promotion-service/pkg/metrics"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
)

func Bootstrap(ctx context.Context, cfg *config.Config) (*App, error) {
	if cfg.App.Env == "prod" || cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	db, err := postgres.New(buildPostgresDSN(cfg))
	if err != nil {
		return nil, err
	}
	configureDBPool(db, cfg)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	promotionRepo := postgresrepo.NewPostgresPromotionRepository(db)
	newsRepo := postgresrepo.NewPostgresNewsRepository(db)
	promotionSvc := promotionservice.NewPromotionService(promotionRepo)
	newsSvc := newsservice.NewNewsService(newsRepo)
	promotionAdminSvc := adminservice.NewPromotionManagementService(promotionSvc)
	promotionAdminFlow := adminhandler.NewPromotionAdminHandler(promotionAdminSvc)
	promotionFlow := promotionhandler.NewPromotionHandler(promotionSvc)
	newsFlow := newshandler.NewNewsHandler(newsSvc)
	healthFlow := adapterhttp.NewHealthHandler(postgres.NewHealthChecker(db))
	router := gin.New()
	router = wireHTTPMiddleware(router)
	adapterhttp.RegisterRoutes(router, adapterhttp.Handlers{Promotion: adapterhttp.NewPromotionHandler(promotionFlow), News: adapterhttp.NewNewsHandler(newsFlow), Health: healthFlow})
	hc := health.New("promotion-service", "dev")
	hc.Register("postgres", true, func(context.Context) error { return db.Ping() })
	router.GET("/metrics/business", gin.WrapF(metrics.Handler()))
	router.GET("/health/live", gin.WrapF(hc.LiveHandler()))
	router.GET("/health/ready", gin.WrapF(hc.ReadyHandler()))
	httpServer := httpinfra.NewServer(fmt.Sprintf(":%d", cfg.Server.Port), router, time.Duration(cfg.Server.ReadTimeoutSec)*time.Second, time.Duration(cfg.Server.WriteTimeoutSec)*time.Second, time.Duration(cfg.Server.IdleTimeoutSec)*time.Second)
	application := &App{HTTPServer: httpServer, Closers: []func() error{db.Close}}
	if cfg.NATS.Enabled {
		nc, err := natsinfra.NewConnection(natsinfra.Config{URL: cfg.NATS.URL, MaxReconnect: 10, ReconnectWait: 2 * time.Second, ClientName: "promotion-service"})
		if err == nil {
			application.Closers = append(application.Closers, func() error { nc.Close(); return nil })
			js, err := natsinfra.SetupJetStream(nc, natsinfra.StreamConfig{Name: cfg.NATS.Stream, Subjects: []string{cfg.NATS.Subject}, Replicas: 1})
			if err == nil {
				consumer := admin.NewAdminPromotionConsumer(js, cfg.NATS.Stream, cfg.NATS.Subject, cfg.NATS.Durable, promotionAdminFlow)
				go func() {
					if err := consumer.Start(ctx); err != nil {
						log.Printf("admin promotion consumer stopped: %v", err)
					}
				}()
			}
			adminSubject := strings.TrimSpace(os.Getenv("ADMIN_CONTROL_SUBJECT"))
			if adminSubject == "" {
				adminSubject = "admin.control.promotion-service.command"
			}
			_, _ = nc.Subscribe(adminSubject, func(m *nats.Msg) {
				var cmd map[string]any
				if err := json.Unmarshal(m.Data, &cmd); err == nil {
					log.Printf("admin command accepted service=promotion-service action=%v", cmd["action"])
				}
			})
		}
	}
	return application, nil
}

func buildPostgresDSN(cfg *config.Config) string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.Name, cfg.DB.SSLMode)
}
func configureDBPool(db *sql.DB, cfg *config.Config) {
	db.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	db.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.DB.ConnMaxLifetimeMinute) * time.Minute)
}
