package event

import (
	"context"
	"coupon-service/internal/model"
)

type CouponEventConsumer interface {
	Handle(ctx context.Context, event model.CouponEvent) error
}
