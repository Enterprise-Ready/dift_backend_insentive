package admin

import (
	"context"

	adminport "coupon-service/internal/interface/admin"
	serviceport "coupon-service/internal/interface/service/admin"
	"coupon-service/internal/model"
)

type CouponAdminHandler struct {
	service serviceport.CouponAdminService
}

func NewCouponAdminHandler(service serviceport.CouponAdminService) *CouponAdminHandler {
	return &CouponAdminHandler{
		service: service,
	}
}

var _ adminport.CouponCommandPort = (*CouponAdminHandler)(nil)

func (h *CouponAdminHandler) CreateCoupon(
	ctx context.Context,
	coupon model.Coupon,
) error {
	return h.service.CreateCoupon(ctx, coupon)
}

func (h *CouponAdminHandler) UpdateCoupon(
	ctx context.Context,
	coupon model.Coupon,
) error {
	return h.service.UpdateCoupon(ctx, coupon)
}

func (h *CouponAdminHandler) DeactivateCoupon(
	ctx context.Context,
	code string,
) error {
	return h.service.DeactivateCoupon(ctx, code)
}
