package profiles

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
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

// GetCombinedFilters godoc
// @Summary      Get Combined User Filters
// @Description  Fetches combined filters for a specific profile and domain
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Param        domain query string false "Domain type (accommodation, dining, activities, itinerary, general)" Enums(accommodation, dining, activities, itinerary, general)
// @Success      200 {object} types.CombinedFilters "Combined Filters"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/search-profile/{profileID}/filters [get]
func (u *HandlerImpl) GetCombinedFilters(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("ProfileHandler").Start(r.Context(), "GetCombinedFilters", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile/{profileID}/filters"),
	))
	defer span.End()

	l := u.logger.With(slog.String("handler", "GetCombinedFilters"))
	l.DebugContext(ctx, "Fetching combined filters")

	// Get UserID from context
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

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}

	// Get domain from query params (default to general)
	domainStr := r.URL.Query().Get("domain")
	if domainStr == "" {
		domainStr = "general"
	}
	
	domain := types.DomainType(domainStr)

	combinedFilters, err := u.userService.GetCombinedFilters(ctx, userID, profileID, domain)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch combined filters", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch combined filters")
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "Profile not found")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to fetch combined filters")
		return
	}

	l.InfoContext(ctx, "Combined filters fetched successfully", slog.String("profileID", profileID.String()), slog.String("domain", string(domain)))
	api.WriteJSONResponse(w, r, http.StatusOK, combinedFilters)
}

// GetAccommodationPreferences godoc
// @Summary      Get Accommodation Preferences
// @Description  Fetches accommodation preferences for a specific profile
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Success      200 {object} types.AccommodationPreferences "Accommodation Preferences"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/search-profile/{profileID}/accommodation [get]
func (u *HandlerImpl) GetAccommodationPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("ProfileHandler").Start(r.Context(), "GetAccommodationPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile/{profileID}/accommodation"),
	))
	defer span.End()

	l := u.logger.With(slog.String("handler", "GetAccommodationPreferences"))
	l.DebugContext(ctx, "Fetching accommodation preferences")

	// Get UserID from context
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

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}

	// Get combined filters and extract accommodation preferences
	combinedFilters, err := u.userService.GetCombinedFilters(ctx, userID, profileID, types.DomainAccommodation)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch accommodation preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch accommodation preferences")
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "Profile not found")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to fetch accommodation preferences")
		return
	}

	if combinedFilters.AccommodationPreferences == nil {
		api.ErrorResponse(w, r, http.StatusNotFound, "Accommodation preferences not found")
		return
	}

	l.InfoContext(ctx, "Accommodation preferences fetched successfully", slog.String("profileID", profileID.String()))
	api.WriteJSONResponse(w, r, http.StatusOK, combinedFilters.AccommodationPreferences)
}

// UpdateAccommodationPreferences godoc
// @Summary      Update Accommodation Preferences
// @Description  Updates accommodation preferences for a specific profile
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Param        preferences body types.AccommodationPreferences true "Accommodation Preferences"
// @Success      200 {object} types.Response "Success"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/search-profile/{profileID}/accommodation [put]
func (u *HandlerImpl) UpdateAccommodationPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("ProfileHandler").Start(r.Context(), "UpdateAccommodationPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile/{profileID}/accommodation"),
	))
	defer span.End()

	l := u.logger.With(slog.String("handler", "UpdateAccommodationPreferences"))
	l.DebugContext(ctx, "Updating accommodation preferences")

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}

	var prefs types.AccommodationPreferences
	if err := api.DecodeJSONBody(w, r, &prefs); err != nil {
		l.WarnContext(ctx, "Failed to decode request body", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request body")
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid request format: %s", err.Error()))
		return
	}

	err = u.userService.UpdateAccommodationPreferences(ctx, profileID, &prefs)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update accommodation preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update accommodation preferences")
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "Profile not found")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to update accommodation preferences")
		return
	}

	l.InfoContext(ctx, "Accommodation preferences updated successfully", slog.String("profileID", profileID.String()))
	api.WriteJSONResponse(w, r, http.StatusOK, map[string]string{"message": "Accommodation preferences updated successfully"})
}

// GetDiningPreferences godoc
// @Summary      Get Dining Preferences
// @Description  Fetches dining preferences for a specific profile
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Success      200 {object} types.DiningPreferences "Dining Preferences"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/search-profile/{profileID}/dining [get]
func (u *HandlerImpl) GetDiningPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("ProfileHandler").Start(r.Context(), "GetDiningPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile/{profileID}/dining"),
	))
	defer span.End()

	l := u.logger.With(slog.String("handler", "GetDiningPreferences"))
	l.DebugContext(ctx, "Fetching dining preferences")

	// Get UserID from context
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

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}

	// Get combined filters and extract dining preferences
	combinedFilters, err := u.userService.GetCombinedFilters(ctx, userID, profileID, types.DomainDining)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch dining preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch dining preferences")
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "Profile not found")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to fetch dining preferences")
		return
	}

	if combinedFilters.DiningPreferences == nil {
		api.ErrorResponse(w, r, http.StatusNotFound, "Dining preferences not found")
		return
	}

	l.InfoContext(ctx, "Dining preferences fetched successfully", slog.String("profileID", profileID.String()))
	api.WriteJSONResponse(w, r, http.StatusOK, combinedFilters.DiningPreferences)
}

// UpdateDiningPreferences godoc
// @Summary      Update Dining Preferences
// @Description  Updates dining preferences for a specific profile
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Param        preferences body types.DiningPreferences true "Dining Preferences"
// @Success      200 {object} types.Response "Success"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/search-profile/{profileID}/dining [put]
func (u *HandlerImpl) UpdateDiningPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("ProfileHandler").Start(r.Context(), "UpdateDiningPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile/{profileID}/dining"),
	))
	defer span.End()

	l := u.logger.With(slog.String("handler", "UpdateDiningPreferences"))
	l.DebugContext(ctx, "Updating dining preferences")

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}

	var prefs types.DiningPreferences
	if err := api.DecodeJSONBody(w, r, &prefs); err != nil {
		l.WarnContext(ctx, "Failed to decode request body", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request body")
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid request format: %s", err.Error()))
		return
	}

	err = u.userService.UpdateDiningPreferences(ctx, profileID, &prefs)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update dining preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update dining preferences")
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "Profile not found")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to update dining preferences")
		return
	}

	l.InfoContext(ctx, "Dining preferences updated successfully", slog.String("profileID", profileID.String()))
	api.WriteJSONResponse(w, r, http.StatusOK, map[string]string{"message": "Dining preferences updated successfully"})
}

// GetActivityPreferences godoc
// @Summary      Get Activity Preferences
// @Description  Fetches activity preferences for a specific profile
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Success      200 {object} types.ActivityPreferences "Activity Preferences"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/search-profile/{profileID}/activities [get]
func (u *HandlerImpl) GetActivityPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("ProfileHandler").Start(r.Context(), "GetActivityPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile/{profileID}/activities"),
	))
	defer span.End()

	l := u.logger.With(slog.String("handler", "GetActivityPreferences"))
	l.DebugContext(ctx, "Fetching activity preferences")

	// Get UserID from context
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

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}

	// Get combined filters and extract activity preferences
	combinedFilters, err := u.userService.GetCombinedFilters(ctx, userID, profileID, types.DomainActivities)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch activity preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch activity preferences")
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "Profile not found")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to fetch activity preferences")
		return
	}

	if combinedFilters.ActivityPreferences == nil {
		api.ErrorResponse(w, r, http.StatusNotFound, "Activity preferences not found")
		return
	}

	l.InfoContext(ctx, "Activity preferences fetched successfully", slog.String("profileID", profileID.String()))
	api.WriteJSONResponse(w, r, http.StatusOK, combinedFilters.ActivityPreferences)
}

// UpdateActivityPreferences godoc
// @Summary      Update Activity Preferences
// @Description  Updates activity preferences for a specific profile
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Param        preferences body types.ActivityPreferences true "Activity Preferences"
// @Success      200 {object} types.Response "Success"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/search-profile/{profileID}/activities [put]
func (u *HandlerImpl) UpdateActivityPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("ProfileHandler").Start(r.Context(), "UpdateActivityPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile/{profileID}/activities"),
	))
	defer span.End()

	l := u.logger.With(slog.String("handler", "UpdateActivityPreferences"))
	l.DebugContext(ctx, "Updating activity preferences")

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}

	var prefs types.ActivityPreferences
	if err := api.DecodeJSONBody(w, r, &prefs); err != nil {
		l.WarnContext(ctx, "Failed to decode request body", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request body")
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid request format: %s", err.Error()))
		return
	}

	err = u.userService.UpdateActivityPreferences(ctx, profileID, &prefs)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update activity preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update activity preferences")
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "Profile not found")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to update activity preferences")
		return
	}

	l.InfoContext(ctx, "Activity preferences updated successfully", slog.String("profileID", profileID.String()))
	api.WriteJSONResponse(w, r, http.StatusOK, map[string]string{"message": "Activity preferences updated successfully"})
}

// GetItineraryPreferences godoc
// @Summary      Get Itinerary Preferences
// @Description  Fetches itinerary preferences for a specific profile
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Success      200 {object} types.ItineraryPreferences "Itinerary Preferences"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/search-profile/{profileID}/itinerary [get]
func (u *HandlerImpl) GetItineraryPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("ProfileHandler").Start(r.Context(), "GetItineraryPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile/{profileID}/itinerary"),
	))
	defer span.End()

	l := u.logger.With(slog.String("handler", "GetItineraryPreferences"))
	l.DebugContext(ctx, "Fetching itinerary preferences")

	// Get UserID from context
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

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}

	// Get combined filters and extract itinerary preferences
	combinedFilters, err := u.userService.GetCombinedFilters(ctx, userID, profileID, types.DomainItinerary)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch itinerary preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch itinerary preferences")
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "Profile not found")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to fetch itinerary preferences")
		return
	}

	if combinedFilters.ItineraryPreferences == nil {
		api.ErrorResponse(w, r, http.StatusNotFound, "Itinerary preferences not found")
		return
	}

	l.InfoContext(ctx, "Itinerary preferences fetched successfully", slog.String("profileID", profileID.String()))
	api.WriteJSONResponse(w, r, http.StatusOK, combinedFilters.ItineraryPreferences)
}

// UpdateItineraryPreferences godoc
// @Summary      Update Itinerary Preferences
// @Description  Updates itinerary preferences for a specific profile
// @Tags         User Profiles
// @Accept       json
// @Produce      json
// @Param        profileID path string true "Profile ID"
// @Param        preferences body types.ItineraryPreferences true "Itinerary Preferences"
// @Success      200 {object} types.Response "Success"
// @Failure      400 {object} types.Response "Bad Request"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Not Found"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /user/search-profile/{profileID}/itinerary [put]
func (u *HandlerImpl) UpdateItineraryPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("ProfileHandler").Start(r.Context(), "UpdateItineraryPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/search-profile/{profileID}/itinerary"),
	))
	defer span.End()

	l := u.logger.With(slog.String("handler", "UpdateItineraryPreferences"))
	l.DebugContext(ctx, "Updating itinerary preferences")

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}

	var prefs types.ItineraryPreferences
	if err := api.DecodeJSONBody(w, r, &prefs); err != nil {
		l.WarnContext(ctx, "Failed to decode request body", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request body")
		api.ErrorResponse(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid request format: %s", err.Error()))
		return
	}

	err = u.userService.UpdateItineraryPreferences(ctx, profileID, &prefs)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update itinerary preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update itinerary preferences")
		if errors.Is(err, types.ErrNotFound) {
			api.ErrorResponse(w, r, http.StatusNotFound, "Profile not found")
			return
		}
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to update itinerary preferences")
		return
	}

	l.InfoContext(ctx, "Itinerary preferences updated successfully", slog.String("profileID", profileID.String()))
	api.WriteJSONResponse(w, r, http.StatusOK, map[string]string{"message": "Itinerary preferences updated successfully"})
}