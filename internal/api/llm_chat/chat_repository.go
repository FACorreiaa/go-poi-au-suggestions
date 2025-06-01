package llmChat

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

var _ Repository = (*RepositoryImpl)(nil)

type Repository interface {
	SaveInteraction(ctx context.Context, interaction types.LlmInteraction) (uuid.UUID, error)
	SaveLlmSuggestedPOIsBatch(ctx context.Context, pois []types.POIDetail, userID, searchProfileID, llmInteractionID, cityID uuid.UUID) error
	GetLlmSuggestedPOIsByInteractionSortedByDistance(ctx context.Context, llmInteractionID uuid.UUID, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetail, error)
	AddChatToBookmark(ctx context.Context, itinerary *types.UserSavedItinerary) (uuid.UUID, error)
	RemoveChatFromBookmark(ctx context.Context, userID, itineraryID uuid.UUID) error
	GetInteractionByID(ctx context.Context, interactionID uuid.UUID) (*types.LlmInteraction, error)
	GetItinerary(ctx context.Context, userID, itineraryID uuid.UUID) (*types.UserSavedItinerary, error)
	GetItineraries(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]types.UserSavedItinerary, int, error)
	UpdateItinerary(ctx context.Context, userID uuid.UUID, itineraryID uuid.UUID, updates types.UpdateItineraryRequest) (*types.UserSavedItinerary, error)
}

type RepositoryImpl struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewRepositoryImpl(pgxpool *pgxpool.Pool, logger *slog.Logger) *RepositoryImpl {
	return &RepositoryImpl{
		logger: logger,
		pgpool: pgxpool,
	}
}

func (r *RepositoryImpl) SaveInteraction(ctx context.Context, interaction types.LlmInteraction) (uuid.UUID, error) {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "SaveInteraction", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "llm_interactions"),
		attribute.String("user.id", interaction.UserID.String()),
		attribute.String("model.used", interaction.ModelUsed),
		attribute.Int("latency.ms", interaction.LatencyMs),
	))
	defer span.End()

	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to start transaction")
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

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to insert interaction")
		return uuid.Nil, fmt.Errorf("failed to insert interaction: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to commit transaction")
		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	span.SetAttributes(attribute.String("interaction.id", interactionID.String()))
	span.SetStatus(codes.Ok, "Interaction saved successfully")
	return interactionID, nil
}

func (r *RepositoryImpl) SaveLlmSuggestedPOIsBatch(ctx context.Context, pois []types.POIDetail, userID, searchProfileID, llmInteractionID, cityID uuid.UUID) error {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "SaveLlmSuggestedPOIsBatch", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "llm_suggested_pois"),
		attribute.String("user.id", userID.String()),
		attribute.String("search_profile.id", searchProfileID.String()),
		attribute.String("llm_interaction.id", llmInteractionID.String()),
		attribute.String("city.id", cityID.String()),
		attribute.Int("pois.count", len(pois)),
	))
	defer span.End()

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
			span.RecordError(err)
			span.SetStatus(codes.Error, fmt.Sprintf("Failed to execute batch insert for POI %d", i))
			return fmt.Errorf("failed to execute batch insert for llm_suggested_poi %d: %w", i, err)
		}
	}

	span.SetStatus(codes.Ok, "POIs batch saved successfully")
	return nil
}

func (r *RepositoryImpl) GetLlmSuggestedPOIsByInteractionSortedByDistance(
	ctx context.Context, llmInteractionID uuid.UUID, cityID uuid.UUID, userLocation types.UserLocation,
) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "GetLlmSuggestedPOIsByInteractionSortedByDistance", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "llm_suggested_pois"),
		attribute.String("llm_interaction.id", llmInteractionID.String()),
		attribute.String("city.id", cityID.String()),
		attribute.Float64("user.latitude", userLocation.UserLat),
		attribute.Float64("user.longitude", userLocation.UserLon),
	))
	defer span.End()

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
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to query sorted POIs")
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
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to scan POI row")
			return nil, fmt.Errorf("failed to scan llm_suggested_poi row: %w", err)
		}
		p.DescriptionPOI = descr.String
		//p.Category = cat.String

		resultPois = append(resultPois, p)
	}

	if err = rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Error iterating POI rows")
		return nil, fmt.Errorf("error iterating llm_suggested_poi rows: %w", err)
	}

	span.SetAttributes(attribute.Int("pois.count", len(resultPois)))
	span.SetStatus(codes.Ok, "POIs retrieved successfully")
	return resultPois, nil
}

func (r *RepositoryImpl) AddChatToBookmark(ctx context.Context, itinerary *types.UserSavedItinerary) (uuid.UUID, error) {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "AddChatToBookmark", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "user_saved_itineraries"),
		attribute.String("user.id", itinerary.UserID.String()),
		attribute.String("title", itinerary.Title),
	))
	defer span.End()

	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to start transaction")
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
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to insert itinerary")
		return uuid.Nil, fmt.Errorf("failed to insert user_saved_itineraries: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to commit transaction")
		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	span.SetAttributes(attribute.String("saved_itinerary.id", savedItineraryID.String()))
	span.SetStatus(codes.Ok, "Itinerary saved successfully")
	return savedItineraryID, nil
}

func (r *RepositoryImpl) GetInteractionByID(ctx context.Context, interactionID uuid.UUID) (*types.LlmInteraction, error) {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "GetInteractionByID", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "llm_interactions"),
		attribute.String("interaction.id", interactionID.String()),
	))
	defer span.End()

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
			span.SetStatus(codes.Error, "Interaction not found")
			return nil, fmt.Errorf("no interaction found with ID %s", interactionID)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to scan interaction row")
		return nil, fmt.Errorf("failed to scan llm_interaction row: %w", err)
	}

	span.SetAttributes(
		attribute.String("user.id", interaction.UserID.String()),
		attribute.String("model.used", interaction.ModelUsed),
		attribute.Int("latency.ms", interaction.LatencyMs),
	)
	span.SetStatus(codes.Ok, "Interaction retrieved successfully")
	return &interaction, nil
}

func (r *RepositoryImpl) RemoveChatFromBookmark(ctx context.Context, userID, itineraryID uuid.UUID) error {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "RemoveChatFromBookmark", trace.WithAttributes(
		semconv.DBSystemPostgreSQL,
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.sql.table", "user_saved_itineraries"),
		attribute.String("user.id", userID.String()),
		attribute.String("itinerary.id", itineraryID.String()),
	))
	defer span.End()

	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to start transaction")
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		DELETE FROM user_saved_itineraries
		WHERE id = $1 AND user_id = $2
	`
	tag, err := tx.Exec(ctx, query, itineraryID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to delete itinerary")
		return fmt.Errorf("failed to delete user_saved_itinerary with ID %s: %w", itineraryID, err)
	}

	if tag.RowsAffected() == 0 {
		err := fmt.Errorf("no itinerary found with ID %s for user %s", itineraryID, userID)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Itinerary not found")
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to commit transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	span.SetStatus(codes.Ok, "Itinerary removed successfully")
	return nil
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
