package llmChat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
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

	// Session methods
	CreateSession(ctx context.Context, session types.ChatSession) error
	GetSession(ctx context.Context, sessionID uuid.UUID) (*types.ChatSession, error)
	GetUserChatSessions(ctx context.Context, userID uuid.UUID) ([]types.ChatSession, error)
	UpdateSession(ctx context.Context, session types.ChatSession) error
	AddMessageToSession(ctx context.Context, sessionID uuid.UUID, message types.ConversationMessage) error

	//
	SaveSinglePOI(ctx context.Context, poi types.POIDetail, userID, cityID uuid.UUID, llmInteractionID uuid.UUID) (uuid.UUID, error)
	GetPOIsBySessionSortedByDistance(ctx context.Context, sessionID, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetail, error)
	CalculateDistancePostGIS(ctx context.Context, userLat, userLon, poiLat, poiLon float64) (float64, error)
	GetOrCreatePOI(ctx context.Context, tx pgx.Tx, poiDetail types.POIDetail, cityID uuid.UUID, sourceInteractionID uuid.UUID) (uuid.UUID, error)

	// RAG
	//SaveInteractionWithEmbedding(ctx context.Context, interaction types.LlmInteraction, embedding []float32) (uuid.UUID, error)
	//FindSimilarInteractions(ctx context.Context, queryEmbedding []float32, limit int, threshold float32) ([]types.LlmInteraction, error)
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
		semconv.DBSystemKey.String(semconv.DBSystemPostgreSQL.Value.AsString()),
		attribute.String("db.operation", "INSERT_COMPLEX"),
		attribute.String("db.sql.table", "llm_interactions,itineraries,itinerary_pois"),
		attribute.String("user.id", interaction.UserID.String()),
		attribute.String("model.used", interaction.ModelUsed),
		attribute.Int("latency.ms", interaction.LatencyMs),
		attribute.String("city.name_from_interaction", interaction.CityName),
	))
	defer span.End()

	var err error
	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to start transaction")
		return uuid.Nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				r.logger.ErrorContext(ctx, "Transaction rollback failed after error", "original_error", err, "rollback_error", rbErr)
				span.RecordError(fmt.Errorf("transaction rollback failed: %v (original error: %w)", rbErr, err))
			}
		}
	}()

	interactionQuery := `
        INSERT INTO llm_interactions (
            user_id, prompt, response_text, model_used, latency_ms, city_name
        ) VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id
    `
	var interactionID uuid.UUID
	err = tx.QueryRow(ctx, interactionQuery,
		interaction.UserID,
		interaction.Prompt,
		interaction.ResponseText,
		interaction.ModelUsed,
		interaction.LatencyMs,
		interaction.CityName,
	).Scan(&interactionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to insert llm_interaction")
		return uuid.Nil, fmt.Errorf("failed to insert llm_interaction: %w", err)
	}
	span.SetAttributes(attribute.String("llm_interaction.id", interactionID.String()))

	var cityID uuid.UUID
	if interaction.CityName != "" {
		cityQuery := `SELECT id FROM cities WHERE name = $1 LIMIT 1`
		err = tx.QueryRow(ctx, cityQuery, interaction.CityName).Scan(&cityID)
		if err != nil {
			if err == pgx.ErrNoRows {
				r.logger.WarnContext(ctx, "City not found in database, itinerary creation will be skipped", "city_name", interaction.CityName, "interaction_id", interactionID.String())
				span.AddEvent("City not found in database", trace.WithAttributes(attribute.String("city.name", interaction.CityName)))
				// err is pgx.ErrNoRows, so cityID remains uuid.Nil, processing continues correctly. Clear err.
				err = nil
			} else {
				span.RecordError(err)
				span.SetStatus(codes.Error, "Failed to get city_id")
				return interactionID, fmt.Errorf("failed to get city_id for city '%s': %w", interaction.CityName, err)
			}
		} else {
			span.SetAttributes(attribute.String("city.id", cityID.String()))
		}
	} else {
		r.logger.InfoContext(ctx, "interaction.CityName is empty, cannot determine city_id. Itinerary creation will be skipped.", "interaction_id", interactionID.String())
		span.AddEvent("interaction.CityName is empty")
	}

	var itineraryID uuid.UUID
	if cityID != uuid.Nil {
		itineraryQuery := `
	        INSERT INTO itineraries (user_id, city_id, source_llm_interaction_id)
	        VALUES ($1, $2, $3)
	        ON CONFLICT (user_id, city_id) DO UPDATE SET
	            updated_at = NOW(),
	            source_llm_interaction_id = EXCLUDED.source_llm_interaction_id
	        RETURNING id
	    `
		err = tx.QueryRow(ctx, itineraryQuery, interaction.UserID, cityID, interactionID).Scan(&itineraryID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to insert or update itinerary")
			return interactionID, fmt.Errorf("failed to insert or update itinerary: %w", err)
		}
		span.SetAttributes(attribute.String("itinerary.id", itineraryID.String()))
	}

	if itineraryID != uuid.Nil {
		var pois []types.POIDetail
		// Only parse POIs for itinerary/general responses, skip for domain-specific responses
		if strings.Contains(interaction.Prompt, "Unified Chat - Domain: dining") ||
			strings.Contains(interaction.Prompt, "Unified Chat - Domain: accommodation") ||
			strings.Contains(interaction.Prompt, "Unified Chat - Domain: activities") {
			// Skip POI parsing for domain-specific responses that don't contain POIs
			r.logger.DebugContext(ctx, "Skipping POI parsing for domain-specific response", "interaction_id", interactionID.String())
			span.AddEvent("Skipped POI parsing for domain-specific response")
		} else {
			pois, err = parsePOIsFromResponse(interaction.ResponseText, r.logger)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "Failed to parse POIs from response")
				return interactionID, fmt.Errorf("failed to parse POIs from response: %w", err)
			}
		}
		span.SetAttributes(attribute.Int("parsed_pois.count", len(pois)))

		if len(pois) > 0 {
			poiBatch := &pgx.Batch{}
			itineraryPoiInsertQuery := `
	            INSERT INTO itinerary_pois (itinerary_id, poi_id, order_index, ai_description)
	            VALUES ($1, $2, $3, $4)
	            ON CONFLICT (itinerary_id, poi_id) DO UPDATE SET
	                order_index = EXCLUDED.order_index,
	                ai_description = EXCLUDED.ai_description,
	                updated_at = NOW()
	        `
			for i, poiDetailFromLlm := range pois {
				var poiDBID uuid.UUID
				poiDBID, err = r.GetOrCreatePOI(ctx, tx, poiDetailFromLlm, cityID, interactionID)
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, "Failed to get or create POI")
					return interactionID, fmt.Errorf("failed to get or create POI '%s': %w", poiDetailFromLlm.Name, err)
				}
				poiBatch.Queue(itineraryPoiInsertQuery, itineraryID, poiDBID, i, poiDetailFromLlm.DescriptionPOI) // Assumes types.POIDetail has DescriptionPOI
			}

			if poiBatch.Len() > 0 {
				br := tx.SendBatch(ctx, poiBatch)
				for i := 0; i < poiBatch.Len(); i++ {
					_, execErr := br.Exec()
					if execErr != nil {
						err = fmt.Errorf("failed to insert itinerary_poi in batch (operation %d of %d for itinerary %s): %w", i+1, poiBatch.Len(), itineraryID.String(), execErr)
						if closeErr := br.Close(); closeErr != nil {
							r.logger.ErrorContext(ctx, "Failed to close batch for itinerary_pois after an exec error", "close_error", closeErr, "original_batch_error", err)
						}
						span.RecordError(err)
						span.SetStatus(codes.Error, "Failed to insert itinerary_poi in batch")
						return interactionID, err
					}
				}
				err = br.Close()
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, "Failed to close batch for itinerary_pois")
					return interactionID, fmt.Errorf("failed to close batch for itinerary_pois: %w", err)
				}
				span.SetAttributes(attribute.Int("itinerary_pois.inserted_or_updated.count", poiBatch.Len()))
			}
		}
	} else {
		if cityID != uuid.Nil {
			r.logger.WarnContext(ctx, "ItineraryID is Nil despite valid CityID, indicating itinerary insert/update issue.", "city_id", cityID.String(), "interaction_id", interactionID.String())
			span.AddEvent("ItineraryID is Nil despite valid CityID.")
		} else {
			r.logger.InfoContext(ctx, "Skipping itinerary_pois: itineraryID is Nil (likely city not found or CityName empty).", "interaction_id", interactionID.String())
			span.AddEvent("Skipping itinerary_pois: itineraryID is Nil.")
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to commit transaction")
		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	span.SetStatus(codes.Ok, "Interaction and related entities saved successfully")
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

// sessions
func (r *RepositoryImpl) CreateSession(ctx context.Context, session types.ChatSession) error {
	query := `
        INSERT INTO chat_sessions (
            id, user_id, current_itinerary, conversation_history, session_context,
            created_at, updated_at, expires_at, status
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `
	itineraryJSON, _ := json.Marshal(session.CurrentItinerary)
	historyJSON, _ := json.Marshal(session.ConversationHistory)
	contextJSON, _ := json.Marshal(session.SessionContext)

	_, err := r.pgpool.Exec(ctx, query, session.ID, session.UserID, itineraryJSON, historyJSON, contextJSON,
		session.CreatedAt, session.UpdatedAt, session.ExpiresAt, session.Status)
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to create session", slog.Any("error", err))
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// GetSession retrieves a session by ID
func (r *RepositoryImpl) GetSession(ctx context.Context, sessionID uuid.UUID) (*types.ChatSession, error) {
	query := `
        SELECT id, user_id, current_itinerary, conversation_history, session_context,
               created_at, updated_at, expires_at, status
        FROM chat_sessions WHERE id = $1
    `
	row := r.pgpool.QueryRow(ctx, query, sessionID)

	var session types.ChatSession
	var itineraryJSON, historyJSON, contextJSON []byte
	err := row.Scan(&session.ID, &session.UserID, &itineraryJSON, &historyJSON, &contextJSON,
		&session.CreatedAt, &session.UpdatedAt, &session.ExpiresAt, &session.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session %s not found", sessionID)
		}
		r.logger.ErrorContext(ctx, "Failed to get session", slog.Any("error", err))
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	json.Unmarshal(itineraryJSON, &session.CurrentItinerary)
	json.Unmarshal(historyJSON, &session.ConversationHistory)
	json.Unmarshal(contextJSON, &session.SessionContext)
	return &session, nil
}

// GetUserChatSessions retrieves chat history from LLM interactions grouped by session/city, ordered by most recent first
func (r *RepositoryImpl) GetUserChatSessions(ctx context.Context, userID uuid.UUID) ([]types.ChatSession, error) {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "GetUserChatSessions", trace.WithAttributes(
		semconv.DBSystemKey.String(semconv.DBSystemPostgreSQL.Value.AsString()),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "llm_interactions"),
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	query := `
        WITH grouped_interactions AS (
            SELECT 
                COALESCE(session_id, city_name || '_' || DATE(created_at)) as session_key,
                user_id,
                city_name,
                MIN(created_at) as first_interaction,
                MAX(created_at) as last_interaction,
                COUNT(*) as interaction_count,
                json_agg(
                    json_build_object(
                        'id', id,
                        'prompt', prompt,
                        'response_text', response_text,
                        'created_at', created_at,
                        'city_name', city_name,
                        'session_id', session_id
                    ) ORDER BY created_at
                ) as interactions
            FROM llm_interactions 
            WHERE user_id = $1 AND prompt IS NOT NULL
            GROUP BY session_key, user_id, city_name
        )
        SELECT 
            session_key,
            user_id,
            city_name,
            first_interaction,
            last_interaction,
            interaction_count,
            interactions
        FROM grouped_interactions
        ORDER BY last_interaction DESC
        LIMIT 50
    `

	rows, err := r.pgpool.Query(ctx, query, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to query LLM interactions")
		r.logger.ErrorContext(ctx, "Failed to get user chat sessions from LLM interactions", slog.Any("error", err), slog.String("user_id", userID.String()))
		return nil, fmt.Errorf("failed to get user chat sessions: %w", err)
	}
	defer rows.Close()

	var sessions []types.ChatSession
	for rows.Next() {
		var sessionKey, cityName string
		var userIDFromDB uuid.UUID
		var firstInteraction, lastInteraction time.Time
		var interactionCount int
		var interactionsJSON string

		err := rows.Scan(&sessionKey, &userIDFromDB, &cityName, &firstInteraction, &lastInteraction, &interactionCount, &interactionsJSON)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to scan LLM interaction row")
			r.logger.ErrorContext(ctx, "Failed to scan LLM interaction row", slog.Any("error", err))
			return nil, fmt.Errorf("failed to scan LLM interaction row: %w", err)
		}

		var interactions []map[string]interface{}
		if err := json.Unmarshal([]byte(interactionsJSON), &interactions); err != nil {
			r.logger.WarnContext(ctx, "Failed to parse interactions JSON", slog.Any("error", err))
			continue
		}

		var conversationHistory []types.ConversationMessage
		for _, interaction := range interactions {
			if prompt, ok := interaction["prompt"].(string); ok && prompt != "" {
				conversationHistory = append(conversationHistory, types.ConversationMessage{
					Role:      "user",
					Content:   prompt,
					Timestamp: parseTimeFromInterface(interaction["created_at"]),
				})
			}
			if response, ok := interaction["response_text"].(string); ok {
				if response == "" {
					response = fmt.Sprintf("I provided recommendations for %s", cityName)
				} else {
					// Convert JSON response to human-readable format
					response = formatResponseForDisplay(response, cityName)
				}
				conversationHistory = append(conversationHistory, types.ConversationMessage{
					Role:      "assistant",
					Content:   response,
					Timestamp: parseTimeFromInterface(interaction["created_at"]),
				})
			}
		}

		sessionID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(sessionKey))
		session := types.ChatSession{
			ID:                  sessionID,
			UserID:              userIDFromDB,
			CityName:            cityName,
			ConversationHistory: conversationHistory,
			CreatedAt:           firstInteraction,
			UpdatedAt:           lastInteraction,
			Status:              "active",
		}
		sessions = append(sessions, session)
	}

	if err = rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Error iterating through LLM interaction rows")
		r.logger.ErrorContext(ctx, "Error iterating through LLM interaction rows", slog.Any("error", err))
		return nil, fmt.Errorf("error iterating through LLM interaction rows: %w", err)
	}

	span.SetAttributes(attribute.Int("sessions.count", len(sessions)))
	return sessions, nil
}

// Helper function to parse time from interface{}
func parseTimeFromInterface(timeInterface interface{}) time.Time {
	switch t := timeInterface.(type) {
	case time.Time:
		return t
	case string:
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed
		}
	}
	return time.Now()
}

// Helper function to format JSON response for human-readable display
func formatResponseForDisplay(response, cityName string) string {
	// Handle responses with prefixed tags like [itinerary], [city_data], etc.
	cleanedResponse := response
	
	// Remove common LLM response prefixes
	prefixPatterns := []string{
		`\[itinerary\]\s*`,
		`\[city_data\]\s*`,
		`\[restaurants\]\s*`,
		`\[hotels\]\s*`,
		`\[activities\]\s*`,
		`\[pois\]\s*`,
	}
	
	for _, pattern := range prefixPatterns {
		re := regexp.MustCompile(`(?i)^` + pattern)
		cleanedResponse = re.ReplaceAllString(cleanedResponse, "")
	}
	
	// Remove markdown code blocks if present
	cleanedResponse = regexp.MustCompile("(?s)```json\\s*(.*)\\s*```").ReplaceAllString(cleanedResponse, "$1")
	cleanedResponse = strings.TrimSpace(cleanedResponse)
	
	// First, check if cleaned response is valid JSON
	if !json.Valid([]byte(cleanedResponse)) {
		// If not JSON, return as-is (might be already formatted text)
		return response
	}

	// Try to parse as AiCityResponse (most common format)
	var cityResponse types.AiCityResponse
	if err := json.Unmarshal([]byte(cleanedResponse), &cityResponse); err == nil {
		// Check if it's a valid itinerary response (either has POIs or itinerary data)
		if len(cityResponse.PointsOfInterest) > 0 || cityResponse.AIItineraryResponse.ItineraryName != "" || len(cityResponse.AIItineraryResponse.PointsOfInterest) > 0 {
			return formatItineraryResponse(cityResponse, cityName)
		}
	}

	// Try to parse as hotel array
	var hotels []types.HotelDetailedInfo
	if err := json.Unmarshal([]byte(cleanedResponse), &hotels); err == nil && len(hotels) > 0 {
		return formatHotelResponse(hotels, cityName)
	}

	// Try to parse as restaurant array
	var restaurants []types.RestaurantDetailedInfo
	if err := json.Unmarshal([]byte(cleanedResponse), &restaurants); err == nil && len(restaurants) > 0 {
		return formatRestaurantResponse(restaurants, cityName)
	}

	// Try to parse as POI array
	var pois []types.POIDetailedInfo
	if err := json.Unmarshal([]byte(cleanedResponse), &pois); err == nil && len(pois) > 0 {
		return formatPOIResponse(pois, cityName)
	}

	// If we can't parse it meaningfully, return a generic message
	return fmt.Sprintf("I provided personalized recommendations for %s. Here are some great options I found for you!", cityName)
}

// Format itinerary response to readable text
func formatItineraryResponse(response types.AiCityResponse, cityName string) string {
	// Determine which POI list to use and total count
	var totalPOIs int
	var firstPOIName string
	
	// Check both POI arrays and get the total count
	if len(response.PointsOfInterest) > 0 {
		totalPOIs += len(response.PointsOfInterest)
		firstPOIName = getFirstPOIName(response.PointsOfInterest)
	}
	
	if len(response.AIItineraryResponse.PointsOfInterest) > 0 {
		totalPOIs += len(response.AIItineraryResponse.PointsOfInterest)
		if firstPOIName == "" {
			firstPOIName = getFirstPOIName(response.AIItineraryResponse.PointsOfInterest)
		}
	}
	
	// If we have an itinerary name, use it
	if response.AIItineraryResponse.ItineraryName != "" {
		if totalPOIs > 0 {
			return fmt.Sprintf("I created a personalized itinerary called '%s' for %s with %d amazing places to visit, including %s and more!",
				response.AIItineraryResponse.ItineraryName,
				cityName,
				totalPOIs,
				firstPOIName)
		} else {
			return fmt.Sprintf("I created a personalized itinerary called '%s' for %s with great recommendations!",
				response.AIItineraryResponse.ItineraryName,
				cityName)
		}
	}

	// Fallback to generic response
	if totalPOIs > 0 {
		return fmt.Sprintf("I found %d great places to visit in %s, including %s. Perfect for your trip!",
			totalPOIs,
			cityName,
			firstPOIName)
	}
	
	return fmt.Sprintf("I provided personalized recommendations for %s. Here are some great options I found for you!", cityName)
}

// Format hotel response to readable text
func formatHotelResponse(hotels []types.HotelDetailedInfo, cityName string) string {
	if len(hotels) == 0 {
		return fmt.Sprintf("I searched for hotels in %s for you.", cityName)
	}

	return fmt.Sprintf("I found %d excellent hotel%s in %s, including %s and other great options that match your preferences!",
		len(hotels),
		pluralize(len(hotels)),
		cityName,
		hotels[0].Name)
}

// Format restaurant response to readable text
func formatRestaurantResponse(restaurants []types.RestaurantDetailedInfo, cityName string) string {
	if len(restaurants) == 0 {
		return fmt.Sprintf("I searched for restaurants in %s for you.", cityName)
	}

	return fmt.Sprintf("I discovered %d fantastic restaurant%s in %s, starting with %s and many more delicious options!",
		len(restaurants),
		pluralize(len(restaurants)),
		cityName,
		restaurants[0].Name)
}

// Format POI response to readable text
func formatPOIResponse(pois []types.POIDetailedInfo, cityName string) string {
	if len(pois) == 0 {
		return fmt.Sprintf("I searched for activities in %s for you.", cityName)
	}

	return fmt.Sprintf("I found %d exciting place%s to visit in %s, including %s and other amazing spots you'll love!",
		len(pois),
		pluralize(len(pois)),
		cityName,
		pois[0].Name)
}

// Helper functions
func getFirstPOIName(pois []types.POIDetail) string {
	if len(pois) > 0 {
		return pois[0].Name
	}
	return "some amazing attractions"
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// Helper function to generate session title from conversation history
func generateSessionTitleFromHistory(history []types.ConversationMessage, cityName string) string {
	// Find first user message
	for _, msg := range history {
		if msg.Role == "user" && len(msg.Content) > 0 {
			words := strings.Fields(msg.Content)
			if len(words) > 4 {
				return strings.Join(words[:4], " ") + "..."
			}
			return msg.Content
		}
	}

	// Fallback to city name if available
	if cityName != "" {
		return "Trip to " + cityName
	}

	return "Chat Session"
}

// UpdateSession updates an existing session
func (r *RepositoryImpl) UpdateSession(ctx context.Context, session types.ChatSession) error {
	query := `
        UPDATE chat_sessions SET current_itinerary = $2, conversation_history = $3, session_context = $4,
                                 updated_at = $5, expires_at = $6, status = $7
        WHERE id = $1
    `
	itineraryJSON, _ := json.Marshal(session.CurrentItinerary)
	historyJSON, _ := json.Marshal(session.ConversationHistory)
	contextJSON, _ := json.Marshal(session.SessionContext)

	_, err := r.pgpool.Exec(ctx, query, session.ID, itineraryJSON, historyJSON, contextJSON,
		session.UpdatedAt, session.ExpiresAt, session.Status)
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to update session", slog.Any("error", err))
		return fmt.Errorf("failed to update session: %w", err)
	}
	return nil
}

// AddMessageToSession appends a message to the session's conversation history
func (r *RepositoryImpl) AddMessageToSession(ctx context.Context, sessionID uuid.UUID, message types.ConversationMessage) error {
	session, err := r.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	session.ConversationHistory = append(session.ConversationHistory, message)
	session.UpdatedAt = time.Now()
	return r.UpdateSession(ctx, *session)
}

func (r *RepositoryImpl) SaveSinglePOI(ctx context.Context, poi types.POIDetail, userID, cityID, llmInteractionID uuid.UUID) (uuid.UUID, error) {
	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "SaveSinglePOI", trace.WithAttributes(
		attribute.String("poi.name", poi.Name), /* ... */
	))
	defer span.End()

	// Validate coordinates before attempting to use them.
	if poi.Latitude < -90 || poi.Latitude > 90 || poi.Longitude < -180 || poi.Longitude > 180 {
		// Or if they are exactly 0,0 and that's considered invalid from LLM
		err := fmt.Errorf("invalid coordinates for POI %s: lat %f, lon %f", poi.Name, poi.Latitude, poi.Longitude)
		span.RecordError(err)
		return uuid.Nil, err
	}

	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// If poi.ID is already set (e.g., from LLM or previous step), use it. Otherwise, generate new.
	recordID := poi.ID
	if recordID == uuid.Nil {
		recordID = uuid.New()
	}

	// Columns: id, user_id, city_id, llm_interaction_id, name, latitude, longitude, location, category, description_poi (10 columns)
	// Values: $1, $2, $3, $4, $5, $6, $7, ST_MakePoint($7, $6), $8, $9 (10 value expressions for 9 placeholders + ST_MakePoint)
	// Corrected: 9 distinct columns from poiData + 1 for location, then id for the record.
	// Order of columns in INSERT INTO: id, user_id, city_id, llm_interaction_id, name, latitude, longitude, location, category, description_poi
	// Placeholders:                $1,    $2,      $3,      $4,                 $5,   $6,       $7,        ST_MakePoint($7,$6), $8, $9
	query := `
        INSERT INTO llm_suggested_pois (
            id, user_id, city_id, llm_interaction_id, name, 
            latitude, longitude, "location", -- Ensure "location" is quoted if it's a reserved keyword or mixed case
            category, description_poi 
            -- Removed distance from INSERT list
        ) VALUES (
            $1, $2, $3, $4, $5, 
            $6, $7, ST_SetSRID(ST_MakePoint($7, $6), 4326), -- Longitude ($7) first, then Latitude ($6)
            $8, $9
        )
        RETURNING id
    `
	// Arguments should be:
	// $1: recordID (for llm_suggested_pois.id)
	// $2: userID
	// $3: cityID
	// $4: llmInteractionID
	// $5: poi.Name
	// $6: poi.Latitude  (for the latitude column)
	// $7: poi.Longitude (for the longitude column AND for ST_MakePoint's X)
	// $8: poi.Category
	// $9: poi.DescriptionPOI

	var returnedID uuid.UUID
	err = tx.QueryRow(ctx, query,
		recordID,         // $1: id
		userID,           // $2: user_id
		cityID,           // $3: city_id
		llmInteractionID, // $4: llm_interaction_id
		poi.Name,         // $5: name
		poi.Latitude,     // $6: latitude column value
		poi.Longitude,    // $7: longitude column value (also used as X in ST_MakePoint)
		// ST_MakePoint will use $7 (poi.Longitude) as X and $6 (poi.Latitude) as Y
		poi.Category,       // $8: category
		poi.DescriptionPOI, // $9: description_poi
	).Scan(&returnedID)

	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to insert llm_suggested_poi", slog.Any("error", err), slog.String("query", query), slog.String("name", poi.Name))
		span.RecordError(err)
		return uuid.Nil, fmt.Errorf("failed to save llm_suggested_poi: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		span.RecordError(err)
		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("LLM Suggested POI saved successfully", slog.String("id", returnedID.String()))
	return returnedID, nil
}

func (r *RepositoryImpl) GetPOIsBySessionSortedByDistance(ctx context.Context, sessionID, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetail, error) {

	query := `
        SELECT id, name, latitude, longitude, category, description_poi, 
               ST_Distance(
                   ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography,
                   location::geography  -- Use the actual geometry column for distance
               ) AS distance
        FROM llm_suggested_pois  -- Assuming this is the correct table to query for session POIs
        WHERE city_id = $1 
        -- Add AND llm_interaction_id IN (SELECT ...) if POIs are tied to specific interactions of the session
        ORDER BY distance ASC;
    `
	rows, err := r.pgpool.Query(ctx, query, cityID, userLocation.UserLon, userLocation.UserLat)
	if err != nil {
		return nil, fmt.Errorf("failed to query POIs for session: %w", err)
	}
	defer rows.Close()

	var pois []types.POIDetail
	for rows.Next() {
		var p types.POIDetail
		var lat, lon, dist sql.NullFloat64 // Use nullable types
		var cat, desc sql.NullString

		// Adjust scan to match selected columns and their nullability
		err := rows.Scan(&p.ID, &p.Name, &lat, &lon, &cat, &desc, &dist)
		if err != nil {
			return nil, fmt.Errorf("failed to scan POI for session: %w", err)
		}

		if lat.Valid {
			p.Latitude = lat.Float64
		}
		if lon.Valid {
			p.Longitude = lon.Float64
		}
		if cat.Valid {
			p.Category = cat.String
		}
		if desc.Valid {
			p.DescriptionPOI = desc.String
		}
		if dist.Valid {
			p.Distance = dist.Float64
		}

		pois = append(pois, p)
	}
	return pois, rows.Err()
}

// calculateDistancePostGIS computes the distance between two points using PostGIS (in meters)
func (l *RepositoryImpl) CalculateDistancePostGIS(ctx context.Context, userLat, userLon, poiLat, poiLon float64) (float64, error) {
	query := `
        SELECT ST_Distance(
            ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
            ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography
        ) AS distance;
    `
	var distance float64
	err := l.pgpool.QueryRow(ctx, query, userLon, userLat, poiLon, poiLat).Scan(&distance)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate distance with PostGIS: %w", err)
	}
	return distance, nil
}

// type POIDetail struct {
// 	Name        string  `json:"name"`
// 	Latitude    float64 `json:"latitude"`
// 	Longitude   float64 `json:"longitude"`
// 	Category    string  `json:"category"`
// 	Description string  `json:"description"`
// }

// type LlmApiResponseData struct {
// 	GeneralCityData struct {
// 		City            string  `json:"city"`
// 		Country         string  `json:"country"`
// 		Description     string  `json:"description"`
// 		CenterLatitude  float64 `json:"center_latitude"`
// 		CenterLongitude float64 `json:"center_longitude"`
// 		// Add other fields from general_city_data if you need them
// 		// Population       string  `json:"population,omitempty"`
// 		// Area             string  `json:"area,omitempty"`
// 		// Timezone         string  `json:"timezone,omitempty"`
// 		// Language         string  `json:"language,omitempty"`
// 		// Weather          string  `json:"weather,omitempty"`
// 		// Attractions      string  `json:"attractions,omitempty"`
// 		// History          string  `json:"history,omitempty"`
// 	} `json:"general_city_data"`

// 	PointsOfInterest []types.POIDetail `json:"points_of_interest"` // <--- ADD THIS FIELD for general POIs

// 	ItineraryResponse struct {
// 		ItineraryName      string            `json:"itinerary_name"`
// 		OverallDescription string            `json:"overall_description"`
// 		PointsOfInterest   []types.POIDetail `json:"points_of_interest"` // This is for itinerary_response.points_of_interest
// 	} `json:"itinerary_response"`
// }

// type LlmApiResponse struct {
// 	SessionID string             `json:"session_id"` // Capture the top-level session_id
// 	Data      LlmApiResponseData `json:"data"`
// 	// Note: The JSON also has a "session_id" inside "data".
// 	// If you need that too, you'd add it to LlmApiResponseData:
// 	// SessionIDInsideData string `json:"session_id,omitempty"`
// }

func parsePOIsFromResponse(responseText string, logger *slog.Logger) ([]types.POIDetail, error) {
	cleanedResponse := cleanJSONResponse(responseText)

	// First try to parse as unified chat response format with "data" wrapper
	var unifiedResponse struct {
		Data types.AiCityResponse `json:"data"`
	}
	err := json.Unmarshal([]byte(cleanedResponse), &unifiedResponse)
	if err == nil {
		// Collect POIs from both general points_of_interest and itinerary points_of_interest
		var allPOIs []types.POIDetail
		if unifiedResponse.Data.PointsOfInterest != nil {
			allPOIs = append(allPOIs, unifiedResponse.Data.PointsOfInterest...)
		}
		if unifiedResponse.Data.AIItineraryResponse.PointsOfInterest != nil {
			allPOIs = append(allPOIs, unifiedResponse.Data.AIItineraryResponse.PointsOfInterest...)
		}
		if len(allPOIs) > 0 {
			logger.Debug("parsePOIsFromResponse: Parsed as unified chat response", "poiCount", len(allPOIs))
			return allPOIs, nil
		}
	}

	// Second, try to parse as a full AiCityResponse (for legacy responses)
	var parsedResponse types.AiCityResponse
	err = json.Unmarshal([]byte(cleanedResponse), &parsedResponse)
	if err == nil && parsedResponse.PointsOfInterest != nil {
		logger.Debug("parsePOIsFromResponse: Parsed as AiCityResponse", "poiCount", len(parsedResponse.PointsOfInterest))
		return parsedResponse.PointsOfInterest, nil
	}

	// Third, try to parse as a single POI (for individual POI additions)
	var singlePOI types.POIDetail
	err = json.Unmarshal([]byte(cleanedResponse), &singlePOI)
	if err == nil && singlePOI.Name != "" {
		logger.Debug("parsePOIsFromResponse: Parsed as single POI", "poiName", singlePOI.Name)
		return []types.POIDetail{singlePOI}, nil
	}

	// If all fail, log the error and return empty
	logger.Warn("parsePOIsFromResponse: Could not parse response as unified chat, AiCityResponse, or single POI",
		"error", err,
		"cleanedResponseLength", len(cleanedResponse),
		"responsePreview", cleanedResponse[:min(200, len(cleanedResponse))])
	return []types.POIDetail{}, nil
}

func (r *RepositoryImpl) GetOrCreatePOI(ctx context.Context, tx pgx.Tx, poiDetail types.POIDetail, cityID uuid.UUID, sourceInteractionID uuid.UUID) (uuid.UUID, error) {
	var poiDBID uuid.UUID
	findPoiQuery := `SELECT id FROM points_of_interest WHERE name = $1 AND city_id = $2 LIMIT 1`
	err := tx.QueryRow(ctx, findPoiQuery, poiDetail.Name, cityID).Scan(&poiDBID)

	if err == pgx.ErrNoRows {
		createPoiQuery := `
            INSERT INTO points_of_interest (name, city_id, location, category, description)
            VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326), $5, $6) RETURNING id`
		err = tx.QueryRow(ctx, createPoiQuery,
			poiDetail.Name,
			cityID,
			poiDetail.Latitude,
			poiDetail.Longitude,
			poiDetail.Category,
			poiDetail.DescriptionPOI, // Assumes types.POIDetail has DescriptionPOI from JSON
		).Scan(&poiDBID)
		if err != nil {
			r.logger.ErrorContext(ctx, "GetOrCreatePOI: Failed to insert new POI", "error", err, "poi_name", poiDetail.Name)
			return uuid.Nil, fmt.Errorf("GetOrCreatePOI: failed to insert new POI '%s': %w", poiDetail.Name, err)
		}
	} else if err != nil {
		r.logger.ErrorContext(ctx, "GetOrCreatePOI: Failed to query existing POI", "error", err, "poi_name", poiDetail.Name)
		return uuid.Nil, fmt.Errorf("GetOrCreatePOI: failed to query existing POI '%s': %w", poiDetail.Name, err)
	}
	return poiDBID, nil
}

// func (r *RepositoryImpl) SaveInteractionWithEmbedding(ctx context.Context, interaction types.LlmInteraction, embedding []float32) (uuid.UUID, error) {
// 	ctx, span := otel.Tracer("LlmInteractionRepo").Start(ctx, "SaveInteractionWithEmbedding", trace.WithAttributes(
// 		semconv.DBSystemPostgreSQL,
// 		attribute.String("db.operation", "INSERT_COMPLEX"),
// 		attribute.String("db.sql.table", "llm_interactions,itineraries,itinerary_pois"),
// 		attribute.String("user.id", interaction.UserID.String()),
// 		attribute.String("model.used", interaction.ModelUsed),
// 		attribute.Int("latency.ms", interaction.LatencyMs),
// 		attribute.String("city.name", interaction.CityName),
// 	))
// 	defer span.End()

// 	var err error
// 	tx, err := r.pgpool.BeginTx(ctx, pgx.TxOptions{})
// 	if err != nil {
// 		span.RecordError(err)
// 		span.SetStatus(codes.Error, "Failed to start transaction")
// 		return uuid.Nil, fmt.Errorf("failed to start transaction: %w", err)
// 	}
// 	defer func() {
// 		if p := recover(); p != nil {
// 			_ = tx.Rollback(ctx)
// 			panic(p)
// 		}
// 		if err != nil {
// 			if rbErr := tx.Rollback(ctx); rbErr != nil {
// 				r.logger.ErrorContext(ctx, "Transaction rollback failed", slog.Any("error", rbErr))
// 			}
// 		}
// 	}()

// 	// Convert embedding to pgvector format
// 	vectorParam := pgvector.NewVector(embedding)

// 	interactionQuery := `
//         INSERT INTO llm_interactions (
//             user_id, prompt, response_text, model_used, latency_ms, city_name, prompt_embedding
//         ) VALUES ($1, $2, $3, $4, $5, $6, $7)
//         RETURNING id
//     `
// 	var interactionID uuid.UUID
// 	err = tx.QueryRow(ctx, interactionQuery,
// 		interaction.UserID,
// 		interaction.Prompt,
// 		interaction.ResponseText,
// 		interaction.ModelUsed,
// 		interaction.LatencyMs,
// 		interaction.CityName,
// 		vectorParam,
// 	).Scan(&interactionID)
// 	if err != nil {
// 		span.RecordError(err)
// 		span.SetStatus(codes.Error, "Failed to insert llm_interaction")
// 		return uuid.Nil, fmt.Errorf("failed to insert llm_interaction: %w", err)
// 	}
// 	span.SetAttributes(attribute.String("llm_interaction.id", interactionID.String()))

// 	// Existing itinerary and POI logic remains unchanged
// 	var cityID uuid.UUID
// 	if interaction.CityName != "" {
// 		cityQuery := `SELECT id FROM cities WHERE name = $1 LIMIT 1`
// 		err = tx.QueryRow(ctx, cityQuery, interaction.CityName).Scan(&cityID)
// 		if err != nil && err != pgx.ErrNoRows {
// 			span.RecordError(err)
// 			return interactionID, fmt.Errorf("failed to get city_id: %w", err)
// 		}
// 	}

// 	var itineraryID uuid.UUID
// 	if cityID != uuid.Nil {
// 		itineraryQuery := `
//             INSERT INTO itineraries (user_id, city_id, source_llm_interaction_id)
//             VALUES ($1, $2, $3)
//             ON CONFLICT (user_id, city_id) DO UPDATE SET
//                 updated_at = NOW(),
//                 source_llm_interaction_id = EXCLUDED.source_llm_interaction_id
//             RETURNING id
//         `
// 		err = tx.QueryRow(ctx, itineraryQuery, interaction.UserID, cityID, interactionID).Scan(&itineraryID)
// 		if err != nil {
// 			span.RecordError(err)
// 			return interactionID, fmt.Errorf("failed to insert itinerary: %w", err)
// 		}
// 	}

// 	if itineraryID != uuid.Nil {
// 		var pois []types.POIDetail
// 		pois, err = parsePOIsFromResponse(interaction.ResponseText, r.logger)
// 		if err != nil {
// 			span.RecordError(err)
// 			return interactionID, fmt.Errorf("failed to parse POIs: %w", err)
// 		}

// 		if len(pois) > 0 {
// 			poiBatch := &pgx.Batch{}
// 			itineraryPoiInsertQuery := `
//                 INSERT INTO itinerary_pois (itinerary_id, poi_id, order_index, ai_description)
//                 VALUES ($1, $2, $3, $4)
//                 ON CONFLICT (itinerary_id, poi_id) DO UPDATE SET
//                     order_index = EXCLUDED.order_index,
//                     ai_description = EXCLUDED.ai_description,
//                     updated_at = NOW()
//             `
// 			for i, poiDetail := range pois {
// 				var poiDBID uuid.UUID
// 				poiDBID, err = r.GetOrCreatePOI(ctx, tx, poiDetail, cityID, interactionID)
// 				if err != nil {
// 					span.RecordError(err)
// 					return interactionID, fmt.Errorf("failed to get or create POI: %w", err)
// 				}
// 				poiBatch.Queue(itineraryPoiInsertQuery, itineraryID, poiDBID, i, poiDetail.DescriptionPOI)
// 			}

// 			if poiBatch.Len() > 0 {
// 				br := tx.SendBatch(ctx, poiBatch)
// 				for i := 0; i < poiBatch.Len(); i++ {
// 					_, execErr := br.Exec()
// 					if execErr != nil {
// 						err = fmt.Errorf("failed to insert itinerary_poi: %w", execErr)
// 						br.Close()
// 						return interactionID, err
// 					}
// 				}
// 				br.Close()
// 			}
// 		}
// 	}

// 	err = tx.Commit(ctx)
// 	if err != nil {
// 		span.RecordError(err)
// 		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
// 	}

// 	span.SetStatus(codes.Ok, "Interaction saved successfully")
// 	return interactionID, nil
// }

// // func (r *RepositoryImpl) CalculateDistancePostGIS(ctx context.Context, poi types.POIDetail, userLocation types.UserLocation) (float64, error) {
// // 	ctx, span := otel.Tracer("Repository").Start(ctx, "CalculateDistancePostGIS", trace.WithAttributes(
// // 		attribute.String("poi.name", poi.Name),
// // 		attribute.Float64("poi.latitude", poi.Latitude),
// // 		attribute.Float64("poi.longitude", poi.Longitude),
// // 		attribute.Float64("user.latitude", userLocation.UserLat),
// // 		attribute.Float64("user.longitude", userLocation.UserLon),
// // 	))
// // 	defer span.End()

// // 	// Validate coordinates
// // 	if poi.Latitude < -90 || poi.Latitude > 90 || poi.Longitude < -180 || poi.Longitude > 180 {
// // 		err := fmt.Errorf("invalid POI coordinates: lat=%f, lon=%f", poi.Latitude, poi.Longitude)
// // 		span.RecordError(err)
// // 		span.SetStatus(codes.Error, "Invalid POI coordinates")
// // 		return 0, err
// // 	}
// // 	if userLocation.UserLat < -90 || userLocation.UserLat > 90 || userLocation.UserLon < -180 || userLocation.UserLon > 180 {
// // 		err := fmt.Errorf("invalid user coordinates: lat=%f, lon=%f", userLocation.UserLat, userLocation.UserLon)
// // 		span.RecordError(err)
// // 		span.SetStatus(codes.Error, "Invalid user coordinates")
// // 		return 0, err
// // 	}

// // 	query := `
// //         SELECT ST_Distance(
// //             ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
// //             ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography
// //         ) AS distance
// //     `
// // 	var distance float64
// // 	err := r.pgpool.QueryRow(ctx, query, poi.Longitude, poi.Latitude, userLocation.UserLon, userLocation.UserLat).Scan(&distance)
// // 	if err != nil {
// // 		span.RecordError(err)
// // 		span.SetStatus(codes.Error, "Failed to calculate distance")
// // 		return 0, fmt.Errorf("failed to calculate distance: %w", err)
// // 	}

// // 	span.SetAttributes(attribute.Float64("distance.meters", distance))
// // 	span.SetStatus(codes.Ok, "Distance calculated successfully")
// // 	r.logger.Info("Distance calculated",
// // 		slog.String("poi.name", poi.Name),
// // 		slog.Float64("distance.meters", distance))
// // 	return distance, nil
// // }
