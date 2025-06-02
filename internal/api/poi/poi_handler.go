package poi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
	AddPoiToFavourites(w http.ResponseWriter, r *http.Request)
	RemovePoiFromFavourites(w http.ResponseWriter, r *http.Request)
	GetFavouritePOIsByUserID(w http.ResponseWriter, r *http.Request)
	GetPOIsByCityID(w http.ResponseWriter, r *http.Request)

	// Search POIs with filters
	GetPOIs(w http.ResponseWriter, r *http.Request)

	GetItinerary(w http.ResponseWriter, r *http.Request)
	GetItineraries(w http.ResponseWriter, r *http.Request)
	UpdateItinerary(w http.ResponseWriter, r *http.Request)
}

type HandlerImpl struct {
	poiService Service
	logger     *slog.Logger
}

func NewHandlerImpl(poiService Service, logger *slog.Logger) *HandlerImpl {
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

// GetItinerary godoc
// @Summary      Get Saved Itinerary
// @Description  Retrieves a specific saved itinerary for the authenticated user.
// @Tags         Itineraries
// @Produce      json
// @Param        itinerary_id path string true "Itinerary ID (UUID)"
// @Success      200 {object} types.UserSavedItinerary "Successfully retrieved itinerary"
// @Failure      400 {object} types.Response "Invalid Itinerary ID format"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      404 {object} types.Response "Itinerary not found"
// @Failure      500 {object} types.Response "Internal server error"
// @Security     BearerAuth
// @Router       /itineraries/{itinerary_id} [get]
func (h *HandlerImpl) GetItinerary(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("LlmInteractionHandler").Start(r.Context(), "GetItinerary")
	defer span.End()
	l := h.logger.With(slog.String("handler", "GetItinerary"))

	userIDStr, ok := auth.GetUserIDFromContext(ctx) // Assuming auth.GetUserIDFromContext exists
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "Unauthorized - User ID missing")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.String("userID_str", userIDStr), slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid User ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}
	span.SetAttributes(attribute.String("user.id", userID.String()))

	itineraryIDStr := chi.URLParam(r, "itinerary_id") // Assuming you use Chi and a path parameter
	if itineraryIDStr == "" {
		l.WarnContext(ctx, "Itinerary ID missing from path")
		span.SetStatus(codes.Error, "Bad Request - Itinerary ID missing")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Itinerary ID is required in path")
		return
	}
	itineraryID, err := uuid.Parse(itineraryIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid itinerary ID format", slog.String("itineraryID_str", itineraryIDStr), slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid Itinerary ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid itinerary ID format")
		return
	}
	span.SetAttributes(attribute.String("itinerary.id", itineraryID.String()))
	l = l.With(slog.String("itineraryID", itineraryID.String()))

	l.DebugContext(ctx, "Attempting to fetch itinerary")
	itinerary, err := h.poiService.GetItinerary(ctx, userID, itineraryID)
	if err != nil {
		l.ErrorContext(ctx, "Service failed to get itinerary", slog.Any("error", err))
		span.RecordError(err)
		// Check if the error is a "not found" type error
		// This depends on how your repository/service signals "not found"
		// For pgx.ErrNoRows, your repo already formats it.
		if strings.Contains(err.Error(), "no itinerary found") { // Simple string check, improve if possible
			span.SetStatus(codes.Error, "Itinerary not found")
			api.ErrorResponse(w, r, http.StatusNotFound, "Itinerary not found")
		} else {
			span.SetStatus(codes.Error, "Internal server error")
			api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to retrieve itinerary: "+err.Error())
		}
		return
	}

	l.InfoContext(ctx, "Successfully fetched itinerary", slog.String("title", itinerary.Title))
	span.SetStatus(codes.Ok, "Itinerary retrieved")
	api.WriteJSONResponse(w, r, http.StatusOK, itinerary)
}

// GetItineraries godoc
// @Summary      List Saved Itineraries
// @Description  Retrieves a paginated list of saved itineraries for the authenticated user.
// @Tags         Itineraries
// @Produce      json
// @Param        page query int false "Page number for pagination (default 1)"
// @Param        page_size query int false "Number of items per page (default 10)"
// @Success      200 {object} llmChat.PaginatedUserItinerariesResponse "Successfully retrieved list of itineraries"
// @Failure      400 {object} types.Response "Invalid pagination parameters"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      500 {object} types.Response "Internal server error"
// @Security     BearerAuth
// @Router       /itineraries [get]
func (h *HandlerImpl) GetItineraries(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("LlmInteractionHandler").Start(r.Context(), "GetItineraries")
	defer span.End()
	l := h.logger.With(slog.String("handler", "GetItineraries"))

	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "Unauthorized - User ID missing")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.String("userID_str", userIDStr), slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid User ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}
	span.SetAttributes(attribute.String("user.id", userID.String()))

	// Get pagination parameters from query
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page <= 0 {
		page = 1 // Default to page 1
	}
	if pageSize <= 0 {
		pageSize = 10 // Default page size
	}
	if pageSize > 100 { // Max page size
		pageSize = 100
	}
	span.SetAttributes(attribute.Int("query.page", page), attribute.Int("query.page_size", pageSize))
	l = l.With(slog.Int("page", page), slog.Int("pageSize", pageSize))

	l.DebugContext(ctx, "Attempting to fetch itineraries")
	paginatedResponse, err := h.poiService.GetItineraries(ctx, userID, page, pageSize)
	if err != nil {
		l.ErrorContext(ctx, "Service failed to get itineraries", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Internal server error")
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to retrieve itineraries: "+err.Error())
		return
	}

	l.InfoContext(ctx, "Successfully fetched itineraries", slog.Int("count", len(paginatedResponse.Itineraries)), slog.Int("total_records", paginatedResponse.TotalRecords))
	span.SetStatus(codes.Ok, "Itineraries retrieved")
	api.WriteJSONResponse(w, r, http.StatusOK, paginatedResponse)
}

// UpdateItinerary godoc
// @Summary      Update Saved Itinerary
// @Description  Updates specified fields of a saved itinerary for the authenticated user.
// @Tags         Itineraries
// @Accept       json
// @Produce      json
// @Param        itinerary_id path string true "Itinerary ID (UUID) to update"
// @Param        updateData body types.UpdateItineraryRequest true "Fields to update"
// @Success      200 {object} types.UserSavedItinerary "Successfully updated itinerary"
// @Failure      400 {object} types.Response "Invalid Itinerary ID format or request body"
// @Failure      401 {object} types.Response "Unauthorized"
// @Failure      403 {object} types.Response "Forbidden (user does not own this itinerary)"
// @Failure      404 {object} types.Response "Itinerary not found"
// @Failure      500 {object} types.Response "Internal server error"
// @Security     BearerAuth
// @Router       /itineraries/{itinerary_id} [put]
func (h *HandlerImpl) UpdateItinerary(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("LlmInteractionHandler").Start(r.Context(), "UpdateItinerary")
	defer span.End()
	l := h.logger.With(slog.String("handler", "UpdateItinerary"))

	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "Unauthorized - User ID missing")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.String("userID_str", userIDStr), slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid User ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}
	span.SetAttributes(attribute.String("user.id", userID.String()))

	itineraryIDStr := chi.URLParam(r, "itinerary_id")
	if itineraryIDStr == "" {
		l.WarnContext(ctx, "Itinerary ID missing from path")
		span.SetStatus(codes.Error, "Bad Request - Itinerary ID missing")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Itinerary ID is required in path")
		return
	}
	itineraryID, err := uuid.Parse(itineraryIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid itinerary ID format", slog.String("itineraryID_str", itineraryIDStr), slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid Itinerary ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid itinerary ID format")
		return
	}
	span.SetAttributes(attribute.String("itinerary.id", itineraryID.String()))
	l = l.With(slog.String("itineraryID", itineraryID.String()))

	var req types.UpdateItineraryRequest
	if err := api.DecodeJSONBody(w, r, &req); err != nil {
		l.WarnContext(ctx, "Failed to decode UpdateItinerary request body", slog.Any("error", err))
		span.RecordError(err)
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Service layer handles the case of no actual updates being provided.
	l.DebugContext(ctx, "Attempting to update itinerary", slog.Any("request_body", req))
	updatedItinerary, err := h.poiService.UpdateItinerary(ctx, userID, itineraryID, req)
	if err != nil {
		l.ErrorContext(ctx, "Service failed to update itinerary", slog.Any("error", err))
		span.RecordError(err)
		if strings.Contains(err.Error(), "not found for user") || strings.Contains(err.Error(), "no itinerary found") { // Adapt this check
			span.SetStatus(codes.Error, "Itinerary not found or not owned")
			api.ErrorResponse(w, r, http.StatusNotFound, "Itinerary not found or you do not have permission to modify it.")
		} else if strings.Contains(err.Error(), "no fields to update") { // If service returns this error
			span.SetStatus(codes.Error, "Bad Request - No fields to update")
			api.ErrorResponse(w, r, http.StatusBadRequest, err.Error())
		} else {
			span.SetStatus(codes.Error, "Internal server error")
			api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to update itinerary: "+err.Error())
		}
		return
	}

	l.InfoContext(ctx, "Successfully updated itinerary", slog.String("title", updatedItinerary.Title))
	span.SetStatus(codes.Ok, "Itinerary updated")
	api.WriteJSONResponse(w, r, http.StatusOK, updatedItinerary)
}
