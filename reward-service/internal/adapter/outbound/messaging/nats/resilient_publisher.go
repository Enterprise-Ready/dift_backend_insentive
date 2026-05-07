package nats

import (
	"context"
	"fmt"
	"time"

	obs "reward-service/pkg/observability"
	"reward-service/pkg/resilience"

	"github.com/nats-io/nats.go"
)

// ResilientJetStreamPublisher wraps JetStreamPublisher with circuit breaker + metrics
type ResilientJetStreamPublisher struct {
	js      nats.JetStreamContext
	cb      *resilience.CircuitBreaker
	metrics *obs.Metrics
}

func NewResilientJetStreamPublisher(
	nc *nats.Conn,
	cb *resilience.CircuitBreaker,
	metrics *obs.Metrics,
) (*ResilientJetStreamPublisher, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("create jetstream context: %w", err)
	}

	return &ResilientJetStreamPublisher{
		js:      js,
		cb:      cb,
		metrics: metrics,
	}, nil
}

// Publish sends a message with circuit breaker protection and automatic retries
func (p *ResilientJetStreamPublisher) Publish(
	ctx context.Context,
	subject string,
	payload []byte,
) error {
	retryCfg := resilience.RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 50 * time.Millisecond,
		MaxInterval:     500 * time.Millisecond,
		Multiplier:      2.0,
		JitterFactor:    0.2,
	}

	err := resilience.WithCircuitBreaker(ctx, p.cb, retryCfg, func(ctx context.Context) error {
		start := time.Now()

		_, pubErr := p.js.PublishMsg(&nats.Msg{
			Subject: subject,
			Data:    payload,
		},
			nats.Context(ctx),
			nats.AckWait(5*time.Second),
		)

		p.metrics.NATSPublishDuration.WithLabelValues(subject).Observe(time.Since(start).Seconds())

		if pubErr != nil {
			p.metrics.NATSPublishTotal.WithLabelValues(subject, "error").Inc()
			return fmt.Errorf("jetstream publish: %w", pubErr)
		}

		p.metrics.NATSPublishTotal.WithLabelValues(subject, "success").Inc()
		return nil
	})

	return err
}

// DeadLetterQueueHandler handles messages that failed processing
type DeadLetterQueueHandler struct {
	js      nats.JetStreamContext
	subject string // DLQ subject
}

func NewDeadLetterQueueHandler(js nats.JetStreamContext, dlqSubject string) *DeadLetterQueueHandler {
	return &DeadLetterQueueHandler{
		js:      js,
		subject: dlqSubject,
	}
}

// Send forwards a failed message to the DLQ with metadata
func (d *DeadLetterQueueHandler) Send(
	ctx context.Context,
	originalSubject string,
	payload []byte,
	errMsg string,
) error {
	// Wrap with error context
	dlqPayload := fmt.Sprintf(`{"original_subject":%q,"error":%q,"payload":%q,"failed_at":%q}`,
		originalSubject,
		errMsg,
		payload,
		time.Now().UTC().Format(time.RFC3339),
	)

	_, err := d.js.PublishMsg(&nats.Msg{
		Subject: d.subject,
		Data:    []byte(dlqPayload),
	}, nats.Context(ctx))

	return err
}

// ConsumerWithDLQ wraps a message handler to automatically DLQ non-retryable failures
type ConsumerMiddleware struct {
	dlq     *DeadLetterQueueHandler
	metrics *obs.Metrics
}

func NewConsumerMiddleware(dlq *DeadLetterQueueHandler, metrics *obs.Metrics) *ConsumerMiddleware {
	return &ConsumerMiddleware{dlq: dlq, metrics: metrics}
}

// WrapHandler returns a NATS message handler with DLQ support and metrics
func (m *ConsumerMiddleware) WrapHandler(
	subject string,
	maxRetries int,
	fn func(ctx context.Context, data []byte) error,
) func(*nats.Msg) {
	return func(msg *nats.Msg) {
		ctx := context.Background()

		// Try up to maxRetries times
		var lastErr error
		for i := 0; i < maxRetries; i++ {
			if err := fn(ctx, msg.Data); err != nil {
				lastErr = err
				time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
				continue
			}
			// Success
			m.metrics.NATSConsumeTotal.WithLabelValues(subject, "success").Inc()
			msg.Ack()
			return
		}

		// All retries exhausted → DLQ
		m.metrics.NATSConsumeTotal.WithLabelValues(subject, "dead").Inc()

		if m.dlq != nil {
			_ = m.dlq.Send(ctx, subject, msg.Data, lastErr.Error())
		}

		msg.Ack() // Ack to prevent infinite re-delivery
	}
}
