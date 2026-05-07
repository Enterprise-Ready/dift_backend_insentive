package model

import "time"

type Coupon struct {
	Code          string
	UserID        string
	DiscountType  DiscountType
	DiscountValue float64
	MinOrder      float64
	MaxDiscount   float64
	MaxUsage      int32
	Used          int32
	ValidFrom     time.Time
	ValidTo       time.Time
	Active        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
