package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	HTTP     HTTPConfig     `mapstructure:"http"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Redpanda RedpandaConfig `mapstructure:"redpanda"`
	NATS     NATSConfig     `mapstructure:"nats"`
}

// =====================
// App
// =====================

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"` // dev | staging | prod
}

// =====================
// HTTP
// =====================

type HTTPConfig struct {
	Port string `mapstructure:"port"`
}

// =====================
// gRPC
// =====================

type GRPCConfig struct {
	Port string `mapstructure:"port"`
}

// =====================
// Database
// =====================

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

// =====================
// Redis
// =====================

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// =====================
// Redpanda
// =====================

type RedpandaConfig struct {
	Brokers []string            `mapstructure:"brokers"`
	GroupID string              `mapstructure:"group_id"`
	Topics  RedpandaTopicConfig `mapstructure:"topics"`
}

type RedpandaTopicConfig struct {
	RewardCouponCommand string `mapstructure:"reward_coupon_command"`
	UserCouponCreated   string `mapstructure:"user_coupon_created"`
}

// =====================
// NATS
// =====================

type NATSConfig struct {
	URL          string `mapstructure:"url"`
	Stream       string `mapstructure:"stream"`
	Subject      string `mapstructure:"subject"`
	Durable      string `mapstructure:"durable"`
	AdminStream  string `mapstructure:"admin_stream"`
	AdminSubject string `mapstructure:"admin_subject"`
	AdminDurable string `mapstructure:"admin_durable"`
}

// =====================
// Load config
// =====================

func Load() Config {

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// defaults
	viper.SetDefault("app.name", "coupon-service")
	viper.SetDefault("app.env", "dev")
	viper.SetDefault("http.port", "8080")
	viper.SetDefault("grpc.port", "9090")
	viper.SetDefault("redis.addr", "")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("nats.url", "nats://localhost:4222")
	viper.SetDefault("nats.stream", "COUPON_EVENTS")
	viper.SetDefault("nats.subject", "coupon.>")
	viper.SetDefault("nats.durable", "coupon-outbox")
	viper.SetDefault("nats.admin_stream", "ADMIN_EVENTS")
	viper.SetDefault("nats.admin_subject", "admin.coupon.*")
	viper.SetDefault("nats.admin_durable", "coupon-admin-consumer")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("read config error: %v", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("unmarshal config error: %v", err)
	}

	validate(cfg)

	return cfg
}

// =====================
// Validation
// =====================

func validate(cfg Config) {

	if cfg.Database.DSN == "" {
		log.Fatal("database.dsn is required")
	}

	if len(cfg.Redpanda.Brokers) == 0 {
		log.Println("warning: redpanda.brokers not set")
	}

	if cfg.NATS.URL == "" {
		log.Println("warning: nats.url not set")
	}
}
