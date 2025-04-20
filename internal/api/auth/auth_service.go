package auth

import "context"

var _ AuthService = (*IAuthService)(nil)

type AuthService interface {
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

type IAuthService struct {
	repo AuthRepo
}

func NewAuthService(repo AuthRepo) *IAuthService {
	return &IAuthService{
		repo: repo,
	}
}

// GetSession implements AuthService.
func (i *IAuthService) GetSession(ctx context.Context, tenant string, sessionID string) (*Session, error) {
	panic("unimplemented")
}

// GetUserRole implements AuthService.
func (i *IAuthService) GetUserRole(ctx context.Context, tenant string, actingAdminID string) (string, error) {
	panic("unimplemented")
}

// InvalidateAllUserRefreshTokens implements AuthService.
func (i *IAuthService) InvalidateAllUserRefreshTokens(ctx context.Context, tenant string, userID string) error {
	panic("unimplemented")
}

// Login implements AuthService.
func (i *IAuthService) Login(ctx context.Context, tenant string, email string, password string) (string, string, error) {
	panic("unimplemented")
}

// Logout implements AuthService.
func (i *IAuthService) Logout(ctx context.Context, tenant string, sessionID string) error {
	panic("unimplemented")
}

// RefreshSession implements AuthService.
func (i *IAuthService) RefreshSession(ctx context.Context, tenant string, refreshToken string) (string, string, error) {
	panic("unimplemented")
}

// Register implements AuthService.
func (i *IAuthService) Register(ctx context.Context, tenant string, username string, email string, password string, role string) error {
	panic("unimplemented")
}

// UpdatePassword implements AuthService.
func (i *IAuthService) UpdatePassword(ctx context.Context, tenant string, userID string, newHashedPassword string) error {
	panic("unimplemented")
}

// ValidateCredentials implements AuthService.
func (i *IAuthService) ValidateCredentials(ctx context.Context, tenant string, email string, password string) (bool, error) {
	panic("unimplemented")
}

// ValidateSession implements AuthService.
func (i *IAuthService) ValidateSession(ctx context.Context, tenant string, sessionID string) (bool, error) {
	panic("unimplemented")
}

// VerifyPassword implements AuthService.
func (i *IAuthService) VerifyPassword(ctx context.Context, tenant string, userID string, password string) error {
	panic("unimplemented")
}
