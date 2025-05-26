package llmInteraction

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

var _ LLmInteractionRepository = (*PostgresLlmInteractionRepo)(nil)

type LLmInteractionRepository interface {
	SaveInteraction(ctx context.Context, interaction types.LlmInteraction) (uuid.UUID, error)
	SaveLlmSuggestedPOIsBatch(ctx context.Context, pois []types.POIDetail, userID, searchProfileID, llmInteractionID, cityID uuid.UUID) error
	GetLlmSuggestedPOIsByInteractionSortedByDistance(ctx context.Context, llmInteractionID uuid.UUID, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetail, error)
	AddChatToBookmark(ctx context.Context, itinerary *types.UserSavedItinerary) (uuid.UUID, error)
	RemoveChatFromBookmark(ctx context.Context, userID, itineraryID uuid.UUID) error
	GetInteractionByID(ctx context.Context, interactionID uuid.UUID) (*types.LlmInteraction, error)
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

func (r *PostgresLlmInteractionRepo) SaveInteraction(ctx context.Context, interaction types.LlmInteraction) (uuid.UUID, error) {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
        INSERT INTO llm_interactions (
            user_id, prompt, response_text, model_used, latency_ms
        ) VALUES ($1, $2, $3, $4, $5)
		RETURNING id
    `

	var interactionID uuid.UUID
	err = r.pgpool.QueryRow(ctx, query,
		interaction.UserID,
		//interaction.ProfileID, // Assuming you added ProfileID to types.LlmInteraction
		interaction.Prompt,
		interaction.ResponseText,
		interaction.ModelUsed,
		interaction.LatencyMs,
		//interaction.PromptTokens,     // Handle nil if these are pointers or use sql.NullInt64
		//interaction.CompletionTokens, // Handle nil
		//interaction.TotalTokens,      // Handle nil
		//interaction.RequestPayload,   // Handle nil
		//interaction.ResponsePayload,  // Handle nil
	).Scan(&interactionID)

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return interactionID, err
}

func (r *PostgresLlmInteractionRepo) SaveLlmSuggestedPOIsBatch(ctx context.Context, pois []types.POIDetail, userID, searchProfileID, llmInteractionID, cityID uuid.UUID) error {
	batch := &pgx.Batch{}
	query := `
        INSERT INTO llm_suggested_pois 
            (user_id, search_profile_id, llm_interaction_id, city_id, 
             name, description_poi, location)
        VALUES 
            ($1, $2, $3, $4, $5, $6, ST_SetSRID(ST_MakePoint($7, $8), 4326))
    `

	for _, poi := range pois {
		batch.Queue(query,
			userID, searchProfileID, llmInteractionID, cityID,
			poi.Name, poi.DescriptionPOI, poi.Longitude, poi.Latitude, // Lon, Lat order for ST_MakePoint
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
		// var cat sql.NullString
		// var addr sql.NullString
		// var web sql.NullString
		// var openH sql.NullString

		err := rows.Scan(
			&p.ID, &p.Name, &descr,
			&p.Longitude, &p.Latitude,
			&p.Distance, // Ensure your types.POIDetail has Distance field
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan llm_suggested_poi row: %w", err)
		}
		p.DescriptionPOI = descr.String
		//p.Category = cat.String

		resultPois = append(resultPois, p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating llm_suggested_poi rows: %w", err)
	}
	return resultPois, nil
}

func (r *PostgresLlmInteractionRepo) AddChatToBookmark(ctx context.Context, itinerary *types.UserSavedItinerary) (uuid.UUID, error) {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO user_saved_itineraries (
			user_id, source_llm_interaction_id, primary_city_id, title, description,
			markdown_content, tags, estimated_duration_days, estimated_cost_level, is_public
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	var savedItineraryID uuid.UUID
	if err := tx.QueryRow(ctx, query,
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
	).Scan(&savedItineraryID); err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert user_saved_itineraries: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return savedItineraryID, nil
}

func (r *PostgresLlmInteractionRepo) GetInteractionByID(ctx context.Context, interactionID uuid.UUID) (*types.LlmInteraction, error) {
	query := `
		SELECT 
			id, user_id, prompt, response_text, model_used, latency_ms,
			prompt_tokens, completion_tokens, total_tokens,
			request_payload, response_payload
		FROM llm_interactions
		WHERE id = $1
	`
	row := r.pgpool.QueryRow(ctx, query, interactionID)

	var interaction types.LlmInteraction

	nullPromptTokens := sql.NullInt64{}
	nullCompletionTokens := sql.NullInt64{}
	nullTotalTokens := sql.NullInt64{}
	nullRequestPayload := sql.NullString{}
	nullResponsePayload := sql.NullString{}

	if err := row.Scan(
		&interaction.ID,
		&interaction.UserID,
		&interaction.Prompt,
		&interaction.ResponseText,
		&interaction.ModelUsed,
		&interaction.LatencyMs,
		&nullPromptTokens,
		&nullCompletionTokens,
		&nullTotalTokens,
		&nullRequestPayload,
		&nullResponsePayload,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("no interaction found with ID %s", interactionID)
		}
		return nil, fmt.Errorf("failed to scan llm_interaction row: %w", err)
	}
	return &interaction, nil
}

func (r *PostgresLlmInteractionRepo) RemoveChatFromBookmark(ctx context.Context, userID, itineraryID uuid.UUID) error {
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		DELETE FROM user_saved_itineraries
		WHERE id = $1 AND user_id = $2
	`
	if _, err := tx.Exec(ctx, query, itineraryID, userID); err != nil {
		return fmt.Errorf("failed to delete user_saved_itinerary with ID %s: %w", itineraryID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
