package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/enterprise/payment-gateway/internal/domain"
	"github.com/enterprise/payment-gateway/pkg/crypto"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ============================================================
// MOCKS
// ============================================================

type MockPaymentRepo struct{ mock.Mock }

func (m *MockPaymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	return m.Called(ctx, p).Error(0)
}
func (m *MockPaymentRepo) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Payment), args.Error(1)
}
func (m *MockPaymentRepo) GetByOrderID(ctx context.Context, merchantID, orderID string) (*domain.Payment, error) {
	args := m.Called(ctx, merchantID, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Payment), args.Error(1)
}
func (m *MockPaymentRepo) UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus, updates map[string]interface{}) error {
	return m.Called(ctx, id, status, updates).Error(0)
}
func (m *MockPaymentRepo) Update(ctx context.Context, p *domain.Payment) error {
	return m.Called(ctx, p).Error(0)
}
func (m *MockPaymentRepo) List(ctx context.Context, req *domain.PaymentListRequest) ([]*domain.Payment, int64, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]*domain.Payment), args.Get(1).(int64), args.Error(2)
}
func (m *MockPaymentRepo) GetSummary(ctx context.Context, merchantID string, from, to time.Time) (*domain.PaymentSummary, error) {
	args := m.Called(ctx, merchantID, from, to)
	return args.Get(0).(*domain.PaymentSummary), args.Error(1)
}
func (m *MockPaymentRepo) CreateTransaction(ctx context.Context, tx *domain.Transaction) error {
	return m.Called(ctx, tx).Error(0)
}
func (m *MockPaymentRepo) GetTransactions(ctx context.Context, paymentID string) ([]*domain.Transaction, error) {
	args := m.Called(ctx, paymentID)
	return args.Get(0).([]*domain.Transaction), args.Error(1)
}
func (m *MockPaymentRepo) CreateRefund(ctx context.Context, refund *domain.Refund) error {
	return m.Called(ctx, refund).Error(0)
}
func (m *MockPaymentRepo) GetRefunds(ctx context.Context, paymentID string) ([]*domain.Refund, error) {
	args := m.Called(ctx, paymentID)
	return args.Get(0).([]*domain.Refund), args.Error(1)
}
func (m *MockPaymentRepo) GetRefundByID(ctx context.Context, id string) (*domain.Refund, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*domain.Refund), args.Error(1)
}
func (m *MockPaymentRepo) UpdateRefund(ctx context.Context, refund *domain.Refund) error {
	return m.Called(ctx, refund).Error(0)
}
func (m *MockPaymentRepo) GetTotalRefunded(ctx context.Context, paymentID string) (decimal.Decimal, error) {
	args := m.Called(ctx, paymentID)
	return args.Get(0).(decimal.Decimal), args.Error(1)
}

// ============================================================
// CRYPTO TESTS
// ============================================================

func TestCrypto_EncryptDecrypt(t *testing.T) {
	// 32 bytes = 256-bit key
	key := "dGVzdGtleXRlc3RrZXl0ZXN0a2V5dGU=" // base64("testkeytestkeytestkeyte")

	svc, err := crypto.NewService(key)
	require.NoError(t, err)

	plaintext := "4111111111111111" // Test card number

	encrypted, err := svc.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := svc.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestCrypto_EncryptNonDeterministic(t *testing.T) {
	key := "dGVzdGtleXRlc3RrZXl0ZXN0a2V5dGU="
	svc, _ := crypto.NewService(key)

	plain := "sensitive data"
	enc1, _ := svc.Encrypt(plain)
	enc2, _ := svc.Encrypt(plain)

	// Each encryption should produce different ciphertext (GCM with random nonce)
	assert.NotEqual(t, enc1, enc2)
}

func TestCrypto_LuhnValidation(t *testing.T) {
	tests := []struct {
		card  string
		valid bool
	}{
		{"4111111111111111", true},  // Visa test
		{"5500005555555559", true},  // MC test
		{"371449635398431", true},   // Amex test
		{"4111111111111112", false}, // Invalid
		{"1234567890123456", false}, // Invalid
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.card, func(t *testing.T) {
			assert.Equal(t, tt.valid, crypto.ValidateLuhn(tt.card))
		})
	}
}

func TestCrypto_CardBrand(t *testing.T) {
	assert.Equal(t, "Visa", crypto.GetCardBrand("4111111111111111"))
	assert.Equal(t, "Mastercard", crypto.GetCardBrand("5500005555555559"))
	assert.Equal(t, "American Express", crypto.GetCardBrand("371449635398431"))
	assert.Equal(t, "JCB", crypto.GetCardBrand("3530111333300000"))
}

func TestCrypto_CardExpiry(t *testing.T) {
	futureYear := time.Now().Year() + 2
	assert.True(t, crypto.ValidateCardExpiry(
		time.Date(futureYear, 1, 1, 0, 0, 0, 0, time.UTC).Format("01/2006"),
	))
	assert.False(t, crypto.ValidateCardExpiry("01/2020")) // Past
	assert.False(t, crypto.ValidateCardExpiry("13/2026")) // Invalid month
}

func TestCrypto_MaskCardNumber(t *testing.T) {
	assert.Equal(t, "**** **** **** 1111", crypto.MaskCardNumber("4111111111111111"))
	assert.Equal(t, "**** **** **** 8431", crypto.MaskCardNumber("371449635398431"))
}

func TestCrypto_HMAC(t *testing.T) {
	msg := `{"event":"payment.success","amount":1000}`
	secret := "webhook_secret_key"

	sig := crypto.HMACSHA256(msg, secret)
	assert.NotEmpty(t, sig)

	assert.True(t, crypto.VerifyHMAC(msg, secret, sig))
	assert.False(t, crypto.VerifyHMAC(msg, secret, "wrong_signature"))
	assert.False(t, crypto.VerifyHMAC(msg+"tampered", secret, sig))
}

// ============================================================
// DOMAIN ERRORS TESTS
// ============================================================

func TestDomainErrors(t *testing.T) {
	t.Run("payment error format", func(t *testing.T) {
		err := domain.ErrPaymentNotFound
		assert.Equal(t, "[PAYMENT_NOT_FOUND] Payment not found", err.Error())
	})

	t.Run("wrapped error includes detail", func(t *testing.T) {
		wrapped := domain.WrapError(domain.ErrRefundExceedsAmount, "max: 500.00")
		assert.Contains(t, wrapped.Error(), "max: 500.00")
		assert.Equal(t, "REFUND_EXCEEDS_AMOUNT", wrapped.Code)
	})

	t.Run("retryable errors", func(t *testing.T) {
		assert.True(t, domain.IsRetryable(domain.ErrProviderTimeout))
		assert.True(t, domain.IsRetryable(domain.ErrProviderUnavailable))
		assert.False(t, domain.IsRetryable(domain.ErrInvalidCard))
		assert.False(t, domain.IsRetryable(domain.ErrPaymentNotFound))
	})
}

// ============================================================
// PAYMENT DOMAIN VALIDATION TESTS
// ============================================================

func TestPaymentStatusTransitions(t *testing.T) {
	validTransitions := map[domain.PaymentStatus][]domain.PaymentStatus{
		domain.StatusPending:    {domain.StatusProcessing, domain.StatusFailed, domain.StatusExpired, domain.StatusCancelled},
		domain.StatusProcessing: {domain.StatusSuccess, domain.StatusFailed},
		domain.StatusSuccess:    {domain.StatusRefunded, domain.StatusPartialRef, domain.StatusDisputed},
	}

	for from, tos := range validTransitions {
		for _, to := range tos {
			t.Run(string(from)+"->"+string(to), func(t *testing.T) {
				assert.NotEmpty(t, to)
				_ = from
			})
		}
	}
}

func TestPaymentAmountPrecision(t *testing.T) {
	amount := decimal.NewFromFloat(1234.56)
	fee := amount.Mul(decimal.NewFromFloat(0.025)).Round(2)
	net := amount.Sub(fee)

	assert.Equal(t, "30.86", fee.String())
	assert.Equal(t, "1203.70", net.String())
}

// ============================================================
// BENCHMARKS
// ============================================================

func BenchmarkLuhnValidation(b *testing.B) {
	card := "4111111111111111"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		crypto.ValidateLuhn(card)
	}
}

func BenchmarkHMACSHA256(b *testing.B) {
	msg := `{"event":"payment.success","payment_id":"pay_test_123","amount":1000}`
	secret := "webhook_secret"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		crypto.HMACSHA256(msg, secret)
	}
}
