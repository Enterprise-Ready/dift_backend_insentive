package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"coupon-service/internal/model"
)

// =============================================
// Rule Repository (Postgres)
// =============================================
// Stores and retrieves CouponRule records.
// Used by the Intelligence Engine as Postgres fallback
// when Redis cache misses.
// =============================================

type RuleRepository struct {
	db *sql.DB
}

func NewRuleRepository(db *sql.DB) *RuleRepository {
	return &RuleRepository{db: db}
}

// FindRulesByCodes fetches active rules for a batch of coupon codes.
// Uses IN clause for single round-trip.
func (r *RuleRepository) FindRulesByCodes(
	ctx context.Context,
	codes []string,
) ([]model.CouponRule, error) {
	if len(codes) == 0 {
		return nil, nil
	}

	// Build $1,$2,...,$N placeholder
	placeholders := make([]string, len(codes))
	args := make([]any, len(codes))
	for i, code := range codes {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = code
	}

	query := fmt.Sprintf(`
		SELECT
			id,
			coupon_code,
			priority,
			stack_group,
			stack_behavior,
			condition_group,
			active,
			valid_from,
			valid_to,
			created_at,
			updated_at
		FROM coupon_rules
		WHERE coupon_code IN (%s)
		  AND active = TRUE
	`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []model.CouponRule
	for rows.Next() {
		var rule model.CouponRule
		var conditionGroupJSON []byte

		if err := rows.Scan(
			&rule.ID,
			&rule.CouponCode,
			&rule.Priority,
			&rule.StackGroup,
			&rule.StackBehavior,
			&conditionGroupJSON,
			&rule.Active,
			&rule.ValidFrom,
			&rule.ValidTo,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(conditionGroupJSON, &rule.ConditionGroup); err != nil {
			return nil, fmt.Errorf("unmarshal condition_group for %s: %w", rule.CouponCode, err)
		}

		rules = append(rules, rule)
	}

	return rules, rows.Err()
}

// FindCouponsByCodes fetches coupon data for a batch of codes.
func (r *RuleRepository) FindCouponsByCodes(
	ctx context.Context,
	codes []string,
) ([]model.Coupon, error) {
	if len(codes) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(codes))
	args := make([]any, len(codes))
	for i, code := range codes {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = code
	}

	query := fmt.Sprintf(`
		SELECT
			code,
			discount_type,
			discount_value,
			min_order,
			max_discount,
			max_usage,
			used,
			valid_from,
			valid_to,
			active,
			created_at,
			updated_at
		FROM coupons
		WHERE code IN (%s)
		  AND active = TRUE
	`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var coupons []model.Coupon
	for rows.Next() {
		var c model.Coupon
		if err := rows.Scan(
			&c.Code,
			&c.DiscountType,
			&c.DiscountValue,
			&c.MinOrder,
			&c.MaxDiscount,
			&c.MaxUsage,
			&c.Used,
			&c.ValidFrom,
			&c.ValidTo,
			&c.Active,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		coupons = append(coupons, c)
	}

	return coupons, rows.Err()
}

// UpsertRule creates or updates a CouponRule.
func (r *RuleRepository) UpsertRule(ctx context.Context, rule model.CouponRule) error {
	condJSON, err := json.Marshal(rule.ConditionGroup)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO coupon_rules (
			coupon_code,
			priority,
			stack_group,
			stack_behavior,
			condition_group,
			active,
			valid_from,
			valid_to
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (coupon_code) WHERE active = TRUE
		DO UPDATE SET
			priority        = EXCLUDED.priority,
			stack_group     = EXCLUDED.stack_group,
			stack_behavior  = EXCLUDED.stack_behavior,
			condition_group = EXCLUDED.condition_group,
			valid_from      = EXCLUDED.valid_from,
			valid_to        = EXCLUDED.valid_to,
			updated_at      = NOW()
	`,
		rule.CouponCode,
		rule.Priority,
		rule.StackGroup,
		string(rule.StackBehavior),
		condJSON,
		rule.Active,
		rule.ValidFrom,
		rule.ValidTo,
	)
	return err
}

// DeactivateRule marks a rule as inactive.
func (r *RuleRepository) DeactivateRule(ctx context.Context, couponCode string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE coupon_rules
		SET active = FALSE, updated_at = NOW()
		WHERE coupon_code = $1
	`, couponCode)
	return err
}
