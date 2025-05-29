package llmInteraction

import (
	"fmt"

	"github.com/google/uuid"
)

func generatePOICacheKey(city string, lat, lon float64, userID uuid.UUID) string {
	return fmt.Sprintf("poi:%s:%.6f:%.6f:%s", city, lat, lon, userID.String())
}

func generateHotelCacheKey(city string, lat, lon float64, userID uuid.UUID) string {
	return fmt.Sprintf("hotel:%s:%.6f:%.6f:%s", city, lat, lon, userID.String())
}

func generateRestaurantCacheKey(city string, lat, lon float64, userID uuid.UUID) string {
	return fmt.Sprintf("restaurant:%s:%.6f:%.6f:%s", city, lat, lon, userID.String())
}
