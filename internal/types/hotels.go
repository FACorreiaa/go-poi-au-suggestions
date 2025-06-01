package types

import "github.com/google/uuid"

// HotelSearchParameters defines the filters for searching hotels
type HotelSearchParameters struct {
	City             string    `json:"city"`      // Required for context, even with lat/lon
	Latitude         float64   `json:"latitude"`  // Center point of search
	Longitude        float64   `json:"longitude"` // Center point of search
	RadiusKm         float64   `json:"radius_km"` // Search radius in kilometers
	MinRating        *float64  `json:"min_rating,omitempty"`
	PriceRanges      []string  `json:"price_ranges,omitempty"` // e.g., ["$", "$$", "$$$"]
	Categories       []string  `json:"categories,omitempty"`   // e.g., ["Hotel", "Boutique Hotel", "Hostel"]
	Amenities        []string  `json:"amenities,omitempty"`    // e.g., ["wifi", "pool", "gym"]
	Page             int       `json:"page,omitempty"`         // For pagination
	PageSize         int       `json:"page_size,omitempty"`    // For pagination
	LlmInteractionID uuid.UUID `json:"llm_interaction_id"`     // For interaction id

}

// RestaurantSearchParameters defines the filters for searching restaurants
type RestaurantSearchParameters struct {
	City             string    `json:"city"` // Required
	Latitude         float64   `json:"latitude"`
	Longitude        float64   `json:"longitude"`
	RadiusKm         float64   `json:"radius_km"`
	MinRating        *float64  `json:"min_rating,omitempty"`
	PriceRanges      []string  `json:"price_ranges,omitempty"`
	Cuisines         []string  `json:"cuisines,omitempty"`   // e.g., ["Italian", "Vegan", "Sushi"]
	Categories       []string  `json:"categories,omitempty"` // e.g., ["Restaurant", "Cafe", "Bar"]
	Features         []string  `json:"features,omitempty"`   // e.g., ["outdoor_seating", "dog_friendly", "live_music"]
	OpenNow          *bool     `json:"open_now,omitempty"`   // If true, filter by currently open
	Page             int       `json:"page,omitempty"`
	PageSize         int       `json:"page_size,omitempty"`
	LlmInteractionID uuid.UUID `json:"llm_interaction_id"` // For interaction id

}

// PaginatedHotelResponse holds a list of hotels and pagination info
type PaginatedHotelResponse struct {
	Hotels       []HotelDetailedInfo `json:"hotels"`
	TotalRecords int                 `json:"total_records"`
	Page         int                 `json:"page"`
	PageSize     int                 `json:"page_size"`
}

// PaginatedRestaurantResponse holds a list of restaurants and pagination info
type PaginatedRestaurantResponse struct {
	Restaurants  []RestaurantDetailedInfo `json:"restaurants"`
	TotalRecords int                      `json:"total_records"`
	Page         int                      `json:"page"`
	PageSize     int                      `json:"page_size"`
}
