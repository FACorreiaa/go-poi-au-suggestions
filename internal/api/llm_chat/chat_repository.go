package llmChat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
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
	UpdateSession(ctx context.Context, session types.ChatSession) error
	AddMessageToSession(ctx context.Context, sessionID uuid.UUID, message types.ConversationMessage) error

	//
	SaveSinglePOI(ctx context.Context, poi types.POIDetail, userID, cityID uuid.UUID, llmInteractionID uuid.UUID) (uuid.UUID, error)
	GetPOIsBySessionSortedByDistance(ctx context.Context, sessionID, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetail, error)
	CalculateDistancePostGIS(ctx context.Context, userLat, userLon, poiLat, poiLon float64) (float64, error)
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

// func (r *RepositoryImpl) CalculateDistancePostGIS(ctx context.Context, poi types.POIDetail, userLocation types.UserLocation) (float64, error) {
// 	ctx, span := otel.Tracer("Repository").Start(ctx, "CalculateDistancePostGIS", trace.WithAttributes(
// 		attribute.String("poi.name", poi.Name),
// 		attribute.Float64("poi.latitude", poi.Latitude),
// 		attribute.Float64("poi.longitude", poi.Longitude),
// 		attribute.Float64("user.latitude", userLocation.UserLat),
// 		attribute.Float64("user.longitude", userLocation.UserLon),
// 	))
// 	defer span.End()

// 	// Validate coordinates
// 	if poi.Latitude < -90 || poi.Latitude > 90 || poi.Longitude < -180 || poi.Longitude > 180 {
// 		err := fmt.Errorf("invalid POI coordinates: lat=%f, lon=%f", poi.Latitude, poi.Longitude)
// 		span.RecordError(err)
// 		span.SetStatus(codes.Error, "Invalid POI coordinates")
// 		return 0, err
// 	}
// 	if userLocation.UserLat < -90 || userLocation.UserLat > 90 || userLocation.UserLon < -180 || userLocation.UserLon > 180 {
// 		err := fmt.Errorf("invalid user coordinates: lat=%f, lon=%f", userLocation.UserLat, userLocation.UserLon)
// 		span.RecordError(err)
// 		span.SetStatus(codes.Error, "Invalid user coordinates")
// 		return 0, err
// 	}

// 	query := `
//         SELECT ST_Distance(
//             ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
//             ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography
//         ) AS distance
//     `
// 	var distance float64
// 	err := r.pgpool.QueryRow(ctx, query, poi.Longitude, poi.Latitude, userLocation.UserLon, userLocation.UserLat).Scan(&distance)
// 	if err != nil {
// 		span.RecordError(err)
// 		span.SetStatus(codes.Error, "Failed to calculate distance")
// 		return 0, fmt.Errorf("failed to calculate distance: %w", err)
// 	}

// 	span.SetAttributes(attribute.Float64("distance.meters", distance))
// 	span.SetStatus(codes.Ok, "Distance calculated successfully")
// 	r.logger.Info("Distance calculated",
// 		slog.String("poi.name", poi.Name),
// 		slog.Float64("distance.meters", distance))
// 	return distance, nil
// }
