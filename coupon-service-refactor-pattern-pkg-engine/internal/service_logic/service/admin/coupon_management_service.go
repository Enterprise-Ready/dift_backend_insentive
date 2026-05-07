package admin

import (
	"context"
	"errors"
	"time"

	"coupon-service/internal/interface/repository"
	serviceport "coupon-service/internal/interface/service/admin"
	"coupon-service/internal/model"
	"coupon-service/pkg/metrics"
)

var (
	ErrInvalidCoupon      = errors.New("invalid coupon data")
	ErrCouponAlreadyExist = errors.New("coupon already exists")
	ErrCouponNotFound     = errors.New("coupon not found")
)

type CouponManagementService struct {
	repo       repository.CouponRepository
	outboxRepo repository.OutboxRepository
}

func NewCouponManagementService(
	repo repository.CouponRepository,
	outboxRepo repository.OutboxRepository,
) *CouponManagementService {
	return &CouponManagementService{
		repo:       repo,
		outboxRepo: outboxRepo,
	}
}

var _ serviceport.CouponAdminService = (*CouponManagementService)(nil)

//////////////////////////////////////////////////
// Validation
//////////////////////////////////////////////////

func validateNewCoupon(c model.Coupon) error {

	if c.Code == "" ||
		c.DiscountValue <= 0 ||
		c.MaxUsage <= 0 ||
		c.ValidTo.Before(c.ValidFrom) {
		return ErrInvalidCoupon
	}

	switch c.DiscountType {
	case model.DiscountPercent, model.DiscountFixed:
	default:
		return ErrInvalidCoupon
	}

	return nil
}

func validateUpdateCoupon(c model.Coupon) error {

	if c.DiscountValue <= 0 ||
		c.MaxUsage <= 0 ||
		c.ValidTo.Before(c.ValidFrom) {
		return ErrInvalidCoupon
	}

	switch c.DiscountType {
	case model.DiscountPercent, model.DiscountFixed:
	default:
		return ErrInvalidCoupon
	}

	return nil
}

//////////////////////////////////////////////////
// CreateCoupon
//////////////////////////////////////////////////

func (s *CouponManagementService) CreateCoupon(
	ctx context.Context,
	coupon model.Coupon,
) error {
	started := time.Now()
	status := "success"
	defer func() { metrics.RecordAdminCommand("create", status, started) }()

	if err := validateNewCoupon(coupon); err != nil {
		return err
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existing, err := s.repo.FindByCodeTx(ctx, tx, coupon.Code)
	if err != nil {
		return err
	}

	if existing != nil {
		return ErrCouponAlreadyExist
	}

	now := time.Now()

	coupon.Active = true
	coupon.Used = 0
	coupon.CreatedAt = now
	coupon.UpdatedAt = now

	if err := s.repo.SaveTx(ctx, tx, &coupon); err != nil {
		return err
	}

	event := model.CouponEvent{
		Type:          model.CouponCreated,
		CouponCode:    coupon.Code,
		DiscountType:  string(coupon.DiscountType),
		DiscountValue: coupon.DiscountValue,
		MinOrder:      coupon.MinOrder,
		MaxDiscount:   coupon.MaxDiscount,
		MaxUsage:      coupon.MaxUsage,
		ValidFrom:     coupon.ValidFrom,
		ValidTo:       coupon.ValidTo,
		Active:        coupon.Active,
		OccurredAt:    now,
	}

	if err := s.outboxRepo.InsertTx(
		ctx,
		tx,
		model.OutboxInsert{
			AggregateType: "coupon",
			AggregateID:   coupon.Code,
			EventType:     string(model.CouponCreated),
			Payload:       event,
		},
	); err != nil {
		return err
	}

	return tx.Commit()
}

//////////////////////////////////////////////////
// UpdateCoupon
//////////////////////////////////////////////////

func (s *CouponManagementService) UpdateCoupon(
	ctx context.Context,
	coupon model.Coupon,
) error {
	started := time.Now()
	status := "success"
	defer func() { metrics.RecordAdminCommand("update", status, started) }()

	if coupon.Code == "" {
		return ErrInvalidCoupon
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existing, err := s.repo.FindByCodeTx(ctx, tx, coupon.Code)
	if err != nil {
		return err
	}

	if existing == nil {
		return ErrCouponNotFound
	}

	if err := validateUpdateCoupon(coupon); err != nil {
		return err
	}

	now := time.Now()

	coupon.Used = existing.Used
	coupon.CreatedAt = existing.CreatedAt
	coupon.UpdatedAt = now

	if err := s.repo.SaveTx(ctx, tx, &coupon); err != nil {
		return err
	}

	event := model.CouponEvent{
		Type:          model.CouponUpdated,
		CouponCode:    coupon.Code,
		DiscountType:  string(coupon.DiscountType),
		DiscountValue: coupon.DiscountValue,
		MinOrder:      coupon.MinOrder,
		MaxDiscount:   coupon.MaxDiscount,
		MaxUsage:      coupon.MaxUsage,
		ValidFrom:     coupon.ValidFrom,
		ValidTo:       coupon.ValidTo,
		Active:        coupon.Active,
		OccurredAt:    now,
	}

	if err := s.outboxRepo.InsertTx(
		ctx,
		tx,
		model.OutboxInsert{
			AggregateType: "coupon",
			AggregateID:   coupon.Code,
			EventType:     string(model.CouponUpdated),
			Payload:       event,
		},
	); err != nil {
		return err
	}

	return tx.Commit()
}

//////////////////////////////////////////////////
// DeactivateCoupon
//////////////////////////////////////////////////

func (s *CouponManagementService) DeactivateCoupon(
	ctx context.Context,
	code string,
) error {
	started := time.Now()
	status := "success"
	defer func() { metrics.RecordAdminCommand("deactivate", status, started) }()

	if code == "" {
		return ErrInvalidCoupon
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existing, err := s.repo.FindByCodeTx(ctx, tx, code)
	if err != nil {
		return err
	}

	if existing == nil {
		return ErrCouponNotFound
	}

	if !existing.Active {
		return nil
	}

	if err := s.repo.DeactivateTx(ctx, tx, code); err != nil {
		return err
	}

	now := time.Now()

	event := model.CouponEvent{
		Type:       model.CouponDeactivated,
		CouponCode: code,
		Active:     false,
		OccurredAt: now,
	}

	if err := s.outboxRepo.InsertTx(
		ctx,
		tx,
		model.OutboxInsert{
			AggregateType: "coupon",
			AggregateID:   code,
			EventType:     string(model.CouponDeactivated),
			Payload:       event,
		},
	); err != nil {
		return err
	}

	return tx.Commit()
}
