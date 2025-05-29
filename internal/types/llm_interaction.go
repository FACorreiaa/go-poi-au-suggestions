package types

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"google.golang.org/genai"
)

type LlmInteraction struct {
	ID               uuid.UUID       `json:"id"`
	SessionID        uuid.UUID       `json:"session_id"`
	UserID           uuid.UUID       `json:"user_id"`
	ProfileID        uuid.UUID       `json:"profile_id"`
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
	SessionID           string              `json:"session_id"`
}

type GenAIResponse struct {
	SessionID            string            `json:"session_id"`
	LlmInteractionID     uuid.UUID         `json:"llm_interaction_id"`
	City                 string            `json:"city,omitempty"`
	Country              string            `json:"country,omitempty"`
	StateProvince        string            `json:"state_province,omitempty"` // New
	CityDescription      string            `json:"city_description,omitempty"`
	Latitude             float64           `json:"latitude,omitempty"`  // New: for city center
	Longitude            float64           `json:"longitude,omitempty"` // New: for city center
	ItineraryName        string            `json:"itinerary_name,omitempty"`
	ItineraryDescription string            `json:"itinerary_description,omitempty"`
	GeneralPOI           []POIDetail       `json:"general_poi,omitempty"`
	PersonalisedPOI      []POIDetail       `json:"personalised_poi,omitempty"` // Consider changing to []PersonalizedPOIDetail
	POIDetailedInfo      []POIDetailedInfo `json:"poi_detailed_info,omitempty"`
	Err                  error             `json:"-"`
}

type AIRequestPayloadForLog struct {
	ModelName        string                       `json:"model_name"`
	GenerationConfig *genai.GenerateContentConfig `json:"generation_config,omitempty"`
	Content          *genai.Content               `json:"content"` // The actual content sent (prompt)
	// You could add other things like "tools" if you use function calling
}

type ChatTurn struct { // You might not need this explicit struct if directly using []*genai.Content
	Role  string       `json:"role"` // "user" or "model"
	Parts []genai.Part `json:"parts"`
}

type ChatSession struct {
	History             []*ChatTurn  // If you want to store a serializable version
	InternalChatSession *genai.Chats // Holds the live SDK chat session
	LastUpdatedAt       time.Time
}

type UserLocation struct {
	UserLat        float64
	UserLon        float64
	SearchRadiusKm float64 // Radius in kilometers for searching nearby POIs
}

type UserSavedItinerary struct {
	ID                     uuid.UUID
	UserID                 uuid.UUID
	SourceLlmInteractionID uuid.NullUUID // or uuid.UUID if always present for a bookmark
	PrimaryCityID          uuid.NullUUID
	Title                  string
	Description            sql.NullString
	MarkdownContent        string
	Tags                   []string // pgx handles TEXT[] as []string
	EstimatedDurationDays  sql.NullInt32
	EstimatedCostLevel     sql.NullInt32
	IsPublic               bool
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type BookmarkRequest struct {
	LlmInteractionID uuid.UUID `json:"llm_interaction_id"`
	Title            string    `json:"title"`
	Description      *string   `json:"description"` // Optional
	Tags             []string  `json:"tags"`        // Optional
	IsPublic         *bool     `json:"is_public"`   // Optional
}

type ChatMessage struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Timestamp time.Time
	Role      string
	Content   string
}

type POIDetailrequest struct {
	CityName  string  `json:"city"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

type HotelUserPreferences struct {
	NumberOfGuests      int32     `json:"number_of_guests"`
	PreferredCategories string    `json:"preferred_category"`    // e.g., "budget", "luxury"
	PreferredTags       []string  `json:"preferredTags"`         // e.g., ["pet-friendly", "free wifi"]
	MaxPriceRange       string    `json:"preferred_price_range"` // e.g., "$", "$$"
	MinRating           float64   `json:"preferred_rating"`      // e.g., 4.0
	NumberOfNights      int64     `json:"number_of_nights"`
	NumberOfRooms       int32     `json:"number_of_rooms"`
	PreferredCheckIn    time.Time `json:"preferred_check_in"`
	PreferredCheckOut   time.Time `json:"preferred_check_out"`
	SearchRadiusKm      float64   `json:"search_radius_km"` // Optional, for filtering hotels within a certain radius
}

type HotelDetailedInfo struct {
	ID               uuid.UUID `json:"id"`
	City             string    `json:"city"`
	Name             string    `json:"name"`
	Latitude         float64   `json:"latitude"`
	Longitude        float64   `json:"longitude"`
	Category         string    `json:"category"` // e.g., "Hotel", "Hostel"
	Description      string    `json:"description"`
	Address          string    `json:"address"`
	PhoneNumber      *string   `json:"phone_number"`
	Website          *string   `json:"website"`
	OpeningHours     *string   `json:"opening_hours"`
	PriceRange       *string   `json:"price_range"`
	Rating           float64   `json:"rating"`
	Tags             []string  `json:"tags"`
	Images           []string  `json:"images"`
	LlmInteractionID uuid.UUID `json:"llm_interaction_id"`
	Err              error     `json:"-"` // Not serialized
}

type HotelPreferenceRequest struct {
	City        string               `json:"city"`
	Lat         float64              `json:"lat"`
	Lon         float64              `json:"lon"`
	Preferences HotelUserPreferences `json:"preferences"`
	Distance    float64              `json:"distance"` // Optional, for filtering hotels within a certain radius
}

type RestaurantUserPreferences struct {
	PreferredCuisine    string
	PreferredPriceRange string
	DietaryRestrictions string
	Ambiance            string
	SpecialFeatures     string
}

type RestaurantDetailedInfo struct {
	ID               uuid.UUID `json:"id"`
	City             string    `json:"city"`
	Name             string    `json:"name"`
	Latitude         float64   `json:"latitude"`
	Longitude        float64   `json:"longitude"`
	Category         string    `json:"category"`
	Description      string    `json:"description"`
	Address          *string   `json:"address"`
	Website          *string   `json:"website"`
	PhoneNumber      *string   `json:"phone_number"`
	OpeningHours     *string   `json:"opening_hours"`
	PriceLevel       *string   `json:"price_level"`  // Changed to *string
	CuisineType      *string   `json:"cuisine_type"` // Changed to *string
	Tags             []string  `json:"tags"`
	Images           []string  `json:"images"`
	Rating           float64   `json:"rating"`
	LlmInteractionID uuid.UUID `json:"llm_interaction_id"`
	Err              error     `json:"-"`
}
