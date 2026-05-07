package public

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	dto "coupon-service/internal/dto/http/public"
	httpport "coupon-service/internal/interface/http/public"
	claimservice "coupon-service/internal/service_logic/service/claim"
)

type CouponHTTPHandler struct {
	handler httpport.CouponPort
}

func NewCouponHTTPHandler(
	handler httpport.CouponPort,
) *CouponHTTPHandler {
	return &CouponHTTPHandler{
		handler: handler,
	}
}

// ======================
// GET /coupons
// ======================
func (h *CouponHTTPHandler) ListCoupons(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	coupons, err := h.handler.ListActiveCoupons(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]dto.CouponResponse, 0, len(coupons))
	for _, c := range coupons {
		resp = append(resp, dto.FromCoupon(c))
	}

	writeJSON(w, http.StatusOK, resp)
}

// ======================
// POST /coupons/{code}/claim
// ======================
func (h *CouponHTTPHandler) ClaimCoupon(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "missing idempotency key")
		return
	}

	code := chi.URLParam(r, "code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing coupon code")
		return
	}

	err := h.handler.ClaimCoupon(ctx, userID, code, idempotencyKey)
	if err != nil {

		switch err {

		case claimservice.ErrInvalidInput:
			writeError(w, http.StatusBadRequest, err.Error())

		case claimservice.ErrCouponNotFound:
			writeError(w, http.StatusNotFound, err.Error())

		case claimservice.ErrCouponInactive:
			writeError(w, http.StatusBadRequest, err.Error())

		case claimservice.ErrQuotaExceeded:
			writeError(w, http.StatusConflict, err.Error())

		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

// ======================
// Helper Functions
// ======================

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
	})
}
