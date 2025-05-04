package userSearchProfile

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	userInterest "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_interests"
	userTags "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_tags"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

// Ensure implementation satisfies the interface
var _ UserSearchProfilesService = (*UserSearchProfilesServiceImpl)(nil)

// UserSearchProfilesService defines the business logic contract for user operations.
type UserSearchProfilesService interface {
	//GetUserPreferenceProfiles User  Profiles
	GetUserPreferenceProfiles(ctx context.Context, userID uuid.UUID) ([]types.UserPreferenceProfileResponse, error)
	GetUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) (*types.UserPreferenceProfileResponse, error)
	GetDefaultUserPreferenceProfile(ctx context.Context, userID uuid.UUID) (*types.UserPreferenceProfileResponse, error)
	CreateProfile(ctx context.Context, userID uuid.UUID, params types.CreateUserPreferenceProfileParams) (*types.UserPreferenceProfileResponse, error)
	UpdateUserPreferenceProfile(ctx context.Context, profileID uuid.UUID, params types.UpdateUserPreferenceProfileParams) error
	DeleteUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error
	SetDefaultUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error
}

// UserServiceImpl provides the implementation for UserService.
type UserSearchProfilesServiceImpl struct {
	logger   *slog.Logger
	prefRepo UserSearchProfilesRepo
	intRepo  userInterest.UserInterestRepo
	tagRepo  userTags.UserTagsRepo
}

func NewUserProfilesService(prefRepo UserSearchProfilesRepo, intRepo userInterest.UserInterestRepo, tagRepo userTags.UserTagsRepo, logger *slog.Logger) *UserSearchProfilesServiceImpl {
	return &UserSearchProfilesServiceImpl{
		prefRepo: prefRepo,
		intRepo:  intRepo,
		tagRepo:  tagRepo,
		logger:   logger,
	}
}

// GetUserPreferenceProfiles retrieves all preference profiles for a user.
func (s *UserSearchProfilesServiceImpl) GetUserPreferenceProfiles(ctx context.Context, userID uuid.UUID) ([]types.UserPreferenceProfileResponse, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetUserPreferenceProfiles", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetUserPreferenceProfiles"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching user preference profiles")

	profiles, err := s.prefRepo.GetProfiles(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user preference profiles", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user preference profiles")
		return nil, fmt.Errorf("error fetching user preference profiles: %w", err)
	}

	l.InfoContext(ctx, "User preference profiles fetched successfully", slog.Int("count", len(profiles)))
	span.SetStatus(codes.Ok, "User preference profiles fetched successfully")
	return profiles, nil
}

// GetUserPreferenceProfile retrieves a specific preference profile by ID.
func (s *UserSearchProfilesServiceImpl) GetUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) (*types.UserPreferenceProfileResponse, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetUserPreferenceProfile", trace.WithAttributes(
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Fetching user preference profile")

	profile, err := s.prefRepo.GetProfile(ctx, profileID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch user preference profile")
		return nil, fmt.Errorf("error fetching user preference profile: %w", err)
	}

	l.InfoContext(ctx, "User preference profile fetched successfully")
	span.SetStatus(codes.Ok, "User preference profile fetched successfully")
	return profile, nil
}

// GetDefaultUserPreferenceProfile retrieves the default preference profile for a user.
func (s *UserSearchProfilesServiceImpl) GetDefaultUserPreferenceProfile(ctx context.Context, userID uuid.UUID) (*types.UserPreferenceProfileResponse, error) {
	ctx, span := otel.Tracer("UserService").Start(ctx, "GetDefaultUserPreferenceProfile", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "GetDefaultUserPreferenceProfile"), slog.String("userID", userID.String()))
	l.DebugContext(ctx, "Fetching default user preference profile")

	profile, err := s.prefRepo.GetProfile(ctx, userID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch default user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch default user preference profile")
		return nil, fmt.Errorf("error fetching default user preference profile: %w", err)
	}

	l.InfoContext(ctx, "Default user preference profile fetched successfully")
	span.SetStatus(codes.Ok, "Default user preference profile fetched successfully")
	return profile, nil
}

// CreateProfile creates a new preference profile for a user.
//func (s *UserProfilesServiceImpl) CreateProfile(ctx context.Context, userID uuid.UUID, params api.CreateUserPreferenceProfileParams) (*api.UserPreferenceProfile, error) {
//	ctx, span := otel.Tracer("UserService").Start(ctx, "CreateUserPreferenceProfile", trace.WithAttributes(
//		attribute.String("user.id", userID.String()),
//	))
//	defer span.End()
//
//	l := s.logger.With(slog.String("method", "CreateUserPreferenceProfile"), slog.String("userID", userID.String()))
//	l.DebugContext(ctx, "Creating user preference profile", slog.String("profileName", params.ProfileName))
//
//	profile, err := s.prefRepo.CreateProfile(ctx, userID, params)
//	if err != nil {
//		l.ErrorContext(ctx, "Failed to create user preference profile", slog.Any("error", err))
//		span.RecordError(err)
//		span.SetStatus(codes.Error, "Failed to create user preference profile")
//		return nil, fmt.Errorf("error creating user preference profile: %w", err)
//	}
//
//	l.InfoContext(ctx, "User preference profile created successfully", slog.String("profileID", profile.ID.String()))
//	span.SetStatus(codes.Ok, "User preference profile created successfully")
//	return profile, nil
//}

// CreateProfile TODO fix Create profile interests and tags
// func (s *UserSearchProfilesServiceImpl) CreateProfile(ctx context.Context, userID uuid.UUID, params types.CreateUserPreferenceProfileParams) (*types.UserPreferenceProfileResponse, error) { // Return the richer response type
//
//		ctx, span := otel.Tracer("PreferenceService").Start(ctx, "CreateProfile")
//		defer span.End()
//
//		l := s.logger.With(slog.String("method", "CreateProfile"), slog.String("userID", userID.String()), slog.String("profileName", params.ProfileName))
//		l.DebugContext(ctx, "Attempting to create profile and link associations")
//
//		// --- 1. Validate input further if needed (e.g., check if profile name is empty) ---
//		if params.ProfileName == "" {
//			return nil, fmt.Errorf("%w: profile name cannot be empty", types.ErrBadRequest)
//		}
//
//		tx, err := s.prefRepo.(*PostgresUserSearchProfilesRepo).pgpool.Begin(ctx)
//		if err != nil {
//			l.ErrorContext(ctx, "Failed to begin transaction", slog.Any("error", err))
//			span.RecordError(err)
//			span.SetStatus(codes.Error, "Transaction begin failed")
//			return nil, fmt.Errorf("failed to begin transaction: %w", err)
//		}
//		defer tx.Rollback(ctx)
//
//		// --- 2. Create the base profile ---
//		// NOTE: The repo method CreateProfile should ONLY insert into user_preference_profiles
//		// and return the core profile data. It should NOT handle tags/interests.
//		createdProfileCore, err := s.prefRepo.CreateProfile(ctx, userID, params)
//		if err != nil {
//			l.ErrorContext(ctx, "Failed to create base profile in repo", slog.Any("error", err))
//			span.RecordError(err)
//			span.SetStatus(codes.Error, "Repo failed creating profile")
//			// Map repo errors (like ErrConflict) if applicable
//			return nil, fmt.Errorf("failed to create profile core: %w", err)
//		}
//		profileID := createdProfileCore.ID
//		l.InfoContext(ctx, "Base profile created successfully", slog.String("profileID", profileID.String()))
//
//		// --- 3. Link Interests and Avoid Tags Concurrently ---
//		g, childCtx := errgroup.WithContext(ctx)
//
//		// Link Interests
//		if len(params.Interests) > 0 {
//			interestIDs := params.Interests // Capture loop variable
//			g.Go(func() error {
//				l.DebugContext(childCtx, "Linking interests to profile", slog.Int("count", len(interestIDs)), slog.String("profileID", profileID.String()))
//				for _, interestID := range interestIDs {
//					linkErr := s.intRepo.AddInterestToProfile(childCtx, profileID, interestID)
//					if linkErr != nil {
//						// Log the specific error but potentially continue linking others
//						l.ErrorContext(childCtx, "Failed to link interest to profile", slog.String("interestID", interestID.String()), slog.Any("error", linkErr))
//						// Optionally: Collect errors and return an aggregate error at the end
//						// return fmt.Errorf("failed linking interest %s: %w", interestID, linkErr) // Causes errgroup to cancel
//					}
//				}
//				return nil // Success for this goroutine (unless an error was returned above)
//			})
//		}
//
//		// Link Avoid Tags
//		if len(params.Tags) > 0 {
//			tagIDs := params.Tags // Capture loop variable
//			g.Go(func() error {
//				l.DebugContext(childCtx, "Linking avoid tags to profile", slog.Int("count", len(tagIDs)), slog.String("profileID", profileID.String()))
//				for _, tagID := range tagIDs {
//					linkErr := s.tagRepo.AddTagToProfile(childCtx, profileID, tagID)
//					if linkErr != nil {
//						l.ErrorContext(childCtx, "Failed to link avoid tag to profile", slog.String("tagID", tagID.String()), slog.Any("error", linkErr))
//						// return fmt.Errorf("failed linking avoid tag %s: %w", tagID, linkErr) // Causes errgroup to cancel
//					}
//				}
//				return nil // Success for this goroutine
//			})
//		}
//
//		// Wait for linking operations
//		if err := g.Wait(); err != nil {
//			l.ErrorContext(ctx, "Error occurred during interest/tag association", slog.Any("error", err))
//			// Profile was created, but associations might be incomplete.
//			span.RecordError(err)
//			span.SetStatus(codes.Error, "Failed associating items")
//			// Return the created profile data along with the association error?
//			// Or consider rolling back the profile creation (requires full transaction management)?
//			// Returning the error indicating partial success is one option.
//			return createdProfileCore, fmt.Errorf("profile created, but failed associating items: %w", err)
//		}
//
//		// --- 4. Fetch Associated Data for Response (Optional but good UX) ---
//		// After successful creation and linking, fetch the linked data to return the full response object.
//		// Can also run these concurrently.
//		gResp, respCtx := errgroup.WithContext(ctx)
//		var fetchedInterests []types.Interest
//		var fetchedTags []types.Tags
//
//		gResp.Go(func() error {
//			var fetchErr error
//			fetchedInterests, fetchErr = s.intRepo.GetAllInterests(respCtx)
//			l.DebugContext(respCtx, "Fetched interests for response", slog.Int("count", len(fetchedInterests)), slog.Any("error", fetchErr)) // Log count and error
//			return fetchErr                                                                                                                  // Return error if fetching fails
//		})
//
//		gResp.Go(func() error {
//			var fetchErr error
//			fetchedTags, fetchErr = s.tagRepo.GetAll(respCtx, userID)
//			l.DebugContext(respCtx, "Fetched tags for response", slog.Int("count", len(fetchedTags)), slog.Any("error", fetchErr)) // Log count and error
//			return fetchErr
//		})
//
//		if err = gResp.Wait(); err != nil {
//			l.ErrorContext(ctx, "Error occurred fetching associated data for response", slog.Any("error", err))
//			return createdProfileCore, nil
//		}
//
//		if err = tx.Commit(ctx); err != nil {
//			l.ErrorContext(ctx, "Failed to commit transaction", slog.Any("error", err))
//			span.RecordError(err)
//			span.SetStatus(codes.Error, "Transaction commit failed")
//			return nil, fmt.Errorf("failed to commit transaction: %w", err)
//		}
//
//		// --- 5. Assemble Final Response ---
//		fullResponse := &types.UserPreferenceProfileResponse{
//			// Copy fields from createdProfileCore
//			ID:                   createdProfileCore.ID,
//			UserID:               createdProfileCore.UserID,
//			ProfileName:          createdProfileCore.ProfileName,
//			IsDefault:            createdProfileCore.IsDefault,
//			SearchRadiusKm:       createdProfileCore.SearchRadiusKm,
//			PreferredTime:        createdProfileCore.PreferredTime,
//			BudgetLevel:          createdProfileCore.BudgetLevel,
//			PreferredPace:        createdProfileCore.PreferredPace,
//			PreferAccessiblePOIs: createdProfileCore.PreferAccessiblePOIs,
//			PreferOutdoorSeating: createdProfileCore.PreferOutdoorSeating,
//			PreferDogFriendly:    createdProfileCore.PreferDogFriendly,
//			PreferredVibes:       createdProfileCore.PreferredVibes,
//			PreferredTransport:   createdProfileCore.PreferredTransport,
//			DietaryNeeds:         createdProfileCore.DietaryNeeds,
//			CreatedAt:            createdProfileCore.CreatedAt,
//			UpdatedAt:            createdProfileCore.UpdatedAt,
//			Interests:            fetchedInterests,
//			Tags:                 fetchedTags,
//		}
//
//		l.InfoContext(ctx, "Successfully created profile and processed associations")
//		span.SetStatus(codes.Ok, "Profile created with associations")
//		return fullResponse, nil
//	}
//
// userProfiles/service.go
func (s *UserSearchProfilesServiceImpl) CreateProfile(ctx context.Context, userID uuid.UUID, params types.CreateUserPreferenceProfileParams) (*types.UserPreferenceProfileResponse, error) {
	ctx, span := otel.Tracer("PreferenceService").Start(ctx, "CreateProfile", trace.WithAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("profile.name", params.ProfileName),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "CreateProfile"), slog.String("userID", userID.String()), slog.String("profileName", params.ProfileName))
	l.DebugContext(ctx, "Creating user preference profile with associations", slog.Any("tags", params.Tags), slog.Any("interests", params.Interests))

	// Validate input
	if params.ProfileName == "" {
		l.WarnContext(ctx, "Profile name is required")
		span.SetStatus(codes.Error, "Profile name is required")
		return nil, fmt.Errorf("%w: profile name cannot be empty", types.ErrBadRequest)
	}

	// Begin a transaction
	tx, err := s.prefRepo.(*PostgresUserSearchProfilesRepo).pgpool.Begin(ctx)
	if err != nil {
		l.ErrorContext(ctx, "Failed to begin transaction", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Transaction begin failed")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if not committed

	// Create the base profile
	profile, err := s.prefRepo.CreateProfile(ctx, userID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to create base profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Profile creation failed")
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	// Validate and link interests
	var interestObjects []types.Interest
	for _, interestID := range params.Interests {
		// Validate interest exists
		interest, err := s.intRepo.GetInterest(ctx, interestID)
		if err != nil {
			l.ErrorContext(ctx, "Failed to validate interest", slog.String("interestID", interestID.String()), slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Interest validation failed")
			return nil, fmt.Errorf("invalid interest %s: %w", interestID, err)
		}

		// Link interest to profile
		if err := s.intRepo.AddInterestToProfile(ctx, profile.ID, interestID); err != nil {
			l.ErrorContext(ctx, "Failed to link interest to profile", slog.String("interestID", interestID.String()), slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Interest linking failed")
			return nil, fmt.Errorf("failed to link interest %s to profile: %w", interestID, err)
		}

		interestObjects = append(interestObjects, *interest)
	}

	// Validate and link tags
	var tagObjects []types.Tags
	for _, tagID := range params.Tags {
		// Validate tag exists
		tag, err := s.tagRepo.Get(ctx, userID, tagID)
		if err != nil {
			l.ErrorContext(ctx, "Failed to validate tag", slog.String("tagID", tagID.String()), slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Tag validation failed")
			return nil, fmt.Errorf("invalid tag %s: %w", tagID, err)
		}

		// Link tag to profile
		if err := s.tagRepo.LinkPersonalTagToProfile(ctx, userID, profile.ID, tagID); err != nil {
			l.ErrorContext(ctx, "Failed to link tag to profile", slog.String("tagID", tagID.String()), slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Tag linking failed")
			return nil, fmt.Errorf("failed to link tag %s to profile: %w", tagID, err)
		}

		tagObjects = append(tagObjects, *tag)
	}

	// Fetch linked interests and tags (optional, since we already have the objects)
	// This step ensures we return the exact linked data, accounting for any database-side filtering
	fetchedInterests, err := s.intRepo.GetInterestsForProfile(ctx, profile.ID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch linked interests", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Fetch interests failed")
		return nil, fmt.Errorf("failed to fetch linked interests: %w", err)
	}

	fetchedTags, err := s.tagRepo.GetTagsForProfile(ctx, profile.ID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to fetch linked tags", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Fetch tags failed")
		return nil, fmt.Errorf("failed to fetch linked tags: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		l.ErrorContext(ctx, "Failed to commit transaction", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Transaction commit failed")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Assemble the response
	response := &types.UserPreferenceProfileResponse{
		ID:                   profile.ID,
		UserID:               profile.UserID,
		ProfileName:          profile.ProfileName,
		IsDefault:            profile.IsDefault,
		SearchRadiusKm:       profile.SearchRadiusKm,
		PreferredTime:        profile.PreferredTime,
		BudgetLevel:          profile.BudgetLevel,
		PreferredPace:        profile.PreferredPace,
		PreferAccessiblePOIs: profile.PreferAccessiblePOIs,
		PreferOutdoorSeating: profile.PreferOutdoorSeating,
		PreferDogFriendly:    profile.PreferDogFriendly,
		PreferredVibes:       profile.PreferredVibes,
		PreferredTransport:   profile.PreferredTransport,
		DietaryNeeds:         profile.DietaryNeeds,
		Interests:            fetchedInterests, // Use fetched data for consistency
		Tags:                 fetchedTags,      // Use fetched data for consistency
		CreatedAt:            profile.CreatedAt,
		UpdatedAt:            profile.UpdatedAt,
	}

	l.InfoContext(ctx, "User preference profile created successfully", slog.String("profileID", profile.ID.String()), slog.Int("interestCount", len(fetchedInterests)), slog.Int("tagCount", len(fetchedTags)))
	span.SetStatus(codes.Ok, "Profile created with associations")
	return response, nil
}

// UpdateUserPreferenceProfile updates a preference profile.
func (s *UserSearchProfilesServiceImpl) UpdateUserPreferenceProfile(ctx context.Context, profileID uuid.UUID, params types.UpdateUserPreferenceProfileParams) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "UpdateUserPreferenceProfile", trace.WithAttributes(
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "UpdateUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Updating user preference profile")

	err := s.prefRepo.UpdateProfile(ctx, profileID, params)
	if err != nil {
		l.ErrorContext(ctx, "Failed to update user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update user preference profile")
		return fmt.Errorf("error updating user preference profile: %w", err)
	}

	l.InfoContext(ctx, "User preference profile updated successfully")
	span.SetStatus(codes.Ok, "User preference profile updated successfully")
	return nil
}

// DeleteUserPreferenceProfile deletes a preference profile.
func (s *UserSearchProfilesServiceImpl) DeleteUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "DeleteUserPreferenceProfile", trace.WithAttributes(
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "DeleteUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Deleting user preference profile")

	err := s.prefRepo.DeleteProfile(ctx, profileID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to delete user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to delete user preference profile")
		return fmt.Errorf("error deleting user preference profile: %w", err)
	}

	l.InfoContext(ctx, "User preference profile deleted successfully")
	span.SetStatus(codes.Ok, "User preference profile deleted successfully")
	return nil
}

// SetDefaultUserPreferenceProfile sets a profile as the default for a user.
func (s *UserSearchProfilesServiceImpl) SetDefaultUserPreferenceProfile(ctx context.Context, profileID uuid.UUID) error {
	ctx, span := otel.Tracer("UserService").Start(ctx, "SetDefaultUserPreferenceProfile", trace.WithAttributes(
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l := s.logger.With(slog.String("method", "SetDefaultUserPreferenceProfile"), slog.String("profileID", profileID.String()))
	l.DebugContext(ctx, "Setting profile as default")

	err := s.prefRepo.SetDefaultProfile(ctx, profileID)
	if err != nil {
		l.ErrorContext(ctx, "Failed to set default user preference profile", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to set default user preference profile")
		return fmt.Errorf("error setting default user preference profile: %w", err)
	}

	l.InfoContext(ctx, "User preference profile set as default successfully")
	span.SetStatus(codes.Ok, "User preference profile set as default successfully")
	return nil
}
