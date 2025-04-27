package user

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
)

// Ensure implementation satisfies the interface
var _ UserService = (*ServiceUserImpl)(nil)

// UserService defines the business logic contract for user operations.
type UserService interface {
	// GetUserProfile Profile Management
	GetUserProfile(ctx context.Context, userID uuid.UUID) (*api.UserProfile, error)
	UpdateUserProfile(ctx context.Context, userID uuid.UUID, params api.UpdateProfileParams) error

	// UpdateLastLogin Status & Activity
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error
	MarkEmailAsVerified(ctx context.Context, userID uuid.UUID) error
	DeactivateUser(ctx context.Context, userID uuid.UUID) error
	ReactivateUser(ctx context.Context, userID uuid.UUID) error
}

// ServiceUserImpl provides the implementation for UserService.
type ServiceUserImpl struct {
	logger *slog.Logger
	repo   UserRepo
}

// NewUserService creates a new user service instance.
func NewUserService(repo UserRepo, logger *slog.Logger) *ServiceUserImpl {
	return &ServiceUserImpl{
		logger: logger,
		repo:   repo,
	}
}

// GetUserProfile retrieves a user's profile by ID.
func (s *ServiceUserImpl) GetUserProfile(ctx context.Context, userID uuid.UUID) (*api.UserProfile, error) {
	l := s.logger.With(slog.String("method", "GetUserProfile"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user profile")

	profile, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user profile", slog.Any("error", err))
		return nil, fmt.Errorf("error fetching user profile: %w", err)
	}

	l.InfoContext(ctx, "User profile fetched successfully")
	return profile, nil
}

// UpdateUserProfile updates a user's profile.
func (s *ServiceUserImpl) UpdateUserProfile(ctx context.Context, userID uuid.UUID, params api.UpdateProfileParams) error {
	l := s.logger.With(slog.String("method", "UpdateUserProfile"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Updating user profile")

	err := s.repo.UpdateProfile(ctx, userID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user profile", slog.Any("error", err))
		return fmt.Errorf("error updating user profile: %w", err)
	}

	l.InfoContext(ctx, "User profile updated successfully")
	return nil
}

// UpdateLastLogin updates the last login timestamp for a user.
func (s *ServiceUserImpl) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	l := s.logger.With(slog.String("method", "UpdateLastLogin"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Updating user last login timestamp")

	err := s.repo.UpdateLastLogin(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user last login timestamp", slog.Any("error", err))
		return fmt.Errorf("error updating user last login timestamp: %w", err)
	}

	l.InfoContext(ctx, "User last login timestamp updated successfully")
	return nil
}

// MarkEmailAsVerified marks a user's email as verified.
func (s *ServiceUserImpl) MarkEmailAsVerified(ctx context.Context, userID uuid.UUID) error {
	l := s.logger.With(slog.String("method", "MarkEmailAsVerified"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Marking user email as verified")

	err := s.repo.MarkEmailAsVerified(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to mark user email as verified", slog.Any("error", err))
		return fmt.Errorf("error marking user email as verified: %w", err)
	}

	l.InfoContext(ctx, "User email marked as verified successfully")
	return nil
}

// DeactivateUser deactivates a user.
func (s *ServiceUserImpl) DeactivateUser(ctx context.Context, userID uuid.UUID) error {
	l := s.logger.With(slog.String("method", "DeactivateUser"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Deactivating user")

	err := s.repo.DeactivateUser(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to deactivate user", slog.Any("error", err))
		return fmt.Errorf("error deactivating user: %w", err)
	}

	l.InfoContext(ctx, "User deactivated successfully")
	return nil
}

// ReactivateUser reactivates a user.
func (s *ServiceUserImpl) ReactivateUser(ctx context.Context, userID uuid.UUID) error {
	l := s.logger.With(slog.String("method", "ReactivateUser"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Reactivating user")

	err := s.repo.ReactivateUser(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to reactivate user", slog.Any("error", err))
		return fmt.Errorf("error reactivating user: %w", err)
	}

	l.InfoContext(ctx, "User reactivated successfully")
	return nil
}
