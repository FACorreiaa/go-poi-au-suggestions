package types

import (
	"time"

	"github.com/google/uuid"
)

type UserStats struct {
	PlacesVisited  int `json:"places_visited"`
	ReviewsWritten int `json:"reviews_written"`
	ListsCreated   int `json:"lists_created"`
	Followers      int `json:"followers"`
	Following      int `json:"following"`
}

type UserProfile struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	Username        *string    `json:"username,omitempty"` // Use pointer if nullable/optional unique
	Firstname       *string    `json:"firstname,omitempty"`
	Lastname        *string    `json:"lastname,omitempty"`
	PhoneNumber     *string    `json:"phone,omitempty"`
	Age             *int       `json:"age,omitempty"`
	City            *string    `json:"city,omitempty"`
	Country         *string    `json:"country,omitempty"`
	AboutYou        *string    `json:"about_you,omitempty"`
	Bio             *string    `json:"bio,omitempty"`               // Maps to about_you for frontend compatibility
	Location        *string    `json:"location,omitempty"`          // New field for user location
	JoinedDate      time.Time  `json:"joinedDate"`                  // Maps to created_at
	Avatar          *string    `json:"avatar,omitempty"`            // Maps to profile_image_url
	Interests       []string   `json:"interests,omitempty"`         // New array field
	Badges          []string   `json:"badges,omitempty"`            // New array field
	Stats           *UserStats `json:"stats,omitempty"`             // New nested stats object
	PasswordHash    string     `json:"-"`                           // Exclude from JSON responses
	DisplayName     *string    `json:"display_name,omitempty"`      // Use pointer if nullable
	ProfileImageURL *string    `json:"profile_image_url,omitempty"` // Use pointer if nullable
	IsActive        bool       `json:"is_active"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"` // Use pointer if nullable
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`     // Use pointer if nullable
	Theme           *string    `json:"theme,omitempty"`             // Use pointer if nullable
	Language        *string    `json:"language,omitempty"`          // Use pointer if nullable
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// UpdateProfileParams defines the fields allowed for profile updates.
// Use pointers for optional fields, allowing partial updates.
type UpdateProfileParams struct {
	Username        *string   `json:"username,omitempty"`
	PhoneNumber     *string   `json:"phone,omitempty"`
	Email           *string   `json:"email,omitempty"`
	DisplayName     *string   `json:"display_name,omitempty"`
	ProfileImageURL *string   `json:"profile_image_url,omitempty"`
	Firstname       *string   `json:"firstname,omitempty"`
	Lastname        *string   `json:"lastname,omitempty"`
	Age             *int      `json:"age,omitempty"`
	City            *string   `json:"city,omitempty"`
	Country         *string   `json:"country,omitempty"`
	AboutYou        *string   `json:"about_you,omitempty"`
	Location        *string   `json:"location,omitempty"`
	Interests       *[]string `json:"interests,omitempty"`
	Badges          *[]string `json:"badges,omitempty"`
}
