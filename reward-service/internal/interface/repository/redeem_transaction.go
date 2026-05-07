package repository

import (
	"context"

	"reward-service/internal/model"
)

type RedeemTransactionRepository interface {
	SaveRequest(ctx context.Context, redeem model.Redeem) error

	ExistsByRedeemID(ctx context.Context, redeemID string) (bool, error)

	UpdateResult(ctx context.Context, result model.RedeemResult) error

	GetByRedeemID(ctx context.Context, redeemID string) (*model.Redeem, error)
}
