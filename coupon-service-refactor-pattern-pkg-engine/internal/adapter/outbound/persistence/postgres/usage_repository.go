package postgres

import (
	"context"
	"database/sql"

	repo "coupon-service/internal/interface/repository"
)

type UsageRepository struct {
	db *sql.DB
}

func NewUsageRepository(db *sql.DB) *UsageRepository {
	return &UsageRepository{db: db}
}

var _ repo.UsageRepository = (*UsageRepository)(nil)

func (r *UsageRepository) InsertTx(
	ctx context.Context,
	tx *sql.Tx,
	couponCode string,
	userID string,
	orderID string,
) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO coupon_usage_history (
			coupon_code,
			user_id,
			order_id
		)
		VALUES ($1,$2,$3)
	`, couponCode, userID, orderID)
	if err != nil {
		return repo.ErrDuplicateUsage
	}

	return nil
}
