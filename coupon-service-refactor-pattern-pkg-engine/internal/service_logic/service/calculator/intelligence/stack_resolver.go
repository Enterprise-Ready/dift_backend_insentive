package intelligence

import (
	"sort"

	"coupon-service/internal/model"
)

// =============================================
// Stack Resolver
// =============================================
// Resolves which coupons can be applied together
// given their StackBehavior and Priority.
//
// Rules:
//   1. Sort by Priority DESC (highest first)
//   2. If any coupon is Exclusive → only that coupon applies
//      (the highest-priority exclusive wins)
//   3. Restrict coupons only stack within the same StackGroup
//   4. Allow coupons can stack with anyone
//
// Returns an ordered slice of coupons to apply.
// =============================================

// ResolveStack takes eligible coupons (already passed rule evaluation)
// and returns the final ordered slice that should be applied.
func ResolveStack(eligible []model.CouponRule) []model.CouponRule {
	if len(eligible) == 0 {
		return nil
	}

	// Step 1: Sort by Priority DESC
	sorted := make([]model.CouponRule, len(eligible))
	copy(sorted, eligible)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	// Step 2: Check for exclusive coupons
	for _, rule := range sorted {
		if rule.StackBehavior == model.StackExclusive {
			// The highest-priority exclusive takes all
			return []model.CouponRule{rule}
		}
	}

	// Step 3: Resolve Restrict vs Allow
	// Strategy:
	//   - Collect all "allow" coupons freely
	//   - For "restrict" coupons: only one group is allowed.
	//     The group that has the highest-priority restrict coupon wins.
	//     All restrict coupons in that group are included.
	//     Restrict coupons from other groups are excluded.

	var allowedGroup string // winning restrict group
	for _, rule := range sorted {
		if rule.StackBehavior == model.StackRestrict && allowedGroup == "" {
			allowedGroup = rule.StackGroup
			break
		}
	}

	result := make([]model.CouponRule, 0, len(sorted))
	for _, rule := range sorted {
		switch rule.StackBehavior {
		case model.StackAllow:
			result = append(result, rule)
		case model.StackRestrict:
			if rule.StackGroup == allowedGroup {
				result = append(result, rule)
			}
			// silently drop coupons in other groups
		}
	}

	return result
}

// ConflictReport contains stack resolution metadata (for observability/logging)
type ConflictReport struct {
	TotalSubmitted int
	Applied        []string // coupon codes that were applied
	Dropped        []string // coupon codes that were dropped
	Reason         string
}

// ResolveStackWithReport is like ResolveStack but also returns a ConflictReport.
func ResolveStackWithReport(eligible []model.CouponRule) ([]model.CouponRule, ConflictReport) {
	if len(eligible) == 0 {
		return nil, ConflictReport{}
	}

	resolved := ResolveStack(eligible)

	appliedSet := make(map[string]struct{}, len(resolved))
	for _, r := range resolved {
		appliedSet[r.CouponCode] = struct{}{}
	}

	dropped := make([]string, 0)
	reason := "stacking allowed"
	for _, rule := range eligible {
		if _, ok := appliedSet[rule.CouponCode]; !ok {
			dropped = append(dropped, rule.CouponCode)
		}
	}

	if len(dropped) > 0 {
		// Determine reason
		for _, rule := range eligible {
			if rule.StackBehavior == model.StackExclusive {
				if _, ok := appliedSet[rule.CouponCode]; ok {
					reason = "exclusive coupon took precedence"
					break
				}
			}
		}
		if reason == "stacking allowed" {
			reason = "restrict group conflict resolved by priority"
		}
	}

	applied := make([]string, 0, len(resolved))
	for _, r := range resolved {
		applied = append(applied, r.CouponCode)
	}

	return resolved, ConflictReport{
		TotalSubmitted: len(eligible),
		Applied:        applied,
		Dropped:        dropped,
		Reason:         reason,
	}
}
