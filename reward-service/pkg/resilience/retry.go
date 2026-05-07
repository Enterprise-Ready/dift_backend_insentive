package resilience

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts     int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	JitterFactor    float64 // 0.0 - 1.0, adds randomness to prevent thundering herd
	RetryableErrors []error // if nil, all errors are retried
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		Multiplier:      2.0,
		JitterFactor:    0.3,
	}
}

// RetryWithBackoff retries fn with exponential backoff
func RetryWithBackoff(ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) error) error {
	var lastErr error
	interval := cfg.InitialInterval

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Check if error is retryable
		if len(cfg.RetryableErrors) > 0 && !isRetryable(lastErr, cfg.RetryableErrors) {
			return lastErr
		}

		// Don't sleep after last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Calculate sleep with jitter
		sleep := addJitter(interval, cfg.JitterFactor)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}

		// Increase interval (capped at max)
		interval = time.Duration(float64(interval) * cfg.Multiplier)
		if interval > cfg.MaxInterval {
			interval = cfg.MaxInterval
		}
	}

	return &RetryExhaustedError{
		Attempts: cfg.MaxAttempts,
		Err:      lastErr,
	}
}

func isRetryable(err error, retryableErrors []error) bool {
	for _, re := range retryableErrors {
		if errors.Is(err, re) {
			return true
		}
	}
	return false
}

func addJitter(d time.Duration, factor float64) time.Duration {
	if factor == 0 {
		return d
	}
	jitter := time.Duration(float64(d) * factor * (rand.Float64()*2 - 1))
	return d + jitter
}

// RetryExhaustedError is returned when all retries are exhausted
type RetryExhaustedError struct {
	Attempts int
	Err      error
}

func (e *RetryExhaustedError) Error() string {
	return "retry exhausted after " + string(rune('0'+e.Attempts)) + " attempts: " + e.Err.Error()
}

func (e *RetryExhaustedError) Unwrap() error {
	return e.Err
}

// WithCircuitBreaker combines retry + circuit breaker
func WithCircuitBreaker(
	ctx context.Context,
	cb *CircuitBreaker,
	retryCfg RetryConfig,
	fn func(ctx context.Context) error,
) error {
	return RetryWithBackoff(ctx, retryCfg, func(ctx context.Context) error {
		return cb.Execute(ctx, fn)
	})
}
