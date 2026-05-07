package app

import (
	"context"
	"time"

	"coupon-service/config"
	natsadmin "coupon-service/internal/adapter/inbound/event/natsadmin"
	grpcadapter "coupon-service/internal/adapter/inbound/grpc"
	publichttp "coupon-service/internal/adapter/inbound/http/claimquery"
	outboxworker "coupon-service/internal/adapter/inbound/worker"
	redisadapter "coupon-service/internal/adapter/outbound/cache/redis"
	sagasvc "coupon-service/internal/service_logic/service/claim_saga"
	pb "coupon-service/proto/pb/order_service"
	"google.golang.org/grpc"
)

func wireAdapters(ctx context.Context, cfg config.Config, features config.FeatureFlags, infra Infra, repos Repositories, producers Producers, services Services, handlers Handlers, logger Logger) (Adapters, error) {
	_ = logger
	var a Adapters
	if handlers.Public != nil {
		a.HTTP = publichttp.NewCouponHTTPHandler(handlers.Public)
	}
	if handlers.GRPC != nil {
		grpcHandler := grpcadapter.NewCouponGRPCHandler(handlers.GRPC)
		a.GRPC = grpcHandler
		a.RegisterGRPC = func(s *grpc.Server) { pb.RegisterCouponServiceServer(s, grpcHandler) }
	}
	if features.EnableNATSConsumer && infra.NATSJS != nil && handlers.Admin != nil {
		a.AdminConsumer = natsadmin.NewAdminCouponConsumer(infra.NATSJS, cfg.NATS.AdminStream, cfg.NATS.AdminSubject, cfg.NATS.AdminDurable, handlers.Admin)
	}
	if features.EnableDatabase && repos.Outbox != nil && producers.Coupon != nil {
		a.OutboxWorker = outboxworker.NewWorker(infra.DB, repos.Outbox, producers.Coupon, 2*time.Second)
	}
	if features.EnableClaim && infra.Redis != nil && repos.Saga != nil && producers.Coupon != nil {
		a.RateLimiter = redisadapter.NewCouponClaimRateLimiter(infra.Redis, redisadapter.DefaultWindow, redisadapter.DefaultMaxRequests)
		claimStep := sagasvc.NewClaimStep(repos.Coupon, repos.Usage, repos.Idempotency)
		reserveStep := sagasvc.NewReserveStep(producers.Coupon)
		confirmStep := sagasvc.NewConfirmStep(repos.Coupon, repos.Outbox)
		orchestrator := sagasvc.NewOrchestrator(repos.Saga, a.RateLimiter, []sagasvc.SagaStep{claimStep, reserveStep, confirmStep})
		recoveryWorker := sagasvc.NewRecoveryWorker(repos.Saga, orchestrator, 30*time.Second, 2*time.Minute)
		go recoveryWorker.Start(ctx)
		services.Saga = orchestrator
	}
	a.Router = wireHTTPRouter(cfg, features, infra.DB, a.HTTP)
	return a, nil
}
