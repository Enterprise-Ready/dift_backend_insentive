package postgres

import (
	"context"
	"database/sql"

	"reward-service/internal/model"
)

type EarnRepository struct {
	db *sql.DB
}

func NewEarnRepository(db *sql.DB) *EarnRepository {
	return &EarnRepository{
		db: db,
	}
}

// =====================
// Save
// =====================
func (r *EarnRepository) Save(
	ctx context.Context,
	earn model.Earn,
) error {

	_, err := r.db.ExecContext(ctx, `
	INSERT INTO reward_earn_transactions
	(
		earn_id,
		user_id,
		ref_id,
		point,
		source,
		created_at
	)
	VALUES ($1,$2,$3,$4,$5,$6)
	`,
		earn.EarnID,
		earn.UserID,
		earn.RefID,
		earn.Point,
		earn.Source,
		earn.CreatedAt,
	)

	return err
}

// =====================
// ExistsByEarnID
// =====================
func (r *EarnRepository) ExistsByEarnID(
	ctx context.Context,
	earnID string,
) (bool, error) {

	var exists bool

	err := r.db.QueryRowContext(ctx, `
	SELECT EXISTS (
		SELECT 1 FROM reward_earn_transactions WHERE earn_id = $1
	)
	`, earnID).Scan(&exists)

	return exists, err
}

// =====================
// ExistsByRefID  ⭐ ต้องเพิ่มตัวนี้ด้วย
// =====================
func (r *EarnRepository) ExistsByRefID(
	ctx context.Context,
	refID string,
) (bool, error) {

	var exists bool

	err := r.db.QueryRowContext(ctx, `
	SELECT EXISTS (
		SELECT 1 FROM reward_earn_transactions WHERE ref_id = $1
	)
	`, refID).Scan(&exists)

	return exists, err
}
