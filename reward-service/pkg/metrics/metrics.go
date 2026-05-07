package metrics

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	_ "github.com/PlatformCore/libpackage/observability/metrics"
)

type BusinessMetrics struct {
	EarnAccepted    atomic.Int64
	RedeemRequested atomic.Int64
	RedeemSucceeded atomic.Int64
	RedeemFailed    atomic.Int64
}

func NewBusinessMetrics() *BusinessMetrics { return &BusinessMetrics{} }
func (m *BusinessMetrics) Snapshot() map[string]int64 {
	return map[string]int64{
		"reward_earn_accepted_total":    m.EarnAccepted.Load(),
		"reward_redeem_requested_total": m.RedeemRequested.Load(),
		"reward_redeem_succeeded_total": m.RedeemSucceeded.Load(),
		"reward_redeem_failed_total":    m.RedeemFailed.Load(),
	}
}
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
}
func BusinessHandler(m *BusinessMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m.Snapshot())
	}
}
