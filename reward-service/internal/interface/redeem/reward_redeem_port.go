package reward

import (
	"context"
	"reward-service/internal/model"
)

type RewardRedeemPort interface {
	SendRedeemRequest(ctx context.Context, redeem model.Redeem) error
}
