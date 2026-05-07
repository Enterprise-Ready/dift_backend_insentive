package repository

import (
	"context"

	"reward-service/internal/model"
)

type RewardQueryRepository interface {
	GetByUserID(
		ctx context.Context,
		userID string,
	) (*model.Reward, error)
}
