//go:build integration

package settings

import (
	"context"
	// "database/sql" // If direct inserts need sql.Null types
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types" // Adjust path
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testsettingsDB *pgxpool.Pool
var testSettingsService SettingsService // Use the interface
var testSettingsRepo SettingsRepository    // Actual repository implementation for setup/cleanup

// settings structure for database interaction (matching your table)
// This should align with your types.settings but might be simpler for direct DB seeding.
type dbsettings struct {
	UserID              uuid.UUID
	ProfileID           uuid.UUID // Assuming this exists and is part of a PK or unique constraint with UserID
	ReceiveNotifications bool
	Theme               string
	Language            string
	// CreatedAt and UpdatedAt are usually handled by DB defaults or triggers
}


func TestMain(m *testing.M) {
	if err := godotenv.Load("../../../.env.test"); err != nil { // Adjust path
		log.Println("Warning: .env.test file not found for settings integration tests.")
	}

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		log.Fatal("TEST_DATABASE_URL environment variable is not set for settings integration tests")
	}

	var err error
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil { log.Fatalf("Unable to parse TEST_DATABASE_URL: %v\n", err) }
	config.MaxConns = 5

	testsettingsDB, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil { log.Fatalf("Unable to create connection pool for settings tests: %v\n", err) }
	defer testsettingsDB.Close()

	if err := testsettingsDB.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping test database for settings tests: %v\n", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	// Initialize with your *actual* PostgresSettingsRepository implementation
	testSettingsRepo = NewPostgresSettingsRepository(testsettingsDB, logger) // Replace with your actual repo constructor
	testSettingsService = NewsettingsService(testSettingsRepo, logger)

	exitCode := m.Run()
	os.Exit(exitCode)
}

// Helper to clear the user_settings table (adjust table name)
func clearsettingsTable(t *testing.T) {
	t.Helper()
	_, err := testsettingsDB.Exec(context.Background(), "DELETE FROM user_settings")
	require.NoError(t, err, "Failed to clear user_settings table")
}

// Helper to create a user directly for FK constraints if needed
func createTestUserForSettingsTests(t *testing.T, id uuid.UUID, username string) {
    t.Helper()
    // This assumes a 'users' table exists for FK 'user_settings.user_id'
    // If your user_settings.user_id is the PK and not an FK, this isn't strictly needed
    // for settings table alone, but good if you have relations.
    _, err := testsettingsDB.Exec(context.Background(),
        "INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING",
        id, username, fmt.Sprintf("%s_settings@example.com", username), "test_hash")
    require.NoError(t, err, "Failed to insert test user for settings")
}


// Helper to create settings directly for testing setup
func createTestSettingsDirectly(t *testing.T, settings dbsettings) {
	t.Helper()
	_, err := testsettingsDB.Exec(context.Background(),
		"INSERT INTO user_settings (user_id, profile_id, receive_notifications, theme, language, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, NOW(), NOW()) ON CONFLICT (user_id, profile_id) DO UPDATE SET receive_notifications = $3, theme = $4, language = $5, updated_at = NOW()",
		settings.UserID, settings.ProfileID, settings.ReceiveNotifications, settings.Theme, settings.Language)
	require.NoError(t, err, "Failed to insert test settings")
}

func TestSettingsServiceImpl_Integration(t *testing.T) {
	ctx := context.Background()
	clearsettingsTable(t) // Clear before all sub-tests in this suite

	userID1 := uuid.New()
	profileID1 := uuid.New() // Assuming a user might have multiple settings profiles, or this is just an ID for the settings row itself.
	createTestUserForSettingsTests(t, userID1, "settings_user1") // Create user if settings table has FK

	initialSettings := dbsettings{
		UserID:              userID1,
		ProfileID:           profileID1,
		ReceiveNotifications: true,
		Theme:               "dark",
		Language:            "en",
	}
	createTestSettingsDirectly(t, initialSettings)

	t.Run("Get User Settings - Found", func(t *testing.T) {
		settings, err := testSettingsService.Getsettings(ctx, userID1)
		require.NoError(t, err)
		require.NotNil(t, settings)
		assert.Equal(t, userID1, settings.UserID)
		assert.Equal(t, profileID1, settings.ProfileID) // Check if your Get returns this
		assert.Equal(t, initialSettings.ReceiveNotifications, settings.ReceiveNotifications)
		assert.Equal(t, initialSettings.Theme, settings.Theme)
		assert.Equal(t, initialSettings.Language, settings.Language)
	})

	t.Run("Get User Settings - Not Found", func(t *testing.T) {
		nonExistentUserID := uuid.New()
		_, err := testSettingsService.Getsettings(ctx, nonExistentUserID)
		require.Error(t, err)
		// Check for a specific "not found" error, e.g., by checking if errors.Is(err, types.ErrNotFound)
		// or by string contains if your repo/service formats it that way.
		assert.Contains(t, err.Error(), "error fetching user settings") // Service wraps it
	})

	t.Run("Update User Settings - Success", func(t *testing.T) {
		newTheme := "light_mode"
		newLang := "fr"
		notificationsOff := false
		updateParams := UpdatesettingsParams{
			Theme:                &newTheme,
			Language:             &newLang,
			ReceiveNotifications: ¬ificationsOff,
		}

		// The profileID in Updatesettings refers to the specific settings profile to update for the user.
		err := testSettingsService.Updatesettings(ctx, userID1, profileID1, updateParams)
		require.NoError(t, err)

		// Verify by fetching again
		updatedSettings, err := testSettingsService.Getsettings(ctx, userID1)
		require.NoError(t, err)
		require.NotNil(t, updatedSettings)
		assert.Equal(t, newTheme, updatedSettings.Theme)
		assert.Equal(t, newLang, updatedSettings.Language)
		assert.Equal(t, notificationsOff, updatedSettings.ReceiveNotifications)
		assert.True(t, updatedSettings.UpdatedAt.After(initialSettings.UpdatedAt), "UpdatedAt should be more recent")
	})

	t.Run("Update User Settings - Partial Update (only theme)", func(t *testing.T) {
		// Fetch current to ensure other fields don't change
		currentSettings, _ := testSettingsService.Getsettings(ctx, userID1)
		originalLang := currentSettings.Language
		originalNotifications := currentSettings.ReceiveNotifications

		veryNewTheme := "solarized"
		partialUpdateParams := UpdatesettingsParams{
			Theme: &veryNewTheme,
		}
		err := testSettingsService.Updatesettings(ctx, userID1, profileID1, partialUpdateParams)
		require.NoError(t, err)

		updatedSettings, err := testSettingsService.Getsettings(ctx, userID1)
		require.NoError(t, err)
		require.NotNil(t, updatedSettings)
		assert.Equal(t, veryNewTheme, updatedSettings.Theme)
		assert.Equal(t, originalLang, updatedSettings.Language) // Should not change
		assert.Equal(t, originalNotifications, updatedSettings.ReceiveNotifications) // Should not change
	})

	t.Run("Update User Settings - Not Found (wrong profileID or userID)", func(t *testing.T) {
		nonExistentProfileID := uuid.New()
		theme := "any"
		updateParams := UpdatesettingsParams{Theme: &theme}
		err := testSettingsService.Updatesettings(ctx, userID1, nonExistentProfileID, updateParams)
		require.Error(t, err)
		// Assert specific "not found" error if your repo/service provides it
		assert.Contains(t, err.Error(), "error updating user settings")

		nonExistentUserID := uuid.New()
		err = testSettingsService.Updatesettings(ctx, nonExistentUserID, profileID1, updateParams)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error updating user settings")
	})
}

// To run integration tests:
// TEST_DATABASE_URL="postgres://user:password@localhost:5432/test_db_name?sslmode=disable" go test -v ./internal/settings -tags=integration -count=1