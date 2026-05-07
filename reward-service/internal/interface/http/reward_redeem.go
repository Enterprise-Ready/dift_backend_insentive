package http

import (
	"context"

	"reward-service/internal/model"
)

type RewardRedeemHTTPPort interface {
	RequestRedeem(
		ctx context.Context,
		redeem model.Redeem,
	) error
}
