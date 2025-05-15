package llmInteraction

import (
	"fmt"
	"strings"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

func GetUserPreferencesPrompt(searchProfile *types.UserPreferenceProfileResponse) string {
	return fmt.Sprintf(`
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
}

func GetCityDescriptionPrompt(cityName string) string {
	return fmt.Sprintf(`
            Generate the country this city belongs to and a brief description of %s.
            Return the response STRICTLY as a JSON object with:
            {
            "city": "%s",
            "country": "the country of the city",
            "description": "A brief description of the city, including its history and main attractions."
            }`, cityName, cityName)
}

func GetGeneralPOI(cityName string) string {
	return fmt.Sprintf(`
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
}

func GetPersonalizedPOI(interestNames []string, cityName, tagsPromptPart, userPrefs string) string {
	return fmt.Sprintf(`
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
}
