package natsadapter

import (
	"context"
	"log"
	"time"

	event "coupon-service/internal/interface/coupon_event"
	"coupon-service/internal/model"
	couponpb "coupon-service/proto/pb/coupon_event"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type CouponEventConsumer struct {
	js      nats.JetStreamContext
	subject string
	stream  string
	durable string
	handler event.CouponEventConsumerPort
}

func NewCouponEventConsumer(
	js nats.JetStreamContext,
	subject string,
	stream string,
	durable string,
	handler event.CouponEventConsumerPort,
) *CouponEventConsumer {
	return &CouponEventConsumer{
		js:      js,
		subject: subject,
		stream:  stream,
		durable: durable,
		handler: handler,
	}
}

func (c *CouponEventConsumer) Start(ctx context.Context) error {

	sub, err := c.js.PullSubscribe(
		c.subject,
		c.durable,
		nats.BindStream(c.stream),
	)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			msgs, err := sub.Fetch(10, nats.MaxWait(2*time.Second))
			if err != nil {
				continue
			}

			for _, msg := range msgs {

				var pbEvent couponpb.CouponEvent
				if err := proto.Unmarshal(msg.Data, &pbEvent); err != nil {
					_ = msg.Term()
					continue
				}

				// 🔥 map protobuf -> domain model
				event := model.CouponEvent{
					Type:          model.CouponEventType(pbEvent.Type.String()),
					UserID:        pbEvent.UserId,
					CouponCode:    pbEvent.CouponCode,
					DiscountType:  pbEvent.DiscountType,
					DiscountValue: pbEvent.DiscountValue,
					MinOrder:      pbEvent.MinOrder,
					MaxDiscount:   pbEvent.MaxDiscount,
					//MaxUsage:      int(pbEvent.MaxUsage),
					// parse time ถ้าต้องการ
				}

				if err := c.handler.Handle(event); err != nil {
					log.Printf("handler error: %v", err)
					_ = msg.Nak()
					continue
				}

				_ = msg.Ack()
			}
		}
	}()

	return nil
}
