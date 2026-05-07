package repository

import (
	"context"
	"database/sql"

	"coupon-service/internal/model"
)

type CouponRepository interface {

	// =========================
	// Transaction
	// =========================

	BeginTx(ctx context.Context) (*sql.Tx, error)

	// =========================
	// Query (Non-Tx)
	// =========================

	FindByCode(
		ctx context.Context,
		code string,
	) (*model.Coupon, error)

	FindAllActive(
		ctx context.Context,
	) ([]model.Coupon, error)

	// =========================
	// Query (Tx)
	// =========================

	FindByCodeTx(
		ctx context.Context,
		tx *sql.Tx,
		code string,
	) (*model.Coupon, error)

	LockByCodeTx(
		ctx context.Context,
		tx *sql.Tx,
		code string,
	) (*model.Coupon, error)

	// =========================
	// Command (Tx Based)
	// =========================

	SaveTx(
		ctx context.Context,
		tx *sql.Tx,
		coupon *model.Coupon,
	) error

	DeactivateTx(
		ctx context.Context,
		tx *sql.Tx,
		code string,
	) error

	IncreaseUsageTx(
		ctx context.Context,
		tx *sql.Tx,
		code string,
	) error

	// =========================
	// Command (Non-Tx)
	// =========================

	Deactivate(
		ctx context.Context,
		code string,
	) error

	Save(
		ctx context.Context,
		coupon *model.Coupon,
	) error
}
