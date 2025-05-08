package userSearchProfile

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestUserSearchProfilesRepoLoad performs load tests for the UserSearchProfilesRepo
// These tests require a running database
func TestUserSearchProfilesRepoLoad(t *testing.T) {
	// Skip if not running load tests
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	// Setup
	ctx := context.Background()

	// Connect to the database
	// Note: In a real test, you would use environment variables or a test config
	dbURL := "postgres://postgres:postgres@localhost:5432/go_poi_test?sslmode=disable"
	pgpool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pgpool.Close()

	// Create a repository
	logger := slog.Default()
	repo := NewPostgresUserRepo(pgpool, logger)

	// Create a test user ID
	userID := uuid.New()

	// Create test profiles
	profileIDs, err := createTestProfiles(ctx, repo, userID, 10)
	if err != nil {
		t.Fatalf("Failed to create test profiles: %v", err)
	}

	// Test cases
	t.Run("GetProfiles_Load", func(t *testing.T) {
		// Test parameters
		numConcurrentRequests := 100
		numRequestsPerGoroutine := 10

		// Create a wait group to wait for all goroutines to finish
		var wg sync.WaitGroup
		wg.Add(numConcurrentRequests)

		// Create a channel to collect results
		results := make(chan time.Duration, numConcurrentRequests*numRequestsPerGoroutine)

		// Start goroutines
		for i := 0; i < numConcurrentRequests; i++ {
			go func() {
				defer wg.Done()

				for j := 0; j < numRequestsPerGoroutine; j++ {
					// Measure time to get profiles
					start := time.Now()
					_, err := repo.GetProfiles(ctx, userID)
					duration := time.Since(start)

					// Record result
					if err != nil {
						t.Logf("Error getting profiles: %v", err)
					} else {
						results <- duration
					}
				}
			}()
		}

		// Wait for all goroutines to finish
		wg.Wait()
		close(results)

		// Analyze results
		var totalDuration time.Duration
		var count int
		var minDuration time.Duration = time.Hour
		var maxDuration time.Duration

		for duration := range results {
			totalDuration += duration
			count++

			if duration < minDuration {
				minDuration = duration
			}
			if duration > maxDuration {
				maxDuration = duration
			}
		}

		// Calculate average
		avgDuration := totalDuration / time.Duration(count)

		// Log results
		t.Logf("GetProfiles Load Test Results:")
		t.Logf("  Total Requests: %d", count)
		t.Logf("  Average Duration: %v", avgDuration)
		t.Logf("  Min Duration: %v", minDuration)
		t.Logf("  Max Duration: %v", maxDuration)

		// Assert that the average duration is within acceptable limits
		// This is a subjective threshold and should be adjusted based on your requirements
		require.Less(t, avgDuration, 100*time.Millisecond, "Average duration should be less than 100ms")
	})

	t.Run("GetProfile_Load", func(t *testing.T) {
		// Test parameters
		numConcurrentRequests := 100
		numRequestsPerGoroutine := 10

		// Create a wait group to wait for all goroutines to finish
		var wg sync.WaitGroup
		wg.Add(numConcurrentRequests)

		// Create a channel to collect results
		results := make(chan time.Duration, numConcurrentRequests*numRequestsPerGoroutine)

		// Start goroutines
		for i := 0; i < numConcurrentRequests; i++ {
			go func(profileIndex int) {
				defer wg.Done()

				// Use a different profile for each goroutine to distribute the load
				profileID := profileIDs[profileIndex%len(profileIDs)]
				userID := uuid.New()
				for j := 0; j < numRequestsPerGoroutine; j++ {
					// Measure time to get profile
					start := time.Now()
					_, err := repo.GetProfile(ctx, userID, profileID)
					duration := time.Since(start)

					// Record result
					if err != nil {
						t.Logf("Error getting profile: %v", err)
					} else {
						results <- duration
					}
				}
			}(i)
		}

		// Wait for all goroutines to finish
		wg.Wait()
		close(results)

		// Analyze results
		var totalDuration time.Duration
		var count int
		var minDuration time.Duration = time.Hour
		var maxDuration time.Duration

		for duration := range results {
			totalDuration += duration
			count++

			if duration < minDuration {
				minDuration = duration
			}
			if duration > maxDuration {
				maxDuration = duration
			}
		}

		// Calculate average
		avgDuration := totalDuration / time.Duration(count)

		// Log results
		t.Logf("GetProfile Load Test Results:")
		t.Logf("  Total Requests: %d", count)
		t.Logf("  Average Duration: %v", avgDuration)
		t.Logf("  Min Duration: %v", minDuration)
		t.Logf("  Max Duration: %v", maxDuration)

		// Assert that the average duration is within acceptable limits
		require.Less(t, avgDuration, 50*time.Millisecond, "Average duration should be less than 50ms")
	})

	t.Run("CreateProfile_Load", func(t *testing.T) {
		// Test parameters
		numConcurrentRequests := 50
		numRequestsPerGoroutine := 5

		// Create a wait group to wait for all goroutines to finish
		var wg sync.WaitGroup
		wg.Add(numConcurrentRequests)

		// Create a channel to collect results
		results := make(chan time.Duration, numConcurrentRequests*numRequestsPerGoroutine)

		// Start goroutines
		for i := 0; i < numConcurrentRequests; i++ {
			go func(index int) {
				defer wg.Done()

				for j := 0; j < numRequestsPerGoroutine; j++ {
					// Create unique profile name to avoid conflicts
					profileName := uuid.New().String()

					// Create profile params
					params := types.CreateUserPreferenceProfileParams{
						ProfileName: profileName,
						IsDefault:   boolPtr(false),
					}

					// Measure time to create profile
					start := time.Now()
					_, err := repo.CreateProfile(ctx, userID, params)
					duration := time.Since(start)

					// Record result
					if err != nil {
						t.Logf("Error creating profile: %v", err)
					} else {
						results <- duration
					}
				}
			}(i)
		}

		// Wait for all goroutines to finish
		wg.Wait()
		close(results)

		// Analyze results
		var totalDuration time.Duration
		var count int
		var minDuration time.Duration = time.Hour
		var maxDuration time.Duration

		for duration := range results {
			totalDuration += duration
			count++

			if duration < minDuration {
				minDuration = duration
			}
			if duration > maxDuration {
				maxDuration = duration
			}
		}

		// Calculate average
		avgDuration := totalDuration / time.Duration(count)

		// Log results
		t.Logf("CreateProfile Load Test Results:")
		t.Logf("  Total Requests: %d", count)
		t.Logf("  Average Duration: %v", avgDuration)
		t.Logf("  Min Duration: %v", minDuration)
		t.Logf("  Max Duration: %v", maxDuration)

		// Assert that the average duration is within acceptable limits
		require.Less(t, avgDuration, 200*time.Millisecond, "Average duration should be less than 200ms")
	})
}

// Helper functions

// createTestProfiles creates test profiles in the database
func createTestProfiles(ctx context.Context, repo *PostgresUserSearchProfilesRepo, userID uuid.UUID, count int) ([]uuid.UUID, error) {
	profileIDs := make([]uuid.UUID, 0, count)

	for i := 0; i < count; i++ {
		// Create profile params
		params := types.CreateUserPreferenceProfileParams{
			ProfileName: uuid.New().String(),
			IsDefault:   boolPtr(false),
		}

		// Create profile
		profile, err := repo.CreateProfile(ctx, userID, params)
		if err != nil {
			return nil, err
		}

		profileIDs = append(profileIDs, profile.ID)
	}

	return profileIDs, nil
}

// Helper function for creating a pointer to a bool
func boolPtr(b bool) *bool {
	return &b
}
