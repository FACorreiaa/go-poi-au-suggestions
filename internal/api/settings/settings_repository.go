package settings

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

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

var _ SettingsRepository = (*PostgresSettingsRepo)(nil)

type SettingsRepository interface {
	// Get retrieves the settings profile for a specific user.
	// Returns ErrNotFound if no settings exist for the user (shouldn't happen if trigger works).
	Get(ctx context.Context, userID uuid.UUID) (*types.Settings, error)

	// Update updates specific fields in the user's settings profile.
	// Uses pointers in params struct for partial updates. Ensures updated_at is set.
	// Returns ErrNotFound if the user doesn't have a settings row (shouldn't happen).
	Update(ctx context.Context, userID, profileID uuid.UUID, params types.UpdatesettingsParams) error
}

type PostgresSettingsRepo struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewPostgressettingsRepo(pgxpool *pgxpool.Pool, logger *slog.Logger) *PostgresSettingsRepo {
	return &PostgresSettingsRepo{
		logger: logger,
		pgpool: pgxpool,
	}
}

func (r *PostgresSettingsRepo) Get(ctx context.Context, userID uuid.UUID) (*types.Settings, error) {
	var settings types.Settings
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "Getsettings", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.sql.table", "user_preference_profiles"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "Getsettings"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user settings")

	query := `
		SELECT search_radius_km, preferred_time, budget_level, 
		       preferred_pace, prefer_accessible_pois,
			   prefer_outdoor_seating, preferred_transport, prefer_dog_friendly, 
			   created_at, updated_at FROM user_preference_profiles
		WHERE user_id = $1
	`

	err := r.pgpool.QueryRow(ctx, query, userID).Scan(
		&settings.SearchRadius,
		&settings.PreferredTime,
		&settings.BudgetLevel,
		&settings.PreferredPace,
		&settings.PreferAccessiblePOIs,
		&settings.PreferOutdoorSeating,
		&settings.PreferredTransport,
		&settings.PreferDogFriendly,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)

	if err != nil {
		// Use domain.ErrNotFound for pgx.ErrNoRows
		if errors.Is(err, pgx.ErrNoRows) {
			l.WarnContext(ctx, "User settings not found")
			span.SetStatus(codes.Error, "Settings not found")
			return nil, fmt.Errorf("user settings not found: %w", types.ErrNotFound)
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

func (r *PostgresSettingsRepo) Update(ctx context.Context, userID, profileID uuid.UUID, params types.UpdatesettingsParams) error {
	ctx, span := otel.Tracer("SettingsRepo").Start(ctx, "Updatesettings", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "user_preference_profiles"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "Updatesettings"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Updating user settings", slog.Any("params", params))

	// Build query dynamically
	var setClauses []string
	var args []interface{}
	argID := 1 // Start placeholders at $1

	// Check each field in params
	if params.SearchRadius != nil {
		setClauses = append(setClauses, fmt.Sprintf("default_search_radius_km = $%d", argID))
		args = append(args, *params.SearchRadius)
		argID++
		span.SetAttributes(attribute.Bool("update.radius", true))
	}
	if params.PreferredTime != nil {
		setClauses = append(setClauses, fmt.Sprintf("preferred_time = $%d", argID))
		args = append(args, *params.PreferredTime)
		argID++
		span.SetAttributes(attribute.Bool("update.time", true))
	}
	if params.BudgetLevel != nil {
		setClauses = append(setClauses, fmt.Sprintf("default_budget_level = $%d", argID))
		args = append(args, *params.BudgetLevel)
		argID++
		span.SetAttributes(attribute.Bool("update.budget", true))
	}
	if params.PreferredPace != nil {
		setClauses = append(setClauses, fmt.Sprintf("preferred_pace = $%d", argID))
		args = append(args, *params.PreferredPace)
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
	if params.IsDefault != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_default = $%d", argID))
		args = append(args, *params.IsDefault)
		argID++
		span.SetAttributes(attribute.Bool("update.is_default", true))
	}

	// If no fields were provided to update, return successfully
	if len(setClauses) == 0 {
		l.InfoContext(ctx, "Updatesettings called with no fields to update")
		span.SetStatus(codes.Ok, "No update fields provided")
		return nil
	}

	// Always add updated_at if other fields changed
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argID))
	args = append(args, time.Now())
	argID++

	// Define placeholders for WHERE clause
	whereIDPlaceholder := argID
	whereUserIDPlaceholder := argID + 1
	args = append(args, profileID, userID)

	// Construct the query
	query := fmt.Sprintf(`UPDATE user_preference_profiles SET %s WHERE id = $%d AND user_id = $%d`,
		strings.Join(setClauses, ", "),
		whereIDPlaceholder,
		whereUserIDPlaceholder,
	)

	l.DebugContext(ctx, "Executing dynamic user settings update query", slog.String("query", query), slog.Int("arg_count", len(args)))

	// Execute the dynamic query
	tag, err := r.pgpool.Exec(ctx, query, args...)
	if err != nil {
		l.ErrorContext(ctx, "Failed to execute update user settings query", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error updating user settings: %w", err)
	}

	// Check if the settings row existed and was updated
	if tag.RowsAffected() == 0 {
		l.WarnContext(ctx, "User settings not found for update (or no change needed)", slog.Int64("rows_affected", tag.RowsAffected()))
		span.SetStatus(codes.Error, "User settings not found or no change")
		return fmt.Errorf("user settings not found for user %s: %w", userID.String(), types.ErrNotFound)
	}

	l.InfoContext(ctx, "User settings updated successfully")
	span.SetStatus(codes.Ok, "User settings updated")
	return nil
}
