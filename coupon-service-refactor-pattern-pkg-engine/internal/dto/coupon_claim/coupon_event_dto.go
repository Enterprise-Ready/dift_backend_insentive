package dto

import (
	"coupon-service/internal/model"
	couponpb "coupon-service/proto/pb/coupon_event"
	"time"
)

func CouponEventToPB(evt model.CouponEvent) *couponpb.CouponEvent {

	return &couponpb.CouponEvent{
		Type:          couponpb.CouponEventType(couponpb.CouponEventType_value[string(evt.Type)]),
		UserId:        evt.UserID,
		CouponCode:    evt.CouponCode,
		DiscountType:  evt.DiscountType,
		DiscountValue: evt.DiscountValue,
		MinOrder:      evt.MinOrder,
		MaxDiscount:   evt.MaxDiscount,
		MaxUsage:      evt.MaxUsage,
		ValidFrom:     evt.ValidFrom.Format(time.RFC3339),
		ValidTo:       evt.ValidTo.Format(time.RFC3339),
	}

}
