package user

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	// For potentially testing UpdateLastLogin with time
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types" // Ensure this path is correct
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockUserRepo is a mock implementation of UserRepo
type MockUserRepo struct {
	mock.Mock
}

func (m *MockUserRepo) GetUserByID(ctx context.Context, userID uuid.UUID) (*types.UserProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserProfile), args.Error(1)
}

func (m *MockUserRepo) UpdateProfile(ctx context.Context, userID uuid.UUID, params types.UpdateProfileParams) error {
	args := m.Called(ctx, userID, params)
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

// ChangePassword
func (m *MockUserRepo) ChangePassword(ctx context.Context, email, oldPassword, newPassword string) error {
	args := m.Called(ctx, email, oldPassword, newPassword)
	return args.Error(0)
}

// Helper to setup service with mock repository
func setupUserServiceTest() (*ServiceUserImpl, *MockUserRepo) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})) // or io.Discard
	mockRepo := new(MockUserRepo)
	service := NewUserService(mockRepo, logger)
	return service, mockRepo
}

func TestServiceUserImpl_GetUserProfile(t *testing.T) {
	service, mockRepo := setupUserServiceTest()
	ctx := context.Background()
	userID := uuid.New()

	username := "testuser"
	t.Run("success", func(t *testing.T) {
		expectedProfile := &types.UserProfile{
			ID:       userID,
			Username: &username,
			Email:    "test@example.com",
		}
		mockRepo.On("GetUserByID", ctx, userID).Return(expectedProfile, nil).Once()

		profile, err := service.GetUserProfile(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedProfile, profile)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error - not found", func(t *testing.T) {
		// Assuming your repo returns a specific error for not found, or pgx.ErrNoRows
		// For this mock, we just return a generic error that the service wraps.
		repoErr := errors.New("user not found in repo") // Or a specific types.ErrNotFound
		mockRepo.On("GetUserByID", ctx, userID).Return(nil, repoErr).Once()

		_, err := service.GetUserProfile(ctx, userID)
		require.Error(t, err)
		// Service wraps the error
		assert.True(t, errors.Is(err, repoErr), "Expected service error to wrap repository error")
		assert.Contains(t, err.Error(), "error fetching user profile:")
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository returns other error", func(t *testing.T) {
		repoErr := errors.New("database connection error")
		mockRepo.On("GetUserByID", ctx, userID).Return(nil, repoErr).Once()

		_, err := service.GetUserProfile(ctx, userID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, repoErr))
		mockRepo.AssertExpectations(t)
	})
}

func TestServiceUserImpl_UpdateUserProfile(t *testing.T) {
	service, mockRepo := setupUserServiceTest()
	ctx := context.Background()
	userID := uuid.New()

	newUsername := "newusername" // Assuming Username might also be *string, adjust if not
	newFirstName := "NewFirst"
	newLastName := "NewLast"
	newCountry := "Testland"
	newCity := "Testville"
	params := types.UpdateProfileParams{
		Username:  &newUsername,  // Assuming Username is a *string in UpdateProfileParams
		Firstname: &newFirstName, // Assuming Firstname is a *string in UpdateProfileParams
		Lastname:  &newLastName,  // Assuming Lastname is a *string in UpdateProfileParams
		Country:   &newCountry,   // Assuming Country is a *string in UpdateProfileParams
		City:      &newCity,      // Assuming City is a *string in UpdateProfileParams

	}

	t.Run("success", func(t *testing.T) {
		mockRepo.On("UpdateProfile", ctx, userID, params).Return(nil).Once()

		err := service.UpdateUserProfile(ctx, userID, params)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		expectedErr := errors.New("db error on update profile")
		mockRepo.On("UpdateProfile", ctx, userID, params).Return(expectedErr).Once()

		err := service.UpdateUserProfile(ctx, userID, params)
		require.Error(t, err)
		assert.True(t, errors.Is(err, expectedErr))
		assert.Contains(t, err.Error(), "error updating user profile:")
		mockRepo.AssertExpectations(t)
	})
}

func TestServiceUserImpl_UpdateLastLogin(t *testing.T) {
	service, mockRepo := setupUserServiceTest()
	ctx := context.Background()
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		mockRepo.On("UpdateLastLogin", ctx, userID).Return(nil).Once()

		err := service.UpdateLastLogin(ctx, userID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		expectedErr := errors.New("db error on update last login")
		mockRepo.On("UpdateLastLogin", ctx, userID).Return(expectedErr).Once()

		err := service.UpdateLastLogin(ctx, userID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, expectedErr))
		assert.Contains(t, err.Error(), "error updating user last login timestamp:")
		mockRepo.AssertExpectations(t)
	})
}

func TestServiceUserImpl_MarkEmailAsVerified(t *testing.T) {
	service, mockRepo := setupUserServiceTest()
	ctx := context.Background()
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		mockRepo.On("MarkEmailAsVerified", ctx, userID).Return(nil).Once()

		err := service.MarkEmailAsVerified(ctx, userID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		expectedErr := errors.New("db error on mark email verified")
		mockRepo.On("MarkEmailAsVerified", ctx, userID).Return(expectedErr).Once()

		err := service.MarkEmailAsVerified(ctx, userID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, expectedErr))
		assert.Contains(t, err.Error(), "error marking user email as verified:")
		mockRepo.AssertExpectations(t)
	})
}

func TestServiceUserImpl_DeactivateUser(t *testing.T) {
	service, mockRepo := setupUserServiceTest()
	ctx := context.Background()
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		mockRepo.On("DeactivateUser", ctx, userID).Return(nil).Once()

		err := service.DeactivateUser(ctx, userID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		expectedErr := errors.New("db error on deactivate user")
		mockRepo.On("DeactivateUser", ctx, userID).Return(expectedErr).Once()

		err := service.DeactivateUser(ctx, userID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, expectedErr))
		assert.Contains(t, err.Error(), "error deactivating user:")
		mockRepo.AssertExpectations(t)
	})
}

func TestServiceUserImpl_ReactivateUser(t *testing.T) {
	service, mockRepo := setupUserServiceTest()
	ctx := context.Background()
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		mockRepo.On("ReactivateUser", ctx, userID).Return(nil).Once()

		err := service.ReactivateUser(ctx, userID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		expectedErr := errors.New("db error on reactivate user")
		mockRepo.On("ReactivateUser", ctx, userID).Return(expectedErr).Once()

		err := service.ReactivateUser(ctx, userID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, expectedErr))
		assert.Contains(t, err.Error(), "error reactivating user:")
		mockRepo.AssertExpectations(t)
	})
}
