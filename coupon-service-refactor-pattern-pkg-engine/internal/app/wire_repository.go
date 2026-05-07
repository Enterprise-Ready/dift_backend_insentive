package app

import (
	repoadapter "coupon-service/internal/adapter/outbound/persistence/postgres"
	"database/sql"
)

func wireRepositories(db *sql.DB) Repositories {
	if db == nil {
		return Repositories{}
	}
	return Repositories{
		Coupon:      repoadapter.NewCouponRepository(db),
		Usage:       repoadapter.NewUsageRepository(db),
		Outbox:      repoadapter.NewOutboxRepository(db),
		Idempotency: repoadapter.NewIdempotencyRepository(db),
		Saga:        repoadapter.NewSagaRepository(db),
	}
}
