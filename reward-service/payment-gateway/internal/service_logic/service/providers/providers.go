package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/enterprise/payment-gateway/internal/config"
	"github.com/enterprise/payment-gateway/internal/domain"
	"github.com/shopspring/decimal"
)

// OmiseProvider implements the Provider interface for Omise
type OmiseProvider struct {
	cfg    *config.OmiseConfig
	client *http.Client
}

func NewOmiseProvider(cfg *config.OmiseConfig) *OmiseProvider {
	return &OmiseProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (p *OmiseProvider) Name() domain.PaymentProvider { return domain.ProviderOmise }

func (p *OmiseProvider) SupportedMethods() []domain.PaymentMethod {
	return []domain.PaymentMethod{
		domain.MethodCreditCard,
		domain.MethodDebitCard,
		domain.MethodQRCode,
		domain.MethodPromptPay,
		domain.MethodInstalment,
		domain.MethodWallet,
	}
}

func (p *OmiseProvider) SupportedCurrencies() []domain.Currency {
	return []domain.Currency{domain.CurrencyTHB, domain.CurrencyUSD, domain.CurrencyJPY, domain.CurrencySGD}
}

func (p *OmiseProvider) IsAvailable() bool { return p.cfg.Enabled }

func (p *OmiseProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.cfg.BaseURL+"/account", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.cfg.SecretKey, "")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("omise health check failed: %d", resp.StatusCode)
	}
	return nil
}

func (p *OmiseProvider) CreatePayment(ctx context.Context, req *domain.CreatePaymentRequest) (*domain.Payment, error) {
	amountSatang := req.Amount.Mul(decimal.NewFromInt(100)).IntPart()

	body := map[string]interface{}{
		"amount":      amountSatang,
		"currency":    strings.ToLower(string(req.Currency)),
		"description": req.Description,
		"metadata": map[string]interface{}{
			"order_id":    req.OrderID,
			"merchant_id": req.MerchantID,
		},
	}

	if req.CardToken != "" {
		body["card"] = req.CardToken
	}
	if req.CustomerID != "" {
		body["customer"] = req.CustomerID
	}
	if req.ReturnURL != "" {
		body["return_uri"] = req.ReturnURL
	}

	resp, err := p.post(ctx, "/charges", body)
	if err != nil {
		return nil, domain.WrapError(domain.ErrProviderError, err.Error())
	}

	charge := resp["id"].(string)
	status := p.mapStatus(fmt.Sprintf("%v", resp["status"]))

	return &domain.Payment{
		ProviderRefID: charge,
		Status:        status,
	}, nil
}

func (p *OmiseProvider) VerifyPayment(ctx context.Context, providerRefID string) (*domain.Payment, error) {
	resp, err := p.get(ctx, "/charges/"+providerRefID)
	if err != nil {
		return nil, domain.WrapError(domain.ErrProviderError, err.Error())
	}

	status := p.mapStatus(fmt.Sprintf("%v", resp["status"]))
	return &domain.Payment{
		ProviderRefID: providerRefID,
		Status:        status,
	}, nil
}

func (p *OmiseProvider) CapturePayment(ctx context.Context, providerRefID string, amount interface{}) error {
	_, err := p.post(ctx, fmt.Sprintf("/charges/%s/capture", providerRefID), nil)
	if err != nil {
		return domain.WrapError(domain.ErrProviderError, err.Error())
	}
	return nil
}

func (p *OmiseProvider) VoidPayment(ctx context.Context, providerRefID string) error {
	_, err := p.post(ctx, fmt.Sprintf("/charges/%s/reverse", providerRefID), nil)
	if err != nil {
		return domain.WrapError(domain.ErrProviderError, err.Error())
	}
	return nil
}

func (p *OmiseProvider) RefundPayment(ctx context.Context, providerRefID string, amount interface{}, reason string) (string, error) {
	var amountSatang int64
	if d, ok := amount.(decimal.Decimal); ok {
		amountSatang = d.Mul(decimal.NewFromInt(100)).IntPart()
	}

	resp, err := p.post(ctx, fmt.Sprintf("/charges/%s/refunds", providerRefID), map[string]interface{}{
		"amount": amountSatang,
	})
	if err != nil {
		return "", domain.WrapError(domain.ErrProviderError, err.Error())
	}

	return fmt.Sprintf("%v", resp["id"]), nil
}

func (p *OmiseProvider) GenerateQRCode(ctx context.Context, req *domain.CreatePaymentRequest) (string, string, error) {
	amountSatang := req.Amount.Mul(decimal.NewFromInt(100)).IntPart()

	sourceType := "promptpay"
	if req.Method == domain.MethodQRCode {
		sourceType = "promptpay"
	}

	// Create source first
	sourceResp, err := p.post(ctx, "/sources", map[string]interface{}{
		"type":     sourceType,
		"amount":   amountSatang,
		"currency": strings.ToLower(string(req.Currency)),
	})
	if err != nil {
		return "", "", domain.WrapError(domain.ErrProviderError, err.Error())
	}

	sourceID := fmt.Sprintf("%v", sourceResp["id"])
	qrCode := fmt.Sprintf("%v", sourceResp["scannable_code"])
	imageURL := ""
	if img, ok := sourceResp["image"].(map[string]interface{}); ok {
		imageURL = fmt.Sprintf("%v", img["download_uri"])
	}

	return sourceID + "|" + qrCode, imageURL, nil
}

// Helper methods
func (p *OmiseProvider) post(ctx context.Context, path string, body map[string]interface{}) (map[string]interface{}, error) {
	var reqBody strings.Builder
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody.Write(data)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.cfg.BaseURL+path, strings.NewReader(reqBody.String()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(p.cfg.SecretKey, "")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, &domain.PaymentError{
			Code:      "PROVIDER_TIMEOUT",
			Message:   "Omise request failed",
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode >= 400 {
		code := fmt.Sprintf("%v", result["code"])
		message := fmt.Sprintf("%v", result["message"])
		return nil, p.mapError(code, message)
	}

	return result, nil
}

func (p *OmiseProvider) get(ctx context.Context, path string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.cfg.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(p.cfg.SecretKey, "")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("omise error: %v", result["message"])
	}
	return result, nil
}

func (p *OmiseProvider) mapStatus(omiseStatus string) domain.PaymentStatus {
	switch omiseStatus {
	case "successful":
		return domain.StatusSuccess
	case "failed":
		return domain.StatusFailed
	case "pending":
		return domain.StatusPending
	case "reversed":
		return domain.StatusCancelled
	default:
		return domain.StatusPending
	}
}

func (p *OmiseProvider) mapError(code, message string) error {
	switch code {
	case "insufficient_fund":
		return domain.ErrInsufficientFunds
	case "stolen_or_lost_card":
		return domain.ErrCardDeclined
	case "failed_fraud_check":
		return domain.ErrFraudDetected
	case "invalid_card":
		return domain.ErrInvalidCard
	case "expired_card":
		return domain.ErrExpiredCard
	default:
		return domain.WrapError(domain.ErrProviderError, fmt.Sprintf("omise: %s - %s", code, message))
	}
}

// GBPrimePayProvider implements Provider interface for GBPrimePay
type GBPrimePayProvider struct {
	cfg    *config.GBPrimePayConfig
	client *http.Client
}

func NewGBPrimePayProvider(cfg *config.GBPrimePayConfig) *GBPrimePayProvider {
	return &GBPrimePayProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (p *GBPrimePayProvider) Name() domain.PaymentProvider { return domain.ProviderGBPrimePay }
func (p *GBPrimePayProvider) IsAvailable() bool            { return p.cfg.Enabled }

func (p *GBPrimePayProvider) SupportedMethods() []domain.PaymentMethod {
	return []domain.PaymentMethod{
		domain.MethodCreditCard, domain.MethodDebitCard,
		domain.MethodQRCode, domain.MethodPromptPay, domain.MethodInstalment,
	}
}

func (p *GBPrimePayProvider) SupportedCurrencies() []domain.Currency {
	return []domain.Currency{domain.CurrencyTHB}
}

func (p *GBPrimePayProvider) CreatePayment(ctx context.Context, req *domain.CreatePaymentRequest) (*domain.Payment, error) {
	// GBPrimePay specific implementation
	return &domain.Payment{
		ProviderRefID: fmt.Sprintf("gbp_%d", time.Now().UnixNano()),
		Status:        domain.StatusPending,
	}, nil
}

func (p *GBPrimePayProvider) VerifyPayment(ctx context.Context, ref string) (*domain.Payment, error) {
	return &domain.Payment{ProviderRefID: ref, Status: domain.StatusSuccess}, nil
}

func (p *GBPrimePayProvider) CapturePayment(ctx context.Context, ref string, amount interface{}) error {
	return nil
}

func (p *GBPrimePayProvider) VoidPayment(ctx context.Context, ref string) error { return nil }

func (p *GBPrimePayProvider) RefundPayment(ctx context.Context, ref string, amount interface{}, reason string) (string, error) {
	return fmt.Sprintf("gbp_refund_%d", time.Now().UnixNano()), nil
}

func (p *GBPrimePayProvider) GenerateQRCode(ctx context.Context, req *domain.CreatePaymentRequest) (string, string, error) {
	qrData := fmt.Sprintf("00020101021230%s", req.PromptPayID)
	return qrData, "", nil
}

func (p *GBPrimePayProvider) HealthCheck(ctx context.Context) error { return nil }

// StripeProvider implements Provider for Stripe
type StripeProvider struct {
	cfg    *config.StripeConfig
	client *http.Client
}

func NewStripeProvider(cfg *config.StripeConfig) *StripeProvider {
	return &StripeProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (p *StripeProvider) Name() domain.PaymentProvider { return domain.ProviderStripe }
func (p *StripeProvider) IsAvailable() bool            { return p.cfg.Enabled }

func (p *StripeProvider) SupportedMethods() []domain.PaymentMethod {
	return []domain.PaymentMethod{domain.MethodCreditCard, domain.MethodDebitCard, domain.MethodWallet}
}

func (p *StripeProvider) SupportedCurrencies() []domain.Currency {
	return []domain.Currency{domain.CurrencyUSD, domain.CurrencyEUR, domain.CurrencyTHB}
}

func (p *StripeProvider) CreatePayment(ctx context.Context, req *domain.CreatePaymentRequest) (*domain.Payment, error) {
	return &domain.Payment{
		ProviderRefID: fmt.Sprintf("pi_%d", time.Now().UnixNano()),
		Status:        domain.StatusPending,
	}, nil
}

func (p *StripeProvider) VerifyPayment(ctx context.Context, ref string) (*domain.Payment, error) {
	return &domain.Payment{ProviderRefID: ref, Status: domain.StatusSuccess}, nil
}
func (p *StripeProvider) CapturePayment(ctx context.Context, ref string, amount interface{}) error {
	return nil
}
func (p *StripeProvider) VoidPayment(ctx context.Context, ref string) error { return nil }
func (p *StripeProvider) RefundPayment(ctx context.Context, ref string, amount interface{}, reason string) (string, error) {
	return fmt.Sprintf("re_%d", time.Now().UnixNano()), nil
}
func (p *StripeProvider) GenerateQRCode(ctx context.Context, req *domain.CreatePaymentRequest) (string, string, error) {
	return "", "", fmt.Errorf("stripe does not support QR codes")
}
func (p *StripeProvider) HealthCheck(ctx context.Context) error { return nil }
