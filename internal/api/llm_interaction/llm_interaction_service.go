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

// func (l *LlmInteractiontServiceImpl) GetPromptResponse_NonConcurrent(ctx context.Context, cityName string, userID, profileID uuid.UUID) (*types.AIItineraryResponse, error) {
// 	// Fetch user interests
// 	interests, err := l.interestRepo.GetInterestsForProfile(ctx, profileID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to fetch user interests: %w", err)
// 	}

// 	// Fetch search profile
// 	searchProfile, err := l.searchProfileRepo.GetSearchProfile(ctx, userID, profileID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to fetch search profile: %w", err)
// 	}

// 	// fetch tags repo
// 	tags, err := l.tagsRepo.GetTagsForProfile(ctx, profileID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to fetch user tags: %w", err)
// 	}

// 	var interestNames []string
// 	var tagInfoForPrompt []string
// 	if len(interests) == 0 {
// 		l.logger.WarnContext(ctx, "No interests found for profile, using fallback.", slog.String("profileID", profileID.String()))
// 		interestNames = []string{"general sightseeing", "local experiences"}
// 	} else {
// 		for _, interest := range interests {
// 			if interest != nil {
// 				interestNames = append(interestNames, interest.Name)
// 			}
// 		}
// 	}
// 	l.logger.DebugContext(ctx, "Fetched profile interests", slog.Any("interestNames", interestNames))

// 	if len(tags) > 0 {
// 		for _, tag := range tags {
// 			if tag != nil {
// 				tagDetail := tag.Name
// 				if tag.Description != nil && *tag.Description != "" {
// 					tagDetail += fmt.Sprintf(" (meaning: %s)", *tag.Description)
// 				}
// 				tagInfoForPrompt = append(tagInfoForPrompt, tagDetail)
// 			}
// 		}
// 	}
// 	l.logger.DebugContext(ctx, "Fetched profile tags", slog.Any("tagsForPrompt", tagInfoForPrompt))

// 	tagsPromptPart := ""
// 	if len(tagInfoForPrompt) > 0 {
// 		tagsPromptPart = fmt.Sprintf("\n    - Additionally, consider these specific user tags/preferences: [%s]. These might relate to vibes to seek or avoid, specific needs, or types of activities.", strings.Join(tagInfoForPrompt, "; "))
// 	}

// 	prompt := fmt.Sprintf(`
//     Generate a personalized itinerary for %s.
//     The user is interested in: [%s].%s
//     The user's general preferences from their profile are:
//     - Search Radius: %.1f km
//     - Preferred Time: %s
//     - Budget Level: %d (0=any, 1=cheap, 4=expensive)
//     - Prefers Outdoor Seating: %t
//     - Prefers Dog Friendly: %t
//     - Preferred Dietary Needs: [%s]
//     - Preferred Pace: %s
//     - Prefers Accessible POIs: %t
//     - Preferred Vibes: [%s]
//     - Preferred Transport: %s

//     Return the response STRICTLY as a single JSON object. Do NOT include any text or explanation before or after the JSON.
// 	The JSON size should be enough to fit all the time for the user interests. Weather a full day, half a day or a few hours,
// 	depending on the user interests and the city. The itinerary should be a mix of activities, including some that are more active and some that are more relaxed,
// 	(depending on the user interests, profile and tags)
//     The JSON object should have the following structure:
//     {
//       "itinerary_name": "A creative and descriptive name for this itinerary based on the city, interests, tags, and preferences",
//       "overall_description": "A 1 paragraph descriptive story about exploring %s with these interests and preferences.",
//       "points_of_interest": [
//         {
//           "name": "Name of the Point of Interest",
//           "latitude": <float_latitude_value>,
//           "longitude": <float_longitude_value>,
//           "category": "Primary category (e.g., Museum, Historical Site, Park, Restaurant, Bar)",
//           "description_poi": "A 2-3 sentence description of this specific POI and why it's relevant."
//         }
//         // ... include several relevant POIs ...
//       ]
//       // Optionally, if interests suggest other categories like 'restaurants' or 'bars',
//       // you can include them as separate arrays similar to 'points_of_interest'.
//     }
// `, cityName, strings.Join(interestNames, ", "), tagsPromptPart,
// 		searchProfile.SearchRadiusKm, searchProfile.PreferredTime, searchProfile.BudgetLevel,
// 		searchProfile.PreferOutdoorSeating, searchProfile.PreferDogFriendly, strings.Join(searchProfile.DietaryNeeds, ", "),
// 		searchProfile.PreferredPace, searchProfile.PreferAccessiblePOIs, strings.Join(searchProfile.PreferredVibes, ", "),
// 		searchProfile.PreferredTransport,
// 		cityName, // For the overall_description part
// 	)

// 	config := &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}
// 	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	var txt string
// 	for _, candidate := range response.Candidates {
// 		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
// 			log.Println("Candidate has no content or parts.")
// 			continue
// 		}

// 		part := candidate.Content.Parts[0]
// 		txt = part.Text
// 		// fmt.Printf("Part text: [%s]\n", txt)
// 		// if txt != "" {
// 		// 	log.Printf("Extracted text: [%s]\n", txt)
// 		// 	type POI struct {
// 		// 		Name      string  `json:"name"`
// 		// 		Latitude  float64 `json:"latitude"`
// 		// 		Longitude float64 `json:"longitude"`
// 		// 		Category  string  `json:"category"`
// 		// 	}
// 		// 	var pois []POI

// 		// 	if err := json.Unmarshal([]byte(txt), &pois); err != nil {
// 		// 		log.Printf("Failed to unmarshal AI response text into POIs: %v. Text was: %s\n", err, txt)
// 		// 	} else {
// 		// 		fmt.Println("POIs (successfully unmarshalled):", pois)
// 		// 	}
// 		// } else {
// 		// 	log.Println("Part's text was empty.")
// 		// }
// 	}

// 	jsonStr := txt
// 	jsonStr = strings.TrimPrefix(jsonStr, "```json")
// 	jsonStr = strings.TrimSuffix(jsonStr, "```")
// 	jsonStr = strings.TrimSpace(jsonStr)

// 	if !strings.HasPrefix(jsonStr, "{") || !strings.HasSuffix(jsonStr, "}") {
// 		l.logger.ErrorContext(ctx, "Extracted text does not appear to be a valid JSON object after trimming", slog.String("trimmed_text_snippet", TruncateString(jsonStr, 200)))
// 		return nil, fmt.Errorf("AI response was not a valid JSON object after cleaning attempts")
// 	}

// 	var itineraryData types.AIItineraryResponse // Defined above or in types package
// 	if err := json.Unmarshal([]byte(jsonStr), &itineraryData); err != nil {
// 		l.logger.ErrorContext(ctx, "Failed to unmarshal AI JSON response", slog.String("json_string_snippet", TruncateString(jsonStr, 200)), slog.Any("error", err))
// 		return nil, fmt.Errorf("failed to parse AI response JSON: %w", err)
// 	}

// 	l.logger.InfoContext(ctx, "Successfully parsed AI itinerary response",
// 		slog.String("itinerary_name", itineraryData.ItineraryName),
// 		slog.Int("poi_count", len(itineraryData.PointsOfInterest)))

// 	// TODO Validate and store the itineraryData in DB
// 	// 1. Validate the contents of itineraryData (e.g., are latitudes/longitudes valid numbers?)
// 	// 2. For each POIDetail in itineraryData.PointsOfInterest:
// 	//    a. Check if it exists in your database (by name and city).
// 	//    b. If not, insert it (this is how you "seed" your DB).
// 	//    c. If it exists, you might update its ai_summary or other AI-derived fields.
// 	// 3. Store the overall itinerary structure (itinerary_name, overall_description, and links to POIs)
// 	//    in your `itineraries` and `itinerary_pois` tables.
// 	// 4. Store the PoI details in the `pois` table if they don't exist.
// 	// 5. Return the itineraryData or a success message.

// 	return &itineraryData, nil
// }

func (l *LlmInteractiontServiceImpl) GetPromptResponse(ctx context.Context, cityName string, userID, profileID uuid.UUID) (*types.AiCityResponse, error) {
	// Fetch user interests, search profile, and tags
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
	userPrefs := fmt.Sprintf(`
    - Search Radius: %.1f km
    - Preferred Time: %s
    - Budget Level: %d (0=any, 1=cheap, 4=expensive)
    - Prefers Outdoor Seating: %t
    - Prefers Dog Friendly: %t
    - Preferred Dietary Needs: [%s]
    - Preferred Pace: %s
    - Prefers Accessible POIs: %t
    - Preferred Vibes: [%s]
    - Preferred Transport: %s
`, searchProfile.SearchRadiusKm, searchProfile.PreferredTime, searchProfile.BudgetLevel,
		searchProfile.PreferOutdoorSeating, searchProfile.PreferDogFriendly, strings.Join(searchProfile.DietaryNeeds, ", "),
		searchProfile.PreferredPace, searchProfile.PreferAccessiblePOIs, strings.Join(searchProfile.PreferredVibes, ", "),
		searchProfile.PreferredTransport)

	// Define result struct to collect data from goroutines
	type result struct {
		city                 string
		country              string
		cityDescription      string
		itineraryName        string
		itineraryDescription string
		generalPOI           []types.POIDetail
		personalisedPOI      []types.POIDetail
		err                  error
	}
	resultCh := make(chan result, 3)
	var wg sync.WaitGroup
	wg.Add(3)

	// **Goroutine 1: Generate city, country, and description**
	go func() {
		defer wg.Done()
		prompt := fmt.Sprintf(`
            Generate the country this city belongs to and a brief description of %s.
            Return the response STRICTLY as a JSON object with:
            {
            "city": "%s",
            "country": "the country of the city",
            "description": "A brief description of the city, including its history and main attractions."
            }`, cityName, cityName)
		//startTime := time.Now()
		config := &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}
		response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
		//latencyMs := int(time.Since(startTime).Milliseconds())
		if err != nil {
			resultCh <- result{err: fmt.Errorf("failed to generate city data: %w", err)}
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
			resultCh <- result{err: fmt.Errorf("no valid city data content from AI")}
			return
		}

		jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
		var cityData struct {
			City        string `json:"city"`
			Country     string `json:"country"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &cityData); err != nil {
			resultCh <- result{err: fmt.Errorf("failed to parse city data JSON: %w", err)}
			return
		}

		resultCh <- result{
			city:            cityData.City,
			country:         cityData.Country,
			cityDescription: cityData.Description,
		}
	}()

	// **Goroutine 2: Generate general points of interest**
	go func() {
		defer wg.Done()
		prompt := fmt.Sprintf(`
            Generate a list of maximum 5 general points of interest that people usually see no matter the taste or preference for this city %s.
            Return the response STRICTLY as a JSON object with:
            {
            "points_of_interest": [
                {
                "name": "Name of the Point of Interest",
                "latitude": <float>,
                "longitude": <float>,
                "category": "Primary category (e.g., Museum, Historical Site, Park, Restaurant, Bar)",
                "description_poi": "A 2-3 sentence description of this specific POI and why it's relevant."
                }
            ]
            }`, cityName)
		//startTime := time.Now()
		config := &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}
		response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
		//latencyMs := int(time.Since(startTime).Milliseconds())
		if err != nil {
			resultCh <- result{err: fmt.Errorf("failed to generate general POIs: %w", err)}
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
			resultCh <- result{err: fmt.Errorf("no valid general POI content from AI")}
			return
		}

		jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
		var poiData struct {
			PointsOfInterest []types.POIDetail `json:"points_of_interest"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &poiData); err != nil {
			resultCh <- result{err: fmt.Errorf("failed to parse general POI JSON: %w", err)}
			return
		}

		resultCh <- result{generalPOI: poiData.PointsOfInterest}
	}()

	// **Goroutine 3: Generate itinerary name, description, and personalized POIs**
	go func() {
		defer wg.Done()
		prompt := fmt.Sprintf(`
            Generate a creative itinerary name, a personalized description, and a list of personalized points of interest for %s based on the user's interests: [%s].%s
            The user's general preferences are:
            %s
            Return the response STRICTLY as a JSON object with:
            {
            "itinerary_name": "A creative and descriptive name for this itinerary",
            "overall_description": "A 1 paragraph short descriptive story about exploring %s with these interests and preferences.",
            "points_of_interest": [
                {
                "name": "Name of the Point of Interest",
                "latitude": <float>,
                "longitude": <float>,
                "category": "Primary category (e.g., Museum, Historical Site, Park, Restaurant, Bar)",
                "description_poi": "A 2-3 very short sentence description of this specific POI and why it's relevant to the user's interests."
                }
            ]
            }`, cityName, strings.Join(interestNames, ", "), tagsPromptPart, userPrefs, cityName)
		config := &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}
		response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
		if err != nil {
			resultCh <- result{err: fmt.Errorf("failed to generate personalized itinerary: %w", err)}
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
			resultCh <- result{err: fmt.Errorf("no valid personalized itinerary content from AI")}
			return
		}

		jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
		var itineraryData struct {
			ItineraryName      string            `json:"itinerary_name"`
			OverallDescription string            `json:"overall_description"`
			PointsOfInterest   []types.POIDetail `json:"points_of_interest"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &itineraryData); err != nil {
			resultCh <- result{err: fmt.Errorf("failed to parse personalized itinerary JSON: %w", err)}
			return
		}

		resultCh <- result{
			itineraryName:        itineraryData.ItineraryName,
			itineraryDescription: itineraryData.OverallDescription,
			personalisedPOI:      itineraryData.PointsOfInterest,
		}
	}()

	// Close result channel after goroutines complete
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results from goroutines
	var itinerary types.AiCityResponse
	var errors []error
	for res := range resultCh {
		if res.err != nil {
			errors = append(errors, res.err)
			continue
		}
		if res.city != "" {
			itinerary.GeneralCityData.City = res.city
			itinerary.GeneralCityData.Country = res.country
			itinerary.GeneralCityData.Description = res.cityDescription
		}
		if res.itineraryName != "" {
			itinerary.AIItineraryResponse.ItineraryName = res.itineraryName
			itinerary.AIItineraryResponse.OverallDescription = res.itineraryDescription
		}
		if len(res.generalPOI) > 0 {
			itinerary.PointsOfInterest = res.generalPOI
		}
		if len(res.personalisedPOI) > 0 {
			itinerary.AIItineraryResponse.PointsOfInterest = res.personalisedPOI
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
			Name:      itinerary.GeneralCityData.City,
			Country:   itinerary.GeneralCityData.Country,
			AiSummary: itinerary.GeneralCityData.Description,
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
