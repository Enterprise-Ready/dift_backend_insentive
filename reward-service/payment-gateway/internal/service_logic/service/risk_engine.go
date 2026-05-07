package service

import (
	"context"
	"fmt"
	"time"

	"github.com/enterprise/payment-gateway/internal/config"
	"github.com/enterprise/payment-gateway/internal/domain"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// RiskEngine evaluates transactions for fraud risk
type RiskEngine struct {
	cfg    *config.RiskConfig
	redis  redis.UniversalClient
	logger *zap.Logger
}

type RiskResult struct {
	Score          int
	Level          domain.RiskLevel
	Reasons        []string
	Blocked        bool
	RequiresReview bool
}

func NewRiskEngine(cfg *config.RiskConfig, redis redis.UniversalClient, logger *zap.Logger) *RiskEngine {
	return &RiskEngine{cfg: cfg, redis: redis, logger: logger}
}

// Evaluate calculates a risk score for a payment request
func (r *RiskEngine) Evaluate(ctx context.Context, req *domain.CreatePaymentRequest) (*RiskResult, error) {
	if !r.cfg.Enabled {
		return &RiskResult{Score: 0, Level: domain.RiskLow}, nil
	}

	result := &RiskResult{Reasons: []string{}}
	score := 0

	// --- Rule 1: Large transaction amount ---
	if req.Amount.GreaterThan(decimal.NewFromFloat(100000)) {
		score += 30
		result.Reasons = append(result.Reasons, "very_large_amount")
	} else if req.Amount.GreaterThan(decimal.NewFromFloat(50000)) {
		score += 15
		result.Reasons = append(result.Reasons, "large_amount")
	}

	// --- Rule 2: Velocity check (transactions per hour) ---
	velScore, err := r.checkVelocity(ctx, req)
	if err != nil {
		r.logger.Warn("velocity check failed", zap.Error(err))
	}
	if velScore > 0 {
		score += velScore
		result.Reasons = append(result.Reasons, "high_velocity")
	}

	// --- Rule 3: Amount velocity (total amount per hour) ---
	amtScore, err := r.checkAmountVelocity(ctx, req)
	if err == nil && amtScore > 0 {
		score += amtScore
		result.Reasons = append(result.Reasons, "high_amount_velocity")
	}

	// --- Rule 4: IP blacklist ---
	if r.cfg.EnableIPBlacklist && req.IPAddress != "" {
		if blacklisted, _ := r.isIPBlacklisted(ctx, req.IPAddress); blacklisted {
			score += 100
			result.Reasons = append(result.Reasons, "blacklisted_ip")
		}
	}

	// --- Rule 5: Card blacklist ---
	if r.cfg.EnableCardBlacklist && req.CardNumber != "" {
		if blacklisted, _ := r.isCardBlacklisted(ctx, req.CardNumber); blacklisted {
			score += 100
			result.Reasons = append(result.Reasons, "blacklisted_card")
		}
	}

	// --- Rule 6: Unusual hour (2 AM - 5 AM local time) ---
	hour := time.Now().Hour()
	if hour >= 2 && hour <= 5 {
		score += 10
		result.Reasons = append(result.Reasons, "unusual_hour")
	}

	// --- Rule 7: Crypto payment high amount ---
	if req.Method == domain.MethodCrypto && req.Amount.GreaterThan(decimal.NewFromFloat(10000)) {
		score += 20
		result.Reasons = append(result.Reasons, "crypto_high_amount")
	}

	// --- Rule 8: Missing customer info for large amount ---
	if req.Amount.GreaterThan(decimal.NewFromFloat(50000)) {
		if req.CustomerEmail == "" || req.CustomerPhone == "" {
			score += 15
			result.Reasons = append(result.Reasons, "missing_customer_info_large_amount")
		}
	}

	result.Score = score
	result.Level = r.scoreToLevel(score)
	result.Blocked = score >= r.cfg.BlockScore
	result.RequiresReview = score >= r.cfg.ReviewScore && score < r.cfg.BlockScore

	r.logger.Info("risk evaluation complete",
		zap.Int("score", score),
		zap.String("level", string(result.Level)),
		zap.Strings("reasons", result.Reasons),
	)

	return result, nil
}

func (r *RiskEngine) checkVelocity(ctx context.Context, req *domain.CreatePaymentRequest) (int, error) {
	keys := []string{}
	if req.CustomerID != "" {
		keys = append(keys, fmt.Sprintf("vel:cust:%s", req.CustomerID))
	}
	if req.IPAddress != "" {
		keys = append(keys, fmt.Sprintf("vel:ip:%s", req.IPAddress))
	}

	for _, key := range keys {
		count, err := r.redis.Incr(ctx, key).Result()
		if err != nil {
			return 0, err
		}
		r.redis.Expire(ctx, key, time.Duration(r.cfg.VelocityWindowMinutes)*time.Minute)

		if count > int64(r.cfg.MaxTransactionsPerHour) {
			return 40, nil
		} else if count > int64(r.cfg.MaxTransactionsPerHour/2) {
			return 20, nil
		}
	}
	return 0, nil
}

func (r *RiskEngine) checkAmountVelocity(ctx context.Context, req *domain.CreatePaymentRequest) (int, error) {
	if req.CustomerID == "" {
		return 0, nil
	}
	key := fmt.Sprintf("vel:amt:%s", req.CustomerID)

	amountStr := req.Amount.String()
	pipe := r.redis.Pipeline()
	pipe.IncrByFloat(ctx, key, req.Amount.InexactFloat64())
	pipe.Expire(ctx, key, time.Duration(r.cfg.VelocityWindowMinutes)*time.Minute)
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}

	totalAmount := cmds[0].(*redis.FloatCmd).Val()
	_ = amountStr

	if totalAmount > r.cfg.MaxAmountPerHour*1.5 {
		return 40, nil
	} else if totalAmount > r.cfg.MaxAmountPerHour {
		return 20, nil
	}
	return 0, nil
}

func (r *RiskEngine) isIPBlacklisted(ctx context.Context, ip string) (bool, error) {
	key := fmt.Sprintf("blacklist:ip:%s", ip)
	exists, err := r.redis.Exists(ctx, key).Result()
	return exists > 0, err
}

func (r *RiskEngine) isCardBlacklisted(ctx context.Context, cardNumber string) (bool, error) {
	// Hash the card number before checking
	h := fmt.Sprintf("blacklist:card:%x", cardNumber) // In production use crypto.SHA256Hash
	exists, err := r.redis.Exists(ctx, h).Result()
	return exists > 0, err
}

func (r *RiskEngine) AddIPBlacklist(ctx context.Context, ip string, ttl time.Duration) error {
	key := fmt.Sprintf("blacklist:ip:%s", ip)
	return r.redis.Set(ctx, key, 1, ttl).Err()
}

func (r *RiskEngine) scoreToLevel(score int) domain.RiskLevel {
	switch {
	case score >= 80:
		return domain.RiskCritical
	case score >= 60:
		return domain.RiskHigh
	case score >= 30:
		return domain.RiskMedium
	default:
		return domain.RiskLow
	}
}
