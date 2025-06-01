package types

import (
	"time"

	"github.com/google/uuid"
)

// settings mirrors the old database table structure.
// Deprecated: Use UserPreferenceProfile instead
type Settings struct {
	UserID                uuid.UUID     `json:"user_id"`
	DefaultSearchRadiusKm float64       `json:"default_search_radius_km"` // Use float64 for NUMERIC
	PreferredTime         DayPreference `json:"preferred_time"`
	DefaultBudgetLevel    int           `json:"default_budget_level"`
	PreferredPace         SearchPace    `json:"preferred_pace"`
	PreferAccessiblePOIs  bool          `json:"prefer_accessible_pois"`
	PreferOutdoorSeating  bool          `json:"prefer_outdoor_seating"`
	PreferDogFriendly     bool          `json:"prefer_dog_friendly"`
	SearchRadius          float64       `json:"search_radius"`       // For SearchProfile
	BudgetLevel           int           `json:"budget_level"`        // For SearchProfile
	PreferredTransport    string        `json:"preferred_transport"` // For SearchProfile
	CreatedAt             time.Time     `json:"created_at"`
	UpdatedAt             time.Time     `json:"updated_at"`
}

// UpdatesettingsParams is used for updating user settings.
// Deprecated: Use UpdateSearchProfileParams instead
type UpdatesettingsParams struct {
	DefaultSearchRadiusKm *float64       `json:"default_search_radius_km,omitempty"`
	PreferredTime         *DayPreference `json:"preferred_time,omitempty"`
	DefaultBudgetLevel    *int           `json:"default_budget_level,omitempty"`
	BudgetLevel           *int           `json:"budget_level,omitempty"` // For SearchProfile
	IsDefault             *bool          `json:"is_default,omitempty"`   // For SearchProfile
	PreferredPace         *SearchPace    `json:"preferred_pace,omitempty"`
	PreferAccessiblePOIs  *bool          `json:"prefer_accessible_pois,omitempty"`
	PreferOutdoorSeating  *bool          `json:"prefer_outdoor_seating,omitempty"`
	PreferDogFriendly     *bool          `json:"prefer_dog_friendly,omitempty"`
	SearchRadius          *float64       `json:"search_radius,omitempty"` // For SearchProfile
}
