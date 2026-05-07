package public

import (
	"context"

	httpport "coupon-service/internal/interface/http/public"
	claimport "coupon-service/internal/interface/service/claim"
	queryport "coupon-service/internal/interface/service/query"
	"coupon-service/internal/model"
)

type CouponPublicHandler struct {
	queryService queryport.CouponQueryService
	claimService claimport.CouponClaimService
}

func NewCouponPublicHandler(
	queryService queryport.CouponQueryService,
	claimService claimport.CouponClaimService,
) *CouponPublicHandler {
	return &CouponPublicHandler{
		queryService: queryService,
		claimService: claimService,
	}
}

var _ httpport.CouponPort = (*CouponPublicHandler)(nil)

func (h *CouponPublicHandler) ListActiveCoupons(
	ctx context.Context,
) ([]model.Coupon, error) {
	return h.queryService.ListActiveCoupons(ctx)
}

func (h *CouponPublicHandler) ClaimCoupon(
	ctx context.Context,
	userID string,
	couponCode string,
	idempotencyKey string,
) error {
	return h.claimService.Claim(ctx, userID, couponCode, idempotencyKey)
}
