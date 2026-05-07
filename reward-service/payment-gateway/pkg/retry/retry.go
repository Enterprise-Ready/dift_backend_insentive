package retry

import (
	"context"
	"math"
	"time"

	"github.com/enterprise/payment-gateway/internal/domain"
	"go.uber.org/zap"
)

type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	JitterFactor float64
}

var DefaultConfig = RetryConfig{
	MaxAttempts:  3,
	InitialDelay: 200 * time.Millisecond,
	MaxDelay:     5 * time.Second,
	Multiplier:   2.0,
	JitterFactor: 0.1,
}

// Do executes fn with retry logic using exponential backoff
func Do(ctx context.Context, cfg RetryConfig, logger *zap.Logger, fn func(attempt int) error) error {
	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		lastErr = fn(attempt)
		if lastErr == nil {
			return nil
		}

		// Don't retry non-retryable errors
		if !domain.IsRetryable(lastErr) {
			return lastErr
		}

		if attempt == cfg.MaxAttempts {
			break
		}

		delay := calculateDelay(cfg, attempt)
		logger.Warn("retrying operation",
			zap.Int("attempt", attempt),
			zap.Duration("delay", delay),
			zap.Error(lastErr),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

func calculateDelay(cfg RetryConfig, attempt int) time.Duration {
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))
	// Add jitter
	jitter := delay * cfg.JitterFactor * (2*randFloat() - 1)
	delay += jitter

	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}
	return time.Duration(delay)
}

// Simple PRNG for jitter (not crypto-safe, just for scheduling)
var seed uint64 = 12345

func randFloat() float64 {
	seed ^= seed << 13
	seed ^= seed >> 7
	seed ^= seed << 17
	return float64(seed&0xFFFFFF) / float64(0xFFFFFF)
}
