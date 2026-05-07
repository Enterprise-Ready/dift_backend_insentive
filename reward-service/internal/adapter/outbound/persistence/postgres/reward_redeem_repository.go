package postgres

import (
	"context"
	"database/sql"
	"errors"

	"reward-service/internal/model"

	"github.com/lib/pq"
)

type RewardRedeemRepository struct {
	db *sql.DB
}

func NewRewardRedeemRepository(db *sql.DB) *RewardRedeemRepository {
	return &RewardRedeemRepository{
		db: db,
	}
}

// =====================
// SaveRequest
// =====================
// Insert redeem request (idempotent)
func (r *RewardRedeemRepository) SaveRequest(
	ctx context.Context,
	rm model.Redeem,
) error {

	_, err := r.db.ExecContext(ctx, `
	INSERT INTO reward_redeem_requests
	(
		redeem_id,
		user_id,
		point,
		requested_at
	)
	VALUES ($1,$2,$3,$4)
	`,
		rm.RedeemID,
		rm.UserID,
		rm.Point,
		rm.RequestedAt,
	)

	if err != nil {
		// Unique constraint violation (idempotent safe)
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil
		}
		return err
	}

	return nil
}

// =====================
// ExistsByRedeemID
// =====================
func (r *RewardRedeemRepository) ExistsByRedeemID(
	ctx context.Context,
	redeemID string,
) (bool, error) {

	var exists bool

	err := r.db.QueryRowContext(ctx, `
	SELECT EXISTS (
		SELECT 1 FROM reward_redeem_requests WHERE redeem_id = $1
	)
	`, redeemID).Scan(&exists)

	return exists, err
}

// =====================
// GetByRedeemID
// =====================
func (r *RewardRedeemRepository) GetByRedeemID(
	ctx context.Context,
	redeemID string,
) (*model.Redeem, error) {

	var redeem model.Redeem

	err := r.db.QueryRowContext(ctx, `
	SELECT redeem_id, user_id, point, requested_at
	FROM reward_redeem_requests
	WHERE redeem_id = $1
	`, redeemID).Scan(
		&redeem.RedeemID,
		&redeem.UserID,
		&redeem.Point,
		&redeem.RequestedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &redeem, nil
}

// =====================
// UpdateResult
// =====================
func (r *RewardRedeemRepository) UpdateResult(
	ctx context.Context,
	res model.RedeemResult,
) error {

	result, err := r.db.ExecContext(ctx, `
	UPDATE reward_redeem_requests
	SET
		success = $1,
		reason = $2,
		processed_at = $3
	WHERE redeem_id = $4
	`,
		res.Success,
		res.Reason,
		res.ProcessedAt,
		res.RedeemID,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errors.New("redeem_id not found")
	}

	return nil
}
