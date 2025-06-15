package llmChat

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/codes"

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
	// GetPrompResponse poi
	//GetPrompResponse(w http.ResponseWriter, r *http.Request)
	SaveItenerary(w http.ResponseWriter, r *http.Request)
	RemoveItenerary(w http.ResponseWriter, r *http.Request)
	GetPOIDetails(w http.ResponseWriter, r *http.Request)

	// GetHotelsByPreference hotels
	GetHotelsByPreference(w http.ResponseWriter, r *http.Request)
	GetHotelsNearby(w http.ResponseWriter, r *http.Request)
	// TODO
	GetHotelByID(w http.ResponseWriter, r *http.Request)

	// GetRestaurantsByPreferences restaurants
	GetRestaurantsByPreferences(w http.ResponseWriter, r *http.Request)
	GetRestaurantsNearby(w http.ResponseWriter, r *http.Request)
	// TODO
	GetRestaurantDetails(w http.ResponseWriter, r *http.Request)

	// GetNearbyRecommendations(w http.ResponseWriter, r *http.Request)
	GetNearbyRecommendations(w http.ResponseWriter, r *http.Request)

	// RAG-enabled chat methods
	// RAGEnabledChatQuery(w http.ResponseWriter, r *http.Request)
	// SearchSimilarPOIs(w http.ResponseWriter, r *http.Request)

	// sessions
	StartChatSessionHandler(w http.ResponseWriter, r *http.Request)
	StartChatSessionStreamHandler(w http.ResponseWriter, r *http.Request)
	ContinueChatSessionHandler(w http.ResponseWriter, r *http.Request)

	// Context-aware chat handlers
	// StartHotelChatSession(w http.ResponseWriter, r *http.Request)
	// StartHotelChatSessionStream(w http.ResponseWriter, r *http.Request)
	// ContinueHotelChatSession(w http.ResponseWriter, r *http.Request)
	// ContinueHotelChatSessionStream(w http.ResponseWriter, r *http.Request)

	// StartRestaurantChatSession(w http.ResponseWriter, r *http.Request)
	// StartRestaurantChatSessionStream(w http.ResponseWriter, r *http.Request)
	// ContinueRestaurantChatSession(w http.ResponseWriter, r *http.Request)
	// ContinueRestaurantChatSessionStream(w http.ResponseWriter, r *http.Request)

	// StartItineraryChatSession(w http.ResponseWriter, r *http.Request)
	// StartItineraryChatSessionStream(w http.ResponseWriter, r *http.Request)
	// ContinueItineraryChatSession(w http.ResponseWriter, r *http.Request)
	// ContinueItineraryChatSessionStream(w http.ResponseWriter, r *http.Request)
}
type HandlerImpl struct {
	llmInteractionService LlmInteractiontService
	logger                *slog.Logger
}

func NewLLMHandlerImpl(llmInteractionService LlmInteractiontService, logger *slog.Logger) *HandlerImpl {
	return &HandlerImpl{
		llmInteractionService: llmInteractionService,
		logger:                logger,
	}
}

func (h *HandlerImpl) StartChatSessionHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "StartChatSession", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/prompt-response/chat/sessions/{profileID}"),
	))
	defer span.End()
	l := h.logger.With(slog.String("HandlerImpl", "StartChatSession"))

	// Support both legacy and new request formats
	var req struct {
		CityName       string                `json:"city_name"`
		ContextType    types.ChatContextType `json:"context_type,omitempty"`
		InitialMessage string                `json:"initial_message,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Default to general context for backward compatibility
	if req.ContextType == "" {
		req.ContextType = types.ContextGeneral
	}

	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
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
	span.SetAttributes(attribute.String("app.profile.id", profileID.String()))

	userLocation := &types.UserLocation{
		UserLat: 38.7223,
		UserLon: -9.1393,
	}

	// Pass context type and initial message to service
	prompt := req.InitialMessage
	if prompt == "" {
		prompt = getDefaultPromptForContext(req.ContextType, req.CityName)
	}

	// sessionID, itinerary, err := h.llmInteractionService.StartNewSessionWithContext(ctx, userID, profileID, req.CityName, prompt, userLocation, req.ContextType)
	// if err != nil {
	// 	// Fallback to original method for backward compatibility
	// 	sessionID, itinerary, err = h.llmInteractionService.StartNewSession(ctx, userID, profileID, req.CityName, prompt, userLocation)
	// 	if err != nil {
	// 		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to start session: "+err.Error())
	// 		return
	// 	}
	// }
	sessionID, itinerary, err := h.llmInteractionService.StartNewSession(ctx, userID, profileID, req.CityName, prompt, userLocation)

	response := struct {
		SessionID uuid.UUID             `json:"session_id"`
		Data      *types.AiCityResponse `json:"data"`
	}{SessionID: sessionID, Data: itinerary}
	api.WriteJSONResponse(w, r, http.StatusCreated, response)
}

func (h *HandlerImpl) StartChatSessionStream(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "StartChatSessionStream", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/prompt-response/chat/sessions/{profileID}"),
	))
	defer span.End()
	l := h.logger.With(slog.String("HandlerImpl", "StartChatSessionStream"))

	// Parse request body
	var req struct {
		CityName string `json:"city_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Authentication and user ID validation
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	// Profile ID validation
	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid profile ID format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
		return
	}
	span.SetAttributes(attribute.String("app.profile.id", profileID.String()))

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable buffering in proxies like Nginx

	// Ensure the response is flushed to the client
	flusher, ok := w.(http.Flusher)
	if !ok {
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	// User location (hardcoded for now)
	userLocation := &types.UserLocation{
		UserLat: 41.3851,
		UserLon: 2.1734,
	}

	// Start the streaming session
	streamingResp, err := h.llmInteractionService.StartNewSessionStreamed(ctx, userID, profileID, req.CityName, "", userLocation)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to start session: "+err.Error())
		return
	}

	// Send session ID as the first event
	sessionData := struct {
		SessionID uuid.UUID `json:"session_id"`
	}{SessionID: streamingResp.SessionID}
	sessionJSON, _ := json.Marshal(sessionData)
	fmt.Fprintf(w, "event: session\n")
	fmt.Fprintf(w, "data: %s\n\n", sessionJSON)
	flusher.Flush()

	// Stream events from streamingResp.Stream
	for {
		select {
		case event, ok := <-streamingResp.Stream:
			if !ok {
				// Channel closed, streaming complete
				fmt.Fprintf(w, "event: complete\n")
				fmt.Fprintf(w, "data: {}\n\n")
				flusher.Flush()
				return
			}
			// Send the event
			eventJSON, err := json.Marshal(event)
			if err != nil {
				l.ErrorContext(ctx, "Failed to marshal stream event", slog.Any("error", err))
				continue
			}
			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n\n", eventJSON)
			flusher.Flush()
		case <-ctx.Done():
			// Client disconnected or context canceled
			l.InfoContext(ctx, "Client disconnected or context canceled")
			return
		case <-r.Context().Done():
			// Client explicitly closed the connection
			l.InfoContext(ctx, "Client closed connection")
			return
		}
	}
}

func (h *HandlerImpl) ContinueChatSessionHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionIDStr := chi.URLParam(r, "sessionID")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid session ID")
		return
	}

	// Support both legacy and new request formats
	var req struct {
		Message     string                `json:"message"`
		CityName    string                `json:"city_name,omitempty"`
		ContextType types.ChatContextType `json:"context_type,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Default to general context for backward compatibility
	if req.ContextType == "" {
		req.ContextType = types.ContextGeneral
	}

	userLocation := &types.UserLocation{
		UserLat: 41.3851,
		UserLon: 2.1734,
	}

	// Try context-aware method first, fallback to original for backward compatibility
	// itinerary, err := h.llmInteractionService.ContinueSessionWithContext(ctx, sessionID, req.Message, userLocation, req.ContextType)
	// if err != nil {
	// 	// Fallback to original method
	// 	itinerary, err = h.llmInteractionService.ContinueSession(ctx, sessionID, req.Message, userLocation)
	// 	if err != nil {
	// 		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to continue session: "+err.Error())
	// 		return
	// 	}
	// }
	itinerary, err := h.llmInteractionService.ContinueSession(ctx, sessionID, req.Message, userLocation)

	response := struct {
		Data *types.AiCityResponse `json:"data"`
	}{Data: itinerary}
	api.WriteJSONResponse(w, r, http.StatusOK, response)
}

// Helper function to generate default prompts based on context
func getDefaultPromptForContext(contextType types.ChatContextType, cityName string) string {
	switch contextType {
	case types.ContextHotels:
		return fmt.Sprintf("Find hotels in %s", cityName)
	case types.ContextRestaurants:
		return fmt.Sprintf("Find restaurants in %s", cityName)
	case types.ContextItineraries:
		return fmt.Sprintf("Create an itinerary for %s", cityName)
	default:
		return fmt.Sprintf("Tell me about %s", cityName)
	}
}

// func (HandlerImpl *HandlerImpl) GetPrompResponse(w http.ResponseWriter, r *http.Request) {
// 	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "GetPrompResponse", trace.WithAttributes(
// 		semconv.HTTPRequestMethodKey.String(r.Method),
// 		semconv.HTTPRouteKey.String("/user/interests"),
// 	))
// 	defer span.End()

// 	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "GetUserProfile"))
// 	l.DebugContext(ctx, "Fetching user profile")

// 	userIDStr, ok := auth.GetUserIDFromContext(ctx)
// 	if !ok || userIDStr == "" {
// 		l.ErrorContext(ctx, "User ID not found in context")
// 		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
// 	}

// 	userID, err := uuid.Parse(userIDStr)
// 	if err != nil {
// 		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
// 	}
// 	span.SetAttributes(semconv.EnduserIDKey.String(userID.String()))
// 	l = l.With(slog.String("userID", userID.String()))

// 	profileIDStr := chi.URLParam(r, "profileID")
// 	profileID, err := uuid.Parse(profileIDStr)
// 	if err != nil {
// 		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
// 		span.RecordError(err)
// 		span.SetStatus(codes.Error, "Invalid profile ID format")
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format in URL")
// 		return
// 	}
// 	span.SetAttributes(attribute.String("app.profile.id", profileID.String()))
// 	l = l.With(slog.String("profileID", profileID.String()))

// 	cityName := r.URL.Query().Get("city")
// 	if cityName == "" {
// 		l.WarnContext(ctx, "City name missing from query parameters")
// 		span.SetStatus(codes.Error, "City name missing")
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Query parameter 'city' is required.")
// 		return
// 	}
// 	span.SetAttributes(attribute.String("app.city.name", cityName))
// 	l = l.With(slog.String("cityName", cityName))

// 	l.InfoContext(ctx, "Processing itinerary request")

// 	// TODO set userLocation from route later
// 	userLocation := &types.UserLocation{
// 		UserLat: 41.3851,
// 		UserLon: 2.1734,
// 	}

// 	// sessionIDParam := r.URL.Query().Get("session_id")

// 	// isNewConversation := (sessionIDParam == "")
// 	// var sessionIDPtr *string
// 	// if !isNewConversation {
// 	// 	sessionIDPtr = &sessionIDParam
// 	// }

// 	//
// 	// userRequest := UserRequest{
// 	// 	Interests:  []string{"art", "history"},
// 	// 	Tags:       []string{"family-friendly"},
// 	// 	Categories: []string{"restaurants"},
// 	// }

// 	itineraryResponse, err := HandlerImpl.llmInteractionService.GetIteneraryResponse(ctx, cityName, userID, profileID, userLocation)
// 	responsePayload := struct {
// 		Data *types.AiCityResponse `json:"data"`
// 		//SessionID string                `json:"session_id"` // IMPORTANT: Send this back
// 	}{
// 		Data: itineraryResponse,
// 		//SessionID: chatSessionID,
// 	}

// 	if err != nil {
// 		l.ErrorContext(ctx, "Service failed to generate prompt response", slog.Any("error", err))
// 		span.RecordError(err)
// 		span.SetStatus(codes.Error, "Service error")
// 		// Determine appropriate HTTP status code based on the error type
// 		// For example, if it's a validation error from the service, could be 400.
// 		// If it's an AI error or DB error, could be 500.
// 		// For now, using a generic 500.
// 		// You might want to check error types (e.g., errors.Is(err, types.ErrNotFound)) for more specific statuses.
// 		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to generate itinerary: %s", err.Error()))
// 		return
// 	}

// 	if itineraryResponse == nil {
// 		l.ErrorContext(ctx, "Service returned nil itinerary response without error")
// 		span.SetStatus(codes.Error, "Service returned nil response")
// 		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to generate itinerary: received empty response from service.")
// 		return
// 	}

// 	// 5. Send successful response
// 	l.InfoContext(ctx, "Successfully generated itinerary response",
// 		slog.String("itinerary_name", itineraryResponse.AIItineraryResponse.ItineraryName),
// 		slog.Int("poi_count", len(itineraryResponse.PointsOfInterest)))

// 	span.SetAttributes(attribute.String("app.itinerary.name", itineraryResponse.AIItineraryResponse.ItineraryName))
// 	span.SetStatus(codes.Ok, "Itinerary generated")
// 	l.InfoContext(ctx, "User preference profile created successfully")
// 	api.WriteJSONResponse(w, r, http.StatusCreated, responsePayload)
// }

func (HandlerImpl *HandlerImpl) SaveItenerary(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "SaveItenerary", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/save_itinerary"),
	))
	defer span.End()

	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "SaveItenerary"))
	l.DebugContext(ctx, "Saving itinerary")

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

	var req types.BookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		l.ErrorContext(ctx, "Failed to decode request body", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.LlmInteractionID == uuid.Nil {
		l.ErrorContext(ctx, "LlmInteractionID is required", slog.Any("itinerary", req))
		api.ErrorResponse(w, r, http.StatusBadRequest, "LlmInteractionID is required")
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		l.ErrorContext(ctx, "Title is required", slog.Any("title", req))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Title is required")
		return
	}

	savedItinerary, err := HandlerImpl.llmInteractionService.SaveItenerary(ctx, userID, req)
	if err != nil {
		l.ErrorContext(ctx, "Failed to save itinerary", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to save itinerary: %s", err.Error()))
		return
	}

	l.InfoContext(ctx, "Itinerary saved successfully")
	api.WriteJSONResponse(w, r, http.StatusCreated, savedItinerary)
}

func (HandlerImpl *HandlerImpl) RemoveItenerary(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "RemoveItenerary", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/remove_itinerary"),
	))
	defer span.End()

	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "RemoveItenerary"))
	l.DebugContext(ctx, "Removing itinerary")

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

	itineraryIDStr := chi.URLParam(r, "itineraryID")
	itineraryID, err := uuid.Parse(itineraryIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid itinerary ID format", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid itinerary ID format")
		return
	}
	span.SetAttributes(attribute.String("app.itinerary.id", itineraryID.String()))
	l = l.With(slog.String("itineraryID", itineraryID.String()))

	if err := HandlerImpl.llmInteractionService.RemoveItenerary(ctx, userID, itineraryID); err != nil {
		l.ErrorContext(ctx, "Failed to remove itinerary", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to remove itinerary: %s", err.Error()))
		return
	}

	l.InfoContext(ctx, "Itinerary removed successfully")
	api.WriteJSONResponse(w, r, http.StatusNoContent, nil)
}

func (HandlerImpl *HandlerImpl) GetPOIDetails(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "GetPOIDetails", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/get_poi_details"),
	))
	defer span.End()

	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "GetPOIDetails"))
	l.DebugContext(ctx, "Get POI details")

	// Authenticate user
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		l.ErrorContext(ctx, "User ID not found in context")
		span.SetStatus(codes.Error, "Unauthorized")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid user ID format", slog.Any("error", err))
		span.SetStatus(codes.Error, "Invalid user ID")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}
	span.SetAttributes(semconv.EnduserIDKey.String(userID.String()))
	l = l.With(slog.String("userID", userID.String()))

	// Decode request body
	var req types.POIDetailrequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		l.ErrorContext(ctx, "Failed to decode request body", slog.Any("error", err))
		span.SetStatus(codes.Error, "Invalid request body")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.CityName == "" {
		l.ErrorContext(ctx, "City name is required")
		span.SetStatus(codes.Error, "Missing city name")
		api.ErrorResponse(w, r, http.StatusBadRequest, "City name is required")
		return
	}
	if req.Latitude < -90 || req.Latitude > 90 || req.Longitude < -180 || req.Longitude > 180 {
		l.ErrorContext(ctx, "Invalid coordinates", slog.Float64("latitude", req.Latitude), slog.Float64("longitude", req.Longitude))
		span.SetStatus(codes.Error, "Invalid coordinates")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid latitude or longitude")
		return
	}

	// Convert to service request type (if different)
	serviceReq := types.POIDetailrequest{
		CityName:  req.CityName,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	}

	// Call service to get POI details
	pois, err := HandlerImpl.llmInteractionService.GetPOIDetailsResponse(ctx, userID, serviceReq.CityName, serviceReq.Latitude, serviceReq.Longitude)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch POI details", slog.Any("error", err))
		span.SetStatus(codes.Error, "Service error")
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to fetch POI details")
		return
	}

	// Prepare response
	response := struct {
		POIs *types.POIDetailedInfo `json:"pois"`
	}{
		POIs: pois,
	}

	// Encode response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		l.ErrorContext(ctx, "Failed to encode response", slog.Any("error", err))
		span.SetStatus(codes.Error, "Response encoding failed")
		return
	}

	l.InfoContext(ctx, "Successfully fetched POI details")
	span.SetStatus(codes.Ok, "Success")
}

// GetHotelsByPreference TODO FIX ALL ANE BELOW DB ACCESS
func (HandlerImpl *HandlerImpl) GetHotelsByPreference(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "HotelsByPreference", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/hotels_by_preference"),
	))
	defer span.End()

	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "HotelsByPreference"))
	l.DebugContext(ctx, "Fetching hotels by preference")

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

	var req types.HotelPreferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		l.ErrorContext(ctx, "Failed to decode request body", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.City == "" {
		l.ErrorContext(ctx, "City name is required")
		api.ErrorResponse(w, r, http.StatusBadRequest, "City name is required")
		return
	}

	if req.Lat < -90 || req.Lat > 90 || req.Lon < -180 || req.Lon > 180 {
		l.ErrorContext(ctx, "Invalid coordinates", slog.Float64("latitude", req.Lat), slog.Float64("longitude", req.Lon))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid latitude or longitude")
		return
	}

	// Validate preferences
	if req.Preferences.NumberOfGuests <= 0 {
		req.Preferences.NumberOfGuests = 1 // Default
	}
	if req.Preferences.PreferredCategories == "" {
		req.Preferences.PreferredCategories = "budget" // Default
	}
	if req.Preferences.PreferredCheckIn.IsZero() {
		req.Preferences.PreferredCheckIn = time.Now()
	}
	if req.Preferences.PreferredCheckOut.IsZero() {
		req.Preferences.PreferredCheckOut = time.Now().Add(24 * time.Hour)
	}
	if req.Preferences.NumberOfNights <= 0 {
		req.Preferences.NumberOfNights = 1
	}
	if req.Preferences.NumberOfRooms <= 0 {
		req.Preferences.NumberOfRooms = 1
	}

	hotels, err := HandlerImpl.llmInteractionService.GetHotelsByPreferenceResponse(ctx, userID, req.City, req.Lat, req.Lon, req.Preferences)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch hotels by preference", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch hotels: %s", err.Error()))
		return
	}

	response := struct {
		Hotels []types.HotelDetailedInfo `json:"hotels"`
	}{Hotels: hotels}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		l.ErrorContext(ctx, "Failed to encode response", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to encode response")
		return
	}

	l.InfoContext(ctx, "Successfully fetched hotels by preference")
	span.SetStatus(codes.Ok, "Success")
}

func (HandlerImpl *HandlerImpl) GetHotelsNearby(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "HotelsNearby", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/hotels_nearby"),
	))
	defer span.End()

	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "HotelsNearby"))
	l.DebugContext(ctx, "Fetching nearby hotels")

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

	var req types.HotelPreferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		l.ErrorContext(ctx, "Failed to decode request body", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.City == "" {
		l.ErrorContext(ctx, "City name is required")
		api.ErrorResponse(w, r, http.StatusBadRequest, "City name is required")
		return
	}

	if req.Lat < -90 || req.Lat > 90 || req.Lon < -180 || req.Lon > 180 {
		l.ErrorContext(ctx, "Invalid coordinates", slog.Float64("latitude", req.Lat), slog.Float64("longitude", req.Lon))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid latitude or longitude")
		return
	}

	if req.Distance <= 0 {
		req.Distance = 5.0 // Default search radius in km
	}

	userLocation := &types.UserLocation{
		UserLat:        req.Lat,
		UserLon:        req.Lon,
		SearchRadiusKm: req.Distance,
	}
	hotels, err := HandlerImpl.llmInteractionService.GetHotelsNearbyResponse(ctx, userID, req.City, userLocation)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch nearby hotels", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch hotels: %s", err.Error()))
		return
	}
	response := struct {
		Hotels []types.HotelDetailedInfo `json:"hotels"`
	}{Hotels: hotels}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		l.ErrorContext(ctx, "Failed to encode response", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to encode response")
		return
	}
	l.InfoContext(ctx, "Successfully fetched nearby hotels")
	span.SetStatus(codes.Ok, "Success")
	api.WriteJSONResponse(w, r, http.StatusOK, response)
	span.SetAttributes(attribute.String("app.city.name", req.City),
		attribute.Float64("app.location.latitude", req.Lat),
		attribute.Float64("app.location.longitude", req.Lon),
		attribute.String("app.preferences.preferred_category", req.Preferences.PreferredCategories),
		attribute.String("app.preferences.preferred_price_range", req.Preferences.MaxPriceRange),
		attribute.Float64("app.preferences.preferred_rating", req.Preferences.MinRating),
		attribute.Int64("app.preferences.number_of_nights", req.Preferences.NumberOfNights),
		attribute.String("app.preferences.preferred_check_in", req.Preferences.PreferredCheckIn.Format(time.RFC3339)),
		attribute.String("app.preferences.preferred_check_out", req.Preferences.PreferredCheckOut.Format(time.RFC3339)),
	)
}

func (HandlerImpl *HandlerImpl) GetHotelByID(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "HotelByID", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/hotel_by_id"),
	))
	defer span.End()

	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "HotelByID"))
	l.DebugContext(ctx, "Fetching hotel by ID")

	hotelIDStr := chi.URLParam(r, "hotelID")
	hotelID, err := uuid.Parse(hotelIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid hotel ID format", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid hotel ID format")
		return
	}
	span.SetAttributes(attribute.String("app.hotel.id", hotelID.String()))
	l = l.With(slog.String("hotelID", hotelID.String()))
	hotel, err := HandlerImpl.llmInteractionService.GetHotelByIDResponse(ctx, hotelID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch hotel by ID", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch hotel: %s", err.Error()))
		return
	}
	if hotel == nil {
		l.ErrorContext(ctx, "Hotel not found", slog.String("hotelID", hotelID.String()))
		api.ErrorResponse(w, r, http.StatusNotFound, "Hotel not found")
		return
	}
	response := struct {
		Hotel *types.HotelDetailedInfo `json:"hotel"`
	}{Hotel: hotel}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		l.ErrorContext(ctx, "Failed to encode response", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to encode response")
		return
	}
	l.InfoContext(ctx, "Successfully fetched hotel by ID", slog.String("hotelID", hotelID.String()))
	span.SetStatus(codes.Ok, "Success")
	api.WriteJSONResponse(w, r, http.StatusOK, response)
	span.SetAttributes(attribute.String("app.hotel.name", hotel.Name),
		attribute.Float64("app.hotel.latitude", hotel.Latitude),
		attribute.Float64("app.hotel.longitude", hotel.Longitude),
		attribute.Float64("app.hotel.rating", hotel.Rating),
		attribute.String("app.hotel.category", hotel.Category),
	)
}

func (HandlerImpl *HandlerImpl) GetRestaurantsByPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "GetRestaurantsByPreferences", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/restaurants_by_preferences"),
	))
	defer span.End()

	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "GetRestaurantsByPreferences"))
	l.DebugContext(ctx, "Fetching restaurants by preferences")

	// Parse request body
	var req struct {
		City        string                          `json:"city"`
		Lat         float64                         `json:"lat"`
		Lon         float64                         `json:"lon"`
		Preferences types.RestaurantUserPreferences `json:"preferences"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		l.ErrorContext(ctx, "Failed to decode request body", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get user ID from context
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

	// set default values for preferences if not provided
	if req.Preferences.PreferredCuisine == "" {
		req.Preferences.PreferredCuisine = "any"
	}
	if req.Preferences.PreferredPriceRange == "" {
		req.Preferences.PreferredPriceRange = "any"
	}
	if req.Preferences.DietaryRestrictions == "" {
		req.Preferences.DietaryRestrictions = "none"
	}
	if req.Preferences.Ambiance == "" {
		req.Preferences.Ambiance = "any"
	}
	if req.Preferences.SpecialFeatures == "" {
		req.Preferences.SpecialFeatures = "none"
	}

	// Call service method
	restaurants, err := HandlerImpl.llmInteractionService.GetRestaurantsByPreferencesResponse(ctx, userID, req.City, req.Lat, req.Lon, req.Preferences)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch restaurants by preferences", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch restaurants: %s", err.Error()))
		return
	}

	// Prepare response
	response := struct {
		Restaurants []types.RestaurantDetailedInfo `json:"restaurants"`
	}{Restaurants: restaurants}

	// Encode response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		l.ErrorContext(ctx, "Failed to encode response", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to encode response")
		return
	}

	l.InfoContext(ctx, "Successfully fetched restaurants by preferences")
	span.SetStatus(codes.Ok, "Success")
}

func (HandlerImpl *HandlerImpl) GetRestaurantsNearby(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "GetRestaurantsNearby", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/restaurants_nearby"),
	))
	defer span.End()

	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "GetRestaurantsNearby"))
	l.DebugContext(ctx, "Fetching nearby restaurants")

	// Get query parameters
	city := r.URL.Query().Get("city")
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")

	// Parse latitude and longitude
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		l.ErrorContext(ctx, "Invalid latitude", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid latitude")
		return
	}
	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		l.ErrorContext(ctx, "Invalid longitude", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid longitude")
		return
	}

	// Get user ID from context
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

	// Create UserLocation
	userLocation := types.UserLocation{
		UserLat: lat,
		UserLon: lon,
	}

	// Call service method
	restaurants, err := HandlerImpl.llmInteractionService.GetRestaurantsNearbyResponse(ctx, userID, city, userLocation)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch nearby restaurants", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch restaurants: %s", err.Error()))
		return
	}

	// Prepare response
	response := struct {
		Restaurants []types.RestaurantDetailedInfo `json:"restaurants"`
	}{Restaurants: restaurants}

	// Encode response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		l.ErrorContext(ctx, "Failed to encode response", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to encode response")
		return
	}

	l.InfoContext(ctx, "Successfully fetched nearby restaurants")
	span.SetStatus(codes.Ok, "Success")
}

func (HandlerImpl *HandlerImpl) GetRestaurantDetails(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "GetRestaurantDetails", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/restaurant/{restaurantID}"),
	))
	defer span.End()

	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "GetRestaurantDetails"))
	l.DebugContext(ctx, "Fetching restaurant details")

	// Get restaurant ID from route parameter
	restaurantIDStr := chi.URLParam(r, "restaurantID")
	restaurantID, err := uuid.Parse(restaurantIDStr)
	if err != nil {
		l.ErrorContext(ctx, "Invalid restaurant ID format", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid restaurant ID format")
		return
	}
	span.SetAttributes(attribute.String("app.restaurant.id", restaurantID.String()))

	// Call service method
	restaurant, err := HandlerImpl.llmInteractionService.GetRestaurantDetailsResponse(ctx, restaurantID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch restaurant details", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch restaurant: %s", err.Error()))
		return
	}
	if restaurant == nil {
		l.WarnContext(ctx, "Restaurant not found", slog.String("restaurantID", restaurantID.String()))
		api.ErrorResponse(w, r, http.StatusNotFound, "Restaurant not found")
		return
	}

	// Prepare response
	response := struct {
		Restaurant *types.RestaurantDetailedInfo `json:"restaurant"`
	}{Restaurant: restaurant}

	// Encode response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		l.ErrorContext(ctx, "Failed to encode response", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to encode response")
		return
	}

	l.InfoContext(ctx, "Successfully fetched restaurant details", slog.String("restaurantID", restaurantID.String()))
	span.SetStatus(codes.Ok, "Success")
}

// // GetPOIsByDistance test this
func (HandlerImpl *HandlerImpl) GetPOIsByDistance(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "GetPOIsByDistance", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/pois_by_distance"),
	))
	defer span.End()

	l := HandlerImpl.logger.With(slog.String("HandlerImpl", "GetPOIsByDistance"))
	l.DebugContext(ctx, "Fetching POIs by distance")

	// Get query parameters
	city := r.URL.Query().Get("city")
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	distanceStr := r.URL.Query().Get("distance")

	// Parse latitude
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		l.ErrorContext(ctx, "Invalid latitude", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid latitude")
		return
	}

	// Parse longitude
	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		l.ErrorContext(ctx, "Invalid longitude", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid longitude")
		return
	}

	// Parse distance
	distance, err := strconv.ParseFloat(distanceStr, 64)
	if err != nil || distance <= 0 {
		l.ErrorContext(ctx, "Invalid distance", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid distance")
		return
	}

	// Get user ID from context
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

	// Call service method
	pois, err := HandlerImpl.llmInteractionService.GetGeneralPOIByDistanceResponse(ctx, userID, city, lat, lon, distance)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch POIs", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch POIs: %s", err.Error()))
		return
	}

	// Prepare response
	response := struct {
		PointsOfInterest []types.POIDetailedInfo `json:"points_of_interest"`
	}{PointsOfInterest: pois}

	// Encode response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		l.ErrorContext(ctx, "Failed to encode response", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to encode response")
		return
	}

	l.InfoContext(ctx, "Successfully fetched POIs")
	span.SetStatus(codes.Ok, "Success")
}

func (HandlerImpl *HandlerImpl) GetNearbyRecommendations(w http.ResponseWriter, r *http.Request) {
	return
}

// RAGEnabledChatQuery handles queries using RAG for enhanced responses
// func (h *HandlerImpl) RAGEnabledChatQuery(w http.ResponseWriter, r *http.Request) {
// 	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "RAGEnabledChatQuery", trace.WithAttributes(
// 		semconv.HTTPRequestMethodKey.String(r.Method),
// 		semconv.HTTPRouteKey.String("/prompt-response/rag/query"),
// 	))
// 	defer span.End()

// 	l := h.logger.With(slog.String("handler", "RAGEnabledChatQuery"))

// 	// Parse request body
// 	var req struct {
// 		Query       string `json:"query"`
// 		CityContext string `json:"city_context,omitempty"`
// 		SessionID   string `json:"session_id,omitempty"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		l.ErrorContext(ctx, "Failed to decode request body", slog.Any("error", err))
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
// 		return
// 	}

// 	// Validate query
// 	if strings.TrimSpace(req.Query) == "" {
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Query cannot be empty")
// 		return
// 	}

// 	// Get user ID from context
// 	userIDStr, ok := auth.GetUserIDFromContext(ctx)
// 	if !ok || userIDStr == "" {
// 		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
// 		return
// 	}
// 	userID, err := uuid.Parse(userIDStr)
// 	if err != nil {
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
// 		return
// 	}

// 	// Parse profile ID from URL
// 	profileIDStr := chi.URLParam(r, "profileID")
// 	profileID, err := uuid.Parse(profileIDStr)
// 	if err != nil {
// 		l.ErrorContext(ctx, "Invalid profile ID format", slog.Any("error", err))
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format")
// 		return
// 	}

// 	// Parse session ID if provided
// 	var sessionID uuid.UUID
// 	if req.SessionID != "" {
// 		sessionID, err = uuid.Parse(req.SessionID)
// 		if err != nil {
// 			l.WarnContext(ctx, "Invalid session ID format, proceeding without session", slog.String("session_id", req.SessionID))
// 			sessionID = uuid.Nil
// 		}
// 	} else {
// 		sessionID = uuid.New() // Generate new session if not provided
// 	}

// 	span.SetAttributes(
// 		attribute.String("user.id", userID.String()),
// 		attribute.String("profile.id", profileID.String()),
// 		attribute.String("query", req.Query),
// 		attribute.String("session.id", sessionID.String()),
// 	)

// 	// Generate RAG response
// 	ragResponse, err := h.llmInteractionService.GetRAGEnabledChatResponse(ctx, req.Query, userID, profileID, sessionID, req.CityContext)
// 	if err != nil {
// 		l.ErrorContext(ctx, "Failed to generate RAG response", slog.Any("error", err))
// 		span.RecordError(err)
// 		span.SetStatus(codes.Error, "Failed to generate RAG response")
// 		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to generate response")
// 		return
// 	}

// 	// Prepare response
// 	response := struct {
// 		SessionID   uuid.UUID              `json:"session_id"`
// 		Answer      string                 `json:"answer"`
// 		SourcePOIs  []types.POIDetail      `json:"source_pois"`
// 		Confidence  float64                `json:"confidence"`
// 		Suggestions []string               `json:"suggestions"`
// 		Metadata    map[string]interface{} `json:"metadata"`
// 	}{
// 		SessionID:   sessionID,
// 		Answer:      ragResponse.Answer,
// 		SourcePOIs:  ragResponse.SourcePOIs,
// 		Confidence:  ragResponse.Confidence,
// 		Suggestions: ragResponse.Suggestions,
// 		Metadata: map[string]interface{}{
// 			"query":        req.Query,
// 			"city_context": req.CityContext,
// 			"timestamp":    time.Now().UTC(),
// 		},
// 	}

// 	span.SetAttributes(
// 		attribute.Float64("response.confidence", ragResponse.Confidence),
// 		attribute.Int("response.source_pois", len(ragResponse.SourcePOIs)),
// 		attribute.Int("response.suggestions", len(ragResponse.Suggestions)),
// 	)
// 	span.SetStatus(codes.Ok, "RAG response generated successfully")

// 	l.InfoContext(ctx, "RAG response generated successfully",
// 		slog.Float64("confidence", ragResponse.Confidence),
// 		slog.Int("source_pois", len(ragResponse.SourcePOIs)),
// 		slog.Int("suggestions", len(ragResponse.Suggestions)))

// 	api.WriteJSONResponse(w, r, http.StatusOK, response)
// }

// // SearchSimilarPOIs handles semantic search for POIs
// func (h *HandlerImpl) SearchSimilarPOIs(w http.ResponseWriter, r *http.Request) {
// 	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "SearchSimilarPOIs", trace.WithAttributes(
// 		semconv.HTTPRequestMethodKey.String(r.Method),
// 		semconv.HTTPRouteKey.String("/prompt-response/rag/search"),
// 	))
// 	defer span.End()

// 	l := h.logger.With(slog.String("handler", "SearchSimilarPOIs"))

// 	// Parse query parameters
// 	query := r.URL.Query().Get("query")
// 	if strings.TrimSpace(query) == "" {
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Query parameter 'query' is required")
// 		return
// 	}

// 	cityIDStr := r.URL.Query().Get("city_id")
// 	var cityID *uuid.UUID
// 	if cityIDStr != "" {
// 		parsed, err := uuid.Parse(cityIDStr)
// 		if err != nil {
// 			api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid city_id format")
// 			return
// 		}
// 		cityID = &parsed
// 	}

// 	limitStr := r.URL.Query().Get("limit")
// 	limit := 10 // Default limit
// 	if limitStr != "" {
// 		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 50 {
// 			limit = parsed
// 		}
// 	}

// 	span.SetAttributes(
// 		attribute.String("query", query),
// 		attribute.Int("limit", limit),
// 	)
// 	if cityID != nil {
// 		span.SetAttributes(attribute.String("city.id", cityID.String()))
// 	}

// 	// Search for similar POIs
// 	relevantPOIs, err := h.llmInteractionService.SearchRelevantPOIsForRAG(ctx, query, cityID, limit)
// 	if err != nil {
// 		l.ErrorContext(ctx, "Failed to search similar POIs", slog.Any("error", err))
// 		span.RecordError(err)
// 		span.SetStatus(codes.Error, "Failed to search similar POIs")
// 		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to search similar POIs")
// 		return
// 	}

// 	// Prepare response
// 	response := struct {
// 		Query    string                 `json:"query"`
// 		CityID   *uuid.UUID             `json:"city_id,omitempty"`
// 		POIs     []types.POIDetail      `json:"pois"`
// 		Count    int                    `json:"count"`
// 		Metadata map[string]interface{} `json:"metadata"`
// 	}{
// 		Query:  query,
// 		CityID: cityID,
// 		POIs:   relevantPOIs,
// 		Count:  len(relevantPOIs),
// 		Metadata: map[string]interface{}{
// 			"search_method": "semantic_similarity",
// 			"limit":         limit,
// 			"timestamp":     time.Now().UTC(),
// 		},
// 	}

// 	span.SetAttributes(
// 		attribute.Int("results.count", len(relevantPOIs)),
// 	)
// 	span.SetStatus(codes.Ok, "Similar POIs found successfully")

// 	l.InfoContext(ctx, "Similar POIs search completed",
// 		slog.String("query", query),
// 		slog.Int("results", len(relevantPOIs)))

// 	api.WriteJSONResponse(w, r, http.StatusOK, response)
// }

// Context-aware chat handlers

// Hotel Chat Handlers
// func (h *HandlerImpl) StartHotelChatSession(w http.ResponseWriter, r *http.Request) {
// 	h.startContextualChatSession(w, r, types.ContextHotels)
// }

// func (h *HandlerImpl) StartHotelChatSessionStream(w http.ResponseWriter, r *http.Request) {
// 	h.startContextualChatSessionStream(w, r, types.ContextHotels)
// }

// func (h *HandlerImpl) ContinueHotelChatSession(w http.ResponseWriter, r *http.Request) {
// 	h.continueContextualChatSession(w, r, types.ContextHotels)
// }

// func (h *HandlerImpl) ContinueHotelChatSessionStream(w http.ResponseWriter, r *http.Request) {
// 	h.continueContextualChatSessionStream(w, r, types.ContextHotels)
// }

// // Restaurant Chat Handlers
// func (h *HandlerImpl) StartRestaurantChatSession(w http.ResponseWriter, r *http.Request) {
// 	h.startContextualChatSession(w, r, types.ContextRestaurants)
// }

// func (h *HandlerImpl) StartRestaurantChatSessionStream(w http.ResponseWriter, r *http.Request) {
// 	h.startContextualChatSessionStream(w, r, types.ContextRestaurants)
// }

// func (h *HandlerImpl) ContinueRestaurantChatSession(w http.ResponseWriter, r *http.Request) {
// 	h.continueContextualChatSession(w, r, types.ContextRestaurants)
// }

// func (h *HandlerImpl) ContinueRestaurantChatSessionStream(w http.ResponseWriter, r *http.Request) {
// 	h.continueContextualChatSessionStream(w, r, types.ContextRestaurants)
// }

// // Itinerary Chat Handlers
// func (h *HandlerImpl) StartItineraryChatSession(w http.ResponseWriter, r *http.Request) {
// 	h.startContextualChatSession(w, r, types.ContextItineraries)
// }

// func (h *HandlerImpl) StartItineraryChatSessionStream(w http.ResponseWriter, r *http.Request) {
// 	h.startContextualChatSessionStream(w, r, types.ContextItineraries)
// }

// func (h *HandlerImpl) ContinueItineraryChatSession(w http.ResponseWriter, r *http.Request) {
// 	h.continueContextualChatSession(w, r, types.ContextItineraries)
// }

// func (h *HandlerImpl) ContinueItineraryChatSessionStream(w http.ResponseWriter, r *http.Request) {
// 	h.continueContextualChatSessionStream(w, r, types.ContextItineraries)
// }

// Helper methods for contextual chat
// func (h *HandlerImpl) startContextualChatSession(w http.ResponseWriter, r *http.Request, contextType types.ChatContextType) {
// 	ctx, span := otel.Tracer("HandlerImpl").Start(r.Context(), "StartContextualChatSession", trace.WithAttributes(
// 		semconv.HTTPRequestMethodKey.String(r.Method),
// 		attribute.String("context_type", string(contextType)),
// 	))
// 	defer span.End()

// 	var req types.StartChatRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
// 		return
// 	}

// 	// Override context type with the specific one from the endpoint
// 	req.ContextType = contextType

// 	userIDStr, ok := auth.GetUserIDFromContext(ctx)
// 	if !ok || userIDStr == "" {
// 		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
// 		return
// 	}
// 	userID, err := uuid.Parse(userIDStr)
// 	if err != nil {
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
// 		return
// 	}

// 	profileIDStr := chi.URLParam(r, "profileID")
// 	profileID, err := uuid.Parse(profileIDStr)
// 	if err != nil {
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid profile ID format")
// 		return
// 	}

// 	userLocation := &types.UserLocation{
// 		UserLat: 38.7223,
// 		UserLon: -9.1393,
// 	}

// 	prompt := req.InitialMessage
// 	if prompt == "" {
// 		prompt = getDefaultPromptForContext(req.ContextType, req.CityName)
// 	}

// 	sessionID, itinerary, err := h.llmInteractionService.StartNewSession(ctx, userID, profileID, req.CityName, prompt, userLocation, req.ContextType)
// 	if err != nil {
// 		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to start session: "+err.Error())
// 		return
// 	}

// 	response := struct {
// 		SessionID uuid.UUID             `json:"session_id"`
// 		Data      *types.AiCityResponse `json:"data"`
// 	}{SessionID: sessionID, Data: itinerary}

// 	api.WriteJSONResponse(w, r, http.StatusCreated, response)
// }

// func (h *HandlerImpl) continueContextualChatSession(w http.ResponseWriter, r *http.Request, contextType types.ChatContextType) {
// 	ctx := r.Context()
// 	sessionIDStr := chi.URLParam(r, "sessionID")
// 	sessionID, err := uuid.Parse(sessionIDStr)
// 	if err != nil {
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid session ID")
// 		return
// 	}

// 	var req types.ContinueChatRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request body")
// 		return
// 	}

// 	// Override context type with the specific one from the endpoint
// 	req.ContextType = contextType

// 	userLocation := &types.UserLocation{
// 		UserLat: 41.3851,
// 		UserLon: 2.1734,
// 	}

// 	itinerary, err := h.llmInteractionService.ContinueSessionWithContext(ctx, sessionID, req.Message, userLocation, req.ContextType)
// 	if err != nil {
// 		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to continue session: "+err.Error())
// 		return
// 	}

// 	response := struct {
// 		Data *types.AiCityResponse `json:"data"`
// 	}{Data: itinerary}
// 	api.WriteJSONResponse(w, r, http.StatusOK, response)
// }

// func (h *HandlerImpl) startContextualChatSessionStream(w http.ResponseWriter, r *http.Request, contextType types.ChatContextType) {
// 	// Set SSE headers
// 	w.Header().Set("Content-Type", "text/event-stream")
// 	w.Header().Set("Cache-Control", "no-cache")
// 	w.Header().Set("Connection", "keep-alive")
// 	w.Header().Set("X-Accel-Buffering", "no")

// 	flusher, ok := w.(http.Flusher)
// 	if !ok {
// 		api.ErrorResponse(w, r, http.StatusInternalServerError, "Streaming not supported")
// 		return
// 	}

// 	ctx := r.Context()

// 	userIDStr, ok := auth.GetUserIDFromContext(ctx)
// 	if !ok || userIDStr == "" {
// 		h.writeSSEError(w, "Authentication required")
// 		return
// 	}
// 	userID, err := uuid.Parse(userIDStr)
// 	if err != nil {
// 		h.writeSSEError(w, "Invalid user ID format")
// 		return
// 	}

// 	profileIDStr := chi.URLParam(r, "profileID")
// 	profileID, err := uuid.Parse(profileIDStr)
// 	if err != nil {
// 		h.writeSSEError(w, "Invalid profile ID format")
// 		return
// 	}

// 	var req types.StartChatRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		h.writeSSEError(w, "Invalid request body")
// 		return
// 	}

// 	// Override context type
// 	req.ContextType = contextType

// 	userLocation := &types.UserLocation{
// 		UserLat: 41.3851,
// 		UserLon: 2.1734,
// 	}

// 	prompt := req.InitialMessage
// 	if prompt == "" {
// 		prompt = getDefaultPromptForContext(req.ContextType, req.CityName)
// 	}

// 	streamResp, err := h.llmInteractionService.StartNewSessionStreamedWithContext(ctx, userID, profileID, req.CityName, prompt, userLocation, req.ContextType)
// 	if err != nil {
// 		h.writeSSEError(w, fmt.Sprintf("Failed to start session: %v", err))
// 		return
// 	}
// 	defer streamResp.Cancel()

// 	// Stream events
// 	for {
// 		select {
// 		case event, ok := <-streamResp.Stream:
// 			if !ok {
// 				return
// 			}

// 			data, err := json.Marshal(event)
// 			if err != nil {
// 				continue
// 			}

// 			fmt.Fprintf(w, "id: %s\n", event.EventID)
// 			fmt.Fprintf(w, "event: %s\n", event.Type)
// 			fmt.Fprintf(w, "data: %s\n\n", data)
// 			flusher.Flush()

// 		case <-ctx.Done():
// 			return
// 		}
// 	}
// }

// func (h *HandlerImpl) continueContextualChatSessionStream(w http.ResponseWriter, r *http.Request, contextType types.ChatContextType) {
// 	// Set headers for SSE
// 	w.Header().Set("Content-Type", "text/event-stream")
// 	w.Header().Set("Cache-Control", "no-cache")
// 	w.Header().Set("Connection", "keep-alive")
// 	w.Header().Set("X-Accel-Buffering", "no")

// 	ctx := r.Context()

// 	sessionIDStr := chi.URLParam(r, "sessionID")
// 	sessionID, err := uuid.Parse(sessionIDStr)
// 	if err != nil {
// 		http.Error(w, "Invalid session ID", http.StatusBadRequest)
// 		return
// 	}

// 	var req types.ContinueChatRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "Invalid request body", http.StatusBadRequest)
// 		return
// 	}

// 	// Override context type
// 	req.ContextType = contextType

// 	userLocation := &types.UserLocation{
// 		UserLat: 41.3851,
// 		UserLon: 2.1734,
// 	}

// 	eventCh := make(chan types.StreamEvent)
// 	defer close(eventCh)

// 	go func() {
// 		err := h.llmInteractionService.ContinueSessionStreamedWithContext(ctx, sessionID, req.Message, userLocation, req.ContextType, eventCh)
// 		if err != nil {
// 			select {
// 			case eventCh <- types.StreamEvent{
// 				Type:      string(types.TypeError),
// 				Error:     err.Error(),
// 				IsFinal:   true,
// 				EventID:   uuid.New().String(),
// 				Timestamp: time.Now(),
// 			}:
// 			case <-ctx.Done():
// 			}
// 		}
// 	}()

// 	flusher, ok := w.(http.Flusher)
// 	if !ok {
// 		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
// 		return
// 	}

// 	for {
// 		select {
// 		case event := <-eventCh:
// 			data, err := json.Marshal(event)
// 			if err != nil {
// 				continue
// 			}

// 			fmt.Fprintf(w, "event: %s\n", event.Type)
// 			fmt.Fprintf(w, "data: %s\n\n", data)
// 			flusher.Flush()

// 			if event.IsFinal {
// 				return
// 			}

// 		case <-ctx.Done():
// 			return
// 		}
// 	}
// }
