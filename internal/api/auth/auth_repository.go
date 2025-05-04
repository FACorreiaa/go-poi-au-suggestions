package auth

import (
	"context"
	"errors"

	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

var _ AuthRepo = (*PostgresAuthRepo)(nil)

type AuthRepo interface {
	// GetUserByEmail fetches user details needed for validation/token generation.
	GetUserByEmail(ctx context.Context, email string) (*types.UserAuth, error)
	// GetUserByID fetches user details by ID.
	GetUserByID(ctx context.Context, userID string) (*types.UserAuth, error)
	// Register stores a new user with a HASHED password. Returns new user ID.
	Register(ctx context.Context, username, email, hashedPassword string) (string, error)
	// VerifyPassword checks if the given password matches the hash for the userID.
	VerifyPassword(ctx context.Context, userID, password string) error // Password is plain text here
	// UpdatePassword updates the user's HASHED password.
	UpdatePassword(ctx context.Context, userID, newHashedPassword string) error

	// --- Refresh Token Handling ---
	// StoreRefreshToken saves a new refresh token for a user.
	StoreRefreshToken(ctx context.Context, userID, token string, expiresAt time.Time) error
	// ValidateRefreshTokenAndGetUserID checks if a refresh token is valid and returns the user ID.
	ValidateRefreshTokenAndGetUserID(ctx context.Context, refreshToken string) (userID string, err error)
	// InvalidateRefreshToken marks a specific refresh token as revoked.
	InvalidateRefreshToken(ctx context.Context, refreshToken string) error
	// InvalidateAllUserRefreshTokens marks all tokens for a user as revoked.
	InvalidateAllUserRefreshTokens(ctx context.Context, userID string) error
}

type PostgresAuthRepo struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewPostgresAuthRepo(pgxpool *pgxpool.Pool, logger *slog.Logger) *PostgresAuthRepo {
	return &PostgresAuthRepo{
		logger: logger,
		pgpool: pgxpool,
	}
}

// GetUserByEmail implements auth.AuthRepo.
func (r *PostgresAuthRepo) GetUserByEmail(ctx context.Context, email string) (*types.UserAuth, error) {
	var user types.UserAuth
	query := `SELECT id, username, email, password_hash FROM users WHERE email = $1 AND is_active = TRUE`
	err := r.pgpool.QueryRow(ctx, query, email).Scan(&user.ID, &user.Username, &user.Email, &user.Password)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user with email %s not found: %w", email, types.ErrNotFound) // Use a domain error
		}
		r.logger.ErrorContext(ctx, "Error fetching user by email", slog.Any("error", err), slog.String("email", email))
		return nil, fmt.Errorf("database error fetching user: %w", err)
	}
	return &user, nil
}

// GetUserByID implements auth.AuthRepo.
func (r *PostgresAuthRepo) GetUserByID(ctx context.Context, userID string) (*types.UserAuth, error) {
	var user types.UserAuth
	// Select fields needed by token generation or other logic
	query := `SELECT id, username, email FROM users WHERE id = $1 AND is_active = TRUE`
	err := r.pgpool.QueryRow(ctx, query, userID).Scan(&user.ID, &user.Username, &user.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user with ID %s not found: %w", userID, types.ErrNotFound) // Use a domain error
		}
		r.logger.ErrorContext(ctx, "Error fetching user by ID", slog.Any("error", err), slog.String("userID", userID))
		return nil, fmt.Errorf("database error fetching user by ID: %w", err)
	}
	return &user, nil
}

// Register implements auth.AuthRepo. Expects HASHED password.
func (r *PostgresAuthRepo) Register(ctx context.Context, username, email, hashedPassword string) (string, error) {
	tracer := otel.Tracer("MyRESTAPI")

	// Start a span for the repository layer
	ctx, span := tracer.Start(ctx, "PostgresAuthRepo.Register", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.system", "postgresql"),
		attribute.String("db.statement", "INSERT INTO users ..."),
	))
	defer span.End()

	// Record query start time
	//startTime := time.Now()

	var userID string
	query := `INSERT INTO users (username, email, password_hash, created_at) VALUES ($1, $2, $3, $4) RETURNING id`
	err := r.pgpool.QueryRow(ctx, query, username, email, hashedPassword, time.Now()).Scan(&userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Database error")
		//dbQueryErrorsTotal.Add(ctx, 1)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", fmt.Errorf("email or username already exists: %w", types.ErrConflict)
		}
		r.logger.ErrorContext(ctx, "Error inserting user", slog.Any("error", err), slog.String("email", email))
		return "", fmt.Errorf("database error registering user: %w", err)
	}

	// Record query duration
	//duration := time.Since(startTime).Seconds()
	//dbQueryDurationSeconds.Record(ctx, duration)

	span.SetStatus(codes.Ok, "User inserted into database")
	return userID, nil
}

// VerifyPassword implements auth.AuthRepo. Compares plain password to stored hash.
func (r *PostgresAuthRepo) VerifyPassword(ctx context.Context, userID, password string) error {
	var storedHash string
	query := `SELECT password_hash FROM users WHERE id = $1 AND is_active = TRUE`
	err := r.pgpool.QueryRow(ctx, query, userID).Scan(&storedHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user not found: %w", types.ErrNotFound)
		}
		r.logger.ErrorContext(ctx, "Error fetching password hash for verification", slog.Any("error", err), slog.String("userID", userID))
		return fmt.Errorf("database error verifying password: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
	if err != nil {
		// Don't log the actual password, but log the failure type
		l := r.logger.With(slog.String("userID", userID))
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			l.WarnContext(ctx, "Password mismatch during verification")
			return fmt.Errorf("invalid password: %w", types.ErrUnauthenticated) // Specific error
		}
		l.ErrorContext(ctx, "Error comparing password hash", slog.Any("error", err))
		return fmt.Errorf("error during password comparison: %w", err)
	}
	return nil
}

// UpdatePassword implements auth.AuthRepo. Expects HASHED password.
func (r *PostgresAuthRepo) UpdatePassword(ctx context.Context, userID, newHashedPassword string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2 AND is_active = TRUE`
	tag, err := r.pgpool.Exec(ctx, query, newHashedPassword, userID)
	if err != nil {
		r.logger.ErrorContext(ctx, "Error updating password hash", slog.Any("error", err), slog.String("userID", userID))
		return fmt.Errorf("database error updating password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		r.logger.WarnContext(ctx, "User not found or no password change needed during update", slog.String("userID", userID))
		return fmt.Errorf("user not found or password unchanged: %w", types.ErrNotFound) // Or a different domain error
	}
	return nil
}

// StoreRefreshToken implements auth.AuthRepo.
func (r *PostgresAuthRepo) StoreRefreshToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	query := `INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`
	_, err := r.pgpool.Exec(ctx, query, userID, token, expiresAt)
	if err != nil {
		r.logger.ErrorContext(ctx, "Error storing refresh token", slog.Any("error", err), slog.String("userID", userID))
		return fmt.Errorf("database error storing refresh token: %w", err)
	}
	return nil
}

// ValidateRefreshTokenAndGetUserID implements auth.AuthRepo.
func (r *PostgresAuthRepo) ValidateRefreshTokenAndGetUserID(ctx context.Context, refreshToken string) (string, error) {
	var userID string
	var expiresAt time.Time
	var revokedAt *time.Time // Use pointer for nullable timestamp

	query := `SELECT user_id, expires_at, revoked_at FROM refresh_tokens WHERE token = $1`
	err := r.pgpool.QueryRow(ctx, query, refreshToken).Scan(&userID, &expiresAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("refresh token not found: %w", types.ErrUnauthenticated)
		}
		r.logger.ErrorContext(ctx, "Error querying refresh token", slog.Any("error", err))
		return "", fmt.Errorf("database error validating refresh token: %w", err)
	}

	if revokedAt != nil {
		return "", fmt.Errorf("refresh token has been revoked: %w", types.ErrUnauthenticated)
	}
	if time.Now().After(expiresAt) {
		return "", fmt.Errorf("refresh token has expired: %w", types.ErrUnauthenticated)
	}

	return userID, nil // Return only the userID
}

// InvalidateRefreshToken implements auth.AuthRepo.
func (r *PostgresAuthRepo) InvalidateRefreshToken(ctx context.Context, refreshToken string) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE token = $1 AND revoked_at IS NULL`
	tag, err := r.pgpool.Exec(ctx, query, refreshToken)
	if err != nil {
		r.logger.ErrorContext(ctx, "Error invalidating refresh token", slog.Any("error", err))
		return fmt.Errorf("database error invalidating token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		r.logger.WarnContext(ctx, "Refresh token not found or already invalidated during invalidation attempt")
		// Depending on context (e.g., logout), this might not be a critical error
		// return fmt.Errorf("token not found or already revoked: %w", ErrNotFound)
	}
	return nil
}

// InvalidateAllUserRefreshTokens implements auth.AuthRepo.
func (r *PostgresAuthRepo) InvalidateAllUserRefreshTokens(ctx context.Context, userID string) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.pgpool.Exec(ctx, query, userID)
	if err != nil {
		r.logger.ErrorContext(ctx, "Error invalidating all refresh tokens for user", slog.Any("error", err), slog.String("userID", userID))
		return fmt.Errorf("database error invalidating tokens: %w", err)
	}
	// Log how many were invalidated? (tag.RowsAffected())
	return nil
}
