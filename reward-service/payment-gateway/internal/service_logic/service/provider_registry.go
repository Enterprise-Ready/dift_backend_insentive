package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/enterprise/payment-gateway/internal/domain"
)

// ============================================================
// PROVIDER INTERFACE
// ============================================================

// Provider defines the interface all payment providers must implement
type Provider interface {
	// Name returns the provider name
	Name() domain.PaymentProvider

	// SupportedMethods returns the payment methods this provider supports
	SupportedMethods() []domain.PaymentMethod

	// SupportedCurrencies returns currencies this provider supports
	SupportedCurrencies() []domain.Currency

	// CreatePayment initiates a payment
	CreatePayment(ctx context.Context, req *domain.CreatePaymentRequest) (*domain.Payment, error)

	// VerifyPayment checks payment status with the provider
	VerifyPayment(ctx context.Context, providerRefID string) (*domain.Payment, error)

	// CapturePayment captures a pre-authorized payment
	CapturePayment(ctx context.Context, providerRefID string, amount interface{}) error

	// VoidPayment voids/cancels a payment
	VoidPayment(ctx context.Context, providerRefID string) error

	// RefundPayment processes a refund
	RefundPayment(ctx context.Context, providerRefID string, amount interface{}, reason string) (string, error)

	// GenerateQRCode generates QR code data for the payment
	GenerateQRCode(ctx context.Context, req *domain.CreatePaymentRequest) (qrData string, qrImageURL string, err error)

	// IsAvailable checks if the provider is currently available
	IsAvailable() bool

	// HealthCheck performs a health check against the provider
	HealthCheck(ctx context.Context) error
}

// ============================================================
// PROVIDER REGISTRY
// ============================================================

// Registry manages all payment providers
type Registry struct {
	mu        sync.RWMutex
	providers map[domain.PaymentProvider]Provider
	// method -> []providers (ordered by priority)
	methodProviders map[domain.PaymentMethod][]domain.PaymentProvider
}

func NewRegistry() *Registry {
	return &Registry{
		providers:       make(map[domain.PaymentProvider]Provider),
		methodProviders: make(map[domain.PaymentMethod][]domain.PaymentProvider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[p.Name()] = p

	for _, method := range p.SupportedMethods() {
		r.methodProviders[method] = append(r.methodProviders[method], p.Name())
	}
}

// Get returns a provider by name
func (r *Registry) Get(name domain.PaymentProvider) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return p, nil
}

// GetForMethod returns the best available provider for a payment method
func (r *Registry) GetForMethod(method domain.PaymentMethod, preferredProvider domain.PaymentProvider) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try preferred provider first
	if preferredProvider != "" {
		if p, ok := r.providers[preferredProvider]; ok && p.IsAvailable() {
			for _, m := range p.SupportedMethods() {
				if m == method {
					return p, nil
				}
			}
		}
	}

	// Fall back to any available provider for this method
	providerNames, ok := r.methodProviders[method]
	if !ok || len(providerNames) == 0 {
		return nil, domain.ErrNoProviderAvailable
	}

	for _, name := range providerNames {
		if p, ok := r.providers[name]; ok && p.IsAvailable() {
			return p, nil
		}
	}

	return nil, domain.WrapError(domain.ErrNoProviderAvailable,
		fmt.Sprintf("no available provider for method %s", method))
}

// ListAll returns all registered providers
func (r *Registry) ListAll() map[domain.PaymentProvider]Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[domain.PaymentProvider]Provider)
	for k, v := range r.providers {
		result[k] = v
	}
	return result
}

// HealthCheck runs health checks on all providers
func (r *Registry) HealthCheck(ctx context.Context) map[string]string {
	r.mu.RLock()
	providers := make(map[domain.PaymentProvider]Provider)
	for k, v := range r.providers {
		providers[k] = v
	}
	r.mu.RUnlock()

	results := make(map[string]string)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, p := range providers {
		wg.Add(1)
		go func(n domain.PaymentProvider, prov Provider) {
			defer wg.Done()
			status := "healthy"
			if err := prov.HealthCheck(ctx); err != nil {
				status = fmt.Sprintf("unhealthy: %s", err.Error())
			}
			mu.Lock()
			results[string(n)] = status
			mu.Unlock()
		}(name, p)
	}
	wg.Wait()
	return results
}
