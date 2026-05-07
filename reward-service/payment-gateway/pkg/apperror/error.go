package apperror

import "github.com/enterprise/payment-gateway/internal/domain"

func New(code, message string, status int) *domain.PaymentError {
	return &domain.PaymentError{Code: code, Message: message, HTTPStatus: status}
}
