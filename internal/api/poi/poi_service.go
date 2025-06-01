package poi

import (
	"context"
	"log/slog"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"
)

var _ POIService = (*POIServiceImpl)(nil)

// LlmInteractiontService defines the business logic contract for user operations.
type POIService interface {
	AddPoiToFavourites(ctx context.Context, userID, poiID uuid.UUID) (uuid.UUID, error)
	RemovePoiFromFavourites(ctx context.Context, poiID uuid.UUID, userID uuid.UUID) error
	GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetail, error)
	GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetail, error)

	//SearchPOIs
	SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetail, error)
}

type POIServiceImpl struct {
	logger        *slog.Logger
	poiRepository POIRepository
}

func NewPOIServiceImpl(poiRepository POIRepository, logger *slog.Logger) *POIServiceImpl {
	return &POIServiceImpl{
		logger:        logger,
		poiRepository: poiRepository,
	}
}
func (s *POIServiceImpl) AddPoiToFavourites(ctx context.Context, userID, poiID uuid.UUID) (uuid.UUID, error) {
	poi, err := s.poiRepository.AddPoiToFavourites(ctx, userID, poiID)
	if err != nil {
		s.logger.Error("failed to add POI to favourites", "error", err)
		return uuid.Nil, err
	}
	return poi, nil
}
func (s *POIServiceImpl) RemovePoiFromFavourites(ctx context.Context, poiID uuid.UUID, userID uuid.UUID) error {
	if err := s.poiRepository.RemovePoiFromFavourites(ctx, poiID, userID); err != nil {
		s.logger.Error("failed to remove POI from favourites", "error", err)
		return err
	}

	return nil
}
func (s *POIServiceImpl) GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetail, error) {
	pois, err := s.poiRepository.GetFavouritePOIsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get favourite POIs by user ID", "error", err)
		return nil, err
	}
	return pois, nil
}
func (s *POIServiceImpl) GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetail, error) {
	pois, err := s.poiRepository.GetPOIsByCityID(ctx, cityID)
	if err != nil {
		s.logger.Error("failed to get POIs by city ID", "error", err)
		return nil, err
	}
	return pois, nil
}

func (s *POIServiceImpl) SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetail, error) {
	pois, err := s.poiRepository.SearchPOIs(ctx, filter)
	if err != nil {
		s.logger.Error("failed to search POIs", "error", err)
		return nil, err
	}
	return pois, nil
}
