package circuit

import (
	"fmt"
	"time"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

// Manager manages circuit breakers per provider
type Manager struct {
	breakers map[string]*gobreaker.CircuitBreaker
	logger   *zap.Logger
}

func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		breakers: make(map[string]*gobreaker.CircuitBreaker),
		logger:   logger,
	}
}

// GetOrCreate returns existing or creates new circuit breaker for a provider
func (m *Manager) GetOrCreate(providerName string) *gobreaker.CircuitBreaker {
	if cb, ok := m.breakers[providerName]; ok {
		return cb
	}

	settings := gobreaker.Settings{
		Name:        providerName,
		MaxRequests: 3, // max requests in half-open state
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			m.logger.Warn("circuit breaker state changed",
				zap.String("provider", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	}

	cb := gobreaker.NewCircuitBreaker(settings)
	m.breakers[providerName] = cb
	return cb
}

// Execute runs fn through the circuit breaker
func (m *Manager) Execute(providerName string, fn func() (interface{}, error)) (interface{}, error) {
	cb := m.GetOrCreate(providerName)
	result, err := cb.Execute(fn)
	if err == gobreaker.ErrOpenState {
		return nil, fmt.Errorf("circuit breaker OPEN for provider %s: service unavailable", providerName)
	}
	return result, err
}

// State returns the current state of a provider's circuit breaker
func (m *Manager) State(providerName string) string {
	if cb, ok := m.breakers[providerName]; ok {
		return cb.State().String()
	}
	return "unknown"
}
