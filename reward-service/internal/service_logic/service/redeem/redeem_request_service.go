package redeem

import (
	"context"
	"errors"
	"time"

	rewardport "reward-service/internal/interface/redeem"
	repoport "reward-service/internal/interface/repository"
	serviceport "reward-service/internal/interface/service/redeem"
	"reward-service/internal/model"

	"github.com/google/uuid"
)

type RedeemRequestService struct {
	repo       repoport.RedeemTransactionRepository
	redeemPort rewardport.RewardRedeemPort
}

func NewRedeemRequestService(
	repo repoport.RedeemTransactionRepository,
	redeemPort rewardport.RewardRedeemPort,
) *RedeemRequestService {
	return &RedeemRequestService{
		repo:       repo,
		redeemPort: redeemPort,
	}
}

func (s *RedeemRequestService) RequestRedeem(
	ctx context.Context,
	redeem model.Redeem,
) error {

	// 1️⃣ validate
	if redeem.UserID == "" {
		return errors.New("user_id is required")
	}
	if redeem.Point <= 0 {
		return errors.New("point must be greater than zero")
	}

	// 2️⃣ สร้าง RedeemID เป็น idempotency key
	redeem.RedeemID = uuid.NewString()
	redeem.RequestedAt = time.Now().Unix()

	// 3️⃣ กัน duplicate
	exists, err := s.repo.ExistsByRedeemID(ctx, redeem.RedeemID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// 4️⃣ save request (status = pending)
	if err := s.repo.SaveRequest(ctx, redeem); err != nil {
		return err
	}

	// 5️⃣ async ส่งไป user-reward-service
	return s.redeemPort.SendRedeemRequest(ctx, redeem)
}

var _ serviceport.RedeemRequestService = (*RedeemRequestService)(nil)
