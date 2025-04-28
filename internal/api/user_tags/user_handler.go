package user

import (
	"fmt"
	"log/slog"
)

// UserTagsHandler handles HTTP requests related to user operations.
type UserTagsHandler struct {
	userTagsService UserTagsService
	logger          *slog.Logger
}

// NewUserTagsHandler creates a new user handler instance.
func NewUserTagsHandler(userService UserTagsService, logger *slog.Logger) *UserTagsHandler {
	instanceAddress := fmt.Sprintf("%p", logger)
	slog.Info("Creating NewUserHandler", slog.String("logger_address", instanceAddress), slog.Bool("logger_is_nil", logger == nil))
	if logger == nil {
		panic("PANIC: Attempting to create UserHandler with nil logger!")
	}

	return &UserTagsHandler{
		userTagsService: userService,
		logger:          logger,
	}
}
