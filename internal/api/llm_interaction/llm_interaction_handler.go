package llmInteraction

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/codes"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	generativeAI "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/generative_ai"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"

	"google.golang.org/genai"
)

type LlmInteractionHandler struct {
	llmInteractionService LlmInteractiontService
	logger                *slog.Logger
}

func NewLLMHandler(llmInteractionService LlmInteractiontService, logger *slog.Logger) *LlmInteractionHandler {
	return &LlmInteractionHandler{
		llmInteractionService: llmInteractionService,
		logger:                logger,
	}
}

func RunLLM(ctx context.Context) {
	aiClient, err := generativeAI.NewAIClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	prompt := `Generate a list of points of interest in Berlin. 
				Return the response in JSON format with each POI containing 'name', 'latitude', 'longitude', and 'category'.
				Do not wrap the response in json markers.`

	config := &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}
	response, err := aiClient.GenerateResponse(ctx, prompt, config)
	if err != nil {
		log.Fatal(err)
	}
	for _, candidate := range response.Candidates {
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			log.Println("Candidate has no content or parts.")
			continue
		}

		part := candidate.Content.Parts[0]
		txt := part.Text
		fmt.Printf("Part text: [%s]\n", txt)
		if txt != "" {
			log.Printf("Extracted text: [%s]\n", txt)
			type POI struct {
				Name      string  `json:"name"`
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
				Category  string  `json:"category"`
			}
			var pois []POI

			if err := json.Unmarshal([]byte(txt), &pois); err != nil {
				log.Printf("Failed to unmarshal AI response text into POIs: %v. Text was: %s\n", err, txt)
			} else {
				fmt.Println("POIs (successfully unmarshalled):", pois)
			}
		} else {
			log.Println("Part's text was empty.")
		}
	}
}

func (handler *LlmInteractionHandler) GetPrompResponse(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("UserInterestHandler").Start(r.Context(), "GetPrompResponse", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/user/interests"),
	))
	defer span.End()

	l := handler.logger.With(slog.String("handler", "GetUserProfile"))
	l.DebugContext(ctx, "Fetching user profile")

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
	span.SetAttributes(semconv.EnduserIDKey.String(userID.String()))
	l = l.With(slog.String("userID", userID.String()))

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
	l = l.With(slog.String("profileID", profileID.String()))

	cityName := r.URL.Query().Get("city")
	if cityName == "" {
		l.WarnContext(ctx, "City name missing from query parameters")
		span.SetStatus(codes.Error, "City name missing")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Query parameter 'city' is required.")
		return
	}
	span.SetAttributes(attribute.String("app.city.name", cityName))
	l = l.With(slog.String("cityName", cityName))

	l.InfoContext(ctx, "Processing itinerary request")

	// TODO set userLocation from route later
	userLocation := &types.UserLocation{
		UserLat: 41.3851,
		UserLon: 2.1734,
	}

	// sessionIDParam := r.URL.Query().Get("session_id")

	// isNewConversation := (sessionIDParam == "")
	// var sessionIDPtr *string
	// if !isNewConversation {
	// 	sessionIDPtr = &sessionIDParam
	// }

	//
	// userRequest := UserRequest{
	// 	Interests:  []string{"art", "history"},
	// 	Tags:       []string{"family-friendly"},
	// 	Categories: []string{"restaurants"},
	// }

	itineraryResponse, err := handler.llmInteractionService.GetPromptResponse(ctx, cityName, userID, profileID, userLocation)
	responsePayload := struct {
		Data *types.AiCityResponse `json:"data"`
		//SessionID string                `json:"session_id"` // IMPORTANT: Send this back
	}{
		Data: itineraryResponse,
		//SessionID: chatSessionID,
	}

	if err != nil {
		l.ErrorContext(ctx, "Service failed to generate prompt response", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Service error")
		// Determine appropriate HTTP status code based on the error type
		// For example, if it's a validation error from the service, could be 400.
		// If it's an AI error or DB error, could be 500.
		// For now, using a generic 500.
		// You might want to check error types (e.g., errors.Is(err, types.ErrNotFound)) for more specific statuses.
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to generate itinerary: %s", err.Error()))
		return
	}

	if itineraryResponse == nil {
		l.ErrorContext(ctx, "Service returned nil itinerary response without error")
		span.SetStatus(codes.Error, "Service returned nil response")
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to generate itinerary: received empty response from service.")
		return
	}

	// 5. Send successful response
	l.InfoContext(ctx, "Successfully generated itinerary response",
		slog.String("itinerary_name", itineraryResponse.AIItineraryResponse.ItineraryName),
		slog.Int("poi_count", len(itineraryResponse.PointsOfInterest)))

	span.SetAttributes(attribute.String("app.itinerary.name", itineraryResponse.AIItineraryResponse.ItineraryName))
	span.SetStatus(codes.Ok, "Itinerary generated")
	l.InfoContext(ctx, "User preference profile created successfully")
	api.WriteJSONResponse(w, r, http.StatusCreated, responsePayload)
}

func (handler *LlmInteractionHandler) SaveItenerary(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("LlmInteractionHandler").Start(r.Context(), "SaveItenerary", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/save_itinerary"),
	))
	defer span.End()

	l := handler.logger.With(slog.String("handler", "SaveItenerary"))
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

	savedItinerary, err := handler.llmInteractionService.SaveItenerary(ctx, userID, req)
	if err != nil {
		l.ErrorContext(ctx, "Failed to save itinerary", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to save itinerary: %s", err.Error()))
		return
	}

	l.InfoContext(ctx, "Itinerary saved successfully")
	api.WriteJSONResponse(w, r, http.StatusCreated, savedItinerary)
}

func (handler *LlmInteractionHandler) RemoveItenerary(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("LlmInteractionHandler").Start(r.Context(), "RemoveItenerary", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/llm_interaction/remove_itinerary"),
	))
	defer span.End()

	l := handler.logger.With(slog.String("handler", "RemoveItenerary"))
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

	if err := handler.llmInteractionService.RemoveItenerary(ctx, userID, itineraryID); err != nil {
		l.ErrorContext(ctx, "Failed to remove itinerary", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to remove itinerary: %s", err.Error()))
		return
	}

	l.InfoContext(ctx, "Itinerary removed successfully")
	api.WriteJSONResponse(w, r, http.StatusNoContent, nil)
}
