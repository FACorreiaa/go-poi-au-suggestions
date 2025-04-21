package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	database "github.com/FACorreiaa/go-poi-au-suggestions/app/db"
	appLogger "github.com/FACorreiaa/go-poi-au-suggestions/app/logger"
	appMiddleware "github.com/FACorreiaa/go-poi-au-suggestions/app/middleware"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	api "github.com/FACorreiaa/go-poi-au-suggestions/internal/router"

	"github.com/FACorreiaa/go-poi-au-suggestions/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
)

func main() {
	// --- Initial Loading ---
	// Use standard log until slog is configured, in case godotenv fails
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found or error loading:", err)
	}

	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatalf("FATAL: Error initializing config: %v", err)
	}

	// --- Logger Setup ---
	logger := setupLogger()
	slog.SetDefault(logger) // Set globally after initialization

	// --- Application Context & Shutdown ---
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// --- Database Setup ---
	dbConfig, err := database.NewDatabaseConfig(&cfg, logger)
	if err != nil {
		logger.Error("Failed to generate database config", slog.Any("error", err))
		os.Exit(1)
	}

	// Run migrations *before* initializing the main pool
	err = database.RunMigrations(dbConfig.ConnectionURL, logger)
	if err != nil {
		logger.Error("Failed to run database migrations", slog.Any("error", err))
		os.Exit(1) // Exit if migrations fail
	}

	// Initialize connection pool
	pool, err := database.Init(dbConfig.ConnectionURL, logger)
	if err != nil {
		logger.Error("Failed to initialize database pool", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close() // Ensure pool is closed on exit

	// Wait for DB to be ready (optional but good practice)
	if !database.WaitForDB(ctx, pool, logger) {
		logger.Error("Database not ready after waiting, exiting.")
		os.Exit(1)
	}

	// --- Dependency Injection (Example) ---
	// Initialize services, handlers, etc., injecting dependencies like pool and logger
	authRepo := auth.NewAuthRepoFactory(pool)
	authService := auth.NewAuthService(authRepo)
	authHandler := auth.NewAuthHandler(authService)

	// --- Router Setup ---
	// Assume SetupRouter initializes handlers and wires routes
	// router := api.SetupRouter(pool, logger /*, other services/handlers */)
	// --- Temporary Router Setup (Replace with actual SetupRouter call) ---
	routerConfig := &api.Config{
		AuthHandler:            authHandler,
		AuthenticateMiddleware: appMiddleware.Authenticate,
		// Add other handlers here:
		// POIHandler: poiHandler,
	}
	// Create the main application router by calling the setup function
	mainRouter := api.SetupRouter(routerConfig)

	router := chi.NewMux() // Use NewMux for Chi v5
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(appLogger.StructuredLogger(logger)) // Use your slog middleware
	router.Use(middleware.Recoverer)
	router.Use(middleware.StripSlashes)
	router.Use(middleware.Timeout(60 * time.Second))       // Example timeout
	router.Use(middleware.Compress(5, "application/json")) // Compress JSON responses
	router.Mount("/", mainRouter)

	// Example public route
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(r.Context(), "Root endpoint hit")
		w.Write([]byte("Welcome to WanderWiseAI API"))
	})
	// Mount actual API handlers (replace placeholders)
	// router.Mount("/api/v1/auth", api.AuthRoutes(authHandler))
	// router.Group(func(r chi.Router) {
	// 	r.Use(appMiddleware.Authenticate)
	// 	r.Mount("/api/v1/users", api.UserRoutes(...))
	// })
	// --- End Temporary Router Setup ---

	// --- HTTP Server Setup ---
	serverAddress := fmt.Sprintf(":%s", cfg.Server.HTTPPort)
	srv := &http.Server{
		Addr:         serverAddress,
		Handler:      router, // Use your Chi router
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError), // Pipe server errors to slog
	}

	// --- Start Server Goroutine ---
	go func() {
		logger.Info("Starting HTTP server", slog.String("address", serverAddress))
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server ListenAndServe error", slog.Any("error", err))
			cancel() // Trigger shutdown if server fails unexpectedly
		}
	}()

	// --- Wait for Shutdown Signal ---
	<-ctx.Done() // Block until context is cancelled (Ctrl+C, SIGTERM)

	// --- Graceful Shutdown ---
	logger.Info("Shutdown signal received, starting graceful shutdown...")

	// Create a context with timeout for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second) // 10 seconds to shutdown
	defer shutdownCancel()

	// Attempt to gracefully shut down the HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server graceful shutdown failed", slog.Any("error", err))
	} else {
		logger.Info("HTTP server gracefully stopped")
	}

	// Pool is closed by defer statement earlier
	logger.Info("Application shut down complete.")
}

// setupLogger configures and returns the application logger.
func setupLogger() *slog.Logger {
	var logger *slog.Logger
	env := os.Getenv("APP_ENV")

	if env == "development" || env == "" { // Default to development if not set
		// Colored logs for development
		tintOpts := &tint.Options{
			Level:      slog.LevelDebug, // More verbose in dev
			TimeFormat: time.Kitchen,
			AddSource:  true, // Show file:line
		}
		logger = slog.New(tint.NewHandler(os.Stdout, tintOpts))
		log.Println("Initialized development logger (tint)") // Use standard log before slog default is set
	} else {
		// JSON logs for production or other environments
		jsonOpts := &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: false, // Don't add source in prod unless needed for specific errors
		}
		logger = slog.New(slog.NewJSONHandler(os.Stdout, jsonOpts))
		log.Println("Initialized production logger (JSON)")
	}
	return logger
}
