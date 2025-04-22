package models

import (
	"time"

	"github.com/google/uuid"
)

// POI represents a point of interest in the system
type POI struct {
	Base
	Name        string     `json:"name" db:"name"`
	Description string     `json:"description" db:"description"`
	Address     string     `json:"address" db:"address"`
	City        string     `json:"city" db:"city"`
	Country     string     `json:"country" db:"country"`
	Latitude    float64    `json:"latitude" db:"latitude"`
	Longitude   float64    `json:"longitude" db:"longitude"`
	Category    string     `json:"category" db:"category"`
	Tags        []string   `json:"tags" db:"tags,array"`
	ImageURL    string     `json:"image_url" db:"image_url"`
	Website     string     `json:"website" db:"website"`
	Phone       string     `json:"phone" db:"phone"`
	OpeningHours string     `json:"opening_hours" db:"opening_hours"`
	PriceLevel  int        `json:"price_level" db:"price_level"`
	Source      POISource  `json:"source" db:"source"`
	ExternalID  string     `json:"external_id" db:"external_id"`
	AvgRating   float64    `json:"avg_rating" db:"avg_rating"`
	RatingCount int        `json:"rating_count" db:"rating_count"`
	IsActive    bool       `json:"is_active" db:"is_active"`
	VerifiedAt  *time.Time `json:"verified_at" db:"verified_at"`
}

// POIInterest represents a many-to-many relationship between POIs and interests
type POIInterest struct {
	POIID      uuid.UUID `json:"poi_id" db:"poi_id"`
	InterestID uuid.UUID `json:"interest_id" db:"interest_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// NewPOI creates a new POI with default values
func NewPOI(name, description, city, country string, latitude, longitude float64, category string, source POISource) *POI {
	return &POI{
		Base: Base{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:        name,
		Description: description,
		City:        city,
		Country:     country,
		Latitude:    latitude,
		Longitude:   longitude,
		Category:    category,
		Source:      source,
		IsActive:    true,
		AvgRating:   0,
		RatingCount: 0,
	}
}

// NewPOIInterest creates a new POI interest relationship
func NewPOIInterest(poiID, interestID uuid.UUID) *POIInterest {
	return &POIInterest{
		POIID:      poiID,
		InterestID: interestID,
		CreatedAt:  time.Now(),
	}
}