package userInterest

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
var _ UserInterestService = (*UserInterestServiceImpl)(nil)

// UserInterestService defines the business logic contract for user operations.
type UserInterestService interface {
	//RemoveUserInterest remove interests
	RemoveUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error
	GetAllInterests(ctx context.Context) ([]*types.Interest, error)
	CreateInterest(ctx context.Context, name string, description *string, isActive bool, userID string) (*types.Interest, error)
	UpdateUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID, params types.UpdateUserInterestParams) error
}

// UserInterestServiceImpl provides the implementation for UserInterestService.
type UserInterestServiceImpl struct {
	logger *slog.Logger
	repo   UserInterestRepo
}

// NewUserInterestService creates a new user service instance.
func NewUserInterestService(repo UserInterestRepo, logger *slog.Logger) *UserInterestServiceImpl {
	return &UserInterestServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

// CreateInterest create user interest
func (s *UserInterestServiceImpl) CreateInterest(ctx context.Context, name string, description *string, isActive bool, userID string) (*types.Interest, error) {
	ctx, span := otel.Tracer("UserInterestService").Start(ctx, "CreateUserInterest", trace.WithAttributes(
		attribute.String("name", name),
		attribute.String("description", *description),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "CreateUserInterest"),
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

// RemoveUserInterest removes an interest from a user's preferences.
func (s *UserInterestServiceImpl) RemoveUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error {
	ctx, span := otel.Tracer("UserInterestService").Start(ctx, "RemoveUserInterest", trace.WithAttributes(
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
func (s *UserInterestServiceImpl) GetAllInterests(ctx context.Context) ([]*types.Interest, error) {
	ctx, span := otel.Tracer("UserInterestService").Start(ctx, "GetAllInterests")
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

func (s *UserInterestServiceImpl) UpdateUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID, params types.UpdateUserInterestParams) error {
	ctx, span := otel.Tracer("UserInterestService").Start(ctx, "UpdateUserInterest", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("interest.id", interestID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "UpdateUserInterest"), slog.String("userID", userID.String()), slog.String("interestID", interestID.String()))
	l.DebugContext(ctx, "Updating user interest")

	err := s.repo.UpdateUserInterest(ctx, userID, interestID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update user interest")
		return fmt.Errorf("error updating user interest: %w", err)
	}
	return nil
}

// GetUserEnhancedInterests retrieves a user's enhanced interests.
//func (s *UserInterestServiceImpl) GetUserEnhancedInterests(ctx context.Context, userID uuid.UUID) ([]types.EnhancedInterest, error) {
//	ctx, span := otel.Tracer("UserInterestService").Start(ctx, "GetUserEnhancedInterests", trace.WithAttributes(
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
