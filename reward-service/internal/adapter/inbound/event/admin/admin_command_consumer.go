package natsadmin

import (
	"context"
	"encoding/json"

	adminport "reward-service/internal/interface/service/admin"
	adminhandler "reward-service/internal/service_logic/handler/admin"
)

type CommandConsumer struct {
	handler *adminhandler.CommandHandler
}

func NewCommandConsumer(handler *adminhandler.CommandHandler) *CommandConsumer {
	return &CommandConsumer{handler: handler}
}

func (c *CommandConsumer) Handle(ctx context.Context, payload []byte) error {
	var cmd adminport.Command
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return err
	}
	return c.handler.Execute(ctx, cmd)
}
