package userProfiles

import (
	"fmt"
	"log/slog"
	"net/http"

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

// CreateProfile godoc
// @Summary      Create User Preference Profile
// @Description  Creates a new preference profile for the authenticated user
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profile body api.CreateUserPreferenceProfileParams true "Profile Creation Parameters"
// @Success      201 {object} api.UserPreferenceProfile "Created Profile"
// @Failure      400 {object} api.Response "Bad Request"
// @Failure      401 {object} api.Response "Unauthorized"
// @Failure      500 {object} api.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profiles [post]
func (u *UserProfilesHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserProfilesHandler").Start(r.Context(), "CreateProfile", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/profiles"),
	))
	defer span.End()

	l := u.logger.With(slog.String("handler", "CreateProfile"))
	l.DebugContext(ctx, "Creating user preference profile")

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

	var params api.CreateUserPreferenceProfileParams
	if err := api.DecodeJSONBody(w, r, &params); err != nil {
		l.WarnContext(ctx, "Failed to decode request body", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request body")
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid request format: %s", err.Error()))
		return
	}

	// Validate required fields
	if params.ProfileName == "" {
		l.WarnContext(ctx, "Profile name is required")
		span.SetStatus(codes.Error, "Profile name is required")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Profile name is required")
		return
	}

	profile, err := u.userService.CreateUserPreferenceProfile(ctx, userID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to create user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create user preference profile")

		// Check for specific error types
		if err == types.ErrConflict {
			api.ErrorResponse(w, r, http.StatusConflict, "Profile name already exists")
			return
		}

		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to create user preference profile")
		return
	}

	l.InfoContext(ctx, "User preference profile created successfully", slog.String("profileID", profile.ID.String()))
	span.SetStatus(codes.Ok, "User preference profile created successfully")
	span.SetAttributes(attribute.String("profile.id", profile.ID.String()))
	api.WriteJSONResponse(w, r, http.StatusCreated, profile)
}
