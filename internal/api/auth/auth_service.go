package auth

import (
	"context"
	"log/slog"
)

var _ AuthService = (*AuthServiceImpl)(nil)

type AuthService interface {
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

type AuthServiceImpl struct {
	logger *slog.Logger
	repo   AuthRepo
}

func NewAuthService(repo AuthRepo, logger *slog.Logger) *AuthServiceImpl {
	return &AuthServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

// GetSession implements AuthService.
func (a *AuthServiceImpl) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return a.repo.GetSession(ctx, sessionID)
}

// InvalidateAllUserRefreshTokens implements AuthService.
func (a *AuthServiceImpl) InvalidateAllUserRefreshTokens(ctx context.Context, userID string) error {
	return a.repo.InvalidateAllUserRefreshTokens(ctx, userID)
}

// Login implements AuthService.
func (a *AuthServiceImpl) Login(ctx context.Context, email string, password string) (string, string, error) {
	return a.repo.Login(ctx, email, password)
}

// Logout implements AuthService.
func (a *AuthServiceImpl) Logout(ctx context.Context, sessionID string) error {
	return a.repo.Logout(ctx, sessionID)
}

// RefreshSession implements AuthService.
func (a *AuthServiceImpl) RefreshSession(ctx context.Context, refreshToken string) (string, string, error) {
	return a.repo.RefreshSession(ctx, refreshToken)
}

// Register implements AuthService.
func (a *AuthServiceImpl) Register(ctx context.Context, username string, email string, password string) error {
	return a.repo.Register(ctx, username, email, password)
}

// UpdatePassword implements AuthService.
func (a *AuthServiceImpl) UpdatePassword(ctx context.Context, userID string, newHashedPassword string) error {
	return a.repo.UpdatePassword(ctx, userID, newHashedPassword)
}

// ValidateCredentials implements AuthService.
func (a *AuthServiceImpl) ValidateCredentials(ctx context.Context, email string, password string) (bool, error) {
	return a.repo.ValidateCredentials(ctx, email, password)
}

// ValidateSession implements AuthService.
func (a *AuthServiceImpl) ValidateSession(ctx context.Context, sessionID string) (bool, error) {
	return a.repo.ValidateSession(ctx, sessionID)
}

// VerifyPassword implements AuthService.
func (a *AuthServiceImpl) VerifyPassword(ctx context.Context, userID string, password string) error {
	return a.repo.VerifyPassword(ctx, userID, password)
}
