package models

import (
	"time"

	"github.com/google/uuid"
)

// Interest represents a predefined interest/tag that users can select
type Interest struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// UserInterest represents a many-to-many relationship between users and interests
type UserInterest struct {
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	InterestID uuid.UUID `json:"interest_id" db:"interest_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// SavedPOI represents a many-to-many relationship between users and saved points of interest
type SavedPOI struct {
	UserID  uuid.UUID `json:"user_id" db:"user_id"`
	POIID   uuid.UUID `json:"poi_id" db:"poi_id"`
	SavedAt time.Time `json:"saved_at" db:"saved_at"`
}

// NewInterest creates a new interest with default values
func NewInterest(name, description string) *Interest {
	return &Interest{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
	}
}

// NewUserInterest creates a new user interest relationship
func NewUserInterest(userID, interestID uuid.UUID) *UserInterest {
	return &UserInterest{
		UserID:    userID,
		InterestID: interestID,
		CreatedAt:  time.Now(),
	}
}

// NewSavedPOI creates a new saved point of interest relationship
func NewSavedPOI(userID, poiID uuid.UUID) *SavedPOI {
	return &SavedPOI{
		UserID:  userID,
		POIID:   poiID,
		SavedAt: time.Now(),
	}
}