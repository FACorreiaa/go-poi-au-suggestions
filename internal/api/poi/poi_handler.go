package poi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type POIHandler struct {
	poiService POIService
	logger     *slog.Logger
}

func NewPOIHandler(poiService POIService, logger *slog.Logger) *POIHandler {
	return &POIHandler{
		poiService: poiService,
		logger:     logger,
	}
}

// AddPoiToFavourites adds a POI to the user's favourites.
func (h *POIHandler) AddPoiToFavourites(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("AddPoiToFavourites").Start(r.Context(), "AddPoiToFavourites", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/save_itinerary"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "AddPoiToFavourites"))
	l.DebugContext(ctx, "Add Poi to Favourites handler invoked")

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
	span.SetAttributes(semconv.EnduserIDKey.String(userID.String()))
	l = l.With(slog.String("userID", userID.String()))

	var req types.AddPoiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		l.ErrorContext(ctx, "Failed to decode request body", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ID == "" {
		l.ErrorContext(ctx, "POI ID is required")
		api.ErrorResponse(w, r, http.StatusBadRequest, "POI ID is required")
		return
	}

	poiID, err := uuid.Parse(req.ID)
	if err != nil {
		l.ErrorContext(ctx, "Invalid POI ID format", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid POI ID format")
		return
	}

	savedItinerary, err := h.poiService.AddPoiToFavourites(ctx, userID, poiID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to save itinerary", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to save itinerary: %s", err.Error()))
		return
	}

	l.InfoContext(ctx, "Itinerary saved successfully")
	api.WriteJSONResponse(w, r, http.StatusCreated, savedItinerary)
}

// RemovePoiFromFavourites removes a POI from the user's favourites.
func (h *POIHandler) RemovePoiFromFavourites(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("RemovePoiFromFavourites").Start(r.Context(), "RemovePoiFromFavourites", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/save_itinerary"),
	))
	defer span.End()
	l := h.logger.With(slog.String("handler", "RemovePoiFromFavourites"))
	l.DebugContext(ctx, "Remove Poi from Favourites handler invoked")
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
	span.SetAttributes(semconv.EnduserIDKey.String(userID.String()))
	l = l.With(slog.String("userID", userID.String()))
	var req types.AddPoiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		l.ErrorContext(ctx, "Failed to decode request body", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.ID == "" {
		l.ErrorContext(ctx, "POI ID is required")
		api.ErrorResponse(w, r, http.StatusBadRequest, "POI ID is required")
		return
	}
	poiID, err := uuid.Parse(req.ID)
	if err != nil {
		l.ErrorContext(ctx, "Invalid POI ID format", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid POI ID format")
		return
	}
	if err := h.poiService.RemovePoiFromFavourites(ctx, poiID, userID); err != nil {
		l.ErrorContext(ctx, "Failed to remove POI from favourites", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to remove POI from favourites: %s", err.Error()))
		return
	}
	l.InfoContext(ctx, "POI removed from favourites successfully")
	api.WriteJSONResponse(w, r, http.StatusOK, map[string]string{"message": "POI removed from favourites successfully"})
}

func (handler *POIHandler) GetFavouritePOIsByUserID(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("LlmInteractionHandler").Start(r.Context(), "GetFavouritePOIsByUserID", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/favourite_pois"),
	))
	defer span.End()

	l := handler.logger.With(slog.String("handler", "GetFavouritePOIsByUserID"))
	l.DebugContext(ctx, "Fetching favourite POIs by user ID")

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
	span.SetAttributes(semconv.EnduserIDKey.String(userID.String()))
	l = l.With(slog.String("userID", userID.String()))

	favouritePOIs, err := handler.poiService.GetFavouritePOIsByUserID(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch favourite POIs by user ID", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch favourite POIs: %s", err.Error()))
		return
	}

	l.InfoContext(ctx, "Successfully fetched favourite POIs by user ID")
	api.WriteJSONResponse(w, r, http.StatusOK, favouritePOIs)
}

// GetPOIsByCityID retrieves POIs by city ID.
func (h *POIHandler) GetPOIsByCityID(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("GetPOIsByCityID").Start(r.Context(), "GetPOIsByCityID", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/poi/city"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "GetPOIsByCityID"))
	l.DebugContext(ctx, "Get POIs by City ID handler invoked")

	cityIDStr := chi.URLParam(r, "cityID")
	if cityIDStr == "" {
		l.ErrorContext(ctx, "City ID is required")
		api.ErrorResponse(w, r, http.StatusBadRequest, "City ID is required")
		return
	}

	cityID, err := uuid.Parse(cityIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid city ID format", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid city ID format")
		return
	}

	pois, err := h.poiService.GetPOIsByCityID(ctx, cityID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to get POIs by city ID", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to get POIs: %s", err.Error()))
		return
	}

	l.InfoContext(ctx, "Successfully retrieved POIs by city ID")
	api.WriteJSONResponse(w, r, http.StatusOK, pois)
	span.SetAttributes(semconv.EnduserIDKey.String(cityID.String()))
	span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(http.StatusOK))
	span.SetAttributes(semconv.HTTPRouteKey.String("/poi/city"))
}
