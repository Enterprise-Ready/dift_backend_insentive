package repository

import (
	"context"
	"database/sql"
	"errors"
)

var ErrDuplicateUsage = errors.New("duplicate coupon usage")

type UsageRepository interface {
	InsertTx(
		ctx context.Context,
		tx *sql.Tx,
		couponCode string,
		userID string,
		orderID string,
	) error
}
