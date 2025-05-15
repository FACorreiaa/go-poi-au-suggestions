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
	City        string `json:"city"`
	Country     string `json:"country"`
	Description string `json:"description"`
}

type AiCityResponse struct {
	GeneralCityData     GeneralCityData     `json:"general_city_data"`
	PointsOfInterest    []POIDetail         `json:"points_of_interest"`
	AIItineraryResponse AIItineraryResponse `json:"itinerary_response"`
}

type GenAIResponse struct {
	City                 string
	Country              string
	CityDescription      string
	ItineraryName        string
	ItineraryDescription string
	GeneralPOI           []POIDetail
	PersonalisedPOI      []POIDetail
	Err                  error
}
