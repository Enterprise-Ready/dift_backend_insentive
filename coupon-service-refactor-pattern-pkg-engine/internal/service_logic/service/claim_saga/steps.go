package saga

import (
	"context"
	"errors"
	"fmt"
	"time"

	"coupon-service/internal/interface/repository"
	"coupon-service/internal/model"
)

// =============================================
// Saga Steps
// =============================================
// Three steps in forward order:
//
//  1. ClaimStep   – idempotency check, lock coupon, insert usage
//  2. ReserveStep – publish NATS event (coupon.reserved), await order-service ACK
//  3. ConfirmStep – finalize usage count, mark success
//
// Each step has a Compensate() that undoes the forward action.
// =============================================

// ─────────────────────────────────────────────
// Step 1: Claim
// ─────────────────────────────────────────────

// ClaimStep locks the coupon and inserts a usage record.
// Compensate: removes the usage record and releases the lock.
type ClaimStep struct {
	couponRepo repository.CouponRepository
	usageRepo  repository.UsageRepository
	idempRepo  repository.IdempotencyRepository
}

func NewClaimStep(
	couponRepo repository.CouponRepository,
	usageRepo repository.UsageRepository,
	idempRepo repository.IdempotencyRepository,
) *ClaimStep {
	return &ClaimStep{
		couponRepo: couponRepo,
		usageRepo:  usageRepo,
		idempRepo:  idempRepo,
	}
}

func (s *ClaimStep) Name() model.SagaStepName { return model.StepClaim }

func (s *ClaimStep) Execute(ctx context.Context, payload *model.SagaPayload) error {
	tx, err := s.couponRepo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Idempotency check
	record, err := s.idempRepo.InsertOrGetTx(
		ctx, tx,
		payload.IdempotencyKey,
		payload.UserID,
		model.IdempotencyProcessing,
	)
	if err != nil {
		return err
	}
	if record.Status == model.IdempotencySuccess {
		// Already claimed — saga can consider this step done
		now := time.Now().UTC()
		payload.ClaimedAt = &now
		return nil
	}

	// Lock coupon FOR UPDATE (prevents concurrent over-redemption)
	coupon, err := s.couponRepo.LockByCodeTx(ctx, tx, payload.CouponCode)
	if err != nil {
		return fmt.Errorf("lock coupon: %w", err)
	}
	if coupon == nil {
		return ErrCouponNotFound
	}

	now := time.Now().UTC()

	// Validity checks
	if now.Before(coupon.ValidFrom) {
		return ErrCouponNotStarted
	}
	if now.After(coupon.ValidTo) {
		return ErrCouponExpired
	}
	if !coupon.Active {
		return ErrCouponInactive
	}
	if coupon.Used >= coupon.MaxUsage {
		return ErrQuotaExceeded
	}

	// Insert usage record (UNIQUE(coupon_code, user_id) prevents double-claim)
	if err := s.usageRepo.InsertTx(ctx, tx, payload.CouponCode, payload.UserID, ""); err != nil {
		if errors.Is(err, repository.ErrDuplicateUsage) {
			return ErrAlreadyClaimed
		}
		return err
	}

	// Increase usage count atomically
	if err := s.couponRepo.IncreaseUsageTx(ctx, tx, payload.CouponCode); err != nil {
		return fmt.Errorf("increase usage: %w", err)
	}

	if err := s.idempRepo.MarkSuccessTx(ctx, tx, payload.IdempotencyKey); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	payload.ClaimedAt = &now
	return nil
}

// Compensate decreases usage and removes the usage record.
// In practice: update used = used - 1 WHERE used > 0
func (s *ClaimStep) Compensate(ctx context.Context, payload *model.SagaPayload) error {
	if payload.ClaimedAt == nil {
		// Claim never succeeded — nothing to undo
		return nil
	}

	tx, err := s.couponRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Decrement usage
	_, err = tx.ExecContext(ctx, `
		UPDATE coupons
		SET used = GREATEST(used - 1, 0),
		    updated_at = NOW()
		WHERE code = $1
	`, payload.CouponCode)
	if err != nil {
		return fmt.Errorf("decrement usage: %w", err)
	}

	// Remove usage record to allow re-claim
	_, err = tx.ExecContext(ctx, `
		DELETE FROM coupon_usage_history
		WHERE coupon_code = $1 AND user_id = $2
	`, payload.CouponCode, payload.UserID)
	if err != nil {
		return fmt.Errorf("remove usage record: %w", err)
	}

	return tx.Commit()
}

// ─────────────────────────────────────────────
// Step 2: Reserve
// ─────────────────────────────────────────────

// EventPublisher publishes events to NATS JetStream.
type EventPublisher interface {
	Publish(ctx context.Context, e model.CouponEvent) error
}

// ReserveStep publishes a "coupon.reserved" event to NATS and records the orderID.
// Compensate: publishes a "coupon.reserve_cancelled" event.
type ReserveStep struct {
	publisher EventPublisher
}

func NewReserveStep(publisher EventPublisher) *ReserveStep {
	return &ReserveStep{publisher: publisher}
}

func (s *ReserveStep) Name() model.SagaStepName { return model.StepReserve }

func (s *ReserveStep) Execute(ctx context.Context, payload *model.SagaPayload) error {
	event := model.CouponEvent{
		Type:       model.CouponClaimed,
		UserID:     payload.UserID,
		CouponCode: payload.CouponCode,
		OccurredAt: time.Now().UTC(),
	}

	if err := s.publisher.Publish(ctx, event); err != nil {
		return fmt.Errorf("publish reserve event: %w", err)
	}

	// In a real system, you would await an ACK from order-service via NATS reply-to.
	// For this implementation, fire-and-forget with the outbox pattern guaranteeing delivery.
	payload.OrderID = "reserved"
	return nil
}

func (s *ReserveStep) Compensate(ctx context.Context, payload *model.SagaPayload) error {
	if payload.OrderID == "" {
		return nil
	}

	// Publish cancellation event
	event := model.CouponEvent{
		Type:       model.CouponDeactivated, // reuse; extend model.CouponEventType for "RESERVE_CANCELLED"
		UserID:     payload.UserID,
		CouponCode: payload.CouponCode,
		OccurredAt: time.Now().UTC(),
	}
	return s.publisher.Publish(ctx, event)
}

// ─────────────────────────────────────────────
// Step 3: Confirm
// ─────────────────────────────────────────────

// ConfirmStep writes the final outbox event (persisted delivery guarantee)
// and marks the saga as confirmed in the outbox.
type ConfirmStep struct {
	couponRepo repository.CouponRepository
	outboxRepo repository.OutboxRepository
}

func NewConfirmStep(
	couponRepo repository.CouponRepository,
	outboxRepo repository.OutboxRepository,
) *ConfirmStep {
	return &ConfirmStep{
		couponRepo: couponRepo,
		outboxRepo: outboxRepo,
	}
}

func (s *ConfirmStep) Name() model.SagaStepName { return model.StepConfirm }

func (s *ConfirmStep) Execute(ctx context.Context, payload *model.SagaPayload) error {
	tx, err := s.couponRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	event := model.CouponEvent{
		Type:       model.CouponClaimed,
		UserID:     payload.UserID,
		CouponCode: payload.CouponCode,
		OccurredAt: time.Now().UTC(),
	}

	// Persist to outbox for guaranteed delivery via outbox worker
	if err := s.outboxRepo.InsertTx(ctx, tx, model.OutboxInsert{
		AggregateType: "coupon",
		AggregateID:   payload.CouponCode,
		EventType:     string(model.CouponClaimed),
		Payload:       event,
	}); err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	return tx.Commit()
}

// Compensate for Confirm is a no-op:
// if we got here, ClaimStep.Compensate already decremented usage.
// The outbox event may have already been published — downstream services
// must be idempotent consumers.
func (s *ConfirmStep) Compensate(_ context.Context, _ *model.SagaPayload) error {
	return nil
}

// ─────────────────────────────────────────────
// Step-level errors
// ─────────────────────────────────────────────

var (
	ErrCouponNotFound   = errors.New("coupon not found")
	ErrCouponInactive   = errors.New("coupon inactive")
	ErrCouponExpired    = errors.New("coupon expired")
	ErrCouponNotStarted = errors.New("coupon not yet started")
	ErrQuotaExceeded    = errors.New("coupon quota exceeded")
	ErrAlreadyClaimed   = errors.New("coupon already claimed by user")
)
