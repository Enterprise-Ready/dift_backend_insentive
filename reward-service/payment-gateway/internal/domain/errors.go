package domain

import (
	"fmt"
	"net/http"
)

// PaymentError is a structured error type for the payment domain
type PaymentError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Detail     string `json:"detail,omitempty"`
	HTTPStatus int    `json:"-"`
	Retryable  bool   `json:"retryable"`
}

func (e *PaymentError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Sentinel errors for all payment scenarios
var (
	// Validation errors
	ErrInvalidAmount      = &PaymentError{Code: "INVALID_AMOUNT", Message: "Amount must be greater than zero", HTTPStatus: http.StatusBadRequest}
	ErrInvalidCurrency    = &PaymentError{Code: "INVALID_CURRENCY", Message: "Unsupported currency", HTTPStatus: http.StatusBadRequest}
	ErrInvalidMethod      = &PaymentError{Code: "INVALID_METHOD", Message: "Invalid payment method", HTTPStatus: http.StatusBadRequest}
	ErrInvalidCard        = &PaymentError{Code: "INVALID_CARD", Message: "Invalid card details", HTTPStatus: http.StatusBadRequest}
	ErrInvalidCardNumber  = &PaymentError{Code: "INVALID_CARD_NUMBER", Message: "Card number is invalid", HTTPStatus: http.StatusBadRequest}
	ErrExpiredCard        = &PaymentError{Code: "EXPIRED_CARD", Message: "Card has expired", HTTPStatus: http.StatusBadRequest}
	ErrInvalidCVV         = &PaymentError{Code: "INVALID_CVV", Message: "CVV is incorrect", HTTPStatus: http.StatusBadRequest}
	ErrInvalidPromptPayID = &PaymentError{Code: "INVALID_PROMPTPAY_ID", Message: "PromptPay ID is invalid", HTTPStatus: http.StatusBadRequest}

	// Payment state errors
	ErrPaymentNotFound     = &PaymentError{Code: "PAYMENT_NOT_FOUND", Message: "Payment not found", HTTPStatus: http.StatusNotFound}
	ErrPaymentExpired      = &PaymentError{Code: "PAYMENT_EXPIRED", Message: "Payment has expired", HTTPStatus: http.StatusGone}
	ErrPaymentAlreadyPaid  = &PaymentError{Code: "ALREADY_PAID", Message: "Payment already completed", HTTPStatus: http.StatusConflict}
	ErrPaymentCancelled    = &PaymentError{Code: "PAYMENT_CANCELLED", Message: "Payment has been cancelled", HTTPStatus: http.StatusConflict}
	ErrInvalidStatusChange = &PaymentError{Code: "INVALID_STATUS_CHANGE", Message: "Invalid payment status transition", HTTPStatus: http.StatusConflict}

	// Refund errors
	ErrRefundExceedsAmount = &PaymentError{Code: "REFUND_EXCEEDS_AMOUNT", Message: "Refund amount exceeds original payment", HTTPStatus: http.StatusBadRequest}
	ErrRefundNotAllowed    = &PaymentError{Code: "REFUND_NOT_ALLOWED", Message: "Refund not allowed for this payment", HTTPStatus: http.StatusBadRequest}
	ErrRefundWindowClosed  = &PaymentError{Code: "REFUND_WINDOW_CLOSED", Message: "Refund window has closed", HTTPStatus: http.StatusBadRequest}
	ErrRefundNotFound      = &PaymentError{Code: "REFUND_NOT_FOUND", Message: "Refund not found", HTTPStatus: http.StatusNotFound}

	// Provider errors (retryable)
	ErrProviderTimeout     = &PaymentError{Code: "PROVIDER_TIMEOUT", Message: "Payment provider timed out", HTTPStatus: http.StatusGatewayTimeout, Retryable: true}
	ErrProviderUnavailable = &PaymentError{Code: "PROVIDER_UNAVAILABLE", Message: "Payment provider is unavailable", HTTPStatus: http.StatusServiceUnavailable, Retryable: true}
	ErrProviderError       = &PaymentError{Code: "PROVIDER_ERROR", Message: "Payment provider returned an error", HTTPStatus: http.StatusBadGateway, Retryable: true}
	ErrNoProviderAvailable = &PaymentError{Code: "NO_PROVIDER_AVAILABLE", Message: "No payment provider available for this method", HTTPStatus: http.StatusServiceUnavailable}

	// Auth errors
	ErrUnauthorized     = &PaymentError{Code: "UNAUTHORIZED", Message: "Authentication required", HTTPStatus: http.StatusUnauthorized}
	ErrForbidden        = &PaymentError{Code: "FORBIDDEN", Message: "Access denied", HTTPStatus: http.StatusForbidden}
	ErrInvalidAPIKey    = &PaymentError{Code: "INVALID_API_KEY", Message: "API key is invalid or expired", HTTPStatus: http.StatusUnauthorized}
	ErrMerchantNotFound = &PaymentError{Code: "MERCHANT_NOT_FOUND", Message: "Merchant not found", HTTPStatus: http.StatusNotFound}
	ErrMerchantInactive = &PaymentError{Code: "MERCHANT_INACTIVE", Message: "Merchant account is inactive", HTTPStatus: http.StatusForbidden}

	// Rate limit errors
	ErrRateLimitExceeded  = &PaymentError{Code: "RATE_LIMIT_EXCEEDED", Message: "Too many requests", HTTPStatus: http.StatusTooManyRequests}
	ErrDailyLimitExceeded = &PaymentError{Code: "DAILY_LIMIT_EXCEEDED", Message: "Daily payment limit exceeded", HTTPStatus: http.StatusForbidden}
	ErrTransactionLimit   = &PaymentError{Code: "TRANSACTION_LIMIT", Message: "Transaction limit exceeded", HTTPStatus: http.StatusForbidden}

	// Idempotency errors
	ErrIdempotencyConflict = &PaymentError{Code: "IDEMPOTENCY_CONFLICT", Message: "Idempotency key conflict", HTTPStatus: http.StatusConflict}
	ErrDuplicatePayment    = &PaymentError{Code: "DUPLICATE_PAYMENT", Message: "Duplicate payment detected", HTTPStatus: http.StatusConflict}

	// Risk errors
	ErrHighRiskTransaction = &PaymentError{Code: "HIGH_RISK", Message: "Transaction flagged as high risk", HTTPStatus: http.StatusForbidden}
	ErrFraudDetected       = &PaymentError{Code: "FRAUD_DETECTED", Message: "Fraudulent activity detected", HTTPStatus: http.StatusForbidden}
	ErrBlacklisted         = &PaymentError{Code: "BLACKLISTED", Message: "Entity is blacklisted", HTTPStatus: http.StatusForbidden}

	// Insufficient funds
	ErrInsufficientFunds = &PaymentError{Code: "INSUFFICIENT_FUNDS", Message: "Insufficient funds", HTTPStatus: http.StatusPaymentRequired}
	ErrCardDeclined      = &PaymentError{Code: "CARD_DECLINED", Message: "Card was declined", HTTPStatus: http.StatusPaymentRequired}
	ErrDoNotHonor        = &PaymentError{Code: "DO_NOT_HONOR", Message: "Bank declined the transaction", HTTPStatus: http.StatusPaymentRequired}

	// Internal
	ErrInternal      = &PaymentError{Code: "INTERNAL_ERROR", Message: "Internal server error", HTTPStatus: http.StatusInternalServerError}
	ErrDatabaseError = &PaymentError{Code: "DATABASE_ERROR", Message: "Database operation failed", HTTPStatus: http.StatusInternalServerError}
)

// Wrap creates a new error with additional context
func WrapError(base *PaymentError, detail string) *PaymentError {
	return &PaymentError{
		Code:       base.Code,
		Message:    base.Message,
		Detail:     detail,
		HTTPStatus: base.HTTPStatus,
		Retryable:  base.Retryable,
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if pe, ok := err.(*PaymentError); ok {
		return pe.Retryable
	}
	return false
}
