package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP HTTPConfig `yaml:"http"`
	GRPC GRPCConfig `yaml:"grpc"`

	NATS NATSConfig `yaml:"nats"`

	DB DBConfig `yaml:"db"`
}

// =====================
// HTTP
// =====================

type HTTPConfig struct {
	Address string `yaml:"address"`
}

// =====================
// GRPC
// =====================

type GRPCConfig struct {
	UserRewardAddress string `yaml:"user_reward_address"`
}

// =====================
// NATS
// =====================

type NATSConfig struct {
	URL                  string `yaml:"url"`
	StreamName           string `yaml:"stream_name"`
	DurableName          string `yaml:"durable_name"`
	SubjectHistory       string `yaml:"subject_history"`
	SubjectRewardEarn    string `yaml:"subject_reward_earn"`
	SubjectRedeemRequest string `yaml:"subject_redeem_request"`
	SubjectRedeemResult  string `yaml:"subject_redeem_result"`
}

// =====================
// Database
// =====================

type DBConfig struct {
	DSN string `yaml:"dsn"`
}

// =====================
// Load
// =====================

func Load() *Config {

	file := getEnv("CONFIG_FILE", "config.yaml")

	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("cannot read config file: %v", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("cannot parse config yaml: %v", err)
	}

	return &cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
