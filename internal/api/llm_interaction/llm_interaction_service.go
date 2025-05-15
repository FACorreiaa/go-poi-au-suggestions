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

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/city"
	generativeAI "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/generative_ai"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/poi"
	userInterest "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_interests"
	userSearchProfile "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_search_profiles"
	userTags "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_tags"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

const defaultTemperature = 0.5

type ChatSession struct {
	History []genai.Chat
}

var sessions = make(map[string]*ChatSession) // In-memory session store
var sessionsMu sync.Mutex                    // Mutex for thread-safe access

// Ensure implementation satisfies the interface
var _ LlmInteractiontService = (*LlmInteractiontServiceImpl)(nil)

// LlmInteractiontService defines the business logic contract for user operations.
type LlmInteractiontService interface {
	GetPromptResponse(ctx context.Context, cityName string, userID, profileID uuid.UUID) (*types.AiCityResponse, error)
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

func (l *LlmInteractiontServiceImpl) generateGeneralCityData(wg *sync.WaitGroup,
	ctx context.Context,
	cityName string,
	resultCh chan<- types.GenAIResponse,
	config *genai.GenerateContentConfig) {
	go func() {
		defer wg.Done()
		prompt := GetCityDescriptionPrompt(cityName)
		response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
		if err != nil {
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
			resultCh <- types.GenAIResponse{Err: fmt.Errorf("no valid city data content from AI")}
			return
		}

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
			resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse city data JSON: %w", err)}
			return
		}

		stateProvinceValue := ""
		if cityDataFromAI.StateProvince != nil {
			stateProvinceValue = *cityDataFromAI.StateProvince
		}

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

func (l *LlmInteractiontServiceImpl) generateGeneralPOI(wg *sync.WaitGroup,
	ctx context.Context,
	cityName string,
	resultCh chan<- types.GenAIResponse,
	config *genai.GenerateContentConfig) {
	defer wg.Done()
	prompt := GetGeneralPOI(cityName)
	//startTime := time.Now()
	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	//latencyMs := int(time.Since(startTime).Milliseconds())
	if err != nil {
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
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("no valid general POI content from AI")}
		return
	}

	jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
	var poiData struct {
		PointsOfInterest []types.POIDetail `json:"points_of_interest"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &poiData); err != nil {
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse general POI JSON: %w", err)}
		return
	}

	resultCh <- types.GenAIResponse{GeneralPOI: poiData.PointsOfInterest}
}

func (l *LlmInteractiontServiceImpl) generatePersonalisedPOI(wg *sync.WaitGroup, ctx context.Context,
	cityName string, userID, profileID uuid.UUID, resultCh chan<- types.GenAIResponse,
	interestNames []string, tagsPromptPart string, userPrefs string,
	config *genai.GenerateContentConfig) {
	defer wg.Done()
	startTime := time.Now()

	prompt := GetPersonalizedPOI(interestNames, cityName, tagsPromptPart, userPrefs)

	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	if err != nil {
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
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("no valid personalized itinerary content from AI")}
		return
	}

	jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
	var itineraryData struct {
		ItineraryName      string            `json:"itinerary_name"`
		OverallDescription string            `json:"overall_description"`
		PointsOfInterest   []types.POIDetail `json:"points_of_interest"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &itineraryData); err != nil {
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse personalized itinerary JSON: %w", err)}
		return
	}

	latencyMs := int(time.Since(startTime).Milliseconds())
	interaction := types.LlmInteraction{
		UserID:       userID,
		Prompt:       prompt,
		ResponseText: txt,
		ModelUsed:    "gemini-2.0-flash", // Adjust based on your AI client
		LatencyMs:    latencyMs,
		// request payload
		// response payload
		// Add token counts if available from response (depends on genai API)
		// PromptTokens, CompletionTokens, TotalTokens
		// RequestPayload, ResponsePayload if you serialize the full request/response
	}
	if err := l.llmInteractionRepo.SaveInteraction(ctx, interaction); err != nil {
		l.logger.WarnContext(ctx, "Failed to save LLM interaction", slog.Any("error", err))
		// Decide whether to fail the request or continue
	}

	resultCh <- types.GenAIResponse{
		ItineraryName:        itineraryData.ItineraryName,
		ItineraryDescription: itineraryData.OverallDescription,
		PersonalisedPOI:      itineraryData.PointsOfInterest,
	}
}

func (l *LlmInteractiontServiceImpl) GetPromptResponse(ctx context.Context, cityName string, userID, profileID uuid.UUID) (*types.AiCityResponse, error) {
	// sessionsMu.Lock()
	// session, ok := sessions[sessionID]
	// if !ok {
	// 	session = &ChatSession{History: []genai.ChatMessage{}}
	// 	sessions[sessionID] = session
	// }
	// sessionsMu.Unlock()
	// Fetch user interests, search profile, and tags
	config := &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)}
	l.logger.DebugContext(ctx, "Starting itinerary generation", slog.String("cityName", cityName), slog.String("userID", userID.String()), slog.String("profileID", profileID.String()))
	interests, err := l.interestRepo.GetInterestsForProfile(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user interests: %w", err)
	}
	searchProfile, err := l.searchProfileRepo.GetSearchProfile(ctx, userID, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search profile: %w", err)
	}
	tags, err := l.tagsRepo.GetTagsForProfile(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user tags: %w", err)
	}

	var interestNames []string
	var tagInfoForPrompt []string
	if len(interests) == 0 {
		l.logger.WarnContext(ctx, "No interests found for profile, using fallback.", slog.String("profileID", profileID.String()))
		interestNames = []string{"general sightseeing", "local experiences"}
	} else {
		for _, interest := range interests {
			if interest != nil {
				interestNames = append(interestNames, interest.Name)
			}
		}
	}
	if len(tags) > 0 {
		for _, tag := range tags {
			if tag != nil {
				tagDetail := tag.Name
				if tag.Description != nil && *tag.Description != "" {
					tagDetail += fmt.Sprintf(" (meaning: %s)", *tag.Description)
				}
				tagInfoForPrompt = append(tagInfoForPrompt, tagDetail)
			}
		}
	}
	tagsPromptPart := ""
	if len(tagInfoForPrompt) > 0 {
		tagsPromptPart = fmt.Sprintf("\n    - Additionally, consider these specific user tags/preferences: [%s].", strings.Join(tagInfoForPrompt, "; "))
	}

	// Common user preferences for prompts
	userPrefs := GetUserPreferencesPrompt(searchProfile)

	// Define result struct to collect data from goroutines

	resultCh := make(chan types.GenAIResponse, 3)
	var wg sync.WaitGroup
	wg.Add(3)

	// **Goroutine 1: Generate city, country, and description**
	go l.generateGeneralCityData(&wg, ctx, cityName, resultCh, config)
	// **Goroutine 2: Generate general points of interest**
	go l.generateGeneralPOI(&wg, ctx, cityName, resultCh, config)

	// **Goroutine 3: Generate itinerary name, description, and personalized POIs**
	go l.generatePersonalisedPOI(&wg, ctx, cityName, userID, profileID, resultCh, interestNames, tagsPromptPart, userPrefs, config)

	// Close result channel after goroutines complete
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results from goroutines
	var itinerary types.AiCityResponse
	var errors []error
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
			// itinerary.GeneralCityData.BoundingBoxWKT = res.BoundingBoxWKT // TODO
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
		}
	}

	// Handle any errors from goroutines
	if len(errors) > 0 {
		l.logger.ErrorContext(ctx, "Errors during itinerary generation", slog.Any("errors", errors))
		return nil, fmt.Errorf("failed to generate itinerary: %v", errors)
	}

	// Validate that the itinerary has a name and personalized POIs
	if itinerary.AIItineraryResponse.ItineraryName == "" || len(itinerary.AIItineraryResponse.PointsOfInterest) == 0 {
		l.logger.ErrorContext(ctx, "Incomplete itinerary generated")
		return nil, fmt.Errorf("incomplete itinerary: missing name or personalized POIs")
	}

	l.logger.InfoContext(ctx, "Successfully generated itinerary",
		slog.String("itinerary_name", itinerary.AIItineraryResponse.ItineraryName),
		slog.Int("poi_count", len(itinerary.AIItineraryResponse.PointsOfInterest)))

	// Check if city exists in the database and save if not
	city, err := l.cityRepo.FindCityByNameAndCountry(ctx, itinerary.GeneralCityData.City, itinerary.GeneralCityData.Country)
	if err != nil && err != sql.ErrNoRows {
		l.logger.ErrorContext(ctx, "Failed to check city existence", slog.Any("error", err))
		return nil, fmt.Errorf("failed to check city existence: %w", err)
	}

	var cityID uuid.UUID
	if city == nil {
		cityDetail := types.CityDetail{
			Name:            itinerary.GeneralCityData.City,
			Country:         itinerary.GeneralCityData.Country,
			StateProvince:   itinerary.GeneralCityData.StateProvince,
			AiSummary:       itinerary.GeneralCityData.Description,
			CenterLatitude:  itinerary.GeneralCityData.CenterLatitude,  // Pass these to SaveCity
			CenterLongitude: itinerary.GeneralCityData.CenterLongitude, // Pass these to SaveCity
		}
		cityID, err = l.cityRepo.SaveCity(ctx, cityDetail)
		if err != nil {
			l.logger.ErrorContext(ctx, "Failed to save city", slog.Any("error", err))
			return nil, fmt.Errorf("failed to save city: %w", err)
		}
	} else {
		cityID = city.ID
	}

	// Save general POIs to the database if they don’t exist
	for _, poi := range itinerary.PointsOfInterest {
		existingPoi, err := l.poiRepo.FindPoiByNameAndCity(ctx, poi.Name, cityID)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to check POI existence", slog.String("poi_name", poi.Name), slog.Any("error", err))
			continue
		}
		if existingPoi == nil {
			_, err = l.poiRepo.SavePoi(ctx, poi, cityID)
			if err != nil {
				l.logger.WarnContext(ctx, "Failed to save POI", slog.String("poi_name", poi.Name), slog.Any("error", err))
				continue
			}
		}
	}

	return &itinerary, nil
}

func TruncateString(str string, num int) string {
	if len(str) > num {
		return str[0:num] + "..."
	}
	return str
}
