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
var _ UserService = (*UserServiceImpl)(nil)

// UserService defines the business logic contract for user operations.
type UserService interface {
	// Profile Management
	GetUserProfile(ctx context.Context, userID uuid.UUID) (*api.UserProfile, error)
	UpdateUserProfile(ctx context.Context, userID uuid.UUID, params api.UpdateProfileParams) error

	// User Preferences
	GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]api.Interest, error)
	AddUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error
	RemoveUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error
	GetAllInterests(ctx context.Context) ([]api.Interest, error)
	CreateInterest(ctx context.Context, name string, description *string, isActive bool) (*api.Interest, error)
	UpdateUserInterestPreferenceLevel(ctx context.Context, userID uuid.UUID, interestID uuid.UUID, preferenceLevel int) error
	GetUserEnhancedInterests(ctx context.Context, userID uuid.UUID) ([]api.EnhancedInterest, error)

	// User Preference Profiles
	GetUserPreferenceProfiles(ctx context.Context, userID uuid.UUID) ([]api.UserPreferenceProfile, error)
	GetUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) (*api.UserPreferenceProfile, error)
	GetDefaultUserPreferenceProfile(ctx context.Context, userID uuid.UUID) (*api.UserPreferenceProfile, error)
	CreateUserPreferenceProfile(ctx context.Context, userID uuid.UUID, params api.CreateUserPreferenceProfileParams) (*api.UserPreferenceProfile, error)
	UpdateUserPreferenceProfile(ctx context.Context, profileID uuid.UUID, params api.UpdateUserPreferenceProfileParams) error
	DeleteUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error
	SetDefaultUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error

	// Global Tags & User Avoid Tags
	GetAllGlobalTags(ctx context.Context) ([]api.GlobalTag, error)
	GetUserAvoidTags(ctx context.Context, userID uuid.UUID) ([]api.UserAvoidTag, error)
	AddUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error
	RemoveUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error

	// Status & Activity
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error
	MarkEmailAsVerified(ctx context.Context, userID uuid.UUID) error
	DeactivateUser(ctx context.Context, userID uuid.UUID) error
	ReactivateUser(ctx context.Context, userID uuid.UUID) error
}

// UserServiceImpl provides the implementation for UserService.
type UserServiceImpl struct {
	logger *slog.Logger
	repo   UserRepo
}

// NewUserService creates a new user service instance.
func NewUserService(repo UserRepo, logger *slog.Logger) *UserServiceImpl {
	return &UserServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

// GetUserProfile retrieves a user's profile by ID.
func (s *UserServiceImpl) GetUserProfile(ctx context.Context, userID uuid.UUID) (*api.UserProfile, error) {
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
func (s *UserServiceImpl) UpdateUserProfile(ctx context.Context, userID uuid.UUID, params api.UpdateProfileParams) error {
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

// GetUserPreferences retrieves a user's preferences.
func (s *UserServiceImpl) GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]api.Interest, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetUserPreferences", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetUserPreferences"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user preferences")

	preferences, err := s.repo.GetUserPreferences(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user preferences")
		return nil, fmt.Errorf("error fetching user preferences: %w", err)
	}

	l.InfoContext(ctx, "User preferences fetched successfully", slog.Int("count", len(preferences)))
	span.SetStatus(codes.Ok, "User preferences fetched successfully")
	return preferences, nil
}

// CreateInterest create user interest
func (s *UserServiceImpl) CreateInterest(ctx context.Context, name string, description *string, isActive bool) (*api.Interest, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "CreateUserInterest", trace.WithAttributes(
		attribute.String("name", name),
		attribute.String("description", *description),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "CreateUserInterest"),
		slog.String("name", name), slog.String("description", *description))
	l.DebugContext(ctx, "Adding user interest")

	interest, err := s.repo.CreateInterest(ctx, name, description, isActive)
	if err != nil {
		l.ErrorContext(ctx, "Failed to add user interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to add user interest")
		return nil, fmt.Errorf("error adding user interest: %w", err)
	}

	l.InfoContext(ctx, "User interest created successfully")
	span.SetStatus(codes.Ok, "User interest created successfully")
	return interest, nil
}

// AddUserInterest adds an interest to a user's preferences.
func (s *UserServiceImpl) AddUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "AddUserInterest", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("interest.id", interestID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "AddUserInterest"), slog.String("userID", userID.String()), slog.String("interestID", interestID.String()))
	l.DebugContext(ctx, "Adding user interest")

	err := s.repo.AddUserInterest(ctx, userID, interestID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to add user interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to add user interest")
		return fmt.Errorf("error adding user interest: %w", err)
	}

	l.InfoContext(ctx, "User interest added successfully")
	span.SetStatus(codes.Ok, "User interest added successfully")
	return nil
}

// RemoveUserInterest removes an interest from a user's preferences.
func (s *UserServiceImpl) RemoveUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "RemoveUserInterest", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("interest.id", interestID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "RemoveUserInterest"), slog.String("userID", userID.String()), slog.String("interestID", interestID.String()))
	l.DebugContext(ctx, "Removing user interest")

	err := s.repo.RemoveUserInterest(ctx, userID, interestID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to remove user interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to remove user interest")
		return fmt.Errorf("error removing user interest: %w", err)
	}

	l.InfoContext(ctx, "User interest removed successfully")
	span.SetStatus(codes.Ok, "User interest removed successfully")
	return nil
}

// GetAllInterests retrieves all available interests.
func (s *UserServiceImpl) GetAllInterests(ctx context.Context) ([]api.Interest, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetAllInterests")
	defer span.End()

	l := s.logger.With(slog.String("method", "GetAllInterests"))
	l.DebugContext(ctx, "Fetching all interests")

	interests, err := s.repo.GetAllInterests(ctx)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch all interests", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch all interests")
		return nil, fmt.Errorf("error fetching all interests: %w", err)
	}

	l.InfoContext(ctx, "All interests fetched successfully", slog.Int("count", len(interests)))
	span.SetStatus(codes.Ok, "All interests fetched successfully")
	return interests, nil
}

// UpdateUserInterestPreferenceLevel updates the preference level for a user interest.
func (s *UserServiceImpl) UpdateUserInterestPreferenceLevel(ctx context.Context, userID uuid.UUID, interestID uuid.UUID, preferenceLevel int) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "UpdateUserInterestPreferenceLevel", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("interest.id", interestID.String()),
		attribute.Int("preference_level", preferenceLevel),
	))
	defer span.End()

	l := s.logger.With(
		slog.String("method", "UpdateUserInterestPreferenceLevel"),
		slog.String("userID", userID.String()),
		slog.String("interestID", interestID.String()),
		slog.Int("preferenceLevel", preferenceLevel),
	)
	l.DebugContext(ctx, "Updating user interest preference level")

	// Validate preference level
	if preferenceLevel < 0 {
		err := fmt.Errorf("preference level must be non-negative: %d", preferenceLevel)
		l.ErrorContext(ctx, "Invalid preference level", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid preference level")
		return err
	}

	err := s.repo.UpdateUserInterestPreferenceLevel(ctx, userID, interestID, preferenceLevel)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user interest preference level", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update user interest preference level")
		return fmt.Errorf("error updating user interest preference level: %w", err)
	}

	l.InfoContext(ctx, "User interest preference level updated successfully")
	span.SetStatus(codes.Ok, "User interest preference level updated successfully")
	return nil
}

// GetUserEnhancedInterests retrieves a user's enhanced interests.
func (s *UserServiceImpl) GetUserEnhancedInterests(ctx context.Context, userID uuid.UUID) ([]api.EnhancedInterest, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetUserEnhancedInterests", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetUserEnhancedInterests"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user enhanced interests")

	interests, err := s.repo.GetUserEnhancedInterests(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user enhanced interests", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user enhanced interests")
		return nil, fmt.Errorf("error fetching user enhanced interests: %w", err)
	}

	l.InfoContext(ctx, "User enhanced interests fetched successfully", slog.Int("count", len(interests)))
	span.SetStatus(codes.Ok, "User enhanced interests fetched successfully")
	return interests, nil
}

// GetUserPreferenceProfiles retrieves all preference profiles for a user.
func (s *UserServiceImpl) GetUserPreferenceProfiles(ctx context.Context, userID uuid.UUID) ([]api.UserPreferenceProfile, error) {
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
func (s *UserServiceImpl) GetUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) (*api.UserPreferenceProfile, error) {
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
func (s *UserServiceImpl) GetDefaultUserPreferenceProfile(ctx context.Context, userID uuid.UUID) (*api.UserPreferenceProfile, error) {
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
func (s *UserServiceImpl) CreateUserPreferenceProfile(ctx context.Context, userID uuid.UUID, params api.CreateUserPreferenceProfileParams) (*api.UserPreferenceProfile, error) {
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
func (s *UserServiceImpl) UpdateUserPreferenceProfile(ctx context.Context, profileID uuid.UUID, params api.UpdateUserPreferenceProfileParams) error {
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
func (s *UserServiceImpl) DeleteUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error {
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
func (s *UserServiceImpl) SetDefaultUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error {
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

// GetAllGlobalTags retrieves all global tags.
func (s *UserServiceImpl) GetAllGlobalTags(ctx context.Context) ([]api.GlobalTag, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetAllGlobalTags")
	defer span.End()

	l := s.logger.With(slog.String("method", "GetAllGlobalTags"))
	l.DebugContext(ctx, "Fetching all global tags")

	tags, err := s.repo.GetAllGlobalTags(ctx)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch all global tags", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch all global tags")
		return nil, fmt.Errorf("error fetching all global tags: %w", err)
	}

	l.InfoContext(ctx, "All global tags fetched successfully", slog.Int("count", len(tags)))
	span.SetStatus(codes.Ok, "All global tags fetched successfully")
	return tags, nil
}

// GetUserAvoidTags retrieves all avoid tags for a user.
func (s *UserServiceImpl) GetUserAvoidTags(ctx context.Context, userID uuid.UUID) ([]api.UserAvoidTag, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetUserAvoidTags", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetUserAvoidTags"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user avoid tags")

	tags, err := s.repo.GetUserAvoidTags(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user avoid tags", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user avoid tags")
		return nil, fmt.Errorf("error fetching user avoid tags: %w", err)
	}

	l.InfoContext(ctx, "User avoid tags fetched successfully", slog.Int("count", len(tags)))
	span.SetStatus(codes.Ok, "User avoid tags fetched successfully")
	return tags, nil
}

// AddUserAvoidTag adds an avoid tag for a user.
func (s *UserServiceImpl) AddUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "AddUserAvoidTag", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("tag.id", tagID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "AddUserAvoidTag"), slog.String("userID", userID.String()), slog.String("tagID", tagID.String()))
	l.DebugContext(ctx, "Adding user avoid tag")

	err := s.repo.AddUserAvoidTag(ctx, userID, tagID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to add user avoid tag", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to add user avoid tag")
		return fmt.Errorf("error adding user avoid tag: %w", err)
	}

	l.InfoContext(ctx, "User avoid tag added successfully")
	span.SetStatus(codes.Ok, "User avoid tag added successfully")
	return nil
}

// RemoveUserAvoidTag removes an avoid tag for a user.
func (s *UserServiceImpl) RemoveUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "RemoveUserAvoidTag", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("tag.id", tagID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "RemoveUserAvoidTag"), slog.String("userID", userID.String()), slog.String("tagID", tagID.String()))
	l.DebugContext(ctx, "Removing user avoid tag")

	err := s.repo.RemoveUserAvoidTag(ctx, userID, tagID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to remove user avoid tag", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to remove user avoid tag")
		return fmt.Errorf("error removing user avoid tag: %w", err)
	}

	l.InfoContext(ctx, "User avoid tag removed successfully")
	span.SetStatus(codes.Ok, "User avoid tag removed successfully")
	return nil
}

// UpdateLastLogin updates the last login timestamp for a user.
func (s *UserServiceImpl) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
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
func (s *UserServiceImpl) MarkEmailAsVerified(ctx context.Context, userID uuid.UUID) error {
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
func (s *UserServiceImpl) DeactivateUser(ctx context.Context, userID uuid.UUID) error {
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
func (s *UserServiceImpl) ReactivateUser(ctx context.Context, userID uuid.UUID) error {
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
