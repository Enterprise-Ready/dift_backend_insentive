package health

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
)

// Status represents health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// CheckFn is a function that checks health of a dependency
type CheckFn func(ctx context.Context) error

// Checker manages health checks
type Checker struct {
	mu     sync.RWMutex
	checks map[string]CheckFn
}

func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]CheckFn),
	}
}

// Register adds a named health check
func (c *Checker) Register(name string, fn CheckFn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = fn
}

// CheckResult holds the result of a single check
type CheckResult struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency_ms"`
}

// HealthReport is the full health report
type HealthReport struct {
	Status    Status                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Service   string                 `json:"service"`
	Checks    map[string]CheckResult `json:"checks"`
}

// RunAll executes all checks concurrently with timeout
func (c *Checker) RunAll(ctx context.Context) HealthReport {
	c.mu.RLock()
	checks := make(map[string]CheckFn, len(c.checks))
	for k, v := range c.checks {
		checks[k] = v
	}
	c.mu.RUnlock()

	type result struct {
		name string
		res  CheckResult
	}

	results := make(chan result, len(checks))
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for name, fn := range checks {
		go func(n string, f CheckFn) {
			start := time.Now()
			err := f(ctx)
			latency := time.Since(start)

			r := CheckResult{
				Latency: latency.Round(time.Millisecond).String(),
			}
			if err != nil {
				r.Status = StatusUnhealthy
				r.Message = err.Error()
			} else {
				r.Status = StatusHealthy
			}
			results <- result{name: n, res: r}
		}(name, fn)
	}

	report := HealthReport{
		Status:    StatusHealthy,
		Timestamp: time.Now().UTC(),
		Checks:    make(map[string]CheckResult),
	}

	for range checks {
		r := <-results
		report.Checks[r.name] = r.res

		if r.res.Status == StatusUnhealthy && report.Status == StatusHealthy {
			report.Status = StatusDegraded
		}
	}

	return report
}

// PostgresCheck creates a health check for PostgreSQL
func PostgresCheck(db *sql.DB) CheckFn {
	return func(ctx context.Context) error {
		return db.PingContext(ctx)
	}
}

// NATSCheck creates a health check for NATS
func NATSCheck(nc *nats.Conn) CheckFn {
	return func(ctx context.Context) error {
		if nc.Status() != nats.CONNECTED {
			return nats.ErrConnectionClosed
		}
		return nil
	}
}

// GinHealthHandler returns Gin handler for health checks
func GinHealthHandler(checker *Checker, service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		report := checker.RunAll(c.Request.Context())
		report.Service = service

		httpStatus := http.StatusOK
		if report.Status == StatusUnhealthy {
			httpStatus = http.StatusServiceUnavailable
		} else if report.Status == StatusDegraded {
			httpStatus = http.StatusMultiStatus
		}

		c.JSON(httpStatus, report)
	}
}

// GinLivenessHandler returns 200 if service is alive (for k8s liveness probe)
func GinLivenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	}
}

// RegisterHealthRoutes registers /health, /ready, /live endpoints
func RegisterHealthRoutes(router *gin.Engine, checker *Checker, service string) {
	router.GET("/health", GinHealthHandler(checker, service))
	router.GET("/ready", GinHealthHandler(checker, service)) // readiness probe
	router.GET("/live", GinLivenessHandler())                // liveness probe
}
