//go:build integration

package profiles

import (
	"context"
	// "database/sql" // For sql.NullString if needed for direct inserts
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"

	interestsRepoImpl "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/interests/repository" // Path to actual interest repo impl
	userTagRepoImpl "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/tags/repository"        // Path to actual tag repo impl
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"                                      // Adjust path

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testUserProfileDB *pgxpool.Pool
var testUserProfileService profilessService             // Use the interface
var testinterestsRepoForProfile interests.interestsRepo // Actual repo for setup
var testUserTagRepoForProfile tags.tagsRepo             // Actual repo for setup
// var testUserProfileRepo profilessRepo // Not strictly needed if testing through service, but can be useful for direct verification

func TestMain(m *testing.M) {
	if err := godotenv.Load("../../../.env.test"); err != nil {
		log.Println("Warning: .env.test file not found for user_search_profile integration tests.")
	}
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		log.Fatal("TEST_DATABASE_URL is not set for user_search_profile integration tests")
	}

	var err error
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("Unable to parse TEST_DATABASE_URL: %v\n", err)
	}
	config.MaxConns = 5

	testUserProfileDB, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
	}
	defer testUserProfileDB.Close()
	if err := testUserProfileDB.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping test database: %v\n", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Initialize actual repositories
	profilesRepo := NewPostgresprofilessRepo(testUserProfileDB, logger)                          // Your actual constructor
	testinterestsRepoForProfile = interestsRepoImpl.NewRepositoryImpl(testUserProfileDB, logger) // Actual constructor
	testUserTagRepoForProfile = userTagRepoImpl.NewRepositoryImpl(testUserProfileDB, logger)     // Actual constructor

	testUserProfileService = NewUserProfilesService(profilesRepo, testinterestsRepoForProfile, testUserTagRepoForProfile, logger)

	exitCode := m.Run()
	os.Exit(exitCode)
}

// Helper to clear tables
func clearUserPreferenceProfilesTable(t *testing.T) {
	t.Helper()
	// Must delete from linking tables first if foreign keys exist
	_, err := testUserProfileDB.Exec(context.Background(), "DELETE FROM profile_interests") // Adjust table name
	require.NoError(t, err)
	_, err = testUserProfileDB.Exec(context.Background(), "DELETE FROM profile_tags") // Adjust table name
	require.NoError(t, err)
	_, err = testUserProfileDB.Exec(context.Background(), "DELETE FROM user_preference_profiles")
	require.NoError(t, err)
}
func clearUserProfileTestUsers(t *testing.T) { // Separate user cleanup if needed
	t.Helper()
	// Assuming user_preference_profiles FK to users is ON DELETE CASCADE or handled
	// Or delete users that are only test users. For simplicity, this might not be needed if users are few.
}
func clearUserProfileTestInterests(t *testing.T) {
	t.Helper()
	_, err := testUserProfileDB.Exec(context.Background(), "DELETE FROM interests WHERE name LIKE 'IntegTestInterest%'")
	require.NoError(t, err)
}
func clearUserProfileTestTags(t *testing.T) {
	t.Helper()
	_, err := testUserProfileDB.Exec(context.Background(), "DELETE FROM tags WHERE name LIKE 'IntegTestTag%'")
	require.NoError(t, err)
}

// Helper to create a user directly for testing setup
func createTestUserForProfileTests(t *testing.T, usernameSuffix string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	username := "profile_user_" + usernameSuffix
	_, err := testUserProfileDB.Exec(context.Background(),
		"INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, $4) ON CONFLICT (username) DO NOTHING RETURNING id", // Assuming username is unique
		id, username, fmt.Sprintf("%s@example.com", username), "test_hash").Scan(&id) // Scan back ID in case of conflict
	if err != nil && err != pgx.ErrNoRows { // ErrNoRows if ON CONFLICT DO NOTHING and it did nothing
		// if it did nothing, we need to fetch the existing one
		errFetch := testUserProfileDB.QueryRow(context.Background(), "SELECT id FROM users WHERE username = $1", username).Scan(&id)
		require.NoError(t, errFetch, "Failed to fetch existing test user after conflict")
	} else {
		require.NoError(t, err, "Failed to insert test user")
	}
	return id
}

func createTestInterestForProfileTests(t *testing.T, name string, userID uuid.UUID) *types.Interest {
	t.Helper()
	// Use the actual interest service/repo to create it if possible, or direct insert
	// For consistency, using the repo methods (which interestsService wraps)
	// Assuming interestsService has a CreateInterest method in its repo.
	desc := "Integration test interest"
	interest, err := testinterestsRepoForProfile.CreateInterest(context.Background(), name, &desc, true, userID.String())
	require.NoError(t, err)
	return interest
}

func createTestTagForProfileTests(t *testing.T, name string, userID uuid.UUID) *types.Tags {
	t.Helper()
	desc := "Integration test tag"
	tag, err := testUserTagRepoForProfile.CreateTag(context.Background(), name, &desc, userID.String()) // Assuming CreateTag in repo
	require.NoError(t, err)
	return tag
}

func TestprofilessServiceImpl_Integration(t *testing.T) {
	ctx := context.Background()
	// Clear all relevant tables at the start of this test suite
	clearUserPreferenceProfilesTable(t)
	clearUserProfileTestInterests(t)
	clearUserProfileTestTags(t)
	// clearUserProfileTestUsers(t) // If users are specific to these tests and not shared

	userID1 := createTestUserForProfileTests(t, "one")
	userID2 := createTestUserForProfileTests(t, "two")

	interest1 := createTestInterestForProfileTests(t, "IntegTestInterest_Art", userID1)
	interest2 := createTestInterestForProfileTests(t, "IntegTestInterest_Food", userID1)
	tag1 := createTestTagForProfileTests(t, "IntegTestTag_Outdoor", userID1)
	tag2 := createTestTagForProfileTests(t, "IntegTestTag_Budget", userID1)

	var createdProfileID1 uuid.UUID
	var createdProfileID2 uuid.UUID

	t.Run("CreateSearchProfile", func(t *testing.T) {
		params1 := types.CreateUserPreferenceProfileParams{
			ProfileName:    "Weekend Getaway",
			IsDefault:      true,
			SearchRadiusKm: 10.0,
			Interests:      []uuid.UUID{interest1.ID, interest2.ID},
			Tags:           []uuid.UUID{tag1.ID},
			// ... other preferences
		}
		// Using CreateSearchProfile (the simpler one that does sequential linking)
		profile1, err := testUserProfileService.CreateSearchProfile(ctx, userID1, params1)
		require.NoError(t, err)
		require.NotNil(t, profile1)
		createdProfileID1 = profile1.ID
		assert.Equal(t, "Weekend Getaway", profile1.ProfileName)
		assert.True(t, profile1.IsDefault)
		assert.Len(t, profile1.Interests, 2) // Assuming CreateSearchProfile fetches them back
		assert.Len(t, profile1.Tags, 1)

		params2 := types.CreateUserPreferenceProfileParams{
			ProfileName: "Quick Bites",
			IsDefault:   false,
			Interests:   []uuid.UUID{interest2.ID},
			Tags:        []uuid.UUID{tag2.ID},
		}
		profile2, err := testUserProfileService.CreateSearchProfile(ctx, userID1, params2)
		require.NoError(t, err)
		require.NotNil(t, profile2)
		createdProfileID2 = profile2.ID
		assert.Equal(t, "Quick Bites", profile2.ProfileName)
		assert.False(t, profile2.IsDefault)
	})

	t.Run("GetSearchProfiles for user1", func(t *testing.T) {
		profiles, err := testUserProfileService.GetSearchProfiles(ctx, userID1)
		require.NoError(t, err)
		assert.Len(t, profiles, 2) // We created two for userID1
	})

	t.Run("GetSearchProfile for existing profile", func(t *testing.T) {
		profile, err := testUserProfileService.GetSearchProfile(ctx, userID1, createdProfileID1)
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, "Weekend Getaway", profile.ProfileName)
		assert.Len(t, profile.Interests, 2) // Assuming the response includes linked items
		assert.Len(t, profile.Tags, 1)
	})

	t.Run("GetDefaultSearchProfile for user1", func(t *testing.T) {
		defaultProfile, err := testUserProfileService.GetDefaultSearchProfile(ctx, userID1)
		require.NoError(t, err)
		require.NotNil(t, defaultProfile)
		assert.Equal(t, "Weekend Getaway", defaultProfile.ProfileName)
		assert.True(t, defaultProfile.IsDefault)
	})

	t.Run("UpdateSearchProfile", func(t *testing.T) {
		newName := "Updated Weekend Adventure"
		newRadius := 15.5
		updateParams := types.UpdateSearchProfileParams{
			ProfileName:    &newName,
			SearchRadiusKm: &newRadius,
			// To test interest/tag updates, UpdateSearchProfileParams needs fields for them
			// and the service/repo logic to handle re-linking.
		}
		err := testUserProfileService.UpdateSearchProfile(ctx, userID1, createdProfileID1, updateParams)
		require.NoError(t, err)

		profile, err := testUserProfileService.GetSearchProfile(ctx, userID1, createdProfileID1)
		require.NoError(t, err)
		assert.Equal(t, newName, profile.ProfileName)
		assert.Equal(t, newRadius, profile.SearchRadiusKm)
	})

	t.Run("SetDefaultSearchProfile", func(t *testing.T) {
		// Set profile2 as default
		err := testUserProfileService.SetDefaultSearchProfile(ctx, userID1, createdProfileID2)
		require.NoError(t, err)

		defaultProfile, err := testUserProfileService.GetDefaultSearchProfile(ctx, userID1)
		require.NoError(t, err)
		require.NotNil(t, defaultProfile)
		assert.Equal(t, createdProfileID2, defaultProfile.ID)
		assert.True(t, defaultProfile.IsDefault)

		// Check old default is no longer default
		oldDefaultProfile, err := testUserProfileService.GetSearchProfile(ctx, userID1, createdProfileID1)
		require.NoError(t, err)
		assert.False(t, oldDefaultProfile.IsDefault)
	})

	t.Run("DeleteSearchProfile", func(t *testing.T) {
		err := testUserProfileService.DeleteSearchProfile(ctx, userID1, createdProfileID1)
		require.NoError(t, err)

		_, err = testUserProfileService.GetSearchProfile(ctx, userID1, createdProfileID1)
		require.Error(t, err) // Should be not found

		profiles, _ := testUserProfileService.GetSearchProfiles(ctx, userID1)
		assert.Len(t, profiles, 1) // Only profile2 should remain for userID1
	})

	t.Run("GetSearchProfiles for user2 (should be empty)", func(t *testing.T) {
		profiles, err := testUserProfileService.GetSearchProfiles(ctx, userID2)
		require.NoError(t, err)
		assert.Empty(t, profiles)
	})
}

// To run:
// TEST_DATABASE_URL="postgres://user:pass@host:port/dbname_test?sslmode=disable" go test -v ./internal/profiles -tags=integration -count=1
