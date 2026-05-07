package query

import (
	"context"

	"coupon-service/internal/model"
)

type CouponQueryService interface {
	ListActiveCoupons(ctx context.Context) ([]model.Coupon, error)
}
