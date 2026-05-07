package app

import (
	"context"

	"coupon-service/config"
)

func Bootstrap(ctx context.Context, cfg config.Config, features config.FeatureFlags, logger Logger) (*App, error) {
	infra, err := wireInfra(ctx, cfg, features, logger)
	if err != nil {
		return nil, err
	}
	repos := wireRepositories(infra.DB)
	producers := wireProducers(infra.NATSJS, cfg, features)
	services := wireServices(repos, features)
	handlers := wireHandlers(services)
	adapters, err := wireAdapters(ctx, cfg, features, infra, repos, producers, services, handlers, logger)
	if err != nil {
		return nil, err
	}
	infra.HTTPServer = wireHTTPServer(cfg, features, adapters.Router)
	infra.GRPCServer = wireGRPCServer(cfg, features, adapters.RegisterGRPC)
	return &App{Infra: infra, Adapters: adapters, logger: logger}, nil
}
