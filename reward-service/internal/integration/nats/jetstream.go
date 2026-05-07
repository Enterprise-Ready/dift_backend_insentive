package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type JetStreamPublisher struct {
	js nats.JetStreamContext
}

func NewJetStreamPublisher(nc *nats.Conn) (*JetStreamPublisher, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("create jetstream context failed: %w", err)
	}

	return &JetStreamPublisher{
		js: js,
	}, nil
}

func (j *JetStreamPublisher) Publish(
	ctx context.Context,
	subject string,
	payload []byte,
) error {

	_, err := j.js.PublishMsg(&nats.Msg{
		Subject: subject,
		Data:    payload,
	}, nats.Context(ctx), nats.AckWait(5*time.Second))

	if err != nil {
		return fmt.Errorf("jetstream publish failed: %w", err)
	}

	return nil
}
