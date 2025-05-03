package userProfiles

import (
	"fmt"
	"log/slog"
	"net/http"
)

// UserHandler handles HTTP requests related to user operations.
type UserProfilesHandler struct {
	userService UserProfilesService
	logger      *slog.Logger
}

// NewUserHandler creates a new user handler instance.
func NewUserHandler(userService UserProfilesService, logger *slog.Logger) *UserProfilesHandler {
	instanceAddress := fmt.Sprintf("%p", logger)
	slog.Info("Creating NewUserHandler", slog.String("logger_address", instanceAddress), slog.Bool("logger_is_nil", logger == nil))
	if logger == nil {
		panic("PANIC: Attempting to create UserHandler with nil logger!")
	}

	return &UserProfilesHandler{
		userService: userService,
		logger:      logger,
	}
}

func (u *UserProfilesHandler) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := u.logger.With(slog.String("handler", "GetUserProfile"))
	l.DebugContext(ctx, "Fetching user profile")

	return
}
