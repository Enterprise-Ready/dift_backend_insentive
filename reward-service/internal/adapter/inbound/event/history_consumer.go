package nats

import (
	"context"
	"fmt"

	"reward-service/internal/dto"
	port "reward-service/internal/interface/earn_reward"
	eventpb "reward-service/proto/pb/event"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type HistoryConsumer struct {
	js       nats.JetStreamContext
	tripSub  string
	orderSub string
	port     port.HistoryConsumerPort
}

func NewHistoryConsumer(
	js nats.JetStreamContext,
	tripSubject string,
	orderSubject string,
	p port.HistoryConsumerPort,
) *HistoryConsumer {
	return &HistoryConsumer{
		js:       js,
		tripSub:  tripSubject,
		orderSub: orderSubject,
		port:     p,
	}
}

func (c *HistoryConsumer) Start(ctx context.Context) error {

	// ===============================
	// Trip Event
	// ===============================
	_, err := c.js.Subscribe(
		c.tripSub,
		func(msg *nats.Msg) {
			if err := c.handleTrip(ctx, msg.Data); err != nil {
				msg.Nak()
				return
			}
			msg.Ack()
		},
		nats.Durable("reward-trip-consumer"),
		nats.ManualAck(),
	)
	if err != nil {
		return err
	}

	// ===============================
	// Order Event
	// ===============================
	_, err = c.js.Subscribe(
		c.orderSub,
		func(msg *nats.Msg) {
			if err := c.handleOrder(ctx, msg.Data); err != nil {
				msg.Nak()
				return
			}
			msg.Ack()
		},
		nats.Durable("reward-order-consumer"),
		nats.ManualAck(),
	)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

func (c *HistoryConsumer) handleTrip(
	ctx context.Context,
	data []byte,
) error {

	var event eventpb.TripEvent

	if err := proto.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal trip event failed: %w", err)
	}

	earn := dto.TripEventToEarn(&event)

	return c.port.HandleEarn(ctx, earn)
}

func (c *HistoryConsumer) handleOrder(
	ctx context.Context,
	data []byte,
) error {

	var event eventpb.OrderEvent

	if err := proto.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal order event failed: %w", err)
	}

	earn := dto.OrderEventToEarn(&event)

	return c.port.HandleEarn(ctx, earn)
}
