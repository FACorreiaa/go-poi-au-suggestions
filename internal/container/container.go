package container

import (
	"context"
	"log/slog"

	database "github.com/FACorreiaa/go-poi-au-suggestions/app/db"
	"github.com/FACorreiaa/go-poi-au-suggestions/config"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Container holds all application dependencies
type Container struct {
	Config      *config.Config
	Logger      *slog.Logger
	Pool        *pgxpool.Pool
	AuthHandler *auth.AuthHandler
	// Add other handlers, services, and repositories as needed
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

	// Initialize handlers
	authHandler := auth.NewAuthHandler(authService, logger)

	// Create and return the container
	return &Container{
		Config:      cfg,
		Logger:      logger,
		Pool:        pool,
		AuthHandler: authHandler,
		// Add other handlers, services, and repositories as needed
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
