package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	database "github.com/FACorreiaa/go-poi-au-suggestions/app/db"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/FACorreiaa/go-poi-au-suggestions/config"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/city"
	generativeAI "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/generative_ai"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/poi"
)

func main() {
	ctx := context.Background()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Set up logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Load configuration
	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbConfig, err := database.NewDatabaseConfig(cfg, logger)
	if err != nil {
		logger.Error("Failed to generate database config", slog.Any("error", err))
		os.Exit(1)
	}

	// Set up database connection
	dbpool, err := pgxpool.New(ctx, dbConfig.ConnectionURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbpool.Close()

	// Test database connection
	if err := dbpool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	logger.Info("Connected to database successfully")

	// Initialize services
	embeddingService, err := generativeAI.NewEmbeddingService(ctx, logger)
	if err != nil {
		log.Fatalf("Failed to create embedding service: %v", err)
	}
	defer embeddingService.Close()

	poiRepository := poi.NewRepository(dbpool, logger)
	poiService := poi.NewServiceImpl(poiRepository, embeddingService, logger)

	cityRepository := city.NewCityRepository(dbpool, logger)

	logger.Info("Starting embedding generation for existing data...")

	// Generate embeddings for POIs
	logger.Info("Generating embeddings for POIs...")
	err = poiService.GenerateEmbeddingsForAllPOIs(ctx, 20) // Process in batches of 20
	if err != nil {
		logger.Error("Failed to generate POI embeddings", slog.Any("error", err))
	} else {
		logger.Info("Successfully generated embeddings for all POIs")
	}

	// Generate embeddings for cities
	logger.Info("Generating embeddings for cities...")
	err = generateCityEmbeddings(ctx, embeddingService, cityRepository, logger)
	if err != nil {
		logger.Error("Failed to generate city embeddings", slog.Any("error", err))
	} else {
		logger.Info("Successfully generated embeddings for all cities")
	}

	logger.Info("Embedding generation completed!")
}

func generateCityEmbeddings(ctx context.Context, embeddingService *generativeAI.EmbeddingService, cityRepo city.Repository, logger *slog.Logger) error {
	batchSize := 10
	totalProcessed := 0
	totalErrors := 0

	for {
		// Get batch of cities without embeddings
		cities, err := cityRepo.GetCitiesWithoutEmbeddings(ctx, batchSize)
		if err != nil {
			return fmt.Errorf("failed to get cities without embeddings: %w", err)
		}

		if len(cities) == 0 {
			// No more cities to process
			break
		}

		logger.Info("Processing batch of cities", slog.Int("batch_size", len(cities)))

		// Process each city in the batch
		for _, cityData := range cities {
			// Generate embedding
			embedding, err := embeddingService.GenerateCityEmbedding(ctx, cityData.Name, cityData.Country, cityData.AiSummary)
			if err != nil {
				logger.Error("Failed to generate embedding for city",
					slog.Any("error", err),
					slog.String("city_id", cityData.ID.String()),
					slog.String("city_name", cityData.Name))
				totalErrors++
				continue
			}

			// Update city with embedding
			err = cityRepo.UpdateCityEmbedding(ctx, cityData.ID, embedding)
			if err != nil {
				logger.Error("Failed to update city embedding",
					slog.Any("error", err),
					slog.String("city_id", cityData.ID.String()),
					slog.String("city_name", cityData.Name))
				totalErrors++
				continue
			}

			totalProcessed++
			logger.Debug("City embedding generated successfully",
				slog.String("city_id", cityData.ID.String()),
				slog.String("city_name", cityData.Name))
		}

		// Break if we processed fewer cities than the batch size (end of data)
		if len(cities) < batchSize {
			break
		}
	}

	logger.Info("Batch city embedding generation completed",
		slog.Int("total_processed", totalProcessed),
		slog.Int("total_errors", totalErrors))

	if totalErrors > 0 {
		return fmt.Errorf("city embedding generation completed with %d errors out of %d total cities", totalErrors, totalProcessed+totalErrors)
	}

	return nil
}
