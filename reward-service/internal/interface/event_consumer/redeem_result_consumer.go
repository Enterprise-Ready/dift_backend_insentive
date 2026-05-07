package event_consumer

import (
	"context"

	"reward-service/internal/model"
)

type RedeemResultConsumer interface {
	HandleResult(ctx context.Context, result model.RedeemResult) error
}
