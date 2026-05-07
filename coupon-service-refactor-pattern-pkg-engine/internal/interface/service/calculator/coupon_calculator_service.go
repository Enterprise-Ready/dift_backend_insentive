package calculator

import (
	"context"

	"coupon-service/internal/model"
)

type CouponCalculatorService interface {
	ApplyCoupon(
		ctx context.Context,
		cmd model.ApplyCouponCommand,
	) (model.ApplyCouponResult, error)
}
