package repository

import (
	"context"
	"database/sql"

	"coupon-service/internal/model"
)

type IdempotencyRepository interface {
	InsertOrGetTx(
		ctx context.Context,
		tx *sql.Tx,
		key string,
		userID string,
		initialStatus model.IdempotencyStatus,
	) (*model.IdempotencyRecord, error)

	MarkSuccessTx(
		ctx context.Context,
		tx *sql.Tx,
		key string,
	) error

	MarkFailedTx(
		ctx context.Context,
		tx *sql.Tx,
		key string,
	) error
}
