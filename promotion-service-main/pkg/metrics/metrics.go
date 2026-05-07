package metrics

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	gometrics "github.com/PlatformCore/libpackage/observability/metrics"
)

type BusinessMetrics struct {
	service  string
	registry *gometrics.SimpleRegistry
	mu       sync.RWMutex
	last     map[string]time.Time
}

func New(service string) *BusinessMetrics {
	if service == "" {
		service = "promotion-service"
	}
	return &BusinessMetrics{service: service, registry: gometrics.NewSimpleRegistry(), last: map[string]time.Time{}}
}

var Default = New("promotion-service")

func (m *BusinessMetrics) Inc(name string) {
	if m == nil {
		return
	}
	m.registry.Inc(m.service + "." + name)
	m.mu.Lock()
	m.last[name] = time.Now().UTC()
	m.mu.Unlock()
}
func (m *BusinessMetrics) Add(name string, delta int64) {
	if m == nil {
		return
	}
	m.registry.Add(m.service+"."+name, delta)
	m.mu.Lock()
	m.last[name] = time.Now().UTC()
	m.mu.Unlock()
}
func (m *BusinessMetrics) Set(name string, value float64) {
	if m == nil {
		return
	}
	m.registry.Set(m.service+"."+name, value)
	m.mu.Lock()
	m.last[name] = time.Now().UTC()
	m.mu.Unlock()
}
func (m *BusinessMetrics) Observe(name string, value float64) {
	if m == nil {
		return
	}
	m.registry.Observe(m.service+"."+name, value)
	m.mu.Lock()
	m.last[name] = time.Now().UTC()
	m.mu.Unlock()
}
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Default.registry.Snapshot())
	}
}
