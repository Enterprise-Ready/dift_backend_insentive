package redeem

import (
	"context"
	"log"
	"time"

	repo "reward-service/internal/interface/repository"
	serviceport "reward-service/internal/interface/service/redeem"
	"reward-service/internal/model"
)

type RedeemResultService struct {
	repo repo.RedeemTransactionRepository
}

var _ serviceport.RedeemResultService = (*RedeemResultService)(nil)

func NewRedeemResultService(
	repo repo.RedeemTransactionRepository,
) *RedeemResultService {
	return &RedeemResultService{repo: repo}
}

func (s *RedeemResultService) HandleResult(
	ctx context.Context,
	result model.RedeemResult,
) error {

	result.ProcessedAt = time.Now().Unix()

	// update status
	if err := s.repo.UpdateResult(ctx, result); err != nil {
		return err
	}

	if !result.Success {
		log.Printf(
			"redeem failed user=%s reason=%s",
			result.UserID,
			result.Reason,
		)
	}

	return nil
}
