package models

import (
	"encoding/json"

	"github.com/google/uuid"
)

// City represents a city in the system
type City struct {
	Base
	Name          string  `json:"name" db:"name"`
	StateProvince string  `json:"state_province" db:"state_province"`
	Country       string  `json:"country" db:"country"`
	CenterLocation string  `json:"center_location" db:"center_location"` // Stored as PostGIS Point
	BoundingBox   string  `json:"bounding_box" db:"bounding_box"`       // Stored as PostGIS Polygon
	AISummary     string  `json:"ai_summary" db:"ai_summary"`
	Embedding     []float64 `json:"embedding" db:"embedding"`           // Vector embedding
}

// PointOfInterest represents a point of interest in the system
type PointOfInterest struct {
	Base
	Name             string          `json:"name" db:"name"`
	Description      string          `json:"description" db:"description"`
	Location         string          `json:"location" db:"location"`           // Stored as PostGIS Point
	CityID           *uuid.UUID      `json:"city_id" db:"city_id"`
	Address          string          `json:"address" db:"address"`
	POIType          string          `json:"poi_type" db:"poi_type"`
	Website          string          `json:"website" db:"website"`
	PhoneNumber      string          `json:"phone_number" db:"phone_number"`
	OpeningHours     json.RawMessage `json:"opening_hours" db:"opening_hours"` // Stored as JSONB
	PriceLevel       int             `json:"price_level" db:"price_level"`
	AverageRating    float64         `json:"average_rating" db:"average_rating"`
	RatingCount      int             `json:"rating_count" db:"rating_count"`
	Source           POISource       `json:"source" db:"source"`
	SourceID         string          `json:"source_id" db:"source_id"`
	IsVerified       bool            `json:"is_verified" db:"is_verified"`
	IsSponsored      bool            `json:"is_sponsored" db:"is_sponsored"`
	AISummary        string          `json:"ai_summary" db:"ai_summary"`
	Embedding        []float64       `json:"embedding" db:"embedding"`         // Vector embedding
	Tags             []string        `json:"tags" db:"tags"`                   // Array of tags
	AccessibilityInfo string          `json:"accessibility_info" db:"accessibility_info"`
}

// NewCity creates a new city with default values
func NewCity(name, country string) *City {
	return &City{
		Base:    Base{ID: uuid.New()},
		Name:    name,
		Country: country,
	}
}

// NewPointOfInterest creates a new point of interest with default values
func NewPointOfInterest(name, description string, source POISource) *PointOfInterest {
	return &PointOfInterest{
		Base:        Base{ID: uuid.New()},
		Name:        name,
		Description: description,
		Source:      source,
		IsVerified:  false,
		IsSponsored: false,
		RatingCount: 0,
	}
}