package claim

import (
	"context"
	"errors"
	"time"

	repo "coupon-service/internal/interface/repository"
	serviceport "coupon-service/internal/interface/service/claim"
	"coupon-service/internal/model"
	"coupon-service/pkg/metrics"
)

//////////////////////////////////////////////////
// Service Errors
//////////////////////////////////////////////////

var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrCouponNotFound    = errors.New("coupon not found")
	ErrCouponInactive    = errors.New("coupon inactive")
	ErrCouponExpired     = errors.New("coupon expired")
	ErrCouponNotStarted  = errors.New("coupon not started")
	ErrQuotaExceeded     = errors.New("coupon quota exceeded")
	ErrAlreadyClaimed    = errors.New("coupon already claimed by user")
	ErrIdempotencyLocked = errors.New("request already processing")
)

//////////////////////////////////////////////////
// Service
//////////////////////////////////////////////////

type CouponClaimService struct {
	couponRepo      repo.CouponRepository
	usageRepo       repo.UsageRepository
	outboxRepo      repo.OutboxRepository
	idempotencyRepo repo.IdempotencyRepository
}

func NewCouponClaimService(
	couponRepo repo.CouponRepository,
	usageRepo repo.UsageRepository,
	outboxRepo repo.OutboxRepository,
	idempotencyRepo repo.IdempotencyRepository,
) *CouponClaimService {
	return &CouponClaimService{
		couponRepo:      couponRepo,
		usageRepo:       usageRepo,
		outboxRepo:      outboxRepo,
		idempotencyRepo: idempotencyRepo,
	}
}

var _ serviceport.CouponClaimService = (*CouponClaimService)(nil)

//////////////////////////////////////////////////
// Claim Logic
//////////////////////////////////////////////////

func (s *CouponClaimService) Claim(
	ctx context.Context,
	userID string,
	couponCode string,
	idempotencyKey string,
) error {
	started := time.Now()
	status := "success"
	defer func() { metrics.RecordClaim(status, started) }()

	if userID == "" || couponCode == "" || idempotencyKey == "" {
		status = "invalid_input"
		return ErrInvalidInput
	}

	tx, err := s.couponRepo.BeginTx(ctx)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	record, err := s.idempotencyRepo.InsertOrGetTx(
		ctx,
		tx,
		idempotencyKey,
		userID,
		model.IdempotencyProcessing,
	)
	if err != nil {
		return err
	}

	if record.Status == model.IdempotencySuccess {
		return nil
	}

	if record.Status == model.IdempotencyProcessing &&
		time.Since(record.CreatedAt) < 5*time.Minute {

		status = "idempotency_locked"
		return ErrIdempotencyLocked
	}

	coupon, err := s.couponRepo.LockByCodeTx(
		ctx,
		tx,
		couponCode,
	)
	if err != nil {

		_ = s.idempotencyRepo.MarkFailedTx(ctx, tx, idempotencyKey)

		return err
	}

	if coupon == nil {

		_ = s.idempotencyRepo.MarkFailedTx(ctx, tx, idempotencyKey)

		status = "not_found"
		return ErrCouponNotFound
	}

	now := time.Now()

	if now.Before(coupon.ValidFrom) {

		_ = s.idempotencyRepo.MarkSuccessTx(ctx, tx, idempotencyKey)

		status = "not_started"
		return ErrCouponNotStarted
	}

	if now.After(coupon.ValidTo) {

		_ = s.idempotencyRepo.MarkSuccessTx(ctx, tx, idempotencyKey)

		status = "expired"
		return ErrCouponExpired
	}

	if !coupon.Active {

		_ = s.idempotencyRepo.MarkSuccessTx(ctx, tx, idempotencyKey)

		status = "inactive"
		return ErrCouponInactive
	}

	if coupon.Used >= coupon.MaxUsage {

		_ = s.idempotencyRepo.MarkSuccessTx(ctx, tx, idempotencyKey)

		status = "quota_exceeded"
		return ErrQuotaExceeded
	}

	err = s.usageRepo.InsertTx(
		ctx,
		tx,
		couponCode,
		userID,
		"",
	)

	if err != nil {

		if errors.Is(err, repo.ErrDuplicateUsage) {

			_ = s.idempotencyRepo.MarkSuccessTx(ctx, tx, idempotencyKey)

			status = "already_claimed"
			return ErrAlreadyClaimed
		}

		_ = s.idempotencyRepo.MarkFailedTx(ctx, tx, idempotencyKey)

		return err
	}

	err = s.couponRepo.IncreaseUsageTx(
		ctx,
		tx,
		couponCode,
	)

	if err != nil {

		_ = s.idempotencyRepo.MarkFailedTx(ctx, tx, idempotencyKey)

		return err
	}

	event := model.CouponEvent{
		Type:       model.CouponClaimed,
		UserID:     userID,
		CouponCode: coupon.Code,
		OccurredAt: time.Now(),
	}

	outbox := model.OutboxInsert{
		AggregateType: "coupon",
		AggregateID:   coupon.Code,
		EventType:     string(model.CouponClaimed),
		Payload:       event,
	}

	err = s.outboxRepo.InsertTx(ctx, tx, outbox)
	if err != nil {

		_ = s.idempotencyRepo.MarkFailedTx(ctx, tx, idempotencyKey)

		return err
	}

	err = s.idempotencyRepo.MarkSuccessTx(ctx, tx, idempotencyKey)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	committed = true

	return nil
}
