package profiles

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types" // Adjust path
	"github.com/google/uuid"                                     // For mocking transaction

	// For mocking transaction
	"github.com/pashagolub/pgxmock/v4" // pgxmock for transaction mocking
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks for Dependencies ---

type MockprofilessRepo struct {
	mock.Mock
	// For transaction testing (if CreateSearchProfileCC uses it)
	pgxmock.PgxPoolIface // Embed PgxPoolIface for transaction mocking
}

// Implement profilessRepo methods
func (m *MockprofilessRepo) GetSearchProfiles(ctx context.Context, userID uuid.UUID) ([]types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.UserPreferenceProfileResponse), args.Error(1)
}
func (m *MockprofilessRepo) GetSearchProfile(ctx context.Context, userID, profileID uuid.UUID) (*types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserPreferenceProfileResponse), args.Error(1)
}
func (m *MockprofilessRepo) GetDefaultSearchProfile(ctx context.Context, userID uuid.UUID) (*types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserPreferenceProfileResponse), args.Error(1)
}
func (m *MockprofilessRepo) CreateSearchProfile(ctx context.Context, userID uuid.UUID, params types.CreateUserPreferenceProfileParams) (*types.UserPreferenceProfileResponse, error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.UserPreferenceProfileResponse), args.Error(1)
}
func (m *MockprofilessRepo) UpdateSearchProfile(ctx context.Context, userID, profileID uuid.UUID, params types.UpdateSearchProfileParams) error {
	args := m.Called(ctx, userID, profileID, params)
	return args.Error(0)
}
func (m *MockprofilessRepo) DeleteSearchProfile(ctx context.Context, userID, profileID uuid.UUID) error {
	args := m.Called(ctx, userID, profileID)
	return args.Error(0)
}
func (m *MockprofilessRepo) SetDefaultSearchProfile(ctx context.Context, userID, profileID uuid.UUID) error {
	args := m.Called(ctx, userID, profileID)
	return args.Error(0)
}

// Implement pgxmock.PgxPoolIface methods if needed for transaction testing, or ensure your
// actual PostgresprofilessRepo has a way to provide a *pgxpool.Pool for the service to Begin Tx.
// For simplicity in unit tests of CreateSearchProfileCC, we might mock the Begin/Commit/Rollback behavior
// if the repo itself doesn't expose the pool directly but has a method to start a Tx.
// If your service does `s.prefRepo.(*PostgresprofilessRepo).pgpool.Begin(ctx)`,
// then MockprofilessRepo needs to support returning a mock pool or mock transaction.

// --- Mock interestsRepo ---
type MockinterestsRepo struct {
	mock.Mock
}

// Implement methods from interests.interestsRepo used by profilessServiceImpl
func (m *MockinterestsRepo) AddInterestToProfile(ctx context.Context, profileID uuid.UUID, interestID uuid.UUID) error {
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
func (m *MockinterestsRepo) GetInterest(ctx context.Context, interestID uuid.UUID) (*types.Interest, error) {
	args := m.Called(ctx, interestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Interest), args.Error(1)
}
func (m *MockinterestsRepo) GetAllInterests(ctx context.Context) ([]*types.Interest, error) { // Added for CreateSearchProfileCC
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Interest), args.Error(1)
}

// --- Mock tagsRepo ---
type MocktagsRepo struct {
	mock.Mock
}

// Implement methods from tags.tagsRepo used
func (m *MocktagsRepo) LinkPersonalTagToProfile(ctx context.Context, userID uuid.UUID, profileID uuid.UUID, tagID uuid.UUID) error {
	args := m.Called(ctx, userID, profileID, tagID)
	return args.Error(0)
}
func (m *MocktagsRepo) GetTagsForProfile(ctx context.Context, profileID uuid.UUID) ([]*types.Tags, error) {
	args := m.Called(ctx, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Tags), args.Error(1)
}
func (m *MocktagsRepo) Get(ctx context.Context, userID uuid.UUID, tagID uuid.UUID) (*types.Tags, error) {
	args := m.Called(ctx, userID, tagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Tags), args.Error(1)
}
func (m *MocktagsRepo) GetAll(ctx context.Context, userID uuid.UUID) ([]*types.Tags, error) { // Added for CreateSearchProfileCC
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Tags), args.Error(1)
}

// Helper
func setupprofilessServiceTest() (*profilessServiceImpl, *MockprofilessRepo, *MockinterestsRepo, *MocktagsRepo) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})) // Use LevelError to reduce noise
	mockPrefRepo := new(MockprofilessRepo)
	mockIntRepo := new(MockinterestsRepo)
	mockTagRepo := new(MocktagsRepo)
	service := NewUserProfilesService(mockPrefRepo, mockIntRepo, mockTagRepo, logger)
	return service, mockPrefRepo, mockIntRepo, mockTagRepo
}

func TestprofilessServiceImpl_GetSearchProfile(t *testing.T) {
	service, mockPrefRepo, _, _ := setupprofilessServiceTest()
	ctx := context.Background()
	userID := uuid.New()
	profileID := uuid.New()

	t.Run("success", func(t *testing.T) {
		expectedProfile := &types.UserPreferenceProfileResponse{ID: profileID, UserID: userID, ProfileName: "Test Profile"}
		mockPrefRepo.On("GetSearchProfile", ctx, userID, profileID).Return(expectedProfile, nil).Once()

		profile, err := service.GetSearchProfile(ctx, userID, profileID)
		require.NoError(t, err)
		assert.Equal(t, expectedProfile, profile)
		mockPrefRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		repoErr := errors.New("db error fetching profile")
		mockPrefRepo.On("GetSearchProfile", ctx, userID, profileID).Return(nil, repoErr).Once()

		_, err := service.GetSearchProfile(ctx, userID, profileID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, repoErr))
		assert.Contains(t, err.Error(), "error fetching user preference profile:")
		mockPrefRepo.AssertExpectations(t)
	})
}

// Add similar unit tests for:
// - GetSearchProfiles
// - GetDefaultSearchProfile
// - UpdateSearchProfile
// - DeleteSearchProfile
// - SetDefaultSearchProfile

// Unit tests for CreateSearchProfile (the simpler version first)
func TestprofilessServiceImpl_CreateSearchProfile(t *testing.T) {
	service, mockPrefRepo, mockIntRepo, mockTagRepo := setupprofilessServiceTest()
	ctx := context.Background()
	userID := uuid.New()
	profileName := "My Travel Style"
	interestID1 := uuid.New()
	tagID1 := uuid.New()

	params := types.CreateUserPreferenceProfileParams{
		ProfileName: profileName,
		Interests:   []uuid.UUID{interestID1},
		Tags:        []uuid.UUID{tagID1},
		// ... other params
	}
	createdCoreProfile := &types.UserPreferenceProfileResponse{
		ID:          uuid.New(),
		UserID:      userID,
		ProfileName: profileName,
		// ... other core fields populated by repo.CreateSearchProfile
	}

	t.Run("success - simple create with associations", func(t *testing.T) {
		// Mock transaction behavior for PostgresprofilessRepo.pgpool.Begin
		// This is tricky if the service directly accesses pgpool. It's better if the repo handles transactions.
		// For now, assuming CreateSearchProfile in repo doesn't start its own transaction.
		// And the service's transaction logic is what we are testing.

		// Mock validation calls
		mockIntRepo.On("GetInterest", ctx, interestID1).Return(&types.Interest{ID: interestID1, Name: "Hiking"}, nil).Once()
		mockTagRepo.On("Get", ctx, userID, tagID1).Return(&types.Tags{ID: tagID1, Name: "Mountains"}, nil).Once()

		// Mock repo.CreateSearchProfile
		mockPrefRepo.On("CreateSearchProfile", ctx, userID, params).Return(createdCoreProfile, nil).Once()

		// Mock linking calls (these happen inside the transaction in the service)
		// To test the transactional version (CreateSearchProfileCC), we need to mock Begin, Commit, Rollback
		// and the repo methods called within. This is complex with testify/mock alone for pgx transactions.
		// Using pgxmock for `mockPrefRepo` if it was setup with `pgxmock.NewPool()` would be better.

		// For the simpler CreateSearchProfile (not CC version):
		// It directly calls repo.CreateSearchProfile, then AddInterestToProfile, LinkPersonalTagToProfile
		// THEN fetches. This order needs to be mocked.

		mockIntRepo.On("AddInterestToProfile", ctx, createdCoreProfile.ID, interestID1).Return(nil).Once()
		mockTagRepo.On("LinkPersonalTagToProfile", ctx, userID, createdCoreProfile.ID, tagID1).Return(nil).Once()

		// Mock fetching associated data for the response
		mockIntRepo.On("GetInterestsForProfile", ctx, createdCoreProfile.ID).Return([]*types.Interest{{ID: interestID1, Name: "Hiking"}}, nil).Once()
		mockTagRepo.On("GetTagsForProfile", ctx, createdCoreProfile.ID).Return([]*types.Tags{{ID: tagID1, Name: "Mountains"}}, nil).Once()

		// Mock transaction parts - this is where it gets hard if service has `s.prefRepo.(*PostgresprofilessRepo).pgpool.Begin(ctx)`
		// If we are testing `CreateSearchProfile` (not `CreateSearchProfileCC` which has explicit Tx):
		// We assume the repo methods themselves are not transactional in this simple version.

		// --- Setup for CreateSearchProfile (non-CC, transactional version) ---
		// This requires mocking the transaction object itself.
		// Using github.com/pashagolub/pgxmock/v4 for this.
		// First, the service would need to take a pgxpool.Pool or a TxBeginner interface.
		// Let's assume for this unit test, `CreateSearchProfile` is the one *without* explicit errgroup/tx in the service.
		// The `CreateSearchProfileCC` is harder to unit test cleanly without careful mocking of the transaction.

		// Test the `CreateSearchProfile` (the one with TODO fix later)
		profileResponse, err := service.CreateSearchProfile(ctx, userID, params) // Using the one that internally calls repo methods sequentially.
		require.NoError(t, err)
		require.NotNil(t, profileResponse)
		assert.Equal(t, createdCoreProfile.ID, profileResponse.ID)
		assert.Equal(t, profileName, profileResponse.ProfileName)
		require.Len(t, profileResponse.Interests, 1)
		assert.Equal(t, "Hiking", profileResponse.Interests[0].Name)
		require.Len(t, profileResponse.Tags, 1)
		assert.Equal(t, "Mountains", profileResponse.Tags[0].Name)

		mockPrefRepo.AssertExpectations(t)
		mockIntRepo.AssertExpectations(t)
		mockTagRepo.AssertExpectations(t)
	})

	t.Run("CreateSearchProfile - empty profile name", func(t *testing.T) {
		emptyNameParams := types.CreateUserPreferenceProfileParams{ProfileName: ""}
		_, err := service.CreateSearchProfile(ctx, userID, emptyNameParams)
		require.Error(t, err)
		assert.True(t, errors.Is(err, types.ErrBadRequest))
		assert.Contains(t, err.Error(), "profile name cannot be empty")
	})

	t.Run("CreateSearchProfile - invalid interest ID", func(t *testing.T) {
		invalidInterestID := uuid.New()
		paramsWithInvalidInterest := types.CreateUserPreferenceProfileParams{
			ProfileName: "TestInvalidInterest",
			Interests:   []uuid.UUID{invalidInterestID},
		}
		repoErr := fmt.Errorf("interest %s not found", invalidInterestID) // Mock this error
		mockIntRepo.On("GetInterest", ctx, invalidInterestID).Return(nil, repoErr).Once()

		_, err := service.CreateSearchProfile(ctx, userID, paramsWithInvalidInterest)
		require.Error(t, err)
		assert.True(t, errors.Is(err, types.ErrNotFound))
		assert.Contains(t, err.Error(), fmt.Sprintf("invalid interest %s", invalidInterestID))
		mockIntRepo.AssertExpectations(t) // Ensure GetInterest was called
		// mockPrefRepo.AssertNotCalled(t, "CreateSearchProfile") // Base profile shouldn't be created
	})

	// TODO: Add tests for CreateSearchProfileCC (the one with errgroup and explicit transaction)
	// This will require more advanced mocking for the transaction flow (Begin, Commit, Rollback)
	// using a library like pgxmock if your prefRepo exposes its pgxpool.Pool.
	// Or, the transaction logic should ideally be *within* the repository method itself,
	// making the service easier to test (service just calls repo.CreateProfileWithAssociations).
}

// ... other unit tests for Update, Delete, SetDefault ...
