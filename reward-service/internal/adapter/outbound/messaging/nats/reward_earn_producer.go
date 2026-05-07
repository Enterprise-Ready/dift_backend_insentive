package nats

import (
	"context"
	"fmt"

	"reward-service/internal/dto"
	port "reward-service/internal/interface/earn_reward"
	"reward-service/internal/model"

	"google.golang.org/protobuf/proto"
)

type PublishFunc func(ctx context.Context, subject string, payload []byte) error

type RewardEarnProducer struct {
	publish PublishFunc
	subject string
}

func NewRewardEarnProducer(
	publish PublishFunc,
	subject string,
) port.RewardEarnProducerPort {
	return &RewardEarnProducer{
		publish: publish,
		subject: subject,
	}
}

func (r *RewardEarnProducer) SendEarn(
	ctx context.Context,
	earn model.Earn,
) error {

	pbMsg := dto.EarnToRewardEarnPB(earn)

	payload, err := proto.Marshal(pbMsg)
	if err != nil {
		return fmt.Errorf("marshal reward earn failed: %w", err)
	}

	if err := r.publish(ctx, r.subject, payload); err != nil {
		return fmt.Errorf("publish reward earn failed: %w", err)
	}

	return nil
}
