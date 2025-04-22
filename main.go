// cmd/server/main.go
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

	router "github.com/FACorreiaa/go-poi-au-suggestions/internal/router"

	database "github.com/FACorreiaa/go-poi-au-suggestions/app/db"
	l "github.com/FACorreiaa/go-poi-au-suggestions/app/logger"

	"github.com/go-chi/chi/v5" // Import chi directly
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"

	appMiddleware "github.com/FACorreiaa/go-poi-au-suggestions/app/middleware"
	"github.com/FACorreiaa/go-poi-au-suggestions/config"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/container"
)

func main() {
	// --- Initial Loading ---
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
	slog.SetDefault(logger)

	// --- Application Context & Shutdown ---
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// --- Initialize Container ---
	c, err := container.NewContainer(&cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize container", slog.Any("error", err))
		os.Exit(1)
	}
	defer c.Close() // Ensure resources are closed on exit

	// --- Wait for Database ---
	if !c.WaitForDB(ctx) {
		logger.Error("Database not ready after waiting, exiting.")
		os.Exit(1)
	}

	// --- Run Migrations ---
	dbConfig, err := database.NewDatabaseConfig(&cfg, logger)
	if err != nil {
		logger.Error("Failed to generate database config", slog.Any("error", err))
		os.Exit(1)
	}

	err = c.RunMigrations(dbConfig.ConnectionURL)
	if err != nil {
		logger.Error("Failed to run database migrations", slog.Any("error", err))
		os.Exit(1) // Exit if migrations fail
	}

	// --- Router Setup ---
	routerConfig := &router.Config{
		AuthHandler:            c.AuthHandler,
		AuthenticateMiddleware: appMiddleware.Authenticate, // Your JWT auth middleware
		// Add other handlers...
	}
	mainRouter := router.SetupRouter(routerConfig)

	// --- Server-Wide Middleware Setup ---
	rootRouter := chi.NewMux()
	rootRouter.Use(chiMiddleware.RequestID)
	rootRouter.Use(chiMiddleware.RealIP)
	rootRouter.Use(l.StructuredLogger(logger)) // Your slog middleware
	rootRouter.Use(chiMiddleware.Recoverer)
	rootRouter.Use(chiMiddleware.StripSlashes)
	rootRouter.Use(chiMiddleware.Timeout(60 * time.Second))
	rootRouter.Use(chiMiddleware.Compress(5, "application/json")) // Added compress back
	// Mount your application router under the root router
	rootRouter.Mount("/", mainRouter)

	// --- HTTP Server Setup ---
	serverAddress := fmt.Sprintf(":%s", cfg.Server.HTTPPort)
	srv := &http.Server{
		Addr:         serverAddress,
		Handler:      rootRouter, // Use the root router
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	// --- Start Server Goroutine & Graceful Shutdown ---
	go func() {
		logger.Info("Starting HTTP server", slog.String("address", serverAddress))
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server ListenAndServe error", slog.Any("error", err))
			cancel()
		}
	}()

	<-ctx.Done()

	logger.Info("Shutdown signal received, starting graceful shutdown...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server graceful shutdown failed", slog.Any("error", err))
	} else {
		logger.Info("HTTP server gracefully stopped")
	}

	logger.Info("Application shut down complete.")
}

// setupLogger function remains the same
func setupLogger() *slog.Logger {
	var logger *slog.Logger
	env := os.Getenv("APP_ENV")
	if env == "development" || env == "" {
		tintOpts := &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
			AddSource:  true,
		}
		logger = slog.New(tint.NewHandler(os.Stdout, tintOpts))
		log.Println("Initialized development logger (tint)")
	} else {
		jsonOpts := &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: false,
		}
		logger = slog.New(slog.NewJSONHandler(os.Stdout, jsonOpts))
		log.Println("Initialized production logger (JSON)")
	}
	return logger
}
