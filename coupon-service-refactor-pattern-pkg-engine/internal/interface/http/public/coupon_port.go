package public

import (
	"context"

	"coupon-service/internal/model"
)

type CouponPort interface {
	ListActiveCoupons(ctx context.Context) ([]model.Coupon, error)
	ClaimCoupon(
		ctx context.Context,
		userID string,
		couponCode string,
		idempotencyKey string,
	) error
}
