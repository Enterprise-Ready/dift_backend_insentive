package admin

import (
	"context"

	adminport "reward-service/internal/interface/service/admin"
)

type CommandHandler struct {
	service adminport.CommandService
}

func NewCommandHandler(service adminport.CommandService) *CommandHandler {
	return &CommandHandler{service: service}
}

func (h *CommandHandler) Execute(ctx context.Context, cmd adminport.Command) error {
	return h.service.Execute(ctx, cmd)
}
