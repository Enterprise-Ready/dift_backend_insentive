package grpc

import (
	"context"

	"coupon-service/internal/model"
)

type CouponPort interface {
	ApplyCoupon(
		ctx context.Context,
		cmd model.ApplyCouponCommand,
	) (model.ApplyCouponResult, error)
}
