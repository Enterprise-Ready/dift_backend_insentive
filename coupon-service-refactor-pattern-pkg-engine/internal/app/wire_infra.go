package app

import (
	"context"
	"time"

	"coupon-service/config"
	natsinfra "coupon-service/internal/integration/nats"
	postgresinfra "coupon-service/internal/integration/postgres"

	"github.com/redis/go-redis/v9"
)

func wireInfra(ctx context.Context, cfg config.Config, features config.FeatureFlags, logger Logger) (Infra, error) {
	var infra Infra
	if features.EnableDatabase {
		db, err := postgresinfra.NewPostgres(cfg.Database.DSN)
		if err != nil {
			return infra, err
		}
		infra.DB = db
	}
	if cfg.Redis.Addr != "" {
		rc := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
		if err := rc.Ping(ctx).Err(); err != nil {
			logger.Warnf("redis unavailable, saga rate limit disabled: %v", err)
			_ = rc.Close()
		} else {
			infra.Redis = rc
		}
	}
	if features.EnableNATSProducer || features.EnableNATSConsumer {
		nc, err := natsinfra.NewConnection(natsinfra.Config{URL: cfg.NATS.URL, MaxReconnect: 10, ReconnectWait: 2 * time.Second, ClientName: "coupon-service"})
		if err != nil {
			return infra, err
		}
		infra.NATSConn = nc
		js, err := natsinfra.SetupJetStream(nc, natsinfra.StreamConfig{Name: cfg.NATS.Stream, Subjects: []string{cfg.NATS.Subject}, Replicas: 1})
		if err != nil {
			return infra, err
		}
		infra.NATSJS = js
		if features.EnableNATSConsumer {
			if _, err := natsinfra.SetupJetStream(nc, natsinfra.StreamConfig{Name: cfg.NATS.AdminStream, Subjects: []string{cfg.NATS.AdminSubject}, Replicas: 1}); err != nil {
				return infra, err
			}
		}
	}
	return infra, nil
}
