package llmInteraction

import (
	"fmt"
	"strings"
)

func generateCityDataPrompt(cityName string) string {
	return fmt.Sprintf(`
        Generate the country this city belongs to and a brief description of %s.
        Return the response STRICTLY as a JSON object with:
        {
        "city": "%s",
        "country": "the country of the city",
        "description": "A brief description of the city, including its history and main attractions."
        }`, cityName, cityName)
}

func generateGeneralPOIPrompt(cityName string) string {
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

func generatePersonalizedItineraryPrompt(cityName string, userData *UserData) string {
	tagsPromptPart := ""
	if len(userData.TagInfoForPrompt) > 0 {
		tagsPromptPart = fmt.Sprintf("\n    - Additionally, consider these specific user tags/preferences: [%s].", strings.Join(userData.TagInfoForPrompt, "; "))
	}
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
        }`, cityName, strings.Join(userData.InterestNames, ", "), tagsPromptPart, userData.UserPrefs, cityName)
}
