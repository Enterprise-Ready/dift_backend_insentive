package calculation

import (
	"context"

	"reward-service/internal/model"
)

type RewardCalculationService interface {
	HandleEarn(ctx context.Context, earn model.Earn) error
}
