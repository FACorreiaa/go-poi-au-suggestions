package llmChat

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func generatePOICacheKey(city string, lat, lon, distance float64, userID uuid.UUID) string {
	return fmt.Sprintf("poi:%s:%f:%f:%f:%s", city, lat, lon, distance, userID.String())
}

func generateHotelCacheKey(city string, lat, lon float64, userID uuid.UUID) string {
	return fmt.Sprintf("hotel:%s:%.6f:%.6f:%s", city, lat, lon, userID.String())
}

func generateRestaurantCacheKey(city string, lat, lon float64, userID uuid.UUID) string {
	return fmt.Sprintf("restaurant:%s:%.6f:%.6f:%s", city, lat, lon, userID.String())
}

func cleanJSONResponse(response string) string {
	// Remove ```json and ``` markers
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}

	if strings.HasSuffix(response, "```") {
		response = strings.TrimSuffix(response, "```")
	}

	return strings.TrimSpace(response)
}

// extractPOIName extracts the full POI name from the message
func extractPOIName(message string) string {
	// Remove common words and keep the rest as the POI name
	words := strings.Fields(strings.ToLower(message))
	filtered := []string{}
	stopWords := map[string]bool{
		"add": true, "remove": true, "to": true, "from": true, "my": true,
		"itinerary": true, "with": true, "replace": true, "the": true, "in": true,
	}
	for _, w := range words {
		if !stopWords[w] {
			filtered = append(filtered, w)
		}
	}
	if len(filtered) == 0 {
		return "Unknown POI"
	}
	// Capitalize each word for proper formatting
	// cases.Title
	// use this https://pkg.go.dev/golang.org/x/text/cases later and handle language as well
	return strings.Title(strings.Join(filtered, " "))
}
