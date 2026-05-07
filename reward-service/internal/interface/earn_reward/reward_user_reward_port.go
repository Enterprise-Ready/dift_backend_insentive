package port

import (
	"context"

	"reward-service/internal/model"
)

type RewardEarnProducerPort interface {
	SendEarn(ctx context.Context, earn model.Earn) error
}
