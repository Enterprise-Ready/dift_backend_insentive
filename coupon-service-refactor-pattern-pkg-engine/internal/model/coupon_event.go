// internal/model/coupon_event.go
package model

import "time"

type CouponEventType string

const (
	CouponCreated        CouponEventType = "CREATED"
	CouponUpdated        CouponEventType = "UPDATED"
	CouponDeactivated    CouponEventType = "DEACTIVATED"
	CouponClaimed        CouponEventType = "CLAIMED"
	CouponUsageIncreased CouponEventType = "USAGE_INCREASED"
)

type CouponEvent struct {
	Type          CouponEventType
	UserID        string
	CouponCode    string
	DiscountType  string
	DiscountValue float64
	MinOrder      float64
	MaxDiscount   float64
	MaxUsage      int32
	ValidFrom     time.Time
	ValidTo       time.Time
	OccurredAt    time.Time
	Active        bool //เพิ่ม
}
