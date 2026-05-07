package health

import (
	"encoding/json"
	"net/http"
	"time"

	enginehealth "github.com/PlatformCore/libpackage/observability/healthcheck"
)

func Controller(service string, version string) http.HandlerFunc {
	_ = enginehealth.NewRegistry()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"service": service, "version": version, "status": "ok", "checked_at": time.Now().UTC()})
	}
}
