package profiles

import (
	"errors"
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
	GetSearchProfile(w http.ResponseWriter, r *http.Request)
	GetSearchProfiles(w http.ResponseWriter, r *http.Request)
	CreateSearchProfile(w http.ResponseWriter, r *http.Request)
	GetDefaultSearchProfile(w http.ResponseWriter, r *http.Request)
	UpdateSearchProfile(w http.ResponseWriter, r *http.Request)
	DeleteSearchProfile(w http.ResponseWriter, r *http.Request)
	SetDefaultSearchProfile(w http.ResponseWriter, r *http.Request)
	
}
type HandlerImpl struct {
	userService Service
	logger      *slog.Logger
}

// NewUserHandlerImpl creates a new user HandlerImpl instance.
func NewUserHandlerImpl(userService Service, logger *slog.Logger) *HandlerImpl {
	instanceAddress := fmt.Sprintf("%p", logger)
	slog.Info("Creating NewUserHandlerImpl", slog.String("logger_address", instanceAddress), slog.Bool("logger_is_nil", logger == nil))
	if logger == nil {
		panic("PANIC: Attempting to create UserHandlerImpl with nil logger!")
	}

	return &HandlerImpl{
		userService: userService,
		logger:      logger,
	}
}

// GetSearchProfile godoc
// @Summary      Get User Preference Profile
// @Description  Fetches a specific preference profile for the authenticated user
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Success      200 {object} types.UserPreferenceProfileResponse "User Preference Profile"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profiles/{profileID} [get]
func (u *HandlerImpl) GetSearchProfile(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "GetAllInterests", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/interests"),
	))
	defer span.End()

	l := u.logger.With(slog.String("HandlerImpl", "GetUserProfile"))
	l.DebugContext(ctx, "Fetching user profile")

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
	}

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}

	profile, err := u.userService.GetSearchProfile(ctx, userID, profileID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user profile", slog.Any("error", err))
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "User profile not found")
		}
	}

	l.InfoContext(ctx, "User profile fetched successfully", slog.String("profileID", profile.ID.String()))
	api.WriteJSONResponse(w, r, http.StatusOK, profile)
	w.WriteHeader(http.StatusOK)
}

// GetSearchProfiles godoc
// @Summary      Get User Preference Profiles
// @Description  Fetches all preference profiles for the authenticated user
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Success      200 {array} types.UserPreferenceProfileResponse "User Preference Profiles"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profiles [get]
// Assuming this is in a HandlerImpl file
func (u *HandlerImpl) GetSearchProfiles(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "GetSearchProfiles", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/interests"),
	))
	defer span.End()

	l := u.logger.With(slog.String("HandlerImpl", "GetUserProfile"))
	l.DebugContext(ctx, "Fetching user profile")

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
	}

	profile, err := u.userService.GetSearchProfiles(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user profile", slog.Any("error", err))
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "User profile not found")
		}
	}

	l.InfoContext(ctx, "User profile fetched successfully")
	api.WriteJSONResponse(w, r, http.StatusOK, profile)
	w.WriteHeader(http.StatusOK)
}

// CreateProfile godoc
// @Summary      Create User Preference Profile
// @Description  Creates a new preference profile for the authenticated user
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profile body types.CreateUserPreferenceProfileParams true "Profile Creation Parameters"
// @Success      201 {object} types.UserPreferenceProfileResponse "Created Profile"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profiles [post]
// Assuming this is in a HandlerImpl file
func (u *HandlerImpl) CreateSearchProfile(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserProfilesHandlerImpl").Start(r.Context(), "CreateProfile", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile"),
	))
	defer span.End()

	l := u.logger.With(slog.String("HandlerImpl", "CreateProfile"))
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

	var params types.CreateUserPreferenceProfileParams
	if err := api.DecodeJSONBody(w, r, &params); err != nil {
		l.WarnContext(ctx, "Failed to decode request body", slog.Any("error", err), slog.Any("params", params))
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

	_, err = u.userService.CreateSearchProfile(ctx, userID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to create user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create user preference profile")
		if errors.Is(err, types.ErrConflict) {
			api.ErrorResponse(w, r, http.StatusConflict, "Profile name already exists")
			return
		}
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid tag or interest ID")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to create user preference profile")
		return
	}

	l.InfoContext(ctx, "User preference profile created successfully")
	span.SetStatus(codes.Ok, "User preference profile created successfully")
	api.WriteJSONResponse(w, r, http.StatusCreated, nil)
}

// GetDefaultSearchProfile godoc
// @Summary      Get Default User Preference Profile
// @Description  Fetches the default preference profile for the authenticated user
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Success      200 {object} types.UserPreferenceProfileResponse "Default User Preference Profile"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profiles/default [get]
// Assuming this is in a HandlerImpl file
func (u *HandlerImpl) GetDefaultSearchProfile(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserProfilesHandlerImpl").Start(r.Context(), "CreateProfile", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile"),
	))
	defer span.End()

	l := u.logger.With(slog.String("HandlerImpl", "CreateProfile"))
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

	profile, err := u.userService.GetDefaultSearchProfile(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to create user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create user preference profile")
		if errors.Is(err, types.ErrConflict) {
			api.ErrorResponse(w, r, http.StatusConflict, "Profile name already exists")
			return
		}
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid tag or interest ID")
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

// UpdateSearchProfile godoc
// @Summary      Update User Preference Profile
// @Description  Updates an existing preference profile for the authenticated user
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Param        profile body types.UpdateUserPreferenceProfileParams true "Profile Update Parameters"
// @Success      200 {object} types.UserPreferenceProfileResponse "Updated Profile"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profiles/{profileID} [put]
// Assuming this is in a HandlerImpl file
func (u *HandlerImpl) UpdateSearchProfile(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserProfilesHandlerImpl").Start(r.Context(), "CreateProfile", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile"),
	))
	defer span.End()

	l := u.logger.With(slog.String("HandlerImpl", "CreateProfile"))
	l.DebugContext(ctx, "Creating user preference profile")

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "User ID not found in context")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
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

	var params types.UpdateSearchProfileParams
	if err := api.DecodeJSONBody(w, r, &params); err != nil {
		l.WarnContext(ctx, "Failed to decode request body", slog.Any("error", err), slog.Any("params", params))
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

	if err := u.userService.UpdateSearchProfile(ctx, userID, profileID, params); err != nil {
		l.ErrorContext(ctx, "Failed to create user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create user preference profile")
		if errors.Is(err, types.ErrConflict) {
			api.ErrorResponse(w, r, http.StatusConflict, "Profile name already exists")
			return
		}
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid tag or interest ID")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to create user preference profile")
		return
	}

	l.InfoContext(ctx, "User preference profile created successfully")
	span.SetStatus(codes.Ok, "User preference profile created successfully")
	api.WriteJSONResponse(w, r, http.StatusCreated, nil)
}

// DeleteSearchProfile godoc
// @Summary      Delete User Preference Profile
// @Description  Deletes an existing preference profile for the authenticated user
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Success      204 {object} nil "No Content"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profiles/{profileID} [delete]
// Assuming this is in a HandlerImpl file
func (u *HandlerImpl) DeleteSearchProfile(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserProfilesHandlerImpl").Start(r.Context(), "CreateProfile", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile"),
	))
	defer span.End()

	l := u.logger.With(slog.String("HandlerImpl", "CreateProfile"))
	l.DebugContext(ctx, "Creating user preference profile")

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "User ID not found in context")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
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

	if err := u.userService.DeleteSearchProfile(ctx, userID, profileID); err != nil {
		l.ErrorContext(ctx, "Failed to delete user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to delete user preference profile")
		if errors.Is(err, types.ErrConflict) {
			api.ErrorResponse(w, r, http.StatusConflict, "Profile name already exists")
			return
		}
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid tag or interest ID")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to delete user preference profile")
		return
	}

	l.InfoContext(ctx, "User preference profile delete successfully")
	span.SetStatus(codes.Ok, "User preference profile delete successfully")
	api.WriteJSONResponse(w, r, http.StatusCreated, nil)
}

// SetDefaultSearchProfile godoc
// @Summary      Set Default User Preference Profile
// @Description  Sets an existing preference profile as the default for the authenticated user
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Success      200 {object} types.UserPreferenceProfileResponse "Default User Preference Profile"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/profiles/default/{profileID} [put]
// Assuming this is in a HandlerImpl file
func (u *HandlerImpl) SetDefaultSearchProfile(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserProfilesHandlerImpl").Start(r.Context(), "SetDefaultSearchProfile", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile"),
	))
	defer span.End()

	l := u.logger.With(slog.String("HandlerImpl", "CreateProfile"))
	l.DebugContext(ctx, "Creating user preference profile")

	// Get UserID from context (set by Authenticate middleware)
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "User ID not found in context")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
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

	if err := u.userService.SetDefaultSearchProfile(ctx, userID, profileID); err != nil {
		l.ErrorContext(ctx, "Failed to delete user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to delete user preference profile")
		if errors.Is(err, types.ErrConflict) {
			api.ErrorResponse(w, r, http.StatusConflict, "Profile name already exists")
			return
		}
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid tag or interest ID")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to delete user preference profile")
		return
	}

	l.InfoContext(ctx, "User preference profile delete successfully")
	span.SetStatus(codes.Ok, "User preference profile delete successfully")
	api.WriteJSONResponse(w, r, http.StatusCreated, nil)
}
