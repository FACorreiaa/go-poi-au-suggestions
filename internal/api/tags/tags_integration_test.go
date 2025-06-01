//go:build integration

package tags

import (
	"context" // For sql.NullString
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types" // Adjust path
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testtagsDB *pgxpool.Pool
var testtagsService tagsService // Use the interface
// var testtagsRepo tagsRepo    // Actual repository for direct interaction if needed for setup

func TestMain(m *testing.M) {
	if err := godotenv.Load("../../../.env.test"); err != nil { // Adjust path
		log.Println("Warning: .env.test file not found for tags integration tests.")
	}

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		log.Fatal("TEST_DATABASE_URL environment variable is not set for tags integration tests")
	}

	var err error
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("Unable to parse TEST_DATABASE_URL: %v\n", err)
	}
	config.MaxConns = 5

	testtagsDB, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("Unable to create connection pool for tags tests: %v\n", err)
	}
	defer testtagsDB.Close()

	if err := testtagsDB.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping test database for tags tests: %v\n", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	// Initialize with your *actual* PostgrestagsRepo implementation
	realRepo := NewPostgrestagsRepo(testtagsDB, logger) // Replace with your actual repo constructor
	testtagsService = NewtagsService(realRepo, logger)
	// testtagsRepo = realRepo // If needed for direct DB manipulation in tests

	exitCode := m.Run()
	os.Exit(exitCode)
}

// Helper to clear tables
func clearUserPersonalTagsTable(t *testing.T) {
	t.Helper()
	// Adjust table name if it's different (e.g., 'tags' or 'personal_tags')
	_, err := testtagsDB.Exec(context.Background(), "DELETE FROM personal_tags")
	require.NoError(t, err, "Failed to clear personal_tags table")
}
func clearTagsTable(t *testing.T) { // If you have a global tags table
	t.Helper()
	// If personal_tags FK to tags, delete personal_tags first or handle cascades
	clearUserPersonalTagsTable(t)
	_, err := testtagsDB.Exec(context.Background(), "DELETE FROM tags WHERE name LIKE 'IntegTestGlobalTag%'") // Be specific
	require.NoError(t, err)
}

// Helper to create a user directly for FK constraints
func createTestUserForTagTests(t *testing.T, usernameSuffix string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	username := "tag_user_" + usernameSuffix
	_, err := testtagsDB.Exec(context.Background(),
		"INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, $4) ON CONFLICT (username) DO UPDATE SET email = $3 RETURNING id",
		id, username, fmt.Sprintf("%s@example.com", username), "test_hash").Scan(&id)
	require.NoError(t, err, "Failed to insert/find test user for tags")
	return id
}

// Helper to convert string to *string for tests
func stringPtr(s string) *string {
	return &s
}

func TesttagsServiceImpl_Integration(t *testing.T) {
	ctx := context.Background()
	clearUserPersonalTagsTable(t) // Clear before all sub-tests in this suite
	// clearTagsTable(t) // If you also have a global tags table managed by these tests

	userID1 := createTestUserForTagTests(t, "one")
	userID2 := createTestUserForTagTests(t, "two")

	var createdTagID1 uuid.UUID
	var createdTagID2 uuid.UUID

	t.Run("CreateTag for user1", func(t *testing.T) {
		params := types.CreatePersonalTagParams{
			Name:        "Vegan Options",
			Description: stringPtr("Places with good vegan food"),
		}
		tag, err := testtagsService.CreateTag(ctx, userID1, params)
		require.NoError(t, err)
		require.NotNil(t, tag)
		createdTagID1 = tag.ID
		assert.Equal(t, "Vegan Options", tag.Name)
		assert.Equal(t, "Places with good vegan food", tag.Description.String)
		assert.Equal(t, userID1, tag.UserID) // Assuming PersonalTag struct has UserID

		// Create another tag for the same user
		params2 := types.CreatePersonalTagParams{Name: "Quiet Study"}
		tag2, err := testtagsService.CreateTag(ctx, userID1, params2)
		require.NoError(t, err)
		createdTagID2 = tag2.ID
	})

	t.Run("CreateTag for user2", func(t *testing.T) {
		params := types.CreatePersonalTagParams{
			Name:        "Dog Friendly",
			Description: stringPtr("Allows dogs inside or has patio"),
		}
		_, err := testtagsService.CreateTag(ctx, userID2, params)
		require.NoError(t, err)
	})

	t.Run("GetTags for user1 (should list personal tags)", func(t *testing.T) {
		// The GetTags method in service calls repo.GetAll(ctx, userID)
		// This implies it gets tags *specific* to or *created by* that user.
		tags, err := testtagsService.GetTags(ctx, userID1)
		require.NoError(t, err)
		assert.Len(t, tags, 2, "User1 should have 2 tags they created")

		foundVegan := false
		foundQuiet := false
		for _, tag := range tags {
			if tag.ID == createdTagID1 {
				foundVegan = true
				assert.Equal(t, "Vegan Options", tag.Name)
			}
			if tag.ID == createdTagID2 {
				foundQuiet = true
				assert.Equal(t, "Quiet Study", tag.Name)
			}
		}
		assert.True(t, foundVegan)
		assert.True(t, foundQuiet)
	})

	t.Run("GetTag for user1 - existing personal tag", func(t *testing.T) {
		tag, err := testtagsService.GetTag(ctx, userID1, createdTagID1)
		require.NoError(t, err)
		require.NotNil(t, tag)
		assert.Equal(t, "Vegan Options", tag.Name)
	})

	t.Run("GetTag for user1 - tag belonging to user2 (should fail or return not found)", func(t *testing.T) {
		// First, get a tag created by user2
		user2Tags, _ := testtagsService.GetTags(ctx, userID2)
		require.NotEmpty(t, user2Tags, "User2 should have tags for this test part")
		user2TagID := user2Tags[0].ID

		_, err := testtagsService.GetTag(ctx, userID1, user2TagID)
		require.Error(t, err) // Expect an error (not found for this user, or access denied)
		// Assert specific error if your repo/service returns one (e.g., types.ErrNotFound)
		assert.Contains(t, err.Error(), "error fetching user avoid tags")
	})

	t.Run("Update tag for user1", func(t *testing.T) {
		newName := "Excellent Vegan Food"
		newDesc := "Top-tier vegan dishes"
		updateParams := types.UpdatePersonalTagParams{
			Name:        &newName,
			Description: &newDesc,
		}
		err := testtagsService.Update(ctx, userID1, createdTagID1, updateParams)
		require.NoError(t, err)

		updatedTag, err := testtagsService.GetTag(ctx, userID1, createdTagID1)
		require.NoError(t, err)
		assert.Equal(t, newName, updatedTag.Name)
		assert.Equal(t, newDesc, updatedTag.Description.String)
	})

	t.Run("Update tag not belonging to user (should fail)", func(t *testing.T) {
		user2Tags, _ := testtagsService.GetTags(ctx, userID2)
		require.NotEmpty(t, user2Tags)
		user2TagID := user2Tags[0].ID
		newName := "Attempted Update"
		updateParams := types.UpdatePersonalTagParams{Name: &newName}

		err := testtagsService.Update(ctx, userID1, user2TagID, updateParams)
		require.Error(t, err) // Should fail as userID1 doesn't own user2TagID
		// Check for specific error like "not found for user" or "permission denied"
		assert.Contains(t, err.Error(), "error updating user avoid tag")
	})

	t.Run("Delete tag for user1", func(t *testing.T) {
		err := testtagsService.DeleteTag(ctx, userID1, createdTagID1)
		require.NoError(t, err)

		_, err = testtagsService.GetTag(ctx, userID1, createdTagID1)
		require.Error(t, err) // Should be not found now
	})

	t.Run("Delete tag not belonging to user (should fail or do nothing)", func(t *testing.T) {
		user2Tags, _ := testtagsService.GetTags(ctx, userID2)
		require.NotEmpty(t, user2Tags)
		user2TagID := user2Tags[0].ID

		err := testtagsService.DeleteTag(ctx, userID1, user2TagID)
		// Behavior depends on repo: error if not found for user, or no error if it just means "ensure it's not linked"
		// Assuming repo.Delete checks ownership and errors if not found for that user.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error removing user avoid tag")

		// Verify user2's tag still exists
		tagStillExists, errGet := testtagsService.GetTag(ctx, userID2, user2TagID)
		require.NoError(t, errGet)
		require.NotNil(t, tagStillExists)
	})
}

// To run integration tests:
// TEST_DATABASE_URL="postgres://user:password@localhost:5432/test_db_name?sslmode=disable" go test -v ./internal/tags -tags=integration -count=1
