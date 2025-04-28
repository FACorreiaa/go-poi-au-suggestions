package user

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
)

var _ UserTagsRepo = (*PostgresUserTagsRepo)(nil)

// UserRepo defines the contract for user data persistence.
type UserTagsRepo interface {
	// GetAllGlobalTags --- Global Tags & User Avoid Tags ---
	// GetAllGlobalTags retrieves all global tags
	GetAllGlobalTags(ctx context.Context) ([]api.GlobalTag, error)

	// GetUserAvoidTags retrieves all avoid tags for a user
	GetUserAvoidTags(ctx context.Context, userID uuid.UUID) ([]api.UserAvoidTag, error)

	// AddUserAvoidTag adds an avoid tag for a user
	AddUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error

	// RemoveUserAvoidTag removes an avoid tag for a user
	RemoveUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error
}

type PostgresUserTagsRepo struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewPostgresUserTagsRepo(pgxpool *pgxpool.Pool, logger *slog.Logger) *PostgresUserTagsRepo {
	return &PostgresUserTagsRepo{
		logger: logger,
		pgpool: pgxpool,
	}
}

// GetAllGlobalTags implements user.UserRepo.
func (r *PostgresUserTagsRepo) GetAllGlobalTags(ctx context.Context) ([]api.GlobalTag, error) {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "GetAllGlobalTags", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.sql.table", "global_tags"),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "GetAllGlobalTags"))
	l.DebugContext(ctx, "Fetching all active global tags")

	query := `
        SELECT id, name, description, tag_type, active, created_at
        FROM global_tags
        WHERE active = TRUE
        ORDER BY name`

	rows, err := r.pgpool.Query(ctx, query)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query global tags", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return nil, fmt.Errorf("database error fetching global tags: %w", err)
	}
	defer rows.Close()

	var tags []api.GlobalTag
	for rows.Next() {
		var t api.GlobalTag
		err := rows.Scan(
			&t.ID, &t.Name, &t.Description, &t.TagType, &t.Active, &t.CreatedAt,
		)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan global tag row", slog.Any("error", err))
			span.RecordError(err)
			return nil, fmt.Errorf("database error scanning global tag: %w", err)
		}
		tags = append(tags, t)
	}

	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating global tag rows", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("database error reading global tags: %w", err)
	}

	l.DebugContext(ctx, "Fetched all active global tags successfully", slog.Int("count", len(tags)))
	span.SetStatus(codes.Ok, "Global tags fetched")
	return tags, nil
}

// GetUserAvoidTags implements user.UserRepo.
func (r *PostgresUserTagsRepo) GetUserAvoidTags(ctx context.Context, userID uuid.UUID) ([]api.UserAvoidTag, error) {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "GetUserAvoidTags", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.sql.table", "user_avoid_tags, global_tags"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "GetUserAvoidTags"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user avoid tags")

	query := `
        SELECT uat.user_id, uat.tag_id, gt.name, gt.tag_type, uat.created_at
        FROM user_avoid_tags uat
        JOIN global_tags gt ON uat.tag_id = gt.id
        WHERE uat.user_id = $1 AND gt.active = TRUE
        ORDER BY gt.name`

	rows, err := r.pgpool.Query(ctx, query, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query user avoid tags", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return nil, fmt.Errorf("database error fetching avoid tags: %w", err)
	}
	defer rows.Close()

	var tags []api.UserAvoidTag
	for rows.Next() {
		var t api.UserAvoidTag
		err := rows.Scan(
			&t.UserID, &t.TagID, &t.TagName, &t.TagType, &t.CreatedAt,
		)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan avoid tag row", slog.Any("error", err))
			span.RecordError(err)
			return nil, fmt.Errorf("database error scanning avoid tag: %w", err)
		}
		tags = append(tags, t)
	}

	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating avoid tag rows", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("database error reading avoid tags: %w", err)
	}

	l.DebugContext(ctx, "Fetched user avoid tags successfully", slog.Int("count", len(tags)))
	span.SetStatus(codes.Ok, "Avoid tags fetched")
	return tags, nil
}

// AddUserAvoidTag implements user.UserRepo.
func (r *PostgresUserTagsRepo) AddUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "AddUserAvoidTag", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "user_avoid_tags"),
		attribute.String("db.user.id", userID.String()),
		attribute.String("db.tag.id", tagID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "AddUserAvoidTag"), slog.String("userID", userID.String()), slog.String("tagID", tagID.String()))
	l.DebugContext(ctx, "Adding user avoid tag")

	query := `
        INSERT INTO user_avoid_tags (user_id, tag_id)
        VALUES ($1, $2)
        ON CONFLICT (user_id, tag_id) DO NOTHING`

	tag, err := r.pgpool.Exec(ctx, query, userID, tagID)
	if err != nil {
		// Check for foreign key violation if tagID doesn't exist in 'global_tags' table
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // Foreign key violation
			l.WarnContext(ctx, "Attempted to add non-existent tag to user", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Foreign key violation")
			return fmt.Errorf("tag ID %s does not exist: %w", tagID.String(), api.ErrNotFound)
		}
		l.ErrorContext(ctx, "Failed to insert user avoid tag", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB INSERT failed")
		return fmt.Errorf("database error adding avoid tag: %w", err)
	}

	if tag.RowsAffected() == 0 {
		l.DebugContext(ctx, "User avoid tag association already exists")
		// Not an error in this case due to ON CONFLICT DO NOTHING
	} else {
		l.InfoContext(ctx, "User avoid tag added successfully")
	}
	span.SetStatus(codes.Ok, "Avoid tag added or already exists")
	return nil
}

// RemoveUserAvoidTag implements user.UserRepo.
func (r *PostgresUserTagsRepo) RemoveUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "RemoveUserAvoidTag", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.sql.table", "user_avoid_tags"),
		attribute.String("db.user.id", userID.String()),
		attribute.String("db.tag.id", tagID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "RemoveUserAvoidTag"), slog.String("userID", userID.String()), slog.String("tagID", tagID.String()))
	l.DebugContext(ctx, "Removing user avoid tag")

	query := "DELETE FROM user_avoid_tags WHERE user_id = $1 AND tag_id = $2"
	tag, err := r.pgpool.Exec(ctx, query, userID, tagID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to delete user avoid tag", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB DELETE failed")
		return fmt.Errorf("database error removing avoid tag: %w", err)
	}

	if tag.RowsAffected() == 0 {
		l.WarnContext(ctx, "Attempted to remove non-existent user avoid tag association")
		// Return an error so the service/handler knows the operation didn't change anything
		span.SetStatus(codes.Error, "Association not found")
		return fmt.Errorf("avoid tag association not found: %w", api.ErrNotFound)
	}

	l.InfoContext(ctx, "User avoid tag removed successfully")
	span.SetStatus(codes.Ok, "Avoid tag removed")
	return nil
}
