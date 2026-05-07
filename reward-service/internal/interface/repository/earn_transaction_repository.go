package repository

import (
	"context"
	"reward-service/internal/model"
)

type EarnTransactionRepository interface {
	Save(ctx context.Context, earn model.Earn) error

	ExistsByEarnID(ctx context.Context, earnID string) (bool, error)

	ExistsByRefID(ctx context.Context, refID string) (bool, error)
}
