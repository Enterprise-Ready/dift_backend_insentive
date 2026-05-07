package dto

import (
	"reward-service/internal/model"
	pb "reward-service/proto/pb/event"
)

func TripEventToEarn(e *pb.TripEvent) model.Earn {
	return model.Earn{
		UserID: e.UserId,
		Point:  e.Point,
		Source: "trip",
		RefID:  e.TripId,
	}
}
