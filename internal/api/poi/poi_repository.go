package poi

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ POIRepository = (*PostgresPOIRepository)(nil)

type POIRepository interface {
	SavePoi(ctx context.Context, poi types.POIDetail, cityID uuid.UUID) (uuid.UUID, error)
	FindPoiByNameAndCity(ctx context.Context, name string, cityID uuid.UUID) (*types.POIDetail, error)
	GetPOIsByNamesAndCitySortedByDistance(ctx context.Context, names []string, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetail, error)
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

func (r *PostgresPOIRepository) GetPOIsByNamesAndCitySortedByDistance(ctx context.Context, names []string, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetail, error) {
	// Construct the user's location as a PostGIS POINT
	userPoint := fmt.Sprintf("SRID=4326;POINT(%f %f)", userLocation.UserLon, userLocation.UserLat)

	// SQL query using ST_Distance for sorting
	query := `
        SELECT 
            id, 
            name, 
            ST_X(location::geometry) AS longitude, 
            ST_Y(location::geometry) AS latitude, 
            poi_type AS category, 
            ai_summary AS description_poi,
            ST_Distance(location::geography, ST_GeomFromText($1, 4326)::geography) AS distance
        FROM points_of_interest
        WHERE name = ANY($2) AND city_id = $3
        ORDER BY distance ASC
    `

	rows, err := r.pgpool.Query(ctx, query, userPoint, names, cityID)
	if err != nil {
		return nil, fmt.Errorf("failed to query POIs: %w", err)
	}
	defer rows.Close()

	var pois []types.POIDetail
	for rows.Next() {
		var poi types.POIDetail
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
