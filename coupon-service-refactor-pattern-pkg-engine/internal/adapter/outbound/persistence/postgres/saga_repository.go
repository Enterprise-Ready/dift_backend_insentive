package postgres

import (
	"context"
	"database/sql"
	"time"

	"coupon-service/internal/model"
)

// =============================================
// Saga Repository (Postgres)
// =============================================
// Append-only saga state transitions.
// All updates use WHERE id = $1 to be safe
// with concurrent workers.
// =============================================

type SagaRepository struct {
	db *sql.DB
}

func NewSagaRepository(db *sql.DB) *SagaRepository {
	return &SagaRepository{db: db}
}

func (r *SagaRepository) Insert(ctx context.Context, saga *model.SagaInstance) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO saga_instances (
			id, user_id, coupon_code, idempotency_key,
			status, current_step, payload, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`,
		saga.ID,
		saga.UserID,
		saga.CouponCode,
		saga.IdempotencyKey,
		saga.Status,
		saga.CurrentStep,
		saga.Payload,
		saga.CreatedAt,
		saga.UpdatedAt,
	)
	return err
}

func (r *SagaRepository) UpdateStatus(
	ctx context.Context,
	sagaID string,
	status model.SagaStatus,
	step model.SagaStepName,
) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE saga_instances
		SET status = $2, current_step = $3, updated_at = NOW()
		WHERE id = $1
	`, sagaID, string(status), string(step))
	return err
}

func (r *SagaRepository) UpdateFailed(
	ctx context.Context,
	sagaID string,
	reason string,
) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE saga_instances
		SET status = 'FAILED', failure_reason = $2, updated_at = NOW()
		WHERE id = $1
	`, sagaID, reason)
	return err
}

func (r *SagaRepository) UpdateCompleted(ctx context.Context, sagaID string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE saga_instances
		SET status = 'COMPLETED', completed_at = $2, updated_at = NOW()
		WHERE id = $1
	`, sagaID, now)
	return err
}

func (r *SagaRepository) FindByIDForUpdate(
	ctx context.Context,
	sagaID string,
) (*model.SagaInstance, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, coupon_code, order_id, idempotency_key,
		       status, current_step, failure_reason, payload, created_at, updated_at, completed_at
		FROM saga_instances
		WHERE id = $1
		FOR UPDATE
	`, sagaID)

	return scanSaga(row)
}

// FindStaleProcessing returns sagas stuck in in-progress states for longer than olderThan.
// Used by the recovery worker.
func (r *SagaRepository) FindStaleProcessing(
	ctx context.Context,
	olderThan time.Duration,
) ([]model.SagaInstance, error) {
	cutoff := time.Now().UTC().Add(-olderThan)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, coupon_code, order_id, idempotency_key,
		       status, current_step, failure_reason, payload, created_at, updated_at, completed_at
		FROM saga_instances
		WHERE status IN ('CLAIMING','RESERVING','CONFIRMING','COMPENSATING')
		  AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT 50
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sagas []model.SagaInstance
	for rows.Next() {
		s, err := scanSagaRow(rows)
		if err != nil {
			return nil, err
		}
		sagas = append(sagas, s)
	}
	return sagas, rows.Err()
}

func (r *SagaRepository) InsertStepLog(ctx context.Context, log model.SagaStepLog) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO saga_step_logs (saga_id, step_name, status, attempt, error, executed_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`,
		log.SagaID,
		string(log.StepName),
		string(log.Status),
		log.Attempt,
		log.Error,
		log.ExecutedAt,
	)
	return err
}

func (r *SagaRepository) FindByIdempotencyKey(
	ctx context.Context,
	key string,
) (*model.SagaInstance, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, coupon_code, order_id, idempotency_key,
		       status, current_step, failure_reason, payload, created_at, updated_at, completed_at
		FROM saga_instances
		WHERE idempotency_key = $1
	`, key)

	saga, err := scanSaga(row)
	if err != nil {
		return nil, err
	}
	return saga, nil
}

// ── Scan helpers ─────────────────────────────────────────────────────

func scanSaga(row *sql.Row) (*model.SagaInstance, error) {
	var s model.SagaInstance
	var orderID sql.NullString
	var currentStep sql.NullString
	var failureReason sql.NullString
	var completedAt sql.NullTime

	err := row.Scan(
		&s.ID,
		&s.UserID,
		&s.CouponCode,
		&orderID,
		&s.IdempotencyKey,
		&s.Status,
		&currentStep,
		&failureReason,
		&s.Payload,
		&s.CreatedAt,
		&s.UpdatedAt,
		&completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	s.OrderID = orderID.String
	s.CurrentStep = model.SagaStepName(currentStep.String)
	s.FailureReason = failureReason.String
	if completedAt.Valid {
		s.CompletedAt = &completedAt.Time
	}

	return &s, nil
}

func scanSagaRow(rows *sql.Rows) (model.SagaInstance, error) {
	var s model.SagaInstance
	var orderID sql.NullString
	var currentStep sql.NullString
	var failureReason sql.NullString
	var completedAt sql.NullTime

	err := rows.Scan(
		&s.ID,
		&s.UserID,
		&s.CouponCode,
		&orderID,
		&s.IdempotencyKey,
		&s.Status,
		&currentStep,
		&failureReason,
		&s.Payload,
		&s.CreatedAt,
		&s.UpdatedAt,
		&completedAt,
	)
	if err != nil {
		return s, err
	}

	s.OrderID = orderID.String
	s.CurrentStep = model.SagaStepName(currentStep.String)
	s.FailureReason = failureReason.String
	if completedAt.Valid {
		s.CompletedAt = &completedAt.Time
	}

	return s, nil
}
