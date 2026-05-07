package admin

import "context"

type Command struct {
	Action         string         `json:"action"`
	Payload        map[string]any `json:"payload"`
	IdempotencyKey string         `json:"idempotency_key"`
}

type CommandService interface {
	Execute(ctx context.Context, cmd Command) error
}
