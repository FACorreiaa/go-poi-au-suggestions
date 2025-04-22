package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JwtSecretKey to change
var JwtSecretKey = []byte("your-secret-key")
var JwtRefreshSecretKey = []byte("your-refresh-key")

type User struct {
	ID        string
	Username  string
	Email     string
	Password  string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
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
	Role     string `json:"role"`
	Tenant   string `json:"tenant"`
	Scope    string `json:"scope"`
	StudioID string `json:"studio_id,omitempty"`
	jwt.RegisteredClaims
}

func generateAccessToken(userID, username, email string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)), // Short-lived
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JwtSecretKey) // Assume JwtSecretKey is a global secret
}
