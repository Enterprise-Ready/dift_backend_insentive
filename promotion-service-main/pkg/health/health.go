package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	healthcheck "github.com/PlatformCore/libpackage/observability/healthcheck"
)

type Checker struct {
	Service string `json:"service"`
	Version string `json:"version"`
	deps    *healthcheck.DependencyRegistry
}

func New(service, version string) *Checker {
	if service == "" {
		service = "promotion-service"
	}
	if version == "" {
		version = "dev"
	}
	return &Checker{Service: service, Version: version, deps: healthcheck.NewDependencyRegistry(2 * time.Second)}
}

func (c *Checker) Register(name string, critical bool, fn healthcheck.DependencyCheck) {
	c.deps.Register(name, critical, fn)
}
func (c *Checker) LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"service": c.Service, "version": c.Version, "status": "ok", "time": time.Now().UTC()})
	}
}
func (c *Checker) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results := c.deps.Check(r.Context())
		overall := healthcheck.Overall(results)
		if overall == healthcheck.StatusDown {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"service": c.Service, "status": overall, "dependencies": results})
	}
}

func AlwaysUp(context.Context) error { return nil }
