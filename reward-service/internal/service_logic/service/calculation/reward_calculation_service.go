package calculation

import (
	"context"
	"errors"
	"time"

	port "reward-service/internal/interface/earn_reward"
	repoPort "reward-service/internal/interface/repository"
	rulePort "reward-service/internal/interface/service"
	serviceport "reward-service/internal/interface/service/calculation"
	"reward-service/internal/model"

	"github.com/google/uuid"
)

type RewardCalculationService struct {
	txRepo repoPort.EarnTransactionRepository
	rule   rulePort.RewardRule
	out    port.RewardEarnProducerPort
}

func NewRewardCalculationService(
	txRepo repoPort.EarnTransactionRepository,
	rule rulePort.RewardRule,
	out port.RewardEarnProducerPort,
) *RewardCalculationService {
	return &RewardCalculationService{
		txRepo: txRepo,
		rule:   rule,
		out:    out,
	}
}

var _ serviceport.RewardCalculationService = (*RewardCalculationService)(nil)

// =======================================================
// ✅ Implement HistoryConsumerPort (ต้องตรง signature เป๊ะ)
// =======================================================
func (s *RewardCalculationService) HandleEarn(
	ctx context.Context,
	earn model.Earn,
) error {

	if earn.RefID == "" {
		return errors.New("ref_id is required")
	}

	// ไม่ต้องคำนวณใหม่
	// ใช้ point ที่มากับ earn เลย
	earn.EarnID = uuid.NewString()
	earn.CreatedAt = time.Now().Unix()

	if err := s.txRepo.Save(ctx, earn); err != nil {
		return err
	}

	return s.out.SendEarn(ctx, earn)
}
