package userTags

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
var _ UserTagsService = (*UserTagsServiceImpl)(nil)

// UserTagsService defines the business logic contract for user operations.
type UserTagsService interface {
	GetTags(ctx context.Context, userID uuid.UUID) ([]types.Tags, error)
	GetTag(ctx context.Context, userID, tagID uuid.UUID) (*types.Tags, error)
	CreateTag(ctx context.Context, userID uuid.UUID, params types.CreatePersonalTagParams) (*types.PersonalTag, error)
	DeleteTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error
	Update(ctx context.Context, userID uuid.UUID, tagID uuid.UUID, params types.UpdatePersonalTagParams) error
}

// UserTagsServiceImpl provides the implementation for UserService.
type UserTagsServiceImpl struct {
	logger *slog.Logger
	repo   UserTagsRepo
}

// NewUserTagsService creates a new user service instance.
func NewUserTagsService(repo UserTagsRepo, logger *slog.Logger) *UserTagsServiceImpl {
	return &UserTagsServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

// GetTags retrieves all global tags.
func (s *UserTagsServiceImpl) GetTags(ctx context.Context, userID uuid.UUID) ([]types.Tags, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetAllGlobalTags")
	defer span.End()

	l := s.logger.With(slog.String("method", "GetAllGlobalTags"))
	l.DebugContext(ctx, "Fetching all global tags")

	tags, err := s.repo.GetAll(ctx, userID)
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

// GetTag retrieves all avoid tags for a user.
func (s *UserTagsServiceImpl) GetTag(ctx context.Context, userID, tagID uuid.UUID) (*types.Tags, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetUserAvoidTags", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetUserAvoidTags"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user avoid tags")

	tag, err := s.repo.Get(ctx, userID, tagID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user avoid tags", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user avoid tags")
		return nil, fmt.Errorf("error fetching user avoid tags: %w", err)
	}

	l.InfoContext(ctx, "User avoid tags fetched successfully")
	span.SetStatus(codes.Ok, "User avoid tags fetched successfully")
	return tag, nil
}

// CreateTag adds an avoid tag for a user.
func (s *UserTagsServiceImpl) CreateTag(ctx context.Context, userID uuid.UUID, params types.CreatePersonalTagParams) (*types.PersonalTag, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "AddUserAvoidTag", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "AddUserAvoidTag"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Adding user avoid tag")

	tag, err := s.repo.Create(ctx, userID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to add user avoid tag", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to add user avoid tag")
		return nil, fmt.Errorf("error adding user avoid tag: %w", err)
	}

	l.InfoContext(ctx, "User avoid tag added successfully")
	span.SetStatus(codes.Ok, "User avoid tag added successfully")
	return tag, nil
}

// DeleteTag removes an avoid tag for a user.
func (s *UserTagsServiceImpl) DeleteTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "RemoveUserAvoidTag", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("tag.id", tagID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "RemoveUserAvoidTag"), slog.String("userID", userID.String()), slog.String("tagID", tagID.String()))
	l.DebugContext(ctx, "Removing user avoid tag")

	err := s.repo.Delete(ctx, userID, tagID)
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

func (s *UserTagsServiceImpl) Update(ctx context.Context, userID uuid.UUID, tagID uuid.UUID, params types.UpdatePersonalTagParams) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "UpdateUserAvoidTag", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("tag.id", tagID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "UpdateUserAvoidTag"), slog.String("userID", userID.String()), slog.String("tagID", tagID.String()))
	l.DebugContext(ctx, "Updating user avoid tag")

	err := s.repo.Update(ctx, userID, tagID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user avoid tag", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update user avoid tag")
		return fmt.Errorf("error updating user avoid tag: %w", err)
	}

	l.InfoContext(ctx, "User avoid tag updated successfully")
	span.SetStatus(codes.Ok, "User avoid tag updated successfully")
	return nil
}
