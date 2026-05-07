package natsadapter

import (
	"context"
	"time"

	dto "coupon-service/internal/dto/coupon_claim"
	couponevent "coupon-service/internal/interface/coupon_event"
	"coupon-service/internal/model"
	"coupon-service/pkg/metrics"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type CouponEventPublisher struct {
	js      nats.JetStreamContext
	subject string
	timeout time.Duration
}

func NewCouponEventPublisher(
	js nats.JetStreamContext,
	subject string,
) *CouponEventPublisher {
	return &CouponEventPublisher{
		js:      js,
		subject: subject,
		timeout: 5 * time.Second,
	}
}

//////////////////////////////////////////////////
// Publish (Context-aware)
//////////////////////////////////////////////////

func (p *CouponEventPublisher) Publish(
	ctx context.Context,
	e model.CouponEvent,
) error {

	// ใช้ context จาก caller แล้วใส่ timeout ครอบเพิ่ม
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// 🔥 map domain → protobuf
	pbEvent := dto.CouponEventToPB(e)

	payload, err := proto.Marshal(pbEvent)
	if err != nil {
		metrics.RecordEventPublished(string(e.Type), "marshal_error")
		return err
	}

	_, err = p.js.PublishMsg(
		&nats.Msg{
			Subject: p.subject,
			Data:    payload,
		},
		nats.Context(ctx),
	)

	if err != nil {
		metrics.RecordEventPublished(string(e.Type), "publish_error")
		return err
	}
	metrics.RecordEventPublished(string(e.Type), "success")
	return nil
}

//////////////////////////////////////////////////
// Compile-time check
//////////////////////////////////////////////////

var _ couponevent.CouponEventPublisher = (*CouponEventPublisher)(nil)
