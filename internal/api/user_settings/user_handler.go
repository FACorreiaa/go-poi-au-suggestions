package userSettings

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
)

type UserSettingsHandler struct {
	userSettingsService UserSettingsService
	logger              *slog.Logger
}

// NewUserSettingsHandler creates a new user handler instance.
func NewUserSettingsHandler(userInterestService UserSettingsService, logger *slog.Logger) *UserSettingsHandler {
	instanceAddress := fmt.Sprintf("%p", logger)
	slog.Info("Creating NewUserInterestHandler", slog.String("logger_address", instanceAddress), slog.Bool("logger_is_nil", logger == nil))
	if logger == nil {
		panic("PANIC: Attempting to create UserInterestHandler with nil logger!")
	}

	return &UserSettingsHandler{
		userSettingsService: userInterestService,
		logger:              logger,
	}
}

// GetUserPreferences godoc
// @Summary      Get User Preferences
// @Description  Retrieves the authenticated user's preferences (interests).
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200 {array} api.Interest "User Preferences"
// @Failure      401 {object} api.Response "Unauthorized"
// @Failure      500 {object} api.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/preferences [get]
func (h *UserSettingsHandler) GetUserPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserInterestHandler").Start(r.Context(), "GetUserPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/preferences"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "GetUserPreferences"))

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

	preferences, err := h.userSettingsService.GetUserPreferences(ctx, userID)
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
