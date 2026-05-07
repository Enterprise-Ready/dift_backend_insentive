package calculator

import (
	"context"
	"errors"
	"time"

	"coupon-service/internal/interface/repository"
	serviceport "coupon-service/internal/interface/service/calculator"
	"coupon-service/internal/model"
	"coupon-service/pkg/metrics"
)

var (
	ErrInvalidCoupon = errors.New("invalid coupon")
	ErrCouponExpired = errors.New("coupon expired")
	ErrOrderTooSmall = errors.New("order too small")
	ErrQuotaExceeded = errors.New("coupon quota exceeded")
)

type CouponCalculatorService struct {
	repo repository.CouponRepository
}

func NewCouponCalculatorService(r repository.CouponRepository) *CouponCalculatorService {
	return &CouponCalculatorService{repo: r}
}

var _ serviceport.CouponCalculatorService = (*CouponCalculatorService)(nil)

func (s *CouponCalculatorService) ApplyCoupon(
	ctx context.Context,
	cmd model.ApplyCouponCommand,
) (model.ApplyCouponResult, error) {
	started := time.Now()
	status := "success"
	defer func() { metrics.RecordCalculation(status, started) }()

	// Basic validation
	if cmd.CouponCode == "" || cmd.OrderTotal <= 0 {
		status = "invalid"
		return model.ApplyCouponResult{}, ErrInvalidCoupon
	}

	// Load coupon
	c, err := s.repo.FindByCode(ctx, cmd.CouponCode)
	if err != nil {
		return model.ApplyCouponResult{}, err
	}
	if c == nil || !c.Active {
		return model.ApplyCouponResult{}, ErrInvalidCoupon
	}

	now := time.Now()

	// Date validation
	if now.Before(c.ValidFrom) || now.After(c.ValidTo) {
		status = "expired"
		return model.ApplyCouponResult{}, ErrCouponExpired
	}

	// Quota validation (ถ้า MaxUsage > 0 แปลว่าจำกัด)
	if c.MaxUsage > 0 && c.Used >= c.MaxUsage {
		status = "quota_exceeded"
		return model.ApplyCouponResult{}, ErrQuotaExceeded
	}

	// Minimum order validation
	if cmd.OrderTotal < c.MinOrder {
		status = "order_too_small"
		return model.ApplyCouponResult{}, ErrOrderTooSmall
	}

	// Calculate discount
	var discount float64

	switch c.DiscountType {
	case model.DiscountPercent:
		discount = cmd.OrderTotal * c.DiscountValue / 100

	case model.DiscountFixed:
		discount = c.DiscountValue

	default:
		return model.ApplyCouponResult{}, ErrInvalidCoupon
	}

	// Apply max discount cap (ถ้า > 0)
	if c.MaxDiscount > 0 && discount > c.MaxDiscount {
		discount = c.MaxDiscount
	}

	final := cmd.OrderTotal - discount
	if final < 0 {
		final = 0
	}

	return model.ApplyCouponResult{
		FinalTotal: final,
		Discount:   discount,
		Valid:      true,
		Message:    "success",
	}, nil
}
