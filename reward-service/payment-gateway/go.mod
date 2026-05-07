module github.com/enterprise/payment-gateway

go 1.22

require (
	github.com/PlatformCore/libpackage/middleware v0.0.0
	github.com/PlatformCore/libpackage/resilience v0.0.0
	github.com/gin-gonic/gin v1.9.1
	github.com/google/uuid v1.6.0
	github.com/redis/go-redis/v9 v9.5.1
	github.com/lib/pq v1.10.9
	github.com/jmoiron/sqlx v1.3.5
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/spf13/viper v1.18.2
	github.com/prometheus/client_golang v1.19.0
	go.uber.org/zap v1.27.0
	go.uber.org/fx v1.21.0
	github.com/sony/gobreaker v0.5.0
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	golang.org/x/crypto v0.21.0
	github.com/go-playground/validator/v10 v10.19.0
	github.com/golang-migrate/migrate/v4 v4.17.0
	github.com/stretchr/testify v1.9.0
	github.com/shopspring/decimal v1.3.1
	github.com/robfig/cron/v3 v3.0.1
	go.opentelemetry.io/otel v1.24.0
	go.opentelemetry.io/otel/trace v1.24.0
	github.com/aws/aws-sdk-go-v2 v1.26.1
	github.com/aws/aws-sdk-go-v2/service/kms v1.30.1
	golang.org/x/time v0.5.0
)

// Local development can replace these with checked-out lib v13 modules.
// replace github.com/PlatformCore/libpackage/middleware => ../libpack-v11-middleware-wrapper-sdk
// replace github.com/PlatformCore/libpackage/resilience => ../libpack-v11-engine-core-sdk/resilience
