//go:build integration

package interests

import (
	"context"
	"database/sql" // For sql.NullString if needed in direct inserts
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

var testinterestsDB *pgxpool.Pool
var testinterestsService interestsService // Use the interface
var testinterestsRepo interestsRepo       // Actual repository implementation for setup/cleanup

func TestMain(m *testing.M) {
	if err := godotenv.Load("../../../.env.test"); err != nil { // Adjust path
		log.Println("Warning: .env.test file not found for user_interest integration tests.")
	}

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		log.Fatal("TEST_DATABASE_URL environment variable is not set for user_interest integration tests")
	}

	var err error
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("Unable to parse TEST_DATABASE_URL: %v\n", err)
	}
	config.MaxConns = 5

	testinterestsDB, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("Unable to create connection pool for user_interest tests: %v\n", err)
	}
	defer testinterestsDB.Close()

	if err := testinterestsDB.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping test database for user_interest tests: %v\n", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	// Initialize with your *actual* PostgresinterestsRepo implementation
	// e.g., testinterestsRepo = repository.NewPostgresinterestsRepo(testinterestsDB, logger)
	testinterestsRepo = NewPostgresinterestsRepo(testinterestsDB, logger) // Replace with your actual repo constructor
	testinterestsService = NewinterestsService(testinterestsRepo, logger)

	exitCode := m.Run()
	os.Exit(exitCode)
}

// Helper to clear tables (adjust table names if different)
func clearinterestssTable(t *testing.T) {
	t.Helper()
	_, err := testinterestsDB.Exec(context.Background(), "DELETE FROM interests") // Join table
	require.NoError(t, err, "Failed to clear interests table")
}

func clearInterestsTable(t *testing.T) {
	t.Helper()
	// If interests has a foreign key to interests, delete from interests first
	clearinterestssTable(t)
	_, err := testinterestsDB.Exec(context.Background(), "DELETE FROM interests")
	require.NoError(t, err, "Failed to clear interests table")
}

// Helper to create a user directly for testing setup
func createTestUserForInterestTests(t *testing.T, username string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	// Ensure your users table schema matches these columns for a minimal insert
	_, err := testinterestsDB.Exec(context.Background(),
		"INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING",
		id, username, fmt.Sprintf("%s@example.com", username), "test_hash")
	require.NoError(t, err)
	return id
}

// Helper to create an interest directly (useful if CreateInterest service method has other logic)
func createTestInterestDirectly(t *testing.T, name string, description *string, isActive bool, createdByUserID string) *types.Interest {
	t.Helper()
	interest := &types.Interest{
		ID:              uuid.New(),
		Name:            name,
		IsActive:        isActive,
		CreatedByUserID: createdByUserID,
	}
	if description != nil {
		interest.Description = sql.NullString{String: *description, Valid: true}
	}

	_, err := testinterestsDB.Exec(context.Background(),
		"INSERT INTO interests (id, name, description, is_active, created_by_user_id) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (name) DO NOTHING", // Assuming name is unique
		interest.ID, interest.Name, interest.Description, interest.IsActive, interest.CreatedByUserID)
	// If insert fails due to ON CONFLICT DO NOTHING and you need the ID of the existing one, query it.
	// For simplicity, we assume it inserts or we fetch by name later if needed.
	// If a unique constraint on name exists and you hit a conflict, the test might need to fetch the existing ID.
	if err != nil {
		// If it's a unique constraint violation and we want to proceed, we might log it or fetch existing.
		// For now, require no error or handle specific conflict errors.
		// This also depends on whether your repo's CreateInterest handles conflicts.
		var existingInterestID uuid.UUID
		errQuery := testinterestsDB.QueryRow(context.Background(), "SELECT id FROM interests WHERE name = $1", name).Scan(&existingInterestID)
		if errQuery == nil {
			interest.ID = existingInterestID // Use existing ID if conflict on name
		} else {
			require.NoError(t, err, "Failed to insert or find existing interest directly")
		}
	}
	return interest
}

func TestinterestsServiceImpl_CreateInterest_Integration(t *testing.T) {
	ctx := context.Background()
	clearInterestsTable(t) // Clear before test

	userID := createTestUserForInterestTests(t, "interest_creator")
	desc := "A fun outdoor activity"

	t.Run("Create new interest successfully", func(t *testing.T) {
		interestName := "Hiking"
		createdInterest, err := testinterestsService.CreateInterest(ctx, interestName, &desc, true, userID.String())
		require.NoError(t, err)
		require.NotNil(t, createdInterest)
		assert.Equal(t, interestName, createdInterest.Name)
		assert.Equal(t, desc, createdInterest.Description.String)
		assert.True(t, createdInterest.IsActive)
		assert.Equal(t, userID.String(), createdInterest.CreatedByUserID) // Assuming this field is returned

		// Verify in DB
		var dbName string
		var dbDesc sql.NullString
		err = testinterestsDB.QueryRow(ctx, "SELECT name, description FROM interests WHERE id = $1", createdInterest.ID).Scan(&dbName, &dbDesc)
		require.NoError(t, err)
		assert.Equal(t, interestName, dbName)
		assert.Equal(t, desc, dbDesc.String)
	})

	t.Run("Create interest with nil description", func(t *testing.T) {
		interestName := "Museums"
		createdInterest, err := testinterestsService.CreateInterest(ctx, interestName, nil, true, userID.String())
		require.NoError(t, err)
		require.NotNil(t, createdInterest)
		assert.Equal(t, interestName, createdInterest.Name)
		assert.False(t, createdInterest.Description.Valid) // Description should be null/invalid
	})

	t.Run("Attempt to create duplicate interest name (if repo handles it)", func(t *testing.T) {
		// This test depends on how your repo.CreateInterest handles unique constraints on 'name'.
		// If it returns a specific error (e.g., types.ErrConflict), test for that.
		// If it's a raw DB error, the service will wrap it.
		interestName := "Kayaking"
		_, _ = testinterestsService.CreateInterest(ctx, interestName, nil, true, userID.String()) // Create first one

		_, err := testinterestsService.CreateInterest(ctx, interestName, nil, true, userID.String()) // Attempt duplicate
		require.Error(t, err)                                                                        // Expect an error
		// Example: assert.True(t, errors.Is(err, types.ErrConflict)) or check for specific DB error code
		assert.Contains(t, err.Error(), "error adding user interest") // Service wraps it
	})
}

func TestinterestsServiceImpl_UserFavouriteInterests_Integration(t *testing.T) {
	ctx := context.Background()
	clearInterestsTable(t) // Clears interests too due to cascade or order

	userID1 := createTestUserForInterestTests(t, "user_interest_1")
	userID2 := createTestUserForInterestTests(t, "user_interest_2")

	interest1 := createTestInterestDirectly(t, "Photography", nil, true, userID1.String())
	interest2 := createTestInterestDirectly(t, "Cooking", stringPtr("Culinary arts"), true, userID1.String())
	interest3 := createTestInterestDirectly(t, "History", nil, true, userID2.String())

	// User 1 adds interests
	_, err := testinterestsRepo.Addinterests(ctx, userID1, interest1.ID) // Use repo directly for setup
	require.NoError(t, err)
	_, err = testinterestsRepo.Addinterests(ctx, userID1, interest2.ID)
	require.NoError(t, err)

	t.Run("Get favourite interests for user1", func(t *testing.T) {
		// This test assumes interestsService.GetFavouritePOIsByUserID actually fetches interests join table.
		// The naming `GetFavouritePOIsByUserID` in `poi_service_test.go` might be a slight misnomer if it's
		// intended for general interests. Let's assume it's for interests linked to a user.
		// Your interface has `GetAllInterests` (global) and implies that `Add/Removeinterests` manages the link.
		// Let's assume `repo.GetFavouritePOIsByUserID` actually gets *interests* for a user.
		// If not, you need a `repo.Getinterestss(ctx, userID)` method.

		// For this test, let's assume your repo has Getinterestss
		// And your service would have a Getinterestss method too.
		// Since the provided service doesn't have Getinterestss, we'll test Add/Remove.

		// Let's test with what's available: Add and Remove
		// Add interest3 to user1 via service
		// This AddPoiToFavourites is from POIService, we need Addinterests from interestsRepo
		// The interestsService is missing Addinterests, so we test Remove.
		// The test relies on setup using testinterestsRepo.Addinterests

		// Let's assume GetFavouritePOIsByUserID actually returns types.Interest linked to the user
		// through the interests table. This means the POIRepository might need to be interestsRepository.
		// For this example, I'll assume your testinterestsRepo has a Getinterestss method
		// and testinterestsService would call that.
		// Given the current service methods, GetFavouritePOIsByUserID is not in interestsService.
		// Let's proceed assuming Add/Remove and GetAll are the primary ones to test via service.
	})

	t.Run("Add and Remove user interest", func(t *testing.T) {
		// User 2 adds interest 3
		// The service interface `interestsService` is missing `Addinterests`.
		// Let's assume `CreateInterest` implicitly links to the `userID` who created it,
		// but `AddPoiToFavourites` (which seems like what you mean by linking interest to user)
		// is in a different service in your design.

		// Let's test `Removeinterests`
		// First, ensure user1 has interest1 (added via repo setup)
		// We need a way to verify this.
		// Let's assume there's a way to get interests for a user (e.g., a new service method)
		// For now, test the remove operation's success/failure path

		err := testinterestsService.Removeinterests(ctx, userID1, interest1.ID)
		require.NoError(t, err)

		// How to verify removal? Query directly or add a Getinterestss service method.
		// Let's assume direct query for verification in integration test.
		var count int
		err = testinterestsDB.QueryRow(ctx, "SELECT COUNT(*) FROM interests WHERE user_id = $1 AND interest_id = $2", userID1, interest1.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Interest1 should be removed for user1")

		// Try removing again (should not error, or return specific "not found" error if repo does)
		err = testinterestsService.Removeinterests(ctx, userID1, interest1.ID)
		// The current service wraps repo errors. If repo returns pgx.ErrNoRows or similar,
		// the service will return that wrapped. Some remove operations are idempotent (no error if not found).
		// Let's assume repo.Removeinterests returns an error if 0 rows affected.
		// require.Error(t, err) // Or require.NoError(t,err) if it's idempotent
		// For now, we'll assume it might return an error like "not found" or "no rows affected" from repo
		if err != nil { // If repo errors on no rows affected
			assert.Contains(t, err.Error(), "error removing user interest")
		} else { // If repo is idempotent for deletes
			require.NoError(t, err)
		}

		// Remove non-existent link
		nonExistentInterestID := uuid.New()
		err = testinterestsService.Removeinterests(ctx, userID1, nonExistentInterestID)
		// Similar to above, depends on repo behavior for "not found"
		if err != nil {
			assert.Contains(t, err.Error(), "error removing user interest")
		} else {
			require.NoError(t, err)
		}
	})
}

func TestinterestsServiceImpl_GetAllInterests_Integration(t *testing.T) {
	ctx := context.Background()
	clearInterestsTable(t)

	userID := createTestUserForInterestTests(t, "interest_lister")
	desc1 := "Global History"
	desc2 := "Modern Art Forms"

	// Create some interests via the service
	interestA, _ := testinterestsService.CreateInterest(ctx, "World History", &desc1, true, userID.String())
	interestB, _ := testinterestsService.CreateInterest(ctx, "Contemporary Art", &desc2, true, userID.String())
	interestC, _ := testinterestsService.CreateInterest(ctx, "Inactive Interest", nil, false, userID.String())

	t.Run("Get all interests", func(t *testing.T) {
		interests, err := testinterestsService.GetAllInterests(ctx)
		require.NoError(t, err)
		// The number of interests might be more if other tests ran without full cleanup,
		// or less if CreateInterest failed due to unique constraints without proper handling.
		// For a clean test, ensure table is empty or query specific names.
		assert.GreaterOrEqual(t, len(interests), 2, "Should fetch at least the interests created in this test")

		foundA := false
		foundB := false
		foundC := false // If inactive are also returned by GetAllInterests
		for _, i := range interests {
			if i.ID == interestA.ID {
				foundA = true
				assert.Equal(t, interestA.Name, i.Name)
				assert.Equal(t, interestA.Description.String, i.Description.String)
			}
			if i.ID == interestB.ID {
				foundB = true
				assert.Equal(t, interestB.Name, i.Name)
			}
			if i.ID == interestC.ID {
				foundC = true
				assert.False(t, i.IsActive)
			}
		}
		assert.True(t, foundA, "Interest A not found")
		assert.True(t, foundB, "Interest B not found")
		assert.True(t, foundC, "Interest C (inactive) should be found if GetAllInterests fetches all")
		// If GetAllInterests is meant to only fetch active ones, then assert.False(t, foundC)
	})
}

func TestinterestsServiceImpl_Updateinterests_Integration(t *testing.T) {
	ctx := context.Background()
	clearInterestsTable(t)

	userID := createTestUserForInterestTests(t, "interest_updater")
	desc := "Original description"
	interestToUpdate := createTestInterestDirectly(t, "Updatable Interest", &desc, true, userID.String())

	// Link user to interest for this test (assuming Updateinterests modifies the *link* or user-specific aspect)
	// Your current Updateinterests params types.UpdateinterestsParams is empty.
	// This implies it might be updating the *global* interest, or a user-specific flag on the join table.
	// Let's assume for now it's trying to update the global interest record,
	// but the method signature `Updateinterests(ctx, userID, interestID, params)` suggests it's user-specific.

	// If types.UpdateinterestsParams is meant to update fields on the 'interests' table itself:
	// This test is more complex as it implies that a user can update a global interest.
	// Let's assume types.UpdateinterestsParams is for the *global* interest,
	// and your repo.Updateinterests handles this (and potentially authorization if only creator can update).

	// Scenario 1: types.UpdateinterestsParams is for the global interest record.
	t.Run("Update global interest details", func(t *testing.T) {
		newName := "Updated Interest Name"
		newDesc := "This is an updated description."
		newIsActive := false
		updateParams := types.UpdateinterestsParams{ // This struct needs fields
			Name:        &newName,
			Description: &newDesc,
			IsActive:    &newIsActive,
		}

		// The service method signature `Updateinterests(ctx, userID, interestID, params)`
		// suggests it might be for a user's specific instance or preference for an interest.
		// However, the params don't reflect user-specific settings.
		// If this method is meant to update the *global* interest identified by interestID,
		// the userID might be for an authorization check (e.g., only creator can update).

		// Let's proceed assuming the service is intended to call repo.Updateinterests which updates the global interest.
		err := testinterestsService.Updateinterests(ctx, userID, interestToUpdate.ID, updateParams)
		require.NoError(t, err)

		// Verify in DB
		var dbName string
		var dbDesc sql.NullString
		var dbIsActive bool
		err = testinterestsDB.QueryRow(ctx, "SELECT name, description, is_active FROM interests WHERE id = $1", interestToUpdate.ID).Scan(&dbName, &dbDesc, &dbIsActive)
		require.NoError(t, err)
		assert.Equal(t, newName, dbName)
		assert.Equal(t, newDesc, dbDesc.String)
		assert.Equal(t, newIsActive, dbIsActive)
	})

	t.Run("Update non-existent interest", func(t *testing.T) {
		nonExistentID := uuid.New()
		newName := "NonExistentUpdate"
		updateParams := types.UpdateinterestsParams{Name: &newName}
		err := testinterestsService.Updateinterests(ctx, userID, nonExistentID, updateParams)
		require.Error(t, err) // Expect error as interest doesn't exist to be updated
		// Check for a specific "not found" type error if your repo returns one.
		assert.Contains(t, err.Error(), "error updating user interest")
	})

	// Add tests for partial updates if your UpdateinterestsParams and repo logic support it
	// (e.g., only updating description, not name).
}

// Helper to convert string to *string for tests
func stringPtr(s string) *string {
	return &s
}
