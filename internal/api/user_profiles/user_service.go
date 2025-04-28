package user

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
)

// Ensure implementation satisfies the interface
var _ UserProfilesService = (*UserProfilesServiceImpl)(nil)

// UserProfilesService defines the business logic contract for user operations.
type UserProfilesService interface {
	//GetUserPreferenceProfiles User  Profiles
	GetUserPreferenceProfiles(ctx context.Context, userID uuid.UUID) ([]api.UserPreferenceProfile, error)
	GetUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) (*api.UserPreferenceProfile, error)
	GetDefaultUserPreferenceProfile(ctx context.Context, userID uuid.UUID) (*api.UserPreferenceProfile, error)
	CreateUserPreferenceProfile(ctx context.Context, userID uuid.UUID, params api.CreateUserPreferenceProfileParams) (*api.UserPreferenceProfile, error)
	UpdateUserPreferenceProfile(ctx context.Context, profileID uuid.UUID, params api.UpdateUserPreferenceProfileParams) error
	DeleteUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error
	SetDefaultUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error
}

// UserServiceImpl provides the implementation for UserService.
type UserProfilesServiceImpl struct {
	logger *slog.Logger
	repo   UserProfilesRepo
}

// NewUserService creates a new user service instance.
func NewUserProfilesService(repo UserProfilesRepo, logger *slog.Logger) *UserProfilesServiceImpl {
	return &UserProfilesServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

// GetUserPreferenceProfiles retrieves all preference profiles for a user.
func (s *UserProfilesServiceImpl) GetUserPreferenceProfiles(ctx context.Context, userID uuid.UUID) ([]api.UserPreferenceProfile, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetUserPreferenceProfiles", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetUserPreferenceProfiles"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user preference profiles")

	profiles, err := s.repo.GetUserPreferenceProfiles(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user preference profiles", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user preference profiles")
		return nil, fmt.Errorf("error fetching user preference profiles: %w", err)
	}

	l.InfoContext(ctx, "User preference profiles fetched successfully", slog.Int("count", len(profiles)))
	span.SetStatus(codes.Ok, "User preference profiles fetched successfully")
	return profiles, nil
}

// GetUserPreferenceProfile retrieves a specific preference profile by ID.
func (s *UserProfilesServiceImpl) GetUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) (*api.UserPreferenceProfile, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetUserPreferenceProfile", trace.WithAttributes(
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Fetching user preference profile")

	profile, err := s.repo.GetUserPreferenceProfile(ctx, profileID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user preference profile")
		return nil, fmt.Errorf("error fetching user preference profile: %w", err)
	}

	l.InfoContext(ctx, "User preference profile fetched successfully")
	span.SetStatus(codes.Ok, "User preference profile fetched successfully")
	return profile, nil
}

// GetDefaultUserPreferenceProfile retrieves the default preference profile for a user.
func (s *UserProfilesServiceImpl) GetDefaultUserPreferenceProfile(ctx context.Context, userID uuid.UUID) (*api.UserPreferenceProfile, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetDefaultUserPreferenceProfile", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetDefaultUserPreferenceProfile"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching default user preference profile")

	profile, err := s.repo.GetDefaultUserPreferenceProfile(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch default user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch default user preference profile")
		return nil, fmt.Errorf("error fetching default user preference profile: %w", err)
	}

	l.InfoContext(ctx, "Default user preference profile fetched successfully")
	span.SetStatus(codes.Ok, "Default user preference profile fetched successfully")
	return profile, nil
}

// CreateUserPreferenceProfile creates a new preference profile for a user.
func (s *UserProfilesServiceImpl) CreateUserPreferenceProfile(ctx context.Context, userID uuid.UUID, params api.CreateUserPreferenceProfileParams) (*api.UserPreferenceProfile, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "CreateUserPreferenceProfile", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "CreateUserPreferenceProfile"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Creating user preference profile", slog.String("profileName", params.ProfileName))

	profile, err := s.repo.CreateUserPreferenceProfile(ctx, userID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to create user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create user preference profile")
		return nil, fmt.Errorf("error creating user preference profile: %w", err)
	}

	l.InfoContext(ctx, "User preference profile created successfully", slog.String("profileID", profile.ID.String()))
	span.SetStatus(codes.Ok, "User preference profile created successfully")
	return profile, nil
}

// UpdateUserPreferenceProfile updates a preference profile.
func (s *UserProfilesServiceImpl) UpdateUserPreferenceProfile(ctx context.Context, profileID uuid.UUID, params api.UpdateUserPreferenceProfileParams) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "UpdateUserPreferenceProfile", trace.WithAttributes(
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "UpdateUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Updating user preference profile")

	err := s.repo.UpdateUserPreferenceProfile(ctx, profileID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update user preference profile")
		return fmt.Errorf("error updating user preference profile: %w", err)
	}

	l.InfoContext(ctx, "User preference profile updated successfully")
	span.SetStatus(codes.Ok, "User preference profile updated successfully")
	return nil
}

// DeleteUserPreferenceProfile deletes a preference profile.
func (s *UserProfilesServiceImpl) DeleteUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "DeleteUserPreferenceProfile", trace.WithAttributes(
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "DeleteUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Deleting user preference profile")

	err := s.repo.DeleteUserPreferenceProfile(ctx, profileID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to delete user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to delete user preference profile")
		return fmt.Errorf("error deleting user preference profile: %w", err)
	}

	l.InfoContext(ctx, "User preference profile deleted successfully")
	span.SetStatus(codes.Ok, "User preference profile deleted successfully")
	return nil
}

// SetDefaultUserPreferenceProfile sets a profile as the default for a user.
func (s *UserProfilesServiceImpl) SetDefaultUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "SetDefaultUserPreferenceProfile", trace.WithAttributes(
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "SetDefaultUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Setting profile as default")

	err := s.repo.SetDefaultUserPreferenceProfile(ctx, profileID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to set default user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to set default user preference profile")
		return fmt.Errorf("error setting default user preference profile: %w", err)
	}

	l.InfoContext(ctx, "User preference profile set as default successfully")
	span.SetStatus(codes.Ok, "User preference profile set as default successfully")
	return nil
}
