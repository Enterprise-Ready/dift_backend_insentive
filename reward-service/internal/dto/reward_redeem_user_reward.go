package dto

import (
	"reward-service/internal/model"
	redeempb "reward-service/proto/pb/redeem"
)

func RedeemToRequestPB(r model.Redeem) *redeempb.RedeemRequest {
	return &redeempb.RedeemRequest{
		RedeemId: r.RedeemID,
		UserId:   r.UserID,
		Point:    r.Point,
	}
}

func RedeemResultFromPB(r *redeempb.RedeemResult) model.RedeemResult {
	return model.RedeemResult{
		RedeemID: r.RedeemId,
		UserID:   r.UserId,
		Point:    r.Point,
		Success:  r.Success,
		Reason:   r.Reason,
	}
}
