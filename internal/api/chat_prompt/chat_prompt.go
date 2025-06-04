package llmChat

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

func getGeneralPOIByDistance(cityName string, lat, lon, distance float64) string {
	return fmt.Sprintf(`
            Generate a list of maximum 5 general points of interest that people usually see no matter the taste or preference for this city %s.
            The user location is at latitude %0.2f and longitude %0.2f, and the distance to search is %0.2f km.
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
            }`, cityName, lat, lon, distance)
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
            "images": ["image_url_1", "image_url_2", ...], // images from wikipedia or pininterest
            "rating": <float> -- Average rating if available
            "stars": type of stars if available (e.g., "3 stars", "5 stars")

		}
	`, city, lat, lon)
}

func getHotelsByPreferencesPrompt(city string, lat, lon float64, userPreferences types.HotelUserPreferences) string {
	return fmt.Sprintf(`
        Generate a list of maximum 5 hotels in the city of %s, near the coordinates %0.2f , %0.2f.
        The hotels should be relevant to the user's interest.
        The result should be tailored to the user's preferences:
        - Preferred Category: %s
        - Preferred Tags: %s
        - Max Price range: %s
        - Preferred Rating: %0.2f
        - Number of Guests: %d
        - Number of Nights: %d
        - Number of Rooms: %d
        - Preferred Check-In Date: %s
        - Preferred Check-Out Date: %s  
        - Distance: %0.2f km (if provided, otherwise use default radius of 5km)
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
    `, city, lat, lon, userPreferences.PreferredCategories, userPreferences.PreferredTags,
		userPreferences.MaxPriceRange, userPreferences.MinRating,
		userPreferences.NumberOfGuests, userPreferences.NumberOfNights, userPreferences.NumberOfRooms,
		userPreferences.PreferredCheckIn.Format("2006-01-02"), userPreferences.PreferredCheckOut.Format("2006-01-02"),
		userPreferences.SearchRadiusKm)
}

func getHotelsNeabyPrompt(city string, userLocation types.UserLocation) string {
	return fmt.Sprintf(`
        Generate a list of maximum 5 hotels nearby the coordinates %0.2f , %0.2f in the city of %s.
        the hotels can be around %0.2f km radius from the user's location or if nothing provided, use the default radius of 5km.
        The hotels should be relevant to the user's interest.
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
    `, userLocation.UserLat, userLocation.UserLon, city, userLocation.SearchRadiusKm)
}

func getRestaurantsByPreferencesPrompt(city string, lat, lon float64, userPreferences types.RestaurantUserPreferences) string {
	return fmt.Sprintf(`
        Generate a list of up to 10 restaurants in the city of %s, near coordinates %.2f, %.2f.
        Tailor the results to the user's preferences:
        - Preferred Cuisine: %s
        - Preferred Price Range: %s
        - Dietary Restrictions: %s
        - Ambiance: %s
        - Special Features: %s
        The result must be in JSON format:
        {
            "restaurants": [
                {
                    "name": "Restaurant Name",
                    "latitude": <float>,
                    "longitude": <float>,
                    "category": "Restaurant|Bar|Cafe",
                    "description": "Brief description of the restaurant and why it matches preferences."
                }
            ]
        }
    `, city, lat, lon, userPreferences.PreferredCuisine, userPreferences.PreferredPriceRange,
		userPreferences.DietaryRestrictions, userPreferences.Ambiance, userPreferences.SpecialFeatures)
}

func getRestaurantsNearbyPrompt(city string, userLocation types.UserLocation) string {
	if userLocation.SearchRadiusKm == 0 {
		userLocation.SearchRadiusKm = 5.0 // Default radius
	}
	return fmt.Sprintf(`
        Generate a list of up to 10 restaurants in the city of %s, within %.2f km of coordinates %.2f, %.2f.
        Include a variety of restaurant categories to provide diverse options.
        The result must be in JSON format:
        {
            "restaurants": [
                {
                    "name": "Restaurant Name",
                    "latitude": <float>,
                    "longitude": <float>,
                    "category": "Restaurant|Bar|Cafe",
                    "description": "Brief description of the restaurant and its proximity to the user's location."
                }
            ]
        }
    `, city, userLocation.SearchRadiusKm, userLocation.UserLat, userLocation.UserLon)
}

func generatedContinuedConversationPrompt(poi, city string) string {
	return fmt.Sprintf(
		`Provide detailed information about "%s" in %s.
        If user writes "Restaurant" add "cuisine_type" to final response and hide "description_poi"
        If user writes "Hotel" add "star_rating" to final response and hide "description_poi"
		Analise this POI (The user can insert a POI name, a Restaurant name or an Hotel/Hostel name) and return the following JSON structure:
    {
        "name": "string (the POI name)",
        "latitude": number (approximate latitude as float),
        "longitude": number (approximate longitude as float),
        "category": "string (e.g., Museum, Park, Historical Site)",
        "description_poi": "string (50-100 words description)"
        "cuisine_type": "string (for Restaurant)",
        "star_rating": "number (for Hotel/Hostel)"
    }
    
    If the POI is not found, return: {"name": "", "latitude": 0, "longitude": 0, "category": "", "description_poi": ""}`,
		poi, city)
}

// getCityDescriptionPrompt generates a prompt for city data
func getCityDescriptionPrompt(cityName string) string {
	return fmt.Sprintf(`
        Provide detailed information about the city %s in JSON format with the following structure:
        {
            "city_name": "%s",
            "country": "Country name",
            "state_province": "State or province, if applicable",
            "description": "A detailed description of the city",
            "center_latitude": float64,
            "center_longitude": float64
        }
    `, cityName, cityName)
}

// getGeneralPOI generates a prompt for general POIs
func getGeneralPOIPrompt(cityName string) string {
	return fmt.Sprintf(`
        Provide a list of general points of interest for %s in JSON format with the following structure:
        {
            "points_of_interest": [
                {
                    "name": "POI name",
                    "category": "Category (e.g., Historical Site, Museum)",
                    "coordinates": {
                        "latitude": float64,
                        "longitude": float64
                    }
                }
            ]
        }
    `, cityName)
}
