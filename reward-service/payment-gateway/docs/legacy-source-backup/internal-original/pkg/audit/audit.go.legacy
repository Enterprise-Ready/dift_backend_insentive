package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/enterprise/payment-gateway/internal/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Logger struct {
	repo   AuditRepository
	logger *zap.Logger
}

type AuditRepository interface {
	Save(ctx context.Context, log *domain.AuditLog) error
}

func NewLogger(repo AuditRepository, logger *zap.Logger) *Logger {
	return &Logger{repo: repo, logger: logger}
}

// Log records an audit event
func (l *Logger) Log(ctx context.Context, entityType, entityID, action, actorID, actorType, ipAddress string, oldVal, newVal interface{}) {
	var oldJSON, newJSON domain.JSONB

	if oldVal != nil {
		if data, err := json.Marshal(oldVal); err == nil {
			json.Unmarshal(data, &oldJSON)
		}
	}
	if newVal != nil {
		if data, err := json.Marshal(newVal); err == nil {
			json.Unmarshal(data, &newJSON)
		}
	}

	log := &domain.AuditLog{
		ID:         uuid.New().String(),
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		ActorID:    actorID,
		ActorType:  actorType,
		IPAddress:  ipAddress,
		OldValue:   oldJSON,
		NewValue:   newJSON,
		CreatedAt:  time.Now(),
	}

	// Save async - don't block the main flow
	go func() {
		saveCtx := context.Background()
		if err := l.repo.Save(saveCtx, log); err != nil {
			l.logger.Error("failed to save audit log",
				zap.Error(err),
				zap.String("entity_type", entityType),
				zap.String("entity_id", entityID),
				zap.String("action", action),
			)
		}
	}()
}

// PaymentAction logs a payment action
func (l *Logger) PaymentAction(ctx context.Context, paymentID, action, actorID, ipAddress string, old, new interface{}) {
	l.Log(ctx, "payment", paymentID, action, actorID, "merchant", ipAddress, old, new)
}

// RefundAction logs a refund action
func (l *Logger) RefundAction(ctx context.Context, refundID, paymentID, action, actorID, ipAddress string) {
	l.Log(ctx, "refund", refundID, action, actorID, "merchant", ipAddress, map[string]string{"payment_id": paymentID}, nil)
}

// MerchantAction logs merchant account changes
func (l *Logger) MerchantAction(ctx context.Context, merchantID, action, actorID, ipAddress string, old, new interface{}) {
	l.Log(ctx, "merchant", merchantID, action, actorID, "admin", ipAddress, old, new)
}
