package settings

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var _ SettingsService = (*SettingsServiceImpl)(nil)

type SettingsService interface {
	// Getsettings retrieves the settings profile for a specific user.
	// Returns ErrNotFound if no settings exist for the user (shouldn't happen if trigger works).
	Getsettings(ctx context.Context, userID uuid.UUID) (*types.Settings, error)

	// Updatesettings updates specific fields in the user's settings profile.
	// Uses pointers in params struct for partial updates. Ensures updated_at is set.
	// Returns ErrNotFound if the user doesn't have a settings row (shouldn't happen).
	Updatesettings(ctx context.Context, userID, profileID uuid.UUID, params types.UpdatesettingsParams) error
}

type SettingsServiceImpl struct {
	logger *slog.Logger
	repo   SettingsRepository
}

// NewsettingsService creates a new user service instance.
func NewsettingsService(repo SettingsRepository, logger *slog.Logger) *SettingsServiceImpl {
	return &SettingsServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

func (s *SettingsServiceImpl) Getsettings(ctx context.Context, userID uuid.UUID) (*types.Settings, error) {
	ctx, span := otel.Tracer("interestsService").Start(ctx, "Getsettings", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "Getsettings"), slog.String("userID", userID.String()))
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

func (s *SettingsServiceImpl) Updatesettings(ctx context.Context, userID, profileID uuid.UUID, params types.UpdatesettingsParams) error {
	ctx, span := otel.Tracer("interestsService").Start(ctx, "Updatesettings", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "Updatesettings"), slog.String("userID", userID.String()))
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
