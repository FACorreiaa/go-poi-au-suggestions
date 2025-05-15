package llmInteraction

import (
	"encoding/json"
	"fmt"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

func parseCityData(jsonStr string) (string, string, string, error) {
	var cityData struct {
		City        string `json:"city"`
		Country     string `json:"country"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &cityData); err != nil {
		return "", "", "", fmt.Errorf("failed to parse city data JSON: %w", err)
	}
	return cityData.City, cityData.Country, cityData.Description, nil
}

func parsePOIs(jsonStr string) ([]types.POIDetail, error) {
	var poiData struct {
		PointsOfInterest []types.POIDetail `json:"points_of_interest"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &poiData); err != nil {
		return nil, fmt.Errorf("failed to parse POI JSON: %w", err)
	}
	return poiData.PointsOfInterest, nil
}

func parseItinerary(jsonStr string) (string, string, []types.POIDetail, error) {
	var itineraryData struct {
		ItineraryName      string            `json:"itinerary_name"`
		OverallDescription string            `json:"overall_description"`
		PointsOfInterest   []types.POIDetail `json:"points_of_interest"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &itineraryData); err != nil {
		return "", "", nil, fmt.Errorf("failed to parse itinerary JSON: %w", err)
	}
	return itineraryData.ItineraryName, itineraryData.OverallDescription, itineraryData.PointsOfInterest, nil
}
