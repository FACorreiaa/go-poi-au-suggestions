package types

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/genai"
)

type LlmInteraction struct {
	ID               uuid.UUID       `json:"id"`
	SessionID        uuid.UUID       `json:"session_id"`
	UserID           uuid.UUID       `json:"user_id"`
	ProfileID        uuid.UUID       `json:"profile_id"`
	CityName         string          `json:"city_name,omitempty"` // The city context for this interaction
	Prompt           string          `json:"prompt"`
	RequestPayload   json.RawMessage `json:"request_payload"`
	ResponseText     string          `json:"response_text"`
	ResponsePayload  json.RawMessage `json:"response_payload"`
	ModelUsed        string          `json:"model_used"`
	PromptTokens     int             `json:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens"`
	TotalTokens      int             `json:"total_tokens"`
	LatencyMs        int             `json:"latency_ms"`
	Timestamp        time.Time       `json:"timestamp"`
}

type AIItineraryResponse struct {
	ItineraryName      string            `json:"itinerary_name"`
	OverallDescription string            `json:"overall_description"`
	PointsOfInterest   []POIDetailedInfo `json:"points_of_interest"`
	Restaurants        []POIDetailedInfo `json:"restaurants,omitempty"`
	Bars               []POIDetailedInfo `json:"bars,omitempty"`
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
	PointsOfInterest    []POIDetailedInfo   `json:"points_of_interest"`
	AIItineraryResponse AIItineraryResponse `json:"itinerary_response"`
	SessionID           uuid.UUID           `json:"session_id"`
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
	GeneralPOI           []POIDetailedInfo `json:"general_poi,omitempty"`
	PersonalisedPOI      []POIDetailedInfo `json:"personalised_poi,omitempty"` // Consider changing to []PersonalizedPOIDetail
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

type UserLocation struct {
	UserLat        float64 `json:"user_lat"`
	UserLon        float64 `json:"user_lon"`
	SearchRadiusKm float64 // Radius in kilometers for searching nearby POIs
}

type UserSavedItinerary struct {
	ID                     uuid.UUID      `json:"id"`
	UserID                 uuid.UUID      `json:"user_id"`
	SourceLlmInteractionID uuid.UUID      `json:"source_llm_interaction_id,omitempty"` // Nullable UUID for the source LLM interaction
	PrimaryCityID          uuid.UUID      `json:"primary_city_id,omitempty"`           // Nullable UUID for the primary city
	Title                  string         `json:"title"`
	Description            sql.NullString `json:"description"`             // Use sql.NullString for nullable text fields
	MarkdownContent        string         `json:"markdown_content"`        // Markdown content for the itinerary
	Tags                   []string       `json:"tags"`                    // Tags for the itinerary
	EstimatedDurationDays  sql.NullInt32  `json:"estimated_duration_days"` // Nullable int32 for estimated duration in days
	EstimatedCostLevel     sql.NullInt32  `json:"estimated_cost_level"`    // Nullable int32 for estimated cost level
	IsPublic               bool           `json:"is_public"`               // Indicates if the itinerary is public
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
}

type UpdateItineraryRequest struct {
	Title                 *string  `json:"title,omitempty"`
	Description           *string  `json:"description,omitempty"` // If nil, means no change. If empty string, means clear description.
	Tags                  []string `json:"tags,omitempty"`        // If nil, no change. If empty slice, clear tags.
	EstimatedDurationDays *int32   `json:"estimated_duration_days,omitempty"`
	EstimatedCostLevel    *int32   `json:"estimated_cost_level,omitempty"`
	IsPublic              *bool    `json:"is_public,omitempty"`
	MarkdownContent       *string  `json:"markdown_content,omitempty"`
}

type PaginatedUserItinerariesResponse struct {
	Itineraries  []UserSavedItinerary `json:"itineraries"`
	TotalRecords int                  `json:"total_records"`
	Page         int                  `json:"page"`
	PageSize     int                  `json:"page_size"`
}

type BookmarkRequest struct {
	LlmInteractionID uuid.UUID `json:"llm_interaction_id"`
	PrimaryCityID    uuid.UUID `json:"primary_city_id"`
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
	CityName  string  `json:"city_name"` // e.g., "New York"
	Latitude  float64 `json:"latitude"`  // e.g., 40.7128
	Longitude float64 `json:"longitude"` // e.g., -74.0060
}

type POIFilter struct {
	Location GeoPoint `json:"location"` // e.g., "restaurant", "hotel", "bar"
	Radius   float64  `json:"radius"`   // Radius in kilometers for filtering POIs
	Category string   `json:"category"` // e.g., "restaurant", "hotel", "bar"
}

type GeoPoint struct {
	Latitude  float64 `json:"latitude"`  // Latitude of the point
	Longitude float64 `json:"longitude"` // Longitude of the point
}
type SearchPOIRequest struct {
	Query      string   `json:"query"` // The search query text
	CityName   string   `json:"city"`
	Latitude   float64  `json:"lat"`
	Longitude  float64  `json:"lon"`
	RadiusKm   float64  `json:"radius_km"`   // Optional, for filtering POIs within a certain radius
	SearchText string   `json:"search_text"` // Optional, for searching by name or description
	SearchTags []string `json:"search_tags"` // Optional, for filtering by tags
	SearchType string   `json:"search_type"` // Optional, e.g., "restaurant", "hotel", "bar"
	SortBy     string   `json:"sort_by"`     // Optional, e.g., "rating", "distance"
	SortOrder  string   `json:"sort_order"`  // Optional, e.g., "asc", "desc"
	MinRating  float64  `json:"min_rating"`  // Optional, for filtering by minimum rating
	MinPrice   string   `json:"min_price"`   // Optional, for filtering by minimum price range
	MinGuests  int32    `json:"min_guests"`  // Optional, for filtering by minimum number of guests (for restaurants)
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

// Context-aware chat types
type ChatContextType string

const (
	ContextHotels      ChatContextType = "hotels"
	ContextRestaurants ChatContextType = "restaurants"
	ContextItineraries ChatContextType = "itineraries"
	ContextGeneral     ChatContextType = "general"
)

type StartChatRequest struct {
	CityName       string          `json:"city_name"`
	ContextType    ChatContextType `json:"context_type"`
	InitialMessage string          `json:"initial_message,omitempty"`
}

type ContinueChatRequest struct {
	Message     string          `json:"message"`
	CityName    string          `json:"city_name,omitempty"`
	ContextType ChatContextType `json:"context_type"`
}

//

type SimpleIntentClassifier struct{}

func (c *SimpleIntentClassifier) Classify(ctx context.Context, message string) (IntentType, error) {
	message = strings.ToLower(message)
	if matched, _ := regexp.MatchString(`add|include|visit`, message); matched {
		return IntentAddPOI, nil
	} else if matched, _ := regexp.MatchString(`remove|delete|skip`, message); matched {
		return IntentRemovePOI, nil
	} else if matched, _ := regexp.MatchString(`what|where|how|why|when`, message); matched {
		return IntentAskQuestion, nil
	}
	return IntentModifyItinerary, nil // Default intent
}

// DomainDetector detects the primary domain from user queries
type DomainDetector struct{}

func (d *DomainDetector) DetectDomain(ctx context.Context, message string) DomainType {
	message = strings.ToLower(message)

	// Accommodation domain keywords
	if matched, _ := regexp.MatchString(`hotel|hostel|accommodation|stay|sleep|room|booking|airbnb|lodge|resort|guesthouse`, message); matched {
		return DomainAccommodation
	}

	// Dining domain keywords
	if matched, _ := regexp.MatchString(`restaurant|food|eat|dine|meal|cuisine|drink|cafe|bar|lunch|dinner|breakfast|brunch`, message); matched {
		return DomainDining
	}

	// Activity domain keywords
	if matched, _ := regexp.MatchString(`activity|museum|park|attraction|tour|visit|see|do|experience|adventure|shopping|nightlife`, message); matched {
		return DomainActivities
	}

	// Itinerary domain keywords
	if matched, _ := regexp.MatchString(`itinerary|plan|schedule|trip|day|week|journey|route|organize|arrange`, message); matched {
		return DomainItinerary
	}

	// Default to general domain
	return DomainGeneral
}

// RecentInteraction represents a recent user interaction with cities and POIs
type RecentInteraction struct {
	ID           uuid.UUID                `json:"id"`
	UserID       uuid.UUID                `json:"user_id"`
	CityName     string                   `json:"city_name"`
	CityID       *uuid.UUID               `json:"city_id,omitempty"`
	Prompt       string                   `json:"prompt"`
	ResponseText string                   `json:"response_text,omitempty"`
	ModelUsed    string                   `json:"model_used"`
	LatencyMs    int                      `json:"latency_ms"`
	CreatedAt    time.Time                `json:"created_at"`
	POIs         []POIDetailedInfo        `json:"pois,omitempty"`
	Hotels       []HotelDetailedInfo      `json:"hotels,omitempty"`
	Restaurants  []RestaurantDetailedInfo `json:"restaurants,omitempty"`
}

// RecentInteractionsResponse groups interactions by city
type RecentInteractionsResponse struct {
	Cities []CityInteractions `json:"cities"`
	Total  int                `json:"total"`
}

// CityInteractions groups interactions for a specific city
type CityInteractions struct {
	CityName     string              `json:"city_name"`
	CityID       *uuid.UUID          `json:"city_id,omitempty"`
	Interactions []RecentInteraction `json:"interactions"`
	POICount     int                 `json:"poi_count"`
	LastActivity time.Time           `json:"last_activity"`
}
