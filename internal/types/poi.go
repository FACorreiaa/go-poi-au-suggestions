package types

import "github.com/google/uuid"

type POIDetail struct {
	ID uuid.UUID `json:"id"`
	//Description    string    `json:"description"`
	Name           string  `json:"name"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	Category       string  `json:"category"`
	DescriptionPOI string  `json:"description_poi"`
	// Rating               float64   `json:"rating"`
	// Address              string    `json:"address"`
	// PhoneNumber          string    `json:"phone_number"`
	// Website              string    `json:"website"`
	// OpeningHours         string    `json:"opening_hours"`
	// Images               []string  `json:"images"`
	// Reviews              []string  `json:"reviews"`
	// PriceRange           string    `json:"price_range"`
	Distance float64 `json:"distance"`
	// DistanceUnit         string    `json:"distance_unit"`
	// DistanceValue        float64   `json:"distance_value"`
	// DistanceText         string    `json:"distance_text"`
	// LocationType         string    `json:"location_type"`
	// LocationID           string    `json:"location_id"`
	// LocationURL          string    `json:"location_url"`
	// LocationRating       float64   `json:"location_rating"`
	// LocationReview       int       `json:"location_review"`
	// LocationAddress      string    `json:"location_address"`
	// LocationPhone        string    `json:"location_phone"`
	// LocationWebsite      string    `json:"location_website"`
	// LocationOpeningHours string    `json:"location_opening_hours"`
}
