package llmChat

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"
)

func generatePOICacheKey(city string, lat, lon, distance float64, userID uuid.UUID) string {
	return fmt.Sprintf("poi:%s:%f:%f:%f:%s", city, lat, lon, distance, userID.String())
}

func generateFilteredPOICacheKeyWithFilters(lat, lon, distance float64, filters map[string]string, userID uuid.UUID) string {
	// Serialize filters to JSON for consistent cache key
	filtersJSON, _ := json.Marshal(filters)
	return fmt.Sprintf("poi_filtered:%f:%f:%f:%s:%s", lat, lon, distance, userID.String(), string(filtersJSON))
}

func generateFilteredPOICacheKey(lat, lon, distance float64, userID uuid.UUID) string {
	return fmt.Sprintf("poi_filtered:%f:%f:%f:%s", lat, lon, distance, userID.String())
}

func generateHotelCacheKey(city string, lat, lon float64, userID uuid.UUID) string {
	return fmt.Sprintf("hotel:%s:%.6f:%.6f:%s", city, lat, lon, userID.String())
}

func generateRestaurantCacheKey(city string, lat, lon float64, userID uuid.UUID) string {
	return fmt.Sprintf("restaurant:%s:%.6f:%.6f:%s", city, lat, lon, userID.String())
}

func cleanJSONResponse(response string) string {
	response = strings.TrimSpace(response)

	// Remove markdown code block markers
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}

	if strings.HasSuffix(response, "```") {
		response = strings.TrimSuffix(response, "```")
	}

	response = strings.TrimSpace(response)

	// Extract JSON from response that might contain explanatory text
	// Look for the first { and last } to extract the JSON object
	firstBrace := strings.Index(response, "{")
	if firstBrace == -1 {
		return response // No JSON found, return as is
	}

	lastBrace := strings.LastIndex(response, "}")
	if lastBrace == -1 || lastBrace <= firstBrace {
		return response // No valid JSON structure found
	}

	// Extract the JSON portion
	jsonPortion := response[firstBrace : lastBrace+1]
	return strings.TrimSpace(jsonPortion)
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

// enrichLLMPOIsWithMetadata adds city and user context to POI data from LLM
func enrichLLMPOIsWithMetadata(pois []types.POIDetailedInfo, cityID uuid.UUID, cityName string, userID uuid.UUID, llmInteractionID uuid.UUID) []types.POIDetailedInfo {
	enrichedPOIs := make([]types.POIDetailedInfo, len(pois))
	for i, poi := range pois {
		enrichedPOI := poi
		enrichedPOI.City = cityName
		enrichedPOI.LlmInteractionID = llmInteractionID
		// Add ID if not set
		if enrichedPOI.ID == uuid.Nil {
			enrichedPOI.ID = uuid.New()
		}
		enrichedPOIs[i] = enrichedPOI
	}
	return enrichedPOIs
}

// applyClientFilters applies filtering logic to POI slice
func applyClientFilters(pois []types.POIDetailedInfo, filters map[string]string) []types.POIDetailedInfo {
	if len(filters) == 0 {
		return pois
	}

	var filtered []types.POIDetailedInfo
	for _, poi := range pois {
		include := true

		// Category filter
		if category, exists := filters["category"]; exists && category != "" && category != "all" {
			if !strings.EqualFold(poi.Category, category) {
				include = false
			}
		}

		// Price range filter
		if priceRange, exists := filters["price_range"]; exists && priceRange != "" && priceRange != "all" {
			if poi.PriceRange == "" || !strings.EqualFold(poi.PriceRange, priceRange) {
				include = false
			}
		}

		// Min rating filter
		if minRatingStr, exists := filters["min_rating"]; exists && minRatingStr != "" && minRatingStr != "all" {
			if minRating := parseFloat(minRatingStr); minRating > 0 && poi.Rating < minRating {
				include = false
			}
		}

		if include {
			filtered = append(filtered, poi)
		}
	}
	return filtered
}

// parseFloat safely parses a string to float64
func parseFloat(s string) float64 {
	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val
	}
	return 0
}
