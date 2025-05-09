package types

import (
	"time"

	"github.com/google/uuid"
)

// UserSettings mirrors the old database table structure.
// Deprecated: Use UserPreferenceProfile instead
type UserSettings struct {
	UserID                uuid.UUID     `json:"user_id"`
	DefaultSearchRadiusKm float64       `json:"default_search_radius_km"` // Use float64 for NUMERIC
	PreferredTime         DayPreference `json:"preferred_time"`
	DefaultBudgetLevel    int           `json:"default_budget_level"`
	PreferredPace         SearchPace    `json:"preferred_pace"`
	PreferAccessiblePOIs  bool          `json:"prefer_accessible_pois"`
	PreferOutdoorSeating  bool          `json:"prefer_outdoor_seating"`
	PreferDogFriendly     bool          `json:"prefer_dog_friendly"`
	CreatedAt             time.Time     `json:"created_at"`
	UpdatedAt             time.Time     `json:"updated_at"`
}

// UpdateUserSettingsParams is used for updating user settings.
// Deprecated: Use UpdateSearchProfileParams instead
type UpdateUserSettingsParams struct {
	DefaultSearchRadiusKm *float64       `json:"default_search_radius_km,omitempty"`
	PreferredTime         *DayPreference `json:"preferred_time,omitempty"`
	DefaultBudgetLevel    *int           `json:"default_budget_level,omitempty"`
	PreferredPace         *SearchPace    `json:"preferred_pace,omitempty"`
	PreferAccessiblePOIs  *bool          `json:"prefer_accessible_pois,omitempty"`
	PreferOutdoorSeating  *bool          `json:"prefer_outdoor_seating,omitempty"`
	PreferDogFriendly     *bool          `json:"prefer_dog_friendly,omitempty"`
}
