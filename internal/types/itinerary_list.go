package types

import (
	"time"

	"github.com/google/uuid"
)

// ItineraryList represents the top-level list containing itineraries
type ItineraryList struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	Description string
	IsPublic    bool
	CityID      uuid.UUID
	Itineraries []Itinerary
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Itinerary represents a single itinerary (a sub-list)
type Itinerary struct {
	ID          uuid.UUID
	Name        string
	Description string
	POIs        []POI
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// POI represents a point of interest within an itinerary
type POI struct {
	ID          uuid.UUID
	Name        string
	Latitude    float64
	Longitude   float64
	Category    string
	Description string
	Position    int
	Notes       string
}

type List struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Name         string
	Description  string
	ImageURL     string
	IsPublic     bool
	IsItinerary  bool
	ParentListID *uuid.UUID // Nullable, as per schema
	CityID       uuid.UUID
	ViewCount    int
	SaveCount    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ListItem struct {
	ListID    uuid.UUID
	PoiID     uuid.UUID
	Position  int
	Notes     string
	DayNumber *int       // Nullable, as per schema
	TimeSlot  *time.Time // Nullable, as per schema
	Duration  *int       // Nullable, as per schema
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UpdateListRequest struct {
	Name        *string    `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
	Description *string    `json:"description,omitempty" validate:"omitempty,max=500"`
	ImageURL    *string    `json:"image_url,omitempty" validate:"omitempty,url"`
	IsPublic    *bool      `json:"is_public,omitempty"`
	CityID      *uuid.UUID `json:"city_id,omitempty"`
}

type AddListItemRequest struct {
	PoiID                   uuid.UUID  `json:"poi_id" validate:"required"`
	Position                int        `json:"position" validate:"gte=0"`
	Notes                   string     `json:"notes,omitempty" validate:"max=1000"`
	DayNumber               *int       `json:"day_number,omitempty" validate:"omitempty,gt=0"`
	TimeSlot                *time.Time `json:"time_slot,omitempty"`
	DurationMinutes         *int       `json:"duration_minutes,omitempty" validate:"omitempty,gt=0"`
	SourceLlmSuggestedPoiID *uuid.UUID `json:"source_llm_suggested_poi_id,omitempty"`
	ItemAIDescription       string     `json:"item_ai_description,omitempty"`
}

type UpdateListItemRequest struct {
	PoiID                   *uuid.UUID `json:"poi_id,omitempty"`
	Position                *int       `json:"position,omitempty" validate:"omitempty,gte=0"`
	Notes                   *string    `json:"notes,omitempty" validate:"omitempty,max=1000"`
	DayNumber               *int       `json:"day_number,omitempty" validate:"omitempty,gt=0"`
	TimeSlot                *time.Time `json:"time_slot,omitempty"`
	DurationMinutes         *int       `json:"duration_minutes,omitempty" validate:"omitempty,gt=0"`
	SourceLlmSuggestedPoiID *uuid.UUID `json:"source_llm_suggested_poi_id,omitempty"`
	ItemAIDescription       *string    `json:"item_ai_description,omitempty"`
}

// ListWithItems combines a List with its items
type ListWithItems struct {
	List  List
	Items []*ListItem
}

type CreateListRequest struct {
	Name        string     `json:"name" validate:"required,min=3,max=100"`
	Description string     `json:"description,omitempty" validate:"max=500"`
	CityID      *uuid.UUID `json:"city_id,omitempty"` // Optional: if the list/itinerary is city-specific
	IsItinerary bool       `json:"is_itinerary"`      // True if this top-level list IS an itinerary itself
	IsPublic    bool       `json:"is_public"`
}

type CreateItineraryForListRequest struct {
	Name        string `json:"name" validate:"required,min=3,max=100"`
	Description string `json:"description,omitempty" validate:"max=500"`
	IsPublic    bool   `json:"is_public"`
}
