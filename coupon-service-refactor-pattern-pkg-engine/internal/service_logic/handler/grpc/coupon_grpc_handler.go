package grpc

import (
	"context"

	grpcport "coupon-service/internal/interface/grpc"
	serviceport "coupon-service/internal/interface/service/calculator"
	"coupon-service/internal/model"
)

type CouponHandler struct {
	service serviceport.CouponCalculatorService
}

func NewCouponHandler(service serviceport.CouponCalculatorService) *CouponHandler {
	return &CouponHandler{
		service: service,
	}
}

var _ grpcport.CouponPort = (*CouponHandler)(nil)

func (h *CouponHandler) ApplyCoupon(
	ctx context.Context,
	cmd model.ApplyCouponCommand,
) (model.ApplyCouponResult, error) {
	return h.service.ApplyCoupon(ctx, cmd)
}
