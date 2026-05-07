package intelligence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"coupon-service/internal/interface/repository"
	"coupon-service/internal/model"

	"github.com/redis/go-redis/v9"
)

// =============================================
// Coupon Intelligence Engine
// =============================================
// Orchestrates:
//   1. Load CouponRule from Redis (warm cache)
//      → fallback to Postgres on miss
//   2. Evaluate ConditionGroup per coupon
//   3. Resolve stacking via StackResolver
//   4. Calculate discount pipeline
//
// Thread-safe. Stateless (all state in Redis / Postgres).
// =============================================

const (
	ruleCachePrefix = "coupon:rule:"
	ruleCacheTTL    = 5 * time.Minute
)

var (
	ErrNoEligibleCoupon = errors.New("no eligible coupon after rule evaluation")
	ErrInvalidRequest   = errors.New("invalid apply request")
)

// RuleRepository is the Postgres-side rule storage interface.
type RuleRepository interface {
	FindRulesByCodes(ctx context.Context, codes []string) ([]model.CouponRule, error)
	FindCouponsByCodes(ctx context.Context, codes []string) ([]model.Coupon, error)
}

// CouponIntelligenceEngine is the main engine.
type CouponIntelligenceEngine struct {
	ruleRepo    RuleRepository
	couponRepo  repository.CouponRepository
	redisClient *redis.Client
}

// NewCouponIntelligenceEngine creates a new engine.
func NewCouponIntelligenceEngine(
	ruleRepo RuleRepository,
	couponRepo repository.CouponRepository,
	redisClient *redis.Client,
) *CouponIntelligenceEngine {
	return &CouponIntelligenceEngine{
		ruleRepo:    ruleRepo,
		couponRepo:  couponRepo,
		redisClient: redisClient,
	}
}

// Apply is the main entry point.
// It evaluates all submitted coupon codes against the rule engine,
// resolves stacking, and returns the discount pipeline result.
func (e *CouponIntelligenceEngine) Apply(
	ctx context.Context,
	req model.ApplyRequest,
) (model.ApplyResult, error) {

	if len(req.CouponCodes) == 0 || req.OrderTotal <= 0 {
		return model.ApplyResult{}, ErrInvalidRequest
	}

	// ── Step 1: Load rules (Redis → Postgres fallback) ────────────────
	rules, err := e.loadRules(ctx, req.CouponCodes)
	if err != nil {
		return model.ApplyResult{}, fmt.Errorf("load rules: %w", err)
	}

	// ── Step 2: Load coupon data (for discount values) ────────────────
	coupons, err := e.loadCoupons(ctx, req.CouponCodes)
	if err != nil {
		return model.ApplyResult{}, fmt.Errorf("load coupons: %w", err)
	}
	couponMap := indexCoupons(coupons)

	// ── Step 3: Evaluate rules → filter eligible ──────────────────────
	now := time.Now()
	eligible := make([]model.CouponRule, 0, len(rules))
	rejected := make([]model.RejectedCoupon, 0)

	for _, rule := range rules {
		if !rule.Active {
			rejected = append(rejected, model.RejectedCoupon{
				CouponCode: rule.CouponCode,
				Reason:     "rule inactive",
			})
			continue
		}
		if now.Before(rule.ValidFrom) || now.After(rule.ValidTo) {
			rejected = append(rejected, model.RejectedCoupon{
				CouponCode: rule.CouponCode,
				Reason:     "rule expired or not started",
			})
			continue
		}

		if EvaluateGroup(rule.ConditionGroup, req.Ctx) {
			eligible = append(eligible, rule)
		} else {
			rejected = append(rejected, model.RejectedCoupon{
				CouponCode: rule.CouponCode,
				Reason:     "condition not met",
			})
		}
	}

	// Also reject codes that had no rule defined
	ruleSet := make(map[string]struct{}, len(rules))
	for _, r := range rules {
		ruleSet[r.CouponCode] = struct{}{}
	}
	for _, code := range req.CouponCodes {
		if _, ok := ruleSet[code]; !ok {
			rejected = append(rejected, model.RejectedCoupon{
				CouponCode: code,
				Reason:     "no rule defined",
			})
		}
	}

	// ── Step 4: Resolve stacking ──────────────────────────────────────
	resolved, _ := ResolveStackWithReport(eligible)

	// ── Step 5: Discount pipeline ─────────────────────────────────────
	return e.calculatePipeline(resolved, couponMap, req.OrderTotal, rejected)
}

// ── Step 5 implementation ─────────────────────────────────────────────

// calculatePipeline applies discounts sequentially (in priority order).
// Each coupon's discount is calculated on the REMAINING total after
// previous discounts — this is the "cascading" model used by major e-commerce.
func (e *CouponIntelligenceEngine) calculatePipeline(
	resolved []model.CouponRule,
	couponMap map[string]model.Coupon,
	orderTotal float64,
	rejected []model.RejectedCoupon,
) (model.ApplyResult, error) {

	remaining := orderTotal
	totalDiscount := 0.0
	applied := make([]model.AppliedCoupon, 0, len(resolved))

	for _, rule := range resolved {
		coupon, ok := couponMap[rule.CouponCode]
		if !ok {
			rejected = append(rejected, model.RejectedCoupon{
				CouponCode: rule.CouponCode,
				Reason:     "coupon data not found",
			})
			continue
		}
		if !coupon.Active {
			rejected = append(rejected, model.RejectedCoupon{
				CouponCode: rule.CouponCode,
				Reason:     "coupon inactive",
			})
			continue
		}

		discount := calcDiscount(coupon, remaining)

		remaining -= discount
		if remaining < 0 {
			remaining = 0
		}
		totalDiscount += discount

		applied = append(applied, model.AppliedCoupon{
			CouponCode:     rule.CouponCode,
			Priority:       rule.Priority,
			DiscountType:   coupon.DiscountType,
			DiscountValue:  coupon.DiscountValue,
			ActualDiscount: discount,
		})
	}

	return model.ApplyResult{
		AppliedCoupons: applied,
		TotalDiscount:  totalDiscount,
		FinalTotal:     remaining,
		Rejected:       rejected,
	}, nil
}

func calcDiscount(c model.Coupon, remaining float64) float64 {
	var discount float64
	switch c.DiscountType {
	case model.DiscountPercent:
		discount = remaining * c.DiscountValue / 100
	case model.DiscountFixed:
		discount = c.DiscountValue
	default:
		return 0
	}
	// Apply MaxDiscount cap
	if c.MaxDiscount > 0 && discount > c.MaxDiscount {
		discount = c.MaxDiscount
	}
	// Cannot discount more than remaining
	if discount > remaining {
		discount = remaining
	}
	return discount
}

// ── Redis cache helpers ────────────────────────────────────────────────

// loadRules loads CouponRules from Redis (warm cache) with Postgres fallback.
// Uses per-code caching to avoid thundering herd.
func (e *CouponIntelligenceEngine) loadRules(
	ctx context.Context,
	codes []string,
) ([]model.CouponRule, error) {

	result := make([]model.CouponRule, 0, len(codes))
	missedCodes := make([]string, 0)

	for _, code := range codes {
		key := ruleCachePrefix + code
		val, err := e.redisClient.Get(ctx, key).Result()
		if err == redis.Nil {
			missedCodes = append(missedCodes, code)
			continue
		}
		if err != nil {
			// Redis error → treat as cache miss, don't fail
			missedCodes = append(missedCodes, code)
			continue
		}

		var rule model.CouponRule
		if err := json.Unmarshal([]byte(val), &rule); err != nil {
			missedCodes = append(missedCodes, code)
			continue
		}
		result = append(result, rule)
	}

	// Fetch misses from Postgres
	if len(missedCodes) > 0 {
		dbRules, err := e.ruleRepo.FindRulesByCodes(ctx, missedCodes)
		if err != nil {
			return nil, err
		}
		// Warm the cache
		for _, rule := range dbRules {
			e.cacheRule(ctx, rule)
		}
		result = append(result, dbRules...)
	}

	return result, nil
}

// cacheRule writes a rule into Redis. Fire-and-forget (errors logged but not returned).
func (e *CouponIntelligenceEngine) cacheRule(ctx context.Context, rule model.CouponRule) {
	data, err := json.Marshal(rule)
	if err != nil {
		return
	}
	key := ruleCachePrefix + rule.CouponCode
	_ = e.redisClient.Set(ctx, key, data, ruleCacheTTL).Err()
}

// InvalidateRuleCache removes a rule from Redis cache.
// Call this from admin service after Create/Update/Deactivate.
func (e *CouponIntelligenceEngine) InvalidateRuleCache(ctx context.Context, code string) error {
	return e.redisClient.Del(ctx, ruleCachePrefix+code).Err()
}

// loadCoupons fetches coupon data (discount values etc.) from Postgres.
func (e *CouponIntelligenceEngine) loadCoupons(
	ctx context.Context,
	codes []string,
) ([]model.Coupon, error) {
	// Use FindAllActive equivalent — here we call FindByCode in parallel
	// In production, add a FindByCodes([]string) batch method to CouponRepository
	coupons := make([]model.Coupon, 0, len(codes))
	for _, code := range codes {
		c, err := e.couponRepo.FindByCode(ctx, code)
		if err != nil {
			return nil, err
		}
		if c != nil {
			coupons = append(coupons, *c)
		}
	}
	return coupons, nil
}

func indexCoupons(coupons []model.Coupon) map[string]model.Coupon {
	m := make(map[string]model.Coupon, len(coupons))
	for _, c := range coupons {
		m[c.Code] = c
	}
	return m
}
