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
	// GetSettings retrieves the settings profile for a specific user.
	// Returns ErrNotFound if no settings exist for the user (shouldn't happen if trigger works).
	GetSettings(ctx context.Context, userID uuid.UUID) (*UserSettings, error)

	// UpdateSettings updates specific fields in the user's settings profile.
	// Uses pointers in params struct for partial updates. Ensures updated_at is set.
	// Returns ErrNotFound if the user doesn't have a settings row (shouldn't happen).
	UpdateSettings(ctx context.Context, userID uuid.UUID, params UpdateUserSettingsParams) error
}

type SettingsServiceImpl struct {
	logger *slog.Logger
	repo   SettingsRepository
}

// NewUserInterestService creates a new user service instance.
func NewUserSettingsService(repo SettingsRepository, logger *slog.Logger) *SettingsServiceImpl {
	return &SettingsServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

func (s *SettingsServiceImpl) GetSettings(ctx context.Context, userID uuid.UUID) (*UserSettings, error) {
	ctx, span := otel.Tracer("UserInterestService").Start(ctx, "GetUserPreferences", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetUserPreferences"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user preferences")

	settings, err := s.repo.GetSettings(ctx, userID)
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

func (s *SettingsServiceImpl) UpdateSettings(ctx context.Context, userID uuid.UUID, params UpdateUserSettingsParams) error {
	ctx, span := otel.Tracer("UserInterestService").Start(ctx, "UpdateUserPreferences", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	return nil
}
