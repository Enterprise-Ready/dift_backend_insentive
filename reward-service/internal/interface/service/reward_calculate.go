// internal/interface/rule/reward_rule.go
package rule

import "reward-service/internal/model"

type RewardRule interface {
	CalculateFromTrip(event model.TripEvent) int
	CalculateFromOrder(event model.OrderEvent) int
}
