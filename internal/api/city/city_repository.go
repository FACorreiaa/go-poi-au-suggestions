package city

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ Repository = (*RepositoryImpl)(nil)

type Repository interface {
	SaveCity(ctx context.Context, city types.CityDetail) (uuid.UUID, error)
	FindCityByNameAndCountry(ctx context.Context, city, country string) (*types.CityDetail, error)
}

type RepositoryImpl struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewCityRepository(pgxpool *pgxpool.Pool, logger *slog.Logger) *RepositoryImpl {
	return &RepositoryImpl{
		logger: logger,
		pgpool: pgxpool,
	}
}

func (r *RepositoryImpl) SaveCity(ctx context.Context, city types.CityDetail) (uuid.UUID, error) {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Construct center_location geometry
	// var centerLocationArg interface{}
	// if city.CenterLatitude != 0 && city.CenterLongitude != 0 {
	// 	if city.CenterLatitude < -90 || city.CenterLatitude > 90 || city.CenterLongitude < -180 || city.CenterLongitude > 180 {
	// 		r.logger.WarnContext(ctx, "Invalid coordinates for city center, saving as NULL",
	// 			slog.String("city", city.Name),
	// 			slog.Float64("lat", city.CenterLatitude),
	// 			slog.Float64("lon", city.CenterLongitude))
	// 		centerLocationArg = nil
	// 	} else {
	// 		centerLocationArg = fmt.Sprintf("ST_SetSRID(ST_MakePoint(%f, %f), 4326)", city.CenterLongitude, city.CenterLatitude)
	// 	}
	// } else {
	// 	centerLocationArg = nil // Save as NULL if no coordinates provided
	// }

	// BoundingBox: For now, we'll insert NULL for bounding_box as AI doesn't provide it easily.
	// You would populate this later if you have a geocoding service or other data source.
	// query := `
	//     INSERT INTO cities (
	//         name, country, state_province, ai_summary, center_location, bounding_box
	//     ) VALUES ($1, $2, $3, $4, ST_GeomFromText($5, 4326), ST_GeomFromText($6, 4326)) RETURNING id
	// `
	// Note: ST_GeomFromText is for WKT strings. For ST_MakePoint, you don't need ST_GeomFromText wrapper if it's directly in SQL.
	// However, pgx typically requires you to pass geometry types in a specific way or as WKT.
	// A common way with pgx for dynamic geometry is to build the SQL string part.
	// Let's adjust to pass coordinates and build the point in SQL.

	query := `
        INSERT INTO cities (
            name, country, state_province, ai_summary, center_location 
            -- bounding_box will use its DEFAULT or be NULL if not specified
        ) VALUES (
            $1, $2, $3, $4, 
            -- Check for 0.0 is a bit naive if 0,0 is a valid location.
            -- It's better if types.CityDetail.CenterLongitude/Latitude are pointers (*float64)
            -- Then you can check for nil. For now, assuming 0.0 implies "not set".
            CASE 
                WHEN ($5::DOUBLE PRECISION IS NOT NULL AND $6::DOUBLE PRECISION IS NOT NULL) 
                     AND ($5::DOUBLE PRECISION != 0.0 OR $6::DOUBLE PRECISION != 0.0) -- Example: only make point if not (0,0) AND both are provided
                     AND ($5::DOUBLE PRECISION >= -180 AND $5::DOUBLE PRECISION <= 180) -- Longitude check
                     AND ($6::DOUBLE PRECISION >= -90 AND $6::DOUBLE PRECISION <= 90)   -- Latitude check
                THEN ST_SetSRID(ST_MakePoint($5::DOUBLE PRECISION, $6::DOUBLE PRECISION), 4326) 
                ELSE NULL 
            END
        ) RETURNING id
    `
	var id uuid.UUID

	// For ST_MakePoint, longitude is first, then latitude
	err = tx.QueryRow(ctx, query,
		city.Name,
		city.Country,
		NewNullString(city.StateProvince),
		city.AiSummary,
		NewNullFloat64(city.CenterLongitude),
		NewNullFloat64(city.CenterLatitude),
	).Scan(&id)

	if err != nil {
		// No need to check for pgx.ErrNoRows on INSERT RETURNING, an error means failure.
		return uuid.Nil, fmt.Errorf("failed to insert city: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return id, nil
}

// Helper function to convert empty strings to sql.NullString for database insertion
func NewNullString(s string) sql.NullString {
	if len(s) == 0 {
		return sql.NullString{} // Valid = false, String is empty
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

// Helper function to convert 0.0 float to sql.NullFloat64 for database insertion
func NewNullFloat64(f float64) sql.NullFloat64 {
	if f == 0.0 { // Or whatever your condition for "not set" is, e.g. NaN
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{
		Float64: f,
		Valid:   true,
	}
}

// You'll also need to update FindCityByNameAndCountry to retrieve these new fields.
func (r *RepositoryImpl) FindCityByNameAndCountry(ctx context.Context, cityName, countryName string) (*types.CityDetail, error) {
	query := `
        SELECT 
            id, name, country, 
            COALESCE(state_province, '') as state_province, -- Handle NULL state_province
            ai_summary,
            ST_Y(center_location) as center_latitude,    -- Extract Y coordinate (latitude)
            ST_X(center_location) as center_longitude   -- Extract X coordinate (longitude)
            -- Add bounding_box retrieval if you store it: ST_AsText(bounding_box) as bounding_box_wkt
        FROM cities
        WHERE LOWER(name) = LOWER($1)
        AND ($2 = '' OR country = $2)
    `

	var cityDetail types.CityDetail
	var lat, lon sql.NullFloat64 // To handle potentially NULL location

	err := r.pgpool.QueryRow(ctx, query, cityName, countryName).Scan(
		&cityDetail.ID,
		&cityDetail.Name,
		&cityDetail.Country,
		&cityDetail.StateProvince,
		&cityDetail.AiSummary,
		&lat,
		&lon,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find city '%s', '%s': %w", cityName, countryName, err)
	}

	if lat.Valid {
		cityDetail.CenterLatitude = lat.Float64
	}
	if lon.Valid {
		cityDetail.CenterLongitude = lon.Float64
	}

	return &cityDetail, nil
}
