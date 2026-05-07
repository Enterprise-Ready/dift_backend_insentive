package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	couponevent "coupon-service/internal/interface/coupon_event"
	repo "coupon-service/internal/interface/repository"
	"coupon-service/internal/model"
)

type Worker struct {
	db         *sql.DB
	outboxRepo repo.OutboxRepository
	publisher  couponevent.CouponEventPublisher
	interval   time.Duration
}

func NewWorker(
	db *sql.DB,
	outboxRepo repo.OutboxRepository,
	publisher couponevent.CouponEventPublisher,
	interval time.Duration,
) *Worker {

	return &Worker{
		db:         db,
		outboxRepo: outboxRepo,
		publisher:  publisher,
		interval:   interval,
	}
}

func (w *Worker) Start(ctx context.Context) {

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-ticker.C:
			w.process(ctx)

		}
	}
}

func (w *Worker) process(ctx context.Context) {

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}

	events, err := w.outboxRepo.GetPending(ctx, tx, 50)
	if err != nil {
		_ = tx.Rollback()
		return
	}

	if len(events) == 0 {
		_ = tx.Commit()
		return
	}

	for _, e := range events {

		var couponEvent model.CouponEvent

		err = json.Unmarshal(e.Payload, &couponEvent)
		if err != nil {
			continue
		}

		err = w.publisher.Publish(ctx, couponEvent)
		if err != nil {
			continue
		}

		err = w.outboxRepo.MarkSent(ctx, tx, e.ID)
		if err != nil {
			continue
		}
	}

	_ = tx.Commit()
}
