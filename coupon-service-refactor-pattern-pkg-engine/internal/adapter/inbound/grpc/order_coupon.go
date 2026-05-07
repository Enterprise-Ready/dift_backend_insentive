package grpc

import (
	"context"

	port "coupon-service/internal/interface/grpc"
	"coupon-service/internal/model"
	pb "coupon-service/proto/pb/order_service"
)

type CouponGRPCHandler struct {
	pb.UnimplementedCouponServiceServer
	handler port.CouponPort
}

func NewCouponGRPCHandler(handler port.CouponPort) *CouponGRPCHandler {
	return &CouponGRPCHandler{handler: handler}
}

func (h *CouponGRPCHandler) ApplyCoupon(
	ctx context.Context,
	req *pb.ApplyCouponRequest,
) (*pb.ApplyCouponResponse, error) {

	// 🔹 สร้าง command object ตาม interface ใหม่
	cmd := model.ApplyCouponCommand{
		UserID:     req.UserId,
		CouponCode: req.CouponCode,
		OrderTotal: req.OrderTotal,
	}

	// 🔹 เรียก service แบบใหม่
	result, err := h.handler.ApplyCoupon(ctx, cmd)
	if err != nil {
		return &pb.ApplyCouponResponse{
			Valid:   false,
			Message: err.Error(),
		}, nil
	}

	return &pb.ApplyCouponResponse{
		FinalTotal: result.FinalTotal,
		Discount:   result.Discount,
		Valid:      true,
	}, nil
}
