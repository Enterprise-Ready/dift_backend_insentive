package model

type RedeemResult struct {
	RedeemID string
	UserID   string
	Point    int64

	Success bool
	Reason  string

	ProcessedAt int64
}
