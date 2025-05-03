package userTags

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
)

// MockUserRepo is a mock implementation of the UserRepo interface
type MockUserRepo struct {
	mock.Mock
}

// Implement all methods of the UserRepo interface
func (m *MockUserRepo) GetUserByID(ctx context.Context, userID uuid.UUID) (*api.UserProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.UserProfile), args.Error(1)
}

func (m *MockUserRepo) ChangePassword(ctx context.Context, email, oldPassword, newPassword string) error {
	args := m.Called(ctx, email, oldPassword, newPassword)
	return args.Error(0)
}

func (m *MockUserRepo) UpdateProfile(ctx context.Context, userID uuid.UUID, params api.UpdateProfileParams) error {
	args := m.Called(ctx, userID, params)
	return args.Error(0)
}

func (m *MockUserRepo) GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]api.Interest, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]api.Interest), args.Error(1)
}

func (m *MockUserRepo) AddUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error {
	args := m.Called(ctx, userID, interestID)
	return args.Error(0)
}

func (m *MockUserRepo) RemoveUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error {
	args := m.Called(ctx, userID, interestID)
	return args.Error(0)
}

func (m *MockUserRepo) GetAllInterests(ctx context.Context) ([]api.Interest, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]api.Interest), args.Error(1)
}

func (m *MockUserRepo) UpdateUserInterestPreferenceLevel(ctx context.Context, userID uuid.UUID, interestID uuid.UUID, preferenceLevel int) error {
	args := m.Called(ctx, userID, interestID, preferenceLevel)
	return args.Error(0)
}

func (m *MockUserRepo) GetUserEnhancedInterests(ctx context.Context, userID uuid.UUID) ([]api.EnhancedInterest, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]api.EnhancedInterest), args.Error(1)
}

func (m *MockUserRepo) GetUserPreferenceProfiles(ctx context.Context, userID uuid.UUID) ([]api.UserPreferenceProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]api.UserPreferenceProfile), args.Error(1)
}

func (m *MockUserRepo) GetUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) (*api.UserPreferenceProfile, error) {
	args := m.Called(ctx, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.UserPreferenceProfile), args.Error(1)
}

func (m *MockUserRepo) GetDefaultUserPreferenceProfile(ctx context.Context, userID uuid.UUID) (*api.UserPreferenceProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.UserPreferenceProfile), args.Error(1)
}

func (m *MockUserRepo) CreateUserPreferenceProfile(ctx context.Context, userID uuid.UUID, params api.CreateUserPreferenceProfileParams) (*api.UserPreferenceProfile, error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.UserPreferenceProfile), args.Error(1)
}

func (m *MockUserRepo) UpdateUserPreferenceProfile(ctx context.Context, profileID uuid.UUID, params api.UpdateUserPreferenceProfileParams) error {
	args := m.Called(ctx, profileID, params)
	return args.Error(0)
}

func (m *MockUserRepo) DeleteUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error {
	args := m.Called(ctx, profileID)
	return args.Error(0)
}

func (m *MockUserRepo) SetDefaultUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error {
	args := m.Called(ctx, profileID)
	return args.Error(0)
}

func (m *MockUserRepo) GetAllGlobalTags(ctx context.Context) ([]api.GlobalTag, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]api.GlobalTag), args.Error(1)
}

func (m *MockUserRepo) GetUserAvoidTags(ctx context.Context, userID uuid.UUID) ([]api.UserAvoidTag, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]api.UserAvoidTag), args.Error(1)
}

func (m *MockUserRepo) AddUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
	args := m.Called(ctx, userID, tagID)
	return args.Error(0)
}

func (m *MockUserRepo) RemoveUserAvoidTag(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
	args := m.Called(ctx, userID, tagID)
	return args.Error(0)
}

func (m *MockUserRepo) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepo) MarkEmailAsVerified(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepo) DeactivateUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepo) ReactivateUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// Test cases for UserService
func TestGetUserProfile(t *testing.T) {
	// Create a mock repository
	mockRepo := new(MockUserRepo)
	logger := slog.Default()
	service := NewUserService(mockRepo, logger)

	// Test case: successful retrieval
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.New()
		expectedProfile := &api.UserProfile{
			ID:       userID,
			Email:    "test@example.com",
			Username: nil,
		}

		// Set up expectations
		mockRepo.On("GetUserByID", ctx, userID).Return(expectedProfile, nil).Once()

		// Call the service method
		profile, err := service.GetUserProfile(ctx, userID)

		// Assert expectations
		assert.NoError(t, err)
		assert.Equal(t, expectedProfile, profile)
		mockRepo.AssertExpectations(t)
	})

	// Test case: error from repository
	t.Run("Error", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.New()
		expectedError := errors.New("database error")

		// Set up expectations
		mockRepo.On("GetUserByID", ctx, userID).Return(nil, expectedError).Once()

		// Call the service method
		profile, err := service.GetUserProfile(ctx, userID)

		// Assert expectations
		assert.Error(t, err)
		assert.Nil(t, profile)
		assert.Contains(t, err.Error(), expectedError.Error())
		mockRepo.AssertExpectations(t)
	})
}

func TestUpdateUserProfile(t *testing.T) {
	// Create a mock repository
	mockRepo := new(MockUserRepo)
	logger := slog.Default()
	service := NewUserService(mockRepo, logger)

	username := "test"
	// Test case: successful update
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.New()
		params := api.UpdateProfileParams{
			Username: &username,
		}

		// Set up expectations
		mockRepo.On("UpdateProfile", ctx, userID, params).Return(nil).Once()

		// Call the service method
		err := service.UpdateUserProfile(ctx, userID, params)

		// Assert expectations
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	// Test case: error from repository
	t.Run("Error", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.New()
		params := api.UpdateProfileParams{
			Username: &username,
		}
		expectedError := errors.New("database error")

		// Set up expectations
		mockRepo.On("UpdateProfile", ctx, userID, params).Return(expectedError).Once()

		// Call the service method
		err := service.UpdateUserProfile(ctx, userID, params)

		// Assert expectations
		assert.Error(t, err)
		assert.Contains(t, err.Error(), expectedError.Error())
		mockRepo.AssertExpectations(t)
	})
}

func TestGetUserPreferences(t *testing.T) {
	// Create a mock repository
	mockRepo := new(MockUserRepo)
	logger := slog.Default()
	service := NewUserService(mockRepo, logger)

	// Test case: successful retrieval
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.New()
		expectedPreferences := []api.Interest{
			{ID: uuid.New(), Name: "Interest 1"},
			{ID: uuid.New(), Name: "Interest 2"},
		}

		// Set up expectations
		mockRepo.On("GetUserPreferences", ctx, userID).Return(expectedPreferences, nil).Once()

		// Call the service method
		preferences, err := service.GetUserPreferences(ctx, userID)

		// Assert expectations
		assert.NoError(t, err)
		assert.Equal(t, expectedPreferences, preferences)
		mockRepo.AssertExpectations(t)
	})

	// Test case: error from repository
	t.Run("Error", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.New()
		expectedError := errors.New("database error")

		// Set up expectations
		mockRepo.On("GetUserPreferences", ctx, userID).Return(nil, expectedError).Once()

		// Call the service method
		preferences, err := service.GetUserPreferences(ctx, userID)

		// Assert expectations
		assert.Error(t, err)
		assert.Nil(t, preferences)
		assert.Contains(t, err.Error(), expectedError.Error())
		mockRepo.AssertExpectations(t)
	})
}
