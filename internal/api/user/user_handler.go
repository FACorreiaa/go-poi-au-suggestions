package user

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

var _ Handler = (*HandlerImpl)(nil)

type Handler interface {
	GetUserProfile(w http.ResponseWriter, r *http.Request)
	UpdateUserProfile(w http.ResponseWriter, r *http.Request)
}

type HandlerImpl struct {
	userService UserService
	logger      *slog.Logger
}

// NewHandlerImpl creates a new user HandlerImpl instance.
func NewHandlerImpl(userService UserService, logger *slog.Logger) *HandlerImpl {
	instanceAddress := fmt.Sprintf("%p", logger)
	slog.Info("Creating NewHandlerImpl", slog.String("logger_address", instanceAddress), slog.Bool("logger_is_nil", logger == nil))
	if logger == nil {
		panic("PANIC: Attempting to create HandlerImpl with nil logger!")
	}

	return &HandlerImpl{
		userService: userService,
		logger:      logger,
	}
}

// GetUserProfile godoc
// @Summary      Get User Profile
// @Description  Retrieves the authenticated user's profile information.
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200 {object} types.UserProfile "User Profile"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "User Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profile [get]
func (h *HandlerImpl) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := h.logger.With(slog.String("HandlerImpl", "GetUserProfile"))

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	profile, err := h.userService.GetUserProfile(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to get user profile", slog.Any("error", err))
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "User not found")
		} else {
			api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to retrieve user profile")
		}
		return
	}

	api.WriteJSONResponse(w, r, http.StatusOK, profile)
}

// UpdateUserProfile godoc
// @Summary      Update User Profile
// @Description  Updates the authenticated user's profile information.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        profile body types.UpdateProfileParams true "Profile Update Parameters"
// @Success      200 {object} types.Response "Profile Updated Successfully"
// @Failure      400 {object} types.Response "Invalid Input"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profile [put]
func (h *HandlerImpl) UpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := h.logger.With(slog.String("HandlerImpl", "UpdateUserProfile"))

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	var params types.UpdateProfileParams
	if err := api.DecodeJSONBody(w, r, &params); err != nil {
		l.WarnContext(ctx, "Failed to decode request", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request format")
		return
	}

	err = h.userService.UpdateUserProfile(ctx, userID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user profile", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to update user profile")
		return
	}

	api.WriteJSONResponse(w, r, http.StatusOK, types.Response{
		Success: true,
		Message: "Profile updated successfully",
	})
}
