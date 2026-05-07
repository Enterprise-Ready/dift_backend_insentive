package app

import (
	"database/sql"
	"net/http"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	natsadmin "coupon-service/internal/adapter/inbound/event/natsadmin"
	grpcadapter "coupon-service/internal/adapter/inbound/grpc"
	publichttp "coupon-service/internal/adapter/inbound/http/claimquery"
	outboxworker "coupon-service/internal/adapter/inbound/worker"
	redisadapter "coupon-service/internal/adapter/outbound/cache/redis"
	natsproducer "coupon-service/internal/adapter/outbound/messaging/nats"
	repoadapter "coupon-service/internal/adapter/outbound/persistence/postgres"
	grpcinfra "coupon-service/internal/integration/grpc"
	httpinfra "coupon-service/internal/integration/http"
	adminhandler "coupon-service/internal/service_logic/handler/admin"
	grpchandler "coupon-service/internal/service_logic/handler/grpc"
	publichandler "coupon-service/internal/service_logic/handler/public"
	adminsvc "coupon-service/internal/service_logic/service/admin"
	calculatorsvc "coupon-service/internal/service_logic/service/calculator"
	claimsvc "coupon-service/internal/service_logic/service/claim"
	sagasvc "coupon-service/internal/service_logic/service/claim_saga"
	querysvc "coupon-service/internal/service_logic/service/query"
)

type Infra struct {
	DB         *sql.DB
	Redis      *redis.Client
	NATSConn   *nats.Conn
	NATSJS     nats.JetStreamContext
	HTTPServer *httpinfra.Server
	GRPCServer *grpcinfra.Server
}

type Repositories struct {
	Coupon      *repoadapter.CouponRepository
	Usage       *repoadapter.UsageRepository
	Outbox      *repoadapter.OutboxRepository
	Idempotency *repoadapter.IdempotencyRepository
	Saga        *repoadapter.SagaRepository
}

type Producers struct {
	Coupon *natsproducer.CouponEventPublisher
}

type Services struct {
	Calculator *calculatorsvc.CouponCalculatorService
	Query      *querysvc.CouponQueryService
	Claim      *claimsvc.CouponClaimService
	Admin      *adminsvc.CouponManagementService
	Saga       *sagasvc.Orchestrator
}

type Handlers struct {
	Public *publichandler.CouponPublicHandler
	Admin  *adminhandler.CouponAdminHandler
	GRPC   *grpchandler.CouponHandler
}

type Adapters struct {
	HTTP          *publichttp.CouponHTTPHandler
	GRPC          *grpcadapter.CouponGRPCHandler
	AdminConsumer *natsadmin.AdminCouponConsumer
	OutboxWorker  *outboxworker.Worker
	RateLimiter   *redisadapter.CouponClaimRateLimiter
	Router        http.Handler
	RegisterGRPC  func(*grpc.Server)
}
