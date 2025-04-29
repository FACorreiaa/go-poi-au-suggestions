package userSettings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/codes"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
)

var _ SettingsRepository = (*PostgresSettingsRepo)(nil)

type SettingsRepository interface {
	// GetSettings retrieves the settings profile for a specific user.
	// Returns ErrNotFound if no settings exist for the user (shouldn't happen if trigger works).
	GetSettings(ctx context.Context, userID uuid.UUID) (*UserSettings, error)

	// UpdateSettings updates specific fields in the user's settings profile.
	// Uses pointers in params struct for partial updates. Ensures updated_at is set.
	// Returns ErrNotFound if the user doesn't have a settings row (shouldn't happen).
	UpdateSettings(ctx context.Context, userID uuid.UUID, params UpdateUserSettingsParams) error
}

type PostgresSettingsRepo struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewPostgresUserSettingsRepo(pgxpool *pgxpool.Pool, logger *slog.Logger) *PostgresSettingsRepo {
	return &PostgresSettingsRepo{
		logger: logger,
		pgpool: pgxpool,
	}
}

func (r *PostgresSettingsRepo) GetSettings(ctx context.Context, userID uuid.UUID) (*UserSettings, error) {
	var settings UserSettings
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "GetUserSettings", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.sql.table", "user_settings"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "GetUserSettings"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user settings")

	query := `
		SELECT default_search_radius_km, preferred_time, default_budget_level, 
		       preferred_pace, prefer_accessible_pois,
			   prefer_outdoor_seating, preferred_transport_mode, prefer_dog_friendly, 
			   created_at, updated_at FROM user_settings
		WHERE user_id = $1
	`

	err := r.pgpool.QueryRow(ctx, query, userID).Scan(
		&settings.DefaultSearchRadiusKm,
		&settings.PreferredTime,
		&settings.DefaultBudgetLevel,
		&settings.PreferredPace,
		&settings.PreferAccessiblePOIs,
		&settings.PreferOutdoorSeating,
		&settings.PreferTransportMode,
		&settings.PreferDogFriendly,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)

	if err != nil {
		// Use domain.ErrNotFound for pgx.ErrNoRows
		if errors.Is(err, pgx.ErrNoRows) {
			l.WarnContext(ctx, "User settings not found")
			span.SetStatus(codes.Error, "Settings not found")
			return nil, fmt.Errorf("user settings not found: %w", api.ErrNotFound)
		}
		l.ErrorContext(ctx, "Failed to query user settings", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB SELECT failed")
		return nil, fmt.Errorf("database error fetching settings: %w", err)
	}

	l.DebugContext(ctx, "User settings fetched successfully")
	span.SetStatus(codes.Ok, "Settings fetched")

	return &settings, nil
}

func (r *PostgresSettingsRepo) UpdateSettings(ctx context.Context, userID uuid.UUID, params UpdateUserSettingsParams) error {
	ctx, span := otel.Tracer("SettingsRepo").Start(ctx, "UpdateUserSettings", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "user_settings"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "UpdateUserSettings"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Updating user settings", slog.Any("params", params))

	// Build query dynamically
	var setClauses []string
	var args []interface{}
	argID := 1 // Start placeholders at $1

	// Check each field in params
	if params.DefaultSearchRadiusKm != nil {
		setClauses = append(setClauses, fmt.Sprintf("default_search_radius_km = $%d", argID))
		args = append(args, *params.DefaultSearchRadiusKm)
		argID++
		span.SetAttributes(attribute.Bool("update.radius", true))
	}
	if params.PreferredTime != nil {
		setClauses = append(setClauses, fmt.Sprintf("preferred_time = $%d", argID))
		args = append(args, *params.PreferredTime) // Pass ENUM value directly
		argID++
		span.SetAttributes(attribute.Bool("update.time", true))
	}
	if params.DefaultBudgetLevel != nil {
		setClauses = append(setClauses, fmt.Sprintf("default_budget_level = $%d", argID))
		args = append(args, *params.DefaultBudgetLevel)
		argID++
		span.SetAttributes(attribute.Bool("update.budget", true))
	}
	if params.PreferredPace != nil {
		setClauses = append(setClauses, fmt.Sprintf("preferred_pace = $%d", argID))
		args = append(args, *params.PreferredPace) // Pass ENUM value directly
		argID++
		span.SetAttributes(attribute.Bool("update.pace", true))
	}
	if params.PreferAccessiblePOIs != nil {
		setClauses = append(setClauses, fmt.Sprintf("prefer_accessible_pois = $%d", argID))
		args = append(args, *params.PreferAccessiblePOIs)
		argID++
		span.SetAttributes(attribute.Bool("update.accessible", true))
	}
	if params.PreferOutdoorSeating != nil {
		setClauses = append(setClauses, fmt.Sprintf("prefer_outdoor_seating = $%d", argID))
		args = append(args, *params.PreferOutdoorSeating)
		argID++
		span.SetAttributes(attribute.Bool("update.outdoor_seating", true))
	}
	if params.PreferDogFriendly != nil {
		setClauses = append(setClauses, fmt.Sprintf("prefer_dog_friendly = $%d", argID))
		args = append(args, *params.PreferDogFriendly)
		argID++
		span.SetAttributes(attribute.Bool("update.dog_friendly", true))
	}

	// If no fields were provided to update, return successfully
	if len(setClauses) == 0 {
		l.InfoContext(ctx, "UpdateUserSettings called with no fields to update")
		span.SetStatus(codes.Ok, "No update fields provided")
		return nil
	}

	// Always add updated_at if other fields changed
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argID))
	args = append(args, time.Now())
	argID++

	// Add the userID for the WHERE clause as the last argument
	args = append(args, userID)

	// Construct the final query
	query := fmt.Sprintf(`UPDATE user_settings SET %s WHERE user_id = $%d`,
		strings.Join(setClauses, ", "),
		argID, // Placeholder for user_id in WHERE clause
	)

	l.DebugContext(ctx, "Executing dynamic user settings update query", slog.String("query", query), slog.Int("arg_count", len(args)))

	// Execute the dynamic query
	tag, err := r.pgpool.Exec(ctx, query, args...)
	if err != nil {
		// Add specific error checking if needed (e.g., CHECK constraint violations)
		l.ErrorContext(ctx, "Failed to execute update user settings query", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error updating user settings: %w", err)
	}

	// Check if the settings row existed and was updated
	if tag.RowsAffected() == 0 {
		l.WarnContext(ctx, "User settings not found for update (or no change needed)", slog.Int64("rows_affected", tag.RowsAffected()))
		span.SetStatus(codes.Error, "User settings not found or no change")
		return fmt.Errorf("user settings not found for user %s: %w", userID.String(), api.ErrNotFound)
	}

	l.InfoContext(ctx, "User settings updated successfully")
	span.SetStatus(codes.Ok, "User settings updated")
	return nil
}
