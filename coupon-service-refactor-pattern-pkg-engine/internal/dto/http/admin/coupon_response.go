package admin

import "coupon-service/internal/model"

type CouponResponse struct {
	Code          string  `json:"code"`
	DiscountType  string  `json:"discount_type"`
	DiscountValue float64 `json:"discount_value"`
	Active        bool    `json:"active"`
}

func FromModel(c model.Coupon) CouponResponse {
	return CouponResponse{
		Code: c.Code,
		//DiscountType:  c.DiscountType,
		DiscountValue: c.DiscountValue,
		Active:        c.Active,
	}
}
