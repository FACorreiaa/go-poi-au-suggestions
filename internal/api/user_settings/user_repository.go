package userSettings

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
)

var _ UserSettingsRepository = (*PostgresUserSettingsRepo)(nil)

type UserSettingsRepository interface {
	// GetUserPreferences --- Preferences  ---
	GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]*api.Interest, error) // Fetches from user_interests join
}

type PostgresUserSettingsRepo struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewPostgresUserSettingsRepo(pgxpool *pgxpool.Pool, logger *slog.Logger) *PostgresUserSettingsRepo {
	return &PostgresUserSettingsRepo{
		logger: logger,
		pgpool: pgxpool,
	}
}

// GetUserPreferences implements user.UserRepo.
func (r *PostgresUserSettingsRepo) GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]*api.Interest, error) {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "GetUserPreferences", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.sql.table", "user_interests, interests"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "GetUserPreferences"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user preferences")

	query := `
        SELECT i.id, i.name, i.description, i.active, i.created_at, i.updated_at
        FROM interests i
        JOIN user_interests ui ON i.id = ui.interest_id
        WHERE ui.user_id = $1 AND i.active = TRUE -- Only return active global interests
        ORDER BY i.name`

	rows, err := r.pgpool.Query(ctx, query, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query user preferences", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return nil, fmt.Errorf("database error fetching preferences: %w", err)
	}
	defer rows.Close()

	var interests []*api.Interest
	for rows.Next() {
		var i api.Interest
		// Ensure Scan matches SELECT order and Interest struct fields
		err := rows.Scan(
			&i.ID, &i.Name, &i.Description, &i.Active, &i.CreatedAt, &i.UpdatedAt,
		)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan interest row", slog.Any("error", err))
			span.RecordError(err)
			// Decide whether to return partial results or fail
			return nil, fmt.Errorf("database error scanning preference: %w", err)
		}
		interests = append(interests, &i)
	}

	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating preference rows", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("database error reading preferences: %w", err)
	}

	l.DebugContext(ctx, "Fetched user preferences successfully", slog.Int("count", len(interests)))
	span.SetStatus(codes.Ok, "Preferences fetched")
	return interests, nil
}
