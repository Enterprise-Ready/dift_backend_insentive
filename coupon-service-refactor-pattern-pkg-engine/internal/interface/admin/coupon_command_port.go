package admin

import (
	"context"

	"coupon-service/internal/model"
)

// CouponCommandPort is the admin command contract used by event-driven adapters (NATS).
type CouponCommandPort interface {
	CreateCoupon(ctx context.Context, coupon model.Coupon) error
	UpdateCoupon(ctx context.Context, coupon model.Coupon) error
	DeactivateCoupon(ctx context.Context, code string) error
}
