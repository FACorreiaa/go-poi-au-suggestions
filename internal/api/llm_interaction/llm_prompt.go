package llmInteraction

import (
	"fmt"
	"strings"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

func getUserPreferencesPrompt(searchProfile *types.UserPreferenceProfileResponse) string {
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

func getCityDescriptionPrompt(cityName string) string {
	return fmt.Sprintf(`
        Provide detailed information for the city: '%s'.
        Return the response STRICTLY as a JSON object with the following keys:
        {
            "city_name": "The official or commonly accepted name of the city",
            "state_province": "The state, province, or region the city belongs to (if applicable, otherwise null or empty string)",
            "country": "The country where the city is located",
            "center_latitude": <float, the latitude of the city's approximate center>,
            "center_longitude": <float, the longitude of the city's approximate center>,
            "description": "A brief description of the city, including its history, main attractions, and culture."
        }
        If state/province is not applicable (e.g., for a city-state or a country's capital that isn't part of a larger state), return null or an empty string for "state_province".
        Ensure latitude is between -90 and 90, and longitude is between -180 and 180.
    `, cityName)
}

// Population      string  `json:"population"`
// 	Area            string  `json:"area"`
// 	Timezone        string  `json:"timezone"`
// 	Language        string  `json:"language"`
// 	Weather         string  `json:"weather"`

func getGeneralPOI(cityName string) string {
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

func getPersonalizedPOI(interestNames []string, cityName, tagsPromptPart, userPrefs string) string {
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

func getPOIDetailsPrompt(city string, lat, lon float64) string {
	return fmt.Sprintf(`
		Generate details for the following POI on the city of %s with the coordinates %0.2f , %0.2f.
		The result should be in the following JSON format:
		{
			"name": "Name of the Point of Interest",
			"description": "Detailed description of the POI and why it's relevant to the user's interest.",
    		"address": "address of the point of interest",
    		"website": "website of the POI if available",
    		"phone_number": "phone number of the POI if available",
    		"opening_hours": "JSONB, -- Store opening hours structured (e.g., OSM opening_hours format or custom JSON)"
    		"price_range": "price level if available",
            "category": "Primary category (e.g., Museum, Historical Site, Park, Restaurant, Bar)",
            "tags": ["tag1", "tag2", ...], -- Tags related to the POI
    "        "images": ["image_url_1", "image_url_2", ...], // images from wikipedia or pininterest
		}
	`, city, lat, lon)
}

// TODO
func getHotelsByPreferences(city string, lat, lon float64, userPreferences string) string {
	return fmt.Sprintf(`
        Generate a list of maximum 10 hotels in the city of %s, near the coordinates %0.2f , %0.2f.
        The result should be tailored to the user's preferences:
        %s
        The result should be in the following JSON format:
        {
            "hotels": [
                {
                    "name": "Name of the Hotel",
                    "latitude": <float>,
                    "longitude": <float>,
                    "category": "Primary category (e.g., Hotel, Hostel, Guesthouse)",
                    "description": "A brief description of this hotel and why it's relevant to the user's interest."
                }
            ]
        }
    `, city, lat, lon, userPreferences)
}

// TODO
func getHotelsNeaby(city string, lat, lon float64) string {
	return fmt.Sprintf(`
        Generate a list of maximum 10 hotels nearby the coordinates %0.2f , %0.2f in the city of %s.
        The result should be in the following JSON format:
        {
            "hotels": [
                {
                    "name": "Name of the Hotel",
                    "latitude": <float>,
                    "longitude": <float>,
                    "category": "Primary category (e.g., Hotel, Hostel, Guesthouse)",
                    "description": "A brief description of this hotel and why it's relevant to the user's interest."
                }
            ]
        }
    `, lat, lon, city)
}

// TODO
func getRestaurantsByPreferences(city string, lat, lon float64, userPreferences string) string {
	return fmt.Sprintf(`
        Generate a list of maximum 10 restaurants in the city of %s, near the coordinates %0.2f , %0.2f.
        The result should be tailored to the user's preferences:
        %s
        The result should be in the following JSON format:
        {
            "restaurants": [
                {
                    "name": "Name of the Restaurant",
                    "latitude": <float>,
                    "longitude": <float>,
                    "category": "Primary category (e.g., Restaurant, Bar, Cafe)",
                    "description": "A brief description of this restaurant and why it's relevant to the user's interest."
                }
            ]
        }
    `, city, lat, lon, userPreferences)
}

// TODO
func getRestarantsNearby(city string, lat, lon float64) string {
	return fmt.Sprintf(`
        Generate a list of maximum 10 restaurants nearby the coordinates %0.2f , %0.2f in the city of %s.
        The result should be in the following JSON format:
        {
            "restaurants": [
                {
                    "name": "Name of the Restaurant",
                    "latitude": <float>,
                    "longitude": <float>,
                    "category": "Primary category (e.g., Restaurant, Bar, Cafe)",
                    "description": "A brief description of this restaurant and why it's relevant to the user's interest."
                }
            ]
        }
    `, lat, lon, city)
}

// TOOD
func getRestaurantDetails(city string, lat, lon float64) string {
	return fmt.Sprintf(`
        Generate details for the following restaurant on the city of %s with the coordinates %0.2f , %0.2f.
        The result should be in the following JSON format:
        {
            "name": "Name of the Restaurant",
            "description": "Detailed description of the restaurant and why it's relevant to the user's interest.",
    		"address": "address of the restaurant",
    		"website": "website of the restaurant if available",
    		"phone_number": "phone number of the restaurant if available",
    		"opening_hours": "JSONB, -- Store opening hours structured (e.g., OSM opening_hours format or custom JSON)"
    		"price_level": "price level if available",
    		"cuisine_type": "Type of cuisine offered by the restaurant"
        }
    `, city, lat, lon)
}

// TODO
// GetPOIReviews TODO build the POI reviews method for the user to have reviews of the POI
func getPOIReviews(city string, lat, lon float64) string {
	return fmt.Sprintf(`
        Generate reviews for the following POI on the city of %s with the coordinates %0.2f , %0.2f.
        The result should be in the following JSON format:
        {
            "reviews": [
                {
                    "author": "Name of the reviewer",
                    "rating": <float, rating from 1 to 5>,
                    "comment": "Review comment from the user"
                }
            ]
        }
    `, city, lat, lon)
}

// TODO
func getPOIRecommendations(city string, lat, lon float64) string {
	return fmt.Sprintf(`
        Generate personalized recommendations for the following POI on the city of %s with the coordinates %0.2f , %0.2f.
        The result should be in the following JSON format:
        {
            "recommendations": [
                {
                    "name": "Name of the recommended Point of Interest",
                    "latitude": <float>,
                    "longitude": <float>,
                    "category": "Primary category (e.g., Museum, Historical Site, Park, Restaurant, Bar)",
                    "reason": "A brief reason why this POI is recommended based on user preferences."
                }
            ]
        }
    `, city, lat, lon)
}
