package route

import (
	publichandler "coupon-service/internal/adapter/inbound/http/claimquery"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, publicH *publichandler.CouponHTTPHandler) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/coupons", func(r chi.Router) {
			r.Get("/", publicH.ListCoupons)
			r.Post("/{code}/claim", publicH.ClaimCoupon)
		})
	})
}
