package poi

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/generative_ai"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var _ Service = (*ServiceImpl)(nil)

// Service defines the business logic contract for POI operations.
type Service interface {
	AddPoiToFavourites(ctx context.Context, userID, poiID uuid.UUID) (uuid.UUID, error)
	RemovePoiFromFavourites(ctx context.Context, poiID uuid.UUID, userID uuid.UUID) error
	GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetail, error)
	GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetail, error)

	// Traditional search
	SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetail, error)

	// Semantic search methods
	SearchPOIsSemantic(ctx context.Context, query string, limit int) ([]types.POIDetail, error)
	SearchPOIsSemanticByCity(ctx context.Context, query string, cityID uuid.UUID, limit int) ([]types.POIDetail, error)
	SearchPOIsHybrid(ctx context.Context, filter types.POIFilter, query string, semanticWeight float64) ([]types.POIDetail, error)
	GenerateEmbeddingForPOI(ctx context.Context, poiID uuid.UUID) error
	GenerateEmbeddingsForAllPOIs(ctx context.Context, batchSize int) error

	// Itinerary management
	GetItinerary(ctx context.Context, userID, itineraryID uuid.UUID) (*types.UserSavedItinerary, error)
	GetItineraries(ctx context.Context, userID uuid.UUID, page, pageSize int) (*types.PaginatedUserItinerariesResponse, error)
	UpdateItinerary(ctx context.Context, userID, itineraryID uuid.UUID, updates types.UpdateItineraryRequest) (*types.UserSavedItinerary, error)
}

type ServiceImpl struct {
	logger           *slog.Logger
	poiRepository    Repository
	embeddingService *generativeAI.EmbeddingService
}

func NewServiceImpl(poiRepository Repository, embeddingService *generativeAI.EmbeddingService, logger *slog.Logger) *ServiceImpl {
	return &ServiceImpl{
		logger:           logger,
		poiRepository:    poiRepository,
		embeddingService: embeddingService,
	}
}
func (s *ServiceImpl) AddPoiToFavourites(ctx context.Context, userID, poiID uuid.UUID) (uuid.UUID, error) {
	poi, err := s.poiRepository.AddPoiToFavourites(ctx, userID, poiID)
	if err != nil {
		s.logger.Error("failed to add POI to favourites", "error", err)
		return uuid.Nil, err
	}
	return poi, nil
}
func (s *ServiceImpl) RemovePoiFromFavourites(ctx context.Context, poiID uuid.UUID, userID uuid.UUID) error {
	if err := s.poiRepository.RemovePoiFromFavourites(ctx, poiID, userID); err != nil {
		s.logger.Error("failed to remove POI from favourites", "error", err)
		return err
	}

	return nil
}
func (s *ServiceImpl) GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetail, error) {
	pois, err := s.poiRepository.GetFavouritePOIsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get favourite POIs by user ID", "error", err)
		return nil, err
	}
	return pois, nil
}
func (s *ServiceImpl) GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetail, error) {
	pois, err := s.poiRepository.GetPOIsByCityID(ctx, cityID)
	if err != nil {
		s.logger.Error("failed to get POIs by city ID", "error", err)
		return nil, err
	}
	return pois, nil
}

func (s *ServiceImpl) SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetail, error) {
	pois, err := s.poiRepository.SearchPOIs(ctx, filter)
	if err != nil {
		s.logger.Error("failed to search POIs", "error", err)
		return nil, err
	}
	return pois, nil
}

func (l *ServiceImpl) GetItinerary(ctx context.Context, userID, itineraryID uuid.UUID) (*types.UserSavedItinerary, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetItinerary")
	defer span.End()

	itinerary, err := l.poiRepository.GetItinerary(ctx, userID, itineraryID)
	if err != nil {
		l.logger.ErrorContext(ctx, "Repository failed to get itinerary", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get itinerary: %w", err)
	}
	if itinerary == nil {
		return nil, fmt.Errorf("itinerary not found")
	}

	span.SetStatus(codes.Ok, "Itinerary retrieved successfully")
	return itinerary, nil
}

func (l *ServiceImpl) GetItineraries(ctx context.Context, userID uuid.UUID, page, pageSize int) (*types.PaginatedUserItinerariesResponse, error) {
	_, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetItineraries", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.Int("page", page),
		attribute.Int("page_size", pageSize),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Service: Getting itineraries for user", slog.String("userID", userID.String()))

	if page <= 0 {
		page = 1 // Default to page 1
	}
	if pageSize <= 0 {
		pageSize = 10 // Default page size
	}

	itineraries, totalRecords, err := l.poiRepository.GetItineraries(ctx, userID, page, pageSize)
	if err != nil {
		l.logger.ErrorContext(ctx, "Repository failed to get itineraries", slog.Any("error", err))
		span.RecordError(err)
		return nil, fmt.Errorf("failed to retrieve itineraries: %w", err)
	}

	span.SetAttributes(attribute.Int("itineraries.count", len(itineraries)), attribute.Int("total_records", totalRecords))
	span.SetStatus(codes.Ok, "Itineraries retrieved")

	return &types.PaginatedUserItinerariesResponse{
		Itineraries:  itineraries,
		TotalRecords: totalRecords,
		Page:         page,
		PageSize:     pageSize,
	}, nil
}

func (l *ServiceImpl) UpdateItinerary(ctx context.Context, userID, itineraryID uuid.UUID, updates types.UpdateItineraryRequest) (*types.UserSavedItinerary, error) {
	_, span := otel.Tracer("LlmInteractionService").Start(ctx, "UpdateItinerary", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("itinerary.id", itineraryID.String()),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Service: Updating itinerary", slog.String("userID", userID.String()), slog.String("itineraryID", itineraryID.String()), slog.Any("updates", updates))

	if updates.Title == nil && updates.Description == nil && updates.Tags == nil &&
		updates.EstimatedDurationDays == nil && updates.EstimatedCostLevel == nil &&
		updates.IsPublic == nil && updates.MarkdownContent == nil {
		span.AddEvent("No update fields provided.")
		l.logger.InfoContext(ctx, "No fields provided for itinerary update, fetching current.", slog.String("itineraryID", itineraryID.String()))
		return l.poiRepository.GetItinerary(ctx, userID, itineraryID) // Assumes GetItinerary checks ownership
	}

	updatedItinerary, err := l.poiRepository.UpdateItinerary(ctx, userID, itineraryID, updates)
	if err != nil {
		l.logger.ErrorContext(ctx, "Repository failed to update itinerary", slog.Any("error", err))
		span.RecordError(err)
		return nil, err // Propagate error (could be not found, or DB error)
	}

	span.SetStatus(codes.Ok, "Itinerary updated")
	return updatedItinerary, nil
}

// SearchPOIsSemantic performs semantic search for POIs using natural language queries
func (s *ServiceImpl) SearchPOIsSemantic(ctx context.Context, query string, limit int) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("POIService").Start(ctx, "SearchPOIsSemantic", trace.WithAttributes(
		attribute.String("query", query),
		attribute.Int("limit", limit),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "SearchPOIsSemantic"))

	if s.embeddingService == nil {
		err := fmt.Errorf("embedding service not available")
		l.ErrorContext(ctx, "Embedding service not initialized", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Embedding service not available")
		return nil, err
	}

	// Generate embedding for the query
	queryEmbedding, err := s.embeddingService.GenerateQueryEmbedding(ctx, query)
	if err != nil {
		l.ErrorContext(ctx, "Failed to generate query embedding", 
			slog.Any("error", err),
			slog.String("query", query))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate query embedding")
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search for similar POIs
	pois, err := s.poiRepository.FindSimilarPOIs(ctx, queryEmbedding, limit)
	if err != nil {
		l.ErrorContext(ctx, "Failed to find similar POIs", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to find similar POIs")
		return nil, fmt.Errorf("failed to find similar POIs: %w", err)
	}

	l.InfoContext(ctx, "Semantic search completed", 
		slog.String("query", query),
		slog.Int("results", len(pois)))
	span.SetAttributes(
		attribute.String("query", query),
		attribute.Int("results.count", len(pois)),
	)
	span.SetStatus(codes.Ok, "Semantic search completed")

	return pois, nil
}

// SearchPOIsSemanticByCity performs semantic search for POIs within a specific city
func (s *ServiceImpl) SearchPOIsSemanticByCity(ctx context.Context, query string, cityID uuid.UUID, limit int) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("POIService").Start(ctx, "SearchPOIsSemanticByCity", trace.WithAttributes(
		attribute.String("query", query),
		attribute.String("city.id", cityID.String()),
		attribute.Int("limit", limit),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "SearchPOIsSemanticByCity"))

	if s.embeddingService == nil {
		err := fmt.Errorf("embedding service not available")
		l.ErrorContext(ctx, "Embedding service not initialized", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Embedding service not available")
		return nil, err
	}

	// Generate embedding for the query
	queryEmbedding, err := s.embeddingService.GenerateQueryEmbedding(ctx, query)
	if err != nil {
		l.ErrorContext(ctx, "Failed to generate query embedding", 
			slog.Any("error", err),
			slog.String("query", query))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate query embedding")
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search for similar POIs in the specified city
	pois, err := s.poiRepository.FindSimilarPOIsByCity(ctx, queryEmbedding, cityID, limit)
	if err != nil {
		l.ErrorContext(ctx, "Failed to find similar POIs by city", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to find similar POIs by city")
		return nil, fmt.Errorf("failed to find similar POIs by city: %w", err)
	}

	l.InfoContext(ctx, "Semantic search by city completed", 
		slog.String("query", query),
		slog.String("city_id", cityID.String()),
		slog.Int("results", len(pois)))
	span.SetAttributes(
		attribute.String("query", query),
		attribute.String("city.id", cityID.String()),
		attribute.Int("results.count", len(pois)),
	)
	span.SetStatus(codes.Ok, "Semantic search by city completed")

	return pois, nil
}

// SearchPOIsHybrid performs hybrid search combining spatial and semantic similarity
func (s *ServiceImpl) SearchPOIsHybrid(ctx context.Context, filter types.POIFilter, query string, semanticWeight float64) ([]types.POIDetail, error) {
	ctx, span := otel.Tracer("POIService").Start(ctx, "SearchPOIsHybrid", trace.WithAttributes(
		attribute.String("query", query),
		attribute.Float64("semantic.weight", semanticWeight),
		attribute.Float64("location.latitude", filter.Location.Latitude),
		attribute.Float64("location.longitude", filter.Location.Longitude),
		attribute.Float64("radius", filter.Radius),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "SearchPOIsHybrid"))

	if s.embeddingService == nil {
		err := fmt.Errorf("embedding service not available")
		l.ErrorContext(ctx, "Embedding service not initialized", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Embedding service not available")
		return nil, err
	}

	// Validate semantic weight
	if semanticWeight < 0 || semanticWeight > 1 {
		err := fmt.Errorf("semantic weight must be between 0 and 1, got: %f", semanticWeight)
		l.ErrorContext(ctx, "Invalid semantic weight", slog.Float64("semantic_weight", semanticWeight))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid semantic weight")
		return nil, err
	}

	// Generate embedding for the query
	queryEmbedding, err := s.embeddingService.GenerateQueryEmbedding(ctx, query)
	if err != nil {
		l.ErrorContext(ctx, "Failed to generate query embedding", 
			slog.Any("error", err),
			slog.String("query", query))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate query embedding")
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Perform hybrid search
	pois, err := s.poiRepository.SearchPOIsHybrid(ctx, filter, queryEmbedding, semanticWeight)
	if err != nil {
		l.ErrorContext(ctx, "Failed to perform hybrid search", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to perform hybrid search")
		return nil, fmt.Errorf("failed to perform hybrid search: %w", err)
	}

	l.InfoContext(ctx, "Hybrid search completed", 
		slog.String("query", query),
		slog.Float64("semantic_weight", semanticWeight),
		slog.Int("results", len(pois)))
	span.SetAttributes(
		attribute.String("query", query),
		attribute.Float64("semantic.weight", semanticWeight),
		attribute.Int("results.count", len(pois)),
	)
	span.SetStatus(codes.Ok, "Hybrid search completed")

	return pois, nil
}

// GenerateEmbeddingForPOI generates and stores embedding for a specific POI
func (s *ServiceImpl) GenerateEmbeddingForPOI(ctx context.Context, poiID uuid.UUID) error {
	ctx, span := otel.Tracer("POIService").Start(ctx, "GenerateEmbeddingForPOI", trace.WithAttributes(
		attribute.String("poi.id", poiID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GenerateEmbeddingForPOI"))

	if s.embeddingService == nil {
		err := fmt.Errorf("embedding service not available")
		l.ErrorContext(ctx, "Embedding service not initialized", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Embedding service not available")
		return err
	}

	// Get POI details to generate embedding
	pois, err := s.poiRepository.GetPOIsWithoutEmbeddings(ctx, 1)
	if err != nil {
		l.ErrorContext(ctx, "Failed to get POI details", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get POI details")
		return fmt.Errorf("failed to get POI details: %w", err)
	}

	if len(pois) == 0 {
		l.InfoContext(ctx, "No POI found for embedding generation", slog.String("poi_id", poiID.String()))
		span.SetStatus(codes.Ok, "No POI found")
		return fmt.Errorf("POI not found or already has embedding")
	}

	poi := pois[0]

	// Generate embedding using POI information
	embedding, err := s.embeddingService.GeneratePOIEmbedding(ctx, poi.Name, poi.DescriptionPOI, poi.Category)
	if err != nil {
		l.ErrorContext(ctx, "Failed to generate POI embedding", 
			slog.Any("error", err),
			slog.String("poi_id", poiID.String()))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate POI embedding")
		return fmt.Errorf("failed to generate POI embedding: %w", err)
	}

	// Update POI with generated embedding
	err = s.poiRepository.UpdatePOIEmbedding(ctx, poiID, embedding)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update POI embedding", 
			slog.Any("error", err),
			slog.String("poi_id", poiID.String()))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update POI embedding")
		return fmt.Errorf("failed to update POI embedding: %w", err)
	}

	l.InfoContext(ctx, "POI embedding generated and stored successfully", 
		slog.String("poi_id", poiID.String()),
		slog.String("poi_name", poi.Name))
	span.SetAttributes(
		attribute.String("poi.id", poiID.String()),
		attribute.String("poi.name", poi.Name),
	)
	span.SetStatus(codes.Ok, "POI embedding generated")

	return nil
}

// GenerateEmbeddingsForAllPOIs generates embeddings for all POIs that don't have them
func (s *ServiceImpl) GenerateEmbeddingsForAllPOIs(ctx context.Context, batchSize int) error {
	ctx, span := otel.Tracer("POIService").Start(ctx, "GenerateEmbeddingsForAllPOIs", trace.WithAttributes(
		attribute.Int("batch.size", batchSize),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GenerateEmbeddingsForAllPOIs"))

	if s.embeddingService == nil {
		err := fmt.Errorf("embedding service not available")
		l.ErrorContext(ctx, "Embedding service not initialized", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Embedding service not available")
		return err
	}

	if batchSize <= 0 {
		batchSize = 10 // Default batch size
	}

	totalProcessed := 0
	totalErrors := 0

	for {
		// Get batch of POIs without embeddings
		pois, err := s.poiRepository.GetPOIsWithoutEmbeddings(ctx, batchSize)
		if err != nil {
			l.ErrorContext(ctx, "Failed to get POIs without embeddings", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to get POIs without embeddings")
			return fmt.Errorf("failed to get POIs without embeddings: %w", err)
		}

		if len(pois) == 0 {
			// No more POIs to process
			break
		}

		l.InfoContext(ctx, "Processing batch of POIs", slog.Int("batch_size", len(pois)))

		// Process each POI in the batch
		for _, poi := range pois {
			// Generate embedding
			embedding, err := s.embeddingService.GeneratePOIEmbedding(ctx, poi.Name, poi.DescriptionPOI, poi.Category)
			if err != nil {
				l.ErrorContext(ctx, "Failed to generate embedding for POI", 
					slog.Any("error", err),
					slog.String("poi_id", poi.ID.String()),
					slog.String("poi_name", poi.Name))
				totalErrors++
				continue
			}

			// Update POI with embedding
			err = s.poiRepository.UpdatePOIEmbedding(ctx, poi.ID, embedding)
			if err != nil {
				l.ErrorContext(ctx, "Failed to update POI embedding", 
					slog.Any("error", err),
					slog.String("poi_id", poi.ID.String()),
					slog.String("poi_name", poi.Name))
				totalErrors++
				continue
			}

			totalProcessed++
			l.DebugContext(ctx, "POI embedding generated successfully", 
				slog.String("poi_id", poi.ID.String()),
				slog.String("poi_name", poi.Name))
		}

		// Break if we processed fewer POIs than the batch size (end of data)
		if len(pois) < batchSize {
			break
		}
	}

	l.InfoContext(ctx, "Batch embedding generation completed", 
		slog.Int("total_processed", totalProcessed),
		slog.Int("total_errors", totalErrors))
	span.SetAttributes(
		attribute.Int("total.processed", totalProcessed),
		attribute.Int("total.errors", totalErrors),
	)

	if totalErrors > 0 {
		span.SetStatus(codes.Error, fmt.Sprintf("Completed with %d errors", totalErrors))
		return fmt.Errorf("embedding generation completed with %d errors out of %d total POIs", totalErrors, totalProcessed+totalErrors)
	}

	span.SetStatus(codes.Ok, "All POI embeddings generated successfully")
	return nil
}
