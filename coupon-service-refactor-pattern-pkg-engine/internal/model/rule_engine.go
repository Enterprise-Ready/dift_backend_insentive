package model

import "time"

// =============================================
// Rule Engine — Core Domain Models
// =============================================
// ออกแบบให้เป็น composable rule tree
// แต่ละ Rule มี Conditions และ Actions
// รองรับ Priority-based stacking
// =============================================

// RuleOperator คือ logical operator สำหรับ ConditionGroup
type RuleOperator string

const (
	RuleOperatorAND RuleOperator = "AND"
	RuleOperatorOR  RuleOperator = "OR"
)

// ConditionField คือ field ที่ใช้ evaluate
type ConditionField string

const (
	FieldOrderTotal  ConditionField = "order_total"
	FieldUserSegment ConditionField = "user_segment"
	FieldItemCount   ConditionField = "item_count"
	FieldCategory    ConditionField = "category"
	FieldPaymentType ConditionField = "payment_type"
	FieldHourOfDay   ConditionField = "hour_of_day"
	FieldDayOfWeek   ConditionField = "day_of_week"
	FieldUsageCount  ConditionField = "usage_count"
)

// ConditionOperator คือ comparison operator
type ConditionOperator string

const (
	CondOpEq  ConditionOperator = "eq"
	CondOpNeq ConditionOperator = "neq"
	CondOpGt  ConditionOperator = "gt"
	CondOpGte ConditionOperator = "gte"
	CondOpLt  ConditionOperator = "lt"
	CondOpLte ConditionOperator = "lte"
	CondOpIn  ConditionOperator = "in"
	CondOpNin ConditionOperator = "nin"
)

// Condition คือ single condition leaf
type Condition struct {
	Field    ConditionField    `json:"field"`
	Operator ConditionOperator `json:"operator"`
	// Value รองรับ string, float64, []string ผ่าน JSON any
	Value any `json:"value"`
}

// ConditionGroup คือ logical group ของ conditions
// รองรับ nested group (recursive)
type ConditionGroup struct {
	Operator   RuleOperator     `json:"operator"`
	Conditions []Condition      `json:"conditions,omitempty"`
	Groups     []ConditionGroup `json:"groups,omitempty"`
}

// StackBehavior กำหนดว่า coupon นี้ stack กับคนอื่นได้ไหม
type StackBehavior string

const (
	StackAllow     StackBehavior = "allow"     // stack ได้กับทุก coupon
	StackRestrict  StackBehavior = "restrict"  // stack ได้เฉพาะ group เดียวกัน
	StackExclusive StackBehavior = "exclusive" // ใช้คนเดียว ห้าม stack
)

// CouponRule คือ intelligence rule ที่ผูกกับ coupon code
type CouponRule struct {
	ID             string         `json:"id"`
	CouponCode     string         `json:"coupon_code"`
	Priority       int            `json:"priority"`    // ยิ่งสูง ยิ่ง apply ก่อน
	StackGroup     string         `json:"stack_group"` // coupon ใน group เดียวกัน stack กันได้
	StackBehavior  StackBehavior  `json:"stack_behavior"`
	ConditionGroup ConditionGroup `json:"condition_group"`
	Active         bool           `json:"active"`
	ValidFrom      time.Time      `json:"valid_from"`
	ValidTo        time.Time      `json:"valid_to"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// EvaluationContext คือ runtime context ที่ส่งเข้า rule engine
// ทุก field ที่ Condition อ้างถึงต้องอยู่ที่นี่
type EvaluationContext struct {
	UserID      string
	UserSegment string // "vip", "new", "regular"
	OrderTotal  float64
	ItemCount   int
	Categories  []string // category ของสินค้าใน cart
	PaymentType string   // "credit_card", "wallet", "cod"
	HourOfDay   int      // 0-23
	DayOfWeek   int      // 0=Sunday, 6=Saturday
	UsageCount  int      // จำนวนครั้งที่ user ใช้ coupon นี้แล้ว
}

// ApplyRequest คือ input ของ Intelligence Engine
type ApplyRequest struct {
	CouponCodes []string // อาจส่งมาหลาย code เพื่อ stacking
	Ctx         EvaluationContext
	OrderTotal  float64
}

// ApplyResult คือ output หลัง engine process เสร็จ
type ApplyResult struct {
	AppliedCoupons []AppliedCoupon
	TotalDiscount  float64
	FinalTotal     float64
	Rejected       []RejectedCoupon
}

// AppliedCoupon คือ coupon ที่ผ่าน rule engine
type AppliedCoupon struct {
	CouponCode     string
	Priority       int
	DiscountType   DiscountType
	DiscountValue  float64
	ActualDiscount float64 // discount จริงหลังคำนวณ (cap ด้วย MaxDiscount)
}

// RejectedCoupon คือ coupon ที่ไม่ผ่าน rule
type RejectedCoupon struct {
	CouponCode string
	Reason     string
}
