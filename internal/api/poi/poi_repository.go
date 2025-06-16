package poi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ Repository = (*RepositoryImpl)(nil)

type Repository interface {
	SavePoi(ctx context.Context, poi types.POIDetail, cityID uuid.UUID) (uuid.UUID, error)
	FindPoiByNameAndCity(ctx context.Context, name string, cityID uuid.UUID) (*types.POIDetail, error)
	//GetPOIsByNamesAndCitySortedByDistance(ctx context.Context, names []string, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetail, error)
	GetPOIsByCityAndDistance(ctx context.Context, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetailedInfo, error)
	AddPoiToFavourites(ctx context.Context, userID, poiID uuid.UUID) (uuid.UUID, error)
	RemovePoiFromFavourites(ctx context.Context, poiID uuid.UUID, userID uuid.UUID) error
	GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetail, error)
	GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetail, error)

	// POI details
	FindPOIDetails(ctx context.Context, cityID uuid.UUID, lat, lon float64, tolerance float64) (*types.POIDetailedInfo, error)
	SavePOIDetails(ctx context.Context, poi types.POIDetailedInfo, cityID uuid.UUID) (uuid.UUID, error)
	SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetail, error)

	// Vector similarity search methods
	FindSimilarPOIs(ctx context.Context, queryEmbedding []float32, limit int) ([]types.POIDetail, error)
	FindSimilarPOIsByCity(ctx context.Context, queryEmbedding []float32, cityID uuid.UUID, limit int) ([]types.POIDetail, error)
	SearchPOIsHybrid(ctx context.Context, filter types.POIFilter, queryEmbedding []float32, semanticWeight float64) ([]types.POIDetail, error)
	UpdatePOIEmbedding(ctx context.Context, poiID uuid.UUID, embedding []float32) error
	GetPOIsWithoutEmbeddings(ctx context.Context, limit int) ([]types.POIDetail, error)

	// Hotels
	FindHotelDetails(ctx context.Context, cityID uuid.UUID, lat, lon, tolerance float64) ([]types.HotelDetailedInfo, error)
	SaveHotelDetails(ctx context.Context, hotel types.HotelDetailedInfo, cityID uuid.UUID) (uuid.UUID, error)
	GetHotelByID(ctx context.Context, hotelID uuid.UUID) (*types.HotelDetailedInfo, error)
	// Restaurants
	FindRestaurantDetails(ctx context.Context, cityID uuid.UUID, lat, lon, tolerance float64, preferences *types.RestaurantUserPreferences) ([]types.RestaurantDetailedInfo, error)
	SaveRestaurantDetails(ctx context.Context, restaurant types.RestaurantDetailedInfo, cityID uuid.UUID) (uuid.UUID, error)
	GetRestaurantByID(ctx context.Context, restaurantID uuid.UUID) (*types.RestaurantDetailedInfo, error)
	// GetPOIsByCityIDAndCategory(ctx context.Context, cityID uuid.UUID, category string) ([]types.POIDetail, error)
	// GetPOIsByCityIDAndCategories(ctx context.Context, cityID uuid.UUID, categories []string) ([]types.POIDetail, error)
	// GetPOIsByCityIDAndName(ctx context.Context, cityID uuid.UUID, name string) ([]types.POIDetail, error)
	// GetPOIsByCityIDAndNames(ctx context.Context, cityID uuid.UUID, names []string) ([]types.POIDetail, error)
	// GetPOIsByCityIDAndNameSortedByDistance(ctx context.Context, cityID uuid.UUID, name string, userLocation types.UserLocation) ([]types.POIDetail, error)
	// GetPOIsByCityIDAndNamesSortedByDistance(ctx context.Context, cityID uuid.UUID, names []string, userLocation types.UserLocation) ([]types.POIDetail, error)

	//AddPersonalizedPOItoFavourites(ctx context.Context, poiID uuid.UUID, userID uuid.UUID) (uuid.UUID, error)

	GetItinerary(ctx context.Context, userID, itineraryID uuid.UUID) (*types.UserSavedItinerary, error)
	GetItineraries(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]types.UserSavedItinerary, int, error)
	UpdateItinerary(ctx context.Context, userID uuid.UUID, itineraryID uuid.UUID, updates types.UpdateItineraryRequest) (*types.UserSavedItinerary, error)
	SaveItinerary(ctx context.Context, userID, cityID uuid.UUID) (uuid.UUID, error)
	SaveItineraryPOIs(ctx context.Context, itineraryID uuid.UUID, pois []types.POIDetail) error
	SavePOItoPointsOfInterest(ctx context.Context, poi types.POIDetail, cityID uuid.UUID) (uuid.UUID, error)
	CityExists(ctx context.Context, cityID uuid.UUID) (bool, error)
}

type RepositoryImpl struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewRepository(pgxpool *pgxpool.Pool, logger *slog.Logger) *RepositoryImpl {
	return &RepositoryImpl{
		logger: logger,
		pgpool: pgxpool,
	}
}

func (r *RepositoryImpl) SavePoi(ctx context.Context, poi types.POIDetail, cityID uuid.UUID) (uuid.UUID, error) {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Validate coordinates
	if poi.Latitude < -90 || poi.Latitude > 90 || poi.Longitude < -180 || poi.Longitude > 180 {
		return uuid.Nil, fmt.Errorf("invalid coordinates: lat=%f, lon=%f", poi.Latitude, poi.Longitude)
	}
	if poi.Name == "" {
		return uuid.Nil, fmt.Errorf("POI name is required")
	}

	query := `
        INSERT INTO points_of_interest (
            name, description, location, city_id, poi_type, source, ai_summary
        ) VALUES (
            $1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326), $5, $6, $7, $8
        ) RETURNING id
    `
	var id uuid.UUID
	if err = tx.QueryRow(ctx, query,
		poi.Name, poi.DescriptionPOI, poi.Longitude, poi.Latitude, cityID,
		poi.Category, "loci_ai", poi.DescriptionPOI,
	).Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return uuid.Nil, nil
		}
		return uuid.Nil, fmt.Errorf("failed to insert POI: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	// Log the successful insertion
	r.logger.Info("POI saved successfully", slog.String("name", poi.Name), slog.String("id", id.String()))

	return id, nil
}

func (r *RepositoryImpl) FindPoiByNameAndCity(ctx context.Context, name string, cityID uuid.UUID) (*types.POIDetail, error) {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
        SELECT name, description, ST_Y(location) as lat, ST_X(location) as lon, poi_type
        FROM points_of_interest
        WHERE name = $1 AND city_id = $2
    `
	var poi types.POIDetail
	if err = tx.QueryRow(ctx, query, name, cityID).Scan(
		&poi.Name, &poi.DescriptionPOI, &poi.Latitude, &poi.Longitude, &poi.Category,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find POI: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	// Log the successful retrieval
	r.logger.Info("POI found successfully",
		slog.String("name", poi.Name),
		slog.Float64("latitude", poi.Latitude),
		slog.Float64("longitude", poi.Longitude),
		slog.String("cityID", cityID.String()))

	return &poi, nil
}

// func (r *RepositoryImpl) GetPOIsByNamesAndCitySortedByDistance(ctx context.Context, names []string, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetail, error) {
// 	// Construct the user's location as a PostGIS POINT
// 	userPoint := fmt.Sprintf("SRID=4326;POINT(%f %f)", userLocation.UserLon, userLocation.UserLat)

// 	// SQL query using ST_Distance for sorting
// 	query := `
//         SELECT
//             id,
//             name,
//             ST_X(location::geometry) AS longitude,
//             ST_Y(location::geometry) AS latitude,
//             poi_type AS category,
//             ai_summary AS description_poi,
//             ST_Distance(location::geography, ST_GeomFromText($1, 4326)::geography) AS distance
//         FROM points_of_interest
//         WHERE name = ANY($2) AND city_id = $3
//         ORDER BY distance ASC
//     `

// 	rows, err := r.pgpool.Query(ctx, query, userPoint, names, cityID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to query POIs: %w", err)
// 	}
// 	defer rows.Close()

// 	var pois []types.POIDetail
// 	for rows.Next() {
// 		var poi types.POIDetail
// 		err := rows.Scan(&poi.ID, &poi.Name, &poi.Longitude,
// 			&poi.Latitude, &poi.Category, &poi.DescriptionPOI, &poi.Distance)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to scan POI row: %w", err)
// 		}
// 		pois = append(pois, poi)
// 	}

// 	if err = rows.Err(); err != nil {
// 		return nil, fmt.Errorf("error iterating POI rows: %w", err)
// 	}

// 	return pois, nil
// }

func (r *RepositoryImpl) GetPOIsByCityAndDistance(ctx context.Context, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetailedInfo, error) {
	userPoint := fmt.Sprintf("SRID=4326;POINT(%f %f)", userLocation.UserLon, userLocation.UserLat)
	query := `
        SELECT 
            id, name, 
            ST_X(location::geometry) AS longitude, 
            ST_Y(location::geometry) AS latitude, 
            poi_type AS category, 
            ai_summary AS description_poi,
            ST_Distance(location::geography, ST_GeomFromText($1, 4326)::geography) AS distance
        FROM points_of_interest
        WHERE city_id = $2 AND ST_DWithin(location::geography, ST_GeomFromText($1, 4326)::geography, $3 * 1000)
        ORDER BY distance ASC
    `
	rows, err := r.pgpool.Query(ctx, query, userPoint, cityID, userLocation.SearchRadiusKm)
	if err != nil {
		return nil, fmt.Errorf("failed to query POIs: %w", err)
	}
	defer rows.Close()

	var pois []types.POIDetailedInfo
	for rows.Next() {
		var poi types.POIDetailedInfo
		err := rows.Scan(&poi.ID, &poi.Name, &poi.Longitude,
			&poi.Latitude, &poi.Category, &poi.DescriptionPOI, &poi.Distance)
		if err != nil {
			return nil, fmt.Errorf("failed to scan POI row: %w", err)
		}
		pois = append(pois, poi)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating POI rows: %w", err)
	}

	return pois, nil
}

func (r *RepositoryImpl) AddPoiToFavourites(ctx context.Context, userID, poiID uuid.UUID) (uuid.UUID, error) {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	query := `
		INSERT INTO user_favorite_pois (poi_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (poi_id, user_id) DO NOTHING
		RETURNING id
	`
	var id uuid.UUID
	if err = tx.QueryRow(ctx, query, poiID, userID).Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return uuid.Nil, nil // No new row inserted
		}
		return uuid.Nil, fmt.Errorf("failed to insert favourite POI: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	// Log the successful insertion
	r.logger.Info("Favourite POI added successfully", slog.String("poiID", poiID.String()), slog.String("userID", userID.String()), slog.String("favouriteID", id.String()))
	return id, nil
}

func (r *RepositoryImpl) RemovePoiFromFavourites(ctx context.Context, poiID uuid.UUID, userID uuid.UUID) error {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		DELETE FROM user_favorite_pois
		WHERE poi_id = $1 AND user_id = $2
	`
	result, err := tx.Exec(ctx, query, poiID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete favourite POI: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("no favourite POI found for deletion")
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	r.logger.Info("Favourite POI removed successfully", slog.String("poiID", poiID.String()), slog.String("userID", userID.String()))
	return nil
}

func (r *RepositoryImpl) GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetail, error) {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	query := `
		SELECT
			p.id, p.name, ST_X(p.location) AS longitude, ST_Y(p.location) AS latitude,
			p.poi_type AS category, p.ai_summary AS description_poi
		FROM points_of_interest p
		INNER JOIN user_favorite_pois uf ON p.id = uf.poi_id
		WHERE uf.user_id = $1
	`
	rows, err := tx.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query favourite POIs: %w", err)
	}
	defer rows.Close()
	var pois []types.POIDetail
	for rows.Next() {
		var poi types.POIDetail
		err := rows.Scan(&poi.ID, &poi.Name, &poi.Longitude, &poi.Latitude, &poi.Category, &poi.DescriptionPOI)
		if err != nil {
			return nil, fmt.Errorf("failed to scan favourite POI row: %w", err)
		}
		pois = append(pois, poi)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating favourite POI rows: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	r.logger.Info("Favourite POIs retrieved successfully", slog.String("userID", userID.String()), slog.Int("count", len(pois)))
	return pois, nil
}

func (r *RepositoryImpl) GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetail, error) {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, name, description, ST_X(location) AS longitude, ST_Y(location) AS latitude, poi_type
		FROM points_of_interest
		WHERE city_id = $1
	`
	rows, err := tx.Query(ctx, query, cityID)
	if err != nil {
		return nil, fmt.Errorf("failed to query POIs by city ID: %w", err)
	}
	defer rows.Close()

	var pois []types.POIDetail
	for rows.Next() {
		var poi types.POIDetail
		err := rows.Scan(&poi.ID, &poi.Name, &poi.DescriptionPOI, &poi.Longitude, &poi.Latitude, &poi.Category)
		if err != nil {
			return nil, fmt.Errorf("failed to scan POI row: %w", err)
		}
		pois = append(pois, poi)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating POI rows: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("POIs retrieved successfully by city ID", slog.String("cityID", cityID.String()), slog.Int("count", len(pois)))
	return pois, nil
}

func (r *RepositoryImpl) FindPOIDetails(ctx context.Context, cityID uuid.UUID, lat, lon float64, tolerance float64) (*types.POIDetailedInfo, error) {
	ctx, span := otel.Tracer("Repository").Start(ctx, "FindPOIDetails", trace.WithAttributes(
		attribute.String("city.id", cityID.String()),
		attribute.Float64("latitude", lat),
		attribute.Float64("longitude", lon),
	))
	defer span.End()

	query := `
        SELECT 
            id, name, description, latitude, longitude, address, website, phone_number,
            opening_hours, price_range, category, tags, images, rating, llm_interaction_id
        FROM poi_details
        WHERE city_id = $1
        AND ST_DWithin(
            location::geography,
            ST_SetSRID(ST_MakePoint($2, $3)::geography, 4326),
            $4
        )
        LIMIT 1
    `
	row := r.pgpool.QueryRow(ctx, query, cityID, lon, lat, tolerance)

	var poi types.POIDetailedInfo
	var llmInteractionID uuid.NullUUID
	err := row.Scan(
		&poi.ID, &poi.Name, &poi.Description, &poi.Latitude, &poi.Longitude,
		&poi.Address, &poi.Website, &poi.PhoneNumber, &poi.OpeningHours,
		&poi.PriceRange, &poi.Category, &poi.Tags, &poi.Images, &poi.Rating,
		&llmInteractionID,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			span.SetStatus(codes.Ok, "No POI found")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to query POI details")
		return nil, fmt.Errorf("failed to query poi_details: %w", err)
	}

	if llmInteractionID.Valid {
		poi.LlmInteractionID = llmInteractionID.UUID
	}
	span.SetStatus(codes.Ok, "POI details found")
	return &poi, nil
}

func (r *RepositoryImpl) SavePOIDetails(ctx context.Context, poi types.POIDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	ctx, span := otel.Tracer("Repository").Start(ctx, "SavePOIDetails", trace.WithAttributes(
		attribute.String("city.id", cityID.String()),
		attribute.String("poi.name", poi.Name),
	))
	defer span.End()

	query := `
        INSERT INTO poi_details (
            id, city_id, name, description, latitude, longitude, location,
            address, website, phone_number, opening_hours, price_range, category,
            tags, images, rating, llm_interaction_id
        ) VALUES (
            $1, $2, $3, $4, $5, $6, ST_SetSRID(ST_MakePoint($7, $8), 4326),
            $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
        )
        RETURNING id
    `
	var id uuid.UUID
	err := r.pgpool.QueryRow(ctx, query,
		uuid.New(), cityID, poi.Name, poi.Description, poi.Latitude, poi.Longitude,
		poi.Longitude, poi.Latitude, // lon, lat for ST_MakePoint
		poi.Address, poi.Website, poi.PhoneNumber, poi.OpeningHours,
		poi.PriceRange, poi.Category, poi.Tags, poi.Images, poi.Rating,
		uuid.NullUUID{UUID: poi.LlmInteractionID, Valid: poi.LlmInteractionID != uuid.Nil},
	).Scan(&id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save POI details")
		return uuid.Nil, fmt.Errorf("failed to save poi_details: %w", err)
	}

	span.SetAttributes(attribute.String("poi.id", id.String()))
	span.SetStatus(codes.Ok, "POI details saved successfully")
	return id, nil
}

func (r *RepositoryImpl) FindHotelDetails(ctx context.Context, cityID uuid.UUID, lat, lon, tolerance float64) ([]types.HotelDetailedInfo, error) {
	ctx, span := otel.Tracer("HotelRepository").Start(ctx, "FindHotelDetails", trace.WithAttributes(
		attribute.String("city.id", cityID.String()),
		attribute.Float64("latitude", lat),
		attribute.Float64("longitude", lon),
	))
	defer span.End()

	query := `
        SELECT 
            id, name, description, latitude, longitude, address, website, phone_number,
            opening_hours, price_range, category, tags, images, rating, llm_interaction_id
        FROM hotel_details
        WHERE city_id = $1
        AND ST_DWithin(
            location::geography,
            ST_SetSRID(ST_MakePoint($2, $3)::geography, 4326),
            $4
        )
    `
	rows, err := r.pgpool.Query(ctx, query, cityID, lon, lat, tolerance)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to query hotel details")
		return nil, fmt.Errorf("failed to query hotel_details: %w", err)
	}
	defer rows.Close()

	var hotels []types.HotelDetailedInfo
	for rows.Next() {
		var hotel types.HotelDetailedInfo
		var llmInteractionID uuid.NullUUID
		var website, phoneNumber, openingHours, priceRange *string
		err := rows.Scan(
			&hotel.ID, &hotel.Name, &hotel.Description, &hotel.Latitude, &hotel.Longitude,
			&hotel.Address, &website, &phoneNumber, &openingHours, &priceRange,
			&hotel.Category, &hotel.Tags, &hotel.Images, &hotel.Rating, &llmInteractionID,
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to scan hotel details")
			return nil, fmt.Errorf("failed to scan hotel_details: %w", err)
		}
		hotel.Website = website
		hotel.PhoneNumber = phoneNumber
		hotel.OpeningHours = openingHours
		hotel.PriceRange = priceRange
		if llmInteractionID.Valid {
			hotel.LlmInteractionID = llmInteractionID.UUID
		}
		hotels = append(hotels, hotel)
	}
	if rows.Err() != nil {
		span.RecordError(rows.Err())
		span.SetStatus(codes.Error, "Failed to iterate hotel details")
		return nil, fmt.Errorf("failed to iterate hotel_details: %w", rows.Err())
	}

	span.SetStatus(codes.Ok, "Hotel details found")
	return hotels, nil
}

func (r *RepositoryImpl) SaveHotelDetails(ctx context.Context, hotel types.HotelDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	ctx, span := otel.Tracer("HotelRepository").Start(ctx, "SaveHotelDetails", trace.WithAttributes(
		attribute.String("city.id", cityID.String()),
		attribute.String("hotel.name", hotel.Name),
	))
	defer span.End()

	var openingHours *string
	if hotel.OpeningHours != nil && *hotel.OpeningHours != "" {
		// Verify it's valid JSON
		if json.Valid([]byte(*hotel.OpeningHours)) {
			openingHours = hotel.OpeningHours
		} else {
			// Log warning and set to nil if invalid
			r.logger.WarnContext(ctx, "Invalid JSON for opening_hours, setting to NULL", slog.String("value", *hotel.OpeningHours))
			openingHours = nil
		}
	}

	query := `
        INSERT INTO hotel_details (
            id, city_id, name, description, latitude, longitude, location,
            address, website, phone_number, opening_hours, price_range, category,
            tags, images, rating, llm_interaction_id
        ) VALUES (
            $1, $2, $3, $4, $5, $6, ST_SetSRID(ST_MakePoint($7, $8), 4326),
            $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
        )
        RETURNING id
    `
	var id uuid.UUID
	err := r.pgpool.QueryRow(ctx, query,
		uuid.New(), cityID, hotel.Name, hotel.Description, hotel.Latitude, hotel.Longitude,
		hotel.Longitude, hotel.Latitude, // lon, lat for ST_MakePoint
		hotel.Address, hotel.Website, hotel.PhoneNumber, openingHours,
		hotel.PriceRange, hotel.Category, hotel.Tags, hotel.Images, hotel.Rating,
		uuid.NullUUID{UUID: hotel.LlmInteractionID, Valid: hotel.LlmInteractionID != uuid.Nil},
	).Scan(&id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save hotel details")
		return uuid.Nil, fmt.Errorf("failed to save hotel_details: %w", err)
	}

	span.SetAttributes(attribute.String("hotel.id", id.String()))
	span.SetStatus(codes.Ok, "Hotel details saved successfully")
	return id, nil
}

func (r *RepositoryImpl) GetHotelByID(ctx context.Context, hotelID uuid.UUID) (*types.HotelDetailedInfo, error) {
	ctx, span := otel.Tracer("HotelRepository").Start(ctx, "GetHotelByID", trace.WithAttributes(
		attribute.String("hotel.id", hotelID.String()),
	))
	defer span.End()

	query := `
		SELECT 
			id, name, description, latitude, longitude, address, website, phone_number,
			opening_hours, price_range, category, tags, images, rating, llm_interaction_id
		FROM hotel_details
		WHERE id = $1
	`
	row := r.pgpool.QueryRow(ctx, query, hotelID)

	var hotel types.HotelDetailedInfo
	var llmInteractionID uuid.NullUUID
	err := row.Scan(
		&hotel.ID, &hotel.Name, &hotel.Description, &hotel.Latitude, &hotel.Longitude,
		&hotel.Address, &hotel.Website, &hotel.PhoneNumber, &hotel.OpeningHours,
		&hotel.PriceRange, &hotel.Category, &hotel.Tags, &hotel.Images, &hotel.Rating,
		&llmInteractionID,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			span.SetStatus(codes.Ok, "No hotel found")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to query hotel details by ID")
		return nil, fmt.Errorf("failed to query hotel_details by ID: %w", err)
	}

	if llmInteractionID.Valid {
		hotel.LlmInteractionID = llmInteractionID.UUID
	}
	span.SetStatus(codes.Ok, "Hotel details found by ID")
	return &hotel, nil
}

func (r *RepositoryImpl) FindRestaurantDetails(ctx context.Context, cityID uuid.UUID, lat, lon, tolerance float64, preferences *types.RestaurantUserPreferences) ([]types.RestaurantDetailedInfo, error) {
	ctx, span := otel.Tracer("RestaurantRepository").Start(ctx, "FindRestaurantDetails")
	defer span.End()

	query := `
        SELECT 
            id, name, description, latitude, longitude, address, website, phone_number,
            opening_hours, price_level, category, tags, images, rating, cuisine_type, llm_interaction_id
        FROM restaurant_details
        WHERE city_id = $1
        AND ST_DWithin(
            location::geography,
            ST_SetSRID(ST_MakePoint($2, $3)::geography, 4326),
            $4
        )
    `
	args := []interface{}{cityID, lon, lat, tolerance}
	if preferences != nil {
		if preferences.PreferredCuisine != "" {
			query += ` AND cuisine_type = $5`
			args = append(args, preferences.PreferredCuisine)
		}
		if preferences.PreferredPriceRange != "" {
			query += fmt.Sprintf(` AND price_level = $%d`, len(args)+1)
			args = append(args, preferences.PreferredPriceRange)
		}
	}

	rows, err := r.pgpool.Query(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to query restaurants")
		return nil, fmt.Errorf("failed to query restaurant_details: %w", err)
	}
	defer rows.Close()

	var restaurants []types.RestaurantDetailedInfo
	for rows.Next() {
		var r types.RestaurantDetailedInfo
		var llmID uuid.NullUUID
		err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Latitude, &r.Longitude, &r.Address,
			&r.Website, &r.PhoneNumber, &r.OpeningHours, &r.PriceLevel, &r.Category,
			&r.Tags, &r.Images, &r.Rating, &r.CuisineType, &llmID)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan restaurant: %w", err)
		}
		if llmID.Valid {
			r.LlmInteractionID = llmID.UUID
		}
		restaurants = append(restaurants, r)
	}
	span.SetStatus(codes.Ok, "Restaurants found")
	return restaurants, nil
}

func (r *RepositoryImpl) SaveRestaurantDetails(ctx context.Context, restaurant types.RestaurantDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	ctx, span := otel.Tracer("RestaurantRepository").Start(ctx, "SaveRestaurantDetails", trace.WithAttributes(
		attribute.String("restaurant.name", restaurant.Name),
		attribute.String("city.id", cityID.String()),
	))
	defer span.End()

	// Normalize opening_hours
	var openingHoursJSON sql.NullString // Use sql.NullString for JSONB to handle NULL correctly
	if restaurant.OpeningHours != nil && *restaurant.OpeningHours != "" {
		if json.Valid([]byte(*restaurant.OpeningHours)) {
			openingHoursJSON.String = *restaurant.OpeningHours
			openingHoursJSON.Valid = true
		} else {
			r.logger.WarnContext(ctx, "Invalid JSON for opening_hours, setting to NULL",
				slog.String("value", *restaurant.OpeningHours),
				slog.String("restaurant_name", restaurant.Name))
			// openingHoursJSON remains invalid, which inserts NULL
		}
	}

	// Normalize price_level and cuisine_type (using sql.NullString is safer for text fields that can be null)
	var priceLevel sql.NullString
	if restaurant.PriceLevel != nil && *restaurant.PriceLevel != "" {
		priceLevel.String = *restaurant.PriceLevel
		priceLevel.Valid = true
	}

	var cuisineType sql.NullString
	if restaurant.CuisineType != nil && *restaurant.CuisineType != "" {
		cuisineType.String = *restaurant.CuisineType
		cuisineType.Valid = true
	}

	// Handle nullable text fields from restaurant struct
	var address sql.NullString
	if restaurant.Address != nil {
		address.String = *restaurant.Address
		address.Valid = true
	}
	var website sql.NullString
	if restaurant.Website != nil {
		website.String = *restaurant.Website
		website.Valid = true
	}
	var phoneNumber sql.NullString
	if restaurant.PhoneNumber != nil {
		phoneNumber.String = *restaurant.PhoneNumber
		phoneNumber.Valid = true
	}
	var category sql.NullString
	if restaurant.Category != "" { // Assuming Category is not a pointer in the struct
		category.String = restaurant.Category
		category.Valid = true
	}

	query := `
        INSERT INTO restaurant_details (
            id, city_id, name, description, latitude, longitude, location,
            address, website, phone_number, opening_hours, price_level, category,
            cuisine_type, tags, images, rating, llm_interaction_id
        ) VALUES (
            $1, $2, $3, $4, $5, $6, ST_SetSRID(ST_MakePoint($7, $8), 4326),
            $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19 -- Added $19
        ) RETURNING id
    `
	var id uuid.UUID
	err := r.pgpool.QueryRow(ctx, query,
		restaurant.ID,
		cityID,                      // $2: city_id
		restaurant.Name,             // $3: name
		restaurant.Description,      // $4: description
		restaurant.Latitude,         // $5: latitude
		restaurant.Longitude,        // $6: longitude
		restaurant.Longitude,        // $7: location (longitude for ST_MakePoint)
		restaurant.Latitude,         // $8: location (latitude for ST_MakePoint)
		address,                     // $9: address (sql.NullString)
		website,                     // $10: website (sql.NullString)
		phoneNumber,                 // $11: phone_number (sql.NullString)
		openingHoursJSON,            // $12: opening_hours (sql.NullString representing JSON)
		priceLevel,                  // $13: price_level (sql.NullString)
		category,                    // $14: category (sql.NullString)
		cuisineType,                 // $15: cuisine_type (sql.NullString)
		restaurant.Tags,             // $16: tags (TEXT[])
		restaurant.Images,           // $17: images (TEXT[])
		restaurant.Rating,           // $18: rating (DOUBLE PRECISION)
		restaurant.LlmInteractionID, // $19: llm_interaction_id (UUID)
	).Scan(&id)

	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to save restaurant details",
			slog.Any("error", err),
			slog.String("restaurant_name", restaurant.Name),
			slog.String("city_id", cityID.String()))
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB INSERT failed")
		return uuid.Nil, fmt.Errorf("failed to save restaurant_details: %w", err)
	}

	// If the `id` scanned back is different from `restaurant.ID` (which it will be if you used uuid.New() in the query's $1)
	// and you need the database-generated ID, then `id` is what you want.
	// If you want to ensure the ID from the service layer (which was already in restaurant.ID) is used and is the PK,
	// then you should pass restaurant.ID as $1. My correction above assumes you pass restaurant.ID as $1.

	span.SetAttributes(attribute.String("db.restaurant.id", id.String())) // Log the ID returned by the DB
	span.SetStatus(codes.Ok, "Restaurant saved")
	return id, nil
}

func (r *RepositoryImpl) GetRestaurantByID(ctx context.Context, restaurantID uuid.UUID) (*types.RestaurantDetailedInfo, error) {
	ctx, span := otel.Tracer("RestaurantRepository").Start(ctx, "GetRestaurantByID")
	defer span.End()

	query := `
        SELECT 
            id, name, description, latitude, longitude, address, website, phone_number,
            opening_hours, price_level, category, tags, images, rating, cuisine_type, llm_interaction_id
        FROM restaurant_details
        WHERE id = $1
    `
	var restaurant types.RestaurantDetailedInfo
	var llmID uuid.NullUUID
	err := r.pgpool.QueryRow(ctx, query, restaurantID).Scan(&restaurant.ID, &restaurant.Name,
		&restaurant.Description, &restaurant.Latitude,
		&restaurant.Longitude, &restaurant.Address,
		&restaurant.Website, &restaurant.PhoneNumber,
		&restaurant.OpeningHours, &restaurant.PriceLevel,
		&restaurant.Category, &restaurant.Tags,
		&restaurant.Images, &restaurant.Rating,
		&restaurant.CuisineType, &llmID)
	if err != nil {
		if err == pgx.ErrNoRows {
			span.SetStatus(codes.Ok, "Restaurant not found")
			return nil, nil
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get restaurant: %w", err)
	}
	if llmID.Valid {
		restaurant.LlmInteractionID = llmID.UUID
	}
	span.SetStatus(codes.Ok, "Restaurant found")
	return &restaurant, nil
}

func (r *RepositoryImpl) SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("Repository").Start(ctx, "SearchPOIs", trace.WithAttributes(
		attribute.Float64("location.latitude", filter.Location.Latitude),
		attribute.Float64("location.longitude", filter.Location.Longitude),
		attribute.Float64("radius", filter.Radius),
		attribute.String("category", filter.Category),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "SearchPOIs"))

	// Base query using PostGIS for geospatial filtering
	query := `
        SELECT 
            id, 
            name, 
            description, 
            ST_X(location::geometry) AS longitude, 
            ST_Y(location::geometry) AS latitude, 
            category,
            ST_Distance(
                location,
                ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
            ) AS distance_meters
        FROM points_of_interest
        WHERE ST_DWithin(
            location,
            ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
            $3
        )
    `
	args := []interface{}{
		filter.Location.Longitude, // $1
		filter.Location.Latitude,  // $2
		filter.Radius * 1000,      // $3 (convert km to meters for ST_DWithin)
	}

	// Add category filter if provided
	if filter.Category != "" {
		query += ` AND category = $4`
		args = append(args, filter.Category) // $4
	}

	// Order by distance
	query += ` ORDER BY distance_meters ASC`

	l.DebugContext(ctx, "Executing POI search query", slog.String("query", query), slog.Any("args", args))

	// Execute query
	rows, err := r.pgpool.Query(ctx, query, args...)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query POIs", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Database query failed")
		return nil, fmt.Errorf("failed to search points_of_interest: %w", err)
	}
	defer rows.Close()

	// Collect results
	var pois []types.POIDetail
	for rows.Next() {
		var poi types.POIDetail
		var distanceMeters float64
		var description sql.NullString // Handle NULL description

		err := rows.Scan(
			&poi.ID,
			&poi.Name,
			&description,
			&poi.Longitude,
			&poi.Latitude,
			&poi.Category,
			&distanceMeters,
		)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan POI row", slog.Any("error", err))
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan POI row: %w", err)
		}

		// Set description if valid
		if description.Valid {
			poi.DescriptionPOI = description.String
		}

		pois = append(pois, poi)
	}

	// Check for errors during row iteration
	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating POI rows", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("error iterating POI rows: %w", err)
	}

	// Log and set span status
	if len(pois) == 0 {
		l.InfoContext(ctx, "No POIs found")
		span.SetStatus(codes.Ok, "No POIs found")
	} else {
		l.InfoContext(ctx, "POIs found", slog.Int("count", len(pois)))
		span.SetStatus(codes.Ok, "POIs found")
	}

	return pois, nil
}

func (r *RepositoryImpl) GetItinerary(ctx context.Context, userID, itineraryID uuid.UUID) (*types.UserSavedItinerary, error) {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "GetItinerary", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "user_saved_itineraries"),
		attribute.String("user.id", userID.String()),
		attribute.String("itinerary.id", itineraryID.String()),
	))
	defer span.End()

	query := `
		SELECT 
			id, user_id, source_llm_interaction_id, primary_city_id, title, description,
			markdown_content, tags, estimated_duration_days, estimated_cost_level, is_public
		FROM user_saved_itineraries
		WHERE id = $1 AND user_id = $2
	`
	row := r.pgpool.QueryRow(ctx, query, itineraryID, userID)

	var itinerary types.UserSavedItinerary
	if err := row.Scan(
		&itinerary.ID,
		&itinerary.UserID,
		&itinerary.SourceLlmInteractionID,
		&itinerary.PrimaryCityID,
		&itinerary.Title,
		&itinerary.Description,
		&itinerary.MarkdownContent,
		&itinerary.Tags,
		&itinerary.EstimatedDurationDays,
		&itinerary.EstimatedCostLevel,
		&itinerary.IsPublic,
	); err != nil {
		if err == pgx.ErrNoRows {
			err = fmt.Errorf("no itinerary found with ID %s for user %s", itineraryID, userID)
			span.RecordError(err)
			return nil, err
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to scan user_saved_itineraries row: %w", err)
	}

	return &itinerary, nil
}

func (r *RepositoryImpl) GetItineraries(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]types.UserSavedItinerary, int, error) {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "GetItineraries", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "user_saved_itineraries"),
		attribute.String("user.id", userID.String()),
		attribute.Int("page", page),
		attribute.Int("page_size", pageSize),
	))
	defer span.End()

	offset := (page - 1) * pageSize
	query := `
		SELECT 
			id, user_id, source_llm_interaction_id, primary_city_id, title, description,
			markdown_content, tags, estimated_duration_days, estimated_cost_level, is_public
		FROM user_saved_itineraries
		WHERE user_id = $1
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pgpool.Query(ctx, query, userID, pageSize, offset)
	if err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("failed to query user_saved_itineraries: %w", err)
	}
	defer rows.Close()

	var itineraries []types.UserSavedItinerary
	for rows.Next() {
		var itinerary types.UserSavedItinerary
		if err := rows.Scan(
			&itinerary.ID,
			&itinerary.UserID,
			&itinerary.SourceLlmInteractionID,
			&itinerary.PrimaryCityID,
			&itinerary.Title,
			&itinerary.Description,
			&itinerary.MarkdownContent,
			&itinerary.Tags,
			&itinerary.EstimatedDurationDays,
			&itinerary.EstimatedCostLevel,
			&itinerary.IsPublic,
		); err != nil {
			if err == pgx.ErrNoRows {
				continue // No more rows to scan
			}
			return nil, 0, fmt.Errorf("failed to scan user_saved_itineraries row: %w", err)
		}
		itineraries = append(itineraries, itinerary)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating user_saved_itineraries rows: %w", err)
	}

	countQuery := `
		SELECT COUNT(*) FROM user_saved_itineraries WHERE user_id = $1
	`
	var totalRecords int
	if err := r.pgpool.QueryRow(ctx, countQuery, userID).Scan(&totalRecords); err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("failed to count user_saved_itineraries: %w", err)
	}
	span.SetAttributes(
		attribute.Int("total_records", totalRecords),
		attribute.Int("itineraries.count", len(itineraries)),
	)
	span.SetStatus(codes.Ok, "Itineraries retrieved successfully")
	return itineraries, totalRecords, nil
}

func (r *RepositoryImpl) UpdateItinerary(ctx context.Context, userID uuid.UUID, itineraryID uuid.UUID, updates types.UpdateItineraryRequest) (*types.UserSavedItinerary, error) {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "UpdateItinerary", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "user_saved_itineraries"),
		attribute.String("user.id", userID.String()),
		attribute.String("itinerary.id", itineraryID.String()),
	))
	defer span.End()

	setClauses := []string{}
	args := []interface{}{}
	argCount := 1 // Start arg counter for $1, $2, ...

	if updates.Title != nil {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", argCount))
		args = append(args, *updates.Title)
		argCount++
		span.SetAttributes(attribute.Bool("update.title", true))
	}
	if updates.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argCount))
		if *updates.Description == "" {
			args = append(args, sql.NullString{Valid: false})
		} else {
			args = append(args, sql.NullString{String: *updates.Description, Valid: true})
		}
		argCount++
		span.SetAttributes(attribute.Bool("update.description", true))
	}
	if updates.Tags != nil {
		setClauses = append(setClauses, fmt.Sprintf("tags = $%d", argCount))
		args = append(args, updates.Tags)
		argCount++
		span.SetAttributes(attribute.Bool("update.tags", true))
	}
	if updates.EstimatedDurationDays != nil {
		setClauses = append(setClauses, fmt.Sprintf("estimated_duration_days = $%d", argCount))
		args = append(args, sql.NullInt32{Int32: *updates.EstimatedDurationDays, Valid: true})
		argCount++
		span.SetAttributes(attribute.Bool("update.duration", true))
	}
	if updates.EstimatedCostLevel != nil {
		setClauses = append(setClauses, fmt.Sprintf("estimated_cost_level = $%d", argCount))
		args = append(args, sql.NullInt32{Int32: *updates.EstimatedCostLevel, Valid: true})
		argCount++
		span.SetAttributes(attribute.Bool("update.cost", true))
	}
	if updates.IsPublic != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_public = $%d", argCount))
		args = append(args, *updates.IsPublic)
		argCount++
		span.SetAttributes(attribute.Bool("update.is_public", true))
	}
	if updates.MarkdownContent != nil {
		setClauses = append(setClauses, fmt.Sprintf("markdown_content = $%d", argCount))
		args = append(args, *updates.MarkdownContent)
		argCount++
		span.SetAttributes(attribute.Bool("update.markdown", true))
	}

	if len(setClauses) == 0 {
		span.AddEvent("No fields provided for update.")
		return nil, fmt.Errorf("no fields to update for itinerary %s", itineraryID)
	}

	// Always update the updated_at timestamp
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argCount))
	args = append(args, time.Now())
	argCount++

	// Store the current argCount for the WHERE clause
	whereIDPlaceholder := argCount
	args = append(args, itineraryID)
	argCount++
	userIDPlaceholder := argCount
	args = append(args, userID)

	query := fmt.Sprintf(`
        UPDATE user_saved_itineraries
        SET %s
        WHERE id = $%d AND user_id = $%d
        RETURNING id, user_id, source_llm_interaction_id, primary_city_id, title, description,
                  markdown_content, tags, estimated_duration_days, estimated_cost_level, is_public,
                  created_at, updated_at
    `, strings.Join(setClauses, ", "), whereIDPlaceholder, userIDPlaceholder)

	r.logger.DebugContext(ctx, "Executing UpdateItinerary query", slog.String("query", query), slog.Any("args_count", len(args)))

	var updatedItinerary types.UserSavedItinerary
	err := r.pgpool.QueryRow(ctx, query, args...).Scan(
		&updatedItinerary.ID,
		&updatedItinerary.UserID,
		&updatedItinerary.SourceLlmInteractionID,
		&updatedItinerary.PrimaryCityID,
		&updatedItinerary.Title,
		&updatedItinerary.Description,
		&updatedItinerary.MarkdownContent,
		&updatedItinerary.Tags,
		&updatedItinerary.EstimatedDurationDays,
		&updatedItinerary.EstimatedCostLevel,
		&updatedItinerary.IsPublic,
		&updatedItinerary.CreatedAt,
		&updatedItinerary.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			notFoundErr := fmt.Errorf("itinerary with ID %s not found for user %s or does not exist", itineraryID, userID)
			span.RecordError(notFoundErr)
			span.SetStatus(codes.Error, "Itinerary not found or not owned by user")
			return nil, notFoundErr
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "DB UPDATE failed")
		r.logger.ErrorContext(ctx, "Failed to update itinerary", slog.Any("error", err))
		return nil, fmt.Errorf("failed to update user_saved_itineraries: %w", err)
	}

	span.SetStatus(codes.Ok, "Itinerary updated successfully")
	return &updatedItinerary, nil
}

func (r *RepositoryImpl) SaveItinerary(ctx context.Context, userID, cityID uuid.UUID) (uuid.UUID, error) {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "SaveItinerary")
	defer span.End()

	query := `
        INSERT INTO itineraries (user_id, city_id, created_at, updated_at)
        VALUES ($1, $2, NOW(), NOW())
        RETURNING id
    `
	var itineraryID uuid.UUID
	err := r.pgpool.QueryRow(ctx, query, userID, cityID).Scan(&itineraryID)
	if err != nil {
		span.RecordError(err)
		return uuid.Nil, fmt.Errorf("failed to save itinerary: %w", err)
	}
	span.SetAttributes(attribute.String("itinerary.id", itineraryID.String()))
	return itineraryID, nil
}

func (r *RepositoryImpl) SavePOItoPointsOfInterest(ctx context.Context, poi types.POIDetail, cityID uuid.UUID) (uuid.UUID, error) {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "SavePOItoPointsOfInterest")
	defer span.End()

	// Check if POI exists
	queryCheck := `
        SELECT id FROM points_of_interest
        WHERE name = $1 AND city_id = $2
    `
	var poiID uuid.UUID
	err := r.pgpool.QueryRow(ctx, queryCheck, poi.Name, cityID).Scan(&poiID)
	if err == nil {
		return poiID, nil // POI already exists
	}
	if err != pgx.ErrNoRows {
		span.RecordError(err)
		return uuid.Nil, fmt.Errorf("failed to check POI existence: %w", err)
	}

	// Insert new POI
	queryInsert := `
        INSERT INTO points_of_interest (id, city_id, name, latitude, longitude, category)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id
    `
	poiID = uuid.New()
	err = r.pgpool.QueryRow(ctx, queryInsert, poiID, cityID, poi.Name, poi.Latitude, poi.Longitude, poi.Category).Scan(&poiID)
	if err != nil {
		span.RecordError(err)
		return uuid.Nil, fmt.Errorf("failed to save POI to points_of_interest: %w", err)
	}
	span.SetAttributes(attribute.String("poi.id", poiID.String()))
	return poiID, nil
}

func (r *RepositoryImpl) SaveItineraryPOIs(ctx context.Context, itineraryID uuid.UUID, pois []types.POIDetail) error {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "SaveItineraryPOIs")
	defer span.End()

	batch := &pgx.Batch{}
	query := `
        INSERT INTO itinerary_pois (itinerary_id, poi_id, order_index, ai_description)
        VALUES ($1, $2, $3, $4)
    `
	for i, poi := range pois {
		poiID, err := r.SavePOItoPointsOfInterest(ctx, poi, poi.CityID) // Assume CityID is added to POIDetail or passed separately
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to ensure POI in points_of_interest: %w", err)
		}
		aiDescription := poi.DescriptionPOI // Use description from llm_suggested_pois
		batch.Queue(query, itineraryID, poiID, i, aiDescription)
	}

	br := r.pgpool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(pois); i++ {
		_, err := br.Exec()
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to save itinerary POI at index %d: %w", i, err)
		}
	}
	span.SetAttributes(attribute.Int("pois.count", len(pois)))
	return nil
}

func (r *RepositoryImpl) CityExists(ctx context.Context, cityID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM cities WHERE id = $1)`
	var exists bool
	err := r.pgpool.QueryRow(ctx, query, cityID).Scan(&exists) // Assuming r.db is your database connection
	if err != nil {
		return false, fmt.Errorf("failed to check city existence: %w", err)
	}
	return exists, nil
}

// FindSimilarPOIs finds POIs similar to the provided query embedding using cosine similarity
func (r *RepositoryImpl) FindSimilarPOIs(ctx context.Context, queryEmbedding []float32, limit int) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("Repository").Start(ctx, "FindSimilarPOIs", trace.WithAttributes(
		attribute.Int("embedding.dimension", len(queryEmbedding)),
		attribute.Int("limit", limit),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "FindSimilarPOIs"))

	// Convert []float32 to pgvector format string
	embeddingStr := fmt.Sprintf("[%v]", strings.Join(func() []string {
		strs := make([]string, len(queryEmbedding))
		for i, v := range queryEmbedding {
			strs[i] = fmt.Sprintf("%f", v)
		}
		return strs
	}(), ","))

	query := `
        SELECT 
            id, 
            name, 
            description, 
            ST_X(location::geometry) AS longitude, 
            ST_Y(location::geometry) AS latitude, 
            poi_type AS category,
            1 - (embedding <=> $1::vector) AS similarity_score
        FROM points_of_interest
        WHERE embedding IS NOT NULL
        ORDER BY embedding <=> $1::vector
        LIMIT $2
    `

	l.DebugContext(ctx, "Executing similarity search query",
		slog.String("query", query),
		slog.Int("embedding_dim", len(queryEmbedding)),
		slog.Int("limit", limit))

	rows, err := r.pgpool.Query(ctx, query, embeddingStr, limit)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query similar POIs", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Database query failed")
		return nil, fmt.Errorf("failed to search similar POIs: %w", err)
	}
	defer rows.Close()

	var pois []types.POIDetail
	for rows.Next() {
		var poi types.POIDetail
		var similarityScore float64
		var description sql.NullString

		err := rows.Scan(
			&poi.ID,
			&poi.Name,
			&description,
			&poi.Longitude,
			&poi.Latitude,
			&poi.Category,
			&similarityScore,
		)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan similar POI row", slog.Any("error", err))
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan similar POI row: %w", err)
		}

		if description.Valid {
			poi.DescriptionPOI = description.String
		}

		// Store similarity score in distance field for now (could add dedicated field)
		poi.Distance = similarityScore

		pois = append(pois, poi)
	}

	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating similar POI rows", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("error iterating similar POI rows: %w", err)
	}

	l.InfoContext(ctx, "Similar POIs found", slog.Int("count", len(pois)))
	span.SetAttributes(attribute.Int("results.count", len(pois)))
	span.SetStatus(codes.Ok, "Similar POIs found")

	return pois, nil
}

// FindSimilarPOIsByCity finds POIs similar to the provided query embedding within a specific city
func (r *RepositoryImpl) FindSimilarPOIsByCity(ctx context.Context, queryEmbedding []float32, cityID uuid.UUID, limit int) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("Repository").Start(ctx, "FindSimilarPOIsByCity", trace.WithAttributes(
		attribute.String("city.id", cityID.String()),
		attribute.Int("embedding.dimension", len(queryEmbedding)),
		attribute.Int("limit", limit),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "FindSimilarPOIsByCity"))

	// Convert []float32 to pgvector format string
	embeddingStr := fmt.Sprintf("[%v]", strings.Join(func() []string {
		strs := make([]string, len(queryEmbedding))
		for i, v := range queryEmbedding {
			strs[i] = fmt.Sprintf("%f", v)
		}
		return strs
	}(), ","))

	query := `
        SELECT 
            id, 
            name, 
            description, 
            ST_X(location::geometry) AS longitude, 
            ST_Y(location::geometry) AS latitude, 
            poi_type AS category,
            1 - (embedding <=> $1::vector) AS similarity_score
        FROM points_of_interest
        WHERE embedding IS NOT NULL AND city_id = $2
        ORDER BY embedding <=> $1::vector
        LIMIT $3
    `

	l.DebugContext(ctx, "Executing city-specific similarity search",
		slog.String("city_id", cityID.String()),
		slog.Int("embedding_dim", len(queryEmbedding)),
		slog.Int("limit", limit))

	rows, err := r.pgpool.Query(ctx, query, embeddingStr, cityID, limit)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query similar POIs by city", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Database query failed")
		return nil, fmt.Errorf("failed to search similar POIs by city: %w", err)
	}
	defer rows.Close()

	var pois []types.POIDetail
	for rows.Next() {
		var poi types.POIDetail
		var similarityScore float64
		var description sql.NullString

		err := rows.Scan(
			&poi.ID,
			&poi.Name,
			&description,
			&poi.Longitude,
			&poi.Latitude,
			&poi.Category,
			&similarityScore,
		)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan similar POI row", slog.Any("error", err))
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan similar POI row: %w", err)
		}

		if description.Valid {
			poi.DescriptionPOI = description.String
		}

		poi.Distance = similarityScore
		poi.CityID = cityID

		pois = append(pois, poi)
	}

	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating similar POI rows", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("error iterating similar POI rows: %w", err)
	}

	l.InfoContext(ctx, "Similar POIs by city found",
		slog.String("city_id", cityID.String()),
		slog.Int("count", len(pois)))
	span.SetAttributes(
		attribute.String("city.id", cityID.String()),
		attribute.Int("results.count", len(pois)),
	)
	span.SetStatus(codes.Ok, "Similar POIs by city found")

	return pois, nil
}

// SearchPOIsHybrid combines spatial filtering with semantic similarity search
func (r *RepositoryImpl) SearchPOIsHybrid(ctx context.Context, filter types.POIFilter, queryEmbedding []float32, semanticWeight float64) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("Repository").Start(ctx, "SearchPOIsHybrid", trace.WithAttributes(
		attribute.Float64("location.latitude", filter.Location.Latitude),
		attribute.Float64("location.longitude", filter.Location.Longitude),
		attribute.Float64("radius", filter.Radius),
		attribute.String("category", filter.Category),
		attribute.Float64("semantic.weight", semanticWeight),
		attribute.Int("embedding.dimension", len(queryEmbedding)),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "SearchPOIsHybrid"))

	// Convert []float32 to pgvector format string
	embeddingStr := fmt.Sprintf("[%v]", strings.Join(func() []string {
		strs := make([]string, len(queryEmbedding))
		for i, v := range queryEmbedding {
			strs[i] = fmt.Sprintf("%f", v)
		}
		return strs
	}(), ","))

	// Hybrid search combining spatial distance and semantic similarity
	query := `
        SELECT 
            id, 
            name, 
            description, 
            ST_X(location::geometry) AS longitude, 
            ST_Y(location::geometry) AS latitude, 
            poi_type AS category,
            ST_Distance(
                location,
                ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
            ) AS distance_meters,
            CASE 
                WHEN embedding IS NOT NULL THEN 1 - (embedding <=> $6::vector)
                ELSE 0 
            END AS similarity_score,
            -- Hybrid score: weighted combination of spatial proximity and semantic similarity
            CASE 
                WHEN embedding IS NOT NULL THEN
                    (1 - $5) * (1 / (1 + ST_Distance(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) / 1000)) +
                    $5 * (1 - (embedding <=> $6::vector))
                ELSE
                    (1 - $5) * (1 / (1 + ST_Distance(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) / 1000))
            END AS hybrid_score
        FROM points_of_interest
        WHERE ST_DWithin(
            location,
            ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
            $3
        )
    `

	args := []interface{}{
		filter.Location.Longitude, // $1
		filter.Location.Latitude,  // $2
		filter.Radius * 1000,      // $3 (convert km to meters)
	}

	// Add category filter if provided
	argIndex := 4
	if filter.Category != "" {
		query += fmt.Sprintf(` AND poi_type = $%d`, argIndex)
		args = append(args, filter.Category)
		argIndex++
	}

	// Add semantic weight and embedding (adjust indexes based on whether category was added)
	args = append(args, semanticWeight) // semantic weight
	args = append(args, embeddingStr)   // embedding

	// Order by hybrid score (descending)
	query += ` ORDER BY hybrid_score DESC`

	l.DebugContext(ctx, "Executing hybrid search query",
		slog.String("query", query),
		slog.Any("args_count", len(args)),
		slog.Float64("semantic_weight", semanticWeight))

	rows, err := r.pgpool.Query(ctx, query, args...)
	if err != nil {
		l.ErrorContext(ctx, "Failed to execute hybrid search", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Database query failed")
		return nil, fmt.Errorf("failed to execute hybrid POI search: %w", err)
	}
	defer rows.Close()

	var pois []types.POIDetail
	for rows.Next() {
		var poi types.POIDetail
		var distanceMeters, similarityScore, hybridScore float64
		var description sql.NullString

		err := rows.Scan(
			&poi.ID,
			&poi.Name,
			&description,
			&poi.Longitude,
			&poi.Latitude,
			&poi.Category,
			&distanceMeters,
			&similarityScore,
			&hybridScore,
		)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan hybrid search POI row", slog.Any("error", err))
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan hybrid search POI row: %w", err)
		}

		if description.Valid {
			poi.DescriptionPOI = description.String
		}

		// Store the actual distance in meters converted to km
		poi.Distance = distanceMeters / 1000

		pois = append(pois, poi)
	}

	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating hybrid search POI rows", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("error iterating hybrid search POI rows: %w", err)
	}

	l.InfoContext(ctx, "Hybrid search POIs found",
		slog.Int("count", len(pois)),
		slog.Float64("semantic_weight", semanticWeight))
	span.SetAttributes(
		attribute.Int("results.count", len(pois)),
		attribute.Float64("semantic.weight", semanticWeight),
	)
	span.SetStatus(codes.Ok, "Hybrid search completed")

	return pois, nil
}

// UpdatePOIEmbedding updates the embedding vector for a specific POI
func (r *RepositoryImpl) UpdatePOIEmbedding(ctx context.Context, poiID uuid.UUID, embedding []float32) error {
	ctx, span := otel.Tracer("Repository").Start(ctx, "UpdatePOIEmbedding", trace.WithAttributes(
		attribute.String("poi.id", poiID.String()),
		attribute.Int("embedding.dimension", len(embedding)),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "UpdatePOIEmbedding"))

	// Convert []float32 to pgvector format string
	embeddingStr := fmt.Sprintf("[%v]", strings.Join(func() []string {
		strs := make([]string, len(embedding))
		for i, v := range embedding {
			strs[i] = fmt.Sprintf("%f", v)
		}
		return strs
	}(), ","))

	query := `
        UPDATE points_of_interest 
        SET embedding = $1::vector, embedding_generated_at = NOW()
        WHERE id = $2
    `

	result, err := r.pgpool.Exec(ctx, query, embeddingStr, poiID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update POI embedding",
			slog.Any("error", err),
			slog.String("poi_id", poiID.String()))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Database update failed")
		return fmt.Errorf("failed to update POI embedding: %w", err)
	}

	if result.RowsAffected() == 0 {
		err := fmt.Errorf("no POI found with ID %s", poiID.String())
		l.WarnContext(ctx, "No POI found for embedding update", slog.String("poi_id", poiID.String()))
		span.RecordError(err)
		span.SetStatus(codes.Error, "POI not found")
		return err
	}

	l.InfoContext(ctx, "POI embedding updated successfully",
		slog.String("poi_id", poiID.String()),
		slog.Int("embedding_dimension", len(embedding)))
	span.SetAttributes(
		attribute.String("poi.id", poiID.String()),
		attribute.Int("embedding.dimension", len(embedding)),
	)
	span.SetStatus(codes.Ok, "POI embedding updated")

	return nil
}

// GetPOIsWithoutEmbeddings retrieves POIs that don't have embeddings generated yet
func (r *RepositoryImpl) GetPOIsWithoutEmbeddings(ctx context.Context, limit int) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("Repository").Start(ctx, "GetPOIsWithoutEmbeddings", trace.WithAttributes(
		attribute.Int("limit", limit),
	))
	defer span.End()

	l := r.logger.With(slog.String("method", "GetPOIsWithoutEmbeddings"))

	query := `
        SELECT 
            id, 
            name, 
            description, 
            ST_X(location::geometry) AS longitude, 
            ST_Y(location::geometry) AS latitude, 
            poi_type AS category,
            city_id
        FROM points_of_interest
        WHERE embedding IS NULL
        ORDER BY created_at ASC
        LIMIT $1
    `

	rows, err := r.pgpool.Query(ctx, query, limit)
	if err != nil {
		l.ErrorContext(ctx, "Failed to query POIs without embeddings", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Database query failed")
		return nil, fmt.Errorf("failed to query POIs without embeddings: %w", err)
	}
	defer rows.Close()

	var pois []types.POIDetail
	for rows.Next() {
		var poi types.POIDetail
		var description sql.NullString

		err := rows.Scan(
			&poi.ID,
			&poi.Name,
			&description,
			&poi.Longitude,
			&poi.Latitude,
			&poi.Category,
			&poi.CityID,
		)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan POI without embedding row", slog.Any("error", err))
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan POI without embedding row: %w", err)
		}

		if description.Valid {
			poi.DescriptionPOI = description.String
		}

		pois = append(pois, poi)
	}

	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating POI without embedding rows", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("error iterating POI without embedding rows: %w", err)
	}

	l.InfoContext(ctx, "POIs without embeddings found", slog.Int("count", len(pois)))
	span.SetAttributes(attribute.Int("results.count", len(pois)))
	span.SetStatus(codes.Ok, "POIs without embeddings retrieved")

	return pois, nil
}
