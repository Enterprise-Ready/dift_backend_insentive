package dto

import (
	"reward-service/internal/model"
	pb "reward-service/proto/pb/event"
)

func OrderEventToEarn(e *pb.OrderEvent) model.Earn {
	return model.Earn{
		UserID: e.UserId,
		Point:  e.Point,
		Source: "order",
		RefID:  e.OrderId,
	}
}
