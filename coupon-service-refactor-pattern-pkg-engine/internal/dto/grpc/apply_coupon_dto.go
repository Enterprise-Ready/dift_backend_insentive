package dto

import (
	"coupon-service/internal/model"
	pb "coupon-service/proto/pb/order_service"
)

//
// ======================
// gRPC → Domain
// ======================
//

func ApplyCouponCommandFromPB(
	req *pb.ApplyCouponRequest,
) model.ApplyCouponCommand {
	return model.ApplyCouponCommand{
		UserID:     req.UserId,
		CouponCode: req.CouponCode,
		OrderTotal: req.OrderTotal,
	}
}

//
// ======================
// Domain → gRPC
// ======================
//

func ApplyCouponResultToPB(
	result model.ApplyCouponResult,
) *pb.ApplyCouponResponse {
	return &pb.ApplyCouponResponse{
		FinalTotal: result.FinalTotal,
		Discount:   result.Discount,
		Valid:      result.Valid,
		Message:    result.Message,
	}
}
