package llmChat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genai"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/city"
	generativeAI "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/generative_ai"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/interests"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/poi"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/profiles"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/tags"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

type SimpleIntentClassifier struct{}

func (c *SimpleIntentClassifier) Classify(ctx context.Context, message string) (types.IntentType, error) {
	message = strings.ToLower(message)
	if matched, _ := regexp.MatchString(`add|include|visit`, message); matched {
		return types.IntentAddPOI, nil
	} else if matched, _ := regexp.MatchString(`remove|delete|skip`, message); matched {
		return types.IntentRemovePOI, nil
	} else if matched, _ := regexp.MatchString(`what|where|how|why|when`, message); matched {
		return types.IntentAskQuestion, nil
	}
	return types.IntentModifyItinerary, nil // Default intent
}

const (
	model              = "gemini-2.0-flash"
	defaultTemperature = 0.5
)

type ChatSession struct {
	History []genai.Chat
}

// Mutex for thread-safe access

// Ensure implementation satisfies the interface
var _ LlmInteractiontService = (*LlmInteractiontServiceImpl)(nil)

// LlmInteractiontService defines the business logic contract for user operations.
type LlmInteractiontService interface {
	GetIteneraryResponse(ctx context.Context, cityName string, userID, profileID uuid.UUID, userLocation *types.UserLocation) (*types.AiCityResponse, error)
	SaveItenerary(ctx context.Context, userID uuid.UUID, req types.BookmarkRequest) (uuid.UUID, error)
	RemoveItenerary(ctx context.Context, userID, itineraryID uuid.UUID) error
	GetPOIDetailsResponse(ctx context.Context, userID uuid.UUID, city string, lat, lon float64) (*types.POIDetailedInfo, error)
	GetGeneralPOIByDistanceResponse(ctx context.Context, userID uuid.UUID, city string, lat, lon, distance float64) ([]types.POIDetailedInfo, error)
	// hotels
	GetHotelsByPreferenceResponse(ctx context.Context, userID uuid.UUID, city string, lat, lon float64, userPreferences types.HotelUserPreferences) ([]types.HotelDetailedInfo, error)
	GetHotelsNearbyResponse(ctx context.Context, userID uuid.UUID, city string, userLocation *types.UserLocation) ([]types.HotelDetailedInfo, error)
	GetHotelByIDResponse(ctx context.Context, hotelID uuid.UUID) (*types.HotelDetailedInfo, error)

	// restaurants
	GetRestaurantsByPreferencesResponse(ctx context.Context, userID uuid.UUID, city string, lat, lon float64, preferences types.RestaurantUserPreferences) ([]types.RestaurantDetailedInfo, error)
	GetRestaurantsNearbyResponse(ctx context.Context, userID uuid.UUID, city string, userLocation types.UserLocation) ([]types.RestaurantDetailedInfo, error)
	GetRestaurantDetailsResponse(ctx context.Context, restaurantID uuid.UUID) (*types.RestaurantDetailedInfo, error)

	StartNewSession(ctx context.Context, userID, profileID uuid.UUID, cityName, message string, userLocation *types.UserLocation) (uuid.UUID, *types.AiCityResponse, error)
	ContinueSession(ctx context.Context, sessionID uuid.UUID, message string, userLocation *types.UserLocation) (*types.AiCityResponse, error)
	StartNewSessionStreamed(ctx context.Context, userID, profileID uuid.UUID, cityName, message string, userLocation *types.UserLocation) (*types.StreamingResponse, error)
	ContinueSessionStreamed(
		ctx context.Context,
		sessionID uuid.UUID,
		message string,
		userLocation *types.UserLocation, // For distance sorting context
		eventCh chan<- types.StreamEvent, // Channel to send events back
	) error

	// RAG
	GetRAGEnabledChatResponse(ctx context.Context, message string, userID, profileID uuid.UUID, sessionID uuid.UUID, cityContext string) (*generativeAI.RAGResponse, error)
	SearchRelevantPOIsForRAG(ctx context.Context, query string, cityID *uuid.UUID, limit int) ([]types.POIDetail, error)
}

type IntentClassifier interface {
	Classify(ctx context.Context, message string) (types.IntentType, error) // e.g., "start_trip", "modify_itinerary"
}

// LlmInteractiontServiceImpl provides the implementation for LlmInteractiontService.
type LlmInteractiontServiceImpl struct {
	logger             *slog.Logger
	interestRepo       interests.Repository
	searchProfileRepo  profiles.Repository
	tagsRepo           tags.Repository
	aiClient           *generativeAI.AIClient
	embeddingService   *generativeAI.EmbeddingService
	ragService         *generativeAI.RAGService
	llmInteractionRepo Repository
	cityRepo           city.Repository
	poiRepo            poi.Repository
	cache              *cache.Cache

	// events
	deadLetterCh     chan types.StreamEvent
	intentClassifier IntentClassifier
}

// NewLlmInteractiontService creates a new user service instance.
func NewLlmInteractiontService(interestRepo interests.Repository,
	searchProfileRepo profiles.Repository,
	tagsRepo tags.Repository,
	llmInteractionRepo Repository,
	cityRepo city.Repository,
	poiRepo poi.Repository,
	logger *slog.Logger) *LlmInteractiontServiceImpl {
	ctx := context.Background()
	aiClient, err := generativeAI.NewAIClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err) // Terminate if initialization fails
	}

	// Initialize embedding service
	embeddingService, err := generativeAI.NewEmbeddingService(ctx, logger)
	if err != nil {
		log.Fatalf("Failed to create embedding service: %v", err) // Terminate if initialization fails
	}

	// Initialize RAG service
	ragService, err := generativeAI.NewRAGService(ctx, logger)
	if err != nil {
		log.Fatalf("Failed to create RAG service: %v", err) // Terminate if initialization fails
	}

	cache := cache.New(24*time.Hour, 1*time.Hour) // Cache for 24 hours with cleanup every hour
	service := &LlmInteractiontServiceImpl{
		logger:             logger,
		tagsRepo:           tagsRepo,
		interestRepo:       interestRepo,
		searchProfileRepo:  searchProfileRepo,
		aiClient:           aiClient,
		embeddingService:   embeddingService,
		ragService:         ragService,
		llmInteractionRepo: llmInteractionRepo,
		cityRepo:           cityRepo,
		poiRepo:            poiRepo,
		cache:              cache,
		deadLetterCh:       make(chan types.StreamEvent, 100),
		intentClassifier:   &SimpleIntentClassifier{},
	}
	go service.processDeadLetterQueue()
	return service
}

func (l *LlmInteractiontServiceImpl) GenerateCityDataWorker(wg *sync.WaitGroup,
	ctx context.Context,
	cityName string,
	resultCh chan<- types.GenAIResponse,
	config *genai.GenerateContentConfig) {
	go func() {
		ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GenerateCityDataWorker", trace.WithAttributes(
			attribute.String("city.name", cityName),
		))
		defer span.End()
		defer wg.Done()

		prompt := getCityDescriptionPrompt(cityName)
		span.SetAttributes(attribute.Int("prompt.length", len(prompt)))

		response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to generate city data")
			resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to generate city data: %w", err)}
			return
		}

		var txt string
		for _, candidate := range response.Candidates {
			if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
				txt = candidate.Content.Parts[0].Text
				break
			}
		}
		if txt == "" {
			err := fmt.Errorf("no valid city data content from AI")
			span.RecordError(err)
			span.SetStatus(codes.Error, "Empty response from AI")
			resultCh <- types.GenAIResponse{Err: err}
			return
		}
		span.SetAttributes(attribute.Int("response.length", len(txt)))

		cleanTxt := cleanJSONResponse(txt)
		var cityDataFromAI struct {
			CityName        string  `json:"city_name"`
			StateProvince   *string `json:"state_province"` // Use pointer for nullable string
			Country         string  `json:"country"`
			CenterLatitude  float64 `json:"center_latitude"`
			CenterLongitude float64 `json:"center_longitude"`
			Description     string  `json:"description"`
			// BoundingBox     string  `json:"bounding_box,omitempty"` // If trying to get BBox string
		}
		if err := json.Unmarshal([]byte(cleanTxt), &cityDataFromAI); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to parse city data JSON")
			resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse city data JSON: %w", err)}
			return
		}

		stateProvinceValue := ""
		if cityDataFromAI.StateProvince != nil {
			stateProvinceValue = *cityDataFromAI.StateProvince
		}

		span.SetAttributes(
			attribute.String("city.name", cityDataFromAI.CityName),
			attribute.String("city.country", cityDataFromAI.Country),
			attribute.Float64("city.latitude", cityDataFromAI.CenterLatitude),
			attribute.Float64("city.longitude", cityDataFromAI.CenterLongitude),
		)
		span.SetStatus(codes.Ok, "City data generated successfully")

		resultCh <- types.GenAIResponse{
			City:            cityDataFromAI.CityName,
			Country:         cityDataFromAI.Country,
			StateProvince:   stateProvinceValue,
			CityDescription: cityDataFromAI.Description,
			Latitude:        cityDataFromAI.CenterLatitude,
			Longitude:       cityDataFromAI.CenterLongitude,
			// BoundingBoxWKT: cityDataFromAI.BoundingBox, // TODO
		}
	}()
}

func (l *LlmInteractiontServiceImpl) GenerateGeneralPOIWorker(wg *sync.WaitGroup,
	ctx context.Context,
	cityName string,
	resultCh chan<- types.GenAIResponse,
	config *genai.GenerateContentConfig) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GenerateGeneralPOIWorker", trace.WithAttributes(
		attribute.String("city.name", cityName),
	))
	defer span.End()
	defer wg.Done()

	prompt := getGeneralPOIPrompt(cityName)
	span.SetAttributes(attribute.Int("prompt.length", len(prompt)))

	startTime := time.Now()
	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	latencyMs := int(time.Since(startTime).Milliseconds())
	span.SetAttributes(attribute.Int("response.latency_ms", latencyMs))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate general POIs")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to generate general POIs: %w", err)}
		return
	}

	var txt string
	for _, candidate := range response.Candidates {
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			txt = candidate.Content.Parts[0].Text
			break
		}
	}
	if txt == "" {
		err := fmt.Errorf("no valid general POI content from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Empty response from AI")
		resultCh <- types.GenAIResponse{Err: err}
		return
	}
	span.SetAttributes(attribute.Int("response.length", len(txt)))

	cleanTxt := cleanJSONResponse(txt)
	var poiData struct {
		PointsOfInterest []types.POIDetail `json:"points_of_interest"`
	}
	if err := json.Unmarshal([]byte(cleanTxt), &poiData); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to parse general POI JSON")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse general POI JSON: %w", err)}
		return
	}

	span.SetAttributes(attribute.Int("pois.count", len(poiData.PointsOfInterest)))
	span.SetStatus(codes.Ok, "General POIs generated successfully")
	resultCh <- types.GenAIResponse{GeneralPOI: poiData.PointsOfInterest}
}

func (l *LlmInteractiontServiceImpl) GeneratePersonalisedPOIWorker(wg *sync.WaitGroup, ctx context.Context,
	cityName string, userID, profileID uuid.UUID, resultCh chan<- types.GenAIResponse,
	interestNames []string, tagsPromptPart string, userPrefs string,
	config *genai.GenerateContentConfig) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GeneratePersonalisedPOIWorker", trace.WithAttributes(
		attribute.String("city.name", cityName),
		attribute.String("user.id", userID.String()),
		attribute.String("profile.id", profileID.String()),
		attribute.Int("interests.count", len(interestNames)),
	))
	defer span.End()
	defer wg.Done()

	startTime := time.Now()

	prompt := getPersonalizedPOI(interestNames, cityName, tagsPromptPart, userPrefs)
	span.SetAttributes(attribute.Int("prompt.length", len(prompt)))

	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate personalized itinerary")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to generate personalized itinerary: %w", err)}
		return
	}

	var txt string
	for _, candidate := range response.Candidates {
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			txt = candidate.Content.Parts[0].Text
			break
		}
	}
	if txt == "" {
		err := fmt.Errorf("no valid personalized itinerary content from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Empty response from AI")
		resultCh <- types.GenAIResponse{Err: err}
		return
	}
	span.SetAttributes(attribute.Int("response.length", len(txt)))

	cleanTxt := cleanJSONResponse(txt)
	var itineraryData struct {
		ItineraryName      string            `json:"itinerary_name"`
		OverallDescription string            `json:"overall_description"`
		PointsOfInterest   []types.POIDetail `json:"points_of_interest"`
	}

	if err := json.Unmarshal([]byte(cleanTxt), &itineraryData); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to parse personalized itinerary JSON")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse personalized itinerary JSON: %w", err)}
		return
	}
	span.SetAttributes(
		attribute.String("itinerary.name", itineraryData.ItineraryName),
		attribute.Int("personalized_pois.count", len(itineraryData.PointsOfInterest)),
	)

	latencyMs := int(time.Since(startTime).Milliseconds())
	span.SetAttributes(attribute.Int("response.latency_ms", latencyMs))

	interaction := types.LlmInteraction{
		UserID:       userID,
		Prompt:       prompt,
		ResponseText: txt,
		ModelUsed:    model, // Adjust based on your AI client
		LatencyMs:    latencyMs,
		CityName:     cityName,
		// request payload
		// response payload
		// Add token counts if available from response (depends on genai API)
		// PromptTokens, CompletionTokens, TotalTokens
		// RequestPayload, ResponsePayload if you serialize the full request/response
	}
	savedInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save LLM interaction")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to save LLM interaction: %w", err)}
		return
	}
	span.SetAttributes(attribute.String("llm_interaction.id", savedInteractionID.String()))
	span.SetStatus(codes.Ok, "Personalized POIs generated successfully")

	resultCh <- types.GenAIResponse{
		ItineraryName:        itineraryData.ItineraryName,
		ItineraryDescription: itineraryData.OverallDescription,
		PersonalisedPOI:      itineraryData.PointsOfInterest,
		LlmInteractionID:     savedInteractionID,
	}
}

// GeneratePersonalisedPOIWorkerWithSemantics generates personalized POIs with semantic search enhancement
func (l *LlmInteractiontServiceImpl) GeneratePersonalisedPOIWorkerWithSemantics(wg *sync.WaitGroup, ctx context.Context,
	cityName string, userID, profileID uuid.UUID, resultCh chan<- types.GenAIResponse,
	interestNames []string, tagsPromptPart string, userPrefs string, semanticPOIs []types.POIDetail,
	config *genai.GenerateContentConfig) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GeneratePersonalisedPOIWorkerWithSemantics", trace.WithAttributes(
		attribute.String("city.name", cityName),
		attribute.String("user.id", userID.String()),
		attribute.String("profile.id", profileID.String()),
		attribute.Int("interests.count", len(interestNames)),
		attribute.Int("semantic_pois.count", len(semanticPOIs)),
	))
	defer span.End()
	defer wg.Done()

	startTime := time.Now()

	// Create enhanced prompt with semantic context
	prompt := l.getPersonalizedPOIWithSemanticContext(interestNames, cityName, tagsPromptPart, userPrefs, semanticPOIs)
	span.SetAttributes(attribute.Int("prompt.length", len(prompt)))

	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate semantic-enhanced personalized itinerary")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to generate semantic-enhanced personalized itinerary: %w", err)}
		return
	}

	var txt string
	for _, candidate := range response.Candidates {
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			txt = candidate.Content.Parts[0].Text
			break
		}
	}
	if txt == "" {
		err := fmt.Errorf("no valid semantic-enhanced personalized itinerary content from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Empty response from AI")
		resultCh <- types.GenAIResponse{Err: err}
		return
	}
	span.SetAttributes(attribute.Int("response.length", len(txt)))

	cleanTxt := cleanJSONResponse(txt)
	var itineraryData struct {
		ItineraryName      string            `json:"itinerary_name"`
		OverallDescription string            `json:"overall_description"`
		PointsOfInterest   []types.POIDetail `json:"points_of_interest"`
	}

	if err := json.Unmarshal([]byte(cleanTxt), &itineraryData); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to parse semantic-enhanced personalized itinerary JSON")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse semantic-enhanced personalized itinerary JSON: %w", err)}
		return
	}
	span.SetAttributes(
		attribute.String("itinerary.name", itineraryData.ItineraryName),
		attribute.Int("personalized_pois.count", len(itineraryData.PointsOfInterest)),
	)

	latencyMs := int(time.Since(startTime).Milliseconds())
	span.SetAttributes(attribute.Int("response.latency_ms", latencyMs))

	interaction := types.LlmInteraction{
		UserID:       userID,
		Prompt:       prompt,
		ResponseText: txt,
		ModelUsed:    model,
		LatencyMs:    latencyMs,
		CityName:     cityName,
	}
	savedInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save semantic-enhanced LLM interaction")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to save semantic-enhanced LLM interaction: %w", err)}
		return
	}
	span.SetAttributes(attribute.String("llm_interaction.id", savedInteractionID.String()))
	span.SetStatus(codes.Ok, "Semantic-enhanced personalized POIs generated successfully")

	resultCh <- types.GenAIResponse{
		ItineraryName:        itineraryData.ItineraryName,
		ItineraryDescription: itineraryData.OverallDescription,
		PersonalisedPOI:      itineraryData.PointsOfInterest,
		LlmInteractionID:     savedInteractionID,
	}
}

// getPersonalizedPOIWithSemanticContext creates an enhanced prompt with semantic POI context
func (l *LlmInteractiontServiceImpl) getPersonalizedPOIWithSemanticContext(interestNames []string, cityName, tagsPromptPart, userPrefs string, semanticPOIs []types.POIDetail) string {
	prompt := fmt.Sprintf(`
        Generate a personalized trip itinerary for %s, tailored to user interests [%s]. 

        **SEMANTIC CONTEXT - Consider these highly relevant POIs found via semantic search:**
        `, cityName, strings.Join(interestNames, ", "))

	// Add semantic POI context
	if len(semanticPOIs) > 0 {
		prompt += "\n**Contextually Relevant POIs:**\n"
		for i, poi := range semanticPOIs {
			if i >= 10 { // Limit context to avoid token overuse
				break
			}
			prompt += fmt.Sprintf("- %s (%s): %s [Lat: %.6f, Lon: %.6f]\n",
				poi.Name, poi.Category, poi.DescriptionPOI, poi.Latitude, poi.Longitude)
		}
		prompt += "\n**Instructions:** Use these semantic matches as inspiration and context. You may include them directly or use them to find similar places. Ensure variety and avoid exact duplicates.\n\n"
	}

	prompt += `Include:
        1. An itinerary name that reflects both user interests and semantic context.
        2. An overall description highlighting semantic relevance.
        3. A list of points of interest with name, category, coordinates, and detailed description.
        Max points of interest allowed by tokens. 

        **PRIORITIZATION:**
        - Highly weight POIs that align with the semantic context provided
        - Ensure semantic relevance in descriptions
        - Balance popular attractions with personalized semantic matches
        - Include variety across different categories while maintaining semantic coherence

        Format the response in JSON with the following structure:
        {
            "itinerary_name": "Name of the itinerary (reflecting semantic context)",
            "overall_description": "Description emphasizing semantic relevance to user interests",
            "points_of_interest": [
                {
                    "name": "POI name",
                    "category": "Category",
                    "coordinates": {
                        "latitude": float64,
                        "longitude": float64
                    },
                    "description": "Detailed description explaining semantic relevance to user interests and why this matches their preferences"
                }
            ]
        }`

	if tagsPromptPart != "" {
		prompt += "\n**User Tags Context:** " + tagsPromptPart
	}
	if userPrefs != "" {
		prompt += "\n**User Preferences:** " + userPrefs
	}

	return prompt
}

func (l *LlmInteractiontServiceImpl) FetchUserData(ctx context.Context, userID, profileID uuid.UUID) (interests []*types.Interest, searchProfile *types.UserPreferenceProfileResponse, tags []*types.Tags, err error) {
	interests, err = l.interestRepo.GetInterestsForProfile(ctx, profileID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch user interests: %w", err)
	}
	searchProfile, err = l.searchProfileRepo.GetSearchProfile(ctx, userID, profileID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch search profile: %w", err)
	}
	tags, err = l.tagsRepo.GetTagsForProfile(ctx, profileID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch user tags: %w", err)
	}
	return interests, searchProfile, tags, nil
}

func (l *LlmInteractiontServiceImpl) PreparePromptData(interests []*types.Interest, tags []*types.Tags, searchProfile *types.UserPreferenceProfileResponse) (interestNames []string, tagsPromptPart string, userPrefs string) {
	if len(interests) == 0 {
		interestNames = []string{"general sightseeing", "local experiences"}
	} else {
		for _, interest := range interests {
			if interest != nil {
				interestNames = append(interestNames, interest.Name)
			}
		}
	}
	var tagInfoForPrompt []string
	for _, tag := range tags {
		if tag != nil {
			tagDetail := tag.Name
			if tag.Description != nil && *tag.Description != "" {
				tagDetail += fmt.Sprintf(" (meaning: %s)", *tag.Description)
			}
			tagInfoForPrompt = append(tagInfoForPrompt, tagDetail)
		}
	}
	if len(tagInfoForPrompt) > 0 {
		tagsPromptPart = fmt.Sprintf("\n    - Additionally, consider these specific user tags/preferences: [%s].", strings.Join(tagInfoForPrompt, "; "))
	}
	userPrefs = getUserPreferencesPrompt(searchProfile)
	return interestNames, tagsPromptPart, userPrefs
}

// RAG-enhanced methods for improved responses using semantic search

// SearchRelevantPOIsForRAG searches for POIs semantically similar to the user's query
func (l *LlmInteractiontServiceImpl) SearchRelevantPOIsForRAG(ctx context.Context, query string, cityID *uuid.UUID, limit int) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "SearchRelevantPOIsForRAG", trace.WithAttributes(
		attribute.String("query", query),
		attribute.Int("limit", limit),
	))
	defer span.End()

	// Generate embedding for the query
	queryEmbedding, err := l.embeddingService.GenerateQueryEmbedding(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate query embedding")
		l.logger.ErrorContext(ctx, "Failed to generate query embedding", slog.Any("error", err))
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search for similar POIs
	var relevantPOIs []types.POIDetail
	if cityID != nil {
		// City-specific search
		relevantPOIs, err = l.poiRepo.FindSimilarPOIsByCity(ctx, queryEmbedding, *cityID, limit)
	} else {
		// Global search
		relevantPOIs, err = l.poiRepo.FindSimilarPOIs(ctx, queryEmbedding, limit)
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to search similar POIs")
		l.logger.ErrorContext(ctx, "Failed to search similar POIs", slog.Any("error", err))
		return nil, fmt.Errorf("failed to search similar POIs: %w", err)
	}

	span.SetAttributes(
		attribute.Int("relevant_pois.count", len(relevantPOIs)),
		attribute.Int("embedding.dimension", len(queryEmbedding)),
	)
	span.SetStatus(codes.Ok, "Relevant POIs found for RAG")

	l.logger.InfoContext(ctx, "Found relevant POIs for RAG",
		slog.Int("count", len(relevantPOIs)),
		slog.String("query", query))

	return relevantPOIs, nil
}

// GenerateRAGResponse generates a response using retrieved POI context
func (l *LlmInteractiontServiceImpl) GenerateRAGResponse(ctx context.Context, query string, userID, profileID uuid.UUID, cityContext string, conversationHistory []generativeAI.ConversationTurn) (*generativeAI.RAGResponse, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GenerateRAGResponse", trace.WithAttributes(
		attribute.String("query", query),
		attribute.String("user.id", userID.String()),
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	// Fetch user data for context
	interests, searchProfile, tags, err := l.FetchUserData(ctx, userID, profileID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user data")
		return nil, fmt.Errorf("failed to fetch user data: %w", err)
	}

	// Prepare user preferences context
	userPreferences := make(map[string]interface{})
	if searchProfile != nil {
		userPreferences["search_radius"] = searchProfile.SearchRadiusKm
		//userPreferences["travel_pace"] = searchProfile.TravelPace
		userPreferences["preferred_transport"] = searchProfile.PreferredTransport
		userPreferences["budget_level"] = searchProfile.BudgetLevel
		//userPreferences["group_size"] = searchProfile.GroupSize
		//userPreferences["accessibility_needs"] = searchProfile.AccessibilityNeeds
		//userPreferences["privacy_level"] = searchProfile.PrivacyLevel
		//userPreferences["preferred_atmosphere"] = searchProfile.PreferredAtmosphere
	}

	// Add interests to preferences
	interestNames := make([]string, 0, len(interests))
	for _, interest := range interests {
		if interest != nil {
			interestNames = append(interestNames, interest.Name)
		}
	}
	userPreferences["interests"] = interestNames

	// Add tags to preferences
	tagNames := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag != nil {
			tagNames = append(tagNames, tag.Name)
		}
	}
	userPreferences["tags"] = tagNames

	// Search for relevant POIs (limit to 5 for context)
	relevantPOIs, err := l.SearchRelevantPOIsForRAG(ctx, query, nil, 5)
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to search relevant POIs, continuing without semantic context", slog.Any("error", err))
		relevantPOIs = []types.POIDetail{} // Continue with empty context
	}

	// Build RAG context
	ragContext := generativeAI.RAGContext{
		Query:               query,
		RelevantPOIs:        relevantPOIs,
		UserPreferences:     userPreferences,
		CityContext:         cityContext,
		ConversationHistory: conversationHistory,
	}

	// Generate RAG response
	ragResponse, err := l.ragService.GenerateRAGResponse(ctx, ragContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate RAG response")
		l.logger.ErrorContext(ctx, "Failed to generate RAG response", slog.Any("error", err))
		return nil, fmt.Errorf("failed to generate RAG response: %w", err)
	}

	span.SetAttributes(
		attribute.Float64("response.confidence", ragResponse.Confidence),
		attribute.Int("response.suggestions.count", len(ragResponse.Suggestions)),
		attribute.Int("source_pois.count", len(ragResponse.SourcePOIs)),
	)
	span.SetStatus(codes.Ok, "RAG response generated successfully")

	l.logger.InfoContext(ctx, "RAG response generated",
		slog.Float64("confidence", ragResponse.Confidence),
		slog.Int("source_pois", len(ragResponse.SourcePOIs)),
		slog.Int("suggestions", len(ragResponse.Suggestions)))

	return ragResponse, nil
}

// EnhancePersonalizedPOIWithRAG enhances personalized POI generation with semantic context
func (l *LlmInteractiontServiceImpl) EnhancePersonalizedPOIWithRAG(ctx context.Context, cityName string, userID, profileID uuid.UUID, cityID *uuid.UUID) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "EnhancePersonalizedPOIWithRAG", trace.WithAttributes(
		attribute.String("city.name", cityName),
		attribute.String("user.id", userID.String()),
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	// Fetch user data
	interests, searchProfile, tags, err := l.FetchUserData(ctx, userID, profileID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user data")
		return nil, fmt.Errorf("failed to fetch user data: %w", err)
	}

	// Build search query from user interests and preferences
	interestNames, tagsPromptPart, userPrefs := l.PreparePromptData(interests, tags, searchProfile)

	// Create a search query combining interests and city
	searchQuery := fmt.Sprintf("%s in %s %s %s",
		strings.Join(interestNames, " "),
		cityName,
		tagsPromptPart,
		userPrefs)

	// Search for semantically similar POIs
	var relevantPOIs []types.POIDetail
	if cityID != nil {
		relevantPOIs, err = l.SearchRelevantPOIsForRAG(ctx, searchQuery, cityID, 10)
	} else {
		relevantPOIs, err = l.SearchRelevantPOIsForRAG(ctx, searchQuery, nil, 10)
	}

	if err != nil {
		l.logger.WarnContext(ctx, "Failed to search semantically similar POIs", slog.Any("error", err))
		// Fall back to regular POI generation if semantic search fails
		return nil, err
	}

	span.SetAttributes(
		attribute.Int("semantic_pois.count", len(relevantPOIs)),
		attribute.String("search_query", searchQuery[:min(100, len(searchQuery))]),
	)
	span.SetStatus(codes.Ok, "Enhanced personalized POIs with RAG")

	l.logger.InfoContext(ctx, "Enhanced personalized POIs with semantic search",
		slog.Int("semantic_pois", len(relevantPOIs)),
		slog.String("city", cityName))

	return relevantPOIs, nil
}

// GetRAGEnabledChatResponse generates a chat response using RAG for better context
func (l *LlmInteractiontServiceImpl) GetRAGEnabledChatResponse(ctx context.Context, message string, userID, profileID uuid.UUID, sessionID uuid.UUID, cityContext string) (*generativeAI.RAGResponse, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetRAGEnabledChatResponse", trace.WithAttributes(
		attribute.String("message", message[:min(100, len(message))]),
		attribute.String("user.id", userID.String()),
		attribute.String("session.id", sessionID.String()),
	))
	defer span.End()

	// TODO: Retrieve conversation history from session storage
	// For now, we'll use an empty history
	conversationHistory := []generativeAI.ConversationTurn{}

	// Store the current user message
	err := l.ragService.StoreConversationTurn(ctx, userID.String(), "user", message, map[string]interface{}{
		"session_id": sessionID.String(),
		"profile_id": profileID.String(),
	})
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to store conversation turn", slog.Any("error", err))
	}

	// Generate RAG response
	ragResponse, err := l.GenerateRAGResponse(ctx, message, userID, profileID, cityContext, conversationHistory)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate RAG chat response")
		return nil, fmt.Errorf("failed to generate RAG chat response: %w", err)
	}

	// Store the assistant response
	err = l.ragService.StoreConversationTurn(ctx, userID.String(), "assistant", ragResponse.Answer, map[string]interface{}{
		"session_id":  sessionID.String(),
		"profile_id":  profileID.String(),
		"confidence":  ragResponse.Confidence,
		"source_pois": len(ragResponse.SourcePOIs),
	})
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to store assistant response", slog.Any("error", err))
	}

	span.SetStatus(codes.Ok, "RAG chat response generated")
	l.logger.InfoContext(ctx, "RAG chat response generated",
		slog.Float64("confidence", ragResponse.Confidence),
		slog.Int("source_pois", len(ragResponse.SourcePOIs)))

	return ragResponse, nil
}

func (l *LlmInteractiontServiceImpl) CollectResults(resultCh <-chan types.GenAIResponse) (itinerary types.AiCityResponse, llmInteractionID uuid.UUID, rawPersonalisedPOIs []types.POIDetail, errors []error) {
	for res := range resultCh {
		if res.Err != nil {
			errors = append(errors, res.Err)
			continue
		}
		if res.City != "" {
			itinerary.GeneralCityData.City = res.City
			itinerary.GeneralCityData.Country = res.Country
			itinerary.GeneralCityData.Description = res.CityDescription
			itinerary.GeneralCityData.StateProvince = res.StateProvince
			itinerary.GeneralCityData.CenterLatitude = res.Latitude
			itinerary.GeneralCityData.CenterLongitude = res.Longitude
		}
		if res.ItineraryName != "" {
			itinerary.AIItineraryResponse.ItineraryName = res.ItineraryName
			itinerary.AIItineraryResponse.OverallDescription = res.ItineraryDescription
		}
		if len(res.GeneralPOI) > 0 {
			itinerary.PointsOfInterest = res.GeneralPOI
		}
		if len(res.PersonalisedPOI) > 0 {
			itinerary.AIItineraryResponse.PointsOfInterest = res.PersonalisedPOI
			rawPersonalisedPOIs = res.PersonalisedPOI
			llmInteractionID = res.LlmInteractionID
		}
	}
	return itinerary, llmInteractionID, rawPersonalisedPOIs, errors
}

func (l *LlmInteractiontServiceImpl) HandleCityData(ctx context.Context, cityData types.GeneralCityData) (cityID uuid.UUID, err error) {
	city, err := l.cityRepo.FindCityByNameAndCountry(ctx, cityData.City, cityData.Country)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to check city existence: %w", err)
	}
	if city == nil {
		cityDetail := types.CityDetail{
			Name:            cityData.City,
			Country:         cityData.Country,
			StateProvince:   cityData.StateProvince,
			AiSummary:       cityData.Description,
			CenterLatitude:  cityData.CenterLatitude,
			CenterLongitude: cityData.CenterLongitude,
		}
		cityID, err = l.cityRepo.SaveCity(ctx, cityDetail)
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to save city: %w", err)
		}
	} else {
		cityID = city.ID
	}
	return cityID, nil
}

func (l *LlmInteractiontServiceImpl) HandleGeneralPOIs(ctx context.Context, pois []types.POIDetail, cityID uuid.UUID) {
	for _, poi := range pois {
		existingPoi, err := l.poiRepo.FindPoiByNameAndCity(ctx, poi.Name, cityID)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to check POI existence", slog.String("poi_name", poi.Name), slog.Any("error", err))
			continue
		}
		if existingPoi == nil {
			_, err = l.poiRepo.SavePoi(ctx, poi, cityID)
			if err != nil {
				l.logger.WarnContext(ctx, "Failed to save POI", slog.String("poi_name", poi.Name), slog.Any("error", err))
			}
		}
	}
}

func (l *LlmInteractiontServiceImpl) HandlePersonalisedPOIs(ctx context.Context, pois []types.POIDetail, cityID uuid.UUID, userLocation *types.UserLocation, llmInteractionID uuid.UUID, userID, profileID uuid.UUID) ([]types.POIDetail, error) {
	if userLocation == nil || cityID == uuid.Nil || len(pois) == 0 {
		return pois, nil // No sorting possible
	}
	err := l.llmInteractionRepo.SaveLlmSuggestedPOIsBatch(ctx, pois, userID, profileID, llmInteractionID, cityID)
	if err != nil {
		return nil, fmt.Errorf("failed to save personalised POIs: %w", err)
	}
	sortedPois, err := l.llmInteractionRepo.GetLlmSuggestedPOIsByInteractionSortedByDistance(ctx, llmInteractionID, cityID, *userLocation)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to fetch sorted POIs", slog.Any("error", err))
		return pois, nil // Return unsorted POIs
	}
	return sortedPois, nil
}

func (l *LlmInteractiontServiceImpl) GetIteneraryResponse(ctx context.Context, cityName string, userID, profileID uuid.UUID, userLocation *types.UserLocation) (*types.AiCityResponse, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetIteneraryResponse", trace.WithAttributes(
		attribute.String("city.name", cityName),
		attribute.String("user.id", userID.String()),
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Starting itinerary generation", slog.String("cityName", cityName), slog.String("userID", userID.String()), slog.String("profileID", profileID.String()))

	// Fetch user data
	interests, searchProfile, tags, err := l.FetchUserData(ctx, userID, profileID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user data")
		return nil, err
	}

	// Prepare prompt data
	interestNames, tagsPromptPart, userPrefs := l.PreparePromptData(interests, tags, searchProfile)
	span.SetAttributes(
		attribute.Int("interests.count", len(interestNames)),
		attribute.Int("tags.count", len(tags)),
	)

	// Determine user location
	if searchProfile.UserLatitude != nil && searchProfile.UserLongitude != nil {
		userLocation = &types.UserLocation{
			UserLat: *searchProfile.UserLatitude,
			UserLon: *searchProfile.UserLongitude,
		}
		span.SetAttributes(
			attribute.Float64("user.latitude", *searchProfile.UserLatitude),
			attribute.Float64("user.longitude", *searchProfile.UserLongitude),
		)
	} else {
		l.logger.WarnContext(ctx, "User location not available, cannot sort personalised POIs by distance")
		span.AddEvent("User location not available")
	}

	// Set up channels and wait group for fan-in fan-out
	resultCh := make(chan types.GenAIResponse, 3)
	var wg sync.WaitGroup
	wg.Add(3)

	// Fan-out: Start workers
	go l.GenerateCityDataWorker(&wg, ctx, cityName, resultCh, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
	go l.GenerateGeneralPOIWorker(&wg, ctx, cityName, resultCh, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
	go l.GeneratePersonalisedPOIWorker(&wg, ctx, cityName, userID, profileID, resultCh, interestNames, tagsPromptPart, userPrefs, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})

	// Close channel after workers complete
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Fan-in: Collect results
	itinerary, llmInteractionID, rawPersonalisedPOIs, errors := l.CollectResults(resultCh)
	if len(errors) > 0 {
		l.logger.ErrorContext(ctx, "Errors during itinerary generation", slog.Any("errors", errors))
		for _, err := range errors {
			span.RecordError(err)
		}
		span.SetStatus(codes.Error, "Failed to generate itinerary")
		return nil, fmt.Errorf("failed to generate itinerary: %v", errors)
	}

	// Handle city data
	cityID, err := l.HandleCityData(ctx, itinerary.GeneralCityData)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to handle city data")
		return nil, err
	}
	span.SetAttributes(attribute.String("city.id", cityID.String()))

	// Handle general POIs
	l.HandleGeneralPOIs(ctx, itinerary.PointsOfInterest, cityID)
	span.SetAttributes(attribute.Int("general_pois.count", len(itinerary.PointsOfInterest)))

	// Handle personalized POIs
	sortedPois, err := l.HandlePersonalisedPOIs(ctx, rawPersonalisedPOIs, cityID, userLocation, llmInteractionID, userID, profileID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to handle personalized POIs")
		return nil, err
	}
	itinerary.AIItineraryResponse.PointsOfInterest = sortedPois
	span.SetAttributes(
		attribute.Int("personalized_pois.count", len(sortedPois)),
		attribute.String("llm_interaction.id", llmInteractionID.String()),
	)

	l.logger.InfoContext(ctx, "Final itinerary ready",
		slog.String("itinerary_name", itinerary.AIItineraryResponse.ItineraryName),
		slog.Int("final_personalised_poi_count", len(itinerary.AIItineraryResponse.PointsOfInterest)))

	span.SetStatus(codes.Ok, "Itinerary generated successfully")
	return &itinerary, nil
}

func TruncateString(str string, num int) string {
	if len(str) > num {
		return str[0:num] + "..."
	}
	return str
}

func (l *LlmInteractiontServiceImpl) SaveItenerary(ctx context.Context, userID uuid.UUID, req types.BookmarkRequest) (uuid.UUID, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "SaveItenerary", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("llm_interaction.id", req.LlmInteractionID.String()),
		attribute.String("title", req.Title),
	))
	defer span.End()

	l.logger.InfoContext(ctx, "Attempting to bookmark interaction",
		slog.String("userID", userID.String()),
		slog.String("llmInteractionID", req.LlmInteractionID.String()),
		slog.String("title", req.Title))

	// Fetch original interaction
	originalInteraction, err := l.llmInteractionRepo.GetInteractionByID(ctx, req.LlmInteractionID)
	if err != nil || originalInteraction == nil {
		l.logger.ErrorContext(ctx, "Failed to fetch original LLM interaction", slog.Any("error", err))
		span.RecordError(err)
		return uuid.Nil, fmt.Errorf("could not retrieve original interaction: %w", err)
	}

	// Prepare and save to user_saved_itineraries
	markdownContent := originalInteraction.ResponseText
	var description sql.NullString
	if req.Description != nil {
		description.String = *req.Description
		description.Valid = true
	}
	isPublic := false
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}
	newBookmark := &types.UserSavedItinerary{
		UserID:                 userID,
		SourceLlmInteractionID: req.LlmInteractionID,
		PrimaryCityID:          req.PrimaryCityID, // Assume this is added to BookmarkRequest
		Title:                  req.Title,
		Description:            description,
		MarkdownContent:        markdownContent,
		Tags:                   req.Tags,
		IsPublic:               isPublic,
	}
	savedID, err := l.llmInteractionRepo.AddChatToBookmark(ctx, newBookmark)
	if err != nil {
		span.RecordError(err)
		return uuid.Nil, err
	}

	// Fetch city ID (assuming PrimaryCityID is available; otherwise, derive from interaction)
	cityID := newBookmark.PrimaryCityID
	if cityID == uuid.Nil {
		// Fallback: Get city from LLM interaction context if possible
		l.logger.WarnContext(ctx, "PrimaryCityID not provided, deriving from interaction")
		// This requires additional logic to parse city from interaction or session
		return savedID, nil // Skip further processing if cityID cannot be determined
	}

	// Save to itineraries
	itineraryID, err := l.poiRepo.SaveItinerary(ctx, userID, cityID)
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to save to itineraries", slog.Any("error", err))
		span.RecordError(err)
		return savedID, nil // Continue even if this fails
	}

	// Fetch POIs from llm_suggested_pois
	pois, err := l.llmInteractionRepo.GetLlmSuggestedPOIsByInteractionSortedByDistance(ctx, req.LlmInteractionID, cityID, types.UserLocation{})
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to fetch suggested POIs", slog.Any("error", err))
		span.RecordError(err)
		return savedID, nil
	}

	// Add CityID to POIs (if not already present)
	for i := range pois {
		pois[i].CityID = cityID // Ensure POIDetail has CityID field
	}

	// Save to itinerary_pois
	if err := l.poiRepo.SaveItineraryPOIs(ctx, itineraryID, pois); err != nil {
		l.logger.WarnContext(ctx, "Failed to save to itinerary_pois", slog.Any("error", err))
		span.RecordError(err)
		return savedID, nil
	}

	l.logger.InfoContext(ctx, "Successfully saved itinerary",
		slog.String("savedItineraryID", savedID.String()),
		slog.String("itineraryID", itineraryID.String()))
	span.SetAttributes(attribute.String("itinerary.id", itineraryID.String()))
	span.SetStatus(codes.Ok, "Itinerary saved successfully")
	return savedID, nil
}

func (l *LlmInteractiontServiceImpl) RemoveItenerary(ctx context.Context, userID, itineraryID uuid.UUID) error {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "RemoveItenerary", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("itinerary.id", itineraryID.String()),
	))
	defer span.End()

	l.logger.InfoContext(ctx, "Attempting to remove chat from bookmark",
		slog.String("itineraryID", itineraryID.String()))

	if err := l.llmInteractionRepo.RemoveChatFromBookmark(ctx, userID, itineraryID); err != nil {
		l.logger.ErrorContext(ctx, "Failed to remove chat from bookmark", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to remove chat from bookmark")
		return fmt.Errorf("failed to remove chat from bookmark: %w", err)
	}

	l.logger.InfoContext(ctx, "Successfully removed chat from bookmark", slog.String("itineraryID", itineraryID.String()))
	span.SetStatus(codes.Ok, "Itinerary removed successfully")
	return nil
}

// getPOIdetails returns a formatted string with POI details.
func (l *LlmInteractiontServiceImpl) getPOIdetails(wg *sync.WaitGroup, ctx context.Context,
	city string, lat float64, lon float64, userID uuid.UUID,
	resultCh chan<- types.POIDetailedInfo, config *genai.GenerateContentConfig) {
	defer wg.Done()
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "getPOIdetails", trace.WithAttributes(
		attribute.String("city.name", city),
		attribute.Float64("latitude", lat),
		attribute.Float64("longitude", lon),
	))
	defer span.End()

	if city == "" || lat == 0 || lon == 0 {
		return
	}

	startTime := time.Now()

	prompt := getPOIDetailsPrompt(city, lat, lon)
	span.SetAttributes(attribute.Int("prompt.length", len(prompt)))
	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate POI details")
		resultCh <- types.POIDetailedInfo{Err: fmt.Errorf("failed to generate POI details: %w", err)}
		return
	}

	var txt string
	for _, candidate := range response.Candidates {
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			txt = candidate.Content.Parts[0].Text
			break
		}
	}
	if txt == "" {
		err := fmt.Errorf("no valid POI details content from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Empty response from AI")
		resultCh <- types.POIDetailedInfo{Err: err}
		return
	}

	span.SetAttributes(attribute.Int("response.length", len(txt)))
	cleanTxt := cleanJSONResponse(txt)
	var detailedInfo types.POIDetailedInfo
	if err := json.Unmarshal([]byte(cleanTxt), &detailedInfo); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to parse POI details JSON")
		resultCh <- types.POIDetailedInfo{Err: fmt.Errorf("failed to parse POI details JSON: %w", err)}
		return
	}
	latencyMs := int(time.Since(startTime).Milliseconds())
	span.SetAttributes(attribute.Int("response.latency_ms", latencyMs))
	span.SetStatus(codes.Ok, "POI details generated successfully")
	interaction := types.LlmInteraction{
		UserID:       userID,
		Prompt:       prompt,
		ResponseText: txt,
		ModelUsed:    model, // Adjust based on your AI client
		LatencyMs:    latencyMs,
		CityName:     city,
		// request payload
		// response payload
		// Add token counts if available from response (depends on genai API)
		// PromptTokens, CompletionTokens, TotalTokens
		// RequestPayload, ResponsePayload if you serialize the full request/response
	}

	savedInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save LLM interaction for POI details")
		resultCh <- types.POIDetailedInfo{Err: fmt.Errorf("failed to save LLM interaction for POI details: %w", err)}
		return
	}
	resultCh <- types.POIDetailedInfo{
		City:         city,
		Name:         detailedInfo.Name,
		Latitude:     detailedInfo.Latitude,
		Longitude:    detailedInfo.Longitude,
		Description:  detailedInfo.Description,
		Address:      detailedInfo.Address,
		OpeningHours: detailedInfo.OpeningHours,
		PhoneNumber:  detailedInfo.PhoneNumber,
		Website:      detailedInfo.Website,
		Rating:       detailedInfo.Rating,
		Tags:         detailedInfo.Tags,
		Images:       detailedInfo.Images,
		PriceRange:   detailedInfo.PriceRange,
		Err:          nil,
		// Include the saved interaction ID for tracking

		LlmInteractionID: savedInteractionID,
	}
	span.SetAttributes(attribute.String("llm_interaction.id", savedInteractionID.String()))
	span.SetStatus(codes.Ok, "POI details generated and saved successfully")
}

func (l *LlmInteractiontServiceImpl) GetPOIDetailsResponse(ctx context.Context, userID uuid.UUID, city string, lat, lon float64) (*types.POIDetailedInfo, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetPOIDetailsResponse", trace.WithAttributes(
		attribute.String("city.name", city),
		attribute.Float64("latitude", lat),
		attribute.Float64("longitude", lon),
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Starting POI details generation",
		slog.String("city", city), slog.Float64("latitude", lat), slog.Float64("longitude", lon), slog.String("userID", userID.String()))

	// Generate cache key
	cacheKey := generatePOICacheKey(city, lat, lon, 0.0, userID)
	span.SetAttributes(attribute.String("cache.key", cacheKey))

	// Check cache
	if cached, found := l.cache.Get(cacheKey); found {
		if poi, ok := cached.(*types.POIDetailedInfo); ok {
			l.logger.InfoContext(ctx, "Cache hit for POI details", slog.String("cache_key", cacheKey))
			span.AddEvent("Cache hit")
			span.SetStatus(codes.Ok, "POI details served from cache")
			return poi, nil
		}
	}

	// Find city ID
	cityData, err := l.cityRepo.FindCityByNameAndCountry(ctx, city, "") // Adjust country if needed
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to find city", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("failed to find city: %w", err)
	}
	if cityData == nil {
		l.logger.WarnContext(ctx, "City not found", slog.String("city", city))
		span.SetStatus(codes.Error, "City not found")
		return nil, fmt.Errorf("city %s not found", city)
	}
	cityID := cityData.ID

	// Check database
	poi, err := l.poiRepo.FindPOIDetails(ctx, cityID, lat, lon, 100.0) // 100m tolerance
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to query POI details from database", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("failed to query POI details: %w", err)
	}
	if poi != nil {
		poi.City = city
		l.cache.Set(cacheKey, poi, cache.DefaultExpiration)
		l.logger.InfoContext(ctx, "Database hit for POI details", slog.String("cache_key", cacheKey))
		span.AddEvent("Database hit")
		span.SetStatus(codes.Ok, "POI details served from database")
		return poi, nil
	}

	// Cache and database miss: fetch from Gemini API
	l.logger.DebugContext(ctx, "Cache and database miss, fetching POI details from AI", slog.String("cache_key", cacheKey))
	span.AddEvent("Cache and database miss")

	resultCh := make(chan types.POIDetailedInfo, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go l.getPOIdetails(&wg, ctx, city, lat, lon, userID, resultCh, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var poiResult *types.POIDetailedInfo
	for res := range resultCh {
		if res.Err != nil {
			l.logger.ErrorContext(ctx, "Error generating POI details", slog.Any("error", res.Err))
			span.RecordError(res.Err)
			span.SetStatus(codes.Error, "Failed to generate POI details")
			return nil, res.Err
		}
		poiResult = &res
		break
	}

	if poiResult == nil {
		l.logger.WarnContext(ctx, "No response received for POI details")
		span.SetStatus(codes.Error, "No response received")
		return nil, fmt.Errorf("no response received for POI details")
	}

	// Save to database
	_, err = l.poiRepo.SavePOIDetails(ctx, *poiResult, cityID)
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to save POI details to database", slog.Any("error", err))
		span.RecordError(err)
		// Continue despite error to avoid blocking user
	}

	// Store in cache
	l.cache.Set(cacheKey, poiResult, cache.DefaultExpiration)
	l.logger.DebugContext(ctx, "Stored POI details in cache", slog.String("cache_key", cacheKey))
	span.AddEvent("Stored in cache")

	span.SetStatus(codes.Ok, "POI details generated and cached successfully")
	return poiResult, nil
}

// var cacheHitCounter = metric.NewCounter("cache_hits", metric.WithDescription("Number of cache hits"))
// var dbHitCounter = metric.NewCounter("db_hits", metric.WithDescription("Number of database hits"))
// var aiCallCounter = metric.NewCounter("ai_calls", metric.WithDescription("Number of AI calls"))

func (l *LlmInteractiontServiceImpl) getHotelsByPreferenceDetails(wg *sync.WaitGroup, ctx context.Context,
	city string, lat float64, lon float64, userID uuid.UUID, userPreferences types.HotelUserPreferences,
	resultCh chan<- []types.HotelDetailedInfo, config *genai.GenerateContentConfig) {
	defer wg.Done()
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "getHotelsByPreferenceDetails", trace.WithAttributes(
		attribute.String("city.name", city),
		attribute.Float64("latitude", lat),
		attribute.Float64("longitude", lon),
	))
	defer span.End()

	if city == "" || lat == 0 || lon == 0 {
		resultCh <- []types.HotelDetailedInfo{{Err: fmt.Errorf("invalid input: city, lat, or lon is empty")}}
		span.SetStatus(codes.Error, "Invalid input")
		return
	}

	startTime := time.Now()

	prompt := getHotelsByPreferencesPrompt(city, lat, lon, userPreferences)
	span.SetAttributes(attribute.Int("prompt.length", len(prompt)))
	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate hotel details")
		resultCh <- []types.HotelDetailedInfo{{Err: fmt.Errorf("failed to generate hotel details: %w", err)}}
		return
	}

	var txt string
	for _, candidate := range response.Candidates {
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			txt = candidate.Content.Parts[0].Text
			break
		}
	}
	if txt == "" {
		err := fmt.Errorf("no valid hotel details content from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Empty response from AI")
		resultCh <- []types.HotelDetailedInfo{{Err: err}}
		return
	}

	span.SetAttributes(attribute.Int("response.length", len(txt)))
	cleanTxt := cleanJSONResponse(txt)
	var hotelResponse struct {
		Hotels []types.HotelDetailedInfo `json:"hotels"`
	}
	if err := json.Unmarshal([]byte(cleanTxt), &hotelResponse); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to parse hotel details JSON")
		resultCh <- []types.HotelDetailedInfo{{Err: fmt.Errorf("failed to parse hotel details JSON: %w", err)}}
		return
	}

	if len(hotelResponse.Hotels) == 0 {
		err := fmt.Errorf("no hotels returned from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "No hotels found")
		resultCh <- []types.HotelDetailedInfo{{Err: err}}
		return
	}

	latencyMs := int(time.Since(startTime).Milliseconds())
	span.SetAttributes(attribute.Int("response.latency_ms", latencyMs))

	interaction := types.LlmInteraction{
		UserID:       userID,
		Prompt:       prompt,
		ResponseText: txt,
		ModelUsed:    model, // Adjust based on AI client
		LatencyMs:    latencyMs,
		CityName:     city,
	}
	savedInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save LLM interaction")
		resultCh <- []types.HotelDetailedInfo{{Err: fmt.Errorf("failed to save LLM interaction: %w", err)}}
		return
	}

	for i := range hotelResponse.Hotels {
		hotelResponse.Hotels[i].ID = uuid.New()
		hotelResponse.Hotels[i].City = city
		hotelResponse.Hotels[i].LlmInteractionID = savedInteractionID
	}

	resultCh <- hotelResponse.Hotels
	span.SetAttributes(attribute.String("llm_interaction.id", savedInteractionID.String()))
	span.SetStatus(codes.Ok, "Hotel details generated and saved successfully")
}

func (l *LlmInteractiontServiceImpl) GetHotelsByPreferenceResponse(ctx context.Context, userID uuid.UUID, city string, lat, lon float64, userPreferences types.HotelUserPreferences) ([]types.HotelDetailedInfo, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetHotelsByPreferenceResponse", trace.WithAttributes(
		attribute.String("city.name", city),
		attribute.Float64("latitude", lat),
		attribute.Float64("longitude", lon),
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Starting hotel details generation by preferences",
		slog.String("city", city), slog.Float64("latitude", lat), slog.Float64("longitude", lon), slog.String("userID", userID.String()))

	// Generate cache key
	cacheKey := generateHotelCacheKey(city, lat, lon, userID)
	span.SetAttributes(attribute.String("cache.key", cacheKey))

	// Check cache
	if cached, found := l.cache.Get(cacheKey); found {
		if hotels, ok := cached.([]types.HotelDetailedInfo); ok {
			l.logger.InfoContext(ctx, "Cache hit for hotel details", slog.String("cache_key", cacheKey))
			span.AddEvent("Cache hit")
			span.SetStatus(codes.Ok, "Hotel details served from cache")
			return hotels, nil
		}
	}

	// Find city ID
	cityData, err := l.cityRepo.FindCityByNameAndCountry(ctx, city, "")
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to find city", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("failed to find city: %w", err)
	}
	if cityData == nil {
		l.logger.WarnContext(ctx, "City not found", slog.String("city", city))
		span.SetStatus(codes.Error, "City not found")
		return nil, fmt.Errorf("city %s not found", city)
	}
	cityID := cityData.ID

	// Check database
	hotels, err := l.poiRepo.FindHotelDetails(ctx, cityID, lat, lon, 1000.0) // 1km tolerance
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to query hotel details from database", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("failed to query hotel details: %w", err)
	}
	if len(hotels) > 0 {
		for i := range hotels {
			hotels[i].City = city
		}
		l.cache.Set(cacheKey, hotels, cache.DefaultExpiration)
		l.logger.InfoContext(ctx, "Database hit for hotel details", slog.String("cache_key", cacheKey))
		span.AddEvent("Database hit")
		span.SetStatus(codes.Ok, "Hotel details served from database")
		return hotels, nil
	}

	// Cache and database miss: fetch from AI
	l.logger.DebugContext(ctx, "Cache and database miss, fetching hotel details from AI", slog.String("cache_key", cacheKey))
	span.AddEvent("Cache and database miss")

	resultCh := make(chan []types.HotelDetailedInfo, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go l.getHotelsByPreferenceDetails(&wg, ctx, city, lat, lon, userID, userPreferences, resultCh, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var hotelResults []types.HotelDetailedInfo
	for res := range resultCh {
		if res[0].Err != nil {
			l.logger.ErrorContext(ctx, "Error generating hotel details", slog.Any("error", res[0].Err))
			span.RecordError(res[0].Err)
			span.SetStatus(codes.Error, "Failed to generate hotel details")
			return nil, res[0].Err
		}
		hotelResults = res
		break
	}

	if len(hotelResults) == 0 {
		l.logger.WarnContext(ctx, "No hotels received for hotel details")
		span.SetStatus(codes.Error, "No hotels received")
		return nil, fmt.Errorf("no hotels received for hotel details")
	}

	// Save to database
	for _, hotel := range hotelResults {
		_, err = l.poiRepo.SaveHotelDetails(ctx, hotel, cityID)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to save hotel details to database", slog.Any("error", err))
			span.RecordError(err)
			// Continue despite error
		}
	}

	// Store in cache
	l.cache.Set(cacheKey, hotelResults, cache.DefaultExpiration)
	l.logger.DebugContext(ctx, "Stored hotel details in cache", slog.String("cache_key", cacheKey))
	span.AddEvent("Stored in cache")

	span.SetStatus(codes.Ok, "Hotel details generated and cached successfully")
	return hotelResults, nil
}

func (l *LlmInteractiontServiceImpl) getHotelsNearby(wg *sync.WaitGroup, ctx context.Context,
	city string, lat float64, lon float64, userID uuid.UUID,
	resultCh chan<- []types.HotelDetailedInfo, config *genai.GenerateContentConfig) {
	defer wg.Done()
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "getHotelsNearby", trace.WithAttributes(
		attribute.String("city.name", city),
		attribute.Float64("latitude", lat),
		attribute.Float64("longitude", lon),
	))
	defer span.End()

	if city == "" || lat == 0 || lon == 0 {
		resultCh <- []types.HotelDetailedInfo{{Err: fmt.Errorf("invalid input: city, lat, or lon is empty")}}
		span.SetStatus(codes.Error, "Invalid input")
		return
	}

	startTime := time.Now()

	userLocation := types.UserLocation{
		UserLat: lat,
		UserLon: lon,
	}
	prompt := getHotelsNeabyPrompt(city, userLocation)
	span.SetAttributes(attribute.Int("prompt.length", len(prompt)))
	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate hotel details")
		resultCh <- []types.HotelDetailedInfo{{Err: fmt.Errorf("failed to generate hotel details: %w", err)}}
		return
	}

	var txt string
	for _, candidate := range response.Candidates {
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			txt = candidate.Content.Parts[0].Text
			break
		}
	}
	if txt == "" {
		err := fmt.Errorf("no valid hotel details content from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Empty response from AI")
		resultCh <- []types.HotelDetailedInfo{{Err: err}}
		return
	}

	span.SetAttributes(attribute.Int("response.length", len(txt)))
	cleanTxt := cleanJSONResponse(txt)
	var hotelResponse struct {
		Hotels []types.HotelDetailedInfo `json:"hotels"`
	}
	if err := json.Unmarshal([]byte(cleanTxt), &hotelResponse); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to parse hotel details JSON")
		resultCh <- []types.HotelDetailedInfo{{Err: fmt.Errorf("failed to parse hotel details JSON: %w", err)}}
		return
	}

	if len(hotelResponse.Hotels) == 0 {
		err := fmt.Errorf("no hotels returned from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "No hotels found")
		resultCh <- []types.HotelDetailedInfo{{Err: err}}
		return
	}

	latencyMs := int(time.Since(startTime).Milliseconds())
	span.SetAttributes(attribute.Int("response.latency_ms", latencyMs))

	interaction := types.LlmInteraction{
		UserID:       userID,
		Prompt:       prompt,
		ResponseText: txt,
		ModelUsed:    model, // Adjust based on AI client
		LatencyMs:    latencyMs,
		CityName:     city,
	}
	savedInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save LLM interaction")
		resultCh <- []types.HotelDetailedInfo{{Err: fmt.Errorf("failed to save LLM interaction: %w", err)}}
		return
	}

	for i := range hotelResponse.Hotels {
		hotelResponse.Hotels[i].ID = uuid.New()
		hotelResponse.Hotels[i].City = city
		hotelResponse.Hotels[i].LlmInteractionID = savedInteractionID
	}

	resultCh <- hotelResponse.Hotels
	span.SetAttributes(attribute.String("llm_interaction.id", savedInteractionID.String()))
	span.SetStatus(codes.Ok, "Hotel details generated and saved successfully")
}

func (l *LlmInteractiontServiceImpl) GetHotelsNearbyResponse(ctx context.Context, userID uuid.UUID, city string, userLocation *types.UserLocation) ([]types.HotelDetailedInfo, error) {
	lat := userLocation.UserLat
	lon := userLocation.UserLon
	distance := userLocation.SearchRadiusKm
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetHotelsByPreferenceResponse", trace.WithAttributes(
		attribute.String("city.name", city),
		attribute.Float64("latitude", lat),
		attribute.Float64("longitude", lon),
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Starting hotel details generation by preferences",
		slog.String("city", city), slog.Float64("latitude", lat), slog.Float64("longitude", lon), slog.String("userID", userID.String()))

	// Generate cache key
	cacheKey := generateHotelCacheKey(city, lat, lon, userID)
	span.SetAttributes(attribute.String("cache.key", cacheKey))

	// Check cache
	if cached, found := l.cache.Get(cacheKey); found {
		if hotels, ok := cached.([]types.HotelDetailedInfo); ok {
			l.logger.InfoContext(ctx, "Cache hit for hotel details", slog.String("cache_key", cacheKey))
			span.AddEvent("Cache hit")
			span.SetStatus(codes.Ok, "Hotel details served from cache")
			return hotels, nil
		}
	}

	// Find city ID
	cityData, err := l.cityRepo.FindCityByNameAndCountry(ctx, city, "")
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to find city", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("failed to find city: %w", err)
	}
	if cityData == nil {
		l.logger.WarnContext(ctx, "City not found", slog.String("city", city))
		span.SetStatus(codes.Error, "City not found")
		return nil, fmt.Errorf("city %s not found", city)
	}
	cityID := cityData.ID

	// Check database
	hotels, err := l.poiRepo.FindHotelDetails(ctx, cityID, lat, lon, distance) // 1km tolerance
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to query hotel details from database", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("failed to query hotel details: %w", err)
	}
	if len(hotels) > 0 {
		for i := range hotels {
			hotels[i].City = city
		}
		l.cache.Set(cacheKey, hotels, cache.DefaultExpiration)
		l.logger.InfoContext(ctx, "Database hit for hotel details", slog.String("cache_key", cacheKey))
		span.AddEvent("Database hit")
		span.SetStatus(codes.Ok, "Hotel details served from database")
		return hotels, nil
	}

	// Cache and database miss: fetch from AI
	l.logger.DebugContext(ctx, "Cache and database miss, fetching hotel details from AI", slog.String("cache_key", cacheKey))
	span.AddEvent("Cache and database miss")

	resultCh := make(chan []types.HotelDetailedInfo, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go l.getHotelsNearby(&wg, ctx, city, lat, lon, userID, resultCh, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var hotelResults []types.HotelDetailedInfo
	for res := range resultCh {
		if res[0].Err != nil {
			l.logger.ErrorContext(ctx, "Error generating hotel details", slog.Any("error", res[0].Err))
			span.RecordError(res[0].Err)
			span.SetStatus(codes.Error, "Failed to generate hotel details")
			return nil, res[0].Err
		}
		hotelResults = res
		break
	}

	if len(hotelResults) == 0 {
		l.logger.WarnContext(ctx, "No hotels received for hotel details")
		span.SetStatus(codes.Error, "No hotels received")
		return nil, fmt.Errorf("no hotels received for hotel details")
	}

	// Save to database
	for _, hotel := range hotelResults {
		_, err = l.poiRepo.SaveHotelDetails(ctx, hotel, cityID)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to save hotel details to database", slog.Any("error", err))
			span.RecordError(err)
			// Continue despite error
		}
	}

	// Store in cache
	l.cache.Set(cacheKey, hotelResults, cache.DefaultExpiration)
	l.logger.DebugContext(ctx, "Stored hotel details in cache", slog.String("cache_key", cacheKey))
	span.AddEvent("Stored in cache")

	span.SetStatus(codes.Ok, "Hotel details generated and cached successfully")
	return hotelResults, nil
}

func (s *LlmInteractiontServiceImpl) GetHotelByIDResponse(ctx context.Context, hotelID uuid.UUID) (*types.HotelDetailedInfo, error) {
	hotel, err := s.poiRepo.GetHotelByID(ctx, hotelID)
	if err != nil {
		s.logger.Error("failed to get hotel by ID", "error", err)
		return nil, err
	}
	return hotel, nil
}

func (l *LlmInteractiontServiceImpl) getRestaurantsByPreferences(wg *sync.WaitGroup, ctx context.Context,
	city string, lat, lon float64, userID uuid.UUID, preferences types.RestaurantUserPreferences,
	resultCh chan<- []types.RestaurantDetailedInfo, config *genai.GenerateContentConfig) {
	defer wg.Done()
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "getRestaurantsByPreferences")
	defer span.End()

	if city == "" || lat == 0 || lon == 0 {
		resultCh <- []types.RestaurantDetailedInfo{{Err: fmt.Errorf("invalid input: city, lat, or lon is empty")}}
		span.SetStatus(codes.Error, "Invalid input")
		return
	}

	startTime := time.Now()
	prompt := getRestaurantsByPreferencesPrompt(city, lat, lon, preferences)
	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	if err != nil {
		span.RecordError(err)
		resultCh <- []types.RestaurantDetailedInfo{{Err: fmt.Errorf("failed to generate restaurant details: %w", err)}}
		return
	}

	var txt string
	for _, candidate := range response.Candidates {
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			txt = candidate.Content.Parts[0].Text
			break
		}
	}
	if txt == "" {
		err := fmt.Errorf("no valid restaurant details from AI")
		span.RecordError(err)
		resultCh <- []types.RestaurantDetailedInfo{{Err: err}}
		return
	}

	cleanTxt := cleanJSONResponse(txt)
	var restaurantResponse struct {
		Restaurants []types.RestaurantDetailedInfo `json:"restaurants"`
	}
	if err := json.Unmarshal([]byte(cleanTxt), &restaurantResponse); err != nil {
		span.RecordError(err)
		resultCh <- []types.RestaurantDetailedInfo{{Err: fmt.Errorf("failed to parse restaurant JSON: %w", err)}}
		return
	}

	interaction := types.LlmInteraction{
		UserID:       userID,
		Prompt:       prompt,
		ResponseText: txt,
		ModelUsed:    "model_name", // Adjust as needed
		LatencyMs:    int(time.Since(startTime).Milliseconds()),
		CityName:     city,
	}
	savedInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		resultCh <- []types.RestaurantDetailedInfo{{Err: fmt.Errorf("failed to save LLM interaction: %w", err)}}
		return
	}

	for i := range restaurantResponse.Restaurants {
		restaurantResponse.Restaurants[i].ID = uuid.New()
		restaurantResponse.Restaurants[i].LlmInteractionID = savedInteractionID
	}
	resultCh <- restaurantResponse.Restaurants
	span.SetStatus(codes.Ok, "Restaurants generated successfully")
}

func (l *LlmInteractiontServiceImpl) GetRestaurantsByPreferencesResponse(ctx context.Context, userID uuid.UUID, city string, lat, lon float64, preferences types.RestaurantUserPreferences) ([]types.RestaurantDetailedInfo, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetRestaurantsByPreferencesResponse")
	defer span.End()

	cacheKey := generateRestaurantCacheKey(city, lat, lon, userID)
	span.SetAttributes(attribute.String("cache.key", cacheKey))

	if cached, found := l.cache.Get(cacheKey); found {
		if restaurants, ok := cached.([]types.RestaurantDetailedInfo); ok {
			span.SetStatus(codes.Ok, "Served from cache")
			return restaurants, nil
		}
	}

	cityData, err := l.cityRepo.FindCityByNameAndCountry(ctx, city, "")
	if err != nil || cityData == nil {
		span.RecordError(err)
		return nil, fmt.Errorf("city %s not found: %w", city, err)
	}
	cityID := cityData.ID

	restaurants, err := l.poiRepo.FindRestaurantDetails(ctx, cityData.ID, lat, lon, 5000.0, &preferences) // 5km tolerance
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to query restaurants: %w", err)
	}
	if len(restaurants) > 0 {
		l.cache.Set(cacheKey, restaurants, cache.DefaultExpiration)
		span.SetStatus(codes.Ok, "Served from database")
		return restaurants, nil
	}

	resultCh := make(chan []types.RestaurantDetailedInfo, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go l.getRestaurantsByPreferences(&wg, ctx, city, lat, lon, userID, preferences, resultCh, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)})
	wg.Wait()
	close(resultCh)

	restaurants = <-resultCh
	if restaurants[0].Err != nil {
		span.RecordError(restaurants[0].Err)
		return nil, restaurants[0].Err
	}

	for _, r := range restaurants {
		_, err := l.poiRepo.SaveRestaurantDetails(ctx, r, cityID)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to save restaurant details to database", slog.Any("error", err), slog.String("restaurant_name", r.Name))
			span.RecordError(err)
			// Continue despite error
		}
	}
	l.cache.Set(cacheKey, restaurants, cache.DefaultExpiration)
	span.SetStatus(codes.Ok, "Restaurants generated and cached")
	return restaurants, nil
}

func (l *LlmInteractiontServiceImpl) getRestaurantsNearby(wg *sync.WaitGroup, ctx context.Context,
	city string, lat float64, lon float64, userID uuid.UUID, resultCh chan<- []types.RestaurantDetailedInfo, config *genai.GenerateContentConfig) {
	defer wg.Done()
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "getRestaurantsNearby")
	defer span.End()

	// Validate input parameters
	if city == "" || lat == 0 || lon == 0 {
		resultCh <- []types.RestaurantDetailedInfo{{Err: fmt.Errorf("invalid input: city, lat, or lon is empty")}}
		span.SetStatus(codes.Error, "Invalid input")
		return
	}

	// Define user location with a default search radius of 5.0 km
	userLocation := types.UserLocation{UserLat: lat, UserLon: lon, SearchRadiusKm: 5.0}
	prompt := getRestaurantsNearbyPrompt(city, userLocation)
	span.SetAttributes(attribute.Int("prompt.length", len(prompt)))

	// Record the start time for latency calculation
	startTime := time.Now()

	// Generate response from the AI client
	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate restaurant details")
		resultCh <- []types.RestaurantDetailedInfo{{Err: fmt.Errorf("failed to generate restaurant details: %w", err)}}
		return
	}

	// Extract text content from the AI response
	var txt string
	for _, candidate := range response.Candidates {
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			txt = candidate.Content.Parts[0].Text
			break
		}
	}
	if txt == "" {
		err := fmt.Errorf("no valid restaurant details content from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Empty response from AI")
		resultCh <- []types.RestaurantDetailedInfo{{Err: err}}
		return
	}

	// Log response length
	span.SetAttributes(attribute.Int("response.length", len(txt)))

	// Parse the JSON response
	cleanTxt := cleanJSONResponse(txt)
	var restaurantResponse struct {
		Restaurants []types.RestaurantDetailedInfo `json:"restaurants"`
	}
	if err := json.Unmarshal([]byte(cleanTxt), &restaurantResponse); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to parse restaurant details JSON")
		resultCh <- []types.RestaurantDetailedInfo{{Err: fmt.Errorf("failed to parse restaurant details JSON: %w", err)}}
		return
	}

	// Check if any restaurants were returned
	if len(restaurantResponse.Restaurants) == 0 {
		err := fmt.Errorf("no restaurants returned from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "No restaurants found")
		resultCh <- []types.RestaurantDetailedInfo{{Err: err}}
		return
	}

	// Calculate latency
	latencyMs := int(time.Since(startTime).Milliseconds())
	span.SetAttributes(attribute.Int("response.latency_ms", latencyMs))

	// Save interaction details
	interaction := types.LlmInteraction{
		UserID:       userID,
		Prompt:       prompt,
		ResponseText: txt,
		ModelUsed:    "default-model", // Replace with actual model name from your AI client
		LatencyMs:    latencyMs,
		CityName:     city,
	}
	savedInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save LLM interaction")
		resultCh <- []types.RestaurantDetailedInfo{{Err: fmt.Errorf("failed to save LLM interaction: %w", err)}}
		return
	}

	// Assign IDs and city to each restaurant
	for i := range restaurantResponse.Restaurants {
		restaurantResponse.Restaurants[i].ID = uuid.New()
		restaurantResponse.Restaurants[i].City = city
		restaurantResponse.Restaurants[i].LlmInteractionID = savedInteractionID
	}

	// Send results to the channel
	resultCh <- restaurantResponse.Restaurants
	span.SetStatus(codes.Ok, "Restaurant details generated successfully")
}

func (l *LlmInteractiontServiceImpl) GetRestaurantsNearbyResponse(ctx context.Context, userID uuid.UUID, city string, userLocation types.UserLocation) ([]types.RestaurantDetailedInfo, error) {
	lat := userLocation.UserLat
	lon := userLocation.UserLon
	distance := userLocation.SearchRadiusKm
	if distance == 0 {
		distance = 5.0 // Default to 5km
	}

	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetRestaurantsNearbyResponse")
	defer span.End()

	l.logger.DebugContext(ctx, "Starting nearby restaurant details generation",
		slog.String("city", city), slog.Float64("latitude", lat), slog.Float64("longitude", lon), slog.String("userID", userID.String()))

	// Generate cache key
	cacheKey := fmt.Sprintf("restaurants:nearby:%s:%.6f:%.6f:%s", city, lat, lon, userID.String())
	span.SetAttributes(attribute.String("cache.key", cacheKey))

	// Check cache
	if cached, found := l.cache.Get(cacheKey); found {
		if restaurants, ok := cached.([]types.RestaurantDetailedInfo); ok {
			l.logger.InfoContext(ctx, "Cache hit for nearby restaurant details", slog.String("cache_key", cacheKey))
			span.AddEvent("Cache hit")
			span.SetStatus(codes.Ok, "Restaurant details served from cache")
			return restaurants, nil
		}
	}

	// Find city ID
	cityData, err := l.cityRepo.FindCityByNameAndCountry(ctx, city, "")
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to find city", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("failed to find city: %w", err)
	}
	if cityData == nil {
		l.logger.WarnContext(ctx, "City not found", slog.String("city", city))
		span.SetStatus(codes.Error, "City not found")
		return nil, fmt.Errorf("city %s not found", city)
	}
	cityID := cityData.ID

	// Check database
	restaurants, err := l.poiRepo.FindRestaurantDetails(ctx, cityID, lat, lon, distance*1000, nil) // Convert km to meters
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to query restaurant details from database", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("failed to query restaurant details: %w", err)
	}
	if len(restaurants) > 0 {
		for i := range restaurants {
			restaurants[i].City = city
		}
		l.cache.Set(cacheKey, restaurants, cache.DefaultExpiration)
		l.logger.InfoContext(ctx, "Database hit for nearby restaurant details", slog.String("cache_key", cacheKey))
		span.AddEvent("Database hit")
		span.SetStatus(codes.Ok, "Restaurant details served from database")
		return restaurants, nil
	}

	// Cache and database miss: fetch from AI
	l.logger.DebugContext(ctx, "Cache and database miss, fetching nearby restaurant details from AI", slog.String("cache_key", cacheKey))
	span.AddEvent("Cache and database miss")

	resultCh := make(chan []types.RestaurantDetailedInfo, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go l.getRestaurantsNearby(&wg, ctx, city, lat, lon, userID, resultCh, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)})

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var restaurantResults []types.RestaurantDetailedInfo
	for res := range resultCh {
		if res[0].Err != nil {
			l.logger.ErrorContext(ctx, "Error generating nearby restaurant details", slog.Any("error", res[0].Err))
			span.RecordError(res[0].Err)
			span.SetStatus(codes.Error, "Failed to generate restaurant details")
			return nil, res[0].Err
		}
		restaurantResults = res
		break
	}

	if len(restaurantResults) == 0 {
		l.logger.WarnContext(ctx, "No restaurants received for nearby details")
		span.SetStatus(codes.Error, "No restaurants received")
		return nil, fmt.Errorf("no restaurants received for nearby details")
	}

	// Save to database
	for _, restaurant := range restaurantResults {
		_, err = l.poiRepo.SaveRestaurantDetails(ctx, restaurant, cityID)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to save restaurant details to database", slog.Any("error", err))
			span.RecordError(err)
			// Continue despite error
		}
	}

	// Cache the results
	l.cache.Set(cacheKey, restaurantResults, cache.DefaultExpiration)
	l.logger.DebugContext(ctx, "Stored nearby restaurant details in cache", slog.String("cache_key", cacheKey))
	span.AddEvent("Stored in cache")

	span.SetStatus(codes.Ok, "Nearby restaurant details generated and cached successfully")
	return restaurantResults, nil
}

func (l *LlmInteractiontServiceImpl) GetRestaurantDetailsResponse(ctx context.Context, restaurantID uuid.UUID) (*types.RestaurantDetailedInfo, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetRestaurantDetailsResponse")
	defer span.End()

	restaurant, err := l.poiRepo.GetRestaurantByID(ctx, restaurantID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get restaurant: %w", err)
	}
	if restaurant != nil {
		span.SetStatus(codes.Ok, "Restaurant found")
		return restaurant, nil
	}
	// AI generation logic can be added here if needed
	return nil, fmt.Errorf("restaurant not found")
}

func (l *LlmInteractiontServiceImpl) getGeneralPOIByDistance(wg *sync.WaitGroup,
	ctx context.Context,
	userID uuid.UUID,
	cityName string,
	lat, lon, distance float64,
	resultCh chan<- types.GenAIResponse,
	config *genai.GenerateContentConfig) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GenerateGeneralPOIWorker", trace.WithAttributes(
		attribute.String("city.name", cityName),
		attribute.Float64("latitude", lat),
		attribute.Float64("longitude", lon),
		attribute.Float64("distance.km", distance),
		attribute.String("user.id", userID.String())))

	defer span.End()
	defer wg.Done()

	prompt := getGeneralPOIByDistance(cityName, lat, lon, distance)
	span.SetAttributes(attribute.Int("prompt.length", len(prompt)))

	startTime := time.Now()
	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	latencyMs := int(time.Since(startTime).Milliseconds())
	span.SetAttributes(attribute.Int("response.latency_ms", latencyMs))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate general POIs")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to generate general POIs: %w", err)}
		return
	}

	var txt string
	for _, candidate := range response.Candidates {
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			txt = candidate.Content.Parts[0].Text
			break
		}
	}
	if txt == "" {
		err := fmt.Errorf("no valid general POI content from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Empty response from AI")
		resultCh <- types.GenAIResponse{Err: err}
		return
	}
	span.SetAttributes(attribute.Int("response.length", len(txt)))

	cleanTxt := cleanJSONResponse(txt)
	var poiData struct {
		PointsOfInterest []types.POIDetail `json:"points_of_interest"`
	}
	if err := json.Unmarshal([]byte(cleanTxt), &poiData); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to parse general POI JSON")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse general POI JSON: %w", err)}
		return
	}

	span.SetAttributes(attribute.Int("pois.count", len(poiData.PointsOfInterest)))
	span.SetStatus(codes.Ok, "General POIs generated successfully")
	resultCh <- types.GenAIResponse{GeneralPOI: poiData.PointsOfInterest}
}

func (l *LlmInteractiontServiceImpl) GetGeneralPOIByDistanceResponse(ctx context.Context, userID uuid.UUID, city string, lat, lon, distance float64) ([]types.POIDetailedInfo, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetGeneralPOIByDistanceResponse")
	defer span.End()

	cacheKey := generatePOICacheKey(city, lat, lon, distance, userID)
	span.SetAttributes(attribute.String("cache.key", cacheKey))

	if cached, found := l.cache.Get(cacheKey); found {
		if pois, ok := cached.([]types.POIDetailedInfo); ok {
			span.SetStatus(codes.Ok, "Served from cache")
			return pois, nil
		}
	}

	// Fetch cityID
	cityData, err := l.cityRepo.FindCityByNameAndCountry(ctx, city, "")
	if err != nil || cityData == nil {
		span.RecordError(err)
		return nil, fmt.Errorf("city %s not found: %w", city, err)
	}
	cityID := cityData.ID

	// Query database
	userLocation := types.UserLocation{UserLat: lat, UserLon: lon, SearchRadiusKm: distance}
	pois, err := l.poiRepo.GetPOIsByCityAndDistance(ctx, cityID, userLocation)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to query POIs: %w", err)
	}
	if len(pois) > 0 {
		l.cache.Set(cacheKey, pois, cache.DefaultExpiration)
		span.SetStatus(codes.Ok, "Served from database")
		return pois, nil
	}

	// Generate via AI
	resultCh := make(chan types.GenAIResponse, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go l.getGeneralPOIByDistance(&wg, ctx, userID, city, lat, lon, distance, resultCh, &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0.7),
		MaxOutputTokens: 16384,
	})
	wg.Wait()
	close(resultCh)

	genAIResponse := <-resultCh
	if genAIResponse.Err != nil {
		span.RecordError(genAIResponse.Err)
		return nil, genAIResponse.Err
	}

	// Convert AI response to POIDetailedInfo if necessary
	var poisDetailed []types.POIDetailedInfo
	for _, p := range genAIResponse.GeneralPOI {
		poisDetailed = append(poisDetailed, types.POIDetailedInfo{
			ID:        p.ID,
			Name:      p.Name,
			Latitude:  p.Latitude,
			Longitude: p.Longitude,
			Category:  p.Category,
		})
	}

	// Save to database
	for _, poi := range poisDetailed {
		_, err := l.poiRepo.SavePOIDetails(ctx, poi, cityID)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to save POI", slog.Any("error", err), slog.String("poi_name", poi.Name))
		}
	}

	l.cache.Set(cacheKey, poisDetailed, cache.DefaultExpiration)
	span.SetStatus(codes.Ok, "POIs generated and cached")
	return poisDetailed, nil
}

// StartNewSession creates a new chat session
func (l *LlmInteractiontServiceImpl) StartNewSession(ctx context.Context, userID, profileID uuid.UUID, cityName, message string, userLocation *types.UserLocation) (uuid.UUID, *types.AiCityResponse, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "StartNewSession", trace.WithAttributes(
		attribute.String("city.name", cityName),
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Starting new chat session", slog.String("cityName", cityName), slog.String("userID", userID.String()))

	// Generate message if not provided
	if message == "" {
		message = fmt.Sprintf("Plan a trip to %s", cityName)
	}
	span.SetAttributes(attribute.String("message", message))

	// Fetch user data
	interests, searchProfile, tags, err := l.FetchUserData(ctx, userID, profileID) // ProfileID set to nil for simplicity
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user data")
		l.logger.ErrorContext(ctx, "Failed to fetch user data", slog.Any("error", err))
		return uuid.Nil, nil, fmt.Errorf("failed to fetch user data: %w", err)
	}

	// Prepare prompt data
	interestNames, tagsPromptPart, userPrefs := l.PreparePromptData(interests, tags, searchProfile)
	span.SetAttributes(
		attribute.Int("interests.count", len(interestNames)),
		attribute.Int("tags.count", len(tags)),
	)

	// Determine user location
	if searchProfile.UserLatitude != nil && searchProfile.UserLongitude != nil {

		span.SetAttributes(
			attribute.Float64("user.latitude", userLocation.UserLat),
			attribute.Float64("user.longitude", userLocation.UserLon),
		)
	} else {
		l.logger.WarnContext(ctx, "User location not available, cannot sort personalised POIs by distance")
		span.AddEvent("User location not available")
	}

	// Enhance with semantic search context - get contextually relevant POIs
	var semanticPOIs []types.POIDetail
	if len(interestNames) > 0 {
		l.logger.DebugContext(ctx, "Enhancing session with semantic POI recommendations")
		cityUUID, cityErr := l.cityRepo.GetCityIDByName(ctx, cityName)
		if cityErr == nil {
			// Generate semantic recommendations based on user message and interests
			searchQuery := fmt.Sprintf("%s %s", message, strings.Join(interestNames, " "))
			semanticPOIs, err = l.enhancePOIRecommendationsWithSemantics(ctx, searchQuery, cityUUID, interestNames, 20)
			if err != nil {
				l.logger.WarnContext(ctx, "Failed to get semantic POI recommendations", slog.Any("error", err))
				span.AddEvent("Semantic POI recommendations failed")
			} else {
				l.logger.InfoContext(ctx, "Enhanced session with semantic POI context",
					slog.Int("semantic_pois_count", len(semanticPOIs)))
				span.SetAttributes(attribute.Int("semantic_pois.count", len(semanticPOIs)))
			}
		}
	}

	// Set up channels and wait group for fan-in fan-out
	resultCh := make(chan types.GenAIResponse, 3)
	var wg sync.WaitGroup
	wg.Add(3)

	// Fan-out: Start workers
	go l.GenerateCityDataWorker(&wg, ctx, cityName, resultCh, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
	go l.GenerateGeneralPOIWorker(&wg, ctx, cityName, resultCh, &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](defaultTemperature),
		MaxOutputTokens: 16384,
	})
	go l.GeneratePersonalisedPOIWorkerWithSemantics(&wg, ctx, cityName, userID, uuid.Nil, resultCh, interestNames, tagsPromptPart, userPrefs, semanticPOIs, &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](defaultTemperature),
		MaxOutputTokens: 16384,
	})

	// Close channel after workers complete
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Fan-in: Collect results
	itinerary, llmInteractionID, rawPersonalisedPOIs, errors := l.CollectResults(resultCh)
	if len(errors) > 0 {
		l.logger.ErrorContext(ctx, "Errors during itinerary generation", slog.Any("errors", errors))
		for _, err := range errors {
			span.RecordError(err)
		}
		span.SetStatus(codes.Error, "Failed to generate itinerary")
		return uuid.Nil, nil, fmt.Errorf("failed to generate itinerary: %v", errors)
	}

	// Handle city data
	cityID, err := l.HandleCityData(ctx, itinerary.GeneralCityData)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to handle city data")
		l.logger.ErrorContext(ctx, "Failed to handle city data", slog.Any("error", err))
		return uuid.Nil, nil, err
	}
	span.SetAttributes(attribute.String("city.id", cityID.String()))

	// Handle general POIs
	l.HandleGeneralPOIs(ctx, itinerary.PointsOfInterest, cityID)
	span.SetAttributes(attribute.Int("general_pois.count", len(itinerary.PointsOfInterest)))

	// Handle personalized POIs
	sortedPois, err := l.HandlePersonalisedPOIs(ctx, rawPersonalisedPOIs, cityID, userLocation, llmInteractionID, userID, uuid.Nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to handle personalized POIs")
		l.logger.ErrorContext(ctx, "Failed to handle personalized POIs", slog.Any("error", err))
		return uuid.Nil, nil, err
	}
	itinerary.AIItineraryResponse.PointsOfInterest = sortedPois
	span.SetAttributes(
		attribute.Int("personalized_pois.count", len(sortedPois)),
		attribute.String("llm_interaction.id", llmInteractionID.String()),
	)

	// Create new session
	sessionID := uuid.New()
	session := types.ChatSession{
		ID:               sessionID,
		UserID:           userID,
		CurrentItinerary: &itinerary,
		ConversationHistory: []types.ConversationMessage{
			{Role: "user", Content: message, Timestamp: time.Now()},
			{Role: "assistant", Content: "Here’s your initial trip plan for " + cityName, Timestamp: time.Now()},
		},
		SessionContext: types.SessionContext{
			CityName:            cityName,
			ConversationSummary: fmt.Sprintf("Initial trip plan for %s", cityName),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    "active",
	}

	// Save session to database
	if err := l.llmInteractionRepo.CreateSession(ctx, session); err != nil {
		l.logger.ErrorContext(ctx, "Failed to save session", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save session")
		return uuid.Nil, nil, fmt.Errorf("failed to save session: %w", err)
	}

	l.logger.InfoContext(ctx, "New session started",
		slog.String("sessionID", sessionID.String()),
		slog.String("itinerary_name", itinerary.AIItineraryResponse.ItineraryName),
		slog.Int("personalised_poi_count", len(itinerary.AIItineraryResponse.PointsOfInterest)))
	span.SetAttributes(attribute.String("session.id", sessionID.String()))
	span.SetStatus(codes.Ok, "Session started successfully")

	return sessionID, &itinerary, nil
}

// ContinueSession handles subsequent messages in an existing session
func (l *LlmInteractiontServiceImpl) ContinueSession(ctx context.Context, sessionID uuid.UUID, message string, userLocation *types.UserLocation) (*types.AiCityResponse, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "ContinueSession", trace.WithAttributes(
		attribute.String("session.id", sessionID.String()),
		attribute.String("message", message),
	))
	defer span.End()

	// Fetch session
	session, err := l.llmInteractionRepo.GetSession(ctx, sessionID)
	if err != nil || session.Status != "active" {
		l.logger.ErrorContext(ctx, "Invalid or inactive session", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid or inactive session")
		return nil, fmt.Errorf("invalid or inactive session: %w", err)
	}

	// Fetch city ID
	city, err := l.cityRepo.FindCityByNameAndCountry(ctx, session.SessionContext.CityName, "")
	if err != nil || city == nil {
		l.logger.ErrorContext(ctx, "Failed to find city", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("city %s not found: %w", session.SessionContext.CityName, err)
	}
	cityID := city.ID

	// Add user message
	userMessage := types.ConversationMessage{
		ID:          uuid.New(),
		Role:        "user",
		Content:     message,
		Timestamp:   time.Now(),
		MessageType: types.TypeModificationRequest,
	}
	if err := l.llmInteractionRepo.AddMessageToSession(ctx, sessionID, userMessage); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to add message to session: %w", err)
	}

	intent, err := l.intentClassifier.Classify(ctx, message)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to classify intent", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to classify intent")
		return nil, fmt.Errorf("failed to classify intent: %w", err)
	}

	// Enhance with semantic POI recommendations for all intents
	semanticPOIs, err := l.generateSemanticPOIRecommendations(ctx, message, cityID, session.UserID, userLocation, 0.6)
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to generate semantic POI recommendations", slog.Any("error", err))
		span.AddEvent("Semantic POI recommendations failed")
	} else {
		l.logger.InfoContext(ctx, "Generated semantic POI recommendations for session",
			slog.Int("semantic_recommendations", len(semanticPOIs)))
		span.SetAttributes(attribute.Int("semantic_recommendations.count", len(semanticPOIs)))
	}

	var responseText string
	switch intent {
	case "add_poi":
		poiName := extractPOIName(message)
		for _, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
			if strings.EqualFold(poi.Name, poiName) {
				responseText = fmt.Sprintf("%s is already in your itinerary.", poiName)
				break
			}
		}
		if responseText == "" {
			newPOI, err := l.generatePOIData(ctx, poiName, session.SessionContext.CityName, userLocation, session.UserID, cityID)
			if err != nil {
				l.logger.ErrorContext(ctx, "Failed to generate POI data", slog.Any("error", err))
				span.RecordError(err)
				responseText = fmt.Sprintf("Could not add %s due to an error.", poiName)
			} else {
				session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = append(
					session.CurrentItinerary.AIItineraryResponse.PointsOfInterest, newPOI)
				responseText = fmt.Sprintf("I’ve added %s to your itinerary.", poiName)
			}
		}
	case "remove_poi":
		poiName := extractPOIName(message)
		for i, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
			if strings.Contains(strings.ToLower(poi.Name), strings.ToLower(poiName)) {
				session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = append(
					session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[:i],
					session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[i+1:]...,
				)
				responseText = fmt.Sprintf("I’ve removed %s from your itinerary.", poiName)
				break
			}
		}
		if responseText == "" {
			responseText = fmt.Sprintf("Could not find %s in your itinerary.", poiName)
		}
	case "ask_question":
		responseText = "I’m here to help! For now, I’ll assume you’re asking about your trip. What specifically would you like to know?"
	default: // modify_itinerary
		if matches := regexp.MustCompile(`replace\s+(.+?)\s+with\s+(.+?)(?:\s+in\s+my\s+itinerary)?`).FindStringSubmatch(strings.ToLower(message)); len(matches) == 3 {
			oldPOI := matches[1]
			newPOIName := matches[2]
			for i, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
				if strings.Contains(strings.ToLower(poi.Name), oldPOI) {
					newPOI, err := l.generatePOIData(ctx, newPOIName, session.SessionContext.CityName, userLocation, session.UserID, cityID)
					if err != nil {
						l.logger.ErrorContext(ctx, "Failed to generate POI data", slog.Any("error", err))
						span.RecordError(err)
						responseText = fmt.Sprintf("Could not replace %s with %s due to an error.", oldPOI, newPOIName)
					} else {
						session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[i] = newPOI
						responseText = fmt.Sprintf("I’ve replaced %s with %s in your itinerary.", oldPOI, newPOIName)
					}
					break
				}
			}
			if responseText == "" {
				responseText = fmt.Sprintf("Could not find %s in your itinerary.", oldPOI)
			}
		} else {
			responseText = "I’ve noted your request to modify the itinerary. Please specify the changes (e.g., 'replace X with Y')."
		}
	}

	// Sort POIs by distance if userLocation is provided
	if (intent == "add_poi" || intent == "modify_itinerary") && userLocation != nil && userLocation.UserLat != 0 && userLocation.UserLon != 0 {
		sortedPOIs, err := l.llmInteractionRepo.GetPOIsBySessionSortedByDistance(ctx, sessionID, cityID, *userLocation)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to sort POIs by distance", slog.Any("error", err))
			span.RecordError(err)
		} else {
			session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = sortedPOIs
			l.logger.InfoContext(ctx, "POIs sorted by distance",
				slog.Int("poi_count", len(sortedPOIs)))
			span.SetAttributes(attribute.Int("sorted_pois.count", len(sortedPOIs)))
		}
	}

	// Add assistant response
	assistantMessage := types.ConversationMessage{
		ID:          uuid.New(),
		Role:        "assistant",
		Content:     responseText,
		Timestamp:   time.Now(),
		MessageType: types.TypeModificationRequest,
	}
	if err := l.llmInteractionRepo.AddMessageToSession(ctx, sessionID, assistantMessage); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to add assistant message: %w", err)
	}

	// Update session
	session.UpdatedAt = time.Now()
	session.ExpiresAt = time.Now().Add(24 * time.Hour)
	if err := l.llmInteractionRepo.UpdateSession(ctx, *session); err != nil {
		l.logger.ErrorContext(ctx, "Failed to update session", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	l.logger.InfoContext(ctx, "Session continued",
		slog.String("sessionID", sessionID.String()),
		slog.String("intent", string(intent)))

	span.SetStatus(codes.Ok, "Session continued successfully")
	return session.CurrentItinerary, nil
}

// generatePOIData queries the LLM for POI details and calculates distance using PostGIS
func (l *LlmInteractiontServiceImpl) generatePOIData(ctx context.Context, poiName, cityName string, userLocation *types.UserLocation, userID, cityID uuid.UUID) (types.POIDetail, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GeneratePOIData", trace.WithAttributes(
		attribute.String("poi.name", poiName),
		attribute.String("city.name", cityName),
	))
	defer span.End()

	// Create a prompt for the LLM
	prompt := generatedContinuedConversationPrompt(poiName, cityName)

	// Generate LLM response
	response, err := l.aiClient.GenerateContent(ctx, prompt, &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.7),
	})
	if err != nil {
		span.RecordError(err)
		return types.POIDetail{}, fmt.Errorf("failed to generate POI data: %w", err)
	}

	interaction := types.LlmInteraction{
		UserID:       userID,
		Prompt:       prompt,
		ResponseText: response,
		ModelUsed:    model,
		CityName:     cityName,
	}
	savedLlmInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to save LLM interaction in generatePOIData", slog.Any("error", err))
		// Decide if this is fatal for POI generation. It might be if FK is NOT NULL.
		return types.POIDetail{}, fmt.Errorf("failed to save LLM interaction: %w", err)
	}
	span.SetAttributes(attribute.String("llm.interaction_id.for_poi_data", savedLlmInteractionID.String()))

	cleanResponse := cleanJSONResponse(response)
	var poiData types.POIDetail
	if err := json.Unmarshal([]byte(cleanResponse), &poiData); err != nil || poiData.Name == "" {
		l.logger.WarnContext(ctx, "LLM returned invalid or empty POI data",
			slog.String("poiName", poiName),
			slog.String("llmResponse", response),
			slog.Any("unmarshalError", err))
		span.AddEvent("Invalid LLM response")
		poiData = types.POIDetail{
			ID:             uuid.New(),
			Name:           poiName,
			Latitude:       0,
			Longitude:      0,
			Category:       "Attraction",
			DescriptionPOI: fmt.Sprintf("Added %s based on user request, but detailed data not available.", poiName),
			Distance:       0,
		}
	}
	if poiData.ID == uuid.Nil { // Assign an ID if LLM didn't provide one
		poiData.ID = uuid.New()
	}
	poiData.LlmInteractionID = savedLlmInteractionID

	// Calculate distance if coordinates are valid
	if userLocation != nil && userLocation.UserLat != 0 && userLocation.UserLon != 0 && poiData.Latitude != 0 && poiData.Longitude != 0 {
		distance, err := l.llmInteractionRepo.CalculateDistancePostGIS(ctx, userLocation.UserLat, userLocation.UserLon, poiData.Latitude, poiData.Longitude)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to calculate distance", slog.Any("error", err))
			span.RecordError(err)
			poiData.Distance = 0
		} else {
			poiData.Distance = distance
			span.SetAttributes(attribute.Float64("poi.distance_meters", distance))
			l.logger.DebugContext(ctx, "Calculated distance for POI",
				slog.String("poiName", poiName),
				slog.Float64("distance_meters", distance))
		}
	} else {
		poiData.Distance = 0
		span.AddEvent("Distance not calculated due to missing location data")
		l.logger.WarnContext(ctx, "Cannot calculate distance",
			slog.Bool("userLocationAvailable", userLocation != nil),
			slog.Float64("userLat", userLocation.UserLat),
			slog.Float64("userLon", userLocation.UserLon),
			slog.Float64("poiLatitude", poiData.Latitude),
			slog.Float64("poiLongitude", poiData.Longitude))
	}

	// Save POI to database
	llmInteractionID := uuid.New()
	_, err = l.llmInteractionRepo.SaveSinglePOI(ctx, poiData, userID, cityID, savedLlmInteractionID)
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to save POI to database", slog.Any("error", err))
		span.RecordError(err)
	}

	span.SetAttributes(
		attribute.String("poi.name", poiData.Name),
		attribute.Float64("poi.latitude", poiData.Latitude),
		attribute.Float64("poi.longitude", poiData.Longitude),
		attribute.String("poi.category", poiData.Category),
		attribute.String("llm_interaction.id", llmInteractionID.String()),
	)
	return poiData, nil
}

// enhancePOIRecommendationsWithSemantics uses embeddings to find similar POIs and enrich recommendations
func (l *LlmInteractiontServiceImpl) enhancePOIRecommendationsWithSemantics(ctx context.Context, userMessage string, cityID uuid.UUID, userPreferences []string, limit int) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "enhancePOIRecommendationsWithSemantics", trace.WithAttributes(
		attribute.String("user.message", userMessage),
		attribute.String("city.id", cityID.String()),
		attribute.Int("limit", limit),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Enhancing POI recommendations with semantic search",
		slog.String("message", userMessage),
		slog.String("city_id", cityID.String()))

	if l.embeddingService == nil {
		l.logger.WarnContext(ctx, "Embedding service not available, falling back to traditional search")
		span.AddEvent("Embedding service not available")
		return []types.POIDetail{}, nil
	}

	// Generate embedding for user message combined with preferences
	searchQuery := userMessage
	if len(userPreferences) > 0 {
		searchQuery += " " + strings.Join(userPreferences, " ")
	}

	queryEmbedding, err := l.embeddingService.GenerateQueryEmbedding(ctx, searchQuery)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to generate query embedding",
			slog.Any("error", err),
			slog.String("query", searchQuery))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate query embedding")
		return []types.POIDetail{}, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search for similar POIs in the city
	similarPOIs, err := l.poiRepo.FindSimilarPOIsByCity(ctx, queryEmbedding, cityID, limit)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to find similar POIs", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to find similar POIs")
		return []types.POIDetail{}, fmt.Errorf("failed to find similar POIs: %w", err)
	}

	l.logger.InfoContext(ctx, "Found semantically similar POIs",
		slog.Int("count", len(similarPOIs)),
		slog.String("city_id", cityID.String()))
	span.SetAttributes(
		attribute.Int("similar_pois.count", len(similarPOIs)),
		attribute.String("search.query", searchQuery),
	)
	span.SetStatus(codes.Ok, "Semantic POI recommendations enhanced")

	return similarPOIs, nil
}

// enhanceConversationContextWithSemantics adds semantic understanding to chat interactions
func (l *LlmInteractiontServiceImpl) enhanceConversationContextWithSemantics(ctx context.Context, sessionID uuid.UUID, userMessage string) (string, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "enhanceConversationContextWithSemantics", trace.WithAttributes(
		attribute.String("session.id", sessionID.String()),
		attribute.String("user.message", userMessage),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Enhancing conversation context with semantic understanding",
		slog.String("session_id", sessionID.String()),
		slog.String("message", userMessage))

	if l.embeddingService == nil {
		l.logger.WarnContext(ctx, "Embedding service not available for conversation enhancement")
		span.AddEvent("Embedding service not available")
		return "", nil
	}

	// Get session to understand the context
	session, err := l.llmInteractionRepo.GetSession(ctx, sessionID)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to get session for context enhancement", slog.Any("error", err))
		span.RecordError(err)
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	// Find city
	city, err := l.cityRepo.FindCityByNameAndCountry(ctx, session.SessionContext.CityName, "")
	if err != nil || city == nil {
		l.logger.ErrorContext(ctx, "Failed to find city for semantic enhancement", slog.Any("error", err))
		span.RecordError(err)
		return "", fmt.Errorf("city not found: %w", err)
	}

	// Generate embedding for the user message
	messageEmbedding, err := l.embeddingService.GenerateQueryEmbedding(ctx, userMessage)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to generate message embedding", slog.Any("error", err))
		span.RecordError(err)
		return "", fmt.Errorf("failed to generate message embedding: %w", err)
	}

	// Find contextually relevant POIs
	contextualPOIs, err := l.poiRepo.FindSimilarPOIsByCity(ctx, messageEmbedding, city.ID, 5)
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to find contextual POIs", slog.Any("error", err))
		span.RecordError(err)
		// Don't fail the conversation, just return empty context
		return "", nil
	}

	// Build enhanced context string
	var contextBuilder strings.Builder
	contextBuilder.WriteString(fmt.Sprintf("Based on your message '%s', here are some relevant places in %s:\n",
		userMessage, session.SessionContext.CityName))

	for i, poi := range contextualPOIs {
		if i >= 3 { // Limit to top 3 for context
			break
		}
		contextBuilder.WriteString(fmt.Sprintf("- %s (%s): %s\n",
			poi.Name, poi.Category, poi.DescriptionPOI))
	}

	enhancedContext := contextBuilder.String()

	l.logger.InfoContext(ctx, "Enhanced conversation context with semantic POI recommendations",
		slog.String("session_id", sessionID.String()),
		slog.Int("contextual_pois", len(contextualPOIs)))
	span.SetAttributes(
		attribute.String("enhanced.context", enhancedContext),
		attribute.Int("contextual_pois.count", len(contextualPOIs)),
	)
	span.SetStatus(codes.Ok, "Conversation context enhanced with semantics")

	return enhancedContext, nil
}

// generateSemanticPOIRecommendations generates POI recommendations using semantic search
func (l *LlmInteractiontServiceImpl) generateSemanticPOIRecommendations(ctx context.Context, userMessage string, cityID uuid.UUID, userID uuid.UUID, userLocation *types.UserLocation, semanticWeight float64) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "generateSemanticPOIRecommendations", trace.WithAttributes(
		attribute.String("user.message", userMessage),
		attribute.String("city.id", cityID.String()),
		attribute.String("user.id", userID.String()),
		attribute.Float64("semantic.weight", semanticWeight),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Generating semantic POI recommendations",
		slog.String("message", userMessage),
		slog.String("city_id", cityID.String()),
		slog.Float64("semantic_weight", semanticWeight))

	if l.embeddingService == nil {
		err := fmt.Errorf("embedding service not available")
		l.logger.ErrorContext(ctx, "Embedding service not available", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Embedding service not available")
		return nil, err
	}

	// Generate embedding for user message
	queryEmbedding, err := l.embeddingService.GenerateQueryEmbedding(ctx, userMessage)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to generate query embedding", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate query embedding")
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	var pois []types.POIDetail

	// If user location is available, use hybrid search (spatial + semantic)
	if userLocation != nil && userLocation.UserLat != 0 && userLocation.UserLon != 0 {
		filter := types.POIFilter{
			Location: types.GeoPoint{
				Latitude:  userLocation.UserLat,
				Longitude: userLocation.UserLon,
			},
			Radius: userLocation.SearchRadiusKm,
		}

		hybridPOIs, err := l.poiRepo.SearchPOIsHybrid(ctx, filter, queryEmbedding, semanticWeight)
		if err != nil {
			l.logger.ErrorContext(ctx, "Failed to perform hybrid search", slog.Any("error", err))
			span.RecordError(err)
			// Fall back to semantic-only search
		} else {
			pois = hybridPOIs
			l.logger.InfoContext(ctx, "Used hybrid search for POI recommendations",
				slog.Int("poi_count", len(pois)))
			span.AddEvent("Used hybrid search")
		}
	}

	// If hybrid search failed or no location available, use semantic-only search
	if len(pois) == 0 {
		semanticPOIs, err := l.poiRepo.FindSimilarPOIsByCity(ctx, queryEmbedding, cityID, 10)
		if err != nil {
			l.logger.ErrorContext(ctx, "Failed to find similar POIs", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to find similar POIs")
			return nil, fmt.Errorf("failed to find similar POIs: %w", err)
		}
		pois = semanticPOIs
		l.logger.InfoContext(ctx, "Used semantic-only search for POI recommendations",
			slog.Int("poi_count", len(pois)))
		span.AddEvent("Used semantic-only search")
	}

	// Generate embeddings for new POIs if needed
	for i, poi := range pois {
		if poi.ID == uuid.Nil {
			continue
		}

		// Generate embedding for this POI if it doesn't have one
		embedding, err := l.embeddingService.GeneratePOIEmbedding(ctx, poi.Name, poi.DescriptionPOI, poi.Category)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to generate embedding for POI",
				slog.Any("error", err),
				slog.String("poi_name", poi.Name))
			continue
		}

		// Update POI with embedding
		err = l.poiRepo.UpdatePOIEmbedding(ctx, poi.ID, embedding)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to update POI embedding",
				slog.Any("error", err),
				slog.String("poi_id", poi.ID.String()))
		}

		pois[i] = poi
	}

	l.logger.InfoContext(ctx, "Generated semantic POI recommendations",
		slog.String("message", userMessage),
		slog.Int("recommendations", len(pois)))
	span.SetAttributes(
		attribute.String("search.query", userMessage),
		attribute.Int("recommendations.count", len(pois)),
		attribute.Float64("semantic.weight", semanticWeight),
	)
	span.SetStatus(codes.Ok, "Semantic POI recommendations generated")

	return pois, nil
}

// handleSemanticAddPOI handles adding POIs with semantic search enhancement
func (l *LlmInteractiontServiceImpl) handleSemanticAddPOI(ctx context.Context, message string, session *types.ChatSession, semanticPOIs []types.POIDetail, userLocation *types.UserLocation, cityID uuid.UUID) (string, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "handleSemanticAddPOI")
	defer span.End()

	// Try semantic matching first - look for POIs semantically similar to the user's request
	if len(semanticPOIs) > 0 {
		// Check if any semantic POI matches what user is asking for
		for _, semanticPOI := range semanticPOIs[:min(3, len(semanticPOIs))] {
			// Check if this semantic POI is already in itinerary
			alreadyExists := false
			for _, existingPOI := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
				if strings.EqualFold(existingPOI.Name, semanticPOI.Name) {
					alreadyExists = true
					break
				}
			}

			if !alreadyExists {
				// Add semantic POI to itinerary
				session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = append(
					session.CurrentItinerary.AIItineraryResponse.PointsOfInterest, semanticPOI)
				l.logger.InfoContext(ctx, "Added semantic POI to itinerary",
					slog.String("poi_name", semanticPOI.Name))
				span.SetAttributes(attribute.String("added_poi", semanticPOI.Name))
				return fmt.Sprintf("Great! I found %s which matches what you're looking for. I've added it to your itinerary. %s",
					semanticPOI.Name, semanticPOI.DescriptionPOI), nil
			}
		}

		// If semantic POIs exist but all are already in itinerary
		return fmt.Sprintf("I found some great options matching your request, but they're already in your itinerary. Here are some suggestions: %s",
			strings.Join(func() []string {
				var names []string
				for i, poi := range semanticPOIs[:min(3, len(semanticPOIs))] {
					names = append(names, poi.Name)
					if i >= 2 {
						break
					}
				}
				return names
			}(), ", ")), nil
	}

	// Fallback to traditional POI name extraction and generation
	poiName := extractPOIName(message)
	if poiName == "" {
		return "I'd be happy to add a POI to your itinerary! Could you please specify which place you'd like to add?", nil
	}

	// Check if already exists
	for _, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
		if strings.EqualFold(poi.Name, poiName) {
			return fmt.Sprintf("%s is already in your itinerary.", poiName), nil
		}
	}

	// Generate new POI data
	newPOI, err := l.generatePOIData(ctx, poiName, session.SessionContext.CityName, userLocation, session.UserID, cityID)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to generate POI data", slog.Any("error", err))
		span.RecordError(err)
		return "", fmt.Errorf("failed to generate POI data: %w", err)
	}

	session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = append(
		session.CurrentItinerary.AIItineraryResponse.PointsOfInterest, newPOI)
	return fmt.Sprintf("I've added %s to your itinerary.", poiName), nil
}

// handleSemanticRemovePOI handles removing POIs with semantic understanding
func (l *LlmInteractiontServiceImpl) handleSemanticRemovePOI(ctx context.Context, message string, session *types.ChatSession) string {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "handleSemanticRemovePOI")
	defer span.End()

	poiName := extractPOIName(message)
	if poiName == "" {
		return "I'd be happy to remove a POI from your itinerary! Could you please specify which place you'd like to remove?"
	}

	// Use semantic matching for removal - be more flexible with name matching
	for i, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
		// Check for exact match or semantic similarity
		if strings.EqualFold(poi.Name, poiName) ||
			strings.Contains(strings.ToLower(poi.Name), strings.ToLower(poiName)) ||
			strings.Contains(strings.ToLower(poiName), strings.ToLower(poi.Name)) {

			removedName := poi.Name
			session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = append(
				session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[:i],
				session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[i+1:]...,
			)
			l.logger.InfoContext(ctx, "Removed POI from itinerary",
				slog.String("removed_poi", removedName))
			span.SetAttributes(attribute.String("removed_poi", removedName))
			return fmt.Sprintf("I've removed %s from your itinerary.", removedName)
		}
	}

	return fmt.Sprintf("I couldn't find %s in your itinerary. Here's what you currently have: %s",
		poiName, strings.Join(func() []string {
			var names []string
			for _, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
				names = append(names, poi.Name)
			}
			return names
		}(), ", "))
}

// handleSemanticQuestion handles questions with contextual semantic information
func (l *LlmInteractiontServiceImpl) handleSemanticQuestion(ctx context.Context, message string, session *types.ChatSession, semanticPOIs []types.POIDetail) string {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "handleSemanticQuestion")
	defer span.End()

	var response strings.Builder
	response.WriteString("I'm here to help with your trip planning! ")

	// Add semantic context if available
	if len(semanticPOIs) > 0 {
		response.WriteString("Based on your question, here are some relevant suggestions for ")
		response.WriteString(session.SessionContext.CityName)
		response.WriteString(":\n\n")

		for i, poi := range semanticPOIs[:min(3, len(semanticPOIs))] {
			response.WriteString(fmt.Sprintf("• %s (%s): %s\n",
				poi.Name, poi.Category, poi.DescriptionPOI))
			if i >= 2 {
				break
			}
		}

		response.WriteString("\nWould you like me to add any of these to your itinerary?")
	} else {
		response.WriteString("What specifically would you like to know about your trip to ")
		response.WriteString(session.SessionContext.CityName)
		response.WriteString("?")
	}

	l.logger.InfoContext(ctx, "Provided semantic question response",
		slog.Int("semantic_suggestions", len(semanticPOIs)))
	span.SetAttributes(attribute.Int("semantic_suggestions.count", len(semanticPOIs)))

	return response.String()
}

// handleSemanticModifyItinerary handles itinerary modifications with semantic understanding
func (l *LlmInteractiontServiceImpl) handleSemanticModifyItinerary(ctx context.Context, message string, session *types.ChatSession, semanticPOIs []types.POIDetail, userLocation *types.UserLocation, cityID uuid.UUID) (string, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "handleSemanticModifyItinerary")
	defer span.End()

	// Check for replacement pattern with semantic enhancement
	if matches := regexp.MustCompile(`replace\s+(.+?)\s+with\s+(.+?)(?:\s+in\s+my\s+itinerary)?`).FindStringSubmatch(strings.ToLower(message)); len(matches) == 3 {
		oldPOI := matches[1]
		newPOIName := matches[2]

		// Find POI to replace with semantic matching
		for i, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
			if strings.Contains(strings.ToLower(poi.Name), oldPOI) {
				// First try to find semantic match for replacement
				var newPOI types.POIDetail
				var found bool

				for _, semanticPOI := range semanticPOIs {
					if strings.Contains(strings.ToLower(semanticPOI.Name), strings.ToLower(newPOIName)) ||
						strings.Contains(strings.ToLower(newPOIName), strings.ToLower(semanticPOI.Name)) {
						newPOI = semanticPOI
						found = true
						break
					}
				}

				// If no semantic match, generate POI data
				if !found {
					var err error
					newPOI, err = l.generatePOIData(ctx, newPOIName, session.SessionContext.CityName, userLocation, session.UserID, cityID)
					if err != nil {
						l.logger.ErrorContext(ctx, "Failed to generate POI data for replacement", slog.Any("error", err))
						span.RecordError(err)
						return "", fmt.Errorf("failed to generate POI data for replacement: %w", err)
					}
				}

				oldName := poi.Name
				session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[i] = newPOI
				l.logger.InfoContext(ctx, "Replaced POI in itinerary",
					slog.String("old_poi", oldName),
					slog.String("new_poi", newPOI.Name),
					slog.Bool("semantic_match", found))
				span.SetAttributes(
					attribute.String("old_poi", oldName),
					attribute.String("new_poi", newPOI.Name),
					attribute.Bool("semantic_match", found),
				)

				if found {
					return fmt.Sprintf("Perfect! I found %s which matches your request and replaced %s with it. %s",
						newPOI.Name, oldName, newPOI.DescriptionPOI), nil
				} else {
					return fmt.Sprintf("I've replaced %s with %s in your itinerary.", oldName, newPOI.Name), nil
				}
			}
		}
		return fmt.Sprintf("I couldn't find %s in your itinerary to replace.", oldPOI), nil
	}

	// General modification request - suggest semantic alternatives
	if len(semanticPOIs) > 0 {
		var response strings.Builder
		response.WriteString("I understand you'd like to modify your itinerary. ")
		response.WriteString("Based on your message, here are some relevant suggestions:\n\n")

		for i, poi := range semanticPOIs[:min(3, len(semanticPOIs))] {
			response.WriteString(fmt.Sprintf("• %s (%s): %s\n",
				poi.Name, poi.Category, poi.DescriptionPOI))
			if i >= 2 {
				break
			}
		}

		response.WriteString("\nWould you like me to add any of these, or could you be more specific about what changes you'd like (e.g., 'replace X with Y')?")
		return response.String(), nil
	}

	return "I'd be happy to help modify your itinerary! Could you please be more specific about what changes you'd like (e.g., 'replace X with Y' or 'add more museums')?", nil
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
