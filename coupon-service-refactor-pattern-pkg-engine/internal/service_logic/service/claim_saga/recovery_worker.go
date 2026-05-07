package saga

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"coupon-service/internal/model"
)

// =============================================
// Saga Recovery Worker
// =============================================
// Periodically scans Postgres for sagas that are
// stuck in CLAIMING / RESERVING / CONFIRMING state
// (i.e., the service crashed mid-execution).
//
// Re-submits them to the orchestrator for re-execution.
// Because each step checks idempotency, re-running is safe.
// =============================================

type RecoveryWorker struct {
	sagaRepo     SagaRepository
	orchestrator *Orchestrator
	interval     time.Duration
	staleAfter   time.Duration
}

func NewRecoveryWorker(
	sagaRepo SagaRepository,
	orchestrator *Orchestrator,
	interval time.Duration,
	staleAfter time.Duration,
) *RecoveryWorker {
	return &RecoveryWorker{
		sagaRepo:     sagaRepo,
		orchestrator: orchestrator,
		interval:     interval,
		staleAfter:   staleAfter,
	}
}

func (w *RecoveryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.recover(ctx)
		}
	}
}

func (w *RecoveryWorker) recover(ctx context.Context) {
	staleSagas, err := w.sagaRepo.FindStaleProcessing(ctx, w.staleAfter)
	if err != nil {
		log.Printf("[saga-recovery] find stale: %v", err)
		return
	}

	for _, saga := range staleSagas {
		w.rerun(ctx, saga)
	}
}

func (w *RecoveryWorker) rerun(ctx context.Context, saga model.SagaInstance) {
	var payload model.SagaPayload
	if err := json.Unmarshal(saga.Payload, &payload); err != nil {
		log.Printf("[saga-recovery] unmarshal payload sagaID=%s: %v", saga.ID, err)
		return
	}

	log.Printf("[saga-recovery] rerunning sagaID=%s step=%s", saga.ID, saga.CurrentStep)

	// Re-execute from where it left off
	// The orchestrator.execute() checks idempotency at each step,
	// so re-running from the beginning is safe but wasteful.
	// Here we re-execute from the current step onward.

	steps := w.orchestrator.stepsFromStep(saga.CurrentStep)
	if len(steps) == 0 {
		log.Printf("[saga-recovery] unknown step %s for sagaID=%s, marking failed", saga.CurrentStep, saga.ID)
		_ = w.sagaRepo.UpdateFailed(ctx, saga.ID, "recovery: unknown step")
		return
	}

	// Re-run forward from current step
	succeededSteps := make([]SagaStep, 0)

	for _, step := range steps {
		stepCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := step.Execute(stepCtx, &payload)
		cancel()

		if err != nil {
			_ = w.sagaRepo.UpdateFailed(ctx, saga.ID, "recovery: "+err.Error())
			w.orchestrator.compensate(ctx, saga.ID, succeededSteps, payload)
			return
		}
		succeededSteps = append(succeededSteps, step)
	}

	_ = w.sagaRepo.UpdateCompleted(ctx, saga.ID)
	log.Printf("[saga-recovery] saga completed sagaID=%s", saga.ID)
}

// stepsFromStep returns the subslice of steps starting from a given step name.
// This enables the recovery worker to resume from a checkpoint.
func (o *Orchestrator) stepsFromStep(from model.SagaStepName) []SagaStep {
	if from == "" {
		return o.steps
	}
	for i, step := range o.steps {
		if step.Name() == from {
			return o.steps[i:]
		}
	}
	return nil
}
