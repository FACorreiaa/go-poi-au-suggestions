package city

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Service interface {
	GetAllCities(ctx context.Context) ([]types.CityDetail, error)
}

type ServiceImpl struct {
	logger *slog.Logger
	repo   Repository
}

func NewCityService(repo Repository, logger *slog.Logger) *ServiceImpl {
	return &ServiceImpl{
		logger: logger,
		repo:   repo,
	}
}

// GetAllCities retrieves all cities from the database
func (s *ServiceImpl) GetAllCities(ctx context.Context) ([]types.CityDetail, error) {
	ctx, span := otel.Tracer("CityService").Start(ctx, "GetAllCities")
	defer span.End()

	l := s.logger.With(slog.String("method", "GetAllCities"))

	l.InfoContext(ctx, "Retrieving all cities from database")

	cities, err := s.repo.GetAllCities(ctx)
	if err != nil {
		l.ErrorContext(ctx, "Failed to retrieve cities from repository", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Repository operation failed")
		return nil, fmt.Errorf("failed to retrieve cities: %w", err)
	}

	l.InfoContext(ctx, "Successfully retrieved cities", slog.Int("count", len(cities)))
	span.SetAttributes(attribute.Int("cities.count", len(cities)))
	span.SetStatus(codes.Ok, "Cities retrieved successfully")

	return cities, nil
}