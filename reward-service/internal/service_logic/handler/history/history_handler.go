package history

import (
	"context"

	port "reward-service/internal/interface/earn_reward"
	serviceport "reward-service/internal/interface/service/calculation"
	"reward-service/internal/model"
)

type HistoryHandler struct {
	service serviceport.RewardCalculationService
}

func NewHistoryHandler(
	service serviceport.RewardCalculationService,
) *HistoryHandler {
	return &HistoryHandler{
		service: service,
	}
}

var _ port.HistoryConsumerPort = (*HistoryHandler)(nil)

func (h *HistoryHandler) HandleEarn(
	ctx context.Context,
	earn model.Earn,
) error {
	return h.service.HandleEarn(ctx, earn)
}
