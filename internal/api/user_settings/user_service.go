package userSettings

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var _ SettingsService = (*SettingsServiceImpl)(nil)

type SettingsService interface {
	// GetUserSettings retrieves the settings profile for a specific user.
	// Returns ErrNotFound if no settings exist for the user (shouldn't happen if trigger works).
	GetUserSettings(ctx context.Context, userID uuid.UUID) (*UserSettings, error)

	// UpdateUserSettings updates specific fields in the user's settings profile.
	// Uses pointers in params struct for partial updates. Ensures updated_at is set.
	// Returns ErrNotFound if the user doesn't have a settings row (shouldn't happen).
	UpdateUserSettings(ctx context.Context, userID, profileID uuid.UUID, params UpdateUserSettingsParams) error
}

type SettingsServiceImpl struct {
	logger *slog.Logger
	repo   SettingsRepository
}

// NewUserSettingsService creates a new user service instance.
func NewUserSettingsService(repo SettingsRepository, logger *slog.Logger) *SettingsServiceImpl {
	return &SettingsServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

func (s *SettingsServiceImpl) GetUserSettings(ctx context.Context, userID uuid.UUID) (*UserSettings, error) {
	ctx, span := otel.Tracer("UserInterestService").Start(ctx, "GetUserSettings", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetUserSettings"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user preferences")

	settings, err := s.repo.Get(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user settings", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user settings")
		return nil, fmt.Errorf("error fetching user settings: %w", err)
	}

	l.InfoContext(ctx, "User settings fetched successfully")
	span.SetStatus(codes.Ok, "User settings fetched successfully")
	return settings, nil
}

func (s *SettingsServiceImpl) UpdateUserSettings(ctx context.Context, userID, profileID uuid.UUID, params UpdateUserSettingsParams) error {
	ctx, span := otel.Tracer("UserInterestService").Start(ctx, "UpdateUserSettings", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "UpdateUserSettings"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Updating user preferences")

	err := s.repo.Update(ctx, userID, profileID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user settings", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update user settings")
		return fmt.Errorf("error updating user settings: %w", err)
	}
	span.SetStatus(codes.Ok, "User settings updated successfully")
	return nil
}
