package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enterprise/payment-gateway/internal/domain"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type PaymentRepo struct {
	db *sqlx.DB
}

func NewPaymentRepo(db *sqlx.DB) *PaymentRepo {
	return &PaymentRepo{db: db}
}

func (r *PaymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	meta, _ := json.Marshal(p.Metadata)
	query := `
		INSERT INTO payments (
			id, merchant_id, order_id, amount, currency, method, provider,
			status, reference_id, provider_ref_id, description,
			customer_id, customer_email, customer_phone, customer_name,
			ip_address, user_agent, metadata, callback_url, return_url,
			qr_code_url, qr_code_data, bank_account_no,
			expires_at, paid_at, fee, net_amount, exchange_rate,
			failure_code, failure_message, risk_score, risk_level,
			idempotency_key, retry_count, created_at, updated_at
		) VALUES (
			:id, :merchant_id, :order_id, :amount, :currency, :method, :provider,
			:status, :reference_id, :provider_ref_id, :description,
			:customer_id, :customer_email, :customer_phone, :customer_name,
			:ip_address, :user_agent, :metadata, :callback_url, :return_url,
			:qr_code_url, :qr_code_data, :bank_account_no,
			:expires_at, :paid_at, :fee, :net_amount, :exchange_rate,
			:failure_code, :failure_message, :risk_score, :risk_level,
			:idempotency_key, :retry_count, :created_at, :updated_at
		)`

	args := map[string]interface{}{
		"id": p.ID, "merchant_id": p.MerchantID, "order_id": p.OrderID,
		"amount": p.Amount, "currency": p.Currency, "method": p.Method,
		"provider": p.Provider, "status": p.Status, "reference_id": p.ReferenceID,
		"provider_ref_id": p.ProviderRefID, "description": p.Description,
		"customer_id": p.CustomerID, "customer_email": p.CustomerEmail,
		"customer_phone": p.CustomerPhone, "customer_name": p.CustomerName,
		"ip_address": p.IPAddress, "user_agent": p.UserAgent,
		"metadata": string(meta), "callback_url": p.CallbackURL,
		"return_url": p.ReturnURL, "qr_code_url": p.QRCodeURL,
		"qr_code_data": p.QRCodeData, "bank_account_no": p.BankAccountNo,
		"expires_at": p.ExpiresAt, "paid_at": p.PaidAt,
		"fee": p.Fee, "net_amount": p.NetAmount, "exchange_rate": p.ExchangeRate,
		"failure_code": p.FailureCode, "failure_message": p.FailureMessage,
		"risk_score": p.RiskScore, "risk_level": p.RiskLevel,
		"idempotency_key": p.IdempotencyKey, "retry_count": p.RetryCount,
		"created_at": p.CreatedAt, "updated_at": p.UpdatedAt,
	}

	_, err := r.db.NamedExecContext(ctx, query, args)
	return err
}

func (r *PaymentRepo) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	var p domain.Payment
	err := r.db.GetContext(ctx, &p, `SELECT * FROM payments WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PaymentRepo) GetByOrderID(ctx context.Context, merchantID, orderID string) (*domain.Payment, error) {
	var p domain.Payment
	err := r.db.GetContext(ctx, &p,
		`SELECT * FROM payments WHERE merchant_id = $1 AND order_id = $2 ORDER BY created_at DESC LIMIT 1`,
		merchantID, orderID,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PaymentRepo) UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus, updates map[string]interface{}) error {
	setClauses := []string{"status = $1", "updated_at = $2"}
	args := []interface{}{status, time.Now()}
	i := 3

	for k, v := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, i))
		args = append(args, v)
		i++
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE payments SET %s WHERE id = $%d", strings.Join(setClauses, ", "), i)
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *PaymentRepo) Update(ctx context.Context, p *domain.Payment) error {
	meta, _ := json.Marshal(p.Metadata)
	p.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE payments SET
			status=$1, provider_ref_id=$2, qr_code_url=$3, qr_code_data=$4,
			failure_code=$5, failure_message=$6, risk_score=$7, risk_level=$8,
			paid_at=$9, retry_count=$10, metadata=$11, updated_at=$12
		WHERE id=$13`,
		p.Status, p.ProviderRefID, p.QRCodeURL, p.QRCodeData,
		p.FailureCode, p.FailureMessage, p.RiskScore, p.RiskLevel,
		p.PaidAt, p.RetryCount, string(meta), p.UpdatedAt, p.ID,
	)
	return err
}

func (r *PaymentRepo) List(ctx context.Context, req *domain.PaymentListRequest) ([]*domain.Payment, int64, error) {
	conditions := []string{"merchant_id = $1"}
	args := []interface{}{req.MerchantID}
	i := 2

	if req.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", i))
		args = append(args, req.Status)
		i++
	}
	if req.Method != "" {
		conditions = append(conditions, fmt.Sprintf("method = $%d", i))
		args = append(args, req.Method)
		i++
	}
	if req.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", i))
		args = append(args, req.DateFrom)
		i++
	}
	if req.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", i))
		args = append(args, req.DateTo)
		i++
	}

	where := strings.Join(conditions, " AND ")

	var total int64
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	err := r.db.GetContext(ctx, &total, fmt.Sprintf("SELECT COUNT(*) FROM payments WHERE %s", where), countArgs...)
	if err != nil {
		return nil, 0, err
	}

	offset := (req.Page - 1) * req.PageSize
	args = append(args, req.PageSize, offset)
	query := fmt.Sprintf(
		"SELECT * FROM payments WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		where, i, i+1,
	)

	var payments []*domain.Payment
	err = r.db.SelectContext(ctx, &payments, query, args...)
	return payments, total, err
}

func (r *PaymentRepo) GetSummary(ctx context.Context, merchantID string, from, to time.Time) (*domain.PaymentSummary, error) {
	summary := &domain.PaymentSummary{
		MerchantID: merchantID,
		DateFrom:   from,
		DateTo:     to,
		ByMethod:   make(map[string]int64),
		ByProvider: make(map[string]int64),
	}

	// Main aggregation
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status = 'SUCCESS' THEN 1 ELSE 0 END) as success,
			SUM(CASE WHEN status = 'FAILED' THEN 1 ELSE 0 END) as failed,
			COALESCE(SUM(CASE WHEN status = 'SUCCESS' THEN amount ELSE 0 END), 0) as total_amount,
			COALESCE(SUM(CASE WHEN status = 'SUCCESS' THEN fee ELSE 0 END), 0) as total_fees,
			COALESCE(AVG(CASE WHEN status = 'SUCCESS' THEN amount END), 0) as avg_amount
		FROM payments
		WHERE merchant_id = $1 AND created_at BETWEEN $2 AND $3`,
		merchantID, from, to,
	).Scan(
		&summary.TotalPayments, &summary.SuccessPayments, &summary.FailedPayments,
		&summary.TotalAmount, &summary.TotalFees, &summary.AverageAmount,
	)
	if err != nil {
		return nil, err
	}

	if summary.TotalPayments > 0 {
		summary.SuccessRate = float64(summary.SuccessPayments) / float64(summary.TotalPayments) * 100
	}

	// By method
	rows, err := r.db.QueryContext(ctx, `
		SELECT method, COUNT(*) FROM payments
		WHERE merchant_id = $1 AND created_at BETWEEN $2 AND $3
		GROUP BY method`, merchantID, from, to)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var method string
			var count int64
			rows.Scan(&method, &count)
			summary.ByMethod[method] = count
		}
	}

	// By provider
	rows2, err := r.db.QueryContext(ctx, `
		SELECT provider, COUNT(*) FROM payments
		WHERE merchant_id = $1 AND created_at BETWEEN $2 AND $3
		GROUP BY provider`, merchantID, from, to)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var provider string
			var count int64
			rows2.Scan(&provider, &count)
			summary.ByProvider[provider] = count
		}
	}

	return summary, nil
}

func (r *PaymentRepo) CreateTransaction(ctx context.Context, tx *domain.Transaction) error {
	rawReq, _ := json.Marshal(tx.RawRequest)
	rawResp, _ := json.Marshal(tx.RawResponse)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO transactions (id, payment_id, type, amount, currency, status, provider,
			provider_ref_id, raw_request, raw_response, error_code, error_message, processed_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		tx.ID, tx.PaymentID, tx.Type, tx.Amount, tx.Currency, tx.Status, tx.Provider,
		tx.ProviderRefID, string(rawReq), string(rawResp), tx.ErrorCode, tx.ErrorMessage,
		tx.ProcessedAt, tx.CreatedAt,
	)
	return err
}

func (r *PaymentRepo) GetTransactions(ctx context.Context, paymentID string) ([]*domain.Transaction, error) {
	var txs []*domain.Transaction
	err := r.db.SelectContext(ctx, &txs,
		`SELECT * FROM transactions WHERE payment_id = $1 ORDER BY created_at ASC`, paymentID)
	return txs, err
}

func (r *PaymentRepo) CreateRefund(ctx context.Context, refund *domain.Refund) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refunds (id, payment_id, amount, currency, status, reason, provider_ref_id, requested_by, processed_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		refund.ID, refund.PaymentID, refund.Amount, refund.Currency, refund.Status,
		refund.Reason, refund.ProviderRefID, refund.RequestedBy, refund.ProcessedAt, refund.CreatedAt,
	)
	return err
}

func (r *PaymentRepo) GetRefunds(ctx context.Context, paymentID string) ([]*domain.Refund, error) {
	var refunds []*domain.Refund
	err := r.db.SelectContext(ctx, &refunds,
		`SELECT * FROM refunds WHERE payment_id = $1 ORDER BY created_at ASC`, paymentID)
	return refunds, err
}

func (r *PaymentRepo) GetRefundByID(ctx context.Context, id string) (*domain.Refund, error) {
	var refund domain.Refund
	err := r.db.GetContext(ctx, &refund, `SELECT * FROM refunds WHERE id = $1`, id)
	return &refund, err
}

func (r *PaymentRepo) UpdateRefund(ctx context.Context, refund *domain.Refund) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE refunds SET status=$1, provider_ref_id=$2, processed_at=$3 WHERE id=$4`,
		refund.Status, refund.ProviderRefID, refund.ProcessedAt, refund.ID,
	)
	return err
}

func (r *PaymentRepo) GetTotalRefunded(ctx context.Context, paymentID string) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.db.GetContext(ctx, &total,
		`SELECT COALESCE(SUM(amount), 0) FROM refunds WHERE payment_id = $1 AND status = 'COMPLETED'`,
		paymentID,
	)
	return total, err
}

// ============================================================
// MERCHANT REPOSITORY
// ============================================================

type MerchantRepo struct {
	db *sqlx.DB
}

func NewMerchantRepo(db *sqlx.DB) *MerchantRepo {
	return &MerchantRepo{db: db}
}

func (r *MerchantRepo) GetByID(ctx context.Context, id string) (*domain.Merchant, error) {
	var m domain.Merchant
	err := r.db.GetContext(ctx, &m, `SELECT * FROM merchants WHERE id = $1`, id)
	return &m, err
}

func (r *MerchantRepo) GetByAPIKey(ctx context.Context, apiKeyHash string) (*domain.Merchant, error) {
	var m domain.Merchant
	err := r.db.GetContext(ctx, &m, `SELECT * FROM merchants WHERE api_key = $1 AND is_active = true`, apiKeyHash)
	return &m, err
}

func (r *MerchantRepo) Update(ctx context.Context, m *domain.Merchant) error {
	m.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE merchants SET name=$1, webhook_url=$2, is_active=$3, updated_at=$4 WHERE id=$5`,
		m.Name, m.WebhookURL, m.IsActive, m.UpdatedAt, m.ID,
	)
	return err
}

// ============================================================
// AUDIT REPOSITORY
// ============================================================

type AuditRepo struct {
	db *sqlx.DB
}

func NewAuditRepo(db *sqlx.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Save(ctx context.Context, log *domain.AuditLog) error {
	oldVal, _ := json.Marshal(log.OldValue)
	newVal, _ := json.Marshal(log.NewValue)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_logs (id, entity_type, entity_id, action, actor_id, actor_type, ip_address, old_value, new_value, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		log.ID, log.EntityType, log.EntityID, log.Action, log.ActorID,
		log.ActorType, log.IPAddress, string(oldVal), string(newVal), log.CreatedAt,
	)
	return err
}

// ============================================================
// WEBHOOK REPOSITORY
// ============================================================

type WebhookRepo struct {
	db *sqlx.DB
}

func NewWebhookRepo(db *sqlx.DB) *WebhookRepo {
	return &WebhookRepo{db: db}
}

func (r *WebhookRepo) Create(ctx context.Context, d *domain.WebhookDelivery) error {
	payload, _ := json.Marshal(d.Payload)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO webhook_deliveries (id, merchant_id, payment_id, event, url, payload, response_status, response_body, attempt_count, next_retry_at, is_delivered, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		d.ID, d.MerchantID, d.PaymentID, d.Event, d.URL, string(payload),
		d.ResponseStatus, d.ResponseBody, d.AttemptCount, d.NextRetryAt,
		d.IsDelivered, time.Now(), time.Now(),
	)
	return err
}

func (r *WebhookRepo) Update(ctx context.Context, d *domain.WebhookDelivery) error {
	d.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE webhook_deliveries SET response_status=$1, is_delivered=$2, attempt_count=$3, next_retry_at=$4, updated_at=$5
		WHERE id=$6`,
		d.ResponseStatus, d.IsDelivered, d.AttemptCount, d.NextRetryAt, d.UpdatedAt, d.ID,
	)
	return err
}

func (r *WebhookRepo) GetPendingRetries(ctx context.Context, limit int) ([]*domain.WebhookDelivery, error) {
	var deliveries []*domain.WebhookDelivery
	err := r.db.SelectContext(ctx, &deliveries, `
		SELECT * FROM webhook_deliveries
		WHERE is_delivered = false AND next_retry_at <= NOW() AND attempt_count < 5
		ORDER BY next_retry_at ASC LIMIT $1`, limit)
	return deliveries, err
}
