package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	repo "coupon-service/internal/interface/repository"
	"coupon-service/internal/model"
)

var (
	ErrIdempotencyConflict = errors.New("idempotency conflict")
)

type IdempotencyRepository struct {
	db *sql.DB
}

func NewIdempotencyRepository(db *sql.DB) *IdempotencyRepository {
	return &IdempotencyRepository{db: db}
}

var _ repo.IdempotencyRepository = (*IdempotencyRepository)(nil)

//////////////////////////////////////////////////
// InsertOrGetTx
//////////////////////////////////////////////////

func (r *IdempotencyRepository) InsertOrGetTx(
	ctx context.Context,
	tx *sql.Tx,
	key string,
	userID string,
	initialStatus model.IdempotencyStatus,
) (*model.IdempotencyRecord, error) {

	// พยายาม insert ก่อน
	_, err := tx.ExecContext(ctx, `
		INSERT INTO idempotency_keys (
			idempotency_key,
			user_id,
			status,
			created_at
		)
		VALUES ($1, $2, $3, NOW())
	`, key, userID, initialStatus)

	if err == nil {
		// insert สำเร็จ → เป็น request ใหม่
		return &model.IdempotencyRecord{
			Key:       key,
			UserID:    userID,
			Status:    initialStatus,
			CreatedAt: time.Now(),
		}, nil
	}

	// ถ้า insert ไม่สำเร็จ → แปลว่า duplicate
	row := tx.QueryRowContext(ctx, `
		SELECT status, created_at
		FROM idempotency_keys
		WHERE idempotency_key = $1
		AND user_id = $2
	`, key, userID)

	var status model.IdempotencyStatus
	var createdAt time.Time

	if err := row.Scan(&status, &createdAt); err != nil {
		return nil, err
	}

	return &model.IdempotencyRecord{
		Key:       key,
		UserID:    userID,
		Status:    status,
		CreatedAt: createdAt,
	}, nil
}

//////////////////////////////////////////////////
// MarkSuccessTx
//////////////////////////////////////////////////

func (r *IdempotencyRepository) MarkSuccessTx(
	ctx context.Context,
	tx *sql.Tx,
	key string,
) error {

	_, err := tx.ExecContext(ctx, `
		UPDATE idempotency_keys
		SET status = $1
		WHERE idempotency_key = $2
	`, model.IdempotencySuccess, key)

	return err
}

//////////////////////////////////////////////////
// MarkFailedTx
//////////////////////////////////////////////////

func (r *IdempotencyRepository) MarkFailedTx(
	ctx context.Context,
	tx *sql.Tx,
	key string,
) error {

	_, err := tx.ExecContext(ctx, `
		UPDATE idempotency_keys
		SET status = $1
		WHERE idempotency_key = $2
	`, model.IdempotencyFailed, key)

	return err
}
