package admin

import (
	"context"

	"coupon-service/internal/model"
)

type CouponAdminService interface {
	CreateCoupon(ctx context.Context, coupon model.Coupon) error
	UpdateCoupon(ctx context.Context, coupon model.Coupon) error
	DeactivateCoupon(ctx context.Context, code string) error
}
