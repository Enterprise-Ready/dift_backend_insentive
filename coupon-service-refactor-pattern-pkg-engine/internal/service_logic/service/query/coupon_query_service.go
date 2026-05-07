package query

import (
	"context"
	"time"

	"coupon-service/internal/interface/repository"
	serviceport "coupon-service/internal/interface/service/query"
	"coupon-service/internal/model"
	"coupon-service/pkg/metrics"
)

type CouponQueryService struct {
	repo repository.CouponRepository
}

func NewCouponQueryService(
	repo repository.CouponRepository,
) *CouponQueryService {
	return &CouponQueryService{
		repo: repo,
	}
}

var _ serviceport.CouponQueryService = (*CouponQueryService)(nil)

func (s *CouponQueryService) ListActiveCoupons(
	ctx context.Context,
) ([]model.Coupon, error) {
	started := time.Now()
	coupons, err := s.repo.FindAllActive(ctx)
	if err != nil {
		metrics.RecordQuery("error", started)
		return nil, err
	}
	metrics.RecordQuery("success", started)
	return coupons, nil
}
