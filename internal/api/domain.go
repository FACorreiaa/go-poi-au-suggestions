package api

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/golang-jwt/jwt/v5"
)

// --- SECURITY WARNING ---
// JwtSecretKey and JwtRefreshSecretKey should be loaded from secure configuration,
// NOT hardcoded. These are placeholders only.
var (
	JwtSecretKey        = []byte("replace-with-secure-env-var")
	JwtRefreshSecretKey = []byte("replace-with-different-secure-env-var")
)

// LoginRequest represents the expected JSON body for user login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com"` // User's email address for login.
	Password string `json:"password" binding:"required" example:"password123"`         // User's password.
}

// LoginResponse represents the successful JSON response after login.
type LoginResponse struct {
	AccessToken  string `json:"access_token" example:"eyJhbGciOiJI..."` // Short-lived JWT access token.
	RefreshToken string `json:"refresh_token" example:"4f1trt8s..."`    // Longer-lived refresh token (often set in HttpOnly cookie instead).
	Message      string `json:"message" example:"Login successful"`     // Confirmation message.
}

// RegisterRequest represents the expected JSON body for user registration.
type RegisterRequest struct {
	Username string `json:"username" binding:"required" example:"testuser"`               // Desired username. Must be unique.
	Email    string `json:"email" binding:"required,email" example:"newuser@example.com"` // User's email address. Must be unique.
	Password string `json:"password" binding:"required,min=8" example:"Str0ngP@ss!"`      // User's desired password (min length 8).
	Role     string `json:"role,omitempty" example:"user"`                                // Optional role assignment (defaults server-side if empty).
}

// RefreshTokenRequest represents the expected JSON body for refreshing tokens.
// Often, the refresh token is read from an HttpOnly cookie instead.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required" example:"4f1trt8s..."` // The refresh token obtained during login.
}

// TokenResponse represents the successful JSON response after refreshing tokens.
type TokenResponse struct {
	AccessToken  string `json:"access_token" example:"eyJhbGciOiJI..."` // The new short-lived JWT access token.
	RefreshToken string `json:"refresh_token" example:"9a8b7c..."`      // The *new* longer-lived refresh token (if rotation is enabled, often set in cookie).
}

// ValidateSessionRequest represents the request body for validating a session (less common with JWTs).
type ValidateSessionRequest struct {
	SessionID string `json:"session_id" binding:"required"` // The session identifier (might be an access token).
}

// ChangePasswordRequest represents the expected JSON body for changing the authenticated user's password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required" example:"currentPassword123"`   // User's current password.
	NewPassword string `json:"new_password" binding:"required,min=8" example:"NewStr0ngP@ss!"` // User's desired new password.
}

// ChangeEmailRequest represents the expected JSON body for changing the authenticated user's email.
type ChangeEmailRequest struct {
	Password string `json:"password" binding:"required" example:"currentPassword123"`           // User's current password for verification.
	NewEmail string `json:"new_email" binding:"required,email" example:"new.email@example.com"` // Desired new email address.
}

// LogoutRequest represents the expected JSON body for logout (if sending refresh token in body).
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"` // Refresh token to invalidate.
}

// Response represents a generic API response for success or error messages.
type Response struct {
	Success bool   `json:"success" example:"true"`                           // Indicates if the operation was successful.
	Message string `json:"message,omitempty" example:"Operation successful"` // Optional success message.
	Error   string `json:"error,omitempty" example:"Resource not found"`     // Optional error message.
}

// ValidateSessionResponse represents the response when validating a session (less common with JWTs).
type ValidateSessionResponse struct {
	Valid    bool   `json:"valid"`              // True if the session/token is valid.
	UserID   string `json:"user_id,omitempty"`  // User ID associated with the session/token.
	Username string `json:"username,omitempty"` // Username associated with the session/token.
	Email    string `json:"email,omitempty"`    // Email associated with the session/token.
}

// Session represents legacy session data (less common with JWT flow).
type Session struct {
	ID       string `json:"id"`       // User ID associated with the session.
	Username string `json:"username"` // Username at the time the session was created.
	Email    string `json:"email"`    // Email at the time the session was created.
}

// Claims represents the custom claims included in the JWT access token.
type Claims struct {
	UserID               string `json:"uid"`             // Custom claim for User ID.
	Username             string `json:"usr,omitempty"`   // Custom claim for Username.
	Email                string `json:"eml"`             // Custom claim for Email.
	Role                 string `json:"rol"`             // Custom claim for User Role.
	SubscriptionPlan     string `json:"pln,omitempty"`   // Custom claim for Subscription Plan (e.g., 'free', 'premium').
	SubscriptionStatus   string `json:"sts,omitempty"`   // Custom claim for Subscription Status (e.g., 'active', 'trialing').
	Scope                string `json:"scope,omitempty"` // Optional scope information.
	jwt.RegisteredClaims        // Embed standard claims (ExpiresAt, IssuedAt, Subject, etc.).
}

// SubscriptionRepository defines methods for accessing subscription data.
type SubscriptionRepository interface {
	// GetCurrentSubscriptionByUserID fetches the active/relevant subscription for a user.
	GetCurrentSubscriptionByUserID(ctx context.Context, userID string) (*Subscription, error)
	// CreateDefaultSubscription creates the initial (e.g., free) subscription for a new user.
	CreateDefaultSubscription(ctx context.Context, userID string) error
}

// Subscription holds basic plan and status information.
type Subscription struct {
	Plan   string `json:"plan"`   // e.g., "free", "premium_monthly".
	Status string `json:"status"` // e.g., "active", "past_due".
}

type DayPreference string

const (
	DayPreferenceAny   DayPreference = "any"
	DayPreferenceDay   DayPreference = "day"
	DayPreferenceNight DayPreference = "night"
)

type SearchPace string

const (
	SearchPaceAny      SearchPace = "any"
	SearchPaceRelaxed  SearchPace = "relaxed"
	SearchPaceModerate SearchPace = "moderate"
	SearchPaceFast     SearchPace = "fast"
)

type TransportPreference string

const (
	TransportPreferenceAny    TransportPreference = "any"
	TransportPreferenceWalk   TransportPreference = "walk"
	TransportPreferencePublic TransportPreference = "public"
	TransportPreferenceCar    TransportPreference = "car"
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
// Deprecated: Use UpdateUserPreferenceProfileParams instead
type UpdateUserSettingsParams struct {
	DefaultSearchRadiusKm *float64       `json:"default_search_radius_km,omitempty"`
	PreferredTime         *DayPreference `json:"preferred_time,omitempty"`
	DefaultBudgetLevel    *int           `json:"default_budget_level,omitempty"`
	PreferredPace         *SearchPace    `json:"preferred_pace,omitempty"`
	PreferAccessiblePOIs  *bool          `json:"prefer_accessible_pois,omitempty"`
	PreferOutdoorSeating  *bool          `json:"prefer_outdoor_seating,omitempty"`
	PreferDogFriendly     *bool          `json:"prefer_dog_friendly,omitempty"`
}

// CreateUserPreferenceProfileParams is used for creating a new user preference profile
//type CreateUserPreferenceProfileParams struct {
//	ProfileName          string               `json:"profile_name" binding:"required"`
//	IsDefault            *bool                `json:"is_default,omitempty"`
//	SearchRadiusKm       *float64             `json:"search_radius_km,omitempty"`
//	PreferredTime        *DayPreference       `json:"preferred_time,omitempty"`
//	BudgetLevel          *int                 `json:"budget_level,omitempty"`
//	PreferredPace        *SearchPace          `json:"preferred_pace,omitempty"`
//	PreferAccessiblePOIs *bool                `json:"prefer_accessible_pois,omitempty"`
//	PreferOutdoorSeating *bool                `json:"prefer_outdoor_seating,omitempty"`
//	PreferDogFriendly    *bool                `json:"prefer_dog_friendly,omitempty"`
//	PreferredVibes       []string             `json:"preferred_vibes,omitempty"`
//	PreferredTransport   *TransportPreference `json:"preferred_transport,omitempty"`
//	DietaryNeeds         []string             `json:"dietary_needs,omitempty"`
//	Tags                 []string             `json:"tags,omitempty"`
//	Interests            []string             `json:"interests,omitempty"`
//}

// UpdateUserPreferenceProfileParams is used for updating a user preference profile
type UpdateUserPreferenceProfileParams struct {
	ProfileName          *string              `json:"profile_name,omitempty"`
	IsDefault            *bool                `json:"is_default,omitempty"`
	SearchRadiusKm       *float64             `json:"search_radius_km,omitempty"`
	PreferredTime        *DayPreference       `json:"preferred_time,omitempty"`
	BudgetLevel          *int                 `json:"budget_level,omitempty"`
	PreferredPace        *SearchPace          `json:"preferred_pace,omitempty"`
	PreferAccessiblePOIs *bool                `json:"prefer_accessible_pois,omitempty"`
	PreferOutdoorSeating *bool                `json:"prefer_outdoor_seating,omitempty"`
	PreferDogFriendly    *bool                `json:"prefer_dog_friendly,omitempty"`
	PreferredVibes       []string             `json:"preferred_vibes,omitempty"`
	PreferredTransport   *TransportPreference `json:"preferred_transport,omitempty"`
	DietaryNeeds         []string             `json:"dietary_needs,omitempty"`
}

type UpdateUserInterestParams struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Active      *bool   `json:"active,omitempty"`
}
