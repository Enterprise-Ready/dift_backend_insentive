package model

type Redeem struct {
	RedeemID    string // idempotency key
	UserID      string
	Point       int64
	RequestedAt int64
}
