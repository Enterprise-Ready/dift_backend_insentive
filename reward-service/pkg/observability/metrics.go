package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for reward-service
type Metrics struct {
	// HTTP metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge

	// Business metrics
	EarnProcessedTotal   *prometheus.CounterVec // label: source (trip/order)
	EarnPointsTotal      *prometheus.CounterVec // label: source
	RedeemRequestsTotal  *prometheus.CounterVec // label: status (success/fail)
	RedeemPointsTotal    *prometheus.CounterVec // label: status
	DuplicateEarnTotal   prometheus.Counter
	DuplicateRedeemTotal prometheus.Counter

	// NATS metrics
	NATSPublishTotal    *prometheus.CounterVec // label: subject, status
	NATSConsumeTotal    *prometheus.CounterVec // label: subject, status
	NATSPublishDuration *prometheus.HistogramVec

	// DB metrics
	DBQueryDuration *prometheus.HistogramVec // label: operation
	DBErrorsTotal   *prometheus.CounterVec   // label: operation

	// Circuit breaker metrics
	CircuitBreakerState   *prometheus.GaugeVec   // label: name (0=closed,1=open,2=half-open)
	CircuitBreakerTripped *prometheus.CounterVec // label: name
}

// NewMetrics creates and registers all metrics
func NewMetrics(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)

	return &Metrics{
		// ── HTTP ──────────────────────────────────────────────
		HTTPRequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "reward_http_requests_total",
			Help: "Total HTTP requests by method, path, status",
		}, []string{"method", "path", "status"}),

		HTTPRequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "reward_http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),

		HTTPRequestsInFlight: factory.NewGauge(prometheus.GaugeOpts{
			Name: "reward_http_requests_in_flight",
			Help: "Current in-flight HTTP requests",
		}),

		// ── Business ──────────────────────────────────────────
		EarnProcessedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "reward_earn_processed_total",
			Help: "Total earn events processed",
		}, []string{"source"}),

		EarnPointsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "reward_earn_points_total",
			Help: "Total points earned",
		}, []string{"source"}),

		RedeemRequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "reward_redeem_requests_total",
			Help: "Total redeem requests",
		}, []string{"status"}),

		RedeemPointsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "reward_redeem_points_total",
			Help: "Total points redeemed",
		}, []string{"status"}),

		DuplicateEarnTotal: factory.NewCounter(prometheus.CounterOpts{
			Name: "reward_duplicate_earn_total",
			Help: "Total duplicate earn events dropped",
		}),

		DuplicateRedeemTotal: factory.NewCounter(prometheus.CounterOpts{
			Name: "reward_duplicate_redeem_total",
			Help: "Total duplicate redeem requests dropped",
		}),

		// ── NATS ──────────────────────────────────────────────
		NATSPublishTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "reward_nats_publish_total",
			Help: "Total NATS messages published",
		}, []string{"subject", "status"}),

		NATSConsumeTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "reward_nats_consume_total",
			Help: "Total NATS messages consumed",
		}, []string{"subject", "status"}),

		NATSPublishDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "reward_nats_publish_duration_seconds",
			Help:    "NATS publish latency",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .5, 1},
		}, []string{"subject"}),

		// ── DB ────────────────────────────────────────────────
		DBQueryDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "reward_db_query_duration_seconds",
			Help:    "Database query latency",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .5, 1, 2.5},
		}, []string{"operation"}),

		DBErrorsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "reward_db_errors_total",
			Help: "Total database errors",
		}, []string{"operation"}),

		// ── Circuit Breaker ───────────────────────────────────
		CircuitBreakerState: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "reward_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		}, []string{"name"}),

		CircuitBreakerTripped: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "reward_circuit_breaker_tripped_total",
			Help: "Total times circuit breaker was tripped",
		}, []string{"name"}),
	}
}

// PrometheusMiddleware records HTTP metrics for each request
func PrometheusMiddleware(m *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		m.HTTPRequestsInFlight.Inc()
		defer m.HTTPRequestsInFlight.Dec()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		m.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		m.HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// MetricsHandler returns the Prometheus metrics HTTP handler
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RegisterMetricsRoute registers /metrics endpoint on router
func RegisterMetricsRoute(router *gin.Engine) {
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
