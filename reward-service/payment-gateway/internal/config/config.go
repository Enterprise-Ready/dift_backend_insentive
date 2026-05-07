package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	JWT       JWTConfig
	Providers ProvidersConfig
	Risk      RiskConfig
	Webhook   WebhookConfig
	Metrics   MetricsConfig
	Crypto    CryptoConfig
	Limits    LimitsConfig
}

type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	Host            string        `mapstructure:"host"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	GracefulTimeout time.Duration `mapstructure:"graceful_timeout"`
	TLSEnabled      bool          `mapstructure:"tls_enabled"`
	TLSCertPath     string        `mapstructure:"tls_cert_path"`
	TLSKeyPath      string        `mapstructure:"tls_key_path"`
	Environment     string        `mapstructure:"environment"`
	Version         string        `mapstructure:"version"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Name            string        `mapstructure:"name"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	MigrationPath   string        `mapstructure:"migration_path"`
}

type RedisConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	ClusterMode  bool          `mapstructure:"cluster_mode"`
	Addrs        []string      `mapstructure:"addrs"`
}

type JWTConfig struct {
	Secret          string        `mapstructure:"secret"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
	Issuer          string        `mapstructure:"issuer"`
}

type ProvidersConfig struct {
	Omise      OmiseConfig      `mapstructure:"omise"`
	GBPrimePay GBPrimePayConfig `mapstructure:"gbprimepay"`
	KBank      BankConfig       `mapstructure:"kbank"`
	SCB        BankConfig       `mapstructure:"scb"`
	KTB        BankConfig       `mapstructure:"ktb"`
	BBL        BankConfig       `mapstructure:"bbl"`
	TrueWallet WalletConfig     `mapstructure:"truewallet"`
	LinePay    WalletConfig     `mapstructure:"linepay"`
	Stripe     StripeConfig     `mapstructure:"stripe"`
	PayPal     PayPalConfig     `mapstructure:"paypal"`
	TwoC2P     TwoC2PConfig     `mapstructure:"twoc2p"`
}

type OmiseConfig struct {
	PublicKey string        `mapstructure:"public_key"`
	SecretKey string        `mapstructure:"secret_key"`
	BaseURL   string        `mapstructure:"base_url"`
	Timeout   time.Duration `mapstructure:"timeout"`
	Enabled   bool          `mapstructure:"enabled"`
}

type GBPrimePayConfig struct {
	Token     string        `mapstructure:"token"`
	SecretKey string        `mapstructure:"secret_key"`
	BaseURL   string        `mapstructure:"base_url"`
	Timeout   time.Duration `mapstructure:"timeout"`
	Enabled   bool          `mapstructure:"enabled"`
}

type BankConfig struct {
	MerchantID string        `mapstructure:"merchant_id"`
	APIKey     string        `mapstructure:"api_key"`
	APISecret  string        `mapstructure:"api_secret"`
	BaseURL    string        `mapstructure:"base_url"`
	Timeout    time.Duration `mapstructure:"timeout"`
	Enabled    bool          `mapstructure:"enabled"`
}

type WalletConfig struct {
	ChannelID string        `mapstructure:"channel_id"`
	SecretKey string        `mapstructure:"secret_key"`
	BaseURL   string        `mapstructure:"base_url"`
	Timeout   time.Duration `mapstructure:"timeout"`
	Enabled   bool          `mapstructure:"enabled"`
}

type StripeConfig struct {
	SecretKey  string        `mapstructure:"secret_key"`
	WebhookKey string        `mapstructure:"webhook_key"`
	BaseURL    string        `mapstructure:"base_url"`
	Timeout    time.Duration `mapstructure:"timeout"`
	Enabled    bool          `mapstructure:"enabled"`
}

type PayPalConfig struct {
	ClientID string        `mapstructure:"client_id"`
	Secret   string        `mapstructure:"secret"`
	BaseURL  string        `mapstructure:"base_url"`
	Timeout  time.Duration `mapstructure:"timeout"`
	Enabled  bool          `mapstructure:"enabled"`
	Sandbox  bool          `mapstructure:"sandbox"`
}

type TwoC2PConfig struct {
	MerchantID string        `mapstructure:"merchant_id"`
	SecretKey  string        `mapstructure:"secret_key"`
	BaseURL    string        `mapstructure:"base_url"`
	Timeout    time.Duration `mapstructure:"timeout"`
	Enabled    bool          `mapstructure:"enabled"`
}

type RiskConfig struct {
	Enabled                bool    `mapstructure:"enabled"`
	MaxRiskScore           int     `mapstructure:"max_risk_score"`
	BlockScore             int     `mapstructure:"block_score"`
	ReviewScore            int     `mapstructure:"review_score"`
	VelocityWindowMinutes  int     `mapstructure:"velocity_window_minutes"`
	MaxTransactionsPerHour int     `mapstructure:"max_transactions_per_hour"`
	MaxAmountPerHour       float64 `mapstructure:"max_amount_per_hour"`
	EnableIPBlacklist      bool    `mapstructure:"enable_ip_blacklist"`
	EnableCardBlacklist    bool    `mapstructure:"enable_card_blacklist"`
	EnableDeviceTracking   bool    `mapstructure:"enable_device_tracking"`
}

type WebhookConfig struct {
	MaxRetries      int           `mapstructure:"max_retries"`
	RetryBackoff    time.Duration `mapstructure:"retry_backoff"`
	Timeout         time.Duration `mapstructure:"timeout"`
	SignatureHeader string        `mapstructure:"signature_header"`
	WorkerCount     int           `mapstructure:"worker_count"`
}

type MetricsConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Path      string `mapstructure:"path"`
	Namespace string `mapstructure:"namespace"`
}

type CryptoConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"`
	MasterKeyID   string `mapstructure:"master_key_id"`
	UseKMS        bool   `mapstructure:"use_kms"`
	KMSRegion     string `mapstructure:"kms_region"`
}

type LimitsConfig struct {
	MaxPaymentAmount     float64 `mapstructure:"max_payment_amount"`
	MinPaymentAmount     float64 `mapstructure:"min_payment_amount"`
	DailyLimitPerMerch   float64 `mapstructure:"daily_limit_per_merchant"`
	DailyLimitPerCard    float64 `mapstructure:"daily_limit_per_card"`
	RateLimitPerSecond   int     `mapstructure:"rate_limit_per_second"`
	RateLimitBurst       int     `mapstructure:"rate_limit_burst"`
	QRCodeExpireMinutes  int     `mapstructure:"qr_code_expire_minutes"`
	PaymentExpireMinutes int     `mapstructure:"payment_expire_minutes"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.graceful_timeout", "10s")
	viper.SetDefault("server.environment", "production")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", "5m")
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("webhook.max_retries", 5)
	viper.SetDefault("webhook.worker_count", 10)
	viper.SetDefault("limits.qr_code_expire_minutes", 15)
	viper.SetDefault("limits.payment_expire_minutes", 30)
	viper.SetDefault("limits.rate_limit_per_second", 100)
	viper.SetDefault("risk.max_transactions_per_hour", 50)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
