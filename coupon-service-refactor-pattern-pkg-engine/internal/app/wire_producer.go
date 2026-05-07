package app

import (
	"coupon-service/config"
	natsproducer "coupon-service/internal/adapter/outbound/messaging/nats"
	"github.com/nats-io/nats.go"
)

func wireProducers(js nats.JetStreamContext, cfg config.Config, features config.FeatureFlags) Producers {
	if !features.EnableNATSProducer || js == nil {
		return Producers{}
	}
	return Producers{Coupon: natsproducer.NewCouponEventPublisher(js, cfg.NATS.Subject)}
}
