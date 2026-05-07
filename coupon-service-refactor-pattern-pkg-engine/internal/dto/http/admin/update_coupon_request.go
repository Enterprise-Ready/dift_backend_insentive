package admin

import "time"

type UpdateCouponRequest struct {
	DiscountType  string    `json:"discount_type"`
	DiscountValue float64   `json:"discount_value"`
	MinOrder      float64   `json:"min_order"`
	MaxDiscount   float64   `json:"max_discount"`
	MaxUsage      int32     `json:"max_usage"`
	ValidFrom     time.Time `json:"valid_from"`
	ValidTo       time.Time `json:"valid_to"`
	Active        bool      `json:"active"`
}
