package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	Base
	Email           string     `json:"email" db:"email"`
	Username        string     `json:"username" db:"username"`
	PasswordHash    string     `json:"-" db:"password_hash"`
	DisplayName     string     `json:"display_name" db:"display_name"`
	ProfileImageURL string     `json:"profile_image_url" db:"profile_image_url"`
	IsActive        bool       `json:"is_active" db:"is_active"`
	EmailVerifiedAt *time.Time `json:"email_verified_at" db:"email_verified_at"`
	LastLoginAt     *time.Time `json:"last_login_at" db:"last_login_at"`
}

// Subscription represents a user's subscription
type Subscription struct {
	Base
	UserID                uuid.UUID           `json:"user_id" db:"user_id"`
	Plan                  SubscriptionPlanType `json:"plan" db:"plan"`
	Status                SubscriptionStatus   `json:"status" db:"status"`
	StartDate             time.Time           `json:"start_date" db:"start_date"`
	EndDate               *time.Time          `json:"end_date" db:"end_date"`
	TrialEndDate          *time.Time          `json:"trial_end_date" db:"trial_end_date"`
	ExternalProvider      string              `json:"external_provider" db:"external_provider"`
	ExternalSubscriptionID string              `json:"external_subscription_id" db:"external_subscription_id"`
}

// NewUser creates a new user with default values
func NewUser(email, username, passwordHash string) *User {
	return &User{
		Base: Base{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Email:        email,
		Username:     username,
		PasswordHash: passwordHash,
		IsActive:     true,
	}
}

// NewSubscription creates a new subscription with default values
func NewSubscription(userID uuid.UUID, plan SubscriptionPlanType) *Subscription {
	return &Subscription{
		Base: Base{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		UserID:    userID,
		Plan:      plan,
		Status:    SubscriptionStatusActive,
		StartDate: time.Now(),
	}
}