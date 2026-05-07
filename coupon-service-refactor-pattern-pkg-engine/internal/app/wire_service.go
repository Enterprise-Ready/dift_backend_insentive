package app

import (
	"coupon-service/config"
	adminsvc "coupon-service/internal/service_logic/service/admin"
	calculatorsvc "coupon-service/internal/service_logic/service/calculator"
	claimsvc "coupon-service/internal/service_logic/service/claim"
	querysvc "coupon-service/internal/service_logic/service/query"
)

func wireServices(repos Repositories, features config.FeatureFlags) Services {
	var s Services
	if features.EnableCalculator && repos.Coupon != nil {
		s.Calculator = calculatorsvc.NewCouponCalculatorService(repos.Coupon)
	}
	if features.EnableClaim && repos.Coupon != nil {
		s.Query = querysvc.NewCouponQueryService(repos.Coupon)
		s.Claim = claimsvc.NewCouponClaimService(repos.Coupon, repos.Usage, repos.Outbox, repos.Idempotency)
	}
	if features.EnableAdmin && repos.Coupon != nil {
		s.Admin = adminsvc.NewCouponManagementService(repos.Coupon, repos.Outbox)
	}
	return s
}
