package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter implements sliding window rate limiting using Redis
type Limiter struct {
	client redis.UniversalClient
}

type Result struct {
	Allowed    bool
	Remaining  int64
	ResetAt    time.Time
	RetryAfter time.Duration
}

func NewLimiter(client redis.UniversalClient) *Limiter {
	return &Limiter{client: client}
}

// Allow checks if a key is within the rate limit (sliding window)
func (l *Limiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (*Result, error) {
	now := time.Now()
	windowStart := now.Add(-window).UnixMilli()
	nowMs := now.UnixMilli()

	redisKey := fmt.Sprintf("rl:%s", key)

	pipe := l.client.Pipeline()
	// Remove old entries
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	// Count current
	countCmd := pipe.ZCard(ctx, redisKey)
	// Add new entry
	pipe.ZAdd(ctx, redisKey, redis.Z{Score: float64(nowMs), Member: nowMs})
	// Set expiry
	pipe.Expire(ctx, redisKey, window+time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}

	count := countCmd.Val()
	allowed := count < limit

	if !allowed {
		// Get the oldest entry to determine reset time
		oldest, err := l.client.ZRangeWithScores(ctx, redisKey, 0, 0).Result()
		resetAt := now.Add(window)
		retryAfter := window
		if err == nil && len(oldest) > 0 {
			oldestMs := int64(oldest[0].Score)
			resetAt = time.UnixMilli(oldestMs + window.Milliseconds())
			retryAfter = time.Until(resetAt)
			if retryAfter < 0 {
				retryAfter = 0
			}
		}

		return &Result{
			Allowed:    false,
			Remaining:  0,
			ResetAt:    resetAt,
			RetryAfter: retryAfter,
		}, nil
	}

	remaining := limit - count - 1
	if remaining < 0 {
		remaining = 0
	}

	return &Result{
		Allowed:   true,
		Remaining: remaining,
		ResetAt:   now.Add(window),
	}, nil
}

// AllowN checks if n tokens are available
func (l *Limiter) AllowN(ctx context.Context, key string, limit int64, window time.Duration, n int64) (*Result, error) {
	// Simplified - check current count
	redisKey := fmt.Sprintf("rl:%s", key)
	count, err := l.client.ZCard(ctx, redisKey).Result()
	if err != nil {
		return nil, err
	}
	if count+n > limit {
		return &Result{Allowed: false, Remaining: limit - count}, nil
	}
	return l.Allow(ctx, key, limit, window)
}
