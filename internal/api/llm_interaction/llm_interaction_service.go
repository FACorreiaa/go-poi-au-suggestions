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
	GetPromptResponse(ctx context.Context, cityName string, userID, profileID uuid.UUID, userLocation *types.UserLocation) (*types.AiCityResponse, error)
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
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to save LLM interaction: %w", err)}
		return
	}

	resultCh <- types.GenAIResponse{
		ItineraryName:        itineraryData.ItineraryName,
		ItineraryDescription: itineraryData.OverallDescription,
		PersonalisedPOI:      itineraryData.PointsOfInterest,
		LlmInteractionID:     savedInteractionID,
	}
}

func (l *LlmInteractiontServiceImpl) GetPromptResponse(ctx context.Context, cityName string, userID, profileID uuid.UUID, userLocation *types.UserLocation) (*types.AiCityResponse, error) {
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

	//
	//var userCurrentLocation *types.UserLocation
	if searchProfile.UserLatitude != nil && searchProfile.UserLongitude != nil {
		userLocation = &types.UserLocation{
			UserLat: *searchProfile.UserLatitude,
			UserLon: *searchProfile.UserLongitude,
		}
	} else {
		l.logger.WarnContext(ctx, "User location not available in search profile, cannot sort personalised POIs by distance.")
		// Depending on requirements, you might proceed without sorting or return an error.
	}

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
	var llmInteractionIDForPersonalisedPOIs uuid.UUID
	var rawPersonalisedPOIs []types.POIDetail

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
		if res.LlmInteractionID != uuid.Nil {
			llmInteractionIDForPersonalisedPOIs = res.LlmInteractionID
			itinerary.AIItineraryResponse.ItineraryName = res.ItineraryName
			itinerary.AIItineraryResponse.OverallDescription = res.ItineraryDescription
			rawPersonalisedPOIs = res.PersonalisedPOI // Store raw POIs for now
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
	if err != nil {
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

	// TODO Sort the POIs by distance from user location
	if userLocation != nil && cityID != uuid.Nil && len(itinerary.AIItineraryResponse.PointsOfInterest) > 0 {
		l.logger.InfoContext(ctx, "Attempting to save and sort personalised POIs by distance.")
		err := l.llmInteractionRepo.SaveLlmSuggestedPOIsBatch(ctx, rawPersonalisedPOIs, userID, profileID, llmInteractionIDForPersonalisedPOIs, cityID)
		if err != nil {
			l.logger.ErrorContext(ctx, "Failed to save personalised POIs", slog.Any("error", err))
			return nil, fmt.Errorf("failed to save personalised POIs: %w", err)
		} else {
			l.logger.InfoContext(ctx, "Successfully saved personalised POIs to the database.")
			l.logger.DebugContext(ctx, "Fetching LLM suggested POIs sorted by distance",
				slog.String("llm_interaction_id", llmInteractionIDForPersonalisedPOIs.String()),
				slog.String("cityID", cityID.String()))

			sortedPois, sortErr := l.llmInteractionRepo.GetLlmSuggestedPOIsByInteractionSortedByDistance(
				ctx, llmInteractionIDForPersonalisedPOIs, cityID, *userLocation,
			)
			if sortErr != nil {
				l.logger.ErrorContext(ctx, "Failed to fetch sorted LLM suggested POIs, using unsorted (but saved).", slog.Any("error", sortErr))
				itinerary.AIItineraryResponse.PointsOfInterest = rawPersonalisedPOIs
			} else {
				l.logger.InfoContext(ctx, "Successfully fetched and sorted LLM suggested POIs.", slog.Int("count", len(sortedPois)))
				itinerary.AIItineraryResponse.PointsOfInterest = sortedPois
			}

		}
		var personalisedPoiNamesToQuery []string
		tempPersonalisedPois := make([]types.POIDetail, 0, len(itinerary.AIItineraryResponse.PointsOfInterest))

		for _, pPoi := range itinerary.AIItineraryResponse.PointsOfInterest { // These are personalised POIs from LLM
			// Check if POI exists, save if not.
			// This step ensures that the POIs are in the DB before attempting to sort them via a DB query.
			existingPersPoi, err := l.poiRepo.FindPoiByNameAndCity(ctx, pPoi.Name, cityID)
			if err != nil && err != sql.ErrNoRows {
				l.logger.WarnContext(ctx, "Error checking personalised POI for saving", slog.String("name", pPoi.Name), slog.Any("error", err))
				tempPersonalisedPois = append(tempPersonalisedPois, pPoi) // Add unsaved POI to list to keep it
				continue
			}

			var dbPoiID uuid.UUID
			var dbPoiLat, dbPoiLon float64

			if existingPersPoi == nil {
				// POI doesn't exist, save it
				// The SavePoi function should ideally handle setting the location GEOMETRY from pPoi.Latitude and pPoi.Longitude
				savedID, saveErr := l.poiRepo.SavePoi(ctx, pPoi, cityID)
				if saveErr != nil {
					l.logger.WarnContext(ctx, "Failed to save new personalised POI", slog.String("name", pPoi.Name), slog.Any("error", saveErr))
					tempPersonalisedPois = append(tempPersonalisedPois, pPoi) // Add unsaved POI
					continue
				}
				dbPoiID = savedID
				dbPoiLat = pPoi.Latitude // Use original LLM lat/lon for the newly saved POI
				dbPoiLon = pPoi.Longitude
				l.logger.DebugContext(ctx, "Saved new personalised POI", slog.String("name", pPoi.Name), slog.String("id", dbPoiID.String()))
			} else {
				// POI already exists
				dbPoiID = existingPersPoi.ID
				dbPoiLat = existingPersPoi.Latitude // Use DB lat/lon for existing POI
				dbPoiLon = existingPersPoi.Longitude
				l.logger.DebugContext(ctx, "Found existing personalised POI", slog.String("name", pPoi.Name), slog.String("id", dbPoiID.String()))
			}

			// Add name for querying sorted list, and update the POI detail for potential use if sorting fails
			pPoi.ID = dbPoiID        // Update the POI in memory with its database ID
			pPoi.Latitude = dbPoiLat // Ensure using consistent lat/lon (from DB if exists, from LLM if new)
			pPoi.Longitude = dbPoiLon
			tempPersonalisedPois = append(tempPersonalisedPois, pPoi)
			personalisedPoiNamesToQuery = append(personalisedPoiNamesToQuery, pPoi.Name)
		}

		// Update the itinerary with POIs that have now been processed (saved/found and IDs updated)
		itinerary.AIItineraryResponse.PointsOfInterest = tempPersonalisedPois

		// If there are any names to query (meaning some POIs were processed successfully)
		if len(personalisedPoiNamesToQuery) > 0 {
			l.logger.DebugContext(ctx, "Fetching personalised POIs sorted by distance",
				slog.Any("names", personalisedPoiNamesToQuery),
				slog.String("cityID", cityID.String()),
				slog.Any("user_location", *userLocation),
			)
			sortedPersonalisedPois, sortErr := l.poiRepo.GetPOIsByNamesAndCitySortedByDistance(ctx, personalisedPoiNamesToQuery, cityID, *userLocation)
			if sortErr != nil {
				l.logger.ErrorContext(ctx, "Failed to fetch sorted personalised POIs, returning them unsorted (but potentially saved/updated).", slog.Any("error", sortErr))
				// itinerary.AIItineraryResponse.PointsOfInterest will remain as tempPersonalisedPois (unsorted but processed)
			} else {
				l.logger.InfoContext(ctx, "Successfully fetched and sorted personalised POIs by distance.", slog.Int("count", len(sortedPersonalisedPois)))
				// We need to be careful here. GetPOIsByNamesAndCitySortedByDistance returns a new list.
				// We should map the sorted POIs back to the itinerary's list or replace it carefully,
				// ensuring all relevant data (like LLM description) is preserved if the DB version is minimal.
				// For simplicity, if the sorted list contains all necessary fields, we can replace it.
				// If `types.POIDetail` from `GetPOIsByNamesAndCitySortedByDistance` is complete, this is fine:
				itinerary.AIItineraryResponse.PointsOfInterest = sortedPersonalisedPois
			}
		} else {
			l.logger.InfoContext(ctx, "No personalised POIs were successfully processed for sorting by distance.")
		}

	} else {
		if userLocation == nil {
			l.logger.InfoContext(ctx, "Skipping sorting of personalised POIs: user location not available.")
		}
		if cityID == uuid.Nil {
			l.logger.InfoContext(ctx, "Skipping sorting of personalised POIs: cityID is nil (city not found/saved).")
		}
		if len(itinerary.AIItineraryResponse.PointsOfInterest) == 0 {
			l.logger.InfoContext(ctx, "Skipping sorting of personalised POIs: no personalised POIs to sort.")
		}
	}

	l.logger.InfoContext(ctx, "Final itinerary ready",
		slog.String("itinerary_name", itinerary.AIItineraryResponse.ItineraryName),
		slog.Int("final_personalised_poi_count", len(itinerary.AIItineraryResponse.PointsOfInterest)))

	return &itinerary, nil
}

func TruncateString(str string, num int) string {
	if len(str) > num {
		return str[0:num] + "..."
	}
	return str
}
