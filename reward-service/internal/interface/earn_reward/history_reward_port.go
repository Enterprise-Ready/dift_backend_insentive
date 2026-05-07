package port

import (
	"context"
	"reward-service/internal/model"
)

type HistoryConsumerPort interface {
	HandleEarn(ctx context.Context, earn model.Earn) error
}

//HistoryConsumerPort
