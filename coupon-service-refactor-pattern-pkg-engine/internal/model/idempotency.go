package model

import "time"

type IdempotencyStatus string

const (
	IdempotencyProcessing IdempotencyStatus = "PROCESSING"
	IdempotencySuccess    IdempotencyStatus = "SUCCESS"
	IdempotencyFailed     IdempotencyStatus = "FAILED"
)

type IdempotencyRecord struct {
	Key       string
	UserID    string
	Status    IdempotencyStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}
