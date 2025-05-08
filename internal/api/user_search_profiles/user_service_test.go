package userSearchProfile

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	userInterest "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_interests"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

// MockUserSearchProfilesRepo is a mock implementation of the UserSearchProfilesRepo interface
type MockUserSearchProfilesRepo struct {
	mock.Mock
}

// Implement all methods of the UserSearchProfilesRepo interface
func (m *MockUserSearchProfilesRepo) GetProfiles(ctx context.Context, userID uuid.UUID) ([]types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.UserPreferenceProfileResponse), args.Error(1)
}

func (m *MockUserSearchProfilesRepo) GetProfile(ctx context.Context, userID, profileID uuid.UUID) (*types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserPreferenceProfileResponse), args.Error(1)
}

func (m *MockUserSearchProfilesRepo) GetDefaultProfile(ctx context.Context, userID uuid.UUID) (*types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserPreferenceProfileResponse), args.Error(1)
}

func (m *MockUserSearchProfilesRepo) CreateProfile(ctx context.Context, userID uuid.UUID, params types.CreateUserPreferenceProfileParams) (*types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserPreferenceProfileResponse), args.Error(1)
}

func (m *MockUserSearchProfilesRepo) UpdateProfile(ctx context.Context, profileID uuid.UUID, params types.UpdateUserPreferenceProfileParams) error {
	args := m.Called(ctx, profileID, params)
	return args.Error(0)
}

func (m *MockUserSearchProfilesRepo) DeleteProfile(ctx context.Context, profileID uuid.UUID) error {
	args := m.Called(ctx, profileID)
	return args.Error(0)
}

func (m *MockUserSearchProfilesRepo) SetDefaultProfile(ctx context.Context, profileID uuid.UUID) error {
	args := m.Called(ctx, profileID)
	return args.Error(0)
}

// MockUserInterestRepo is a mock implementation of the userInterest.UserInterestRepo interface
type MockUserInterestRepo struct {
	mock.Mock
}

// Ensure MockUserInterestRepo implements userInterest.UserInterestRepo
var _ userInterest.UserInterestRepo = (*MockUserInterestRepo)(nil)

// Implement the methods used by UserSearchProfilesService
func (m *MockUserInterestRepo) GetAllInterests(ctx context.Context) ([]types.Interest, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Interest), args.Error(1)
}

func (m *MockUserInterestRepo) AddInterestToProfile(ctx context.Context, profileID uuid.UUID, interestID uuid.UUID) error {
	args := m.Called(ctx, profileID, interestID)
	return args.Error(0)
}

// Add stubs for other methods that are not used in our tests
func (m *MockUserInterestRepo) CreateInterest(ctx context.Context, name string, description *string, isActive bool, userID string) (*types.Interest, error) {
	args := m.Called(ctx, name, description, isActive, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Interest), args.Error(1)
}

func (m *MockUserInterestRepo) RemoveUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID) error {
	args := m.Called(ctx, userID, interestID)
	return args.Error(0)
}

func (m *MockUserInterestRepo) UpdateUserInterest(ctx context.Context, userID uuid.UUID, interestID uuid.UUID, params types.UpdateUserInterestParams) error {
	args := m.Called(ctx, userID, interestID, params)
	return args.Error(0)
}

func (m *MockUserInterestRepo) GetInterest(ctx context.Context, interestID uuid.UUID) (*types.Interest, error) {
	args := m.Called(ctx, interestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Interest), args.Error(1)
}

func (m *MockUserInterestRepo) GetInterestsForProfile(ctx context.Context, profileID uuid.UUID) ([]types.Interest, error) {
	args := m.Called(ctx, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Interest), args.Error(1)
}

// MockUserTagsRepo is a mock implementation of the types.UserTagsRepo interface
type MockUserTagsRepo struct {
	mock.Mock
}

// Implement the methods used by UserSearchProfilesService
func (m *MockUserTagsRepo) LinkPersonalTagToProfile(ctx context.Context, userID uuid.UUID, profileID uuid.UUID, tagID uuid.UUID) error {
	args := m.Called(ctx, userID, profileID, tagID)
	return args.Error(0)
}

// Add stubs for other methods that are not used in our tests
func (m *MockUserTagsRepo) GetAll(ctx context.Context, userID uuid.UUID) ([]types.Tags, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Tags), args.Error(1)
}

func (m *MockUserTagsRepo) Get(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) (*types.Tags, error) {
	args := m.Called(ctx, userID, tagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Tags), args.Error(1)
}

func (m *MockUserTagsRepo) Create(ctx context.Context, userID uuid.UUID, params types.CreatePersonalTagParams) (*types.PersonalTag, error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PersonalTag), args.Error(1)
}

func (m *MockUserTagsRepo) Update(ctx context.Context, userID uuid.UUID, tagID uuid.UUID, params types.UpdatePersonalTagParams) error {
	args := m.Called(ctx, userID, tagID, params)
	return args.Error(0)
}

func (m *MockUserTagsRepo) Delete(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) error {
	args := m.Called(ctx, userID, tagID)
	return args.Error(0)
}

func (m *MockUserTagsRepo) GetTagByName(ctx context.Context, name string) (*types.Tags, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Tags), args.Error(1)
}

func (m *MockUserTagsRepo) GetTagsForProfile(ctx context.Context, profileID uuid.UUID) ([]types.Tags, error) {
	args := m.Called(ctx, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Tags), args.Error(1)
}

// Test cases for UserSearchProfilesService
func TestGetUserPreferenceProfiles(t *testing.T) {
	// Create mock repositories
	mockProfileRepo := new(MockUserSearchProfilesRepo)
	mockInterestRepo := new(MockUserInterestRepo)
	mockTagRepo := new(MockUserTagsRepo)
	logger := slog.Default()

	// Create service with mocks
	service := NewUserProfilesService(mockProfileRepo, mockInterestRepo, mockTagRepo, logger)

	// Test case: successful retrieval
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.New()

		// Define expected profiles
		expectedProfiles := []types.UserPreferenceProfileResponse{
			{
				ID:          uuid.New(),
				UserID:      userID,
				ProfileName: "Profile 1",
				IsDefault:   true,
			},
			{
				ID:          uuid.New(),
				UserID:      userID,
				ProfileName: "Profile 2",
				IsDefault:   false,
			},
		}

		// Set up expectations
		mockProfileRepo.On("GetProfiles", ctx, userID).Return(expectedProfiles, nil).Once()

		// Call the service method
		profiles, err := service.GetUserPreferenceProfiles(ctx, userID)

		// Assert expectations
		assert.NoError(t, err)
		assert.Equal(t, expectedProfiles, profiles)
		mockProfileRepo.AssertExpectations(t)
	})

	// Test case: error from repository
	t.Run("Error", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.New()
		expectedError := errors.New("database error")

		// Set up expectations
		mockProfileRepo.On("GetProfiles", ctx, userID).Return(nil, expectedError).Once()

		// Call the service method
		profiles, err := service.GetUserPreferenceProfiles(ctx, userID)

		// Assert expectations
		assert.Error(t, err)
		assert.Nil(t, profiles)
		assert.Contains(t, err.Error(), expectedError.Error())
		mockProfileRepo.AssertExpectations(t)
	})
}

func TestGetUserPreferenceProfile(t *testing.T) {
	// Create mock repositories
	mockProfileRepo := new(MockUserSearchProfilesRepo)
	mockInterestRepo := new(MockUserInterestRepo)
	mockTagRepo := new(MockUserTagsRepo)
	logger := slog.Default()

	// Create service with mocks
	service := NewUserProfilesService(mockProfileRepo, mockInterestRepo, mockTagRepo, logger)

	// Test case: successful retrieval
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		profileID := uuid.New()
		userID := uuid.New()

		// Define expected profile
		expectedProfile := &types.UserPreferenceProfileResponse{
			ID:          profileID,
			UserID:      userID,
			ProfileName: "Test Profile",
			IsDefault:   true,
		}

		// Set up expectations
		mockProfileRepo.On("GetProfile", ctx, userID, profileID).Return(expectedProfile, nil).Once()

		// Call the service method
		profile, err := service.GetUserPreferenceProfile(ctx, userID, profileID)

		// Assert expectations
		assert.NoError(t, err)
		assert.Equal(t, expectedProfile, profile)
		mockProfileRepo.AssertExpectations(t)
	})

	// Test case: error from repository
	t.Run("Error", func(t *testing.T) {
		ctx := context.Background()
		profileID := uuid.New()
		userID := uuid.New()
		expectedError := errors.New("database error")

		// Set up expectations
		mockProfileRepo.On("GetProfile", ctx, profileID).Return(nil, expectedError).Once()

		// Call the service method
		profile, err := service.GetUserPreferenceProfile(ctx, userID, profileID)

		// Assert expectations
		assert.Error(t, err)
		assert.Nil(t, profile)
		assert.Contains(t, err.Error(), expectedError.Error())
		mockProfileRepo.AssertExpectations(t)
	})
}
