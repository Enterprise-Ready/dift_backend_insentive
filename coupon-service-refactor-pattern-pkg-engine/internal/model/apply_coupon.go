package model

// ApplyCouponCommand
// Use-case input model
type ApplyCouponCommand struct {
	UserID     string
	CouponCode string
	OrderTotal float64
}

// ApplyCouponResult
// Use-case output model
type ApplyCouponResult struct {
	FinalTotal float64
	Discount   float64
	Valid      bool
	Message    string
}
