package types

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// --- ENUM Types ---

// DayPreference represents the DB ENUM 'day_preference_enum'.
type DayPreference string

const (
	DayPreferenceAny   DayPreference = "any"   // No specific preference
	DayPreferenceDay   DayPreference = "day"   // Primarily daytime activities
	DayPreferenceNight DayPreference = "night" // Primarily evening/night activities
)

// Scan implements the sql.Scanner interface for DayPreference.
func (s *DayPreference) Scan(value interface{}) error {
	strVal, ok := value.(string)
	if !ok {
		bytesVal, ok := value.([]byte) // Sometimes comes as bytes
		if !ok {
			return fmt.Errorf("failed to scan DayPreference: expected string or []byte, got %T", value)
		}
		strVal = string(bytesVal)
	}
	// Validate if the scanned value is one of the known enum values
	switch DayPreference(strVal) {
	case DayPreferenceAny, DayPreferenceDay, DayPreferenceNight:
		*s = DayPreference(strVal)
		return nil
	default:
		return fmt.Errorf("unknown DayPreference value: %s", strVal)
	}
}

// Value implements the driver.Valuer interface for DayPreference.
func (s DayPreference) Value() (driver.Value, error) {
	// Optional validation before saving, though DB constraint should catch it
	switch s {
	case DayPreferenceAny, DayPreferenceDay, DayPreferenceNight:
		return string(s), nil
	default:
		return nil, fmt.Errorf("invalid DayPreference value: %s", s)
	}
}

// SearchPace represents the DB ENUM 'search_pace_enum'.
type SearchPace string

const (
	SearchPaceAny      SearchPace = "any"      // No preference
	SearchPaceRelaxed  SearchPace = "relaxed"  // Fewer, longer activities
	SearchPaceModerate SearchPace = "moderate" // Standard pace
	SearchPaceFast     SearchPace = "fast"     // Pack in many activities
)

// Scan implements the sql.Scanner interface for SearchPace.
func (s *SearchPace) Scan(value interface{}) error {
	strVal, ok := value.(string)
	if !ok {
		bytesVal, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan SearchPace: expected string or []byte, got %T", value)
		}
		strVal = string(bytesVal)
	}
	switch SearchPace(strVal) {
	case SearchPaceAny, SearchPaceRelaxed, SearchPaceModerate, SearchPaceFast:
		*s = SearchPace(strVal)
		return nil
	default:
		return fmt.Errorf("unknown SearchPace value: %s", strVal)
	}
}

// Value implements the driver.Valuer interface for SearchPace.
func (s SearchPace) Value() (driver.Value, error) {
	switch s {
	case SearchPaceAny, SearchPaceRelaxed, SearchPaceModerate, SearchPaceFast:
		return string(s), nil
	default:
		return nil, fmt.Errorf("invalid SearchPace value: %s", s)
	}
}

type TransportPreference string

const (
	TransportPreferenceAny    TransportPreference = "any"
	TransportPreferenceWalk   TransportPreference = "walk"
	TransportPreferencePublic TransportPreference = "public"
	TransportPreferenceCar    TransportPreference = "car"
)

// Scan implements the sql.Scanner interface for SearchPace.
func (s *TransportPreference) Scan(value interface{}) error {
	strVal, ok := value.(string)
	if !ok {
		bytesVal, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan SearchPace: expected string or []byte, got %T", value)
		}
		strVal = string(bytesVal)
	}
	switch TransportPreference(strVal) {
	case TransportPreferenceAny, TransportPreferenceWalk, TransportPreferencePublic, TransportPreferenceCar:
		*s = TransportPreference(strVal)
		return nil
	default:
		return fmt.Errorf("unknown SearchPace value: %s", strVal)
	}
}

// Value implements the driver.Valuer interface for SearchPace.
func (s TransportPreference) Value() (driver.Value, error) {
	switch s {
	case TransportPreferenceAny, TransportPreferenceWalk, TransportPreferencePublic, TransportPreferenceCar:
		return string(s), nil
	default:
		return nil, fmt.Errorf("invalid SearchPace value: %s", s)
	}
}

// UserPreferenceProfileResponse represents a user's saved preference profile.
type UserPreferenceProfileResponse struct {
	ID                   uuid.UUID           `json:"id"`
	UserID               uuid.UUID           `json:"user_id"` // Might omit in some API responses
	ProfileName          string              `json:"profile_name"`
	IsDefault            bool                `json:"is_default"`
	SearchRadiusKm       float64             `json:"search_radius_km"`
	PreferredTime        DayPreference       `json:"preferred_time"`
	BudgetLevel          int                 `json:"budget_level"`
	PreferredPace        SearchPace          `json:"preferred_pace"`
	PreferAccessiblePOIs bool                `json:"prefer_accessible_pois"`
	PreferOutdoorSeating bool                `json:"prefer_outdoor_seating"`
	PreferDogFriendly    bool                `json:"prefer_dog_friendly"`
	PreferredVibes       []string            `json:"preferred_vibes"` // Assuming TEXT[] maps to []string
	PreferredTransport   TransportPreference `json:"preferred_transport"`
	DietaryNeeds         []string            `json:"dietary_needs"` // Assuming TEXT[] maps to []string
	Interests            []*Interest         `json:"interests"`     // Interests linked to this profile
	Tags                 []*Tags             `json:"tags"`          // Tags to avoid for this profile
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
}

// CreateUserPreferenceProfileParams defines required fields for creating a new profile.
// Optional fields can be added here or assumed to use DB defaults.
type CreateUserPreferenceProfileParams struct {
	UserID               string               `json:"user_id" binding:"required,uuid"` // Added for clarity
	ProfileName          string               `json:"profile_name" binding:"required"`
	IsDefault            *bool                `json:"is_default,omitempty"` // Default is FALSE in DB
	SearchRadiusKm       *float64             `json:"search_radius_km,omitempty"`
	PreferredTime        *DayPreference       `json:"preferred_time,omitempty"`
	BudgetLevel          *int                 `json:"budget_level,omitempty"`
	PreferredPace        *SearchPace          `json:"preferred_pace,omitempty"`
	PreferAccessiblePOIs *bool                `json:"prefer_accessible_pois,omitempty"`
	PreferOutdoorSeating *bool                `json:"prefer_outdoor_seating,omitempty"`
	PreferDogFriendly    *bool                `json:"prefer_dog_friendly,omitempty"`
	PreferredVibes       []string             `json:"preferred_vibes,omitempty"` // Use empty slice if not provided?
	PreferredTransport   *TransportPreference `json:"preferred_transport,omitempty"`
	DietaryNeeds         []string             `json:"dietary_needs,omitempty"`
	Tags                 []uuid.UUID          `json:"tags,omitempty"`
	Interests            []uuid.UUID          `json:"interests,omitempty"`
}

type Tags struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	TagType     string     `json:"tag_type"` // Consider using a specific enum type
	Description *string    `json:"description"`
	Source      *string    `json:"source"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

// UpdateSearchProfileParams defines fields allowed for updating a profile.
// Pointers allow partial updates.
type UpdateSearchProfileParams struct {
	ProfileName          string               `json:"profile_name" binding:"required"`
	IsDefault            *bool                `json:"is_default,omitempty"` // Default is FALSE in DB
	SearchRadiusKm       *float64             `json:"search_radius_km,omitempty"`
	PreferredTime        *DayPreference       `json:"preferred_time,omitempty"`
	BudgetLevel          *int                 `json:"budget_level,omitempty"`
	PreferredPace        *SearchPace          `json:"preferred_pace,omitempty"`
	PreferAccessiblePOIs *bool                `json:"prefer_accessible_pois,omitempty"`
	PreferOutdoorSeating *bool                `json:"prefer_outdoor_seating,omitempty"`
	PreferDogFriendly    *bool                `json:"prefer_dog_friendly,omitempty"`
	PreferredVibes       []string             `json:"preferred_vibes,omitempty"` // Use empty slice if not provided?
	PreferredTransport   *TransportPreference `json:"preferred_transport,omitempty"`
	DietaryNeeds         []string             `json:"dietary_needs,omitempty"`
	Tags                 []*string            `json:"tags,omitempty"`
	Interests            []*string            `json:"interests,omitempty"`
}
