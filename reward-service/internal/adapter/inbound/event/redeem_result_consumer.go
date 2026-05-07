package nats

import (
	"context"
	"fmt"

	"reward-service/internal/dto"
	eventport "reward-service/internal/interface/event_consumer"
	redeempb "reward-service/proto/pb/redeem"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type RedeemResultConsumer struct {
	nc      *nats.Conn
	subject string
	handler eventport.RedeemResultConsumer
}

func NewRedeemResultConsumer(
	nc *nats.Conn,
	subject string,
	handler eventport.RedeemResultConsumer,
) *RedeemResultConsumer {

	return &RedeemResultConsumer{
		nc:      nc,
		subject: subject,
		handler: handler,
	}
}

func (c *RedeemResultConsumer) Start(ctx context.Context) error {

	_, err := c.nc.Subscribe(c.subject, func(msg *nats.Msg) {

		var pbMsg redeempb.RedeemResult

		if err := proto.Unmarshal(msg.Data, &pbMsg); err != nil {
			fmt.Println("unmarshal redeem result failed:", err)
			return
		}

		result := dto.RedeemResultFromPB(&pbMsg)

		if err := c.handler.HandleResult(ctx, result); err != nil {
			fmt.Println("handle redeem result failed:", err)
			return
		}
	})

	if err != nil {
		return fmt.Errorf("nats subscribe failed: %w", err)
	}

	return nil
}
