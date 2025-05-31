package userSettings

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

var _ Handler = (*HandlerImpl)(nil)

type Handler interface {
	GetUserSettings(w http.ResponseWriter, r *http.Request)
	UpdateUserSettings(w http.ResponseWriter, r *http.Request)
}
type HandlerImpl struct {
	SettingsService SettingsService
	logger          *slog.Logger
}

// NewHandlerImpl creates a new user HandlerImpl instance.
func NewHandlerImpl(userprofileService SettingsService, logger *slog.Logger) *HandlerImpl {
	instanceAddress := fmt.Sprintf("%p", logger)
	slog.Info("Creating NewUserprofileHandlerImpl", slog.String("logger_address", instanceAddress), slog.Bool("logger_is_nil", logger == nil))
	if logger == nil {
		panic("PANIC: Attempting to create UserprofileHandlerImpl with nil logger!")
	}

	return &HandlerImpl{
		SettingsService: userprofileService,
		logger:          logger,
	}
}

// GetUserSettings godoc
// @Summary      Get User Preferences
// @Description  Retrieves the authenticated user's preferences (profiles).
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200 {array} types.UserSettings "User Preferences"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/preferences [get]
func (h *HandlerImpl) GetUserSettings(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserprofileHandlerImpl").Start(r.Context(), "GetUserPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/preferences"),
	))
	defer span.End()

	l := h.logger.With(slog.String("HandlerImpl", "GetUserPreferences"))

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "User ID not found in context")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid user ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	preferences, err := h.SettingsService.GetUserSettings(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to get user preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get user preferences")
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to retrieve user preferences")
		return
	}

	span.SetStatus(codes.Ok, "User preferences retrieved successfully")
	api.WriteJSONResponse(w, r, http.StatusOK, preferences)
}

// UpdateUserSettings godoc
// @Summary      Update User Preferences
// @Description  Updates the authenticated user's preferences (profiles).
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Param        preferences body types.UpdateUserSettingsParams true "Preferences Update Parameters"
// @Success      200 {object} types.Response "Preferences Updated Successfully"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/preferences/{profileID} [put]
func (h *HandlerImpl) UpdateUserSettings(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserprofileHandlerImpl").Start(r.Context(), "UpdateUserPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/preferences"),
	))
	defer span.End()

	l := h.logger.With(slog.String("HandlerImpl", "UpdateUserPreferences"))
	l.Info("Updating user preferences")

	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.WarnContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "Authentication required")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil { // Should ideally not happen if JWT sub is always valid UUID
		l.ErrorContext(ctx, "Invalid user ID format in context", slog.String("user_id_str", userIDStr), slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid User ID in token")
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Invalid user session")
		return
	}

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.WarnContext(ctx, "Invalid profile ID format in URL path", slog.String("profile_id_str", profileIDStr), slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}
	l = l.With(slog.String("profileID", profileID.String()))
	span.SetAttributes(attribute.String("profile.id", profileID.String()))

	var params UpdateUserSettingsParams
	if err := api.DecodeJSONBody(w, r, &params); err != nil {
		l.WarnContext(ctx, "Failed to decode request body", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request body")
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid request format: %s", err.Error()))
		return
	}

	if err = h.SettingsService.UpdateUserSettings(ctx, userID, profileID, params); err != nil {
		l.ErrorContext(ctx, "Failed to update user preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update user preferences")
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to update user preferences")
		return
	}

	l.Info("User preferences updated successfully")
	span.SetStatus(codes.Ok, "User preferences updated successfully")
	api.WriteJSONResponse(w, r, http.StatusOK, types.Response{
		Success: true,
		Message: "Preferences updated successfully",
	})
}
