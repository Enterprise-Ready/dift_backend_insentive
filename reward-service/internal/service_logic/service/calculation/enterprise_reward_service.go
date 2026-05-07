package calculation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	port "reward-service/internal/interface/earn_reward"
	repoPort "reward-service/internal/interface/repository"
	rulePort "reward-service/internal/interface/service"
	serviceport "reward-service/internal/interface/service/calculation"
	"reward-service/internal/model"
	obs "reward-service/pkg/observability"
	"reward-service/pkg/resilience"

	"github.com/google/uuid"
)

// enterpriseRewardCalculationService wraps the base service with
// deduplication, metrics, structured logging, and circuit breaker protection.
type enterpriseRewardCalculationService struct {
	txRepo  repoPort.EarnTransactionRepository
	rule    rulePort.RewardRule
	out     port.RewardEarnProducerPort
	metrics *obs.Metrics
	logger  *obs.Logger
	cb      *resilience.CircuitBreaker
}

func NewEnterpriseRewardCalculationService(
	txRepo repoPort.EarnTransactionRepository,
	rule rulePort.RewardRule,
	out port.RewardEarnProducerPort,
	metrics *obs.Metrics,
	logger *obs.Logger,
	cb *resilience.CircuitBreaker,
) serviceport.RewardCalculationService {
	return &enterpriseRewardCalculationService{
		txRepo:  txRepo,
		rule:    rule,
		out:     out,
		metrics: metrics,
		logger:  logger,
		cb:      cb,
	}
}

var _ serviceport.RewardCalculationService = (*enterpriseRewardCalculationService)(nil)

func (s *enterpriseRewardCalculationService) HandleEarn(
	ctx context.Context,
	earn model.Earn,
) error {
	op := s.logger.StartOp(ctx, "handle_earn",
		"source", earn.Source,
		"user_id", earn.UserID,
		"ref_id", earn.RefID,
	)

	err := s.handleEarnInternal(ctx, earn)
	op.End(err, "source", earn.Source)

	if err != nil {
		s.metrics.EarnProcessedTotal.WithLabelValues(earn.Source + "_error").Inc()
	}
	return err
}

func (s *enterpriseRewardCalculationService) handleEarnInternal(
	ctx context.Context,
	earn model.Earn,
) error {
	// ─── 1. Input validation ─────────────────────────────────────────────────
	if err := validateEarn(earn); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// ─── 2. Idempotency check (by RefID) ─────────────────────────────────────
	// RefID is the external event ID (trip_id / order_id) — guaranteed unique
	// from upstream services. This is our idempotency anchor.
	exists, err := s.txRepo.ExistsByRefID(ctx, earn.RefID)
	if err != nil {
		return fmt.Errorf("idempotency check failed: %w", err)
	}
	if exists {
		s.metrics.DuplicateEarnTotal.Inc()
		s.logger.FromContext(ctx).Warn("duplicate earn dropped",
			"ref_id", earn.RefID,
			"source", earn.Source,
		)
		return nil // idempotent — not an error
	}

	// ─── 3. Assign EarnID + timestamp ────────────────────────────────────────
	earn.EarnID = uuid.NewString()
	earn.CreatedAt = time.Now().Unix()

	// ─── 4. Persist earn transaction ─────────────────────────────────────────
	dbStart := time.Now()
	if err := s.txRepo.Save(ctx, earn); err != nil {
		s.metrics.DBErrorsTotal.WithLabelValues("earn_save").Inc()
		return fmt.Errorf("save earn transaction: %w", err)
	}
	s.metrics.DBQueryDuration.WithLabelValues("earn_save").Observe(time.Since(dbStart).Seconds())

	// ─── 5. Publish via circuit-breaker-protected NATS ───────────────────────
	publishErr := s.cb.Execute(ctx, func(ctx context.Context) error {
		pubStart := time.Now()
		err := s.out.SendEarn(ctx, earn)
		s.metrics.NATSPublishDuration.WithLabelValues("reward_earn").Observe(time.Since(pubStart).Seconds())
		return err
	})

	if publishErr != nil {
		// Publish failed — the earn IS saved in DB (source of truth).
		// The outbox relay will pick it up and retry.
		s.metrics.NATSPublishTotal.WithLabelValues("reward_earn", "error").Inc()
		s.logger.FromContext(ctx).Error("earn publish failed — will be retried by outbox",
			"earn_id", earn.EarnID,
			"error", publishErr,
		)
		// Return nil so consumer ACKs — avoids re-processing the same event
		// The outbox ensures eventual delivery.
		return nil
	}

	s.metrics.NATSPublishTotal.WithLabelValues("reward_earn", "success").Inc()
	s.metrics.EarnProcessedTotal.WithLabelValues(earn.Source).Inc()
	s.metrics.EarnPointsTotal.WithLabelValues(earn.Source).Add(float64(earn.Point))

	s.logger.FromContext(ctx).Info("earn processed",
		"earn_id", earn.EarnID,
		"user_id", earn.UserID,
		"points", earn.Point,
		"source", earn.Source,
	)

	return nil
}

// validateEarn validates earn fields before processing
func validateEarn(earn model.Earn) error {
	if earn.UserID == "" {
		return errors.New("user_id is required")
	}
	if earn.RefID == "" {
		return errors.New("ref_id is required")
	}
	if earn.Source == "" {
		return errors.New("source is required")
	}
	if earn.Point <= 0 {
		return errors.New("point must be > 0")
	}
	allowedSources := map[string]bool{"trip": true, "order": true, "campaign": true}
	if !allowedSources[earn.Source] {
		return fmt.Errorf("invalid source: %s", earn.Source)
	}
	return nil
}

// ─── Enriched Redeem Service ────────────────────────────────────────────────

// enterpriseRedeemRequestService wraps redeem with validation, dedup, metrics
type enterpriseRedeemRequestService struct {
	repo       repoPort.RedeemTransactionRepository
	redeemPort interface {
		SendRedeemRequest(ctx context.Context, redeem model.Redeem) error
	}
	metrics *obs.Metrics
	logger  *obs.Logger
	cb      *resilience.CircuitBreaker
}

func NewEnterpriseRedeemRequestService(
	repo repoPort.RedeemTransactionRepository,
	redeemPort interface {
		SendRedeemRequest(ctx context.Context, redeem model.Redeem) error
	},
	metrics *obs.Metrics,
	logger *obs.Logger,
	cb *resilience.CircuitBreaker,
) *enterpriseRedeemRequestService {
	return &enterpriseRedeemRequestService{
		repo:       repo,
		redeemPort: redeemPort,
		metrics:    metrics,
		logger:     logger,
		cb:         cb,
	}
}

func (s *enterpriseRedeemRequestService) RequestRedeem(
	ctx context.Context,
	redeem model.Redeem,
) error {
	op := s.logger.StartOp(ctx, "request_redeem",
		"user_id", redeem.UserID,
		"point", redeem.Point,
	)

	err := s.requestRedeemInternal(ctx, redeem)
	op.End(err)
	return err
}

func (s *enterpriseRedeemRequestService) requestRedeemInternal(
	ctx context.Context,
	redeem model.Redeem,
) error {
	// ─── 1. Validate ─────────────────────────────────────────────────────────
	if err := validateRedeem(redeem); err != nil {
		return fmt.Errorf("validation: %w", err)
	}

	// ─── 2. Assign IDs ───────────────────────────────────────────────────────
	redeem.RedeemID = uuid.NewString()
	redeem.RequestedAt = time.Now().Unix()

	// ─── 3. Idempotency guard ─────────────────────────────────────────────────
	exists, err := s.repo.ExistsByRedeemID(ctx, redeem.RedeemID)
	if err != nil {
		return fmt.Errorf("idempotency check: %w", err)
	}
	if exists {
		s.metrics.DuplicateRedeemTotal.Inc()
		slog.WarnContext(ctx, "duplicate redeem dropped", "redeem_id", redeem.RedeemID)
		return nil
	}

	// ─── 4. Save request ─────────────────────────────────────────────────────
	dbStart := time.Now()
	if err := s.repo.SaveRequest(ctx, redeem); err != nil {
		s.metrics.DBErrorsTotal.WithLabelValues("redeem_save").Inc()
		return fmt.Errorf("save redeem request: %w", err)
	}
	s.metrics.DBQueryDuration.WithLabelValues("redeem_save").Observe(time.Since(dbStart).Seconds())

	// ─── 5. Publish (with circuit breaker) ───────────────────────────────────
	publishErr := resilience.RetryWithBackoff(
		ctx,
		resilience.RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 50 * time.Millisecond,
			Multiplier:      2.0,
			JitterFactor:    0.3,
		},
		func(ctx context.Context) error {
			return s.cb.Execute(ctx, func(ctx context.Context) error {
				return s.redeemPort.SendRedeemRequest(ctx, redeem)
			})
		},
	)

	if publishErr != nil {
		s.metrics.NATSPublishTotal.WithLabelValues("redeem_request", "error").Inc()
		// Redeem saved in DB — outbox relay will retry publish
		slog.ErrorContext(ctx, "redeem publish failed",
			"redeem_id", redeem.RedeemID,
			"error", publishErr,
		)
		return nil // ACK to prevent reprocessing; outbox handles retry
	}

	s.metrics.NATSPublishTotal.WithLabelValues("redeem_request", "success").Inc()
	s.metrics.RedeemRequestsTotal.WithLabelValues("requested").Inc()
	s.metrics.RedeemPointsTotal.WithLabelValues("requested").Add(float64(redeem.Point))

	return nil
}

func validateRedeem(r model.Redeem) error {
	if r.UserID == "" {
		return errors.New("user_id is required")
	}
	if r.Point <= 0 {
		return errors.New("point must be > 0")
	}
	const maxRedeemPerRequest = int64(1_000_000)
	if r.Point > maxRedeemPerRequest {
		return fmt.Errorf("point exceeds max per request (%d)", maxRedeemPerRequest)
	}
	return nil
}
