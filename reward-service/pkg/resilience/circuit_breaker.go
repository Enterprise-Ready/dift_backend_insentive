package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// State represents circuit breaker state
type State int

const (
	StateClosed   State = iota // Normal operation
	StateOpen                  // Failing - reject calls fast
	StateHalfOpen              // Testing if service recovered
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name string
	cfg  CircuitBreakerConfig

	mu              sync.RWMutex
	state           State
	failures        int
	successes       int
	lastFailure     time.Time
	lastStateChange time.Time
	totalRequests   int64
	totalFailures   int64
	totalRejected   int64

	onStateChange func(name string, from, to State)
}

type CircuitBreakerConfig struct {
	// How many consecutive failures before opening circuit
	FailureThreshold int
	// How many consecutive successes before closing from half-open
	SuccessThreshold int
	// How long to wait in open state before allowing test request
	Timeout time.Duration
	// Optional callback when state changes
	OnStateChange func(name string, from, to State)
}

func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

func NewCircuitBreaker(name string, cfg CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:            name,
		cfg:             cfg,
		state:           StateClosed,
		lastStateChange: time.Now(),
		onStateChange:   cfg.OnStateChange,
	}
}

// Execute runs fn if circuit allows, returns ErrCircuitOpen if not
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := cb.beforeCall(); err != nil {
		cb.mu.Lock()
		cb.totalRejected++
		cb.mu.Unlock()
		return err
	}

	err := fn(ctx)
	cb.afterCall(err)
	return err
}

func (cb *CircuitBreaker) beforeCall() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++

	switch cb.state {
	case StateClosed:
		return nil

	case StateOpen:
		// Check if timeout elapsed → try half-open
		if time.Since(cb.lastFailure) >= cb.cfg.Timeout {
			cb.setState(StateHalfOpen)
			return nil
		}
		return fmt.Errorf("%w: service=%s failures=%d last_failure=%v ago",
			ErrCircuitOpen, cb.name, cb.failures, time.Since(cb.lastFailure).Round(time.Second))

	case StateHalfOpen:
		// Allow one request through to test
		return nil
	}

	return nil
}

func (cb *CircuitBreaker) afterCall(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.successes = 0
	cb.lastFailure = time.Now()
	cb.totalFailures++

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.cfg.FailureThreshold {
			cb.setState(StateOpen)
		}
	case StateHalfOpen:
		cb.setState(StateOpen)
	}
}

func (cb *CircuitBreaker) onSuccess() {
	cb.failures = 0
	cb.successes++

	switch cb.state {
	case StateHalfOpen:
		if cb.successes >= cb.cfg.SuccessThreshold {
			cb.setState(StateClosed)
		}
	}
}

func (cb *CircuitBreaker) setState(new State) {
	old := cb.state
	cb.state = new
	cb.lastStateChange = time.Now()

	if old != new && cb.onStateChange != nil {
		go cb.onStateChange(cb.name, old, new)
	}
}

// Stats returns circuit breaker statistics
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		Name:            cb.name,
		State:           cb.state.String(),
		Failures:        cb.failures,
		TotalRequests:   cb.totalRequests,
		TotalFailures:   cb.totalFailures,
		TotalRejected:   cb.totalRejected,
		LastStateChange: cb.lastStateChange,
	}
}

type CircuitBreakerStats struct {
	Name            string    `json:"name"`
	State           string    `json:"state"`
	Failures        int       `json:"current_failures"`
	TotalRequests   int64     `json:"total_requests"`
	TotalFailures   int64     `json:"total_failures"`
	TotalRejected   int64     `json:"total_rejected"`
	LastStateChange time.Time `json:"last_state_change"`
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

func (r *CircuitBreakerRegistry) GetOrCreate(name string, cfg CircuitBreakerConfig) *CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, ok := r.breakers[name]; ok {
		return cb
	}

	cb := NewCircuitBreaker(name, cfg)
	r.breakers[name] = cb
	return cb
}

func (r *CircuitBreakerRegistry) AllStats() []CircuitBreakerStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make([]CircuitBreakerStats, 0, len(r.breakers))
	for _, cb := range r.breakers {
		stats = append(stats, cb.Stats())
	}
	return stats
}
