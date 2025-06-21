package poi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/genai"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/city"
	generativeAI "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/generative_ai"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
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
	GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetailedInfo, error)
	GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetailedInfo, error)

	// Traditional search
	SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetailedInfo, error)

	// Semantic search methods
	SearchPOIsSemantic(ctx context.Context, query string, limit int) ([]types.POIDetailedInfo, error)
	SearchPOIsSemanticByCity(ctx context.Context, query string, cityID uuid.UUID, limit int) ([]types.POIDetailedInfo, error)
	SearchPOIsHybrid(ctx context.Context, filter types.POIFilter, query string, semanticWeight float64) ([]types.POIDetailedInfo, error)
	GenerateEmbeddingForPOI(ctx context.Context, poiID uuid.UUID) error
	GenerateEmbeddingsForAllPOIs(ctx context.Context, batchSize int) error

	// Itinerary management
	GetItinerary(ctx context.Context, userID, itineraryID uuid.UUID) (*types.UserSavedItinerary, error)
	GetItineraries(ctx context.Context, userID uuid.UUID, page, pageSize int) (*types.PaginatedUserItinerariesResponse, error)
	UpdateItinerary(ctx context.Context, userID, itineraryID uuid.UUID, updates types.UpdateItineraryRequest) (*types.UserSavedItinerary, error)

	// Discover Service
	GetGeneralPOIByDistance(ctx context.Context, userID uuid.UUID, lat, lon, distance float64) ([]types.POIDetailedInfo, error) //, categoryFilter string
	GetGeneralPOIByDistanceWithFilters(ctx context.Context, userID uuid.UUID, lat, lon, distance float64, filters map[string]string) ([]types.POIDetailedInfo, error)
	//filters types.POIFilters
}

type ServiceImpl struct {
	logger           *slog.Logger
	poiRepository    Repository
	embeddingService *generativeAI.EmbeddingService
	aiClient         *generativeAI.AIClient
	cityRepo         city.Repository
	cache            *cache.Cache
}

func NewServiceImpl(poiRepository Repository,
	embeddingService *generativeAI.EmbeddingService,
	cityRepo city.Repository,
	logger *slog.Logger) *ServiceImpl {

	aiClient, err := generativeAI.NewAIClient(context.Background())
	if err != nil {
		logger.Error("Failed to initialize AI client", slog.Any("error", err))
		// For now, set to nil and handle gracefully in methods
		aiClient = nil
	}

	return &ServiceImpl{
		logger:           logger,
		poiRepository:    poiRepository,
		aiClient:         aiClient,
		cityRepo:         cityRepo,
		cache:            cache.New(5*time.Minute, 10*time.Minute),
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
func (s *ServiceImpl) GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetailedInfo, error) {
	pois, err := s.poiRepository.GetFavouritePOIsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get favourite POIs by user ID", "error", err)
		return nil, err
	}
	return pois, nil
}
func (s *ServiceImpl) GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetailedInfo, error) {
	pois, err := s.poiRepository.GetPOIsByCityID(ctx, cityID)
	if err != nil {
		s.logger.Error("failed to get POIs by city ID", "error", err)
		return nil, err
	}
	return pois, nil
}

func (s *ServiceImpl) SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetailedInfo, error) {
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
func (s *ServiceImpl) SearchPOIsSemantic(ctx context.Context, query string, limit int) ([]types.POIDetailedInfo, error) {
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
func (s *ServiceImpl) SearchPOIsSemanticByCity(ctx context.Context, query string, cityID uuid.UUID, limit int) ([]types.POIDetailedInfo, error) {
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
func (s *ServiceImpl) SearchPOIsHybrid(ctx context.Context, filter types.POIFilter, query string, semanticWeight float64) ([]types.POIDetailedInfo, error) {
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

func (l *ServiceImpl) GetGeneralPOIByDistance(ctx context.Context, userID uuid.UUID, lat, lon, distance float64) ([]types.POIDetailedInfo, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetGeneralPOIByDistanceResponse")
	defer span.End()

	cacheKey := generateFilteredPOICacheKey(lat, lon, distance, userID)
	span.SetAttributes(attribute.String("cache.key", cacheKey))

	// Check cache first
	if cached, found := l.cache.Get(cacheKey); found {
		if pois, ok := cached.([]types.POIDetailedInfo); ok {
			l.logger.InfoContext(ctx, "Serving POIs from cache", "key", cacheKey)
			span.SetStatus(codes.Ok, "Served from cache")
			return pois, nil
		}
	}

	l.logger.InfoContext(ctx, "Querying POIs from database first",
		slog.Float64("lat", lat),
		slog.Float64("lon", lon),
		slog.Float64("distance_meters", distance))

	// First, try to get POIs from database using PostGIS
	pois, err := l.poiRepository.GetPOIsByLocationAndDistance(ctx, lat, lon, distance)
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to query POIs from database, will fallback to LLM", slog.Any("error", err))
	} else if len(pois) > 0 {
		l.logger.InfoContext(ctx, "Found POIs in database, using them instead of LLM", slog.Int("count", len(pois)))
		l.cache.Set(cacheKey, pois, cache.DefaultExpiration)
		span.SetStatus(codes.Ok, "Served from database")
		return pois, nil
	}

	// Fallback to LLM if database is empty or query failed
	l.logger.InfoContext(ctx, "No POIs found in database, falling back to LLM generation")

	// Generate via AI
	resultCh := make(chan types.GenAIResponse, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go l.getGeneralPOIByDistance(&wg, ctx, userID, lat, lon, distance, resultCh, &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0.7),
		MaxOutputTokens: 16384,
	})
	wg.Wait()
	close(resultCh)

	genAIResponse := <-resultCh
	if genAIResponse.Err != nil {
		span.RecordError(genAIResponse.Err)
		return nil, genAIResponse.Err
	}

	// Save all LLM-generated POIs to the database
	for _, p := range genAIResponse.GeneralPOI {
		poiID := p.ID
		if poiID == uuid.Nil { // uuid.Nil is "00000000-0000-0000-0000-000000000000"
			poiID = uuid.New()
		}
		cityID, cityName, err := l.cityRepo.GetCity(ctx, p.Latitude, p.Longitude)

		if err != nil {
			l.logger.WarnContext(ctx, "Failed to determine city ID for POI, attempting to create city",
				slog.Any("error", err),
				slog.String("poi_name", p.Name))

			// Create a city using the data from LLM response
			if genAIResponse.City != "" && genAIResponse.Country != "" {
				cityDetail := types.CityDetail{
					Name:            genAIResponse.City,
					Country:         genAIResponse.Country,
					StateProvince:   genAIResponse.StateProvince,
					AiSummary:       genAIResponse.CityDescription,
					CenterLatitude:  genAIResponse.Latitude,
					CenterLongitude: genAIResponse.Longitude,
				}

				// If we don't have city center coordinates from LLM, use the POI coordinates as approximation
				if cityDetail.CenterLatitude == 0 && cityDetail.CenterLongitude == 0 {
					cityDetail.CenterLatitude = p.Latitude
					cityDetail.CenterLongitude = p.Longitude
				}

				createdCityID, cityErr := l.cityRepo.SaveCity(ctx, cityDetail)
				if cityErr != nil {
					l.logger.ErrorContext(ctx, "Failed to create city for POI",
						slog.Any("error", cityErr),
						slog.String("poi_name", p.Name),
						slog.String("city_name", genAIResponse.City))
					continue
				}

				cityID = createdCityID
				cityName = genAIResponse.City
				l.logger.InfoContext(ctx, "Created new city for POI",
					slog.String("city_id", cityID.String()),
					slog.String("city_name", cityName),
					slog.String("poi_name", p.Name))
			} else {
				l.logger.ErrorContext(ctx, "Cannot create city for POI - missing city data from LLM",
					slog.String("poi_name", p.Name))
				continue
			}
		}

		poi := types.POIDetailedInfo{
			ID:        poiID,
			City:      cityName,
			CityID:    cityID,
			Name:      p.Name,
			Latitude:  p.Latitude,
			Longitude: p.Longitude,
			Category:  p.Category,
			// Note: Distance is not saved as it’s query-specific
		}
		_, err = l.poiRepository.SavePOIDetails(ctx, poi, uuid.NullUUID{UUID: cityID, Valid: cityID != uuid.Nil})
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to save POI",
				slog.Any("error", err),
				slog.String("poi_name", poi.Name))
		}
	}

	// Filter LLM-generated POIs for the current response
	var poisDetailed []types.POIDetailedInfo

	for _, p := range genAIResponse.GeneralPOI {
		distanceKm := calculateDistance(lat, lon, p.Latitude, p.Longitude)
		if distanceKm <= distance/1000 { // Convert meters to km for comparison
			poi := types.POIDetailedInfo{
				ID:        p.ID,
				Name:      p.Name,
				Latitude:  p.Latitude,
				Longitude: p.Longitude,
				Category:  p.Category,
				Distance:  distanceKm,
			}
			poisDetailed = append(poisDetailed, poi)
		}
	}

	l.logger.InfoContext(ctx, "Generated and filtered POIs using LLM", slog.Int("count", len(poisDetailed)))

	l.cache.Set(cacheKey, poisDetailed, cache.DefaultExpiration)
	span.SetStatus(codes.Ok, "POIs generated via LLM and cached")
	return poisDetailed, nil
}

func (l *ServiceImpl) GetGeneralPOIByDistanceWithFilters(ctx context.Context, userID uuid.UUID, lat, lon, distance float64, filters map[string]string) ([]types.POIDetailedInfo, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GetGeneralPOIByDistanceWithFilters")
	defer span.End()

	// 1. Build a cache key that includes all parameters.
	cacheKey := generateFilteredPOICacheKeyWithFilters(lat, lon, distance, filters, userID)
	span.SetAttributes(attribute.String("cache.key", cacheKey))

	// 2. Check cache first.
	if cached, found := l.cache.Get(cacheKey); found {
		if pois, ok := cached.([]types.POIDetailedInfo); ok {
			l.logger.InfoContext(ctx, "Serving POIs from cache", "key", cacheKey, "count", len(pois))
			return pois, nil
		}
	}

	// 3. Check the database. This is the primary source of truth.
	l.logger.InfoContext(ctx, "Cache miss. Querying database with filters.", "lat", lat, "lon", lon, "distance_m", distance)
	poisFromDB, err := l.poiRepository.GetPOIsByLocationAndDistanceWithFilters(ctx, lat, lon, distance, filters)
	if err != nil {
		l.logger.WarnContext(ctx, "Database query failed, will fall back to LLM", slog.Any("error", err))
	} else if len(poisFromDB) > 0 {
		l.logger.InfoContext(ctx, "Found POIs in database", "count", len(poisFromDB))
		l.cache.Set(cacheKey, poisFromDB, cache.DefaultExpiration)
		return poisFromDB, nil
	}

	// --- LLM FALLBACK ---
	l.logger.InfoContext(ctx, "No POIs found in database, falling back to LLM generation")
	span.AddEvent("database_miss_fallback_to_llm")

	// 4. Generate data via AI.
	// NOTE: The LLM prompt is simplified. It doesn't need to handle filters,
	// as we will filter the results in our Go code. It just needs to find places.
	genAIResponse, err := l.generatePOIsWithLLM(ctx, userID, lat, lon, distance)
	if err != nil {
		return nil, err
	}

	// 5. Enrich the raw LLM data into a single, canonical list.
	enrichedPOIs, city, err := l.enrichLLMPOIsWithMetadata(ctx, genAIResponse.GeneralPOI, lat, lon, distance)
	if err != nil {
		// If enrichment fails, we cannot proceed reliably.
		return nil, fmt.Errorf("failed to enrich LLM data: %w", err)
	}

	// 6. NOW, apply the filters to the enriched, LLM-generated data.
	filteredLLMPOIs := applyClientFilters(enrichedPOIs, filters)
	l.logger.InfoContext(ctx, "Generated POIs via LLM and applied filters", "initial_count", len(enrichedPOIs), "final_count", len(filteredLLMPOIs))

	// 7. Asynchronously save the *enriched* (but unfiltered) POIs to the database.
	if city != uuid.Nil && len(enrichedPOIs) > 0 {
		go func() {
			// Use context with timeout instead of background context
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			l.logger.InfoContext(bgCtx, "Starting background save of LLM-generated POIs",
				slog.Int("count", len(enrichedPOIs)),
				slog.String("city_id", city.String()))

			savedCount := 0
			failedCount := 0

			for i, poi := range enrichedPOIs {
				select {
				case <-bgCtx.Done():
					l.logger.WarnContext(bgCtx, "Background save cancelled due to timeout",
						slog.Int("saved", savedCount),
						slog.Int("remaining", len(enrichedPOIs)-i))
					return
				default:
				}

				if poiID, saveErr := l.poiRepository.SavePOIDetails(bgCtx, poi, uuid.NullUUID{UUID: city, Valid: city != uuid.Nil}); saveErr != nil {
					failedCount++
					l.logger.ErrorContext(bgCtx, "Failed to save LLM-generated POI",
						slog.Any("error", saveErr),
						slog.String("poi_name", poi.Name),
						slog.String("poi_id", poi.ID.String()),
						slog.Float64("latitude", poi.Latitude),
						slog.Float64("longitude", poi.Longitude))
				} else {
					savedCount++
					l.logger.DebugContext(bgCtx, "Successfully saved POI",
						slog.String("poi_name", poi.Name),
						slog.String("poi_id", poiID.String()))
				}
			}

			l.logger.InfoContext(bgCtx, "Finished background save of POIs",
				slog.Int("saved", savedCount),
				slog.Int("failed", failedCount),
				slog.Int("total", len(enrichedPOIs)))
		}()
	} else {
		l.logger.WarnContext(ctx, "Skipping POI save - invalid city ID or no POIs",
			slog.String("city_id", city.String()),
			slog.Int("poi_count", len(enrichedPOIs)))
	}

	// 8. Cache and return the **filtered** list to the user for this specific request.
	l.cache.Set(cacheKey, filteredLLMPOIs, cache.DefaultExpiration)
	return filteredLLMPOIs, nil
}

func (l *ServiceImpl) generatePOIsWithLLM(ctx context.Context, userID uuid.UUID, lat, lon, distance float64) (*types.GenAIResponse, error) {
	resultCh := make(chan types.GenAIResponse, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go l.getGeneralPOIByDistance(&wg, ctx, userID, lat, lon, distance, resultCh, &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0.7),
		MaxOutputTokens: 8192,
	})

	wg.Wait()
	close(resultCh)

	// Receive the single value from the channel
	result := <-resultCh

	// The received struct might contain an error from the goroutine, so check it.
	if result.Err != nil {
		return nil, result.Err
	}

	// Return a pointer to the result struct and a nil error.
	return &result, nil
}

func (l *ServiceImpl) getGeneralPOIByDistance(wg *sync.WaitGroup,
	ctx context.Context,
	userID uuid.UUID,
	lat, lon, distance float64,
	resultCh chan<- types.GenAIResponse,
	config *genai.GenerateContentConfig) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "GenerateGeneralPOIWorker", trace.WithAttributes(
		attribute.Float64("latitude", lat),
		attribute.Float64("longitude", lon),
		attribute.Float64("distance.km", distance),
		attribute.String("user.id", userID.String())))

	defer span.End()
	defer wg.Done()

	prompt := getGeneralPOIByDistance(lat, lon, distance)
	span.SetAttributes(attribute.Int("prompt.length", len(prompt)))

	if l.aiClient == nil {
		err := fmt.Errorf("AI client is not available - check API key configuration")
		span.RecordError(err)
		span.SetStatus(codes.Error, "AI client unavailable")
		resultCh <- types.GenAIResponse{Err: err}
		return
	}

	startTime := time.Now()
	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
	latencyMs := int(time.Since(startTime).Milliseconds())
	span.SetAttributes(attribute.Int("response.latency_ms", latencyMs))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate general POIs")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to generate general POIs: %w", err)}
		return
	}

	var txt string
	for _, candidate := range response.Candidates {
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			txt = candidate.Content.Parts[0].Text
			break
		}
	}
	if txt == "" {
		err := fmt.Errorf("no valid general POI content from AI")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Empty response from AI")
		resultCh <- types.GenAIResponse{Err: err}
		return
	}
	span.SetAttributes(attribute.Int("response.length", len(txt)))

	cleanTxt := cleanJSONResponse(txt)
	var poiData struct {
		PointsOfInterest []types.POIDetailedInfo `json:"points_of_interest"`
	}
	if err := json.Unmarshal([]byte(cleanTxt), &poiData); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to parse general POI JSON")
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse general POI JSON: %w", err)}
		return
	}

	fmt.Println(cleanTxt)

	span.SetAttributes(attribute.Int("pois.count", len(poiData.PointsOfInterest)))
	span.SetStatus(codes.Ok, "General POIs generated successfully")
	resultCh <- types.GenAIResponse{GeneralPOI: poiData.PointsOfInterest}
}

func (l *ServiceImpl) enrichLLMPOIsWithMetadata(ctx context.Context, pois []types.POIDetailedInfo, userLat, userLon, searchRadiusMeters float64) ([]types.POIDetailedInfo, uuid.UUID, error) {
	var enrichedPOIs []types.POIDetailedInfo

	// For discover endpoint, city resolution is optional since large radius searches cross multiple cities
	// POIs will be saved with NULL city_id, relying on coordinate-based storage instead
	l.logger.InfoContext(ctx, "Processing POIs without city constraint for coordinate-based discovery",
		slog.Float64("lat", userLat),
		slog.Float64("lon", userLon),
		slog.Float64("radius_meters", searchRadiusMeters))

	for _, p := range pois {
		// The LLM can be inaccurate, so we re-calculate distance using our reliable PostGIS.
		distanceMeters, err := l.poiRepository.CalculateDistancePostGIS(ctx, userLat, userLon, p.Latitude, p.Longitude)
		if err != nil {
			l.logger.WarnContext(ctx, "Could not calculate distance for POI, skipping distance field", "poi_name", p.Name, "error", err)
		}

		// Filter out POIs that the LLM returned but are outside the user's requested radius.
		if distanceMeters > searchRadiusMeters {
			continue
		}

		enrichedPOI := p // Copy all original fields

		// Ensure we have an ID
		if enrichedPOI.ID == uuid.Nil {
			enrichedPOI.ID = uuid.New()
		}

		// For coordinate-based discovery, keep city info as empty/nil
		enrichedPOI.City = ""
		enrichedPOI.CityID = uuid.Nil
		enrichedPOI.Distance = distanceMeters

		// Fix description field mapping - use Description instead of DescriptionPOI
		if enrichedPOI.Description == "" && p.DescriptionPOI != "" {
			enrichedPOI.Description = p.DescriptionPOI
		}

		enrichedPOIs = append(enrichedPOIs, enrichedPOI)
	}

	return enrichedPOIs, uuid.Nil, nil
}
