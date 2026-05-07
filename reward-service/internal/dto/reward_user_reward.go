package dto

import (
	"reward-service/internal/model"
	earnpb "reward-service/proto/pb/earn"
)

func EarnToRewardEarnPB(e model.Earn) *earnpb.RewardEarn {
	return &earnpb.RewardEarn{
		EarnId: e.EarnID, // จาก earn_id
		UserId: e.UserID,
		Point:  e.Point,
		Source: e.Source,
		RefId:  e.RefID,
	}
}
