package paymentguard

import (
	"strings"

	"github.com/enterprise/payment-gateway/internal/domain"
)

func NormalizeCurrency(currency string) string {
	if currency == "" {
		return "THB"
	}
	return strings.ToUpper(strings.TrimSpace(currency))
}
func NormalizeCreateRequest(req *domain.CreatePaymentRequest) {
	req.Currency = NormalizeCurrency(req.Currency)
	req.CustomerEmail = strings.TrimSpace(strings.ToLower(req.CustomerEmail))
}
func IsHighRiskMethod(method domain.PaymentMethod) bool {
	return method == domain.PaymentMethodCreditCard
}
