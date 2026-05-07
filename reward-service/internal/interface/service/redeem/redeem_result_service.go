package redeem

import (
	"context"

	"reward-service/internal/model"
)

type RedeemResultService interface {
	HandleResult(ctx context.Context, result model.RedeemResult) error
}
