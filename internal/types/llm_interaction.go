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
	//You might add other dynamic categories here if the AI generates them
	//For example:
	Restaurants []POIDetail `json:"restaurants,omitempty"`
	Bars        []POIDetail `json:"bars,omitempty"`
}
