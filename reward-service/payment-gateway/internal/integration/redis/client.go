package redis

import (
	"context"
	"fmt"

	"github.com/enterprise/payment-gateway/internal/config"
	goredis "github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, cfg *config.RedisConfig) (goredis.UniversalClient, error) {
	var client goredis.UniversalClient
	if cfg.ClusterMode {
		client = goredis.NewClusterClient(&goredis.ClusterOptions{Addrs: cfg.Addrs, Password: cfg.Password, PoolSize: cfg.PoolSize, MinIdleConns: cfg.MinIdleConns, DialTimeout: cfg.DialTimeout, ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout})
	} else {
		client = goredis.NewClient(&goredis.Options{Addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), Password: cfg.Password, DB: cfg.DB, PoolSize: cfg.PoolSize, MinIdleConns: cfg.MinIdleConns, DialTimeout: cfg.DialTimeout, ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout})
	}
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}
