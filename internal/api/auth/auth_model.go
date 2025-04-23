package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrNotFound = errors.New("requested item not found")
var ErrConflict = errors.New("item already exists or conflict")
var ErrUnauthenticated = errors.New("authentication required or invalid credentials")
var ErrForbidden = errors.New("action forbidden")

// JwtSecretKey to change
var JwtSecretKey = []byte("your-secret-key")
var JwtRefreshSecretKey = []byte("your-refresh-key")

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents the login response body
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Message      string `json:"message"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role,omitempty"`
}
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// ValidateSessionRequest represents the validate session request body
type ValidateSessionRequest struct {
	SessionID string `json:"session_id"`
}

// ChangePasswordRequest represents the change password request body
type ChangePasswordRequest struct {
	Username    string `json:"username"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangeEmailRequest represents the change email request body
type ChangeEmailRequest struct {
	Password string `json:"password"`
	NewEmail string `json:"new_email"`
}

// LogoutRequest represents the logout request body
type LogoutRequest struct {
	SessionID string `json:"session_id"`
}

// Generic response for simple success/error messages
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ValidateSessionResponse represents the validate session response body
type ValidateSessionResponse struct {
	Valid    bool   `json:"valid"`
	UserID   string `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
}

type User struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	Password  string     `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

type Session struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Scope    string `json:"scope"`
	jwt.RegisteredClaims
}

type SubscriptionRepository interface {
	GetCurrentSubscriptionByUserID(ctx context.Context, userID string) (*Subscription, error)
	CreateDefaultSubscription(ctx context.Context, userID string) error
}
type Subscription struct {
	Plan   string // e.g., "free", "premium_monthly"
	Status string // e.g., "active", "past_due"
}
