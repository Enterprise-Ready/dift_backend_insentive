package nats

import (
	"github.com/nats-io/nats.go"
)

func EnsureStream(
	js nats.JetStreamContext,
	streamName string,
	subjects []string,
) error {

	_, err := js.AddStream(&nats.StreamConfig{
		Name:      streamName,
		Subjects:  subjects,
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
	})

	if err == nats.ErrStreamNameAlreadyInUse {
		return nil
	}

	return err
}
