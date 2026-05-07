package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"coupon-service/config"
	httpadapter "coupon-service/internal/adapter/inbound/http"
	publichttp "coupon-service/internal/adapter/inbound/http/claimquery"
	httpinfra "coupon-service/internal/integration/http"
	"coupon-service/internal/route"
	"coupon-service/pkg/health"
	couponmetrics "coupon-service/pkg/metrics"
)

func wireHTTPRouter(cfg config.Config, features config.FeatureFlags, db *sql.DB, publicH *publichttp.CouponHTTPHandler) http.Handler {
	_ = cfg
	_ = features
	r := chi.NewRouter()
	r.Get("/metrics/business", couponmetrics.Handler())
	r.Get("/health", health.Controller("coupon-service", "v1"))
	r.Get("/ready", readinessHandler(db))
	r.Post("/internal/admin/control", adminControlHandler())
	if publicH != nil {
		route.RegisterRoutes(r, publicH)
	}
	return httpadapter.BuildSharedHTTPMiddleware("coupon-service", 10*time.Second)(r)
}

func wireHTTPServer(cfg config.Config, features config.FeatureFlags, router http.Handler) *httpinfra.Server {
	if !features.EnableHTTP || router == nil {
		return nil
	}
	return httpinfra.NewServer(normalizeAddress(cfg.HTTP.Port), router)
}

func readinessHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "not_ready", "reason": "database_disabled"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "not_ready", "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ready"})
	}
}

func adminControlHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := strings.TrimSpace(os.Getenv("ADMIN_CONTROL_SHARED_SECRET"))
		if secret != "" && r.Header.Get("X-Admin-Secret") != secret {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_json"})
			return
		}
		action, _ := body["action"].(string)
		if strings.TrimSpace(action) == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "action_required"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"accepted": true, "service": "coupon-service", "action": action})
	}
}
