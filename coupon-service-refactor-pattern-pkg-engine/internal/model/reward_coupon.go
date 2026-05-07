package model

import "time"

// คำสั่งจาก Reward Service
type RewardCouponCommand struct {
	CommandID  string
	UserID     string
	Point      int64
	CouponType string

	DiscountAmount int64
	MinPrice       int64
	MaxDiscount    int64

	ExpiredAt time.Time
	CreatedAt time.Time
}

// ผลลัพธ์ที่สร้างเสร็จแล้ว
type UserCoupon struct {
	UserCouponID string
	UserID       string
	CouponCode   string

	DiscountAmount int64
	MinPrice       int64
	MaxDiscount    int64

	ExpiredAt time.Time
	CreatedAt time.Time
}
