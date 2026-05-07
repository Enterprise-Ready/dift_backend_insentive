package intelligence

import (
	"fmt"
	"strings"

	"coupon-service/internal/model"
)

// =============================================
// Rule Evaluator
// =============================================
// Evaluates a ConditionGroup tree against an
// EvaluationContext. Pure function — no side effects.
//
// Design:
//   - Recursive tree walk (AND / OR short-circuit)
//   - extractField() maps ConditionField → runtime value
//   - compare() handles typed comparison safely
// =============================================

// EvaluateGroup evaluates the entire ConditionGroup tree.
// Returns true if the context satisfies all conditions.
func EvaluateGroup(group model.ConditionGroup, ctx model.EvaluationContext) bool {
	results := make([]bool, 0, len(group.Conditions)+len(group.Groups))

	// Leaf conditions
	for _, cond := range group.Conditions {
		results = append(results, evaluateCondition(cond, ctx))
	}

	// Nested groups (recursive)
	for _, nested := range group.Groups {
		results = append(results, EvaluateGroup(nested, ctx))
	}

	if len(results) == 0 {
		// Empty group = always true (permissive by default)
		return true
	}

	switch group.Operator {
	case model.RuleOperatorAND:
		for _, r := range results {
			if !r {
				return false // short-circuit
			}
		}
		return true

	case model.RuleOperatorOR:
		for _, r := range results {
			if r {
				return true // short-circuit
			}
		}
		return false

	default:
		// Unknown operator = deny (fail-safe)
		return false
	}
}

// evaluateCondition evaluates a single leaf condition.
func evaluateCondition(cond model.Condition, ctx model.EvaluationContext) bool {
	fieldVal, err := extractField(cond.Field, ctx)
	if err != nil {
		// Field not found = condition fails
		return false
	}
	return compare(fieldVal, cond.Operator, cond.Value)
}

// extractField maps a ConditionField to the runtime value from context.
func extractField(field model.ConditionField, ctx model.EvaluationContext) (any, error) {
	switch field {
	case model.FieldOrderTotal:
		return ctx.OrderTotal, nil
	case model.FieldUserSegment:
		return ctx.UserSegment, nil
	case model.FieldItemCount:
		return float64(ctx.ItemCount), nil
	case model.FieldCategory:
		return ctx.Categories, nil
	case model.FieldPaymentType:
		return ctx.PaymentType, nil
	case model.FieldHourOfDay:
		return float64(ctx.HourOfDay), nil
	case model.FieldDayOfWeek:
		return float64(ctx.DayOfWeek), nil
	case model.FieldUsageCount:
		return float64(ctx.UsageCount), nil
	default:
		return nil, fmt.Errorf("unknown field: %s", field)
	}
}

// compare compares a runtime value against a condition value using the given operator.
// Supports numeric, string, and slice (in/nin) comparisons.
func compare(fieldVal any, op model.ConditionOperator, condVal any) bool {
	switch op {

	// ── Numeric comparisons ──────────────────────────────────────────
	case model.CondOpGt, model.CondOpGte, model.CondOpLt, model.CondOpLte:
		fv, ok1 := toFloat64(fieldVal)
		cv, ok2 := toFloat64(condVal)
		if !ok1 || !ok2 {
			return false
		}
		switch op {
		case model.CondOpGt:
			return fv > cv
		case model.CondOpGte:
			return fv >= cv
		case model.CondOpLt:
			return fv < cv
		case model.CondOpLte:
			return fv <= cv
		}

	// ── Equality ─────────────────────────────────────────────────────
	case model.CondOpEq:
		// Try numeric first, fall back to string
		fv, ok1 := toFloat64(fieldVal)
		cv, ok2 := toFloat64(condVal)
		if ok1 && ok2 {
			return fv == cv
		}
		return fmt.Sprintf("%v", fieldVal) == fmt.Sprintf("%v", condVal)

	case model.CondOpNeq:
		fv, ok1 := toFloat64(fieldVal)
		cv, ok2 := toFloat64(condVal)
		if ok1 && ok2 {
			return fv != cv
		}
		return fmt.Sprintf("%v", fieldVal) != fmt.Sprintf("%v", condVal)

	// ── Set membership ───────────────────────────────────────────────
	case model.CondOpIn:
		return setContains(fieldVal, condVal)

	case model.CondOpNin:
		return !setContains(fieldVal, condVal)
	}

	return false
}

// setContains checks if fieldVal is in condVal (a list).
// Supports:
//   - string field vs []string condVal
//   - []string field (categories) vs string condVal (any element matches)
//   - []string field vs []string condVal (intersection)
func setContains(fieldVal any, condVal any) bool {
	// Case: field is a single string → check if condVal list contains it
	if fStr, ok := fieldVal.(string); ok {
		list := toStringSlice(condVal)
		for _, s := range list {
			if strings.EqualFold(s, fStr) {
				return true
			}
		}
		return false
	}

	// Case: field is a []string (e.g. categories) → check intersection
	if fSlice, ok := fieldVal.([]string); ok {
		condList := toStringSlice(condVal)
		condSet := make(map[string]struct{}, len(condList))
		for _, s := range condList {
			condSet[strings.ToLower(s)] = struct{}{}
		}
		for _, item := range fSlice {
			if _, exists := condSet[strings.ToLower(item)]; exists {
				return true
			}
		}
		return false
	}

	return false
}

// ── Helpers ───────────────────────────────────────────────────────────

func toFloat64(v any) (float64, bool) {
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
	case uint:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}

func toStringSlice(v any) []string {
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
