package model

type Earn struct {
	EarnID string // idempotency key
	UserID string
	Point  int64

	Source string // trip, order, campaign
	RefID  string // order_id, trip_id

	CreatedAt int64
}

//Source    EarnSource
