package user

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
)

// UserHandler handles HTTP requests related to user operations.
type UserHandler struct {
	userService UserService
	logger      *slog.Logger
}

// NewUserHandler creates a new user handler instance.
func NewUserHandler(userService UserService, logger *slog.Logger) *UserHandler {
	instanceAddress := fmt.Sprintf("%p", logger)
	slog.Info("Creating NewUserHandler", slog.String("logger_address", instanceAddress), slog.Bool("logger_is_nil", logger == nil))
	if logger == nil {
		panic("PANIC: Attempting to create UserHandler with nil logger!")
	}
	
	return &UserHandler{
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
// @Success      200 {object} api.UserProfile "User Profile"
// @Failure      401 {object} api.Response "Unauthorized"
// @Failure      404 {object} api.Response "User Not Found"
// @Failure      500 {object} api.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profile [get]
func (h *UserHandler) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := h.logger.With(slog.String("handler", "GetUserProfile"))

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		auth.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	profile, err := h.userService.GetUserProfile(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to get user profile", slog.Any("error", err))
		if errors.Is(err, api.ErrNotFound) {
			auth.ErrorResponse(w, r, http.StatusNotFound, "User not found")
		} else {
			auth.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to retrieve user profile")
		}
		return
	}

	auth.WriteJSONResponse(w, r, http.StatusOK, profile)
}

// UpdateUserProfile godoc
// @Summary      Update User Profile
// @Description  Updates the authenticated user's profile information.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        profile body api.UpdateProfileParams true "Profile Update Parameters"
// @Success      200 {object} api.Response "Profile Updated Successfully"
// @Failure      400 {object} api.Response "Invalid Input"
// @Failure      401 {object} api.Response "Unauthorized"
// @Failure      500 {object} api.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profile [put]
func (h *UserHandler) UpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := h.logger.With(slog.String("handler", "UpdateUserProfile"))

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		auth.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	var params api.UpdateProfileParams
	if err := auth.DecodeJSONBody(w, r, &params); err != nil {
		l.WarnContext(ctx, "Failed to decode request", slog.Any("error", err))
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request format")
		return
	}

	err = h.userService.UpdateUserProfile(ctx, userID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user profile", slog.Any("error", err))
		auth.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to update user profile")
		return
	}

	auth.WriteJSONResponse(w, r, http.StatusOK, api.Response{
		Success: true,
		Message: "Profile updated successfully",
	})
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
func (h *UserHandler) GetUserPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserHandler").Start(r.Context(), "GetUserPreferences", trace.WithAttributes(
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
		auth.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid user ID format")
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	preferences, err := h.userService.GetUserPreferences(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to get user preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get user preferences")
		auth.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to retrieve user preferences")
		return
	}

	span.SetStatus(codes.Ok, "User preferences retrieved successfully")
	auth.WriteJSONResponse(w, r, http.StatusOK, preferences)
}

// GetAllInterests godoc
// @Summary      Get All Interests
// @Description  Retrieves all available interests.
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200 {array} api.Interest "All Interests"
// @Failure      500 {object} api.Response "Internal Server Error"
// @Router       /user/interests [get]
func (h *UserHandler) GetAllInterests(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserHandler").Start(r.Context(), "GetAllInterests", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/interests"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "GetAllInterests"))

	interests, err := h.userService.GetAllInterests(ctx)
	if err != nil {
		l.ErrorContext(ctx, "Failed to get all interests", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get all interests")
		auth.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to retrieve interests")
		return
	}

	span.SetStatus(codes.Ok, "All interests retrieved successfully")
	auth.WriteJSONResponse(w, r, http.StatusOK, interests)
}

// AddUserInterest godoc
// @Summary      Add User Interest
// @Description  Adds an interest to the authenticated user's preferences.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        interest body AddInterestRequest true "Interest ID to add"
// @Success      200 {object} api.Response "Interest Added Successfully"
// @Failure      400 {object} api.Response "Invalid Input"
// @Failure      401 {object} api.Response "Unauthorized"
// @Failure      404 {object} api.Response "Interest Not Found"
// @Failure      500 {object} api.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/preferences/interests [post]
func (h *UserHandler) AddUserInterest(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserHandler").Start(r.Context(), "AddUserInterest", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/preferences/interests"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "AddUserInterest"))

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "User ID not found in context")
		auth.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid user ID format")
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	var req AddInterestRequest
	if err := auth.DecodeJSONBody(w, r, &req); err != nil {
		l.WarnContext(ctx, "Failed to decode request", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to decode request")
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request format")
		return
	}

	interestID, err := uuid.Parse(req.InterestID)
	if err != nil {
		l.WarnContext(ctx, "Invalid interest ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid interest ID format")
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid interest ID format")
		return
	}

	err = h.userService.AddUserInterest(ctx, userID, interestID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to add user interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to add user interest")
		if errors.Is(err, api.ErrNotFound) {
			auth.ErrorResponse(w, r, http.StatusNotFound, "Interest not found")
		} else {
			auth.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to add interest")
		}
		return
	}

	span.SetStatus(codes.Ok, "User interest added successfully")
	auth.WriteJSONResponse(w, r, http.StatusOK, api.Response{
		Success: true,
		Message: "Interest added successfully",
	})
}

// RemoveUserInterest godoc
// @Summary      Remove User Interest
// @Description  Removes an interest from the authenticated user's preferences.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        interestId path string true "Interest ID to remove"
// @Success      200 {object} api.Response "Interest Removed Successfully"
// @Failure      400 {object} api.Response "Invalid Input"
// @Failure      401 {object} api.Response "Unauthorized"
// @Failure      404 {object} api.Response "Interest Not Found"
// @Failure      500 {object} api.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/preferences/interests/{interestId} [delete]
func (h *UserHandler) RemoveUserInterest(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserHandler").Start(r.Context(), "RemoveUserInterest", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/preferences/interests/{interestId}"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "RemoveUserInterest"))

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "User ID not found in context")
		auth.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid user ID format")
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	interestIDStr := chi.URLParam(r, "interestId")
	interestID, err := uuid.Parse(interestIDStr)
	if err != nil {
		l.WarnContext(ctx, "Invalid interest ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid interest ID format")
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid interest ID format")
		return
	}

	err = h.userService.RemoveUserInterest(ctx, userID, interestID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to remove user interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to remove user interest")
		if errors.Is(err, api.ErrNotFound) {
			auth.ErrorResponse(w, r, http.StatusNotFound, "Interest association not found")
		} else {
			auth.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to remove interest")
		}
		return
	}

	span.SetStatus(codes.Ok, "User interest removed successfully")
	auth.WriteJSONResponse(w, r, http.StatusOK, api.Response{
		Success: true,
		Message: "Interest removed successfully",
	})
}

// UpdateUserInterestPreferenceLevel godoc
// @Summary      Update User Interest Preference Level
// @Description  Updates the preference level for a user interest.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        interestId path string true "Interest ID"
// @Param        preference body UpdatePreferenceLevelRequest true "Preference Level"
// @Success      200 {object} api.Response "Preference Level Updated Successfully"
// @Failure      400 {object} api.Response "Invalid Input"
// @Failure      401 {object} api.Response "Unauthorized"
// @Failure      404 {object} api.Response "Interest Not Found"
// @Failure      500 {object} api.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/preferences/interests/{interestId}/level [put]
func (h *UserHandler) UpdateUserInterestPreferenceLevel(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserHandler").Start(r.Context(), "UpdateUserInterestPreferenceLevel", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/preferences/interests/{interestId}/level"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "UpdateUserInterestPreferenceLevel"))

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "User ID not found in context")
		auth.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid user ID format")
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	interestIDStr := chi.URLParam(r, "interestId")
	interestID, err := uuid.Parse(interestIDStr)
	if err != nil {
		l.WarnContext(ctx, "Invalid interest ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid interest ID format")
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid interest ID format")
		return
	}

	var req UpdatePreferenceLevelRequest
	if err := auth.DecodeJSONBody(w, r, &req); err != nil {
		l.WarnContext(ctx, "Failed to decode request", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to decode request")
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request format")
		return
	}

	if req.PreferenceLevel < 0 {
		l.WarnContext(ctx, "Invalid preference level", slog.Int("level", req.PreferenceLevel))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid preference level")
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Preference level must be non-negative")
		return
	}

	err = h.userService.UpdateUserInterestPreferenceLevel(ctx, userID, interestID, req.PreferenceLevel)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user interest preference level", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update user interest preference level")
		if errors.Is(err, api.ErrNotFound) {
			auth.ErrorResponse(w, r, http.StatusNotFound, "Interest association not found")
		} else {
			auth.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to update preference level")
		}
		return
	}

	span.SetStatus(codes.Ok, "User interest preference level updated successfully")
	auth.WriteJSONResponse(w, r, http.StatusOK, api.Response{
		Success: true,
		Message: "Preference level updated successfully",
	})
}

// GetUserEnhancedInterests godoc
// @Summary      Get User Enhanced Interests
// @Description  Retrieves the authenticated user's enhanced interests with preference levels.
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200 {array} api.EnhancedInterest "User Enhanced Interests"
// @Failure      401 {object} api.Response "Unauthorized"
// @Failure      500 {object} api.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/preferences/enhanced [get]
func (h *UserHandler) GetUserEnhancedInterests(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserHandler").Start(r.Context(), "GetUserEnhancedInterests", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/preferences/enhanced"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "GetUserEnhancedInterests"))

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "User ID not found in context")
		auth.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid user ID format")
		auth.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	interests, err := h.userService.GetUserEnhancedInterests(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to get user enhanced interests", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get user enhanced interests")
		auth.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to retrieve enhanced interests")
		return
	}

	span.SetStatus(codes.Ok, "User enhanced interests retrieved successfully")
	auth.WriteJSONResponse(w, r, http.StatusOK, interests)
}

// Request and Response structures for the handlers
type AddInterestRequest struct {
	InterestID string `json:"interest_id" binding:"required" example:"d290f1ee-6c54-4b01-90e6-d701748f0851"`
}

type UpdatePreferenceLevelRequest struct {
	PreferenceLevel int `json:"preference_level" binding:"required" example:"2"`
}
