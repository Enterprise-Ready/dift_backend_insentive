package saga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"coupon-service/internal/model"

	"github.com/google/uuid"
)

// =============================================
// Saga Orchestrator
// =============================================
// Manages the Claim → Reserve → Confirm flow.
// On failure at any step, triggers compensating
// transactions in reverse order.
//
// Design decisions:
//   - Each step is a SagaStep interface (Execute + Compensate)
//   - Steps are run in order; on failure, compensate all previous
//   - SagaInstance is persisted to Postgres before each step
//     → crash-safe: a recovery goroutine re-runs stale sagas
//   - Rate limiting enforced via Redis before saga starts
// =============================================

// SagaRepository handles persistence of SagaInstance and SagaStepLog.
type SagaRepository interface {
	Insert(ctx context.Context, saga *model.SagaInstance) error
	UpdateStatus(ctx context.Context, sagaID string, status model.SagaStatus, step model.SagaStepName) error
	UpdateFailed(ctx context.Context, sagaID string, reason string) error
	UpdateCompleted(ctx context.Context, sagaID string) error
	FindByIDForUpdate(ctx context.Context, sagaID string) (*model.SagaInstance, error)
	FindStaleProcessing(ctx context.Context, olderThan time.Duration) ([]model.SagaInstance, error)
	InsertStepLog(ctx context.Context, log model.SagaStepLog) error
	FindByIdempotencyKey(ctx context.Context, key string) (*model.SagaInstance, error)
}

// RateLimiter enforces per-user rate limits via Redis.
type RateLimiter interface {
	// Allow returns true if the request is within limits.
	Allow(ctx context.Context, userID string) (bool, error)
}

// SagaStep is the interface that each step must implement.
type SagaStep interface {
	Name() model.SagaStepName
	Execute(ctx context.Context, payload *model.SagaPayload) error
	Compensate(ctx context.Context, payload *model.SagaPayload) error
}

// Orchestrator manages saga lifecycle.
type Orchestrator struct {
	sagaRepo    SagaRepository
	rateLimiter RateLimiter
	steps       []SagaStep // ordered: Claim, Reserve, Confirm
}

// NewOrchestrator creates a new saga orchestrator with the given steps.
// Steps must be ordered: Claim → Reserve → Confirm.
func NewOrchestrator(
	sagaRepo SagaRepository,
	rateLimiter RateLimiter,
	steps []SagaStep,
) *Orchestrator {
	return &Orchestrator{
		sagaRepo:    sagaRepo,
		rateLimiter: rateLimiter,
		steps:       steps,
	}
}

// Start begins a new saga or returns the existing result for an idempotent key.
func (o *Orchestrator) Start(
	ctx context.Context,
	cmd model.SagaStartCommand,
) (string, error) {

	// ── Idempotency check ──────────────────────────────────────────────
	existing, err := o.sagaRepo.FindByIdempotencyKey(ctx, cmd.IdempotencyKey)
	if err != nil {
		return "", fmt.Errorf("idempotency check: %w", err)
	}
	if existing != nil {
		// Return existing saga ID — caller can poll for status
		return existing.ID, nil
	}

	// ── Rate limit ─────────────────────────────────────────────────────
	allowed, err := o.rateLimiter.Allow(ctx, cmd.UserID)
	if err != nil {
		return "", fmt.Errorf("rate limiter error: %w", err)
	}
	if !allowed {
		return "", ErrRateLimitExceeded
	}

	// ── Create saga instance ───────────────────────────────────────────
	sagaID := uuid.New().String()
	now := time.Now().UTC()

	payload := model.SagaPayload{
		UserID:         cmd.UserID,
		CouponCode:     cmd.CouponCode,
		OrderTotal:     cmd.OrderTotal,
		IdempotencyKey: cmd.IdempotencyKey,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	saga := &model.SagaInstance{
		ID:             sagaID,
		UserID:         cmd.UserID,
		CouponCode:     cmd.CouponCode,
		IdempotencyKey: cmd.IdempotencyKey,
		Status:         model.SagaStatusStarted,
		Payload:        payloadBytes,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := o.sagaRepo.Insert(ctx, saga); err != nil {
		return "", fmt.Errorf("persist saga: %w", err)
	}

	// ── Execute steps asynchronously (called in a goroutine by caller) ─
	// Return sagaID immediately; caller polls /saga/{id}/status
	go o.execute(context.Background(), sagaID, payload)

	return sagaID, nil
}

// execute runs the saga steps in order. Called in a goroutine.
func (o *Orchestrator) execute(ctx context.Context, sagaID string, payload model.SagaPayload) {
	succeededSteps := make([]SagaStep, 0, len(o.steps))

	for _, step := range o.steps {
		// Persist step start
		_ = o.sagaRepo.UpdateStatus(ctx, sagaID, toStatus(step.Name(), false), step.Name())
		_ = o.sagaRepo.InsertStepLog(ctx, model.SagaStepLog{
			SagaID:     sagaID,
			StepName:   step.Name(),
			Status:     model.StepPending,
			Attempt:    1,
			ExecutedAt: time.Now().UTC(),
		})

		// Execute with timeout
		stepCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := step.Execute(stepCtx, &payload)
		cancel()

		if err != nil {
			// Log failure
			_ = o.sagaRepo.InsertStepLog(ctx, model.SagaStepLog{
				SagaID:     sagaID,
				StepName:   step.Name(),
				Status:     model.StepFailed,
				Attempt:    1,
				Error:      err.Error(),
				ExecutedAt: time.Now().UTC(),
			})
			_ = o.sagaRepo.UpdateFailed(ctx, sagaID, fmt.Sprintf("step %s: %s", step.Name(), err.Error()))

			// Compensate all previously succeeded steps in REVERSE order
			o.compensate(ctx, sagaID, succeededSteps, payload)
			return
		}

		// Log success
		_ = o.sagaRepo.InsertStepLog(ctx, model.SagaStepLog{
			SagaID:     sagaID,
			StepName:   step.Name(),
			Status:     model.StepSucceeded,
			Attempt:    1,
			ExecutedAt: time.Now().UTC(),
		})
		succeededSteps = append(succeededSteps, step)
	}

	// All steps succeeded
	_ = o.sagaRepo.UpdateCompleted(ctx, sagaID)
}

// compensate runs compensating transactions in reverse order.
func (o *Orchestrator) compensate(
	ctx context.Context,
	sagaID string,
	succeededSteps []SagaStep,
	payload model.SagaPayload,
) {
	_ = o.sagaRepo.UpdateStatus(ctx, sagaID, model.SagaStatusCompensating, "")

	for i := len(succeededSteps) - 1; i >= 0; i-- {
		step := succeededSteps[i]
		compCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := step.Compensate(compCtx, &payload)
		cancel()

		logStatus := model.StepCompensated
		errMsg := ""
		if err != nil {
			logStatus = model.StepFailed
			errMsg = err.Error()
			// Log but continue compensating remaining steps
		}
		_ = o.sagaRepo.InsertStepLog(ctx, model.SagaStepLog{
			SagaID:     sagaID,
			StepName:   step.Name(),
			Status:     logStatus,
			Attempt:    1,
			Error:      errMsg,
			ExecutedAt: time.Now().UTC(),
		})
	}

	_ = o.sagaRepo.UpdateStatus(ctx, sagaID, model.SagaStatusCompensated, "")
}

// toStatus maps a step name and completion flag to a SagaStatus.
func toStatus(step model.SagaStepName, done bool) model.SagaStatus {
	if !done {
		switch step {
		case model.StepClaim:
			return model.SagaStatusClaiming
		case model.StepReserve:
			return model.SagaStatusReserving
		case model.StepConfirm:
			return model.SagaStatusConfirming
		}
	}
	return model.SagaStatusCompleted
}

var ErrRateLimitExceeded = errors.New("rate limit exceeded: too many coupon claim requests")
