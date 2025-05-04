package types

import "time"

// UserAuth represents the core user entity in the domain.
type UserAuth struct {
	ID        string    `json:"id" example:"d290f1ee-6c54-4b01-90e6-d701748f0851"` // Unique identifier (UUID).
	Username  string    `json:"username" example:"johndoe"`                        // Optional unique username.
	Email     string    `json:"email" example:"john.doe@example.com"`              // Unique email address used for login.
	Password  string    `json:"-"`                                                 // Hashed password (never exposed).
	Role      string    `json:"role" example:"user"`                               // User role (e.g., 'user', 'admin').
	CreatedAt time.Time `json:"created_at"`                                        // Timestamp when the user was created.
	UpdatedAt time.Time `json:"updated_at"`                                        // Timestamp when the user was last updated.
	// DeletedAt *time.Time `json:"deleted_at,omitempty"`                         // Timestamp for soft deletes (if implemented).
}
