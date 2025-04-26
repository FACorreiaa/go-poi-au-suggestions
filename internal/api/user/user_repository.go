package user

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
)

var _ UserRepo = (*PostgresUserRepo)(nil)

// UserRepo defines the contract for user data persistence.
type UserRepo interface {
	// GetUserByID retrieves a user's full profile by their unique ID.
	// Returns api.ErrNotFound if the user doesn't exist or is inactive.
	GetUserByID(ctx context.Context, userID uuid.UUID) (*api.UserProfile, error)

	ChangePassword(ctx context.Context, email, oldPassword, newPassword string) error
	// UpdateProfile updates mutable fields on a user's profile.
	// It takes the userID and a struct containing only the fields to be updated (use pointers).
	// Returns api.ErrNotFound if the user doesn't exist.
	UpdateProfile(ctx context.Context, userID uuid.UUID, params api.UpdateProfileParams) error

	// GetUserPreferences --- Preferences / Interests ---

	GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]api.Interest, error) // Fetches from user_interests join
	CreateInterest(ctx context.Context, name string, description *string, isActive bool) (*api.Interest, error)
	AddUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error
	RemoveUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error
	GetAllInterests(ctx context.Context) ([]api.Interest, error) // Fetches from global interests table
	// UpdateUserInterest(ctx context.Context, interestID uuid.UUID, name string, description *string, isActive bool) error
	// UpdateUserInterestPreferenceLevel updates the preference level for a user interest
	UpdateUserInterestPreferenceLevel(ctx context.Context, userID uuid.UUID, interestID uuid.UUID, preferenceLevel int) error

	// GetUserEnhancedInterests retrieves all interests for a user with their preference levels
	GetUserEnhancedInterests(ctx context.Context, userID uuid.UUID) ([]api.EnhancedInterest, error)

	// GetUserPreferenceProfiles --- User Preference Profiles ---
	// GetUserPreferenceProfiles retrieves all preference profiles for a user
	GetUserPreferenceProfiles(ctx context.Context, userID uuid.UUID) ([]api.UserPreferenceProfile, error)

	// GetUserPreferenceProfile retrieves a specific preference profile by ID
	GetUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) (*api.UserPreferenceProfile, error)

	// GetDefaultUserPreferenceProfile retrieves the default preference profile for a user
	GetDefaultUserPreferenceProfile(ctx context.Context, userID uuid.UUID) (*api.UserPreferenceProfile, error)

	// CreateUserPreferenceProfile creates a new preference profile for a user
	CreateUserPreferenceProfile(ctx context.Context, userID uuid.UUID, params api.CreateUserPreferenceProfileParams) (*api.UserPreferenceProfile, error)

	// UpdateUserPreferenceProfile updates a preference profile
	UpdateUserPreferenceProfile(ctx context.Context, profileID uuid.UUID, params api.UpdateUserPreferenceProfileParams) error

	// DeleteUserPreferenceProfile deletes a preference profile
	DeleteUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error

	// SetDefaultUserPreferenceProfile sets a profile as the default for a user
	SetDefaultUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error

	// GetAllGlobalTags --- Global Tags & User Avoid Tags ---
	// GetAllGlobalTags retrieves all global tags
	GetAllGlobalTags(ctx context.Context) ([]api.GlobalTag, error)

	// GetUserAvoidTags retrieves all avoid tags for a user
	GetUserAvoidTags(ctx context.Context, userID uuid.UUID) ([]api.UserAvoidTag, error)

	// AddUserAvoidTag adds an avoid tag for a user
	AddUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error

	// RemoveUserAvoidTag removes an avoid tag for a user
	RemoveUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error

	// --- Status & Activity ---
	// UpdateLastLogin sets the last_login_at timestamp for a user.
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error

	// MarkEmailAsVerified sets the email_verified_at timestamp.
	MarkEmailAsVerified(ctx context.Context, userID uuid.UUID) error

	// DeactivateUser marks a user as inactive (soft delete).
	// This also invalidates all active sessions/tokens.
	DeactivateUser(ctx context.Context, userID uuid.UUID) error

	// ReactivateUser marks a user as active.
	ReactivateUser(ctx context.Context, userID uuid.UUID) error
}

type PostgresUserRepo struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewPostgresUserRepo(pgxpool *pgxpool.Pool, logger *slog.Logger) *PostgresUserRepo {
	return &PostgresUserRepo{
		logger: logger,
		pgpool: pgxpool,
	}
}

func (r *PostgresUserRepo) ChangePassword(ctx context.Context, email, oldPassword, newPassword string) error {
	var userID, hashedPassword string
	err := r.pgpool.QueryRow(ctx,
		"SELECT id, password_hash FROM users WHERE email = $1",
		email).Scan(&userID, &hashedPassword)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(oldPassword))
	if err != nil {
		return errors.New("invalid old password")
	}

	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	_, err = r.pgpool.Exec(ctx,
		"UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3",
		string(newHashedPassword), time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Invalidate all refresh tokens
	_, err = r.pgpool.Exec(ctx,
		"UPDATE refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL",
		time.Now(), userID)
	if err != nil {
		fmt.Printf("Warning: failed to invalidate refresh tokens: %v\n", err)
	}

	return nil
}

func (r *PostgresUserRepo) GetUserByID(ctx context.Context, userID uuid.UUID) (*api.UserProfile, error) {
	var user api.UserProfile
	query := `
		SELECT username, firstname, lastname, age, city, 
		       country, email, display_name, profile_image_url, 
		       email_verified_at, about_you FROM users WHERE id = $1
	`
	err := r.pgpool.QueryRow(ctx,
		query,
		userID).Scan(&user.Username,
		&user.Firstname,
		&user.Lastname,
		&user.Age,
		&user.City,
		&user.Country,
		&user.Email,
		&user.DisplayName,
		&user.ProfileImageURL,
		&user.EmailVerifiedAt,
		&user.AboutYou)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &user, nil
}

func (r *PostgresUserRepo) UpdateProfile(ctx context.Context, userID uuid.UUID, params api.UpdateProfileParams) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "UpdateProfile", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "users"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "UpdateProfile"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Updating user profile", slog.Any("params", params)) // Log incoming params

	// Use squirrel or build query dynamically
	var setClauses []string
	var args []interface{}
	argID := 1 // Argument counter for placeholders ($1, $2, ...)

	// Check each field in params. If not nil, add to SET clause and args slice.
	if params.Username != nil {
		setClauses = append(setClauses, fmt.Sprintf("username = $%d", argID))
		args = append(args, *params.Username)
		argID++
		span.SetAttributes(attribute.Bool("update.username", true)) // Add trace attribute
	}
	if params.Email != nil {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", argID))
		args = append(args, *params.Email)
		argID++
		span.SetAttributes(attribute.Bool("update.email", true))
	}
	if params.DisplayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", argID))
		args = append(args, *params.DisplayName)
		argID++
		span.SetAttributes(attribute.Bool("update.display_name", true))
	}
	if params.ProfileImageURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("profile_image_url = $%d", argID))
		args = append(args, *params.ProfileImageURL)
		argID++
		span.SetAttributes(attribute.Bool("update.profile_image_url", true))
	}
	if params.Firstname != nil {
		setClauses = append(setClauses, fmt.Sprintf("firstname = $%d", argID))
		args = append(args, *params.Firstname)
		argID++
		span.SetAttributes(attribute.Bool("update.firstname", true))
	}
	if params.Lastname != nil {
		setClauses = append(setClauses, fmt.Sprintf("lastname = $%d", argID))
		args = append(args, *params.Lastname)
		argID++
		span.SetAttributes(attribute.Bool("update.lastname", true))
	}
	if params.Age != nil {
		setClauses = append(setClauses, fmt.Sprintf("age = $%d", argID))
		args = append(args, *params.Age)
		argID++
		span.SetAttributes(attribute.Bool("update.age", true))
	}
	if params.City != nil {
		setClauses = append(setClauses, fmt.Sprintf("city = $%d", argID))
		args = append(args, *params.City)
		argID++
		span.SetAttributes(attribute.Bool("update.city", true))
	}
	if params.Country != nil {
		setClauses = append(setClauses, fmt.Sprintf("country = $%d", argID))
		args = append(args, *params.Country)
		argID++
		span.SetAttributes(attribute.Bool("update.country", true))
	}
	if params.AboutYou != nil {
		setClauses = append(setClauses, fmt.Sprintf("about_you = $%d", argID))
		args = append(args, *params.AboutYou)
		argID++
		span.SetAttributes(attribute.Bool("update.about_you", true))
	}

	// If no fields were provided to update, return early (or error?)
	if len(setClauses) == 0 {
		l.WarnContext(ctx, "UpdateProfile called with no fields to update")
		span.SetStatus(codes.Ok, "No update fields provided") // Not an error, just no-op
		return nil                                            // Or return specific error api.ErrBadRequest("no update fields provided")
	}

	// Add updated_at clause (always update this if other fields change)
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argID))
	args = append(args, time.Now())
	argID++

	// Final WHERE clause argument
	args = append(args, userID)

	// Construct the final query
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d AND is_active = TRUE",
		strings.Join(setClauses, ", "), // e.g., "username = $1, age = $2, updated_at = $3"
		argID,                          // The placeholder for userID
	)

	l.DebugContext(ctx, "Executing dynamic update query", slog.String("query", query), slog.Int("arg_count", len(args)))

	// Execute the dynamic query
	tag, err := r.pgpool.Exec(ctx, query, args...)
	if err != nil {
		// Add specific error checking (e.g., unique constraint violations on email/username if updated)
		l.ErrorContext(ctx, "Failed to execute update profile query", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error updating profile: %w", err)
	}

	// Check if the user existed and was updated
	if tag.RowsAffected() == 0 {
		l.WarnContext(ctx, "User not found or no update occurred", slog.Int64("rows_affected", tag.RowsAffected()))
		span.SetStatus(codes.Error, "User not found or no change")
		// Check if user exists to differentiate "not found" vs "no effective change"
		var exists bool
		// Use a separate query or modify the UPDATE to return something on match
		checkErr := r.pgpool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND is_active = TRUE)", userID).Scan(&exists)
		if checkErr == nil && !exists {
			return fmt.Errorf("user not found for update: %w", api.ErrNotFound)
		}
		// If user exists, maybe the provided values were the same as existing ones.
		// Or maybe user was inactive. Treat as not found for simplicity for now.
		return fmt.Errorf("user not found or update failed: %w", api.ErrNotFound)
	}

	l.InfoContext(ctx, "User profile updated successfully")
	span.SetStatus(codes.Ok, "Profile updated")
	return nil
}

// GetUserPreferences implements user.UserRepo.
func (r *PostgresUserRepo) GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]api.Interest, error) {
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

	var interests []api.Interest
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
		interests = append(interests, i)
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

// CreateInterest implements user.CreateInterest
func (r *PostgresUserRepo) CreateInterest(ctx context.Context, name string, description *string, isActive bool) (*api.Interest, error) {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "CreateInterest", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "interests"),
		attribute.String("interest.name", name), // Add relevant attributes
		attribute.Bool("interest.active", isActive),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "CreateInterest"), slog.String("name", name))
	l.DebugContext(ctx, "Creating new global interest")

	// Input validation basic check
	if name == "" {
		span.SetStatus(codes.Error, "Interest name cannot be empty")
		return nil, fmt.Errorf("interest name cannot be empty: %w", api.ErrBadRequest) // Example domain error
	}

	var interest api.Interest
	query := `
        INSERT INTO interests (name, description, active, created_at, updated_at)
        VALUES ($1, $2, $3, Now(), Now())
        RETURNING id, name, description, active, created_at, updated_at`

	// Note: Use current time for both created_at (via DEFAULT) and updated_at on insert
	err := r.pgpool.QueryRow(ctx, query, name, description, isActive).Scan(
		&interest.ID,
		&interest.Name,
		&interest.Description,
		&interest.Active,
		&interest.CreatedAt,
		&interest.UpdatedAt, // Scan the updated_at timestamp set by the query
	)

	// TODO also add to user_interests

	if err != nil {
		// Check for unique constraint violation (name already exists)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // Unique violation
			l.WarnContext(ctx, "Attempted to create interest with duplicate name", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Duplicate interest name")
			return nil, fmt.Errorf("interest with name '%s' already exists: %w", name, api.ErrConflict)
		}
		// Handle other potential errors
		l.ErrorContext(ctx, "Failed to insert new interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB INSERT failed")
		return nil, fmt.Errorf("database error creating interest: %w", err)
	}

	l.InfoContext(ctx, "Global interest created successfully", slog.String("interestID", interest.ID.String()))
	span.SetAttributes(attribute.String("db.interest.id", interest.ID.String()))
	span.SetStatus(codes.Ok, "Interest created")
	return &interest, nil
}

// AddUserInterest implements user.UserRepo.
func (r *PostgresUserRepo) AddUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "AddUserInterest", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "user_interests"),
		attribute.String("db.user.id", userID.String()),
		attribute.String("db.interest.id", interestID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "AddUserInterest"), slog.String("userID", userID.String()), slog.String("interestID", interestID.String()))
	l.DebugContext(ctx, "Adding user interest")

	query := `
        INSERT INTO user_interests (user_id, interest_id)
        VALUES ($1, $2) ON CONFLICT (user_id, interest_id) DO NOTHING`

	// TODO should I invalidate with an error message when a repeated interest_id is inserted or silently do nothing
	tag, err := r.pgpool.Exec(ctx, query, userID, interestID)
	if err != nil {
		// Check for foreign key violation if interestID doesn't exist in 'interests' table
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // Foreign key violation
			l.WarnContext(ctx, "Attempted to add non-existent interest to user", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Foreign key violation")
			return fmt.Errorf("interest ID %s does not exist: %w", interestID.String(), api.ErrNotFound)
		}
		l.ErrorContext(ctx, "Failed to insert user interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB INSERT failed")
		return fmt.Errorf("database error adding interest: %w", err)
	}

	if tag.RowsAffected() == 0 {
		l.DebugContext(ctx, "User interest association already exists")
		// Not an error in this case due to ON CONFLICT DO NOTHING
	} else {
		l.InfoContext(ctx, "User interest added successfully")
	}
	span.SetStatus(codes.Ok, "Interest added or already exists")
	return nil
}

// RemoveUserInterest implements user.UserRepo.
func (r *PostgresUserRepo) RemoveUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "RemoveUserInterest", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.sql.table", "user_interests"),
		attribute.String("db.user.id", userID.String()),
		attribute.String("db.interest.id", interestID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "RemoveUserInterest"), slog.String("userID", userID.String()), slog.String("interestID", interestID.String()))
	l.DebugContext(ctx, "Removing user interest")

	query := "DELETE FROM user_interests WHERE user_id = $1 AND interest_id = $2"
	tag, err := r.pgpool.Exec(ctx, query, userID, interestID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to delete user interest", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB DELETE failed")
		return fmt.Errorf("database error removing interest: %w", err)
	}

	if tag.RowsAffected() == 0 {
		l.WarnContext(ctx, "Attempted to remove non-existent user interest association")
		// Return an error so the service/handler knows the operation didn't change anything
		span.SetStatus(codes.Error, "Association not found")
		return fmt.Errorf("interest association not found: %w", api.ErrNotFound)
	}

	l.InfoContext(ctx, "User interest removed successfully")
	span.SetStatus(codes.Ok, "Interest removed")
	return nil
}

// GetAllInterests TODO does it make sense to only return the active interests ? Just mark active on the UI ?
// GetAllInterests implements user.UserRepo.
func (r *PostgresUserRepo) GetAllInterests(ctx context.Context) ([]api.Interest, error) {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "GetAllInterests", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.sql.table", "interests"),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "GetAllInterests"))
	l.DebugContext(ctx, "Fetching all active interests")

	query := `
        SELECT id, name, description, active, created_at, updated_at
        FROM interests
        WHERE active = TRUE
        ORDER BY name`

	rows, err := r.pgpool.Query(ctx, query)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query all interests", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return nil, fmt.Errorf("database error fetching interests: %w", err)
	}
	defer rows.Close()

	var interests []api.Interest
	for rows.Next() {
		var i api.Interest
		err := rows.Scan(
			&i.ID, &i.Name, &i.Description, &i.Active, &i.CreatedAt, &i.UpdatedAt,
		)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan interest row", slog.Any("error", err))
			span.RecordError(err)
			return nil, fmt.Errorf("database error scanning interest: %w", err)
		}
		interests = append(interests, i)
	}

	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating interests rows", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("database error reading interests: %w", err)
	}

	l.DebugContext(ctx, "Fetched all active interests successfully", slog.Int("count", len(interests)))
	span.SetStatus(codes.Ok, "Interests fetched")
	return interests, nil
}

//func (r *PostgresUserRepo) UpdateUserInterest(ctx context.Context, interestID uuid.UUID, name string, description *string, isActive bool) error {
//	// Add tracing/logging as needed
//	l := r.logger.With(slog.String("method", "UpdateInterest"), slog.String("interestID", interestID.String()))
//	l.DebugContext(ctx, "Updating global interest details")
//
//	if name == "" {
//		return errors.New("interest name cannot be empty")
//	}
//
//	query := `
//        UPDATE interests
//        SET name = $1,
//            description = $2,
//            active = $3,
//            updated_at = NOW()
//        WHERE id = $4`
//
//	tag, err := r.pgpool.Exec(ctx, query, name, description, isActive, interestID)
//	if err != nil {
//		// Check for unique constraint violation on name
//		var pgErr *pgconn.PgError
//		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
//			l.WarnContext(ctx, "Attempted to update interest with duplicate name", slog.Any("error", err))
//			return fmt.Errorf("interest with name '%s' already exists: %w", name, api.ErrConflict)
//		}
//		l.ErrorContext(ctx, "Failed to update interest", slog.Any("error", err))
//		return fmt.Errorf("database error updating interest: %w", err)
//	}
//
//	if tag.RowsAffected() == 0 {
//		l.WarnContext(ctx, "Interest not found for update")
//		return fmt.Errorf("interest with ID %s not found: %w", interestID.String(), api.ErrNotFound)
//	}
//
//	l.InfoContext(ctx, "Global interest updated successfully")
//	return nil
//}

// UpdateUserInterestPreferenceLevel implements user.UserRepo.
func (r *PostgresUserRepo) UpdateUserInterestPreferenceLevel(ctx context.Context, userID uuid.UUID, interestID uuid.UUID, preferenceLevel int) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "UpdateUserInterestPreferenceLevel", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "user_interests"),
		attribute.String("db.user.id", userID.String()),
		attribute.String("db.interest.id", interestID.String()),
		attribute.Int("preference_level", preferenceLevel),
	))
	defer span.End()

	l := r.logger.With(
		slog.String("method", "UpdateUserInterestPreferenceLevel"),
		slog.String("userID", userID.String()),
		slog.String("interestID", interestID.String()),
		slog.Int("preferenceLevel", preferenceLevel),
	)
	l.DebugContext(ctx, "Updating user interest preference level")

	// Validate preference level
	if preferenceLevel < 0 {
		err := fmt.Errorf("preference level must be non-negative: %d", preferenceLevel)
		l.ErrorContext(ctx, "Invalid preference level", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid preference level")
		return err
	}

	query := `
        UPDATE user_interests 
        SET preference_level = $3
        WHERE user_id = $1 AND interest_id = $2`

	tag, err := r.pgpool.Exec(ctx, query, userID, interestID, preferenceLevel)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user interest preference level", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error updating preference level: %w", err)
	}

	if tag.RowsAffected() == 0 {
		err := fmt.Errorf("interest association not found: %w", api.ErrNotFound)
		l.WarnContext(ctx, "Attempted to update non-existent user interest association")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Association not found")
		return err
	}

	l.InfoContext(ctx, "User interest preference level updated successfully")
	span.SetStatus(codes.Ok, "Preference level updated")
	return nil
}

// GetUserEnhancedInterests implements user.UserRepo.
func (r *PostgresUserRepo) GetUserEnhancedInterests(ctx context.Context, userID uuid.UUID) ([]api.EnhancedInterest, error) {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "GetUserEnhancedInterests", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.sql.table", "user_interests, interests"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "GetUserEnhancedInterests"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user enhanced interests")

	query := `
        SELECT i.id, i.name, i.description, i.active, i.created_at, i.updated_at, ui.preference_level
        FROM interests i
        JOIN user_interests ui ON i.id = ui.interest_id
        WHERE ui.user_id = $1 AND i.active = TRUE
        ORDER BY ui.preference_level DESC, i.name`

	rows, err := r.pgpool.Query(ctx, query, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query user enhanced interests", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return nil, fmt.Errorf("database error fetching enhanced interests: %w", err)
	}
	defer rows.Close()

	var interests []api.EnhancedInterest
	for rows.Next() {
		var i api.EnhancedInterest
		err := rows.Scan(
			&i.ID, &i.Name, &i.Description, &i.Active, &i.CreatedAt, &i.UpdatedAt, &i.PreferenceLevel,
		)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan enhanced interest row", slog.Any("error", err))
			span.RecordError(err)
			return nil, fmt.Errorf("database error scanning enhanced interest: %w", err)
		}
		interests = append(interests, i)
	}

	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating enhanced interest rows", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("database error reading enhanced interests: %w", err)
	}

	l.DebugContext(ctx, "Fetched user enhanced interests successfully", slog.Int("count", len(interests)))
	span.SetStatus(codes.Ok, "Enhanced interests fetched")
	return interests, nil
}

// GetUserPreferenceProfiles implements user.UserRepo.
func (r *PostgresUserRepo) GetUserPreferenceProfiles(ctx context.Context, userID uuid.UUID) ([]api.UserPreferenceProfile, error) {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "GetUserPreferenceProfiles", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.sql.table", "user_preference_profiles"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "GetUserPreferenceProfiles"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user preference profiles")

	query := `
        SELECT id, user_id, profile_name, is_default, search_radius_km, preferred_time, 
               budget_level, preferred_pace, prefer_accessible_pois, prefer_outdoor_seating, 
               prefer_dog_friendly, preferred_vibes, preferred_transport, dietary_needs, 
               created_at, updated_at
        FROM user_preference_profiles
        WHERE user_id = $1
        ORDER BY is_default DESC, profile_name`

	rows, err := r.pgpool.Query(ctx, query, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query user preference profiles", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return nil, fmt.Errorf("database error fetching preference profiles: %w", err)
	}
	defer rows.Close()

	var profiles []api.UserPreferenceProfile
	for rows.Next() {
		var p api.UserPreferenceProfile
		err := rows.Scan(
			&p.ID, &p.UserID, &p.ProfileName, &p.IsDefault, &p.SearchRadiusKm, &p.PreferredTime,
			&p.BudgetLevel, &p.PreferredPace, &p.PreferAccessiblePOIs, &p.PreferOutdoorSeating,
			&p.PreferDogFriendly, &p.PreferredVibes, &p.PreferredTransport, &p.DietaryNeeds,
			&p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan preference profile row", slog.Any("error", err))
			span.RecordError(err)
			return nil, fmt.Errorf("database error scanning preference profile: %w", err)
		}
		profiles = append(profiles, p)
	}

	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating preference profile rows", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("database error reading preference profiles: %w", err)
	}

	l.DebugContext(ctx, "Fetched user preference profiles successfully", slog.Int("count", len(profiles)))
	span.SetStatus(codes.Ok, "Preference profiles fetched")
	return profiles, nil
}

// GetUserPreferenceProfile implements user.UserRepo.
func (r *PostgresUserRepo) GetUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) (*api.UserPreferenceProfile, error) {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "GetUserPreferenceProfile", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.sql.table", "user_preference_profiles"),
		attribute.String("db.profile.id", profileID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "GetUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Fetching user preference profile")

	query := `
        SELECT id, user_id, profile_name, is_default, search_radius_km, preferred_time, 
               budget_level, preferred_pace, prefer_accessible_pois, prefer_outdoor_seating, 
               prefer_dog_friendly, preferred_vibes, preferred_transport, dietary_needs, 
               created_at, updated_at
        FROM user_preference_profiles
        WHERE id = $1`

	var p api.UserPreferenceProfile
	err := r.pgpool.QueryRow(ctx, query, profileID).Scan(
		&p.ID, &p.UserID, &p.ProfileName, &p.IsDefault, &p.SearchRadiusKm, &p.PreferredTime,
		&p.BudgetLevel, &p.PreferredPace, &p.PreferAccessiblePOIs, &p.PreferOutdoorSeating,
		&p.PreferDogFriendly, &p.PreferredVibes, &p.PreferredTransport, &p.DietaryNeeds,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return nil, fmt.Errorf("preference profile not found: %w", api.ErrNotFound)
	}

	l.DebugContext(ctx, "Fetched user preference profile successfully")
	span.SetStatus(codes.Ok, "Preference profile fetched")
	return &p, nil
}

// GetDefaultUserPreferenceProfile implements user.UserRepo.
func (r *PostgresUserRepo) GetDefaultUserPreferenceProfile(ctx context.Context, userID uuid.UUID) (*api.UserPreferenceProfile, error) {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "GetDefaultUserPreferenceProfile", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.sql.table", "user_preference_profiles"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "GetDefaultUserPreferenceProfile"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching default user preference profile")

	query := `
        SELECT id, user_id, profile_name, is_default, search_radius_km, preferred_time, 
               budget_level, preferred_pace, prefer_accessible_pois, prefer_outdoor_seating, 
               prefer_dog_friendly, preferred_vibes, preferred_transport, dietary_needs, 
               created_at, updated_at
        FROM user_preference_profiles
        WHERE user_id = $1 AND is_default = TRUE`

	var p api.UserPreferenceProfile
	err := r.pgpool.QueryRow(ctx, query, userID).Scan(
		&p.ID, &p.UserID, &p.ProfileName, &p.IsDefault, &p.SearchRadiusKm, &p.PreferredTime,
		&p.BudgetLevel, &p.PreferredPace, &p.PreferAccessiblePOIs, &p.PreferOutdoorSeating,
		&p.PreferDogFriendly, &p.PreferredVibes, &p.PreferredTransport, &p.DietaryNeeds,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query default user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return nil, fmt.Errorf("default preference profile not found: %w", api.ErrNotFound)
	}

	l.DebugContext(ctx, "Fetched default user preference profile successfully")
	span.SetStatus(codes.Ok, "Default preference profile fetched")
	return &p, nil
}

// CreateUserPreferenceProfile implements user.UserRepo.
func (r *PostgresUserRepo) CreateUserPreferenceProfile(ctx context.Context, userID uuid.UUID, params api.CreateUserPreferenceProfileParams) (*api.UserPreferenceProfile, error) {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "CreateUserPreferenceProfile", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "user_preference_profiles"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	tx, err := r.pgpool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	l := r.logger.With(slog.String("method", "CreateUserPreferenceProfile"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Creating user preference profile", slog.String("profileName", params.ProfileName))

	// Set default values for optional parameters
	isDefault := false
	if params.IsDefault != nil {
		isDefault = *params.IsDefault
	}

	searchRadiusKm := 5.0
	if params.SearchRadiusKm != nil {
		searchRadiusKm = *params.SearchRadiusKm
	}

	preferredTime := api.DayPreferenceAny
	if params.PreferredTime != nil {
		preferredTime = *params.PreferredTime
	}

	budgetLevel := 0
	if params.BudgetLevel != nil {
		budgetLevel = *params.BudgetLevel
	}

	preferredPace := api.SearchPaceAny
	if params.PreferredPace != nil {
		preferredPace = *params.PreferredPace
	}

	preferAccessiblePOIs := false
	if params.PreferAccessiblePOIs != nil {
		preferAccessiblePOIs = *params.PreferAccessiblePOIs
	}

	preferOutdoorSeating := false
	if params.PreferOutdoorSeating != nil {
		preferOutdoorSeating = *params.PreferOutdoorSeating
	}

	preferDogFriendly := false
	if params.PreferDogFriendly != nil {
		preferDogFriendly = *params.PreferDogFriendly
	}

	preferredVibes := params.PreferredVibes
	if preferredVibes == nil {
		preferredVibes = []string{}
	}

	preferredTransport := api.TransportPreferenceAny
	if params.PreferredTransport != nil {
		preferredTransport = *params.PreferredTransport
	}

	dietaryNeeds := params.DietaryNeeds
	if dietaryNeeds == nil {
		dietaryNeeds = []string{}
	}

	query := `
        INSERT INTO user_preference_profiles (
            user_id, profile_name, is_default, search_radius_km, preferred_time, 
            budget_level, preferred_pace, prefer_accessible_pois, prefer_outdoor_seating, 
            prefer_dog_friendly, preferred_vibes, preferred_transport, dietary_needs
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
        ) RETURNING id, user_id, profile_name, is_default, search_radius_km, preferred_time, 
                   budget_level, preferred_pace, prefer_accessible_pois, prefer_outdoor_seating, 
                   prefer_dog_friendly, preferred_vibes, preferred_transport, dietary_needs, 
                   created_at, updated_at`

	var p api.UserPreferenceProfile
	err = tx.QueryRow(ctx, query,
		userID, params.ProfileName, isDefault, searchRadiusKm, preferredTime,
		budgetLevel, preferredPace, preferAccessiblePOIs, preferOutdoorSeating,
		preferDogFriendly, preferredVibes, preferredTransport, dietaryNeeds,
	).Scan(
		&p.ID, &p.UserID, &p.ProfileName, &p.IsDefault, &p.SearchRadiusKm, &p.PreferredTime,
		&p.BudgetLevel, &p.PreferredPace, &p.PreferAccessiblePOIs, &p.PreferOutdoorSeating,
		&p.PreferDogFriendly, &p.PreferredVibes, &p.PreferredTransport, &p.DietaryNeeds,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // Unique violation
			l.WarnContext(ctx, "Profile name already exists for this user", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Profile name conflict")
			return nil, fmt.Errorf("profile name already exists: %w", api.ErrConflict)
		}
		l.ErrorContext(ctx, "Failed to create user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB INSERT failed")
		return nil, fmt.Errorf("database error creating preference profile: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	l.InfoContext(ctx, "User preference profile created successfully", slog.String("profileID", p.ID.String()))
	span.SetStatus(codes.Ok, "Preference profile created")
	return &p, nil
}

// UpdateUserPreferenceProfile implements user.UserRepo.
func (r *PostgresUserRepo) UpdateUserPreferenceProfile(ctx context.Context, profileID uuid.UUID, params api.UpdateUserPreferenceProfileParams) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "UpdateUserPreferenceProfile", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "user_preference_profiles"),
		attribute.String("db.profile.id", profileID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "UpdateUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Updating user preference profile")

	// Build the update query dynamically based on which fields are provided
	var updates []string
	var args []interface{}
	args = append(args, profileID) // $1 is always profileID

	paramIdx := 2 // Start with $2

	if params.ProfileName != nil {
		updates = append(updates, fmt.Sprintf("profile_name = $%d", paramIdx))
		args = append(args, *params.ProfileName)
		paramIdx++
	}

	if params.IsDefault != nil {
		updates = append(updates, fmt.Sprintf("is_default = $%d", paramIdx))
		args = append(args, *params.IsDefault)
		paramIdx++
	}

	if params.SearchRadiusKm != nil {
		updates = append(updates, fmt.Sprintf("search_radius_km = $%d", paramIdx))
		args = append(args, *params.SearchRadiusKm)
		paramIdx++
	}

	if params.PreferredTime != nil {
		updates = append(updates, fmt.Sprintf("preferred_time = $%d", paramIdx))
		args = append(args, *params.PreferredTime)
		paramIdx++
	}

	if params.BudgetLevel != nil {
		updates = append(updates, fmt.Sprintf("budget_level = $%d", paramIdx))
		args = append(args, *params.BudgetLevel)
		paramIdx++
	}

	if params.PreferredPace != nil {
		updates = append(updates, fmt.Sprintf("preferred_pace = $%d", paramIdx))
		args = append(args, *params.PreferredPace)
		paramIdx++
	}

	if params.PreferAccessiblePOIs != nil {
		updates = append(updates, fmt.Sprintf("prefer_accessible_pois = $%d", paramIdx))
		args = append(args, *params.PreferAccessiblePOIs)
		paramIdx++
	}

	if params.PreferOutdoorSeating != nil {
		updates = append(updates, fmt.Sprintf("prefer_outdoor_seating = $%d", paramIdx))
		args = append(args, *params.PreferOutdoorSeating)
		paramIdx++
	}

	if params.PreferDogFriendly != nil {
		updates = append(updates, fmt.Sprintf("prefer_dog_friendly = $%d", paramIdx))
		args = append(args, *params.PreferDogFriendly)
		paramIdx++
	}

	if params.PreferredVibes != nil {
		updates = append(updates, fmt.Sprintf("preferred_vibes = $%d", paramIdx))
		args = append(args, params.PreferredVibes)
		paramIdx++
	}

	if params.PreferredTransport != nil {
		updates = append(updates, fmt.Sprintf("preferred_transport = $%d", paramIdx))
		args = append(args, *params.PreferredTransport)
		paramIdx++
	}

	if params.DietaryNeeds != nil {
		updates = append(updates, fmt.Sprintf("dietary_needs = $%d", paramIdx))
		args = append(args, params.DietaryNeeds)
		paramIdx++
	}

	// Always update the updated_at timestamp
	updates = append(updates, fmt.Sprintf("updated_at = $%d", paramIdx))
	args = append(args, time.Now())

	// If no updates were provided, return early
	if len(updates) == 1 { // Only updated_at
		l.DebugContext(ctx, "No fields to update")
		return nil
	}

	query := fmt.Sprintf(`
        UPDATE user_preference_profiles
        SET %s
        WHERE id = $1`, strings.Join(updates, ", "))

	tag, err := r.pgpool.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // Unique violation
			l.WarnContext(ctx, "Profile name already exists for this user", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Profile name conflict")
			return fmt.Errorf("profile name already exists: %w", api.ErrConflict)
		}
		l.ErrorContext(ctx, "Failed to update user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error updating preference profile: %w", err)
	}

	if tag.RowsAffected() == 0 {
		err := fmt.Errorf("preference profile not found: %w", api.ErrNotFound)
		l.WarnContext(ctx, "Attempted to update non-existent preference profile")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Profile not found")
		return err
	}

	l.InfoContext(ctx, "User preference profile updated successfully")
	span.SetStatus(codes.Ok, "Preference profile updated")
	return nil
}

// DeleteUserPreferenceProfile implements user.UserRepo.
func (r *PostgresUserRepo) DeleteUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "DeleteUserPreferenceProfile", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.sql.table", "user_preference_profiles"),
		attribute.String("db.profile.id", profileID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "DeleteUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Deleting user preference profile")

	// First check if this is the default profile
	var isDefault bool
	err := r.pgpool.QueryRow(ctx, "SELECT is_default FROM user_preference_profiles WHERE id = $1", profileID).Scan(&isDefault)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err := fmt.Errorf("preference profile not found: %w", api.ErrNotFound)
			l.WarnContext(ctx, "Attempted to delete non-existent preference profile")
			span.RecordError(err)
			span.SetStatus(codes.Error, "Profile not found")
			return err
		}
		l.ErrorContext(ctx, "Failed to check if profile is default", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return fmt.Errorf("database error checking profile: %w", err)
	}

	if isDefault {
		err := errors.New("cannot delete default profile")
		l.WarnContext(ctx, "Attempted to delete default preference profile")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Cannot delete default profile")
		return err
	}

	// Delete the profile
	tag, err := r.pgpool.Exec(ctx, "DELETE FROM user_preference_profiles WHERE id = $1", profileID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to delete user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB DELETE failed")
		return fmt.Errorf("database error deleting preference profile: %w", err)
	}

	if tag.RowsAffected() == 0 {
		// This should not happen since we already checked if the profile exists
		err := fmt.Errorf("preference profile not found: %w", api.ErrNotFound)
		l.WarnContext(ctx, "Attempted to delete non-existent preference profile")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Profile not found")
		return err
	}

	l.InfoContext(ctx, "User preference profile deleted successfully")
	span.SetStatus(codes.Ok, "Preference profile deleted")
	return nil
}

// SetDefaultUserPreferenceProfile implements user.UserRepo.
func (r *PostgresUserRepo) SetDefaultUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "SetDefaultUserPreferenceProfile", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "user_preference_profiles"),
		attribute.String("db.profile.id", profileID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "SetDefaultUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Setting profile as default")

	// First get the user ID for this profile
	var userID uuid.UUID
	err := r.pgpool.QueryRow(ctx, "SELECT user_id FROM user_preference_profiles WHERE id = $1", profileID).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err := fmt.Errorf("preference profile not found: %w", api.ErrNotFound)
			l.WarnContext(ctx, "Attempted to set non-existent profile as default")
			span.RecordError(err)
			span.SetStatus(codes.Error, "Profile not found")
			return err
		}
		l.ErrorContext(ctx, "Failed to get user ID for profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return fmt.Errorf("database error getting profile: %w", err)
	}

	// Begin a transaction
	tx, err := r.pgpool.Begin(ctx)
	if err != nil {
		l.ErrorContext(ctx, "Failed to begin transaction", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB transaction failed")
		return fmt.Errorf("database error beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if not committed

	// First, set all profiles for this user to not be default
	_, err = tx.Exec(ctx, "UPDATE user_preference_profiles SET is_default = FALSE WHERE user_id = $1", userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to reset default profiles", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error resetting default profiles: %w", err)
	}

	// Then set the specified profile as default
	tag, err := tx.Exec(ctx, "UPDATE user_preference_profiles SET is_default = TRUE WHERE id = $1", profileID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to set profile as default", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error setting default profile: %w", err)
	}

	if tag.RowsAffected() == 0 {
		// This should not happen since we already checked if the profile exists
		err := fmt.Errorf("preference profile not found: %w", api.ErrNotFound)
		l.WarnContext(ctx, "Attempted to set non-existent profile as default")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Profile not found")
		return err
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		l.ErrorContext(ctx, "Failed to commit transaction", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB transaction commit failed")
		return fmt.Errorf("database error committing transaction: %w", err)
	}

	l.InfoContext(ctx, "User preference profile set as default successfully")
	span.SetStatus(codes.Ok, "Profile set as default")
	return nil
}

// GetAllGlobalTags implements user.UserRepo.
func (r *PostgresUserRepo) GetAllGlobalTags(ctx context.Context) ([]api.GlobalTag, error) {
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
func (r *PostgresUserRepo) GetUserAvoidTags(ctx context.Context, userID uuid.UUID) ([]api.UserAvoidTag, error) {
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
func (r *PostgresUserRepo) AddUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
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
func (r *PostgresUserRepo) RemoveUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
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

// UpdateLastLogin implements user.UserRepo.
func (r *PostgresUserRepo) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "UpdateLastLogin", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "users"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "UpdateLastLogin"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Updating user last login timestamp")

	query := `
        UPDATE users 
        SET last_login_at = $1, updated_at = $1
        WHERE id = $2`

	now := time.Now()
	tag, err := r.pgpool.Exec(ctx, query, now, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user last login timestamp", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error updating last login: %w", err)
	}

	if tag.RowsAffected() == 0 {
		err := fmt.Errorf("user not found: %w", api.ErrNotFound)
		l.WarnContext(ctx, "Attempted to update last login for non-existent user")
		span.RecordError(err)
		span.SetStatus(codes.Error, "User not found")
		return err
	}

	l.InfoContext(ctx, "User last login timestamp updated successfully")
	span.SetStatus(codes.Ok, "Last login updated")
	return nil
}

// MarkEmailAsVerified implements user.UserRepo.
func (r *PostgresUserRepo) MarkEmailAsVerified(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "MarkEmailAsVerified", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "users"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "MarkEmailAsVerified"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Marking user email as verified")

	query := `
        UPDATE users 
        SET email_verified_at = $1, updated_at = $1
        WHERE id = $2 AND email_verified_at IS NULL`

	now := time.Now()
	tag, err := r.pgpool.Exec(ctx, query, now, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to mark user email as verified", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error marking email as verified: %w", err)
	}

	if tag.RowsAffected() == 0 {
		// Check if the user exists
		var exists bool
		err := r.pgpool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
		if err != nil {
			l.ErrorContext(ctx, "Failed to check if user exists", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "DB query failed")
			return fmt.Errorf("database error checking user existence: %w", err)
		}

		if !exists {
			err := fmt.Errorf("user not found: %w", api.ErrNotFound)
			l.WarnContext(ctx, "Attempted to mark email as verified for non-existent user")
			span.RecordError(err)
			span.SetStatus(codes.Error, "User not found")
			return err
		}

		// User exists but email is already verified
		l.InfoContext(ctx, "User email already verified")
		span.SetStatus(codes.Ok, "Email already verified")
		return nil
	}

	l.InfoContext(ctx, "User email marked as verified successfully")
	span.SetStatus(codes.Ok, "Email verified")
	return nil
}

// DeactivateUser implements user.UserRepo.
func (r *PostgresUserRepo) DeactivateUser(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "DeactivateUser", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "users"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "DeactivateUser"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Deactivating user")

	// Begin a transaction
	tx, err := r.pgpool.Begin(ctx)
	if err != nil {
		l.ErrorContext(ctx, "Failed to begin transaction", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB transaction failed")
		return fmt.Errorf("database error beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if not committed

	// First, check if the user exists and is active
	var isActive bool
	err = tx.QueryRow(ctx, "SELECT is_active FROM users WHERE id = $1", userID).Scan(&isActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			l.WarnContext(ctx, "Attempted to deactivate non-existent user")
			span.RecordError(err)
			span.SetStatus(codes.Error, "User not found")
			return fmt.Errorf("user not found: %w", api.ErrNotFound)
		}
		l.ErrorContext(ctx, "Failed to check user active status", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return fmt.Errorf("database error checking user status: %w", err)
	}

	if !isActive {
		l.InfoContext(ctx, "User is already inactive")
		span.SetStatus(codes.Ok, "User already inactive")
		return nil
	}

	// Deactivate the user
	_, err = tx.Exec(ctx, "UPDATE users SET is_active = FALSE, updated_at = $1 WHERE id = $2", time.Now(), userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to deactivate user", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error deactivating user: %w", err)
	}

	// Invalidate all refresh tokens
	_, err = tx.Exec(ctx, "UPDATE refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL", time.Now(), userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to invalidate refresh tokens", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error invalidating refresh tokens: %w", err)
	}

	// Invalidate all sessions
	_, err = tx.Exec(ctx, "UPDATE sessions SET invalidated_at = $1 WHERE user_id = $2 AND invalidated_at IS NULL", time.Now(), userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to invalidate sessions", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error invalidating sessions: %w", err)
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		l.ErrorContext(ctx, "Failed to commit transaction", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB transaction commit failed")
		return fmt.Errorf("database error committing transaction: %w", err)
	}

	l.InfoContext(ctx, "User deactivated successfully")
	span.SetStatus(codes.Ok, "User deactivated")
	return nil
}

// ReactivateUser implements user.UserRepo.
func (r *PostgresUserRepo) ReactivateUser(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("UserRepo").Start(ctx, "ReactivateUser", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "users"),
		attribute.String("db.user.id", userID.String()),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "ReactivateUser"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Reactivating user")

	// Check if the user exists and is inactive
	var isActive bool
	err := r.pgpool.QueryRow(ctx, "SELECT is_active FROM users WHERE id = $1", userID).Scan(&isActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			l.WarnContext(ctx, "Attempted to reactivate non-existent user")
			span.RecordError(err)
			span.SetStatus(codes.Error, "User not found")
			return fmt.Errorf("user not found: %w", api.ErrNotFound)
		}
		l.ErrorContext(ctx, "Failed to check user active status", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB query failed")
		return fmt.Errorf("database error checking user status: %w", err)
	}

	if isActive {
		l.InfoContext(ctx, "User is already active")
		span.SetStatus(codes.Ok, "User already active")
		return nil
	}

	// Reactivate the user
	_, err = r.pgpool.Exec(ctx, "UPDATE users SET is_active = TRUE, updated_at = $1 WHERE id = $2", time.Now(), userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to reactivate user", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		return fmt.Errorf("database error reactivating user: %w", err)
	}

	l.InfoContext(ctx, "User reactivated successfully")
	span.SetStatus(codes.Ok, "User reactivated")
	return nil
}
