package auth

import (
	"context"
	"log/slog"
)

// Session represents a user session
type Session struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Tenant   string `json:"tenant"`
}

// AuthService defines the interface for authentication operations
type AuthService interface {
	// Login authenticates a user and returns access and refresh tokens
	Login(ctx context.Context, tenant, email, password string) (string, string, error)

	// Logout invalidates a user session
	Logout(ctx context.Context, tenant, sessionID string) error

	// GetSession retrieves a user session
	GetSession(ctx context.Context, tenant, sessionID string) (*Session, error)

	// RefreshSession generates new access and refresh tokens using a valid refresh token
	RefreshSession(ctx context.Context, tenant, refreshToken string) (string, string, error)

	// Register creates a new user
	Register(ctx context.Context, tenant, username, email, password, role string) error

	// ValidateCredentials validates user credentials
	ValidateCredentials(ctx context.Context, tenant, email, password string) (bool, error)

	// ValidateSession checks if a session is valid
	ValidateSession(ctx context.Context, tenant, sessionID string) (bool, error)

	// GetUserRole gets a user's role
	GetUserRole(ctx context.Context, tenant, userID string) (string, error)

	// VerifyPassword verifies a password for a user
	VerifyPassword(ctx context.Context, tenant, userID, password string) error

	// UpdatePassword updates a user's password
	UpdatePassword(ctx context.Context, tenant, userID, newHashedPassword string) error

	// InvalidateAllUserRefreshTokens invalidates all refresh tokens for a user
	InvalidateAllUserRefreshTokens(ctx context.Context, tenant, userID string) error
}

// AuthServiceImpl implements the AuthService interface
type AuthServiceImpl struct {
	logger *slog.Logger
	repo   AuthRepo
}

// NewAuthService creates a new AuthService
func NewAuthService(repo AuthRepo, logger *slog.Logger) *AuthServiceImpl {
	return &AuthServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

// Login authenticates a user and returns access and refresh tokens
func (s *AuthServiceImpl) Login(ctx context.Context, tenant, email, password string) (string, string, error) {
	return s.repo.Login(ctx, tenant, email, password)
}

// Logout invalidates a user session
func (s *AuthServiceImpl) Logout(ctx context.Context, tenant, sessionID string) error {
	return s.repo.Logout(ctx, tenant, sessionID)
}

// GetSession retrieves a user session
func (s *AuthServiceImpl) GetSession(ctx context.Context, tenant, sessionID string) (*Session, error) {
	return s.repo.GetSession(ctx, tenant, sessionID)
}

// RefreshSession generates new access and refresh tokens using a valid refresh token
func (s *AuthServiceImpl) RefreshSession(ctx context.Context, tenant, refreshToken string) (string, string, error) {
	return s.repo.RefreshSession(ctx, tenant, refreshToken)
}

// Register creates a new user
func (s *AuthServiceImpl) Register(ctx context.Context, tenant, username, email, password, role string) error {
	return s.repo.Register(ctx, tenant, username, email, password, role)
}

// ValidateCredentials validates user credentials
func (s *AuthServiceImpl) ValidateCredentials(ctx context.Context, tenant, email, password string) (bool, error) {
	return s.repo.ValidateCredentials(ctx, tenant, email, password)
}

// ValidateSession checks if a session is valid
func (s *AuthServiceImpl) ValidateSession(ctx context.Context, tenant, sessionID string) (bool, error) {
	return s.repo.ValidateSession(ctx, tenant, sessionID)
}

// GetUserRole gets a user's role
func (s *AuthServiceImpl) GetUserRole(ctx context.Context, tenant, userID string) (string, error) {
	return s.repo.GetUserRole(ctx, tenant, userID)
}

// VerifyPassword verifies a password for a user
func (s *AuthServiceImpl) VerifyPassword(ctx context.Context, tenant, userID, password string) error {
	return s.repo.VerifyPassword(ctx, tenant, userID, password)
}

// UpdatePassword updates a user's password
func (s *AuthServiceImpl) UpdatePassword(ctx context.Context, tenant, userID, newHashedPassword string) error {
	return s.repo.UpdatePassword(ctx, tenant, userID, newHashedPassword)
}

// InvalidateAllUserRefreshTokens invalidates all refresh tokens for a user
func (s *AuthServiceImpl) InvalidateAllUserRefreshTokens(ctx context.Context, tenant, userID string) error {
	return s.repo.InvalidateAllUserRefreshTokens(ctx, tenant, userID)
}
