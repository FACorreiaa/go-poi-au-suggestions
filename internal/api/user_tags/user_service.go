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
var _ UserTagsService = (*UserTagsServiceImpl)(nil)

// UserService defines the business logic contract for user operations.
type UserTagsService interface {
	//GetAllGlobalTags Global Tags & User Avoid Tags
	GetAllGlobalTags(ctx context.Context) ([]api.GlobalTag, error)
	GetUserAvoidTags(ctx context.Context, userID uuid.UUID) ([]api.UserAvoidTag, error)
	AddUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error
	RemoveUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error
}

// UserTagsServiceImpl provides the implementation for UserService.
type UserTagsServiceImpl struct {
	logger *slog.Logger
	repo   UserTagsRepo
}

// NewUserService creates a new user service instance.
func NewUserService(repo UserTagsRepo, logger *slog.Logger) *UserTagsServiceImpl {
	return &UserTagsServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

// GetAllGlobalTags retrieves all global tags.
func (s *UserTagsServiceImpl) GetAllGlobalTags(ctx context.Context) ([]api.GlobalTag, error) {
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
func (s *UserTagsServiceImpl) GetUserAvoidTags(ctx context.Context, userID uuid.UUID) ([]api.UserAvoidTag, error) {
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
func (s *UserTagsServiceImpl) AddUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
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
func (s *UserTagsServiceImpl) RemoveUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
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
