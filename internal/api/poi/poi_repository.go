package poi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ POIRepository = (*PostgresPOIRepository)(nil)

type POIRepository interface {
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

}

type PostgresPOIRepository struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewPOIRepository(pgxpool *pgxpool.Pool, logger *slog.Logger) *PostgresPOIRepository {
	return &PostgresPOIRepository{
		logger: logger,
		pgpool: pgxpool,
	}
}

func (r *PostgresPOIRepository) SavePoi(ctx context.Context, poi types.POIDetail, cityID uuid.UUID) (uuid.UUID, error) {
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
		poi.Category, "wanderwise_ai", poi.DescriptionPOI,
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

func (r *PostgresPOIRepository) FindPoiByNameAndCity(ctx context.Context, name string, cityID uuid.UUID) (*types.POIDetail, error) {
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
	r.logger.Info("POI found successfully", slog.String("name", poi.Name), slog.String("cityID", cityID.String()))

	return &poi, nil
}

// func (r *PostgresPOIRepository) GetPOIsByNamesAndCitySortedByDistance(ctx context.Context, names []string, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetail, error) {
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

func (r *PostgresPOIRepository) GetPOIsByCityAndDistance(ctx context.Context, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetailedInfo, error) {
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

func (r *PostgresPOIRepository) AddPoiToFavourites(ctx context.Context, userID, poiID uuid.UUID) (uuid.UUID, error) {
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

func (r *PostgresPOIRepository) RemovePoiFromFavourites(ctx context.Context, poiID uuid.UUID, userID uuid.UUID) error {
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

func (r *PostgresPOIRepository) GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetail, error) {
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

func (r *PostgresPOIRepository) GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetail, error) {
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

func (r *PostgresPOIRepository) FindPOIDetails(ctx context.Context, cityID uuid.UUID, lat, lon float64, tolerance float64) (*types.POIDetailedInfo, error) {
	ctx, span := otel.Tracer("POIRepository").Start(ctx, "FindPOIDetails", trace.WithAttributes(
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

func (r *PostgresPOIRepository) SavePOIDetails(ctx context.Context, poi types.POIDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	ctx, span := otel.Tracer("POIRepository").Start(ctx, "SavePOIDetails", trace.WithAttributes(
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

func (r *PostgresPOIRepository) FindHotelDetails(ctx context.Context, cityID uuid.UUID, lat, lon, tolerance float64) ([]types.HotelDetailedInfo, error) {
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

func (r *PostgresPOIRepository) SaveHotelDetails(ctx context.Context, hotel types.HotelDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
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

func (r *PostgresPOIRepository) GetHotelByID(ctx context.Context, hotelID uuid.UUID) (*types.HotelDetailedInfo, error) {
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

func (r *PostgresPOIRepository) FindRestaurantDetails(ctx context.Context, cityID uuid.UUID, lat, lon, tolerance float64, preferences *types.RestaurantUserPreferences) ([]types.RestaurantDetailedInfo, error) {
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

func (r *PostgresPOIRepository) SaveRestaurantDetails(ctx context.Context, restaurant types.RestaurantDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
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

func (r *PostgresPOIRepository) GetRestaurantByID(ctx context.Context, restaurantID uuid.UUID) (*types.RestaurantDetailedInfo, error) {
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

func (r *PostgresPOIRepository) SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("POIRepository").Start(ctx, "SearchPOIs", trace.WithAttributes(
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
