package types

import (
	"time"

	"github.com/google/uuid"
)

// PersonalTag represents the data structure for a personal tag.
type PersonalTag struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	Name        string     `json:"name"`
	TagType     string     `json:"tag_type"` // Consider using a specific enum type
	Description *string    `json:"description"`
	Source      string     `json:"source"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

// CreatePersonalTagParams holds parameters for creating a new personal tag.
type CreatePersonalTagParams struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TagType     string `json:"tag_type"`
	Active      *bool  `json:"active"`
}

// UpdatePersonalTagParams holds parameters for updating an existing personal tag.
type UpdatePersonalTagParams struct {
	Description string `json:"description"`
	Name        string `json:"name"`
	TagType     string `json:"tag_type"`
	Active      bool   `json:"active"`
}
