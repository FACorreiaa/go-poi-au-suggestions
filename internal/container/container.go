package container

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	database "github.com/FACorreiaa/go-poi-au-suggestions/app/db"
	"github.com/FACorreiaa/go-poi-au-suggestions/config"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/city"
	llmInteraction "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/llm_interaction"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/poi"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user"
	userInterest "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_interests"
	userSearchProfile "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_search_profiles"
	userSettings "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_settings"
	userTags "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_tags"
)

// Container holds all application dependencies
type Container struct {
	Config                    *config.Config
	Logger                    *slog.Logger
	Pool                      *pgxpool.Pool
	AuthHandler               *auth.HandlerImpl
	UserHandler               *user.HandlerImpl
	InterestHandler           *userInterest.HandlerImpl
	SettingsHandler           *userSettings.HandlerImpl
	TagsHandler               *userTags.HandlerImpl
	SearchProfileHandler      *userSearchProfile.HandlerImpl
	LLMInteractionHandlerImpl *llmInteraction.HandlerImpl
	POIHandler                *poi.HandlerImpl
	// Add other HandlerImpls, services, and repositories as needed
}

// NewContainer initializes and returns a new dependency container
func NewContainer(cfg *config.Config, logger *slog.Logger) (*Container, error) {
	// Initialize database
	dbConfig, err := database.NewDatabaseConfig(cfg, logger)
	if err != nil {
		logger.Error("Failed to generate database config", slog.Any("error", err))
		return nil, err
	}

	pool, err := database.Init(dbConfig.ConnectionURL, logger)
	if err != nil {
		logger.Error("Failed to initialize database pool", slog.Any("error", err))
		return nil, err
	}

	// Initialize repositories
	authRepo := auth.NewPostgresAuthRepo(pool, logger)

	// Initialize services
	authService := auth.NewAuthService(authRepo, cfg, logger)

	// Initialize HandlerImpls
	authHandlerImpl := auth.NewAuthHandlerImpl(authService, logger)

	//
	userRepo := user.NewPostgresUserRepo(pool, logger)
	userService := user.NewUserService(userRepo, logger)
	userHandlerImpl := user.NewHandlerImpl(userService, logger)

	userInterestRepo := userInterest.NewPostgresUserInterestRepo(pool, logger)
	userInterestService := userInterest.NewUserInterestService(userInterestRepo, logger)
	HandlerImpl := userInterest.NewHandlerImpl(userInterestService, logger)

	userSettingsRepo := userSettings.NewPostgresUserSettingsRepo(pool, logger)
	userSettingsService := userSettings.NewUserSettingsService(userSettingsRepo, logger)
	userSettingsHandler := userSettings.NewHandlerImpl(userSettingsService, logger)

	userTagsRepo := userTags.NewPostgresUserTagsRepo(pool, logger)
	userTagsService := userTags.NewUserTagsService(userTagsRepo, logger)
	userTagsHandler := userTags.NewHandlerImpl(userTagsService, logger)

	userSearchProfilesRepo := userSearchProfile.NewPostgresUserRepo(pool, logger)
	userSearchProfilesService := userSearchProfile.NewUserProfilesService(userSearchProfilesRepo, userInterestRepo, userTagsRepo, logger)
	userSearchProfilesHandlerImpl := userSearchProfile.NewUserHandlerImpl(userSearchProfilesService, logger)
	// Create and return the container

	// city repository
	cityRepo := city.NewCityRepository(pool, logger)

	poiRepo := poi.NewPOIRepository(pool, logger)
	// initialise the LLM interaction service
	llmInteractionRepo := llmInteraction.NewPostgresLlmInteractionRepo(pool, logger)
	llmInteractionService := llmInteraction.NewLlmInteractiontService(userInterestRepo,
		userSearchProfilesRepo,
		userTagsRepo,
		llmInteractionRepo,
		cityRepo,
		poiRepo,
		logger)
	llmInteractionHandlerImpl := llmInteraction.NewLLMHandlerImpl(llmInteractionService, logger)

	poiRepository := poi.NewPOIRepository(pool, logger)
	poiService := poi.NewPOIServiceImpl(poiRepository, logger)
	poiHandler := poi.NewHandlerImpl(poiService, logger)
	return &Container{
		Config:                    cfg,
		Logger:                    logger,
		Pool:                      pool,
		AuthHandler:               authHandlerImpl,
		UserHandler:               userHandlerImpl,
		InterestHandler:           HandlerImpl,
		SettingsHandler:           userSettingsHandler,
		TagsHandler:               userTagsHandler,
		SearchProfileHandler:      userSearchProfilesHandlerImpl,
		LLMInteractionHandlerImpl: llmInteractionHandlerImpl,
		POIHandler:                poiHandler,
		// Add other HandlerImpls, services, and repositories as needed
	}, nil
}

// Close releases all resources held by the container
func (c *Container) Close() {
	if c.Pool != nil {
		c.Pool.Close()
	}
}

// WaitForDB waits for the database to be ready
func (c *Container) WaitForDB(ctx context.Context) bool {
	return database.WaitForDB(ctx, c.Pool, c.Logger)
}

// RunMigrations runs database migrations
func (c *Container) RunMigrations(connectionURL string) error {
	return database.RunMigrations(connectionURL, c.Logger)
}
