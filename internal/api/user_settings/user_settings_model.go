package userSettings

import (
	"database/sql/driver" // Needed for custom enum Scan/Value
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

// --- Structs ---

// UserSettings represents the user's default preferences and settings.
// This struct mirrors the 'user_settings' database table.
type UserSettings struct {
	UserID                uuid.UUID           `json:"user_id"`                  // Primary Key, Foreign Key to users
	DefaultSearchRadiusKm float64             `json:"default_search_radius_km"` // Default search radius in kilometers
	PreferredTime         DayPreference       `json:"preferred_time"`           // Preferred time of day ('any', 'day', 'night')
	DefaultBudgetLevel    int                 `json:"default_budget_level"`     // Preferred budget (0-4, 0=any)
	PreferredPace         SearchPace          `json:"preferred_pace"`           // Preferred pace ('any', 'relaxed', 'moderate', 'fast')
	PreferAccessiblePOIs  bool                `json:"prefer_accessible_pois"`   // Preference for accessible places
	PreferOutdoorSeating  bool                `json:"prefer_outdoor_seating"`   // Preference for outdoor seating
	PreferTransportMode   TransportPreference `json:"prefer_transport_mode"`
	PreferDogFriendly     bool                `json:"prefer_dog_friendly"` // Preference for dog-friendly places
	CreatedAt             time.Time           `json:"created_at"`          // Timestamp of creation
	UpdatedAt             time.Time           `json:"updated_at"`          // Timestamp of last update
}

// UpdateUserSettingsParams defines the fields allowed for updating user settings.
// Pointers are used to allow partial updates (only provided fields are updated).
type UpdateUserSettingsParams struct {
	DefaultSearchRadiusKm *float64       `json:"default_search_radius_km,omitempty"`
	PreferredTime         *DayPreference `json:"preferred_time,omitempty"`
	DefaultBudgetLevel    *int           `json:"default_budget_level,omitempty"` // Use pointer even for int if 0 is a valid non-default value user might set
	PreferredPace         *SearchPace    `json:"preferred_pace,omitempty"`
	PreferAccessiblePOIs  *bool          `json:"prefer_accessible_pois,omitempty"`
	PreferOutdoorSeating  *bool          `json:"prefer_outdoor_seating,omitempty"`
	PreferDogFriendly     *bool          `json:"prefer_dog_friendly,omitempty"`
}
