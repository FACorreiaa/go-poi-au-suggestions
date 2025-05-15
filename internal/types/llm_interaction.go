package types

import (
	"encoding/json"

	"github.com/google/uuid"
)

type LlmInteraction struct {
	UserID           uuid.UUID       `json:"user_id"`
	Prompt           string          `json:"prompt"`
	RequestPayload   json.RawMessage `json:"request_payload"`
	ResponseText     string          `json:"response_text"`
	ResponsePayload  json.RawMessage `json:"response_payload"`
	ModelUsed        string          `json:"model_used"`
	PromptTokens     int             `json:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens"`
	TotalTokens      int             `json:"total_tokens"`
	LatencyMs        int             `json:"latency_ms"`
}

type AIItineraryResponse struct {
	ItineraryName      string      `json:"itinerary_name"`
	OverallDescription string      `json:"overall_description"`
	PointsOfInterest   []POIDetail `json:"points_of_interest"`
	Restaurants        []POIDetail `json:"restaurants,omitempty"`
	Bars               []POIDetail `json:"bars,omitempty"`
}

type GeneralCityData struct {
	City            string  `json:"city"`
	Country         string  `json:"country"`
	StateProvince   string  `json:"state_province,omitempty"`
	Description     string  `json:"description"`
	CenterLatitude  float64 `json:"center_latitude,omitempty"`
	CenterLongitude float64 `json:"center_longitude,omitempty"`
	Population      string  `json:"population"`
	Area            string  `json:"area"`
	Timezone        string  `json:"timezone"`
	Language        string  `json:"language"`
	Weather         string  `json:"weather"`
	Attractions     string  `json:"attractions"`
	History         string  `json:"history"`
}

type AiCityResponse struct {
	GeneralCityData     GeneralCityData     `json:"general_city_data"`
	PointsOfInterest    []POIDetail         `json:"points_of_interest"`
	AIItineraryResponse AIItineraryResponse `json:"itinerary_response"`
}

type GenAIResponse struct {
	City                 string      `json:"city,omitempty"`
	Country              string      `json:"country,omitempty"`
	StateProvince        string      `json:"state_province,omitempty"` // New
	CityDescription      string      `json:"city_description,omitempty"`
	Latitude             float64     `json:"latitude,omitempty"`  // New: for city center
	Longitude            float64     `json:"longitude,omitempty"` // New: for city center
	ItineraryName        string      `json:"itinerary_name,omitempty"`
	ItineraryDescription string      `json:"itinerary_description,omitempty"`
	GeneralPOI           []POIDetail `json:"general_poi,omitempty"`
	PersonalisedPOI      []POIDetail `json:"personalised_poi,omitempty"` // Consider changing to []PersonalizedPOIDetail
	Err                  error       `json:"-"`
}
