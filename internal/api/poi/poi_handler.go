package poi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

var _ Handler = (*HandlerImpl)(nil)

type Handler interface {
	AddPoiToFavourites(w http.ResponseWriter, r *http.Request)
	RemovePoiFromFavourites(w http.ResponseWriter, r *http.Request)
	GetFavouritePOIsByUserID(w http.ResponseWriter, r *http.Request)
	GetPOIsByCityID(w http.ResponseWriter, r *http.Request)

	// Search POIs with filters
	GetPOIs(w http.ResponseWriter, r *http.Request)
}

type HandlerImpl struct {
	poiService POIService
	logger     *slog.Logger
}

func NewHandlerImpl(poiService POIService, logger *slog.Logger) *HandlerImpl {
	return &HandlerImpl{
		poiService: poiService,
		logger:     logger,
	}
}

// AddPoiToFavourites godoc
// @Summary      Add POI to Favourites
// @Description  Adds a point of interest to the user's favourites list
// @Tags         POI
// @Accept       json
// @Produce      json
// @Param        poi body types.AddPoiRequest true "POI ID to add to favourites"
// @Success      201 {object} interface{} "POI added to favourites successfully"
// @Failure      400 {object} types.Response "Invalid Input"
// @Failure      401 {object} types.Response "Authentication required"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /poi/favourites [post]
func (h *HandlerImpl) AddPoiToFavourites(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("AddPoiToFavourites").Start(r.Context(), "AddPoiToFavourites", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/save_itinerary"),
	))
	defer span.End()

	l := h.logger.With(slog.String("HandlerImpl", "AddPoiToFavourites"))
	l.DebugContext(ctx, "Add Poi to Favourites HandlerImpl invoked")

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

// RemovePoiFromFavourites godoc
// @Summary      Remove POI from Favourites
// @Description  Removes a point of interest from the user's favourites list
// @Tags         POI
// @Accept       json
// @Produce      json
// @Param        poi body types.AddPoiRequest true "POI ID to remove from favourites"
// @Success      200 {object} map[string]string "POI removed from favourites successfully"
// @Failure      400 {object} types.Response "Invalid Input"
// @Failure      401 {object} types.Response "Authentication required"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /poi/favourites [delete]
func (h *HandlerImpl) RemovePoiFromFavourites(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("RemovePoiFromFavourites").Start(r.Context(), "RemovePoiFromFavourites", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/save_itinerary"),
	))
	defer span.End()
	l := h.logger.With(slog.String("HandlerImpl", "RemovePoiFromFavourites"))
	l.DebugContext(ctx, "Remove Poi from Favourites HandlerImpl invoked")
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

// GetFavouritePOIsByUserID godoc
// @Summary      Get User's Favourite POIs
// @Description  Retrieves all points of interest that the authenticated user has marked as favourites
// @Tags         POI
// @Accept       json
// @Produce      json
// @Success      200 {array} interface{} "List of favourite POIs"
// @Failure      400 {object} types.Response "Invalid Input"
// @Failure      401 {object} types.Response "Authentication required"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /poi/favourites [get]
func (HandlerImpl *HandlerImpl) GetFavouritePOIsByUserID(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("LlmInteractionHandlerImpl").Start(r.Context(), "GetFavouritePOIsByUserID", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/favourite_pois"),
	))
	defer span.End()

	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "GetFavouritePOIsByUserID"))
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

	favouritePOIs, err := HandlerImpl.poiService.GetFavouritePOIsByUserID(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch favourite POIs by user ID", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch favourite POIs: %s", err.Error()))
		return
	}

	l.InfoContext(ctx, "Successfully fetched favourite POIs by user ID")
	api.WriteJSONResponse(w, r, http.StatusOK, favouritePOIs)
}

// GetPOIsByCityID godoc
// @Summary      Get POIs by City ID
// @Description  Retrieves all points of interest for a specific city
// @Tags         POI
// @Accept       json
// @Produce      json
// @Param        cityID path string true "City ID"
// @Success      200 {array} interface{} "List of POIs in the city"
// @Failure      400 {object} types.Response "Invalid Input"
// @Failure      500 {object} types.Response "Internal Server Error"
// @Router       /poi/city/{cityID} [get]
func (h *HandlerImpl) GetPOIsByCityID(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("GetPOIsByCityID").Start(r.Context(), "GetPOIsByCityID", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/poi/city"),
	))
	defer span.End()

	l := h.logger.With(slog.String("HandlerImpl", "GetPOIsByCityID"))
	l.DebugContext(ctx, "Get POIs by City ID HandlerImpl invoked")

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

// GetPOI from DB handler
func (h *HandlerImpl) GetPOIs(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("SearchPOIs").Start(r.Context(), "SearchPOIs", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/poi/search"),
	))
	defer span.End()
	l := h.logger.With(slog.String("HandlerImpl", "SearchPOIs"))
	l.DebugContext(ctx, "Search POIs HandlerImpl invoked")

	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, _ := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	radius, _ := strconv.ParseFloat(r.URL.Query().Get("radius"), 64)
	category := r.URL.Query().Get("category")

	filter := types.POIFilter{
		Location: types.GeoPoint{Latitude: lat, Longitude: lon},
		Radius:   radius,
		Category: category,
	}

	pois, err := h.poiService.SearchPOIs(ctx, filter)
	if err != nil {
		l.ErrorContext(ctx, "Failed to search POIs", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to search POIs: %s", err.Error()))
		return
	}

	l.InfoContext(ctx, "Successfully searched POIs")
	api.WriteJSONResponse(w, r, http.StatusOK, pois)
}
