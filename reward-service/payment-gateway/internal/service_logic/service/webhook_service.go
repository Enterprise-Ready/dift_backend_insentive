package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/enterprise/payment-gateway/internal/config"
	"github.com/enterprise/payment-gateway/internal/domain"
	"github.com/enterprise/payment-gateway/pkg/crypto"
	"github.com/enterprise/payment-gateway/pkg/metrics"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// WebhookRepository persists webhook deliveries
type WebhookRepository interface {
	Create(ctx context.Context, delivery *domain.WebhookDelivery) error
	Update(ctx context.Context, delivery *domain.WebhookDelivery) error
	GetPendingRetries(ctx context.Context, limit int) ([]*domain.WebhookDelivery, error)
}

type WebhookService struct {
	repo    WebhookRepository
	cfg     *config.WebhookConfig
	metrics *metrics.Metrics
	client  *http.Client
	logger  *zap.Logger
	queue   chan *webhookJob
}

type webhookJob struct {
	merchant *domain.Merchant
	event    domain.WebhookEvent
	payload  interface{}
}

func NewWebhookService(repo WebhookRepository, cfg *config.WebhookConfig, m *metrics.Metrics, logger *zap.Logger) *WebhookService {
	svc := &WebhookService{
		repo:    repo,
		cfg:     cfg,
		metrics: m,
		logger:  logger,
		queue:   make(chan *webhookJob, 1000),
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}

	// Start workers
	for i := 0; i < cfg.WorkerCount; i++ {
		go svc.worker()
	}

	return svc
}

// Send enqueues a webhook for delivery
func (s *WebhookService) Send(ctx context.Context, merchant *domain.Merchant, event domain.WebhookEvent, payload interface{}) {
	if merchant.WebhookURL == "" {
		return
	}
	select {
	case s.queue <- &webhookJob{merchant: merchant, event: event, payload: payload}:
	default:
		s.logger.Warn("webhook queue full, dropping delivery",
			zap.String("merchant_id", merchant.ID),
			zap.String("event", string(event)),
		)
	}
}

func (s *WebhookService) worker() {
	for job := range s.queue {
		s.deliver(context.Background(), job.merchant, job.event, job.payload, 1)
	}
}

func (s *WebhookService) deliver(ctx context.Context, merchant *domain.Merchant, event domain.WebhookEvent, payload interface{}, attempt int) {
	start := time.Now()

	payloadBytes, err := json.Marshal(map[string]interface{}{
		"id":         uuid.New().String(),
		"event":      string(event),
		"data":       payload,
		"created_at": time.Now().UTC(),
	})
	if err != nil {
		s.logger.Error("failed to marshal webhook payload", zap.Error(err))
		return
	}

	// Sign the payload
	signature := crypto.HMACSHA256(string(payloadBytes), merchant.WebhookSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, merchant.WebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		s.logger.Error("failed to create webhook request", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", string(event))
	req.Header.Set("X-Webhook-Attempt", fmt.Sprintf("%d", attempt))

	delivery := &domain.WebhookDelivery{
		ID:           uuid.New().String(),
		MerchantID:   merchant.ID,
		Event:        event,
		URL:          merchant.WebhookURL,
		AttemptCount: attempt,
	}

	resp, err := s.client.Do(req)
	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		// Schedule retry
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}

		s.logger.Warn("webhook delivery failed",
			zap.String("merchant_id", merchant.ID),
			zap.String("event", string(event)),
			zap.Int("status", status),
			zap.Int("attempt", attempt),
		)

		s.metrics.WebhookDeliveries.WithLabelValues(string(event), "failed").Inc()
		delivery.ResponseStatus = status
		delivery.IsDelivered = false

		if attempt < s.cfg.MaxRetries {
			retryDelay := s.cfg.RetryBackoff * time.Duration(attempt*attempt) // Exponential
			nextRetry := time.Now().Add(retryDelay)
			delivery.NextRetryAt = &nextRetry

			// Schedule retry
			time.AfterFunc(retryDelay, func() {
				s.deliver(ctx, merchant, event, payload, attempt+1)
			})
		}
	} else {
		s.logger.Info("webhook delivered successfully",
			zap.String("merchant_id", merchant.ID),
			zap.String("event", string(event)),
			zap.Int("attempt", attempt),
		)
		s.metrics.WebhookDeliveries.WithLabelValues(string(event), "success").Inc()
		s.metrics.WebhookLatency.WithLabelValues(string(event)).Observe(time.Since(start).Seconds())

		delivery.ResponseStatus = resp.StatusCode
		delivery.IsDelivered = true
	}

	s.repo.Create(ctx, delivery)
}

// RetryPending retries all pending webhook deliveries (called by cron)
func (s *WebhookService) RetryPending(ctx context.Context) error {
	deliveries, err := s.repo.GetPendingRetries(ctx, 100)
	if err != nil {
		return err
	}

	s.logger.Info("retrying pending webhooks", zap.Int("count", len(deliveries)))
	// Re-queue them (simplified - in production would look up full context)
	for _, d := range deliveries {
		d.AttemptCount++
		d.UpdatedAt = time.Now()
		s.repo.Update(ctx, d)
	}
	return nil
}
