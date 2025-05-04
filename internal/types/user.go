package types

import (
	"time"

	"github.com/google/uuid"
)

type UserProfile struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	Username        *string    `json:"username,omitempty"` // Use pointer if nullable/optional unique
	Firstname       *string    `json:"firstname,omitempty"`
	Lastname        *string    `json:"lastname,omitempty"`
	Age             *int       `json:"age,omitempty"`
	City            *string    `json:"city,omitempty"`
	Country         *string    `json:"country,omitempty"`
	AboutYou        *string    `json:"about_you,omitempty"`
	PasswordHash    string     `json:"-"`                           // Exclude from JSON responses
	DisplayName     *string    `json:"display_name,omitempty"`      // Use pointer if nullable
	ProfileImageURL *string    `json:"profile_image_url,omitempty"` // Use pointer if nullable
	IsActive        bool       `json:"is_active"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"` // Use pointer if nullable
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`     // Use pointer if nullable
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// UpdateProfileParams defines the fields allowed for profile updates.
// Use pointers for optional fields, allowing partial updates.
type UpdateProfileParams struct {
	Username        *string // Pointer allows distinguishing between empty string and not provided
	Email           *string
	DisplayName     *string
	ProfileImageURL *string
	Firstname       *string `json:"firstname,omitempty"`
	Lastname        *string `json:"lastname,omitempty"`
	Age             *int    `json:"age,omitempty"`
	City            *string `json:"city,omitempty"`
	Country         *string `json:"country,omitempty"`
	AboutYou        *string `json:"about_you,omitempty"`
	// Add any other mutable fields like bio, location string etc.
}
