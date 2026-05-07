package redeem

import (
	"context"

	httpport "reward-service/internal/interface/http"
	serviceport "reward-service/internal/interface/service/redeem"
	"reward-service/internal/model"
)

type RedeemRequestHandler struct {
	service serviceport.RedeemRequestService
}

func NewRedeemRequestHandler(
	service serviceport.RedeemRequestService,
) *RedeemRequestHandler {
	return &RedeemRequestHandler{
		service: service,
	}
}

var _ httpport.RewardRedeemHTTPPort = (*RedeemRequestHandler)(nil)

func (h *RedeemRequestHandler) RequestRedeem(
	ctx context.Context,
	redeem model.Redeem,
) error {
	return h.service.RequestRedeem(ctx, redeem)
}
