package poi

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types" // Ensure this path is correct
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockPOIRepository is a mock implementation of POIRepository
type MockPOIRepository struct {
	mock.Mock
}

func (m *MockPOIRepository) AddPoiToFavourites(ctx context.Context, userID, poiID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, userID, poiID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockPOIRepository) RemovePoiFromFavourites(ctx context.Context, poiID uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, poiID, userID)
	return args.Error(0)
}

func (m *MockPOIRepository) GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetail, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetail), args.Error(1)
}

func (m *MockPOIRepository) GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetail, error) {
	args := m.Called(ctx, cityID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetail), args.Error(1)
}

func (m *MockPOIRepository) SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetail, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetail), args.Error(1)
}

func (m *MockPOIRepository) FindHotelDetails(ctx context.Context, cityID uuid.UUID, lat, lon, tolerance float64) ([]types.HotelDetailedInfo, error) {
	args := m.Called(ctx, cityID, lat, lon, tolerance)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.HotelDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) FindPOIDetails(ctx context.Context, cityID uuid.UUID, lat, lon float64, tolerance float64) (*types.POIDetailedInfo, error) {
	args := m.Called(ctx, cityID, lat, lon, tolerance)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) FindPoiByNameAndCity(ctx context.Context, name string, cityID uuid.UUID) (*types.POIDetail, error) {
	args := m.Called(ctx, name, cityID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.POIDetail), args.Error(1)
}

func (m *MockPOIRepository) GetPOIsByIDs(ctx context.Context, poiIDs []uuid.UUID) ([]types.POIDetail, error) {
	args := m.Called(ctx, poiIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetail), args.Error(1)
}
func (m *MockPOIRepository) GetPOIsByIDsWithDetails(ctx context.Context, poiIDs []uuid.UUID) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, poiIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}
func (m *MockPOIRepository) GetPOIsByIDsWithDetailsAndCity(ctx context.Context, poiIDs []uuid.UUID, cityID uuid.UUID) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, poiIDs, cityID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) FindRestaurantDetails(ctx context.Context, cityID uuid.UUID, lat, lon, tolerance float64, preferences *types.RestaurantUserPreferences) ([]types.RestaurantDetailedInfo, error) {
	args := m.Called(ctx, cityID, lat, lon, tolerance, preferences)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.RestaurantDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) FindRestaurantDetailsByIDs(ctx context.Context, restaurantIDs []uuid.UUID) ([]types.RestaurantDetailedInfo, error) {
	args := m.Called(ctx, restaurantIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.RestaurantDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) GetHotelByID(ctx context.Context, hotelID uuid.UUID) (*types.HotelDetailedInfo, error) {
	args := m.Called(ctx, hotelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.HotelDetailedInfo), args.Error(1)
}

func (r *MockPOIRepository) GetPOIsByCityAndDistance(ctx context.Context, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetailedInfo, error) {
	args := r.Called(ctx, cityID, userLocation)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (r *MockPOIRepository) GetRestaurantByID(ctx context.Context, restaurantID uuid.UUID) (*types.RestaurantDetailedInfo, error) {
	args := r.Called(ctx, restaurantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.RestaurantDetailedInfo), args.Error(1)
}

func (r *MockPOIRepository) SaveHotelDetails(ctx context.Context, hotel types.HotelDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	args := r.Called(ctx, hotel, cityID)
	if args.Get(0) == nil {
		return uuid.Nil, args.Error(1)
	}
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (r *MockPOIRepository) SavePOIDetails(ctx context.Context, poi types.POIDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	args := r.Called(ctx, poi, cityID)
	if args.Get(0) == nil {
		return uuid.Nil, args.Error(1)
	}
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (r *MockPOIRepository) SavePoi(ctx context.Context, poi types.POIDetail, cityID uuid.UUID) (uuid.UUID, error) {
	args := r.Called(ctx, poi, cityID)
	if args.Get(0) == nil {
		return uuid.Nil, args.Error(1)
	}
	return args.Get(0).(uuid.UUID), args.Error(1)
}

// SaveRestaurantDetails
func (r *MockPOIRepository) SaveRestaurantDetails(ctx context.Context, restaurant types.RestaurantDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	args := r.Called(ctx, restaurant, cityID)
	if args.Get(0) == nil {
		return uuid.Nil, args.Error(1)
	}
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockPOIRepository) GetItinerary(ctx context.Context, userID, itineraryID uuid.UUID) (*types.UserSavedItinerary, error) {
	args := m.Called(ctx, userID, itineraryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserSavedItinerary), args.Error(1)
}

func (m *MockPOIRepository) GetItineraries(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]types.UserSavedItinerary, int, error) {
	args := m.Called(ctx, userID, page, pageSize)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]types.UserSavedItinerary), args.Int(1), args.Error(2)
}
func (m *MockPOIRepository) UpdateItinerary(ctx context.Context, userID uuid.UUID, itineraryID uuid.UUID, updates types.UpdateItineraryRequest) (*types.UserSavedItinerary, error) {
	args := m.Called(ctx, userID, itineraryID, updates)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserSavedItinerary), args.Error(1)
}

func (m *MockPOIRepository) SaveItinerary(ctx context.Context, userID, cityID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, userID, cityID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockPOIRepository) SaveItineraryPOIs(ctx context.Context, itineraryID uuid.UUID, pois []types.POIDetail) error {
	args := m.Called(ctx, itineraryID, pois)
	return args.Error(0)
}

func (m *MockPOIRepository) SavePOItoPointsOfInterest(ctx context.Context, poi types.POIDetail, cityID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, poi, cityID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockPOIRepository) CityExists(ctx context.Context, cityID uuid.UUID) (bool, error) {
	args := m.Called(ctx, cityID)
	return args.Bool(0), args.Error(1)
}

// Helper to setup service with mock repository
func setupPOIServiceTest() (*ServiceImpl, *MockPOIRepository) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})) // or io.Discard
	mockRepo := new(MockPOIRepository)
	service := NewServiceImpl(mockRepo, logger)
	return service, mockRepo
}

func TestPOIServiceImpl_AddPoiToFavourites(t *testing.T) {
	service, mockRepo := setupPOIServiceTest()
	ctx := context.Background()
	userID := uuid.New()
	poiID := uuid.New()
	expectedFavouriteID := uuid.New()

	t.Run("success", func(t *testing.T) {
		mockRepo.On("AddPoiToFavourites", ctx, userID, poiID).Return(expectedFavouriteID, nil).Once()

		favID, err := service.AddPoiToFavourites(ctx, userID, poiID)
		require.NoError(t, err)
		assert.Equal(t, expectedFavouriteID, favID)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		expectedErr := errors.New("db error")
		mockRepo.On("AddPoiToFavourites", ctx, userID, poiID).Return(uuid.Nil, expectedErr).Once()

		_, err := service.AddPoiToFavourites(ctx, userID, poiID)
		require.Error(t, err)
		assert.EqualError(t, err, expectedErr.Error()) // Service just passes through the error
		mockRepo.AssertExpectations(t)
	})
}

func TestPOIServiceImpl_RemovePoiFromFavourites(t *testing.T) {
	service, mockRepo := setupPOIServiceTest()
	ctx := context.Background()
	userID := uuid.New()
	poiID := uuid.New()

	t.Run("success", func(t *testing.T) {
		mockRepo.On("RemovePoiFromFavourites", ctx, poiID, userID).Return(nil).Once()

		err := service.RemovePoiFromFavourites(ctx, poiID, userID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		expectedErr := errors.New("db error on remove")
		mockRepo.On("RemovePoiFromFavourites", ctx, poiID, userID).Return(expectedErr).Once()

		err := service.RemovePoiFromFavourites(ctx, poiID, userID)
		require.Error(t, err)
		assert.EqualError(t, err, expectedErr.Error())
		mockRepo.AssertExpectations(t)
	})
}

func TestPOIServiceImpl_GetFavouritePOIsByUserID(t *testing.T) {
	service, mockRepo := setupPOIServiceTest()
	ctx := context.Background()
	userID := uuid.New()

	t.Run("success - pois found", func(t *testing.T) {
		expectedPOIs := []types.POIDetail{
			{ID: uuid.New(), Name: "Fav POI 1"},
			{ID: uuid.New(), Name: "Fav POI 2"},
		}
		mockRepo.On("GetFavouritePOIsByUserID", ctx, userID).Return(expectedPOIs, nil).Once()

		pois, err := service.GetFavouritePOIsByUserID(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedPOIs, pois)
		mockRepo.AssertExpectations(t)
	})

	t.Run("success - no pois found", func(t *testing.T) {
		var expectedPOIs []types.POIDetail // Empty slice
		mockRepo.On("GetFavouritePOIsByUserID", ctx, userID).Return(expectedPOIs, nil).Once()

		pois, err := service.GetFavouritePOIsByUserID(ctx, userID)
		require.NoError(t, err)
		assert.Empty(t, pois) // Or assert.Equal(t, expectedPOIs, pois)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		expectedErr := errors.New("db error on get favourites")
		mockRepo.On("GetFavouritePOIsByUserID", ctx, userID).Return(nil, expectedErr).Once()

		_, err := service.GetFavouritePOIsByUserID(ctx, userID)
		require.Error(t, err)
		assert.EqualError(t, err, expectedErr.Error())
		mockRepo.AssertExpectations(t)
	})
}

func TestPOIServiceImpl_GetPOIsByCityID(t *testing.T) {
	service, mockRepo := setupPOIServiceTest()
	ctx := context.Background()
	cityID := uuid.New()

	t.Run("success - pois found", func(t *testing.T) {
		expectedPOIs := []types.POIDetail{
			{ID: uuid.New(), Name: "City POI 1"},
			{ID: uuid.New(), Name: "City POI 2"},
		}
		mockRepo.On("GetPOIsByCityID", ctx, cityID).Return(expectedPOIs, nil).Once()

		pois, err := service.GetPOIsByCityID(ctx, cityID)
		require.NoError(t, err)
		assert.Equal(t, expectedPOIs, pois)
		mockRepo.AssertExpectations(t)
	})

	t.Run("success - no pois found", func(t *testing.T) {
		var expectedPOIs []types.POIDetail
		mockRepo.On("GetPOIsByCityID", ctx, cityID).Return(expectedPOIs, nil).Once()

		pois, err := service.GetPOIsByCityID(ctx, cityID)
		require.NoError(t, err)
		assert.Empty(t, pois)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		expectedErr := errors.New("db error on get by city")
		mockRepo.On("GetPOIsByCityID", ctx, cityID).Return(nil, expectedErr).Once()

		_, err := service.GetPOIsByCityID(ctx, cityID)
		require.Error(t, err)
		assert.EqualError(t, err, expectedErr.Error())
		mockRepo.AssertExpectations(t)
	})
}

func TestPOIServiceImpl_SearchPOIs(t *testing.T) {
	service, mockRepo := setupPOIServiceTest()
	ctx := context.Background()
	filter := types.POIFilter{
		Category: "Museum",
	}

	t.Run("success - pois found", func(t *testing.T) {
		expectedPOIs := []types.POIDetail{
			{ID: uuid.New(), Name: "Filtered POI 1", Category: "Museum"},
		}
		mockRepo.On("SearchPOIs", ctx, filter).Return(expectedPOIs, nil).Once()

		pois, err := service.SearchPOIs(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, expectedPOIs, pois)
		mockRepo.AssertExpectations(t)
	})

	t.Run("success - no pois found by filter", func(t *testing.T) {
		var expectedPOIs []types.POIDetail
		mockRepo.On("SearchPOIs", ctx, filter).Return(expectedPOIs, nil).Once()

		pois, err := service.SearchPOIs(ctx, filter)
		require.NoError(t, err)
		assert.Empty(t, pois)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		expectedErr := errors.New("db error on search")
		mockRepo.On("SearchPOIs", ctx, filter).Return(nil, expectedErr).Once()

		_, err := service.SearchPOIs(ctx, filter)
		require.Error(t, err)
		assert.EqualError(t, err, expectedErr.Error())
		mockRepo.AssertExpectations(t)
	})
}
