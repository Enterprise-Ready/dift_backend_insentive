package repository

import (
	"context"
	"database/sql"

	"coupon-service/internal/model"
)

type OutboxRepository interface {
	InsertTx(
		ctx context.Context,
		tx *sql.Tx,
		event model.OutboxInsert,
	) error

	GetPending(
		ctx context.Context,
		tx *sql.Tx,
		limit int,
	) ([]model.OutboxEvent, error)

	MarkSent(
		ctx context.Context,
		tx *sql.Tx,
		id int64,
	) error
}
