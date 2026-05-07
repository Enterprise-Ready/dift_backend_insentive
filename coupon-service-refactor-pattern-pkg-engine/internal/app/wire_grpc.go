package app

import (
	"coupon-service/config"
	grpcinfra "coupon-service/internal/integration/grpc"
	"google.golang.org/grpc"
	"strings"
)

func wireGRPCServer(cfg config.Config, features config.FeatureFlags, register func(*grpc.Server)) *grpcinfra.Server {
	if !features.EnableGRPC || register == nil {
		return nil
	}
	server := grpcinfra.NewServer(cfg.GRPC.Port)
	server.Register(register)
	return server
}

func normalizeAddress(address string) string {
	if strings.HasPrefix(address, ":") {
		return address
	}
	return ":" + address
}
