package dto

import "coupon-service/internal/model"

type CouponResponse struct {
	Code          string  `json:"code"`
	DiscountType  string  `json:"discount_type"`
	DiscountValue float64 `json:"discount_value"`
	MinOrder      float64 `json:"min_order"`
	MaxDiscount   float64 `json:"max_discount"`
	ValidFrom     string  `json:"valid_from"`
	ValidTo       string  `json:"valid_to"`
}

func FromCoupon(c model.Coupon) CouponResponse {

	return CouponResponse{
		Code:          c.Code,
		DiscountType:  string(c.DiscountType),
		DiscountValue: c.DiscountValue,
		MinOrder:      c.MinOrder,
		MaxDiscount:   c.MaxDiscount,
		ValidFrom:     c.ValidFrom.Format("2006-01-02"),
		ValidTo:       c.ValidTo.Format("2006-01-02"),
	}

}
