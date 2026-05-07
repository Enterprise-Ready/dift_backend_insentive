package service

import (
	"context"
	"fmt"
	"time"

	"github.com/enterprise/payment-gateway/internal/config"
	"github.com/enterprise/payment-gateway/internal/domain"
	"github.com/enterprise/payment-gateway/pkg/audit"
	"github.com/enterprise/payment-gateway/pkg/circuit"
	"github.com/enterprise/payment-gateway/pkg/idempotency"
	"github.com/enterprise/payment-gateway/pkg/metrics"
	"github.com/enterprise/payment-gateway/pkg/retry"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// PaymentRepository defines persistence operations
type PaymentRepository interface {
	Create(ctx context.Context, payment *domain.Payment) error
	GetByID(ctx context.Context, id string) (*domain.Payment, error)
	GetByOrderID(ctx context.Context, merchantID, orderID string) (*domain.Payment, error)
	UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus, updates map[string]interface{}) error
	Update(ctx context.Context, payment *domain.Payment) error
	List(ctx context.Context, req *domain.PaymentListRequest) ([]*domain.Payment, int64, error)
	GetSummary(ctx context.Context, merchantID string, from, to time.Time) (*domain.PaymentSummary, error)
	CreateTransaction(ctx context.Context, tx *domain.Transaction) error
	GetTransactions(ctx context.Context, paymentID string) ([]*domain.Transaction, error)
	CreateRefund(ctx context.Context, refund *domain.Refund) error
	GetRefunds(ctx context.Context, paymentID string) ([]*domain.Refund, error)
	GetRefundByID(ctx context.Context, id string) (*domain.Refund, error)
	UpdateRefund(ctx context.Context, refund *domain.Refund) error
	GetTotalRefunded(ctx context.Context, paymentID string) (decimal.Decimal, error)
}

// MerchantRepository defines merchant data access
type MerchantRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Merchant, error)
	GetByAPIKey(ctx context.Context, apiKeyHash string) (*domain.Merchant, error)
	Update(ctx context.Context, merchant *domain.Merchant) error
}

// PaymentService handles core payment business logic
type PaymentService struct {
	paymentRepo  PaymentRepository
	merchantRepo MerchantRepository
	registry     *Registry
	riskEngine   *RiskEngine
	circuit      *circuit.Manager
	idempotency  *idempotency.Store
	audit        *audit.Logger
	metrics      *metrics.Metrics
	webhookSvc   *WebhookService
	cfg          *config.Config
	logger       *zap.Logger
}

func NewPaymentService(
	paymentRepo PaymentRepository,
	merchantRepo MerchantRepository,
	registry *Registry,
	riskEngine *RiskEngine,
	cb *circuit.Manager,
	idempotencyStore *idempotency.Store,
	auditLogger *audit.Logger,
	m *metrics.Metrics,
	webhookSvc *WebhookService,
	cfg *config.Config,
	logger *zap.Logger,
) *PaymentService {
	return &PaymentService{
		paymentRepo:  paymentRepo,
		merchantRepo: merchantRepo,
		registry:     registry,
		riskEngine:   riskEngine,
		circuit:      cb,
		idempotency:  idempotencyStore,
		audit:        auditLogger,
		metrics:      m,
		webhookSvc:   webhookSvc,
		cfg:          cfg,
		logger:       logger,
	}
}

// ============================================================
// CREATE PAYMENT
// ============================================================

func (s *PaymentService) CreatePayment(ctx context.Context, req *domain.CreatePaymentRequest) (*domain.PaymentResponse, error) {
	startTime := time.Now()

	// 1. Validate merchant
	merchant, err := s.merchantRepo.GetByID(ctx, req.MerchantID)
	if err != nil {
		return nil, domain.ErrMerchantNotFound
	}
	if !merchant.IsActive {
		return nil, domain.ErrMerchantInactive
	}

	// 2. Validate amount limits
	if err := s.validateLimits(req); err != nil {
		return nil, err
	}

	// 3. Idempotency check
	if req.IdempotencyKey != "" {
		record, found, err := s.idempotency.Lock(ctx, req.MerchantID, req.IdempotencyKey)
		if err != nil {
			return nil, domain.WrapError(domain.ErrIdempotencyConflict, err.Error())
		}
		if found && record.Status == "completed" {
			s.metrics.IdempotencyHits.WithLabelValues(req.MerchantID, "hit").Inc()
			// Return cached response
			payment, getErr := s.paymentRepo.GetByOrderID(ctx, req.MerchantID, req.OrderID)
			if getErr == nil {
				return &domain.PaymentResponse{Payment: payment}, nil
			}
		}
		s.metrics.IdempotencyHits.WithLabelValues(req.MerchantID, "miss").Inc()
		defer func() {
			if err != nil {
				s.idempotency.Release(ctx, req.MerchantID, req.IdempotencyKey)
			}
		}()
	}

	// 4. Duplicate payment check (same order_id for merchant)
	if existing, err := s.paymentRepo.GetByOrderID(ctx, req.MerchantID, req.OrderID); err == nil {
		if existing.Status == domain.StatusSuccess {
			return nil, domain.ErrPaymentAlreadyPaid
		}
		if existing.Status == domain.StatusPending || existing.Status == domain.StatusProcessing {
			return &domain.PaymentResponse{Payment: existing}, nil
		}
	}

	// 5. Risk evaluation
	riskResult, err := s.riskEngine.Evaluate(ctx, req)
	if err != nil {
		s.logger.Warn("risk evaluation error", zap.Error(err))
	}
	if riskResult != nil && riskResult.Blocked {
		s.metrics.FraudDetected.WithLabelValues("block", "payment_blocked").Inc()
		return nil, domain.WrapError(domain.ErrHighRiskTransaction,
			fmt.Sprintf("risk score: %d, reasons: %v", riskResult.Score, riskResult.Reasons))
	}

	// 6. Select provider
	provider, err := s.registry.GetForMethod(req.Method, req.Provider)
	if err != nil {
		return nil, err
	}

	// 7. Calculate fees
	fee := s.calculateFee(req.Amount, req.Method, provider.Name(), merchant)
	netAmount := req.Amount.Sub(fee)

	// 8. Create initial payment record
	expireTime := time.Now().Add(time.Duration(s.cfg.Limits.PaymentExpireMinutes) * time.Minute)
	payment := &domain.Payment{
		ID:             uuid.New().String(),
		MerchantID:     req.MerchantID,
		OrderID:        req.OrderID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Method:         req.Method,
		Provider:       provider.Name(),
		Status:         domain.StatusPending,
		Description:    req.Description,
		CustomerID:     req.CustomerID,
		CustomerEmail:  req.CustomerEmail,
		CustomerPhone:  req.CustomerPhone,
		CustomerName:   req.CustomerName,
		CallbackURL:    req.CallbackURL,
		ReturnURL:      req.ReturnURL,
		Metadata:       req.Metadata,
		IdempotencyKey: req.IdempotencyKey,
		Fee:            fee,
		NetAmount:      netAmount,
		ExchangeRate:   decimal.NewFromInt(1),
		ExpiresAt:      &expireTime,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if riskResult != nil {
		payment.RiskScore = riskResult.Score
		payment.RiskLevel = riskResult.Level
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, domain.ErrDatabaseError
	}

	// 9. Process payment via provider with circuit breaker + retry
	s.metrics.ActivePayments.Inc()
	defer s.metrics.ActivePayments.Dec()

	var providerPayment *domain.Payment
	var qrImageURL string

	processErr := retry.Do(ctx, retry.DefaultConfig, s.logger, func(attempt int) error {
		if attempt > 1 {
			s.logger.Info("retrying payment", zap.String("payment_id", payment.ID), zap.Int("attempt", attempt))
		}

		_, cbErr := s.circuit.Execute(string(provider.Name()), func() (interface{}, error) {
			providerStart := time.Now()
			var p *domain.Payment
			var err error

			// Route to correct method
			if req.Method == domain.MethodQRCode || req.Method == domain.MethodPromptPay {
				qrData, imgURL, err := provider.GenerateQRCode(ctx, req)
				if err != nil {
					return nil, err
				}
				payment.QRCodeData = qrData
				payment.QRCodeURL = imgURL
				qrImageURL = imgURL
				p = payment
				p.Status = domain.StatusPending
			} else {
				p, err = provider.CreatePayment(ctx, req)
				if err != nil {
					return nil, err
				}
			}

			s.metrics.ProviderLatency.WithLabelValues(string(provider.Name()), "create").
				Observe(time.Since(providerStart).Seconds())
			s.metrics.ProviderRequests.WithLabelValues(string(provider.Name()), "create", "success").Inc()

			providerPayment = p
			return p, nil
		})
		return cbErr
	})

	// 10. Update payment status based on result
	if processErr != nil {
		s.logger.Error("payment processing failed",
			zap.String("payment_id", payment.ID),
			zap.Error(processErr),
		)

		payment.Status = domain.StatusFailed
		payment.FailureMessage = processErr.Error()
		if pe, ok := processErr.(*domain.PaymentError); ok {
			payment.FailureCode = pe.Code
		}
		payment.RetryCount++

		s.paymentRepo.UpdateStatus(ctx, payment.ID, domain.StatusFailed, map[string]interface{}{
			"failure_code":    payment.FailureCode,
			"failure_message": payment.FailureMessage,
			"retry_count":     payment.RetryCount,
		})

		s.metrics.PaymentTotal.WithLabelValues(
			string(req.Method), string(provider.Name()), "failed", string(req.Currency),
		).Inc()

		// Fire webhook
		go s.webhookSvc.Send(context.Background(), merchant, domain.WebhookPaymentFailed, payment)

		return nil, processErr
	}

	// 11. Merge provider response into payment
	if providerPayment != nil {
		if providerPayment.ProviderRefID != "" {
			payment.ProviderRefID = providerPayment.ProviderRefID
		}
		if providerPayment.Status != "" {
			payment.Status = providerPayment.Status
		}
		if providerPayment.QRCodeData != "" {
			payment.QRCodeData = providerPayment.QRCodeData
		}
	}
	payment.UpdatedAt = time.Now()

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		s.logger.Error("failed to update payment after processing", zap.Error(err))
	}

	// 12. Record transaction
	tx := &domain.Transaction{
		ID:            uuid.New().String(),
		PaymentID:     payment.ID,
		Type:          domain.TxTypeCharge,
		Amount:        payment.Amount,
		Currency:      payment.Currency,
		Status:        payment.Status,
		Provider:      payment.Provider,
		ProviderRefID: payment.ProviderRefID,
		CreatedAt:     time.Now(),
	}
	s.paymentRepo.CreateTransaction(ctx, tx)

	// 13. Complete idempotency record
	if req.IdempotencyKey != "" {
		s.idempotency.Complete(ctx, req.MerchantID, req.IdempotencyKey, payment)
	}

	// 14. Emit metrics
	s.metrics.PaymentTotal.WithLabelValues(
		string(req.Method), string(provider.Name()), string(payment.Status), string(req.Currency),
	).Inc()
	s.metrics.PaymentDuration.WithLabelValues(
		string(req.Method), string(provider.Name()), string(payment.Status),
	).Observe(time.Since(startTime).Seconds())
	s.metrics.PaymentAmount.WithLabelValues(string(req.Method), string(req.Currency)).
		Observe(req.Amount.InexactFloat64())

	if riskResult != nil {
		s.metrics.RiskScoreHistogram.Observe(float64(riskResult.Score))
	}

	// 15. Audit log
	s.audit.PaymentAction(ctx, payment.ID, "payment.created", req.MerchantID, req.IPAddress, nil, payment)

	// 16. Fire success webhook
	if payment.Status == domain.StatusSuccess {
		paidAt := time.Now()
		payment.PaidAt = &paidAt
		s.paymentRepo.Update(ctx, payment)
		go s.webhookSvc.Send(context.Background(), merchant, domain.WebhookPaymentSuccess, payment)
	}

	return &domain.PaymentResponse{
		Payment:     payment,
		QRCodeImage: qrImageURL,
	}, nil
}

// ============================================================
// GET PAYMENT
// ============================================================

func (s *PaymentService) GetPayment(ctx context.Context, paymentID, merchantID string) (*domain.Payment, error) {
	payment, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, domain.ErrPaymentNotFound
	}
	// Authorization check
	if payment.MerchantID != merchantID {
		return nil, domain.ErrForbidden
	}

	// Enrich with transactions and refunds
	txs, _ := s.paymentRepo.GetTransactions(ctx, paymentID)
	refunds, _ := s.paymentRepo.GetRefunds(ctx, paymentID)
	payment.Transactions = make([]domain.Transaction, len(txs))
	for i, tx := range txs {
		payment.Transactions[i] = *tx
	}
	payment.Refunds = make([]domain.Refund, len(refunds))
	for i, r := range refunds {
		payment.Refunds[i] = *r
	}

	return payment, nil
}

// ============================================================
// VERIFY / SYNC PAYMENT STATUS
// ============================================================

func (s *PaymentService) VerifyPayment(ctx context.Context, paymentID, merchantID string) (*domain.Payment, error) {
	payment, err := s.GetPayment(ctx, paymentID, merchantID)
	if err != nil {
		return nil, err
	}

	// Only verify pending/processing payments
	if payment.Status != domain.StatusPending && payment.Status != domain.StatusProcessing {
		return payment, nil
	}

	// Check expiry
	if payment.ExpiresAt != nil && time.Now().After(*payment.ExpiresAt) {
		payment.Status = domain.StatusExpired
		s.paymentRepo.UpdateStatus(ctx, paymentID, domain.StatusExpired, nil)
		return payment, nil
	}

	provider, err := s.registry.Get(payment.Provider)
	if err != nil {
		return payment, nil // Return cached status if provider unavailable
	}

	verified, err := provider.VerifyPayment(ctx, payment.ProviderRefID)
	if err != nil {
		s.logger.Warn("payment verification failed", zap.String("payment_id", paymentID), zap.Error(err))
		return payment, nil
	}

	if verified.Status != payment.Status {
		oldStatus := payment.Status
		payment.Status = verified.Status
		if verified.Status == domain.StatusSuccess {
			paidAt := time.Now()
			payment.PaidAt = &paidAt
		}
		s.paymentRepo.UpdateStatus(ctx, paymentID, payment.Status, map[string]interface{}{
			"paid_at": payment.PaidAt,
		})

		s.logger.Info("payment status updated",
			zap.String("payment_id", paymentID),
			zap.String("old_status", string(oldStatus)),
			zap.String("new_status", string(payment.Status)),
		)

		merchant, _ := s.merchantRepo.GetByID(ctx, payment.MerchantID)
		if merchant != nil && payment.Status == domain.StatusSuccess {
			go s.webhookSvc.Send(context.Background(), merchant, domain.WebhookPaymentSuccess, payment)
		}
	}

	return payment, nil
}

// ============================================================
// REFUND
// ============================================================

func (s *PaymentService) CreateRefund(ctx context.Context, req *domain.CreateRefundRequest, merchantID string) (*domain.Refund, error) {
	// Get payment
	payment, err := s.paymentRepo.GetByID(ctx, req.PaymentID)
	if err != nil {
		return nil, domain.ErrPaymentNotFound
	}
	if payment.MerchantID != merchantID {
		return nil, domain.ErrForbidden
	}

	// Validate refund is allowed
	if payment.Status != domain.StatusSuccess && payment.Status != domain.StatusPartialRef {
		return nil, domain.WrapError(domain.ErrRefundNotAllowed,
			fmt.Sprintf("payment status is %s, expected SUCCESS", payment.Status))
	}

	// Check refund window (90 days default)
	if payment.PaidAt != nil && time.Since(*payment.PaidAt) > 90*24*time.Hour {
		return nil, domain.ErrRefundWindowClosed
	}

	// Calculate remaining refundable amount
	totalRefunded, err := s.paymentRepo.GetTotalRefunded(ctx, req.PaymentID)
	if err != nil {
		return nil, domain.ErrDatabaseError
	}
	remaining := payment.Amount.Sub(totalRefunded)
	if req.Amount.GreaterThan(remaining) {
		return nil, domain.WrapError(domain.ErrRefundExceedsAmount,
			fmt.Sprintf("max refundable: %s", remaining.String()))
	}

	// Get provider
	provider, err := s.registry.Get(payment.Provider)
	if err != nil {
		return nil, domain.ErrNoProviderAvailable
	}

	// Create refund record
	refund := &domain.Refund{
		ID:          uuid.New().String(),
		PaymentID:   req.PaymentID,
		Amount:      req.Amount,
		Currency:    payment.Currency,
		Status:      domain.RefundPending,
		Reason:      req.Reason,
		RequestedBy: req.RequestedBy,
		CreatedAt:   time.Now(),
	}

	if err := s.paymentRepo.CreateRefund(ctx, refund); err != nil {
		return nil, domain.ErrDatabaseError
	}

	// Process refund with provider
	var refundErr error
	_, cbErr := s.circuit.Execute(string(provider.Name()), func() (interface{}, error) {
		providerRefID, err := provider.RefundPayment(ctx, payment.ProviderRefID, req.Amount, req.Reason)
		if err != nil {
			return nil, err
		}
		refund.ProviderRefID = providerRefID
		return providerRefID, nil
	})

	processedAt := time.Now()
	if cbErr != nil {
		refundErr = cbErr
		refund.Status = domain.RefundFailed
	} else {
		refund.Status = domain.RefundCompleted
		refund.ProcessedAt = &processedAt
	}

	s.paymentRepo.UpdateRefund(ctx, refund)

	if refundErr != nil {
		return nil, refundErr
	}

	// Update payment status
	newRefundedTotal := totalRefunded.Add(req.Amount)
	if newRefundedTotal.Equal(payment.Amount) {
		payment.Status = domain.StatusRefunded
	} else {
		payment.Status = domain.StatusPartialRef
	}
	s.paymentRepo.UpdateStatus(ctx, payment.ID, payment.Status, nil)

	// Record transaction
	tx := &domain.Transaction{
		ID:            uuid.New().String(),
		PaymentID:     payment.ID,
		Type:          domain.TxTypeRefund,
		Amount:        req.Amount,
		Currency:      payment.Currency,
		Status:        domain.StatusSuccess,
		Provider:      payment.Provider,
		ProviderRefID: refund.ProviderRefID,
		ProcessedAt:   &processedAt,
		CreatedAt:     time.Now(),
	}
	s.paymentRepo.CreateTransaction(ctx, tx)

	// Metrics
	s.metrics.RefundTotal.WithLabelValues("success", string(payment.Currency)).Inc()

	// Audit
	s.audit.RefundAction(ctx, refund.ID, payment.ID, "refund.created", req.RequestedBy, "")

	// Webhook
	merchant, _ := s.merchantRepo.GetByID(ctx, payment.MerchantID)
	if merchant != nil {
		go s.webhookSvc.Send(context.Background(), merchant, domain.WebhookRefundSuccess, refund)
	}

	return refund, nil
}

// ============================================================
// CANCEL PAYMENT
// ============================================================

func (s *PaymentService) CancelPayment(ctx context.Context, paymentID, merchantID, reason string) (*domain.Payment, error) {
	payment, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, domain.ErrPaymentNotFound
	}
	if payment.MerchantID != merchantID {
		return nil, domain.ErrForbidden
	}

	if payment.Status != domain.StatusPending {
		return nil, domain.WrapError(domain.ErrInvalidStatusChange,
			fmt.Sprintf("cannot cancel payment in status %s", payment.Status))
	}

	// Try to void at provider
	if payment.ProviderRefID != "" {
		if provider, err := s.registry.Get(payment.Provider); err == nil {
			provider.VoidPayment(ctx, payment.ProviderRefID)
		}
	}

	payment.Status = domain.StatusCancelled
	payment.FailureMessage = reason
	payment.UpdatedAt = time.Now()

	s.paymentRepo.UpdateStatus(ctx, paymentID, domain.StatusCancelled, map[string]interface{}{
		"failure_message": reason,
	})

	s.audit.PaymentAction(ctx, paymentID, "payment.cancelled", merchantID, "", nil, payment)
	return payment, nil
}

// ============================================================
// LIST PAYMENTS
// ============================================================

func (s *PaymentService) ListPayments(ctx context.Context, req *domain.PaymentListRequest) (*domain.PaymentListResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}

	payments, total, err := s.paymentRepo.List(ctx, req)
	if err != nil {
		return nil, domain.ErrDatabaseError
	}

	totalPages := int(total) / req.PageSize
	if int(total)%req.PageSize > 0 {
		totalPages++
	}

	return &domain.PaymentListResponse{
		Payments:   payments,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

// ============================================================
// SUMMARY / ANALYTICS
// ============================================================

func (s *PaymentService) GetSummary(ctx context.Context, merchantID string, from, to time.Time) (*domain.PaymentSummary, error) {
	return s.paymentRepo.GetSummary(ctx, merchantID, from, to)
}

// ============================================================
// HELPER METHODS
// ============================================================

func (s *PaymentService) validateLimits(req *domain.CreatePaymentRequest) error {
	minAmt := decimal.NewFromFloat(s.cfg.Limits.MinPaymentAmount)
	maxAmt := decimal.NewFromFloat(s.cfg.Limits.MaxPaymentAmount)

	if req.Amount.LessThan(minAmt) {
		return domain.WrapError(domain.ErrInvalidAmount,
			fmt.Sprintf("minimum amount is %s", minAmt.String()))
	}
	if maxAmt.IsPositive() && req.Amount.GreaterThan(maxAmt) {
		return domain.WrapError(domain.ErrTransactionLimit,
			fmt.Sprintf("maximum amount is %s", maxAmt.String()))
	}
	return nil
}

func (s *PaymentService) calculateFee(amount decimal.Decimal, method domain.PaymentMethod, provider domain.PaymentProvider, merchant *domain.Merchant) decimal.Decimal {
	// Default fee rates
	feeRates := map[domain.PaymentMethod]float64{
		domain.MethodCreditCard: 0.025, // 2.5%
		domain.MethodDebitCard:  0.015, // 1.5%
		domain.MethodQRCode:     0.005, // 0.5%
		domain.MethodPromptPay:  0.005, // 0.5%
		domain.MethodBankTx:     0.003, // 0.3%
		domain.MethodWallet:     0.015, // 1.5%
		domain.MethodInstalment: 0.0,   // absorbed by issuer
	}

	rate, ok := feeRates[method]
	if !ok {
		rate = 0.02 // Default 2%
	}

	// Check merchant-specific fee config
	if merchant.FeeConfig != nil {
		if customRate, ok := merchant.FeeConfig[string(method)]; ok {
			if r, ok := customRate.(float64); ok {
				rate = r
			}
		}
	}

	return amount.Mul(decimal.NewFromFloat(rate)).Round(2)
}
