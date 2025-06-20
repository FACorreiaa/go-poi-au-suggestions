package llmChat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	generativeAI "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/generative_ai"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai" // For genai.GenerateContentConfig
)

// --- Mocks for Dependencies ---

// Mock AIClient
type MockAIClient struct {
	mock.Mock
}

// Ensure MockAIClient satisfies an interface if LlmInteractiontServiceImpl uses one.
// For now, assuming direct use of *generativeAI.AIClient struct type.
// To make this more testable, LlmInteractiontServiceImpl should ideally depend on an interface for AIClient.
// Let's define a minimal interface that AIClient should satisfy for our service's needs:
type AIClientInterface interface {
	GenerateResponse(ctx context.Context, prompt string, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
	// Add other methods used by LlmInteractiontServiceImpl if any, e.g., StartChatSession
}

func (m *MockAIClient) GenerateResponse(ctx context.Context, prompt string, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	args := m.Called(ctx, prompt, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*genai.GenerateContentResponse), args.Error(1)
}

// Mock Repositories (Example for POIRepository, create similar for others)
type MockPOIRepository struct {
	mock.Mock
}

func (m *MockPOIRepository) SavePoi(ctx context.Context, poi types.POIDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, poi, cityID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockPOIRepository) FindPoiByNameAndCity(ctx context.Context, name string, cityID uuid.UUID) (*types.POIDetailedInfo, error) {
	args := m.Called(ctx, name, cityID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) GetPOIsByCityAndDistance(ctx context.Context, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, cityID, userLocation)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) GetPOIsByLocationAndDistance(ctx context.Context, lat, lon, radiusMeters float64) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, lat, lon, radiusMeters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) GetPOIsByLocationAndDistanceWithFilters(ctx context.Context, lat, lon, radiusMeters float64, filters map[string]string) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, lat, lon, radiusMeters, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) AddPoiToFavourites(ctx context.Context, userID, poiID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, userID, poiID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockPOIRepository) RemovePoiFromFavourites(ctx context.Context, poiID uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, poiID, userID)
	return args.Error(0)
}

func (m *MockPOIRepository) GetFavouritePOIsByUserID(ctx context.Context, userID uuid.UUID) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) GetPOIsByCityID(ctx context.Context, cityID uuid.UUID) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, cityID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) FindPOIDetailedInfos(ctx context.Context, cityID uuid.UUID, lat, lon float64, tolerance float64) (*types.POIDetailedInfo, error) {
	args := m.Called(ctx, cityID, lat, lon, tolerance)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) SavePOIDetailedInfos(ctx context.Context, poi types.POIDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, poi, cityID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockPOIRepository) SearchPOIs(ctx context.Context, filter types.POIFilter) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) FindSimilarPOIs(ctx context.Context, queryEmbedding []float32, limit int) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, queryEmbedding, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) FindSimilarPOIsByCity(ctx context.Context, queryEmbedding []float32, cityID uuid.UUID, limit int) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, queryEmbedding, cityID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) SearchPOIsHybrid(ctx context.Context, filter types.POIFilter, queryEmbedding []float32, semanticWeight float64) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, filter, queryEmbedding, semanticWeight)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) UpdatePOIEmbedding(ctx context.Context, poiID uuid.UUID, embedding []float32) error {
	args := m.Called(ctx, poiID, embedding)
	return args.Error(0)
}

func (m *MockPOIRepository) GetPOIsWithoutEmbeddings(ctx context.Context, limit int) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) FindHotelDetails(ctx context.Context, cityID uuid.UUID, lat, lon, tolerance float64) ([]types.HotelDetailedInfo, error) {
	args := m.Called(ctx, cityID, lat, lon, tolerance)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.HotelDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) SaveHotelDetails(ctx context.Context, hotel types.HotelDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, hotel, cityID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockPOIRepository) GetHotelByID(ctx context.Context, hotelID uuid.UUID) (*types.HotelDetailedInfo, error) {
	args := m.Called(ctx, hotelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.HotelDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) FindRestaurantDetails(ctx context.Context, cityID uuid.UUID, lat, lon, tolerance float64, preferences *types.RestaurantUserPreferences) ([]types.RestaurantDetailedInfo, error) {
	args := m.Called(ctx, cityID, lat, lon, tolerance, preferences)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.RestaurantDetailedInfo), args.Error(1)
}

func (m *MockPOIRepository) SaveRestaurantDetails(ctx context.Context, restaurant types.RestaurantDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, restaurant, cityID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockPOIRepository) GetRestaurantByID(ctx context.Context, restaurantID uuid.UUID) (*types.RestaurantDetailedInfo, error) {
	args := m.Called(ctx, restaurantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.RestaurantDetailedInfo), args.Error(1)
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
	return args.Get(0).([]types.UserSavedItinerary), args.Get(1).(int), args.Error(2)
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

func (m *MockPOIRepository) SaveItineraryPOIs(ctx context.Context, itineraryID uuid.UUID, pois []types.POIDetailedInfo) error {
	args := m.Called(ctx, itineraryID, pois)
	return args.Error(0)
}

func (m *MockPOIRepository) SavePOItoPointsOfInterest(ctx context.Context, poi types.POIDetailedInfo, cityID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, poi, cityID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockPOIRepository) CityExists(ctx context.Context, cityID uuid.UUID) (bool, error) {
	args := m.Called(ctx, cityID)
	return args.Get(0).(bool), args.Error(1)
}

type MockCityRepository struct{ mock.Mock }

func (m *MockCityRepository) SaveCity(ctx context.Context, city types.CityDetail) (uuid.UUID, error) {
	args := m.Called(ctx, city)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockCityRepository) FindCityByNameAndCountry(ctx context.Context, name, country string) (*types.CityDetail, error) {
	args := m.Called(ctx, name, country)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.CityDetail), args.Error(1)
}

func (m *MockCityRepository) GetCityIDByName(ctx context.Context, cityName string) (uuid.UUID, error) {
	args := m.Called(ctx, cityName)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockCityRepository) GetAllCities(ctx context.Context) ([]types.CityDetail, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.CityDetail), args.Error(1)
}

func (m *MockCityRepository) FindSimilarCities(ctx context.Context, queryEmbedding []float32, limit int) ([]types.CityDetail, error) {
	args := m.Called(ctx, queryEmbedding, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.CityDetail), args.Error(1)
}

func (m *MockCityRepository) UpdateCityEmbedding(ctx context.Context, cityID uuid.UUID, embedding []float32) error {
	args := m.Called(ctx, cityID, embedding)
	return args.Error(0)
}

func (m *MockCityRepository) GetCitiesWithoutEmbeddings(ctx context.Context, limit int) ([]types.CityDetail, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.CityDetail), args.Error(1)
}

func (m *MockCityRepository) GetCity(ctx context.Context, lat, lon float64) (uuid.UUID, string, error) {
	args := m.Called(ctx, lat, lon)
	return args.Get(0).(uuid.UUID), args.Get(1).(string), args.Error(2)
}

type MockLLMInteractionRepository struct{ mock.Mock }

func (m *MockLLMInteractionRepository) SaveInteraction(ctx context.Context, interaction types.LlmInteraction) (uuid.UUID, error) {
	args := m.Called(ctx, interaction)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockLLMInteractionRepository) SaveLlmSuggestedPOIsBatch(ctx context.Context, pois []types.POIDetailedInfo, userID, searchProfileID, llmInteractionID, cityID uuid.UUID) error {
	args := m.Called(ctx, pois, userID, searchProfileID, llmInteractionID, cityID)
	return args.Error(0)
}

func (m *MockLLMInteractionRepository) GetLlmSuggestedPOIsByInteractionSortedByDistance(ctx context.Context, llmInteractionID uuid.UUID, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, llmInteractionID, cityID, userLocation)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockLLMInteractionRepository) AddChatToBookmark(ctx context.Context, itinerary *types.UserSavedItinerary) (uuid.UUID, error) {
	args := m.Called(ctx, itinerary)
	if args.Get(0) == nil {
		return uuid.Nil, args.Error(1)
	}
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockLLMInteractionRepository) RemoveChatFromBookmark(ctx context.Context, userID, itineraryID uuid.UUID) error {
	args := m.Called(ctx, userID, itineraryID)
	return args.Error(0)
}

func (m *MockLLMInteractionRepository) GetInteractionByID(ctx context.Context, interactionID uuid.UUID) (*types.LlmInteraction, error) {
	args := m.Called(ctx, interactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.LlmInteraction), args.Error(1)
}

func (m *MockLLMInteractionRepository) CreateSession(ctx context.Context, session types.ChatSession) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockLLMInteractionRepository) GetSession(ctx context.Context, sessionID uuid.UUID) (*types.ChatSession, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.ChatSession), args.Error(1)
}

func (m *MockLLMInteractionRepository) GetUserChatSessions(ctx context.Context, userID uuid.UUID) ([]types.ChatSession, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.ChatSession), args.Error(1)
}

func (m *MockLLMInteractionRepository) UpdateSession(ctx context.Context, session types.ChatSession) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockLLMInteractionRepository) AddMessageToSession(ctx context.Context, sessionID uuid.UUID, message types.ConversationMessage) error {
	args := m.Called(ctx, sessionID, message)
	return args.Error(0)
}

func (m *MockLLMInteractionRepository) SaveSinglePOI(ctx context.Context, poi types.POIDetailedInfo, userID, cityID uuid.UUID, llmInteractionID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, poi, userID, cityID, llmInteractionID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockLLMInteractionRepository) GetPOIsBySessionSortedByDistance(ctx context.Context, sessionID, cityID uuid.UUID, userLocation types.UserLocation) ([]types.POIDetailedInfo, error) {
	args := m.Called(ctx, sessionID, cityID, userLocation)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.POIDetailedInfo), args.Error(1)
}

func (m *MockLLMInteractionRepository) CalculateDistancePostGIS(ctx context.Context, userLat, userLon, poiLat, poiLon float64) (float64, error) {
	args := m.Called(ctx, userLat, userLon, poiLat, poiLon)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockLLMInteractionRepository) GetOrCreatePOI(ctx context.Context, tx pgx.Tx, POIDetailedInfo types.POIDetailedInfo, cityID uuid.UUID, sourceInteractionID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, tx, POIDetailedInfo, cityID, sourceInteractionID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

type MockinterestsRepo struct{ mock.Mock }

func (m *MockinterestsRepo) CreateInterest(ctx context.Context, name string, description *string, isActive bool, userID string) (*types.Interest, error) {
	args := m.Called(ctx, name, description, isActive, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Interest), args.Error(1)
}

func (m *MockinterestsRepo) Removeinterests(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error {
	args := m.Called(ctx, userID, interestID)
	return args.Error(0)
}

func (m *MockinterestsRepo) GetAllInterests(ctx context.Context) ([]*types.Interest, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Interest), args.Error(1)
}

func (m *MockinterestsRepo) GetInterest(ctx context.Context, interestID uuid.UUID) (*types.Interest, error) {
	args := m.Called(ctx, interestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Interest), args.Error(1)
}

func (m *MockinterestsRepo) Updateinterests(ctx context.Context, userID uuid.UUID, interestID uuid.UUID, params types.UpdateinterestsParams) error {
	args := m.Called(ctx, userID, interestID, params)
	return args.Error(0)
}

func (m *MockinterestsRepo) AddInterestToProfile(ctx context.Context, profileID, interestID uuid.UUID) error {
	args := m.Called(ctx, profileID, interestID)
	return args.Error(0)
}

func (m *MockinterestsRepo) GetInterestsForProfile(ctx context.Context, profileID uuid.UUID) ([]*types.Interest, error) {
	args := m.Called(ctx, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Interest), args.Error(1)
}

type MockSearchProfileRepo struct{ mock.Mock }

func (m *MockSearchProfileRepo) GetSearchProfiles(ctx context.Context, userID uuid.UUID) ([]types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.UserPreferenceProfileResponse), args.Error(1)
}

func (m *MockSearchProfileRepo) GetSearchProfile(ctx context.Context, userID, profileID uuid.UUID) (*types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserPreferenceProfileResponse), args.Error(1)
}

func (m *MockSearchProfileRepo) GetDefaultSearchProfile(ctx context.Context, userID uuid.UUID) (*types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserPreferenceProfileResponse), args.Error(1)
}

func (m *MockSearchProfileRepo) CreateSearchProfile(ctx context.Context, userID uuid.UUID, params types.CreateUserPreferenceProfileParams) (*types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserPreferenceProfileResponse), args.Error(1)
}

func (m *MockSearchProfileRepo) UpdateSearchProfile(ctx context.Context, userID, profileID uuid.UUID, params types.UpdateSearchProfileParams) error {
	args := m.Called(ctx, userID, profileID, params)
	return args.Error(0)
}

func (m *MockSearchProfileRepo) DeleteSearchProfile(ctx context.Context, userID, profileID uuid.UUID) error {
	args := m.Called(ctx, userID, profileID)
	return args.Error(0)
}

func (m *MockSearchProfileRepo) SetDefaultSearchProfile(ctx context.Context, userID, profileID uuid.UUID) error {
	args := m.Called(ctx, userID, profileID)
	return args.Error(0)
}

type MockTagsRepo struct{ mock.Mock }

func (m *MockTagsRepo) GetAll(ctx context.Context, userID uuid.UUID) ([]*types.Tags, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Tags), args.Error(1)
}

func (m *MockTagsRepo) Get(ctx context.Context, userID, tagID uuid.UUID) (*types.Tags, error) {
	args := m.Called(ctx, userID, tagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Tags), args.Error(1)
}

func (m *MockTagsRepo) Create(ctx context.Context, userID uuid.UUID, params types.CreatePersonalTagParams) (*types.PersonalTag, error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PersonalTag), args.Error(1)
}

func (m *MockTagsRepo) Delete(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
	args := m.Called(ctx, userID, tagID)
	return args.Error(0)
}

func (m *MockTagsRepo) Update(ctx context.Context, userID, tagsID uuid.UUID, params types.UpdatePersonalTagParams) error {
	args := m.Called(ctx, userID, tagsID, params)
	return args.Error(0)
}

func (m *MockTagsRepo) GetTagByName(ctx context.Context, name string) (*types.Tags, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Tags), args.Error(1)
}

func (m *MockTagsRepo) LinkPersonalTagToProfile(ctx context.Context, userID, profileID uuid.UUID, tagID uuid.UUID) error {
	args := m.Called(ctx, userID, profileID, tagID)
	return args.Error(0)
}

func (m *MockTagsRepo) GetTagsForProfile(ctx context.Context, profileID uuid.UUID) ([]*types.Tags, error) {
	args := m.Called(ctx, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Tags), args.Error(1)
}

// Helper to setup service with mocks for each test
func setupTestServiceWithMocks() (
	*LlmInteractiontServiceImpl,
	*MockAIClient, // Assuming AIClient will be interface type in service
	*MockinterestsRepo,
	*MockSearchProfileRepo,
	*MockTagsRepo,
	*MockLLMInteractionRepository,
	*MockCityRepository,
	*MockPOIRepository,
) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})) // Or io.Discard for less noise
	mockAI := new(MockAIClient)
	mockInterestR := new(MockinterestsRepo)
	mockSearchProfileR := new(MockSearchProfileRepo)
	mockTagsR := new(MockTagsRepo)
	mockLLMInteractionR := new(MockLLMInteractionRepository)
	mockCityR := new(MockCityRepository)
	mockPOIR := new(MockPOIRepository)

	// To use MockAIClient, LlmInteractiontServiceImpl should accept an AIClientInterface.
	// For now, we can't directly inject MockAIClient if the service expects *generativeAI.AIClient.
	// This is a common pain point. The service constructor needs to accept an interface for AIClient.

	// Let's assume you refactor NewLlmInteractiontService to accept an AIClientInterface:
	// service := NewLlmInteractiontService(..., mockAI, ...)
	// For now, we'll create the service and it will have its own real AIClient,
	// which means tests involving AI calls will be harder to unit test without mocking HTTP calls.
	// A pragmatic approach for methods NOT heavily using AIClient directly for unit tests:
	ctx := context.Background()
	realAIC, _ := generativeAI.NewAIClient(ctx) // This will init real client (needs API key for New, but not for being a field)

	service := &LlmInteractiontServiceImpl{
		logger:            logger,
		interestRepo:      mockInterestR,
		searchProfileRepo: mockSearchProfileR,
		tagsRepo:          mockTagsR,
		aiClient:          realAIC, // For unit tests not hitting AI, this is okay. For those that do, more work needed.
		// Ideally: aiClient: mockAI, (if service takes AIClientInterface)
		llmInteractionRepo: mockLLMInteractionR,
		cityRepo:           mockCityR,
		poiRepo:            mockPOIR,
		cache:              cache.New(5*time.Minute, 10*time.Minute),
	}

	return service, mockAI, mockInterestR, mockSearchProfileR, mockTagsR, mockLLMInteractionR, mockCityR, mockPOIR
}

func TestLlmInteractionServiceImpl_GetPOIDetailedInfosResponse_Unit(t *testing.T) {
	service, mockAI, _, _, _, mockLLMRepo, mockCityRepo, mockPOIRepo := setupTestServiceWithMocks()
	ctx := context.Background()
	userID := uuid.New()
	city := "Test City"
	lat, lon := 10.0, 20.0
	expectedPOIID := uuid.New()
	cacheKey := generatePOICacheKey(city, lat, lon, 0.0, userID) // Assuming 0.0 distance for this specific cache key

	// For this test, we need to be able to mock the AIClient if it's used.
	// If your service's `aiClient` field was an interface type, you could assign `mockAI` to it.
	// e.g., service.aiClient = mockAI (if service.aiClient is AIClientInterface)

	t.Run("Cache Hit", func(t *testing.T) {
		service.cache.Flush() // Clear cache for clean test
		expectedDetails := &types.POIDetailedInfo{ID: expectedPOIID, Name: "Cached POI", City: city, Latitude: lat, Longitude: lon}
		service.cache.Set(cacheKey, expectedDetails, cache.DefaultExpiration)

		details, err := service.GetPOIDetailedInfosResponse(ctx, userID, city, lat, lon)
		require.NoError(t, err)
		require.NotNil(t, details)
		assert.Equal(t, expectedDetails.Name, details.Name)
		// mockPOIRepo.AssertNotCalled(t, "FindPOIDetailedInfos") // If you mocked this method
		// mockAI.AssertNotCalled(t, "GenerateResponse")
	})

	t.Run("Database Hit", func(t *testing.T) {
		service.cache.Flush()
		expectedDBDetails := &types.POIDetailedInfo{ID: expectedPOIID, Name: "DB POI", City: city, Latitude: lat, Longitude: lon}

		// Mock CityRepo
		mockCityRepo.On("FindCityByNameAndCountry", ctx, city, "").Return(&types.CityDetail{ID: uuid.New(), Name: city}, nil).Once()
		// Mock POIRepo to return data
		mockPOIRepo.On("FindPOIDetailedInfos", ctx, mock.AnythingOfType("uuid.UUID"), lat, lon, 100.0).Return(expectedDBDetails, nil).Once()

		details, err := service.GetPOIDetailedInfosResponse(ctx, userID, city, lat, lon)
		require.NoError(t, err)
		require.NotNil(t, details)
		assert.Equal(t, expectedDBDetails.Name, details.Name)

		// Check if it was added to in-memory cache
		cached, found := service.cache.Get(cacheKey)
		assert.True(t, found)
		assert.Equal(t, expectedDBDetails, cached.(*types.POIDetailedInfo))

		// mockAI.AssertNotCalled(t, "GenerateResponse")
		mockCityRepo.AssertExpectations(t)
		mockPOIRepo.AssertExpectations(t)
	})

	t.Run("AI Call Success (Cache and DB Miss)", func(t *testing.T) {
		service.cache.Flush()
		aiResponseJSON := `{"name": "AI POI", "description": "From AI", "latitude": 10.0, "longitude": 20.0}`
		mockGenAIResponse := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{Content: &genai.Content{Parts: []*genai.Part{genai.Text(aiResponseJSON)}}},
			},
		}
		// This mocking assumes LlmInteractiontServiceImpl.aiClient is an interface type
		// and has been set to mockAI. If not, this mock won't be hit.
		// For now, this test won't work as expected without that refactor.
		// mockAI.On("GenerateResponse", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("*genai.GenerateContentConfig")).Return(mockGenAIResponse, nil).Once()

		// Let's simulate what would happen if the AIClient interface was mocked:
		// Setup for real AIClient to be called (if not mocking AIClient interface)
		// This part would be an integration test or needs HTTP mocking.
		// For a unit test, we absolutely need to mock the AI call.
		// Assuming you *can* mock it (e.g. service.aiClient = mockAIClientThatReturnsSuccess):
		if service.aiClient == nil { // Or not our mock
			t.Skip("Skipping AI Call Success test: AIClient is not mockable in current service setup for unit test.")
		}

		mockCityRepo.On("FindCityByNameAndCountry", ctx, city, "").Return(&types.CityDetail{ID: uuid.New(), Name: city}, nil).Once()
		mockPOIRepo.On("FindPOIDetailedInfos", ctx, mock.AnythingOfType("uuid.UUID"), lat, lon, 100.0).Return(nil, nil).Once() // DB Miss
		mockLLMRepo.On("SaveInteraction", ctx, mock.AnythingOfType("types.LlmInteraction")).Return(uuid.New(), nil).Once()
		mockPOIRepo.On("SavePOIDetailedInfos", ctx, mock.AnythingOfType("types.POIDetailedInfo"), mock.AnythingOfType("uuid.UUID")).Return(uuid.New(), nil).Once()

		details, err := service.GetPOIDetailedInfosResponse(ctx, userID, city, lat, lon)
		// This will fail if the AI call is real and not mocked, or if API key is missing.
		// This test highlights the need to make service.aiClient an interface.
		// If you ran this with the skip above, it would not execute this part for AI call.

		// If the AI call was properly mocked to return mockGenAIResponse:
		require.NoError(t, err)
		require.NotNil(t, details)
		// assert.Equal(t, "AI POI", details.Name)
		// cached, found := service.cache.Get(cacheKey)
		// assert.True(t, found)
		// assert.Equal(t, "AI POI", cached.(*types.POIDetailedInfo).Name)
		// mockAI.AssertExpectations(t)
		mockCityRepo.AssertExpectations(t)
		mockPOIRepo.AssertExpectations(t)
		mockLLMRepo.AssertExpectations(t)

		// Because the AIClient is not easily mockable for unit test in the current setup,
		// this specific test case (AI call success) is more suited for an integration test,
		// or requires refactoring the service to accept an AIClientInterface.
		// For now, we can only fully unit test cache hit and DB hit paths.
		t.Log("NOTE: AI Call path for GetPOIDetailedInfosResponse unit test is limited without mocking AIClient interface.")
	})

	// Add more test cases:
	// - City not found in DB
	// - AI returns error
	// - AI returns malformed JSON
	// - SaveInteraction fails
	// - SavePOIDetailedInfos fails
}

// Example for GetItinerary (simpler, as it's mostly a direct repo call)
func TestLlmInteractionServiceImpl_GetItinerary_Unit(t *testing.T) {
	service, _, _, _, _, mockLLMRepo, _, _ := setupTestServiceWithMocks()
	ctx := context.Background()
	userID := uuid.New()
	itineraryID := uuid.New()

	t.Run("Itinerary found", func(t *testing.T) {
		expectedItinerary := &types.UserSavedItinerary{ID: itineraryID, UserID: userID, Title: "My Test Itinerary"}
		mockLLMRepo.On("GetItinerary", ctx, userID, itineraryID).Return(expectedItinerary, nil).Once()

		itinerary, err := service.GetItinerary(ctx, userID, itineraryID)
		require.NoError(t, err)
		require.NotNil(t, itinerary)
		assert.Equal(t, "My Test Itinerary", itinerary.Title)
		mockLLMRepo.AssertExpectations(t)
	})

	t.Run("Itinerary not found", func(t *testing.T) {
		notFoundErr := fmt.Errorf("no itinerary found with ID %s for user %s", itineraryID, userID) // Match repo error
		mockLLMRepo.On("GetItinerary", ctx, userID, itineraryID).Return(nil, notFoundErr).Once()

		_, err := service.GetItinerary(ctx, userID, itineraryID)
		require.Error(t, err)
		assert.EqualError(t, err, notFoundErr.Error())
		mockLLMRepo.AssertExpectations(t)
	})

	t.Run("Repository returns other error", func(t *testing.T) {
		dbErr := errors.New("database connection error")
		mockLLMRepo.On("GetItinerary", ctx, userID, itineraryID).Return(nil, dbErr).Once()

		_, err := service.GetItinerary(ctx, userID, itineraryID)
		require.Error(t, err)
		assert.EqualError(t, err, "database connection error") // Or however service wraps it
		mockLLMRepo.AssertExpectations(t)
	})
}

// Add similar unit tests for:
// - GetItineraries
// - UpdateItinerary
// - SaveItenerary
// - RemoveItenerary
// - GetHotelsByPreferenceResponse (mocking repo's FindHotelDetails, and AI call if fallback)
// - GetRestaurantsByPreferencesResponse (mocking repo's FindRestaurantDetails, and AI call if fallback)
// - etc.

// --- Integration Tests for llmInteraction (Example for GetPOIDetailedInfosResponse) ---
// These would require a running database instance and potentially a configured AI client.

func TestLlmInteractionServiceImpl_GetPOIDetailedInfosResponse_Integration(t *testing.T) {
	if !*runIntegrationTests { // Use the same flag as generativeAI tests
		t.Skip("Skipping integration test: -integration flag not set")
	}
	// Setup:
	// 1. Ensure GOOGLE_GEMINI_API_KEY is set
	// 2. Connect to a real test database (e.g., using Dockerized Postgres)
	// 3. Initialize real repositories and the LlmInteractiontServiceImpl with them.
	// For simplicity, this setup is omitted here but is crucial.

	// Example:
	// logger := slog.New(...)
	// dbpool := setupTestDB(t) // Helper function to connect to test DB
	// realAIC, _ := generativeAI.NewAIClient(context.Background()) // Needs API Key
	// realInterestRepo := interests.NewPostgresinterestsRepo(dbpool, logger)
	// ... initialize all real repos ...
	// service := NewLlmInteractiontService(realInterestRepo, ..., realAIC, logger)

	// ctx := context.Background()
	// userID := uuid.New() // Or a known test user ID
	// city := "Paris"      // A city the AI knows
	// lat, lon := 48.8566, 2.3522

	// t.Run("Fetch from AI and store in DB and cache", func(t *testing.T) {
	//     // Ensure cache and DB are initially empty for this POI
	//     details, err := service.GetPOIDetailedInfosResponse(ctx, userID, city, lat, lon)
	//     require.NoError(t, err)
	//     require.NotNil(t, details)
	//     assert.NotEmpty(t, details.Name)
	//     assert.Equal(t, city, details.City) // Check if it's populated back

	//     // Verify it's in cache
	//     cacheKey := generatePOICacheKey(city, lat, lon, 0.0, userID)
	//     cached, found := service.cache.Get(cacheKey)
	//     assert.True(t, found)
	//     assert.Equal(t, details, cached.(*types.POIDetailedInfo))

	//     // Verify it's in DB (requires querying the DB directly)
	//     // dbFetched, _ := realPOIRepo.FindDetailedPoiByLocation(ctx, city, lat, lon)
	//     // assert.NotNil(t, dbFetched)
	//     // assert.Equal(t, details.Name, dbFetched.Name)
	// })
	t.Skip("Full integration test for GetPOIDetailedInfosResponse requires DB and AI client setup.")
}
