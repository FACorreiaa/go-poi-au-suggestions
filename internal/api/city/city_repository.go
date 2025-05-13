package city

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ CityRepository = (*PostgresCityRepository)(nil)

type CityRepository interface {
	SaveCity(ctx context.Context, city types.CityDetail) (uuid.UUID, error)
	FindCityByNameAndCountry(ctx context.Context, name string) (*types.CityDetail, error)
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
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
        INSERT INTO cities (
            name, country, state_province, ai_summary
        ) VALUES ($1, $2, $3, $4) RETURNING id
    `
	var id uuid.UUID
	if err = tx.QueryRow(ctx, query,
		city.Name, city.Country, city.StateProvince, city.AiSummary,
	).Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return uuid.Nil, nil
		}
		return uuid.Nil, fmt.Errorf("failed to insert city: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return id, nil
}

func (r *PostgresCityRepository) FindCityByNameAndCountry(ctx context.Context, name string) (*types.CityDetail, error) {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	// Perform city and POI operations with tx

	query := `
        SELECT id, name, country, state_province, ai_summary
        FROM cities
        WHERE name = $1 
    `
	var city types.CityDetail
	if err = tx.QueryRow(ctx, query, name).Scan(
		&city.ID, &city.Name, &city.Country, &city.StateProvince, &city.AiSummary,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find city: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &city, nil
}
