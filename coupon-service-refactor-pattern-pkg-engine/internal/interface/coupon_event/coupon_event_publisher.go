package event

import (
	"context"
	"coupon-service/internal/model"
)

type CouponEventPublisher interface {
	Publish(ctx context.Context, e model.CouponEvent) error
}
