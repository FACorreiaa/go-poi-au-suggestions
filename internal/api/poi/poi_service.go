package poi

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var _ Service = (*ServiceImpl)(nil)

// LlmInteractiontService defines the business logic contract for user operations.
type Service interface {
	AddPoiToFavourites(ctx context.Context, userID, poiID uuid.UUID) (uuid.UUID, error)
	RemovePoiFromFavourites(ctx context.Context, poiID uuid.UUID, userID uuid.UUID) error
	GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetail, error)
	GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetail, error)

	//SearchPOIs
	SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetail, error)

	//
	GetItinerary(ctx context.Context, userID, itineraryID uuid.UUID) (*types.UserSavedItinerary, error)
	GetItineraries(ctx context.Context, userID uuid.UUID, page, pageSize int) (*types.PaginatedUserItinerariesResponse, error)
	UpdateItinerary(ctx context.Context, userID, itineraryID uuid.UUID, updates types.UpdateItineraryRequest) (*types.UserSavedItinerary, error)
}

type ServiceImpl struct {
	logger        *slog.Logger
	poiRepository Repository
}

func NewServiceImpl(poiRepository Repository, logger *slog.Logger) *ServiceImpl {
	return &ServiceImpl{
		logger:        logger,
		poiRepository: poiRepository,
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
