package city

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ CityRepository = (*PostgresCityRepository)(nil)

type CityRepository interface {
	SaveCity(ctx context.Context, city types.CityDetail) (uuid.UUID, error)
	FindCityByNameAndCountry(ctx context.Context, name, country string) (*types.CityDetail, error)
}

type PostgresCityRepository struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewCityRepository(pgxpool *pgxpool.Pool, logger *slog.Logger) *PostgresCityRepository {
	return &PostgresCityRepository{
		logger: logger,
		pgpool: pgxpool,
	}
}

func (r *PostgresCityRepository) SaveCity(ctx context.Context, city types.CityDetail) (uuid.UUID, error) {
	query := `
        INSERT INTO cities (
            name, country, state_province, ai_summary
        ) VALUES ($1, $2, $3, $4) RETURNING id
    `
	var id uuid.UUID
	err := r.pgpool.QueryRow(ctx, query,
		city.Name, city.Country, city.StateProvince, city.AiSummary,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to save city: %w", err)
	}
	return id, nil
}

func (r *PostgresCityRepository) FindCityByNameAndCountry(ctx context.Context, name, country string) (*types.CityDetail, error) {
	query := `
        SELECT id, name, country, state_province, ai_summary
        FROM cities
        WHERE name = $1 AND country = $2
    `
	var city types.CityDetail
	err := r.pgpool.QueryRow(ctx, query, name, country).Scan(
		&city.ID, &city.Name, &city.Country, &city.StateProvince, &city.AiSummary,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find city: %w", err)
	}
	return &city, nil
}
