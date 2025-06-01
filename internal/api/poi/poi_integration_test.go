//go:build integration

package poi

import (
	"context"
	"database/sql"
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
	// Your database migration tool/library if you use one programmatically
)

var testDB *pgxpool.Pool
var testService POIService // Use the interface

func TestMain(m *testing.M) {
	// Load .env.test or similar for test database credentials
	if err := godotenv.Load("../../../.env.test"); err != nil { // Adjust path to your .env.test
		log.Println("Warning: .env.test file not found, relying on system environment variables for integration tests.")
	}

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		log.Fatal("TEST_DATABASE_URL environment variable is not set for integration tests")
	}

	var err error
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("Unable to parse TEST_DATABASE_URL: %v\n", err)
	}

	// Optional: Set connection pool parameters for testing
	config.MaxConns = 5
	config.MinConns = 1
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute
	config.ConnConfig.ConnectTimeout = 5 * time.Second

	testDB, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
	}
	defer testDB.Close() // Ensure pool is closed after tests run

	// Ping the database to ensure connection is live
	if err := testDB.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping test database: %v\n", err)
	}

	// Optional: Run Migrations
	// log.Println("Running database migrations for integration tests...")
	// runMigrations(testDB) // Implement this function if you have programmatic migrations

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	// Use your actual PostgresPOIRepository implementation
	realRepo := NewPostgresPOIRepository(testDB, logger) // Assuming this constructor exists
	testService = NewPOIServiceImpl(realRepo, logger)

	// Run tests
	exitCode := m.Run()

	// Optional: Clean up database after tests
	// log.Println("Cleaning up test database...")
	// cleanupTestDatabase(testDB) // Implement this

	os.Exit(exitCode)
}

// Helper function to clear tables (use with caution, specific to your schema)
func clearFavouritesTable(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), "DELETE FROM user_favourite_pois") // Adjust table name
	require.NoError(t, err, "Failed to clear user_favourite_pois table")
}
func clearPoisTable(t *testing.T) {
	t.Helper()
	// Be careful with cascading deletes or foreign key constraints
	// May need to delete from user_favourite_pois first if it references pois
	_, err := testDB.Exec(context.Background(), "DELETE FROM pois") // Adjust table name for POIs
	require.NoError(t, err, "Failed to clear pois table")
}

// You'll need a way to insert a dummy POI and User for testing favourites
func insertTestUser(t *testing.T, id uuid.UUID, username string) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), "INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING",
		id, username, username+"@example.com", "somehash")
	require.NoError(t, err)
}
func insertTestCity(t *testing.T, id uuid.UUID, name string) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), "INSERT INTO cities (id, name, country) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING",
		id, name, "Testland")
	require.NoError(t, err)
}
func insertTestPOI(t *testing.T, id uuid.UUID, name string, cityID uuid.UUID) types.POIDetail {
	t.Helper()
	poi := types.POIDetail{ID: id, Name: name, CityID: cityID, Latitude: 1.0, Longitude: 1.0, Category: "Test"} // Add other required fields
	_, err := testDB.Exec(context.Background(),
		"INSERT INTO pois (id, city_id, name, description_poi, latitude, longitude, location, category) VALUES ($1, $2, $3, $4, $5, $6, ST_SetSRID(ST_MakePoint($7, $8), 4326), $9) ON CONFLICT (id) DO NOTHING",
		poi.ID, poi.CityID, poi.Name, sql.NullString{String: "Desc", Valid: true}, poi.Latitude, poi.Longitude, poi.Longitude, poi.Latitude, poi.Category)
	require.NoError(t, err)
	return poi
}

func TestPOIServiceImpl_Favourites_Integration(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	poiID1 := uuid.New()
	cityID := uuid.New()

	insertTestUser(t, userID, "fav_user")
	insertTestCity(t, cityID, "Fav City")
	insertTestPOI(t, poiID1, "Favourite Test POI 1", cityID)

	t.Run("Add and Get Favourite POI", func(t *testing.T) {
		clearFavouritesTable(t) // Clear before this sub-test

		favID, err := testService.AddPoiToFavourites(ctx, userID, poiID1)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, favID) // Repository should return the ID of the favourite entry

		favourites, err := testService.GetFavouritePOIsByUserID(ctx, userID)
		require.NoError(t, err)
		require.Len(t, favourites, 1)
		assert.Equal(t, poiID1, favourites[0].ID) // Assuming GetFavouritePOIsByUserID returns POIDetail with the original POI's ID
		assert.Equal(t, "Favourite Test POI 1", favourites[0].Name)
	})

	t.Run("Remove Favourite POI", func(t *testing.T) {
		clearFavouritesTable(t)
		_, _ = testService.AddPoiToFavourites(ctx, userID, poiID1) // Add it first

		err := testService.RemovePoiFromFavourites(ctx, poiID1, userID)
		require.NoError(t, err)

		favourites, err := testService.GetFavouritePOIsByUserID(ctx, userID)
		require.NoError(t, err)
		assert.Empty(t, favourites)
	})

	t.Run("Get Favourites for user with no favourites", func(t *testing.T) {
		clearFavouritesTable(t)
		otherUserID := uuid.New()
		insertTestUser(t, otherUserID, "nofav_user")

		favourites, err := testService.GetFavouritePOIsByUserID(ctx, otherUserID)
		require.NoError(t, err)
		assert.Empty(t, favourites)
	})
}

func TestPOIServiceImpl_GetPOIsByCityID_Integration(t *testing.T) {
	ctx := context.Background()
	cityID1 := uuid.New()
	cityID2 := uuid.New() // Another city for isolation

	insertTestCity(t, cityID1, "CityWithPOIs")
	insertTestCity(t, cityID2, "CityWithoutPOIs")

	poi1 := insertTestPOI(t, uuid.New(), "City1 POI A", cityID1)
	poi2 := insertTestPOI(t, uuid.New(), "City1 POI B", cityID1)
	// No POIs for cityID2 initially

	t.Run("Get POIs for city with POIs", func(t *testing.T) {
		pois, err := testService.GetPOIsByCityID(ctx, cityID1)
		require.NoError(t, err)
		require.Len(t, pois, 2)
		// Check if poi1 and poi2 are in the list (order might not be guaranteed)
		foundPoi1 := false
		foundPoi2 := false
		for _, p := range pois {
			if p.ID == poi1.ID {
				foundPoi1 = true
			}
			if p.ID == poi2.ID {
				foundPoi2 = true
			}
		}
		assert.True(t, foundPoi1, "POI1 should be found")
		assert.True(t, foundPoi2, "POI2 should be found")
	})

	t.Run("Get POIs for city with no POIs", func(t *testing.T) {
		pois, err := testService.GetPOIsByCityID(ctx, cityID2)
		require.NoError(t, err)
		assert.Empty(t, pois)
	})
}

func TestPOIServiceImpl_SearchPOIs_Integration(t *testing.T) {
	ctx := context.Background()
	searchCityID := uuid.New()
	insertTestCity(t, searchCityID, "SearchableCity")

	// Clear POIs for this city to ensure clean slate for search tests
	_, err := testDB.Exec(ctx, "DELETE FROM pois WHERE city_id = $1", searchCityID)
	require.NoError(t, err)

	// Seed specific POIs for searching
	poiMuseum := types.POIDetail{ID: uuid.New(), CityID: searchCityID, Name: "Grand Museum", Category: "Museum", Latitude: 10.0, Longitude: 10.0, DescriptionPOI: sql.NullString{String: "Ancient artifacts", Valid: true}}
	poiPark := types.POIDetail{ID: uuid.New(), CityID: searchCityID, Name: "City Park", Category: "Park", Latitude: 10.1, Longitude: 10.1, DescriptionPOI: sql.NullString{String: "Green space", Valid: true}}
	poiCafe := types.POIDetail{ID: uuid.New(), CityID: searchCityID, Name: "Art Cafe", Category: "Cafe", Latitude: 10.05, Longitude: 10.05, DescriptionPOI: sql.NullString{String: "Coffee and art", Valid: true}, Tags: []string{"art", "coffee"}}

	insertPOIForIntegration(t, poiMuseum)
	insertPOIForIntegration(t, poiPark)
	insertPOIForIntegration(t, poiCafe)

	t.Run("Search by category", func(t *testing.T) {
		filter := types.POIFilter{CityID: searchCityID, Category: "Museum"}
		pois, err := testService.SearchPOIs(ctx, filter)
		require.NoError(t, err)
		require.Len(t, pois, 1)
		assert.Equal(t, "Grand Museum", pois[0].Name)
	})

	t.Run("Search by tag", func(t *testing.T) {
		filter := types.POIFilter{CityID: searchCityID, Tags: []string{"art"}}
		pois, err := testService.SearchPOIs(ctx, filter)
		require.NoError(t, err)
		require.Len(t, pois, 1) // Only Art Cafe has "art" tag in this setup
		assert.Equal(t, "Art Cafe", pois[0].Name)
	})

	t.Run("Search by name substring", func(t *testing.T) {
		filter := types.POIFilter{CityID: searchCityID, Name: "grand"} // Case-insensitive search usually
		pois, err := testService.SearchPOIs(ctx, filter)
		require.NoError(t, err)
		require.Len(t, pois, 1)
		assert.Equal(t, "Grand Museum", pois[0].Name)
	})

	t.Run("Search with no results", func(t *testing.T) {
		filter := types.POIFilter{CityID: searchCityID, Category: "Zoo"}
		pois, err := testService.SearchPOIs(ctx, filter)
		require.NoError(t, err)
		assert.Empty(t, pois)
	})
}

// Helper for SearchPOIs_Integration
func insertPOIForIntegration(t *testing.T, poi types.POIDetail) {
	t.Helper()
	_, err := testDB.Exec(context.Background(),
		"INSERT INTO pois (id, city_id, name, description_poi, latitude, longitude, location, category, tags) VALUES ($1, $2, $3, $4, $5, $6, ST_SetSRID(ST_MakePoint($7, $8), 4326), $9, $10) ON CONFLICT (id) DO NOTHING",
		poi.ID, poi.CityID, poi.Name, poi.DescriptionPOI, poi.Latitude, poi.Longitude, poi.Longitude, poi.Latitude, poi.Category, pq.Array(poi.Tags)) // Use pq.Array for []string
	require.NoError(t, err)
}

// To run integration tests:
// TEST_DATABASE_URL="postgres://user:password@localhost:5432/test_db_name?sslmode=disable" go test -v ./internal/api/poi -tags=integration -count=1
