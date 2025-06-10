package settings

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

// MockSettingsRepository is a mock implementation of SettingsRepository
type MockSettingsRepository struct {
	mock.Mock
}

func (m *MockSettingsRepository) Get(ctx context.Context, userID uuid.UUID) (*types.Settings, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Settings), args.Error(1)
}

func (m *MockSettingsRepository) Update(ctx context.Context, userID uuid.UUID, profileID uuid.UUID, params types.UpdatesettingsParams) error {
	args := m.Called(ctx, userID, profileID, params)
	return args.Error(0)
}

// Helper to setup service with mock repository
func setupSettingsServiceTest() (*SettingsServiceImpl, *MockSettingsRepository) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})) // or io.Discard
	mockRepo := new(MockSettingsRepository)
	service := NewsettingsService(mockRepo, logger)
	return service, mockRepo
}

func TestSettingsServiceImpl_Getsettings(t *testing.T) {
	service, mockRepo := setupSettingsServiceTest()
	ctx := context.Background()
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		expectedSettings := &types.Settings{
			UserID:                userID,
			DefaultSearchRadiusKm: 10.0,
			PreferredTime:         types.DayPreference("morning"),
			DefaultBudgetLevel:    2,
			PreferredPace:         types.SearchPace("moderate"),
			PreferAccessiblePOIs:  false,
			PreferOutdoorSeating:  false,
			PreferDogFriendly:     false,
			SearchRadius:          5.0,
			BudgetLevel:           1,
			PreferredTransport:    "walking",
		}
		mockRepo.On("Get", ctx, userID).Return(expectedSettings, nil).Once()

		settings, err := service.Getsettings(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedSettings, settings)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error - not found", func(t *testing.T) {
		repoErr := errors.New("settings not found in repo") // Or types.ErrNotFound
		mockRepo.On("Get", ctx, userID).Return(nil, repoErr).Once()

		_, err := service.Getsettings(ctx, userID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, repoErr), "Expected service error to wrap repository error")
		assert.Contains(t, err.Error(), "error fetching user settings:")
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository returns other error", func(t *testing.T) {
		repoErr := errors.New("database connection error")
		mockRepo.On("Get", ctx, userID).Return(nil, repoErr).Once()

		_, err := service.Getsettings(ctx, userID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, repoErr))
		mockRepo.AssertExpectations(t)
	})
}

func TestSettingsServiceImpl_Updatesettings(t *testing.T) {
	service, mockRepo := setupSettingsServiceTest()
	ctx := context.Background()
	userID := uuid.New()
	profileID := uuid.New() // Assuming this is a valid ID for an existing settings profile row
	params := types.UpdatesettingsParams{
		DefaultSearchRadiusKm: &[]float64{15.0}[0],
		PreferAccessiblePOIs:  &[]bool{true}[0],
		BudgetLevel:           &[]int{2}[0],
	}

	t.Run("success", func(t *testing.T) {
		mockRepo.On("Update", ctx, userID, profileID, params).Return(nil).Once()

		err := service.Updatesettings(ctx, userID, profileID, params)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error - not found or permission issue", func(t *testing.T) {
		// If the repo.Update returns an error when the record for userID/profileID doesn't exist
		repoErr := errors.New("settings profile not found or not owned by user")
		mockRepo.On("Update", ctx, userID, profileID, params).Return(repoErr).Once()

		err := service.Updatesettings(ctx, userID, profileID, params)
		require.Error(t, err)
		assert.True(t, errors.Is(err, repoErr))
		assert.Contains(t, err.Error(), "error updating user settings:")
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository returns other error", func(t *testing.T) {
		repoErr := errors.New("db error on update settings")
		mockRepo.On("Update", ctx, userID, profileID, params).Return(repoErr).Once()

		err := service.Updatesettings(ctx, userID, profileID, params)
		require.Error(t, err)
		assert.True(t, errors.Is(err, repoErr))
		mockRepo.AssertExpectations(t)
	})

	t.Run("no actual updates in params (service should still call repo)", func(t *testing.T) {
		// The service layer currently doesn't check if params is empty, it passes to repo.
		// The repo's Update method would handle dynamic SQL and do nothing if no fields are set.
		emptyParams := types.UpdatesettingsParams{}
		mockRepo.On("Update", ctx, userID, profileID, emptyParams).Return(nil).Once()

		err := service.Updatesettings(ctx, userID, profileID, emptyParams)
		require.NoError(t, err) // Expect no error as repo should handle it gracefully
		mockRepo.AssertExpectations(t)
	})
}
