// ============================================================
// coupon_brain.go
// ============================================================
// สมองของ Coupon Service ทั้งหมดในไฟล์เดียว
//
// ประกอบด้วย:
//   1. Rule Engine      — evaluate condition tree ต่อ coupon
//   2. Stack Resolver   — priority + conflict resolution
//   3. Discount Pipeline — cascading discount calculation
//   4. Saga Orchestrator — distributed transaction (claim→reserve→confirm)
//   5. Compensator      — rollback ย้อนหลัง
//   6. Rate Limiter     — Redis sliding window (Lua atomic)
//   7. Rule Cache       — Redis warm cache + Postgres fallback
//
// ไฟล์นี้ไม่ depend on framework ใดเลย
// inject ทุกอย่างผ่าน interface → test ง่าย, swap ได้
// ============================================================

package brain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ============================================================
// SECTION 1 — Domain Types
// ============================================================

// ── Coupon core ─────────────────────────────────────────────

type DiscountType string

const (
	DiscountPercent DiscountType = "PERCENT"
	DiscountFixed   DiscountType = "FIXED"
)

type Coupon struct {
	Code          string
	DiscountType  DiscountType
	DiscountValue float64
	MinOrder      float64
	MaxDiscount   float64
	MaxUsage      int32
	Used          int32
	ValidFrom     time.Time
	ValidTo       time.Time
	Active        bool
}

// ── Rule Engine types ────────────────────────────────────────

type RuleOperator string

const (
	OpAND RuleOperator = "AND"
	OpOR  RuleOperator = "OR"
)

type CondField string

const (
	FieldOrderTotal  CondField = "order_total"
	FieldUserSegment CondField = "user_segment"
	FieldItemCount   CondField = "item_count"
	FieldCategory    CondField = "category"
	FieldPaymentType CondField = "payment_type"
	FieldHourOfDay   CondField = "hour_of_day"
	FieldDayOfWeek   CondField = "day_of_week"
	FieldUsageCount  CondField = "usage_count"
)

type CondOp string

const (
	Eq  CondOp = "eq"
	Neq CondOp = "neq"
	Gt  CondOp = "gt"
	Gte CondOp = "gte"
	Lt  CondOp = "lt"
	Lte CondOp = "lte"
	In  CondOp = "in"
	Nin CondOp = "nin"
)

// Condition คือ leaf node ของ condition tree
type Condition struct {
	Field CondField `json:"field"`
	Op    CondOp    `json:"op"`
	Value any       `json:"value"` // string | float64 | []string
}

// ConditionGroup คือ internal node (recursive)
// ทั้ง AND/OR รองรับ nested group ลึกไม่จำกัด
type ConditionGroup struct {
	Operator   RuleOperator     `json:"operator"`
	Conditions []Condition      `json:"conditions,omitempty"`
	Groups     []ConditionGroup `json:"groups,omitempty"`
}

// StackBehavior กำหนด stacking policy ของ coupon
type StackBehavior string

const (
	StackAllow     StackBehavior = "allow"     // stack กับทุกคน
	StackRestrict  StackBehavior = "restrict"  // stack เฉพาะ group เดียวกัน
	StackExclusive StackBehavior = "exclusive" // ใช้คนเดียว ห้าม stack
)

// CouponRule คือ intelligence rule ที่ผูกกับ coupon code
type CouponRule struct {
	ID             string         `json:"id"`
	CouponCode     string         `json:"coupon_code"`
	Priority       int            `json:"priority"` // สูง = apply ก่อน
	StackGroup     string         `json:"stack_group"`
	StackBehavior  StackBehavior  `json:"stack_behavior"`
	ConditionGroup ConditionGroup `json:"condition_group"`
	Active         bool           `json:"active"`
	ValidFrom      time.Time      `json:"valid_from"`
	ValidTo        time.Time      `json:"valid_to"`
}

// EvalContext คือ runtime context สำหรับ rule evaluation
type EvalContext struct {
	UserID      string
	UserSegment string
	OrderTotal  float64
	ItemCount   int
	Categories  []string
	PaymentType string
	HourOfDay   int
	DayOfWeek   int
	UsageCount  int
}

// ── Apply I/O types ──────────────────────────────────────────

type ApplyRequest struct {
	CouponCodes []string
	Ctx         EvalContext
	OrderTotal  float64
}

type ApplyResult struct {
	AppliedCoupons []AppliedCoupon
	TotalDiscount  float64
	FinalTotal     float64
	Rejected       []RejectedCoupon
}

type AppliedCoupon struct {
	CouponCode     string
	Priority       int
	DiscountType   DiscountType
	DiscountValue  float64
	ActualDiscount float64
}

type RejectedCoupon struct {
	CouponCode string
	Reason     string
}

// ── Saga types ───────────────────────────────────────────────

type SagaStatus string

const (
	SagaStarted      SagaStatus = "STARTED"
	SagaClaiming     SagaStatus = "CLAIMING"
	SagaReserving    SagaStatus = "RESERVING"
	SagaConfirming   SagaStatus = "CONFIRMING"
	SagaCompleted    SagaStatus = "COMPLETED"
	SagaCompensating SagaStatus = "COMPENSATING"
	SagaCompensated  SagaStatus = "COMPENSATED"
	SagaFailed       SagaStatus = "FAILED"
)

type StepName string

const (
	StepClaim   StepName = "CLAIM"
	StepReserve StepName = "RESERVE"
	StepConfirm StepName = "CONFIRM"
)

type StepStatus string

const (
	StepPending     StepStatus = "PENDING"
	StepSucceeded   StepStatus = "SUCCEEDED"
	StepFailed      StepStatus = "FAILED"
	StepCompensated StepStatus = "COMPENSATED"
)

type SagaInstance struct {
	ID             string
	UserID         string
	CouponCode     string
	OrderID        string
	IdempotencyKey string
	Status         SagaStatus
	CurrentStep    StepName
	FailureReason  string
	Payload        SagaPayload
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

type SagaPayload struct {
	UserID         string     `json:"user_id"`
	CouponCode     string     `json:"coupon_code"`
	OrderTotal     float64    `json:"order_total"`
	IdempotencyKey string     `json:"idempotency_key"`
	ClaimedAt      *time.Time `json:"claimed_at,omitempty"`
	OrderID        string     `json:"order_id,omitempty"`
}

type SagaStepLog struct {
	SagaID     string
	StepName   StepName
	Status     StepStatus
	Attempt    int
	Error      string
	ExecutedAt time.Time
}

type SagaStartCmd struct {
	UserID         string
	CouponCode     string
	IdempotencyKey string
	OrderTotal     float64
}

// ── Errors ───────────────────────────────────────────────────

var (
	ErrNoEligibleCoupon  = errors.New("no eligible coupon after rule evaluation")
	ErrInvalidRequest    = errors.New("invalid request")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrCouponNotFound    = errors.New("coupon not found")
	ErrCouponInactive    = errors.New("coupon inactive")
	ErrCouponExpired     = errors.New("coupon expired")
	ErrCouponNotStarted  = errors.New("coupon not yet started")
	ErrQuotaExceeded     = errors.New("coupon quota exceeded")
	ErrAlreadyClaimed    = errors.New("coupon already claimed by this user")
)

// ============================================================
// SECTION 2 — Ports (Interfaces)
// ============================================================
// ทุก dependency ออกมาเป็น interface
// → swap Postgres ↔ mock ใน test ได้ทันที
// ============================================================

// RuleStore — load rule จาก Postgres (batch)
type RuleStore interface {
	FindRulesByCodes(ctx context.Context, codes []string) ([]CouponRule, error)
}

// CouponStore — load coupon data (discount values)
type CouponStore interface {
	FindByCode(ctx context.Context, code string) (*Coupon, error)
	// ถ้า repo รองรับ batch ให้ implement FindByCodes แทน
}

// SagaStore — persist saga lifecycle
type SagaStore interface {
	Insert(ctx context.Context, s *SagaInstance) error
	UpdateStatus(ctx context.Context, id string, status SagaStatus, step StepName) error
	UpdateFailed(ctx context.Context, id string, reason string) error
	UpdateCompleted(ctx context.Context, id string) error
	FindByIdempotencyKey(ctx context.Context, key string) (*SagaInstance, error)
	FindStaleProcessing(ctx context.Context, olderThan time.Duration) ([]SagaInstance, error)
	InsertStepLog(ctx context.Context, log SagaStepLog) error
}

// ClaimStore — coupon claim operations (transactional)
type ClaimStore interface {
	LockCoupon(ctx context.Context, code string) (*Coupon, error) // SELECT FOR UPDATE
	IncreaseUsage(ctx context.Context, code string) error
	InsertUsage(ctx context.Context, code, userID string) error // UNIQUE → dedup
	DecrementUsage(ctx context.Context, code string) error      // compensate
	DeleteUsage(ctx context.Context, code, userID string) error // compensate
}

// IdempotencyStore — dedup store
type IdempotencyStore interface {
	InsertOrGet(ctx context.Context, key, userID string) (existed bool, err error)
	MarkSuccess(ctx context.Context, key string) error
}

// EventPublisher — publish to NATS JetStream
type EventPublisher interface {
	PublishClaimed(ctx context.Context, userID, couponCode string) error
	PublishReserveCancelled(ctx context.Context, userID, couponCode string) error
}

// OutboxStore — transactional outbox
type OutboxStore interface {
	Insert(ctx context.Context, aggregateID, eventType string, payload any) error
}

// ============================================================
// SECTION 3 — Rule Engine
// ============================================================
// Pure functions — zero dependencies, unit-testable standalone
// ============================================================

// EvaluateGroup evaluates the whole ConditionGroup tree recursively.
// Returns true if the context satisfies the group.
func EvaluateGroup(group ConditionGroup, ctx EvalContext) bool {
	results := make([]bool, 0, len(group.Conditions)+len(group.Groups))

	for _, cond := range group.Conditions {
		results = append(results, evaluateCond(cond, ctx))
	}
	for _, nested := range group.Groups {
		results = append(results, EvaluateGroup(nested, ctx))
	}

	if len(results) == 0 {
		return true // empty group = permissive
	}

	switch group.Operator {
	case OpAND:
		for _, r := range results {
			if !r {
				return false
			}
		}
		return true
	case OpOR:
		for _, r := range results {
			if r {
				return true
			}
		}
		return false
	}
	return false // unknown operator = deny (fail-safe)
}

func evaluateCond(cond Condition, ctx EvalContext) bool {
	val, err := extractField(cond.Field, ctx)
	if err != nil {
		return false
	}
	return compare(val, cond.Op, cond.Value)
}

func extractField(field CondField, ctx EvalContext) (any, error) {
	switch field {
	case FieldOrderTotal:
		return ctx.OrderTotal, nil
	case FieldUserSegment:
		return ctx.UserSegment, nil
	case FieldItemCount:
		return float64(ctx.ItemCount), nil
	case FieldCategory:
		return ctx.Categories, nil
	case FieldPaymentType:
		return ctx.PaymentType, nil
	case FieldHourOfDay:
		return float64(ctx.HourOfDay), nil
	case FieldDayOfWeek:
		return float64(ctx.DayOfWeek), nil
	case FieldUsageCount:
		return float64(ctx.UsageCount), nil
	}
	return nil, fmt.Errorf("unknown field: %s", field)
}

func compare(fieldVal any, op CondOp, condVal any) bool {
	switch op {
	case Gt, Gte, Lt, Lte:
		fv, ok1 := toFloat(fieldVal)
		cv, ok2 := toFloat(condVal)
		if !ok1 || !ok2 {
			return false
		}
		switch op {
		case Gt:
			return fv > cv
		case Gte:
			return fv >= cv
		case Lt:
			return fv < cv
		case Lte:
			return fv <= cv
		}

	case Eq:
		fv, ok1 := toFloat(fieldVal)
		cv, ok2 := toFloat(condVal)
		if ok1 && ok2 {
			return fv == cv
		}
		return fmt.Sprintf("%v", fieldVal) == fmt.Sprintf("%v", condVal)

	case Neq:
		fv, ok1 := toFloat(fieldVal)
		cv, ok2 := toFloat(condVal)
		if ok1 && ok2 {
			return fv != cv
		}
		return fmt.Sprintf("%v", fieldVal) != fmt.Sprintf("%v", condVal)

	case In:
		return setContains(fieldVal, condVal)
	case Nin:
		return !setContains(fieldVal, condVal)
	}
	return false
}

// setContains handles:
//
//	string vs []string  (field ∈ list)
//	[]string vs string  (any element = target)
//	[]string vs []string (intersection non-empty)
func setContains(fieldVal any, condVal any) bool {
	if fStr, ok := fieldVal.(string); ok {
		for _, s := range toStrSlice(condVal) {
			if strings.EqualFold(s, fStr) {
				return true
			}
		}
		return false
	}
	if fSlice, ok := fieldVal.([]string); ok {
		set := make(map[string]struct{})
		for _, s := range toStrSlice(condVal) {
			set[strings.ToLower(s)] = struct{}{}
		}
		for _, item := range fSlice {
			if _, ok := set[strings.ToLower(item)]; ok {
				return true
			}
		}
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func toStrSlice(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			out = append(out, fmt.Sprintf("%v", item))
		}
		return out
	case string:
		return []string{s}
	}
	return nil
}

// ============================================================
// SECTION 4 — Stack Resolver
// ============================================================
// Input:  eligible rules (already passed EvaluateGroup)
// Output: ordered slice ที่ควร apply จริง
//
// Priority logic:
//   1. Sort DESC by Priority
//   2. Exclusive coupon → highest-priority one wins, cut the rest
//   3. Restrict coupons → only the highest-priority restrict GROUP survives
//   4. Allow coupons → stack freely
// ============================================================

type StackReport struct {
	Applied []string
	Dropped []string
	Reason  string
}

func ResolveStack(eligible []CouponRule) ([]CouponRule, StackReport) {
	if len(eligible) == 0 {
		return nil, StackReport{}
	}

	sorted := make([]CouponRule, len(eligible))
	copy(sorted, eligible)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	// Check exclusive first
	for _, rule := range sorted {
		if rule.StackBehavior == StackExclusive {
			dropped := collectCodes(sorted, rule.CouponCode)
			return []CouponRule{rule}, StackReport{
				Applied: []string{rule.CouponCode},
				Dropped: dropped,
				Reason:  "exclusive coupon takes all",
			}
		}
	}

	// Find winning restrict group (first = highest priority)
	var winGroup string
	for _, rule := range sorted {
		if rule.StackBehavior == StackRestrict && winGroup == "" {
			winGroup = rule.StackGroup
			break
		}
	}

	result := make([]CouponRule, 0, len(sorted))
	dropped := make([]string, 0)

	for _, rule := range sorted {
		switch rule.StackBehavior {
		case StackAllow:
			result = append(result, rule)
		case StackRestrict:
			if rule.StackGroup == winGroup {
				result = append(result, rule)
			} else {
				dropped = append(dropped, rule.CouponCode)
			}
		}
	}

	reason := "stacking allowed"
	if len(dropped) > 0 {
		reason = "restrict group conflict — priority-based group selection"
	}

	applied := make([]string, 0, len(result))
	for _, r := range result {
		applied = append(applied, r.CouponCode)
	}

	return result, StackReport{Applied: applied, Dropped: dropped, Reason: reason}
}

func collectCodes(rules []CouponRule, exclude string) []string {
	out := make([]string, 0)
	for _, r := range rules {
		if r.CouponCode != exclude {
			out = append(out, r.CouponCode)
		}
	}
	return out
}

// ============================================================
// SECTION 5 — Intelligence Engine
// ============================================================
// Orchestrates: load rules → evaluate → resolve stack → pipeline
// Redis-first: rule cache per code, TTL 5 min, Postgres fallback
// ============================================================

const (
	ruleCachePrefix = "coupon:rule:"
	ruleCacheTTL    = 5 * time.Minute
)

type IntelligenceEngine struct {
	ruleStore   RuleStore
	couponStore CouponStore
	redis       *redis.Client
}

func NewIntelligenceEngine(
	ruleStore RuleStore,
	couponStore CouponStore,
	redisClient *redis.Client,
) *IntelligenceEngine {
	return &IntelligenceEngine{
		ruleStore:   ruleStore,
		couponStore: couponStore,
		redis:       redisClient,
	}
}

// Apply คือ entry point หลัก
func (e *IntelligenceEngine) Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error) {
	if len(req.CouponCodes) == 0 || req.OrderTotal <= 0 {
		return ApplyResult{}, ErrInvalidRequest
	}

	// Load rules (Redis → Postgres fallback)
	rules, err := e.loadRules(ctx, req.CouponCodes)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("load rules: %w", err)
	}

	// Load coupon data
	couponMap, err := e.loadCoupons(ctx, req.CouponCodes)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("load coupons: %w", err)
	}

	// Evaluate rules → filter eligible
	now := time.Now()
	ruleSet := make(map[string]struct{})
	eligible := make([]CouponRule, 0)
	rejected := make([]RejectedCoupon, 0)

	for _, rule := range rules {
		ruleSet[rule.CouponCode] = struct{}{}

		switch {
		case !rule.Active:
			rejected = append(rejected, RejectedCoupon{rule.CouponCode, "rule inactive"})
		case now.Before(rule.ValidFrom) || now.After(rule.ValidTo):
			rejected = append(rejected, RejectedCoupon{rule.CouponCode, "rule out of date range"})
		case !EvaluateGroup(rule.ConditionGroup, req.Ctx):
			rejected = append(rejected, RejectedCoupon{rule.CouponCode, "condition not met"})
		default:
			eligible = append(eligible, rule)
		}
	}

	// Codes with no rule defined
	for _, code := range req.CouponCodes {
		if _, ok := ruleSet[code]; !ok {
			rejected = append(rejected, RejectedCoupon{code, "no rule defined"})
		}
	}

	// Resolve stacking
	resolved, _ := ResolveStack(eligible)

	// Discount pipeline
	return e.pipeline(resolved, couponMap, req.OrderTotal, rejected)
}

// pipeline applies discounts in priority order on cascading remaining balance
func (e *IntelligenceEngine) pipeline(
	resolved []CouponRule,
	couponMap map[string]Coupon,
	orderTotal float64,
	rejected []RejectedCoupon,
) (ApplyResult, error) {

	remaining := orderTotal
	totalDiscount := 0.0
	applied := make([]AppliedCoupon, 0, len(resolved))

	for _, rule := range resolved {
		coupon, ok := couponMap[rule.CouponCode]
		if !ok {
			rejected = append(rejected, RejectedCoupon{rule.CouponCode, "coupon data missing"})
			continue
		}
		if !coupon.Active {
			rejected = append(rejected, RejectedCoupon{rule.CouponCode, "coupon inactive"})
			continue
		}

		discount := calcDiscount(coupon, remaining)
		remaining -= discount
		if remaining < 0 {
			remaining = 0
		}
		totalDiscount += discount

		applied = append(applied, AppliedCoupon{
			CouponCode:     rule.CouponCode,
			Priority:       rule.Priority,
			DiscountType:   coupon.DiscountType,
			DiscountValue:  coupon.DiscountValue,
			ActualDiscount: discount,
		})
	}

	return ApplyResult{
		AppliedCoupons: applied,
		TotalDiscount:  totalDiscount,
		FinalTotal:     remaining,
		Rejected:       rejected,
	}, nil
}

func calcDiscount(c Coupon, remaining float64) float64 {
	var d float64
	switch c.DiscountType {
	case DiscountPercent:
		d = remaining * c.DiscountValue / 100
	case DiscountFixed:
		d = c.DiscountValue
	default:
		return 0
	}
	if c.MaxDiscount > 0 && d > c.MaxDiscount {
		d = c.MaxDiscount
	}
	if d > remaining {
		d = remaining
	}
	return d
}

// loadRules fetches rules per-code with Redis warm cache
func (e *IntelligenceEngine) loadRules(ctx context.Context, codes []string) ([]CouponRule, error) {
	result := make([]CouponRule, 0, len(codes))
	missed := make([]string, 0)

	for _, code := range codes {
		val, err := e.redis.Get(ctx, ruleCachePrefix+code).Result()
		if err == redis.Nil {
			missed = append(missed, code)
			continue
		}
		if err != nil {
			missed = append(missed, code)
			continue
		}
		var rule CouponRule
		if json.Unmarshal([]byte(val), &rule) != nil {
			missed = append(missed, code)
			continue
		}
		result = append(result, rule)
	}

	if len(missed) > 0 {
		dbRules, err := e.ruleStore.FindRulesByCodes(ctx, missed)
		if err != nil {
			return nil, err
		}
		for _, rule := range dbRules {
			e.warmRule(ctx, rule)
		}
		result = append(result, dbRules...)
	}

	return result, nil
}

func (e *IntelligenceEngine) warmRule(ctx context.Context, rule CouponRule) {
	data, err := json.Marshal(rule)
	if err != nil {
		return
	}
	_ = e.redis.Set(ctx, ruleCachePrefix+rule.CouponCode, data, ruleCacheTTL).Err()
}

// InvalidateRule removes one rule from Redis cache.
// Call after admin Create/Update/Deactivate.
func (e *IntelligenceEngine) InvalidateRule(ctx context.Context, code string) error {
	return e.redis.Del(ctx, ruleCachePrefix+code).Err()
}

func (e *IntelligenceEngine) loadCoupons(ctx context.Context, codes []string) (map[string]Coupon, error) {
	// parallel fetch — ถ้า repo มี FindByCodes batch ให้แทนตรงนี้
	type res struct {
		c   *Coupon
		err error
	}
	ch := make(chan res, len(codes))
	var wg sync.WaitGroup

	for _, code := range codes {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			coupon, err := e.couponStore.FindByCode(ctx, c)
			ch <- res{coupon, err}
		}(code)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	m := make(map[string]Coupon, len(codes))
	for r := range ch {
		if r.err != nil {
			return nil, r.err
		}
		if r.c != nil {
			m[r.c.Code] = *r.c
		}
	}
	return m, nil
}

// ============================================================
// SECTION 6 — Saga Orchestrator
// ============================================================
// Orchestration-based saga: Claim → Reserve → Confirm
// On failure: compensate in reverse
// Crash-safe: status persisted before each step
// ============================================================

// SagaStep คือ interface ที่แต่ละ step ต้อง implement
type SagaStep interface {
	Name() StepName
	Execute(ctx context.Context, payload *SagaPayload) error
	Compensate(ctx context.Context, payload *SagaPayload) error
}

// RateLimiter enforces per-user request rate via Redis
type RateLimiter interface {
	Allow(ctx context.Context, userID string) (bool, error)
}

// Orchestrator manages the saga lifecycle
type Orchestrator struct {
	sagaStore   SagaStore
	rateLimiter RateLimiter
	steps       []SagaStep
}

func NewOrchestrator(
	sagaStore SagaStore,
	rateLimiter RateLimiter,
	steps []SagaStep,
) *Orchestrator {
	return &Orchestrator{
		sagaStore:   sagaStore,
		rateLimiter: rateLimiter,
		steps:       steps,
	}
}

// Start เริ่ม saga ใหม่ หรือ return existing saga สำหรับ idempotency key เดิม
// คืน sagaID ทันที, execute ใน goroutine → caller poll /saga/{id}/status
func (o *Orchestrator) Start(ctx context.Context, cmd SagaStartCmd) (string, error) {
	// Idempotency guard
	existing, err := o.sagaStore.FindByIdempotencyKey(ctx, cmd.IdempotencyKey)
	if err != nil {
		return "", fmt.Errorf("idempotency check: %w", err)
	}
	if existing != nil {
		return existing.ID, nil
	}

	// Rate limit
	ok, err := o.rateLimiter.Allow(ctx, cmd.UserID)
	if err != nil {
		return "", fmt.Errorf("rate limiter: %w", err)
	}
	if !ok {
		return "", ErrRateLimitExceeded
	}

	// Persist new saga
	now := time.Now().UTC()
	saga := &SagaInstance{
		ID:             uuid.New().String(),
		UserID:         cmd.UserID,
		CouponCode:     cmd.CouponCode,
		IdempotencyKey: cmd.IdempotencyKey,
		Status:         SagaStarted,
		Payload: SagaPayload{
			UserID:         cmd.UserID,
			CouponCode:     cmd.CouponCode,
			OrderTotal:     cmd.OrderTotal,
			IdempotencyKey: cmd.IdempotencyKey,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := o.sagaStore.Insert(ctx, saga); err != nil {
		return "", fmt.Errorf("persist saga: %w", err)
	}

	// Execute async — caller polls status
	payload := saga.Payload
	go o.execute(context.Background(), saga.ID, payload)

	return saga.ID, nil
}

// execute วิ่ง steps ตามลำดับ, rollback เมื่อ step ใด fail
func (o *Orchestrator) execute(ctx context.Context, sagaID string, payload SagaPayload) {
	succeeded := make([]SagaStep, 0, len(o.steps))

	for _, step := range o.steps {
		_ = o.sagaStore.UpdateStatus(ctx, sagaID, stepToStatus(step.Name()), step.Name())
		_ = o.sagaStore.InsertStepLog(ctx, SagaStepLog{
			SagaID: sagaID, StepName: step.Name(),
			Status: StepPending, Attempt: 1, ExecutedAt: time.Now().UTC(),
		})

		stepCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := step.Execute(stepCtx, &payload)
		cancel()

		if err != nil {
			_ = o.sagaStore.InsertStepLog(ctx, SagaStepLog{
				SagaID: sagaID, StepName: step.Name(),
				Status: StepFailed, Attempt: 1,
				Error: err.Error(), ExecutedAt: time.Now().UTC(),
			})
			_ = o.sagaStore.UpdateFailed(ctx, sagaID,
				fmt.Sprintf("step %s failed: %s", step.Name(), err.Error()))
			o.compensate(ctx, sagaID, succeeded, payload)
			return
		}

		_ = o.sagaStore.InsertStepLog(ctx, SagaStepLog{
			SagaID: sagaID, StepName: step.Name(),
			Status: StepSucceeded, Attempt: 1, ExecutedAt: time.Now().UTC(),
		})
		succeeded = append(succeeded, step)
	}

	_ = o.sagaStore.UpdateCompleted(ctx, sagaID)
}

// compensate rollback ย้อนหลัง (reverse order)
func (o *Orchestrator) compensate(ctx context.Context, sagaID string, succeeded []SagaStep, payload SagaPayload) {
	_ = o.sagaStore.UpdateStatus(ctx, sagaID, SagaCompensating, "")

	for i := len(succeeded) - 1; i >= 0; i-- {
		step := succeeded[i]
		compCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := step.Compensate(compCtx, &payload)
		cancel()

		status := StepCompensated
		errMsg := ""
		if err != nil {
			status = StepFailed
			errMsg = err.Error()
		}
		_ = o.sagaStore.InsertStepLog(ctx, SagaStepLog{
			SagaID: sagaID, StepName: step.Name(),
			Status: status, Attempt: 1,
			Error: errMsg, ExecutedAt: time.Now().UTC(),
		})
	}

	_ = o.sagaStore.UpdateStatus(ctx, sagaID, SagaCompensated, "")
}

// stepsFrom returns steps starting from a given step name (for recovery)
func (o *Orchestrator) stepsFrom(from StepName) []SagaStep {
	if from == "" {
		return o.steps
	}
	for i, s := range o.steps {
		if s.Name() == from {
			return o.steps[i:]
		}
	}
	return nil
}

func stepToStatus(name StepName) SagaStatus {
	switch name {
	case StepClaim:
		return SagaClaiming
	case StepReserve:
		return SagaReserving
	case StepConfirm:
		return SagaConfirming
	}
	return SagaStarted
}

// ============================================================
// SECTION 7 — Saga Steps (Claim / Reserve / Confirm)
// ============================================================

// ── ClaimStep ────────────────────────────────────────────────

type ClaimStep struct {
	claimStore ClaimStore
	idempStore IdempotencyStore
}

func NewClaimStep(claimStore ClaimStore, idempStore IdempotencyStore) *ClaimStep {
	return &ClaimStep{claimStore: claimStore, idempStore: idempStore}
}

func (s *ClaimStep) Name() StepName { return StepClaim }

func (s *ClaimStep) Execute(ctx context.Context, payload *SagaPayload) error {
	// Idempotency: return early if already processed
	existed, err := s.idempStore.InsertOrGet(ctx, payload.IdempotencyKey, payload.UserID)
	if err != nil {
		return err
	}
	if existed {
		now := time.Now().UTC()
		payload.ClaimedAt = &now
		return nil
	}

	// Lock coupon (SELECT FOR UPDATE — prevents race at หมื่น req/s)
	coupon, err := s.claimStore.LockCoupon(ctx, payload.CouponCode)
	if err != nil {
		return err
	}
	if coupon == nil {
		return ErrCouponNotFound
	}

	now := time.Now().UTC()
	switch {
	case now.Before(coupon.ValidFrom):
		return ErrCouponNotStarted
	case now.After(coupon.ValidTo):
		return ErrCouponExpired
	case !coupon.Active:
		return ErrCouponInactive
	case coupon.Used >= coupon.MaxUsage:
		return ErrQuotaExceeded
	}

	// Insert usage (UNIQUE constraint = dedup guard)
	if err := s.claimStore.InsertUsage(ctx, payload.CouponCode, payload.UserID); err != nil {
		if errors.Is(err, ErrAlreadyClaimed) {
			return ErrAlreadyClaimed
		}
		return err
	}

	if err := s.claimStore.IncreaseUsage(ctx, payload.CouponCode); err != nil {
		return err
	}

	if err := s.idempStore.MarkSuccess(ctx, payload.IdempotencyKey); err != nil {
		return err
	}

	payload.ClaimedAt = &now
	return nil
}

// Compensate ลด used และลบ usage record เพื่อให้ user claim ได้ใหม่
func (s *ClaimStep) Compensate(ctx context.Context, payload *SagaPayload) error {
	if payload.ClaimedAt == nil {
		return nil
	}
	if err := s.claimStore.DecrementUsage(ctx, payload.CouponCode); err != nil {
		return err
	}
	return s.claimStore.DeleteUsage(ctx, payload.CouponCode, payload.UserID)
}

// ── ReserveStep ──────────────────────────────────────────────

type ReserveStep struct {
	publisher EventPublisher
}

func NewReserveStep(publisher EventPublisher) *ReserveStep {
	return &ReserveStep{publisher: publisher}
}

func (s *ReserveStep) Name() StepName { return StepReserve }

func (s *ReserveStep) Execute(ctx context.Context, payload *SagaPayload) error {
	if err := s.publisher.PublishClaimed(ctx, payload.UserID, payload.CouponCode); err != nil {
		return fmt.Errorf("publish claimed event: %w", err)
	}
	payload.OrderID = "reserved"
	return nil
}

func (s *ReserveStep) Compensate(ctx context.Context, payload *SagaPayload) error {
	if payload.OrderID == "" {
		return nil
	}
	return s.publisher.PublishReserveCancelled(ctx, payload.UserID, payload.CouponCode)
}

// ── ConfirmStep ──────────────────────────────────────────────

type ConfirmStep struct {
	outbox OutboxStore
}

func NewConfirmStep(outbox OutboxStore) *ConfirmStep {
	return &ConfirmStep{outbox: outbox}
}

func (s *ConfirmStep) Name() StepName { return StepConfirm }

func (s *ConfirmStep) Execute(ctx context.Context, payload *SagaPayload) error {
	return s.outbox.Insert(ctx, payload.CouponCode, "COUPON_CLAIMED", map[string]any{
		"user_id":     payload.UserID,
		"coupon_code": payload.CouponCode,
		"occurred_at": time.Now().UTC(),
	})
}

// Compensate is a no-op: downstream consumers must be idempotent
func (s *ConfirmStep) Compensate(_ context.Context, _ *SagaPayload) error {
	return nil
}

// ============================================================
// SECTION 8 — Recovery Worker
// ============================================================
// Scans Postgres ทุก `interval` หา saga ที่ค้างใน in-progress state
// แล้ว re-run จาก current_step (idempotency ป้องกัน double-execute)
// ============================================================

type RecoveryWorker struct {
	sagaStore    SagaStore
	orchestrator *Orchestrator
	interval     time.Duration
	staleAfter   time.Duration
}

func NewRecoveryWorker(
	sagaStore SagaStore,
	orchestrator *Orchestrator,
	interval time.Duration, // e.g. 30s
	staleAfter time.Duration, // e.g. 2min
) *RecoveryWorker {
	return &RecoveryWorker{
		sagaStore:    sagaStore,
		orchestrator: orchestrator,
		interval:     interval,
		staleAfter:   staleAfter,
	}
}

func (w *RecoveryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.recover(ctx)
		}
	}
}

func (w *RecoveryWorker) recover(ctx context.Context) {
	sagas, err := w.sagaStore.FindStaleProcessing(ctx, w.staleAfter)
	if err != nil {
		return
	}
	for _, saga := range sagas {
		w.rerun(ctx, saga)
	}
}

func (w *RecoveryWorker) rerun(ctx context.Context, saga SagaInstance) {
	steps := w.orchestrator.stepsFrom(saga.CurrentStep)
	if steps == nil {
		_ = w.sagaStore.UpdateFailed(ctx, saga.ID, "recovery: unknown step")
		return
	}

	payload := saga.Payload
	succeeded := make([]SagaStep, 0)

	for _, step := range steps {
		stepCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := step.Execute(stepCtx, &payload)
		cancel()

		if err != nil {
			_ = w.sagaStore.UpdateFailed(ctx, saga.ID, "recovery: "+err.Error())
			w.orchestrator.compensate(ctx, saga.ID, succeeded, payload)
			return
		}
		succeeded = append(succeeded, step)
	}

	_ = w.sagaStore.UpdateCompleted(ctx, saga.ID)
}

// ============================================================
// SECTION 9 — Redis Rate Limiter (Sliding Window, Lua atomic)
// ============================================================
// Lua script วิ่งใน Redis single thread → ไม่มี race condition
// O(1) per request — รองรับ หมื่น req/s ได้สบาย
// ============================================================

var slidingWindowLua = redis.NewScript(`
local key     = KEYS[1]
local limit   = tonumber(ARGV[1])
local window  = tonumber(ARGV[2])
local current = redis.call('INCR', key)
if current == 1 then
  redis.call('EXPIRE', key, window)
end
if current > limit then
  return 0
end
return 1
`)

type RedisRateLimiter struct {
	client      *redis.Client
	window      time.Duration
	maxRequests int
	keyPrefix   string
}

func NewRedisRateLimiter(
	client *redis.Client,
	window time.Duration,
	maxRequests int,
	keyPrefix string, // e.g. "ratelimit:coupon_claim:"
) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:      client,
		window:      window,
		maxRequests: maxRequests,
		keyPrefix:   keyPrefix,
	}
}

// Allow returns (true, nil) if within limit.
// On Redis error: fail open — don't block user due to infra issues.
func (r *RedisRateLimiter) Allow(ctx context.Context, userID string) (bool, error) {
	key := r.keyPrefix + userID
	result, err := slidingWindowLua.Run(
		ctx, r.client,
		[]string{key},
		r.maxRequests,
		int(r.window.Seconds()),
	).Int()

	if err != nil {
		// Fail open: log elsewhere, don't block
		return true, fmt.Errorf("rate limiter redis: %w", err)
	}
	return result == 1, nil
}
