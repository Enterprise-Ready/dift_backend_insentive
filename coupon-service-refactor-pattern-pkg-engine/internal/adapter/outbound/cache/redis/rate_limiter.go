package redisadapter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// =============================================
// Sliding Window Rate Limiter (Redis)
// =============================================
// Uses a Lua script for atomic check-and-increment.
// This prevents race conditions at high concurrency.
//
// Algorithm: Sliding window counter
//   - Key: "ratelimit:coupon_claim:{userID}"
//   - Value: integer counter
//   - TTL: window size
//   - On each request: INCR + check if > limit
//
// For หมื่น req/s: this runs purely in Redis memory
// with O(1) per request, no Postgres hit.
// =============================================

const (
	// DefaultWindow is the time window for rate limiting
	DefaultWindow = 60 * time.Second
	// DefaultMaxRequests is the max requests per user per window
	DefaultMaxRequests = 10
)

var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])

local current = redis.call('INCR', key)
if current == 1 then
  redis.call('EXPIRE', key, window)
end

if current > limit then
  return 0
end

return 1
`)

// CouponClaimRateLimiter is the Redis-backed rate limiter.
type CouponClaimRateLimiter struct {
	client      *redis.Client
	window      time.Duration
	maxRequests int
}

func NewCouponClaimRateLimiter(
	client *redis.Client,
	window time.Duration,
	maxRequests int,
) *CouponClaimRateLimiter {
	return &CouponClaimRateLimiter{
		client:      client,
		window:      window,
		maxRequests: maxRequests,
	}
}

// Allow returns true if the user is within the rate limit.
// Atomic via Lua — safe at high concurrency.
func (r *CouponClaimRateLimiter) Allow(ctx context.Context, userID string) (bool, error) {
	key := fmt.Sprintf("ratelimit:coupon_claim:%s", userID)
	windowSecs := int(r.window.Seconds())

	result, err := slidingWindowScript.Run(
		ctx,
		r.client,
		[]string{key},
		r.maxRequests,
		windowSecs,
	).Int()

	if err != nil {
		// Redis error: fail open (don't block user due to infra issue)
		return true, fmt.Errorf("rate limiter redis error: %w", err)
	}

	return result == 1, nil
}

// =============================================
// Rule Cache Warmer
// =============================================
// Proactively warms the intelligence engine's rule cache
// in Redis on service startup or after admin changes.
// =============================================

type RuleCacheWarmer struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRuleCacheWarmer(client *redis.Client, ttl time.Duration) *RuleCacheWarmer {
	return &RuleCacheWarmer{client: client, ttl: ttl}
}

// Set stores a pre-serialized rule JSON in Redis.
func (w *RuleCacheWarmer) Set(ctx context.Context, code string, ruleJSON []byte) error {
	key := fmt.Sprintf("coupon:rule:%s", code)
	return w.client.Set(ctx, key, ruleJSON, w.ttl).Err()
}

// Invalidate removes a rule from cache.
func (w *RuleCacheWarmer) Invalidate(ctx context.Context, code string) error {
	key := fmt.Sprintf("coupon:rule:%s", code)
	return w.client.Del(ctx, key).Err()
}
