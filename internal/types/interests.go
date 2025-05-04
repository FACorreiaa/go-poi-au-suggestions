package types

import (
	"time"

	"github.com/google/uuid"
)

// Interest defines the structure for an interest tag.
type Interest struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"` // Use pointer if nullable
	Active      *bool      `json:"active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}
