package model

type Reward struct {
	UserID string

	// cached / projection เท่านั้น
	Balance int64

	UpdatedAt int64
}
