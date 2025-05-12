package llmInteraction

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"strings"

	generativeAI "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/generative_ai"
	userInterest "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_interests"
	userSearchProfile "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_search_profiles"
	userTags "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_tags"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

// Ensure implementation satisfies the interface
var _ LlmInteractiontService = (*LlmInteractiontServiceImpl)(nil)

// LlmInteractiontService defines the business logic contract for user operations.
type LlmInteractiontService interface {
	GetPromptResponse(ctx context.Context, cityName string, userID, profileID uuid.UUID) (*AIItineraryResponse, error)
}

// LlmInteractiontServiceImpl provides the implementation for LlmInteractiontService.
type LlmInteractiontServiceImpl struct {
	logger            *slog.Logger
	interestRepo      userInterest.UserInterestRepo
	searchProfileRepo userSearchProfile.UserSearchProfilesRepo
	tagsRepo          userTags.UserTagsRepo
	aiClient          *generativeAI.AIClient
}

// NewLlmInteractiontService creates a new user service instance.
func NewLlmInteractiontService(interestRepo userInterest.UserInterestRepo,
	searchProfileRepo userSearchProfile.UserSearchProfilesRepo,
	tagsRepo userTags.UserTagsRepo,
	logger *slog.Logger) *LlmInteractiontServiceImpl {
	ctx := context.Background()
	aiClient, err := generativeAI.NewAIClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err) // Terminate if initialization fails
	}
	return &LlmInteractiontServiceImpl{
		logger:            logger,
		tagsRepo:          tagsRepo,
		interestRepo:      interestRepo,
		searchProfileRepo: searchProfileRepo,
		aiClient:          aiClient,
	}
}

type POIDetail struct {
	Name           string  `json:"name"`
	Latitude       float64 `json:"latitude"` // AI should provide numbers for these
	Longitude      float64 `json:"longitude"`
	Category       string  `json:"category"`
	DescriptionPOI string  `json:"description_poi,omitempty"`
}

type AIItineraryResponse struct {
	ItineraryName      string      `json:"itinerary_name"`
	OverallDescription string      `json:"overall_description"`
	PointsOfInterest   []POIDetail `json:"points_of_interest"`
	//You might add other dynamic categories here if the AI generates them
	//For example:
	Restaurants []POIDetail `json:"restaurants,omitempty"`
	Bars        []POIDetail `json:"bars,omitempty"`
}

func (l *LlmInteractiontServiceImpl) GetPromptResponse(ctx context.Context, cityName string, userID, profileID uuid.UUID) (*AIItineraryResponse, error) {
	// Fetch user interests
	interests, err := l.interestRepo.GetInterestsForProfile(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user interests: %w", err)
	}

	// Fetch search profile
	searchProfile, err := l.searchProfileRepo.GetSearchProfile(ctx, userID, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search profile: %w", err)
	}

	// fetch tags repo
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
	l.logger.DebugContext(ctx, "Fetched profile interests", slog.Any("interestNames", interestNames))

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
	l.logger.DebugContext(ctx, "Fetched profile tags", slog.Any("tagsForPrompt", tagInfoForPrompt))

	tagsPromptPart := ""
	if len(tagInfoForPrompt) > 0 {
		tagsPromptPart = fmt.Sprintf("\n    - Additionally, consider these specific user tags/preferences: [%s]. These might relate to vibes to seek or avoid, specific needs, or types of activities.", strings.Join(tagInfoForPrompt, "; "))
	}

	prompt := fmt.Sprintf(`
    Generate a personalized itinerary for %s.
    The user is interested in: [%s].%s
    The user's general preferences from their profile are:
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

    Return the response STRICTLY as a single JSON object. Do NOT include any text or explanation before or after the JSON.
	The JSON size should be enough to fit all the time for the user interests. Weather a full day, half a day or a few hours,
	depending on the user interests and the city. The itinerary should be a mix of activities, including some that are more active and some that are more relaxed,
	(depending on the user interests, profile and tags)
    The JSON object should have the following structure:
    {
      "itinerary_name": "A creative and descriptive name for this itinerary based on the city, interests, tags, and preferences",
      "overall_description": "A 1-2 paragraph descriptive story about exploring %s with these interests and preferences.",
      "points_of_interest": [
        {
          "name": "Name of the Point of Interest",
          "latitude": <float_latitude_value>,
          "longitude": <float_longitude_value>,
          "category": "Primary category (e.g., Museum, Historical Site, Park, Restaurant, Bar)",
          "description_poi": "A 2-3 sentence description of this specific POI and why it's relevant."
        }
        // ... include several relevant POIs ...
      ]
      // Optionally, if interests suggest other categories like 'restaurants' or 'bars',
      // you can include them as separate arrays similar to 'points_of_interest'.
    }
`, cityName, strings.Join(interestNames, ", "), tagsPromptPart,
		searchProfile.SearchRadiusKm, searchProfile.PreferredTime, searchProfile.BudgetLevel,
		searchProfile.PreferOutdoorSeating, searchProfile.PreferDogFriendly, strings.Join(searchProfile.DietaryNeeds, ", "),
		searchProfile.PreferredPace, searchProfile.PreferAccessiblePOIs, strings.Join(searchProfile.PreferredVibes, ", "),
		searchProfile.PreferredTransport,
		cityName, // For the overall_description part
	)

	config := &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}
	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	if err != nil {
		log.Fatal(err)
	}
	var txt string
	for _, candidate := range response.Candidates {
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			log.Println("Candidate has no content or parts.")
			continue
		}

		part := candidate.Content.Parts[0]
		txt = part.Text
		// fmt.Printf("Part text: [%s]\n", txt)
		// if txt != "" {
		// 	log.Printf("Extracted text: [%s]\n", txt)
		// 	type POI struct {
		// 		Name      string  `json:"name"`
		// 		Latitude  float64 `json:"latitude"`
		// 		Longitude float64 `json:"longitude"`
		// 		Category  string  `json:"category"`
		// 	}
		// 	var pois []POI

		// 	if err := json.Unmarshal([]byte(txt), &pois); err != nil {
		// 		log.Printf("Failed to unmarshal AI response text into POIs: %v. Text was: %s\n", err, txt)
		// 	} else {
		// 		fmt.Println("POIs (successfully unmarshalled):", pois)
		// 	}
		// } else {
		// 	log.Println("Part's text was empty.")
		// }
	}

	jsonStr := txt
	jsonStr = strings.TrimPrefix(jsonStr, "```json")
	jsonStr = strings.TrimSuffix(jsonStr, "```")
	jsonStr = strings.TrimSpace(jsonStr)

	if !strings.HasPrefix(jsonStr, "{") || !strings.HasSuffix(jsonStr, "}") {
		l.logger.ErrorContext(ctx, "Extracted text does not appear to be a valid JSON object after trimming", slog.String("trimmed_text_snippet", TruncateString(jsonStr, 200)))
		return nil, fmt.Errorf("AI response was not a valid JSON object after cleaning attempts")
	}

	var itineraryData AIItineraryResponse // Defined above or in types package
	if err := json.Unmarshal([]byte(jsonStr), &itineraryData); err != nil {
		l.logger.ErrorContext(ctx, "Failed to unmarshal AI JSON response", slog.String("json_string_snippet", TruncateString(jsonStr, 200)), slog.Any("error", err))
		return nil, fmt.Errorf("failed to parse AI response JSON: %w", err)
	}

	l.logger.InfoContext(ctx, "Successfully parsed AI itinerary response",
		slog.String("itinerary_name", itineraryData.ItineraryName),
		slog.Int("poi_count", len(itineraryData.PointsOfInterest)))

	// TODO Validate and store the itineraryData in DB
	// 1. Validate the contents of itineraryData (e.g., are latitudes/longitudes valid numbers?)
	// 2. For each POIDetail in itineraryData.PointsOfInterest:
	//    a. Check if it exists in your database (by name and city).
	//    b. If not, insert it (this is how you "seed" your DB).
	//    c. If it exists, you might update its ai_summary or other AI-derived fields.
	// 3. Store the overall itinerary structure (itinerary_name, overall_description, and links to POIs)
	//    in your `itineraries` and `itinerary_pois` tables.
	// 4. Store the PoI details in the `pois` table if they don't exist.
	// 5. Return the itineraryData or a success message.

	return &itineraryData, nil
}

func TruncateString(str string, num int) string {
	if len(str) > num {
		return str[0:num] + "..."
	}
	return str
}
