package rule

import "reward-service/internal/model"

type DefaultRewardRule struct{}

func NewDefaultRewardRule() *DefaultRewardRule {
	return &DefaultRewardRule{}
}

func (r *DefaultRewardRule) CalculateFromTrip(e model.TripEvent) int {
	return int(e.Fare / 10)
}

func (r *DefaultRewardRule) CalculateFromOrder(e model.OrderEvent) int {
	return int(e.Amount / 20)
}
