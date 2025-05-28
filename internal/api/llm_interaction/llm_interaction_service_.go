package llmInteraction

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genai"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/city"
	generativeAI "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/generative_ai"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/poi"
	userInterest "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_interests"
	userSearchProfile "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_search_profiles"
	userTags "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_tags"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

const (
	model              = "gemini-2.0-flash"
	defaultTemperature = 0.5
)

type ChatSession struct {
	History []genai.Chat
}

var sessions = make(map[string]*ChatSession) // In-memory session store
var sessionsMu sync.Mutex                    // Mutex for thread-safe access

// Ensure implementation satisfies the interface
var _ LlmInteractiontService = (*LlmInteractiontServiceImpl)(nil)

// LlmInteractiontService defines the business logic contract for user operations.
type LlmInteractiontService interface {
	GetIteneraryResponse(ctx context.Context, cityName string, userID, profileID uuid.UUID, userLocation *types.UserLocation) (*types.AiCityResponse, error)
	SaveItenerary(ctx context.Context, userID uuid.UUID, req types.BookmarkRequest) (uuid.UUID, error)
	RemoveItenerary(ctx context.Context, userID, itineraryID uuid.UUID) error
	GetPOIDetailsResponse(ctx context.Context, userID uuid.UUID, city string, lat, lon float64) (*types.POIDetailedInfo, error)
}

// LlmInteractiontServiceImpl provides the implementation for LlmInteractiontService.
type LlmInteractiontServiceImpl struct {
	logger             *slog.Logger
	interestRepo       userInterest.UserInterestRepo
	searchProfileRepo  userSearchProfile.UserSearchProfilesRepo
	tagsRepo           userTags.UserTagsRepo
	aiClient           *generativeAI.AIClient
	llmInteractionRepo LLmInteractionRepository
	cityRepo           city.CityRepository
	poiRepo            poi.POIRepository
}

// NewLlmInteractiontService creates a new user service instance.
func NewLlmInteractiontService(interestRepo userInterest.UserInterestRepo,
	searchProfileRepo userSearchProfile.UserSearchProfilesRepo,
	tagsRepo userTags.UserTagsRepo,
	llmInteractionRepo LLmInteractionRepository,
	cityRepo city.CityRepository,
	poiRepo poi.POIRepository,
	logger *slog.Logger) *LlmInteractiontServiceImpl {
	ctx := context.Background()
	aiClient, err := generativeAI.NewAIClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err) // Terminate if initialization fails
	}
	return &LlmInteractiontServiceImpl{
		logger:             logger,
		tagsRepo:           tagsRepo,
		interestRepo:       interestRepo,
		searchProfileRepo:  searchProfileRepo,
		aiClient:           aiClient,
		llmInteractionRepo: llmInteractionRepo,
		cityRepo:           cityRepo,
		poiRepo:            poiRepo,
	}
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

		jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
		var cityDataFromAI struct {
			CityName        string  `json:"city_name"`
			StateProvince   *string `json:"state_province"` // Use pointer for nullable string
			Country         string  `json:"country"`
			CenterLatitude  float64 `json:"center_latitude"`
			CenterLongitude float64 `json:"center_longitude"`
			Description     string  `json:"description"`
			// BoundingBox     string  `json:"bounding_box,omitempty"` // If trying to get BBox string
		}
		if err := json.Unmarshal([]byte(jsonStr), &cityDataFromAI); err != nil {
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

	prompt := getGeneralPOI(cityName)
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

	jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
	var poiData struct {
		PointsOfInterest []types.POIDetail `json:"points_of_interest"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &poiData); err != nil {
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

	jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
	var itineraryData struct {
		ItineraryName      string            `json:"itinerary_name"`
		OverallDescription string            `json:"overall_description"`
		PointsOfInterest   []types.POIDetail `json:"points_of_interest"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &itineraryData); err != nil {
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

	originalInteraction, err := l.llmInteractionRepo.GetInteractionByID(ctx, req.LlmInteractionID)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to fetch original LLM interaction for bookmarking", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch original interaction")
		return uuid.Nil, fmt.Errorf("could not retrieve original interaction: %w", err)
	}
	if originalInteraction == nil { // Or however you check for not found
		l.logger.WarnContext(ctx, "LLM interaction not found for bookmarking", slog.String("llmInteractionID", req.LlmInteractionID.String()))
		err := fmt.Errorf("original interaction %s not found", req.LlmInteractionID)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Interaction not found")
		return uuid.Nil, err
	}
	markdownContent := originalInteraction.ResponseText

	//markdownContent := "Placeholder: Content from LLM Interaction " + req.LlmInteractionID.String() + ". This should be fetched from the DB."
	l.logger.WarnContext(ctx, "Using placeholder markdownContent for bookmark. Implement fetching original interaction text.",
		slog.String("llmInteractionID", req.LlmInteractionID.String()))

	var description sql.NullString
	if req.Description != nil {
		description.String = *req.Description
		description.Valid = true
		span.SetAttributes(attribute.String("description", *req.Description))
	}

	isPublic := false // Default
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
		span.SetAttributes(attribute.Bool("is_public", isPublic))
	}

	newBookmark := &types.UserSavedItinerary{
		UserID:                 userID,
		SourceLlmInteractionID: uuid.NullUUID{UUID: req.LlmInteractionID, Valid: true},
		Title:                  req.Title,
		Description:            description,
		MarkdownContent:        markdownContent,
		Tags:                   req.Tags, // pgx handles nil []string as NULL for TEXT[]
		IsPublic:               isPublic,
	}

	if req.Tags != nil && len(req.Tags) > 0 {
		span.SetAttributes(attribute.Int("tags.count", len(req.Tags)))
	}

	savedID, err := l.llmInteractionRepo.AddChatToBookmark(ctx, newBookmark)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to add chat to bookmark")
		return uuid.Nil, err // Error already logged by repo
	}

	l.logger.InfoContext(ctx, "Successfully bookmarked interaction", slog.String("savedItineraryID", savedID.String()))
	span.SetAttributes(attribute.String("saved_itinerary.id", savedID.String()))
	span.SetStatus(codes.Ok, "Itinerary saved successfully")
	return savedID, nil

	// Save the itinerary using the repository
	// itineraryID, err := l.llmInteractionRepo.SaveItinerary(ctx, itinerary)
	// if err != nil {
	// 	return uuid.Nil, fmt.Errorf("failed to save itinerary: %w", err)
	// }

	// l.logger.InfoContext(ctx, "Itinerary saved successfully", slog.String("itinerary_id", itineraryID.String()))
	// return itineraryID, nil
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
	jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
	var detailedInfo types.POIDetailedInfo
	if err := json.Unmarshal([]byte(jsonStr), &detailedInfo); err != nil {
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

	resultCh := make(chan types.POIDetailedInfo, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go l.getPOIdetails(&wg, ctx, city, lat, lon, userID, resultCh, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for res := range resultCh {
		if res.Err != nil {
			l.logger.ErrorContext(ctx, "Error generating POI details", slog.Any("error", res.Err))
			span.RecordError(res.Err)
			span.SetStatus(codes.Error, "Failed to generate POI details")
			return nil, res.Err
		}
		return &res, nil
	}

	l.logger.WarnContext(ctx, "No response received for POI details")
	return nil, fmt.Errorf("no response received for POI details")
}
