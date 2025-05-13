package types

import "github.com/google/uuid"

// CityDetail matches the cities table structure.
type CityDetail struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Country       string    `json:"country"`
	StateProvince string    `json:"state_province"`
	AiSummary     string    `json:"ai_summary"`
}
