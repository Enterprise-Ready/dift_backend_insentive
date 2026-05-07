package redeem

import (
	"context"

	eventport "reward-service/internal/interface/event_consumer"
	serviceport "reward-service/internal/interface/service/redeem"
	"reward-service/internal/model"
)

type RedeemResultHandler struct {
	service serviceport.RedeemResultService
}

func NewRedeemResultHandler(
	service serviceport.RedeemResultService,
) *RedeemResultHandler {
	return &RedeemResultHandler{
		service: service,
	}
}

var _ eventport.RedeemResultConsumer = (*RedeemResultHandler)(nil)

func (h *RedeemResultHandler) HandleResult(
	ctx context.Context,
	result model.RedeemResult,
) error {
	return h.service.HandleResult(ctx, result)
}
