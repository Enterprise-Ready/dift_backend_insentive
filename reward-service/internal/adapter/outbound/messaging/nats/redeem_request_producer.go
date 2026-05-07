package nats

import (
	"context"
	"fmt"

	"reward-service/internal/dto"
	reward "reward-service/internal/interface/redeem"
	"reward-service/internal/model"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type RedeemRequestProducer struct {
	nc      *nats.Conn
	subject string
}

func NewRedeemRequestProducer(
	nc *nats.Conn,
	subject string,
) reward.RewardRedeemPort {

	return &RedeemRequestProducer{
		nc:      nc,
		subject: subject,
	}
}

func (p *RedeemRequestProducer) SendRedeemRequest(
	ctx context.Context,
	redeem model.Redeem,
) error {

	pbMsg := dto.RedeemToRequestPB(redeem)

	payload, err := proto.Marshal(pbMsg)
	if err != nil {
		return fmt.Errorf("marshal redeem request failed: %w", err)
	}

	if err := p.nc.Publish(p.subject, payload); err != nil {
		return fmt.Errorf("nats publish redeem request failed: %w", err)
	}

	return nil
}
