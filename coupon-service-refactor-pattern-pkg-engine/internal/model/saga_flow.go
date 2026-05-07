package model

import "time"

// =============================================
// Saga Orchestrator — State Machine Models
// =============================================
// Pattern: Orchestration-based saga
// (Orchestrator controls flow, not choreography)
//
// States: STARTED → CLAIMING → RESERVING → CONFIRMING → COMPLETED
//                                         ↘ COMPENSATING → COMPENSATED
//
// Each step has:
//   - Execute:     the forward action
//   - Compensate:  the rollback action (if later step fails)
//
// Persisted to Postgres so the orchestrator survives crashes.
// =============================================

// SagaStatus tracks the overall saga lifecycle
type SagaStatus string

const (
	SagaStatusStarted      SagaStatus = "STARTED"
	SagaStatusClaiming     SagaStatus = "CLAIMING"
	SagaStatusReserving    SagaStatus = "RESERVING"
	SagaStatusConfirming   SagaStatus = "CONFIRMING"
	SagaStatusCompleted    SagaStatus = "COMPLETED"
	SagaStatusCompensating SagaStatus = "COMPENSATING"
	SagaStatusCompensated  SagaStatus = "COMPENSATED"
	SagaStatusFailed       SagaStatus = "FAILED" // unrecoverable
)

// SagaStepName identifies each step in the saga
type SagaStepName string

const (
	StepClaim   SagaStepName = "CLAIM"
	StepReserve SagaStepName = "RESERVE"
	StepConfirm SagaStepName = "CONFIRM"
)

// SagaStepStatus is the result of a single step execution
type SagaStepStatus string

const (
	StepPending     SagaStepStatus = "PENDING"
	StepSucceeded   SagaStepStatus = "SUCCEEDED"
	StepFailed      SagaStepStatus = "FAILED"
	StepCompensated SagaStepStatus = "COMPENSATED"
)

// SagaInstance is the persisted saga record
type SagaInstance struct {
	ID             string       `json:"id"` // UUID
	UserID         string       `json:"user_id"`
	CouponCode     string       `json:"coupon_code"`
	OrderID        string       `json:"order_id"` // set after reserve step
	IdempotencyKey string       `json:"idempotency_key"`
	Status         SagaStatus   `json:"status"`
	CurrentStep    SagaStepName `json:"current_step"`
	FailureReason  string       `json:"failure_reason"` // filled on failure
	Payload        []byte       `json:"payload"`        // JSON blob for step data
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	CompletedAt    *time.Time   `json:"completed_at,omitempty"`
}

// SagaStepLog records the history of each step execution
type SagaStepLog struct {
	ID         int64          `json:"id"`
	SagaID     string         `json:"saga_id"`
	StepName   SagaStepName   `json:"step_name"`
	Status     SagaStepStatus `json:"status"`
	Attempt    int            `json:"attempt"`
	Error      string         `json:"error,omitempty"`
	ExecutedAt time.Time      `json:"executed_at"`
}

// SagaStartCommand is the input to start a new saga
type SagaStartCommand struct {
	UserID         string
	CouponCode     string
	IdempotencyKey string
	// OrderTotal is needed for intelligence engine during confirm
	OrderTotal float64
}

// SagaPayload is the structured data stored in SagaInstance.Payload
type SagaPayload struct {
	UserID         string  `json:"user_id"`
	CouponCode     string  `json:"coupon_code"`
	OrderTotal     float64 `json:"order_total"`
	IdempotencyKey string  `json:"idempotency_key"`
	// Populated after claim step succeeds
	ClaimedAt *time.Time `json:"claimed_at,omitempty"`
	// Populated after reserve step succeeds
	OrderID string `json:"order_id,omitempty"`
}
