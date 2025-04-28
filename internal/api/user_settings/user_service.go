package userSettings

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
)

var _ UserSettingsService = (*UserSettingsServiceImpl)(nil)

type UserSettingsService interface {
	GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]*api.Interest, error) // Fetches from user_interests join
}

type UserSettingsServiceImpl struct {
	logger *slog.Logger
	repo   UserSettingsRepository
}

// NewUserInterestService creates a new user service instance.
func NewUserSettingsService(repo UserSettingsRepository, logger *slog.Logger) *UserSettingsServiceImpl {
	return &UserSettingsServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

func (s *UserSettingsServiceImpl) GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]*api.Interest, error) {
	ctx, span := otel.Tracer("UserInterestService").Start(ctx, "GetUserPreferences", trace.WithAttributes(
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
