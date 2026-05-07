package claim

import "context"

type CouponClaimService interface {
	Claim(
		ctx context.Context,
		userID string,
		couponCode string,
		idempotencyKey string,
	) error
}
