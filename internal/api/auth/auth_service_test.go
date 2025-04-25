package auth

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"

	"github.com/FACorreiaa/go-poi-au-suggestions/config"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
)

// MockAuthRepo is a mock implementation of the AuthRepo interface
type MockAuthRepo struct {
	mock.Mock
}

// Implement all methods of the AuthRepo interface
func (m *MockAuthRepo) GetUserByEmail(ctx context.Context, email string) (*api.UserAuth, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.UserAuth), args.Error(1)
}

func (m *MockAuthRepo) GetUserByID(ctx context.Context, userID string) (*api.UserAuth, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.UserAuth), args.Error(1)
}

func (m *MockAuthRepo) Register(ctx context.Context, username, email, hashedPassword string) (string, error) {
	args := m.Called(ctx, username, email, hashedPassword)
	return args.String(0), args.Error(1)
}

func (m *MockAuthRepo) VerifyPassword(ctx context.Context, userID, password string) error {
	args := m.Called(ctx, userID, password)
	return args.Error(0)
}

func (m *MockAuthRepo) UpdatePassword(ctx context.Context, userID, newHashedPassword string) error {
	args := m.Called(ctx, userID, newHashedPassword)
	return args.Error(0)
}

func (m *MockAuthRepo) StoreRefreshToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	args := m.Called(ctx, userID, token, expiresAt)
	return args.Error(0)
}

func (m *MockAuthRepo) ValidateRefreshTokenAndGetUserID(ctx context.Context, refreshToken string) (string, error) {
	args := m.Called(ctx, refreshToken)
	return args.String(0), args.Error(1)
}

func (m *MockAuthRepo) InvalidateRefreshToken(ctx context.Context, refreshToken string) error {
	args := m.Called(ctx, refreshToken)
	return args.Error(0)
}

func (m *MockAuthRepo) InvalidateAllUserRefreshTokens(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// Test cases for AuthService
func TestLogin(t *testing.T) {
	// Create a mock repository
	mockRepo := new(MockAuthRepo)
	logger := slog.Default()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			SecretKey:       "test-access-secret",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 7 * 24 * time.Hour,
			Issuer:          "test-issuer",
			Audience:        "test-audience",
		},
	}
	service := NewAuthService(mockRepo, cfg, logger)

	// Test case: successful login
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		email := "test@example.com"
		password := "password123"
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

		user := &api.UserAuth{
			ID:       "user123",
			Username: "testuser",
			Email:    email,
			Password: string(hashedPassword),
		}

		// Set up expectations
		mockRepo.On("GetUserByEmail", ctx, email).Return(user, nil).Once()
		mockRepo.On("StoreRefreshToken", ctx, user.ID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil).Once()

		// Call the service method
		accessToken, refreshToken, err := service.Login(ctx, email, password)

		// Assert expectations
		assert.NoError(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)
		mockRepo.AssertExpectations(t)
	})

	// Test case: user not found
	t.Run("UserNotFound", func(t *testing.T) {
		ctx := context.Background()
		email := "nonexistent@example.com"
		password := "password123"

		// Set up expectations
		mockRepo.On("GetUserByEmail", ctx, email).Return(nil, api.ErrNotFound).Once()

		// Call the service method
		accessToken, refreshToken, err := service.Login(ctx, email, password)

		// Assert expectations
		assert.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
		assert.ErrorIs(t, err, api.ErrUnauthenticated)
		mockRepo.AssertExpectations(t)
	})

	// Test case: invalid password
	t.Run("InvalidPassword", func(t *testing.T) {
		ctx := context.Background()
		email := "test@example.com"
		password := "wrongpassword"
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)

		user := &api.UserAuth{
			ID:       "user123",
			Username: "testuser",
			Email:    email,
			Password: string(hashedPassword),
		}

		// Set up expectations
		mockRepo.On("GetUserByEmail", ctx, email).Return(user, nil).Once()

		// Call the service method
		accessToken, refreshToken, err := service.Login(ctx, email, password)

		// Assert expectations
		assert.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
		assert.ErrorIs(t, err, api.ErrUnauthenticated)
		mockRepo.AssertExpectations(t)
	})
}

func TestRegister(t *testing.T) {
	// Create a mock repository
	mockRepo := new(MockAuthRepo)
	logger := slog.Default()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			SecretKey:       "test-access-secret",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 7 * 24 * time.Hour,
			Issuer:          "test-issuer",
			Audience:        "test-audience",
		},
	}
	service := NewAuthService(mockRepo, cfg, logger)

	// Test case: successful registration
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		username := "newuser"
		email := "new@example.com"
		password := "password123"
		userID := "new-user-id"

		// Set up expectations - we can't predict the exact hashed password, so use mock.AnythingOfType
		mockRepo.On("Register", ctx, username, email, mock.AnythingOfType("string")).Return(userID, nil).Once()

		// Call the service method
		err := service.Register(ctx, username, email, password, "user")

		// Assert expectations
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	// Test case: email already exists
	t.Run("EmailExists", func(t *testing.T) {
		ctx := context.Background()
		username := "existinguser"
		email := "existing@example.com"
		password := "password123"

		// Set up expectations
		mockRepo.On("Register", ctx, username, email, mock.AnythingOfType("string")).Return("", api.ErrConflict).Once()

		// Call the service method
		err := service.Register(ctx, username, email, password, "user")

		// Assert expectations
		assert.Error(t, err)
		assert.ErrorIs(t, err, api.ErrConflict)
		mockRepo.AssertExpectations(t)
	})
}

func TestLogout(t *testing.T) {
	// Create a mock repository
	mockRepo := new(MockAuthRepo)
	logger := slog.Default()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			SecretKey:       "test-access-secret",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 7 * 24 * time.Hour,
			Issuer:          "test-issuer",
			Audience:        "test-audience",
		},
	}
	service := NewAuthService(mockRepo, cfg, logger)

	// Test case: successful logout
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		refreshToken := "valid-refresh-token"

		// Set up expectations
		mockRepo.On("InvalidateRefreshToken", ctx, refreshToken).Return(nil).Once()

		// Call the service method
		err := service.Logout(ctx, refreshToken)

		// Assert expectations
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	// Test case: error invalidating token
	t.Run("Error", func(t *testing.T) {
		ctx := context.Background()
		refreshToken := "invalid-refresh-token"
		expectedError := errors.New("database error")

		// Set up expectations
		mockRepo.On("InvalidateRefreshToken", ctx, refreshToken).Return(expectedError).Once()

		// Call the service method
		err := service.Logout(ctx, refreshToken)

		// Assert expectations
		assert.Error(t, err)
		assert.Contains(t, err.Error(), expectedError.Error())
		mockRepo.AssertExpectations(t)
	})
}
