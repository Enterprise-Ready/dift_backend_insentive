package metrics

import (
	"net/http"
	"time"

	enginemetrics "github.com/PlatformCore/libpackage/observability/metrics"
)

var (
	claimsTotal       = enginemetrics.NewCounter("coupon_claim_total", "Coupon claim attempts by status", "status")
	adminCommands     = enginemetrics.NewCounter("coupon_admin_command_total", "Coupon admin commands by action/status", "action", "status")
	calculationsTotal = enginemetrics.NewCounter("coupon_calculation_total", "Coupon calculation attempts by status", "status")
	queriesTotal      = enginemetrics.NewCounter("coupon_query_total", "Coupon query operations by status", "status")
	eventsPublished   = enginemetrics.NewCounter("coupon_event_published_total", "Coupon events published by type/status", "event_type", "status")
	operationLatency  = enginemetrics.NewHistogram("coupon_operation_latency_seconds", "Coupon business operation latency", []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5}, "operation", "status")
)

func Handler() http.HandlerFunc { return enginemetrics.DefaultRegistry.Handler() }

func RecordClaim(status string, started time.Time) {
	claimsTotal.Inc(status)
	operationLatency.Observe(time.Since(started).Seconds(), "claim", status)
}
func RecordAdminCommand(action string, status string, started time.Time) {
	adminCommands.Inc(action, status)
	operationLatency.Observe(time.Since(started).Seconds(), "admin_"+action, status)
}
func RecordCalculation(status string, started time.Time) {
	calculationsTotal.Inc(status)
	operationLatency.Observe(time.Since(started).Seconds(), "calculate", status)
}
func RecordQuery(status string, started time.Time) {
	queriesTotal.Inc(status)
	operationLatency.Observe(time.Since(started).Seconds(), "query", status)
}
func RecordEventPublished(eventType string, status string) {
	eventsPublished.Inc(eventType, status)
}
