package interests

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

// Ensure implementation satisfies the interface
var _ interestsService = (*interestsServiceImpl)(nil)

// interestsService defines the business logic contract for user operations.
type interestsService interface {
	//Removeinterests remove interests
	Removeinterests(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error
	GetAllInterests(ctx context.Context) ([]*types.Interest, error)
	CreateInterest(ctx context.Context, name string, description *string, isActive bool, userID string) (*types.Interest, error)
	Updateinterests(ctx context.Context, userID uuid.UUID, interestID uuid.UUID, params types.UpdateinterestsParams) error
}

// interestsServiceImpl provides the implementation for interestsService.
type interestsServiceImpl struct {
	logger *slog.Logger
	repo   Repository
}

// NewinterestsService creates a new user service instance.
func NewinterestsService(repo Repository, logger *slog.Logger) *interestsServiceImpl {
	return &interestsServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

// CreateInterest create user interest
func (s *interestsServiceImpl) CreateInterest(ctx context.Context, name string, description *string, isActive bool, userID string) (*types.Interest, error) {
	ctx, span := otel.Tracer("interestsService").Start(ctx, "Createinterests", trace.WithAttributes(
		attribute.String("name", name),
		attribute.String("description", *description),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "Createinterests"),
		slog.String("name", name), slog.String("description", *description))
	l.DebugContext(ctx, "Adding user interest")

	interest, err := s.repo.CreateInterest(ctx, name, description, isActive, userID)
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

// Removeinterests removes an interest from a user's preferences.
func (s *interestsServiceImpl) Removeinterests(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error {
	ctx, span := otel.Tracer("interestsService").Start(ctx, "Removeinterests", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("interest.id", interestID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "Removeinterests"), slog.String("userID", userID.String()), slog.String("interestID", interestID.String()))
	l.DebugContext(ctx, "Removing user interest")

	err := s.repo.Removeinterests(ctx, userID, interestID)
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
func (s *interestsServiceImpl) GetAllInterests(ctx context.Context) ([]*types.Interest, error) {
	ctx, span := otel.Tracer("interestsService").Start(ctx, "GetAllInterests")
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

func (s *interestsServiceImpl) Updateinterests(ctx context.Context, userID uuid.UUID, interestID uuid.UUID, params types.UpdateinterestsParams) error {
	ctx, span := otel.Tracer("interestsService").Start(ctx, "Updateinterests", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("interest.id", interestID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "Updateinterests"), slog.String("userID", userID.String()), slog.String("interestID", interestID.String()))
	l.DebugContext(ctx, "Updating user interest")

	err := s.repo.Updateinterests(ctx, userID, interestID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update user interest")
		return fmt.Errorf("error updating user interest: %w", err)
	}
	return nil
}

// GetUserEnhancedInterests retrieves a user's enhanced interests.
//func (s *interestsServiceImpl) GetUserEnhancedInterests(ctx context.Context, userID uuid.UUID) ([]types.EnhancedInterest, error) {
//	ctx, span := otel.Tracer("interestsService").Start(ctx, "GetUserEnhancedInterests", trace.WithAttributes(
//		attribute.String("user.id", userID.String()),
//	))
//	defer span.End()
//
//	l := s.logger.With(slog.String("method", "GetUserEnhancedInterests"), slog.String("userID", userID.String()))
//	l.DebugContext(ctx, "Fetching user enhanced interests")
//
//	interests, err := s.repo.GetUserEnhancedInterests(ctx, userID)
//	if err != nil {
//		l.ErrorContext(ctx, "Failed to fetch user enhanced interests", slog.Any("error", err))
//		span.RecordError(err)
//		span.SetStatus(codes.Error, "Failed to fetch user enhanced interests")
//		return nil, fmt.Errorf("error fetching user enhanced interests: %w", err)
//	}
//
//	l.InfoContext(ctx, "User enhanced interests fetched successfully", slog.Int("count", len(interests)))
//	span.SetStatus(codes.Ok, "User enhanced interests fetched successfully")
//	return interests, nil
//}
