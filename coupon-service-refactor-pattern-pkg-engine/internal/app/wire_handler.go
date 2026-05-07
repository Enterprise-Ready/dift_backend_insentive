package app

import (
	adminhandler "coupon-service/internal/service_logic/handler/admin"
	grpchandler "coupon-service/internal/service_logic/handler/grpc"
	publichandler "coupon-service/internal/service_logic/handler/public"
)

func wireHandlers(services Services) Handlers {
	var h Handlers
	if services.Query != nil && services.Claim != nil {
		h.Public = publichandler.NewCouponPublicHandler(services.Query, services.Claim)
	}
	if services.Admin != nil {
		h.Admin = adminhandler.NewCouponAdminHandler(services.Admin)
	}
	if services.Calculator != nil {
		h.GRPC = grpchandler.NewCouponHandler(services.Calculator)
	}
	return h
}
