package models

import (
	"time"

	"github.com/google/uuid"
)

// City represents a city in the system
type City struct {
	Base
	Name           string  `json:"name" db:"name"`
	StateProvince  string  `json:"state_province" db:"state_province"`
	Country        string  `json:"country" db:"country"`
	CenterLocation string  `json:"center_location" db:"center_location"` // PostGIS Point geometry
	BoundingBox    string  `json:"bounding_box" db:"bounding_box"`       // PostGIS Polygon geometry
	AISummary      string  `json:"ai_summary" db:"ai_summary"`
	Embedding      []float64 `json:"embedding" db:"embedding,array"`     // Vector embedding for similarity search
}

// CityInterest represents popular interests/categories in a city
type CityInterest struct {
	CityID     uuid.UUID `json:"city_id" db:"city_id"`
	InterestID uuid.UUID `json:"interest_id" db:"interest_id"`
	Rank       int       `json:"rank" db:"rank"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// NewCity creates a new city with default values
func NewCity(name, stateProvince, country string, aiSummary string) *City {
	return &City{
		Base: Base{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:          name,
		StateProvince: stateProvince,
		Country:       country,
		AISummary:     aiSummary,
	}
}

// NewCityInterest creates a new city interest relationship
func NewCityInterest(cityID, interestID uuid.UUID, rank int) *CityInterest {
	now := time.Now()
	return &CityInterest{
		CityID:     cityID,
		InterestID: interestID,
		Rank:       rank,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
