package postgres

import (
	"context"
	"database/sql"
	"errors"

	repo "coupon-service/internal/interface/repository"
	"coupon-service/internal/model"
)

var (
	ErrCouponNotFound = errors.New("coupon not found")
	ErrQuotaExceeded  = errors.New("coupon quota exceeded")
)

type CouponRepository struct {
	db *sql.DB
}

func NewCouponRepository(db *sql.DB) *CouponRepository {
	return &CouponRepository{db: db}
}

var _ repo.CouponRepository = (*CouponRepository)(nil)

func (r *CouponRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

func (r *CouponRepository) FindByCode(ctx context.Context, code string) (*model.Coupon, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
	code,
	discount_type,
	discount_value,
	min_order,
	max_discount,
	max_usage,
	used,
	valid_from,
	valid_to,
	active,
	created_at,
	updated_at
FROM coupons
WHERE code=$1
`, code)

	return scanCoupon(row)
}

func (r *CouponRepository) FindAllActive(ctx context.Context) ([]model.Coupon, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
	code,
	discount_type,
	discount_value,
	min_order,
	max_discount,
	max_usage,
	used,
	valid_from,
	valid_to,
	active,
	created_at,
	updated_at
FROM coupons
WHERE active=true
AND NOW() BETWEEN valid_from AND valid_to
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var coupons []model.Coupon
	for rows.Next() {
		var c model.Coupon
		err := rows.Scan(
			&c.Code,
			&c.DiscountType,
			&c.DiscountValue,
			&c.MinOrder,
			&c.MaxDiscount,
			&c.MaxUsage,
			&c.Used,
			&c.ValidFrom,
			&c.ValidTo,
			&c.Active,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		coupons = append(coupons, c)
	}

	return coupons, rows.Err()
}

func (r *CouponRepository) FindByCodeTx(
	ctx context.Context,
	tx *sql.Tx,
	code string,
) (*model.Coupon, error) {
	row := tx.QueryRowContext(ctx, `
SELECT
	code,
	discount_type,
	discount_value,
	min_order,
	max_discount,
	max_usage,
	used,
	valid_from,
	valid_to,
	active,
	created_at,
	updated_at
FROM coupons
WHERE code=$1
`, code)

	return scanCoupon(row)
}

func (r *CouponRepository) LockByCodeTx(
	ctx context.Context,
	tx *sql.Tx,
	code string,
) (*model.Coupon, error) {
	row := tx.QueryRowContext(ctx, `
SELECT
	code,
	discount_type,
	discount_value,
	min_order,
	max_discount,
	max_usage,
	used,
	valid_from,
	valid_to,
	active,
	created_at,
	updated_at
FROM coupons
WHERE code=$1
FOR UPDATE
`, code)

	return scanCoupon(row)
}

func (r *CouponRepository) SaveTx(
	ctx context.Context,
	tx *sql.Tx,
	coupon *model.Coupon,
) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO coupons (
	code,
	discount_type,
	discount_value,
	min_order,
	max_discount,
	max_usage,
	used,
	valid_from,
	valid_to,
	active,
	created_at,
	updated_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
ON CONFLICT (code)
DO UPDATE SET
	discount_type = EXCLUDED.discount_type,
	discount_value = EXCLUDED.discount_value,
	min_order = EXCLUDED.min_order,
	max_discount = EXCLUDED.max_discount,
	max_usage = EXCLUDED.max_usage,
	valid_from = EXCLUDED.valid_from,
	valid_to = EXCLUDED.valid_to,
	active = EXCLUDED.active,
	updated_at = NOW()
`,
		coupon.Code,
		coupon.DiscountType,
		coupon.DiscountValue,
		coupon.MinOrder,
		coupon.MaxDiscount,
		coupon.MaxUsage,
		coupon.Used,
		coupon.ValidFrom,
		coupon.ValidTo,
		coupon.Active,
	)

	return err
}

func (r *CouponRepository) IncreaseUsageTx(
	ctx context.Context,
	tx *sql.Tx,
	code string,
) error {
	res, err := tx.ExecContext(ctx, `
UPDATE coupons
SET used = used + 1
WHERE code=$1
AND active=true
AND used < max_usage
AND NOW() BETWEEN valid_from AND valid_to
`, code)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrQuotaExceeded
	}

	return nil
}

func (r *CouponRepository) DeactivateTx(
	ctx context.Context,
	tx *sql.Tx,
	code string,
) error {
	res, err := tx.ExecContext(ctx, `
UPDATE coupons
SET active=false,
updated_at=NOW()
WHERE code=$1
`, code)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrCouponNotFound
	}

	return nil
}

func (r *CouponRepository) Deactivate(
	ctx context.Context,
	code string,
) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE coupons
SET active=false,
updated_at=NOW()
WHERE code=$1
`, code)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrCouponNotFound
	}

	return nil
}

func (r *CouponRepository) Save(
	ctx context.Context,
	coupon *model.Coupon,
) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO coupons (
	code,
	discount_type,
	discount_value,
	min_order,
	max_discount,
	max_usage,
	used,
	valid_from,
	valid_to,
	active,
	created_at,
	updated_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
ON CONFLICT (code)
DO UPDATE SET
	discount_type = EXCLUDED.discount_type,
	discount_value = EXCLUDED.discount_value,
	min_order = EXCLUDED.min_order,
	max_discount = EXCLUDED.max_discount,
	max_usage = EXCLUDED.max_usage,
	valid_from = EXCLUDED.valid_from,
	valid_to = EXCLUDED.valid_to,
	active = EXCLUDED.active,
	updated_at = NOW()
`,
		coupon.Code,
		coupon.DiscountType,
		coupon.DiscountValue,
		coupon.MinOrder,
		coupon.MaxDiscount,
		coupon.MaxUsage,
		coupon.Used,
		coupon.ValidFrom,
		coupon.ValidTo,
		coupon.Active,
	)

	return err
}

func scanCoupon(row *sql.Row) (*model.Coupon, error) {
	var c model.Coupon
	err := row.Scan(
		&c.Code,
		&c.DiscountType,
		&c.DiscountValue,
		&c.MinOrder,
		&c.MaxDiscount,
		&c.MaxUsage,
		&c.Used,
		&c.ValidFrom,
		&c.ValidTo,
		&c.Active,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &c, nil
}
