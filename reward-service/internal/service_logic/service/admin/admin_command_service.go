package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	adminport "reward-service/internal/interface/service/admin"
	redeemport "reward-service/internal/interface/service/redeem"
	"reward-service/internal/model"
)

var (
	ErrUnknownAction  = errors.New("unknown admin action")
	ErrInvalidPayload = errors.New("invalid admin payload")
)

type CommandService struct {
	redeemRequestService redeemport.RedeemRequestService
	redeemResultService  redeemport.RedeemResultService
}

func NewCommandService(
	redeemRequestService redeemport.RedeemRequestService,
	redeemResultService redeemport.RedeemResultService,
) *CommandService {
	return &CommandService{
		redeemRequestService: redeemRequestService,
		redeemResultService:  redeemResultService,
	}
}

var _ adminport.CommandService = (*CommandService)(nil)

func (s *CommandService) Execute(ctx context.Context, cmd adminport.Command) error {
	switch strings.ToLower(strings.TrimSpace(cmd.Action)) {
	case "redeem.request":
		userID := payloadString(cmd.Payload, "user_id")
		point, ok := payloadInt64(cmd.Payload, "point")
		if userID == "" || !ok {
			return ErrInvalidPayload
		}
		return s.redeemRequestService.RequestRedeem(ctx, model.Redeem{
			UserID: userID,
			Point:  point,
		})
	case "redeem.result":
		redeemID := payloadString(cmd.Payload, "redeem_id")
		userID := payloadString(cmd.Payload, "user_id")
		point, ok := payloadInt64(cmd.Payload, "point")
		success, okSuccess := payloadBool(cmd.Payload, "success")
		if redeemID == "" || userID == "" || !ok || !okSuccess {
			return ErrInvalidPayload
		}
		return s.redeemResultService.HandleResult(ctx, model.RedeemResult{
			RedeemID: redeemID,
			UserID:   userID,
			Point:    point,
			Success:  success,
			Reason:   payloadString(cmd.Payload, "reason"),
		})
	default:
		return fmt.Errorf("%w: %s", ErrUnknownAction, cmd.Action)
	}
}

func payloadString(payload map[string]any, key string) string {
	v, ok := payload[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func payloadInt64(payload map[string]any, key string) (int64, bool) {
	v, ok := payload[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float32:
		return int64(n), true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

func payloadBool(payload map[string]any, key string) (bool, bool) {
	v, ok := payload[key]
	if !ok || v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}
