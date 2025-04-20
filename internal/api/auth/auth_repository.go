package auth

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ AuthRepo = (*AuthRepoFactory)(nil)

type Session struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type AuthRepo interface {
	Login(ctx context.Context, tenant, email, password string) (string, string, error)
	Logout(ctx context.Context, tenant, sessionID string) error
	GetSession(ctx context.Context, tenant, sessionID string) (*Session, error)
	RefreshSession(ctx context.Context, tenant, refreshToken string) (string, string, error) // accessToken, refreshToken, error
	Register(ctx context.Context, tenant, username, email, password, role string) error
	ValidateCredentials(ctx context.Context, tenant, email, password string) (bool, error)
	ValidateSession(ctx context.Context, tenant, sessionID string) (bool, error)
	GetUserRole(ctx context.Context, tenant, actingAdminID string) (string, error)
	VerifyPassword(ctx context.Context, tenant, userID, password string) error
	UpdatePassword(ctx context.Context, tenant, userID, newHashedPassword string) error
	InvalidateAllUserRefreshTokens(ctx context.Context, tenant, userID string) error
}

type AuthRepoFactory struct {
	pgxpool *pgxpool.Pool
}

func NewAuthRepoFactory(pgxpool *pgxpool.Pool) *AuthRepoFactory {
	return &AuthRepoFactory{
		pgxpool: pgxpool,
	}
}

// GetSession implements AuthRepo.
func (a *AuthRepoFactory) GetSession(ctx context.Context, tenant string, sessionID string) (*Session, error) {
	panic("unimplemented")
}

// GetUserRole implements AuthRepo.
func (a *AuthRepoFactory) GetUserRole(ctx context.Context, tenant string, actingAdminID string) (string, error) {
	panic("unimplemented")
}

// InvalidateAllUserRefreshTokens implements AuthRepo.
func (a *AuthRepoFactory) InvalidateAllUserRefreshTokens(ctx context.Context, tenant string, userID string) error {
	panic("unimplemented")
}

// Login implements AuthRepo.
func (a *AuthRepoFactory) Login(ctx context.Context, tenant string, email string, password string) (string, string, error) {
	panic("unimplemented")
}

// Logout implements AuthRepo.
func (a *AuthRepoFactory) Logout(ctx context.Context, tenant string, sessionID string) error {
	panic("unimplemented")
}

// RefreshSession implements AuthRepo.
func (a *AuthRepoFactory) RefreshSession(ctx context.Context, tenant string, refreshToken string) (string, string, error) {
	panic("unimplemented")
}

// Register implements AuthRepo.
func (a *AuthRepoFactory) Register(ctx context.Context, tenant string, username string, email string, password string, role string) error {
	panic("unimplemented")
}

// UpdatePassword implements AuthRepo.
func (a *AuthRepoFactory) UpdatePassword(ctx context.Context, tenant string, userID string, newHashedPassword string) error {
	panic("unimplemented")
}

// ValidateCredentials implements AuthRepo.
func (a *AuthRepoFactory) ValidateCredentials(ctx context.Context, tenant string, email string, password string) (bool, error) {
	panic("unimplemented")
}

// ValidateSession implements AuthRepo.
func (a *AuthRepoFactory) ValidateSession(ctx context.Context, tenant string, sessionID string) (bool, error) {
	panic("unimplemented")
}

// VerifyPassword implements AuthRepo.
func (a *AuthRepoFactory) VerifyPassword(ctx context.Context, tenant string, userID string, password string) error {
	panic("unimplemented")
}
