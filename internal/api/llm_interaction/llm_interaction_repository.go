package llmInteraction

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ LLmInteractionRepository = (*PostgresLlmInteractionRepo)(nil)

type LLmInteractionRepository interface {
	SaveInteraction(ctx context.Context, interaction types.LlmInteraction) error

	SaveLlmSuggestedPOIsBatch(ctx context.Context, pois []types.POIDetail, userID, searchProfileID, llmInteractionID, cityID uuid.UUID) error
	GetLlmSuggestedPOIsByInteractionSortedByDistance(ctx context.Context, llmInteractionID uuid.UUID, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetail, error)
}

type PostgresLlmInteractionRepo struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewPostgresLlmInteractionRepo(pgxpool *pgxpool.Pool, logger *slog.Logger) *PostgresLlmInteractionRepo {
	return &PostgresLlmInteractionRepo{
		logger: logger,
		pgpool: pgxpool,
	}
}

func (r *PostgresLlmInteractionRepo) SaveInteraction(ctx context.Context, interaction types.LlmInteraction) error {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
        INSERT INTO llm_interactions (
            user_id, prompt, response_text, model_used, latency_ms
        ) VALUES ($1, $2, $3, $4, $5)
    `
	_, err = tx.Exec(ctx, query,
		interaction.UserID, interaction.Prompt, interaction.ResponseText,
		interaction.ModelUsed, interaction.LatencyMs,
	)

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return err
}

func (r *PostgresLlmInteractionRepo) SaveLlmSuggestedPOIsBatch(ctx context.Context, pois []types.POIDetail, userID, searchProfileID, llmInteractionID, cityID uuid.UUID) error {
	batch := &pgx.Batch{}
	query := `
        INSERT INTO llm_suggested_pois 
            (user_id, search_profile_id, llm_interaction_id, city_id, 
             name, description_poi, location, category, address, website, opening_hours_suggestion)
        VALUES 
            ($1, $2, $3, $4, $5, $6, ST_SetSRID(ST_MakePoint($7, $8), 4326), $9, $10, $11, $12)
    `
	// Note: searchProfileID and cityID can be nil (uuid.Nil) if not applicable.
	// The database schema should allow NULL for these if that's intended.
	// Handle uuid.Nil for searchProfileID and cityID if they are optional
	var spID sql.Null[uuid.UUID] // Using sql.Null for nullable UUIDs

	var cID sql.Null[uuid.UUID]

	for _, poi := range pois {
		batch.Queue(query,
			userID, spID, llmInteractionID, cID,
			poi.Name, poi.DescriptionPOI, poi.Longitude, poi.Latitude, // Lon, Lat order for ST_MakePoint
			poi.Category,
		)
	}

	br := r.pgpool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(pois); i++ {
		_, err := br.Exec()
		if err != nil {
			// Consider how to handle partial failures. Log and continue, or return error?
			return fmt.Errorf("failed to execute batch insert for llm_suggested_poi %d: %w", i, err)
		}
	}
	return nil
}

func (r *PostgresLlmInteractionRepo) GetLlmSuggestedPOIsByInteractionSortedByDistance(
	ctx context.Context, llmInteractionID uuid.UUID, cityID uuid.UUID, userLocation types.UserLocation,
) ([]types.POIDetail, error) {
	userPoint := fmt.Sprintf("SRID=4326;POINT(%f %f)", userLocation.UserLon, userLocation.UserLat)

	// Ensure cityID filter is applied if cityID is not Nil
	// We filter by llm_interaction_id, so city_id might be redundant if interaction is specific to a city context
	// But adding it for robustness if an interaction could span POIs from different "requested" cities (unlikely for current setup).
	query := `
        SELECT 
            id, 
            name, 
            description_poi,
            ST_X(location::geometry) AS longitude, 
            ST_Y(location::geometry) AS latitude, 
            category,
            address,
            website,
            opening_hours_suggestion,
            ST_Distance(location::geography, ST_GeomFromText($1, 4326)::geography) AS distance
        FROM llm_suggested_pois
        WHERE llm_interaction_id = $2 `

	args := []interface{}{userPoint, llmInteractionID}
	argCounter := 3

	if cityID != uuid.Nil {
		query += fmt.Sprintf("AND city_id = $%d ", argCounter)
		args = append(args, cityID)
		argCounter++
	}

	query += "ORDER BY distance ASC"

	rows, err := r.pgpool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query sorted llm_suggested_pois: %w", err)
	}
	defer rows.Close()

	var resultPois []types.POIDetail
	for rows.Next() {
		var p types.POIDetail
		var descr sql.NullString // Handle nullable fields from DB
		var cat sql.NullString
		var addr sql.NullString
		var web sql.NullString
		var openH sql.NullString

		err := rows.Scan(
			&p.ID, &p.Name, &descr,
			&p.Longitude, &p.Latitude,
			&cat, &addr, &web, &openH,
			&p.Distance, // Ensure your types.POIDetail has Distance field
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan llm_suggested_poi row: %w", err)
		}
		p.DescriptionPOI = descr.String
		p.Category = cat.String

		resultPois = append(resultPois, p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating llm_suggested_poi rows: %w", err)
	}

	return resultPois, nil
}
