package auth

import (
	"context"

	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var _ AuthRepo = (*AuthRepoFactory)(nil)

type AuthRepo interface {
	Login(ctx context.Context, email, password string) (string, string, error)
	Logout(ctx context.Context, sessionID string) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	RefreshSession(ctx context.Context, refreshToken string) (string, string, error) // accessToken, refreshToken, error
	Register(ctx context.Context, username, email, password string) error
	ValidateCredentials(ctx context.Context, email, password string) (bool, error)
	ValidateSession(ctx context.Context, sessionID string) (bool, error)
	VerifyPassword(ctx context.Context, userID, password string) error
	UpdatePassword(ctx context.Context, userID, newHashedPassword string) error
	InvalidateAllUserRefreshTokens(ctx context.Context, userID string) error
}

type AuthRepoFactory struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewAuthRepoFactory(pgxpool *pgxpool.Pool, logger *slog.Logger) *AuthRepoFactory {
	return &AuthRepoFactory{
		logger: logger,
		pgpool: pgxpool,
	}
}

// generateRefreshToken creates a random refresh token
func generateRefreshToken() string {
	return uuid.NewString()
}

// Login authenticates a user and returns an access token
func (r *AuthRepoFactory) Login(ctx context.Context, email, password string) (string, string, error) {
	var user User
	err := r.pgpool.QueryRow(ctx,
		"SELECT id, username, email, password_hash FROM users WHERE email = $1",
		email).Scan(&user.ID, &user.Username, &user.Email, &user.Password)
	if err != nil {
		return "", "", fmt.Errorf("user not found: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", "", errors.New("invalid credentials")
	}

	// Generate access token
	accessToken, err := generateAccessToken(user.ID, user.Username, user.Email)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate and store refresh token
	newRefreshToken := generateRefreshToken()
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days
	_, err = r.pgpool.Exec(ctx,
		"INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)",
		user.ID, newRefreshToken, expiresAt)
	if err != nil {
		return "", "", fmt.Errorf("failed to store refresh token: %w", err)
	}

	return accessToken, newRefreshToken, nil // Note: Refresh token not returned due to proto limitation
}

// SignOut invalidates a user session
func (r *AuthRepoFactory) Logout(ctx context.Context, sessionID string) error {
	return r.InvalidateRefreshToken(ctx, sessionID)
}

// GetSession retrieves a user session
func (r *AuthRepoFactory) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	var userID string
	var expiresAt time.Time
	var invalidatedAt *time.Time

	err := r.pgpool.QueryRow(ctx,
		"SELECT user_id, expires_at, invalidated_at FROM sessions WHERE session_id = $1",
		sessionID).Scan(&userID, &expiresAt, &invalidatedAt)
	if err != nil {
		return nil, errors.New("session not found")
	}

	// Check if session is expired or invalidated
	if time.Now().After(expiresAt) || (invalidatedAt != nil) {
		return nil, errors.New("session expired or invalidated")
	}

	// Session exists in DB but not in Redis, need to fetch user details
	var user struct {
		Username string
		Email    string
	}

	err = r.pgpool.QueryRow(ctx,
		"SELECT username, email FROM users WHERE id = $1",
		userID).Scan(&user.Username, &user.Email)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Recreate session object
	session := &Session{
		ID:       userID,
		Username: user.Username,
		Email:    user.Email,
	}

	return session, nil
}

// GetUserByID retrieves a user by email

// ValidateCredentials validates user credentials
func (r *AuthRepoFactory) ValidateCredentials(ctx context.Context, email, password string) (bool, error) {
	var hashedPassword string
	err := r.pgpool.QueryRow(ctx,
		"SELECT password_hash FROM users WHERE email = $1",
		email).Scan(&hashedPassword)
	if err != nil {
		return false, fmt.Errorf("user not found: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil, nil
}

func (r *AuthRepoFactory) RefreshSession(ctx context.Context, refreshToken string) (string, string, error) {

	var userID string
	var expiresAt time.Time
	var invalidatedAt *time.Time
	err := r.pgpool.QueryRow(ctx,
		"SELECT user_id, expires_at, revoked_at FROM refresh_tokens WHERE token = $1",
		refreshToken).Scan(&userID, &expiresAt, &invalidatedAt)
	if err != nil {
		return "", "", errors.New("invalid refresh token")
	}

	if time.Now().After(expiresAt) || invalidatedAt != nil {
		return "", "", errors.New("refresh token expired or invalidated")
	}

	var username, email, role string
	err = r.pgpool.QueryRow(ctx,
		"SELECT username, email FROM users WHERE id = $1",
		userID).Scan(&username, &email, &role)
	if err != nil {
		return "", "", fmt.Errorf("user not found: %w", err)
	}

	newAccessToken, err := generateAccessToken(userID, username, email)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken := generateRefreshToken()
	newExpiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = r.pgpool.Exec(ctx,
		"INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)",
		userID, newRefreshToken, newExpiresAt)
	if err != nil {
		return "", "", fmt.Errorf("failed to store new refresh token: %w", err)
	}

	_, err = r.pgpool.Exec(ctx,
		"UPDATE refresh_tokens SET revoked_at = $1 WHERE token = $2",
		time.Now(), refreshToken)
	if err != nil {
		fmt.Printf("Warning: failed to invalidate old refresh token: %v\n", err)
	}

	return newAccessToken, newRefreshToken, nil
}

// Register creates a new user in the tenant's database
func (r *AuthRepoFactory) Register(ctx context.Context, username, email, password string) error {

	var userID string
	err := r.pgpool.QueryRow(ctx,
		"SELECT id FROM users WHERE id = $1", userID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("studio not found: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = r.pgpool.Exec(ctx,
		"INSERT INTO users (id, username, email, password_hash, created_at) VALUES ($1, $2, $3, $4, $5)",
		userID, username, email, string(hashedPassword), time.Now())
	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

// ChangePassword updates a user's password
func (r *AuthRepoFactory) ChangePassword(ctx context.Context, email, oldPassword, newPassword string) error {
	var userID, hashedPassword string
	err := r.pgpool.QueryRow(ctx,
		"SELECT id, password_hash FROM users WHERE email = $1",
		email).Scan(&userID, &hashedPassword)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(oldPassword))
	if err != nil {
		return errors.New("invalid old password")
	}

	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	_, err = r.pgpool.Exec(ctx,
		"UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3",
		string(newHashedPassword), time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Invalidate all refresh tokens
	_, err = r.pgpool.Exec(ctx,
		"UPDATE refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL",
		time.Now(), userID)
	if err != nil {
		fmt.Printf("Warning: failed to invalidate refresh tokens: %v\n", err)
	}

	return nil
}

// ChangeEmail updates a user's email
func (r *AuthRepoFactory) ChangeEmail(ctx context.Context, email, password, newEmail string) error {
	var userID, hashedPassword string
	err := r.pgpool.QueryRow(ctx,
		"SELECT id, password_hash FROM users WHERE email = $1",
		email).Scan(&userID, &hashedPassword)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return errors.New("invalid credentials")
	}

	_, err = r.pgpool.Exec(ctx,
		"UPDATE users SET email = $1, updated_at = $2 WHERE id = $3",
		newEmail, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update email: %w", err)
	}

	return nil
}

func (r *AuthRepoFactory) ValidateSession(ctx context.Context, sessionID string) (bool, error) {
	var userID string
	err := r.pgpool.QueryRow(ctx,
		"SELECT user_id FROM sessions WHERE session_id = $1",
		sessionID).Scan(&userID)
	if err != nil {
		return false, fmt.Errorf("session not found: %w", err)
	}

	if userID == "" {
		return false, nil
	}

	return true, nil
}

func (r *AuthRepoFactory) GetUserByID(ctx context.Context, userID string) (*User, error) {
	var user User
	err := r.pgpool.QueryRow(ctx,
		"SELECT id, username, email FROM users WHERE id = $1",
		userID).Scan(&user.ID, &user.Username, &user.Email)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	return &user, nil
}

func (r *AuthRepoFactory) StoreRefreshToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	_, err := r.pgpool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token, expires_at)
         VALUES ($1, $2, $3)`,
		userID, token, expiresAt)
	if err != nil {
		return fmt.Errorf("store refresh token: db insert failed: %w", err)
	}
	return nil
}

func (r *AuthRepoFactory) GetSessionInfoFromRefreshToken(ctx context.Context, refreshToken string) (string, time.Time, *time.Time, error) {
	var userID string
	var expiresAt time.Time
	var invalidatedAt *time.Time // Use pointer for nullable timestamp

	err := r.pgpool.QueryRow(ctx,
		`SELECT user_id, expires_at, revoked_at
         FROM refresh_tokens
         WHERE token = $1`, refreshToken).Scan(&userID, &expiresAt, &invalidatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, nil, errors.New("invalid refresh token")
		}
		return "", time.Time{}, nil, fmt.Errorf("get session info: query failed: %w", err)
	}

	return userID, expiresAt, invalidatedAt, nil
}

func (r *AuthRepoFactory) InvalidateRefreshToken(ctx context.Context, refreshToken string) error {
	tag, err := r.pgpool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $1
         WHERE token = $2 AND revoked_at IS NULL`,
		time.Now(), refreshToken)

	if err != nil {
		return fmt.Errorf("invalidate refresh token: db update failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Token was already revoked or didn't exist, not necessarily an error for logout
		// but could be logged.
		fmt.Printf("Warning: No refresh token found or already revoked for token: %s\n", refreshToken)
	}
	return nil
}

func (r *AuthRepoFactory) InvalidateAllUserRefreshTokens(ctx context.Context, userID string) error {
	_, err := r.pgpool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $1
		 WHERE user_id = $2 AND revoked_at IS NULL`,
		time.Now(), userID)
	if err != nil {
		return fmt.Errorf("invalidate all tokens: db update failed: %w", err)
	}
	// Log how many were invalidated?
	return nil
}

func (r *AuthRepoFactory) VerifyPassword(ctx context.Context, userID, password string) error {
	var hashedPassword string
	err := r.pgpool.QueryRow(ctx, "SELECT password_hash FROM users WHERE id = $1", userID).Scan(&hashedPassword)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("user not found")
		}
		return fmt.Errorf("verify password: query failed: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return errors.New("invalid password")
	}
	return nil
}

func (r *AuthRepoFactory) UpdatePassword(ctx context.Context, userID, newHashedPassword string) error {
	tag, err := r.pgpool.Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3`,
		newHashedPassword, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("update password: db update failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found or password unchanged") // Or specific domain error
	}
	return nil
}
