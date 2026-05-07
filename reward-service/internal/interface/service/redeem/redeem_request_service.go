package redeem

import (
	"context"

	"reward-service/internal/model"
)

type RedeemRequestService interface {
	RequestRedeem(ctx context.Context, redeem model.Redeem) error
}
