package llmChat

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/google/uuid"
)

func generatePOICacheKey(city string, lat, lon, distance float64, userID uuid.UUID) string {
	return fmt.Sprintf("poi:%s:%f:%f:%f:%s", city, lat, lon, distance, userID.String())
}

func generateHotelCacheKey(city string, lat, lon float64, userID uuid.UUID) string {
	return fmt.Sprintf("hotel:%s:%.6f:%.6f:%s", city, lat, lon, userID.String())
}

func generateRestaurantCacheKey(city string, lat, lon float64, userID uuid.UUID) string {
	return fmt.Sprintf("restaurant:%s:%.6f:%.6f:%s", city, lat, lon, userID.String())
}

func cleanJSONResponse(response string) string {
	response = strings.TrimSpace(response)

	// Remove markdown code block markers
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}

	if strings.HasSuffix(response, "```") {
		response = strings.TrimSuffix(response, "```")
	}

	response = strings.TrimSpace(response)

	// Extract JSON from response that might contain explanatory text
	// Look for the first { and last } to extract the JSON object
	firstBrace := strings.Index(response, "{")
	if firstBrace == -1 {
		return response // No JSON found, return as is
	}

	lastBrace := strings.LastIndex(response, "}")
	if lastBrace == -1 || lastBrace <= firstBrace {
		return response // No valid JSON structure found
	}

	// Extract the JSON portion
	jsonPortion := response[firstBrace : lastBrace+1]
	return strings.TrimSpace(jsonPortion)
}

// extractPOIName extracts the full POI name from the message
func extractPOIName(message string) string {
	// Remove common words and keep the rest as the POI name
	words := strings.Fields(strings.ToLower(message))
	filtered := []string{}
	stopWords := map[string]bool{
		"add": true, "remove": true, "to": true, "from": true, "my": true,
		"itinerary": true, "with": true, "replace": true, "the": true, "in": true,
	}
	for _, w := range words {
		if !stopWords[w] {
			filtered = append(filtered, w)
		}
	}
	if len(filtered) == 0 {
		return "Unknown POI"
	}
	// Capitalize each word for proper formatting
	// cases.Title
	// use this https://pkg.go.dev/golang.org/x/text/cases later and handle language as well
	return strings.Title(strings.Join(filtered, " "))
}

// generateRealisticRating creates a realistic rating based on category and name patterns
func (l *ServiceImpl) generateRealisticRating(category, name string) float64 {
	baseRating := 4.0 // Default good rating

	// Adjust based on category
	switch strings.ToLower(category) {
	case "museum", "historical", "landmark", "cultural":
		baseRating = 4.3 // Museums tend to have higher ratings
	case "restaurant", "cafe", "bar":
		baseRating = 4.1 // Food places have variable ratings
	case "park", "nature", "garden":
		baseRating = 4.4 // Parks usually well-rated
	case "shopping", "store":
		baseRating = 3.9 // Shopping has more varied ratings
	case "attraction", "entertainment":
		baseRating = 4.2 // Attractions usually good
	}

	// Add some realistic variation (±0.5)

	variation, _ := rand.Int(rand.Reader, big.NewInt(100))
	adjustedVariation := (float64(variation.Int64()) - 50) / 100 // -0.5 to +0.5

	rating := baseRating + adjustedVariation

	// Ensure rating is within reasonable bounds
	if rating < 2.5 {
		rating = 2.5
	}
	if rating > 5.0 {
		rating = 5.0
	}

	// Round to 1 decimal place
	return float64(int(rating*10)) / 10
}

// generateRealisticPriceLevel creates appropriate price levels based on category
func (l *ServiceImpl) generateRealisticPriceLevel(category, description string) string {
	descLower := strings.ToLower(description)
	categoryLower := strings.ToLower(category)

	// Check for price indicators in description
	if strings.Contains(descLower, "free") || strings.Contains(descLower, "no cost") {
		return "Free"
	}

	if strings.Contains(descLower, "luxury") || strings.Contains(descLower, "premium") ||
		strings.Contains(descLower, "exclusive") || strings.Contains(descLower, "upscale") {
		return "€€€€"
	}

	if strings.Contains(descLower, "expensive") || strings.Contains(descLower, "fine dining") {
		return "€€€"
	}

	// Category-based defaults
	switch categoryLower {
	case "museum", "gallery", "exhibition":
		return "€€" // Museums usually have moderate entry fees
	case "park", "garden", "nature", "beach":
		return "Free" // Parks are usually free
	case "restaurant":
		if strings.Contains(descLower, "michelin") || strings.Contains(descLower, "fine") {
			return "€€€€"
		} else if strings.Contains(descLower, "casual") || strings.Contains(descLower, "local") {
			return "€€"
		}
		return "€€€" // Default for restaurants
	case "cafe", "bar":
		return "€€" // Cafes and bars are usually moderate
	case "historical", "landmark", "monument":
		return "€" // Historical sites often have low entry fees
	case "shopping", "market":
		return "€€" // Shopping varies, moderate default
	case "entertainment", "theater", "cinema":
		return "€€€" // Entertainment usually moderate to expensive
	case "hotel", "accommodation":
		return "€€€" // Hotels tend to be more expensive
	default:
		return "€€" // Default moderate pricing
	}
}

// generateRealisticOpeningHours creates realistic opening hours based on category
func (l *ServiceImpl) generateRealisticOpeningHours(category string) map[string]string {
	categoryLower := strings.ToLower(category)

	switch categoryLower {
	case "museum", "gallery", "exhibition":
		return map[string]string{
			"monday":    "10:00-18:00",
			"tuesday":   "10:00-18:00",
			"wednesday": "10:00-18:00",
			"thursday":  "10:00-18:00",
			"friday":    "10:00-18:00",
			"saturday":  "10:00-18:00",
			"sunday":    "10:00-17:00",
		}
	case "restaurant":
		return map[string]string{
			"monday":    "12:00-23:00",
			"tuesday":   "12:00-23:00",
			"wednesday": "12:00-23:00",
			"thursday":  "12:00-23:00",
			"friday":    "12:00-24:00",
			"saturday":  "12:00-24:00",
			"sunday":    "12:00-22:00",
		}
	case "cafe":
		return map[string]string{
			"monday":    "07:00-20:00",
			"tuesday":   "07:00-20:00",
			"wednesday": "07:00-20:00",
			"thursday":  "07:00-20:00",
			"friday":    "07:00-21:00",
			"saturday":  "08:00-21:00",
			"sunday":    "08:00-19:00",
		}
	case "bar":
		return map[string]string{
			"monday":    "18:00-02:00",
			"tuesday":   "18:00-02:00",
			"wednesday": "18:00-02:00",
			"thursday":  "18:00-02:00",
			"friday":    "18:00-03:00",
			"saturday":  "18:00-03:00",
			"sunday":    "18:00-01:00",
		}
	case "park", "garden", "nature":
		return map[string]string{
			"monday":    "06:00-22:00",
			"tuesday":   "06:00-22:00",
			"wednesday": "06:00-22:00",
			"thursday":  "06:00-22:00",
			"friday":    "06:00-22:00",
			"saturday":  "06:00-22:00",
			"sunday":    "06:00-22:00",
		}
	case "shopping", "store", "market":
		return map[string]string{
			"monday":    "10:00-19:00",
			"tuesday":   "10:00-19:00",
			"wednesday": "10:00-19:00",
			"thursday":  "10:00-19:00",
			"friday":    "10:00-20:00",
			"saturday":  "10:00-20:00",
			"sunday":    "12:00-18:00",
		}
	default:
		// Default business hours
		return map[string]string{
			"monday":    "09:00-18:00",
			"tuesday":   "09:00-18:00",
			"wednesday": "09:00-18:00",
			"thursday":  "09:00-18:00",
			"friday":    "09:00-18:00",
			"saturday":  "10:00-17:00",
			"sunday":    "10:00-16:00",
		}
	}
}

// generateAddressFromLocation creates a realistic address
func (l *ServiceImpl) generateAddressFromLocation(lat, lon float64, city string) string {
	if city == "" {
		city = "Porto" // Default city
	}

	// Generate a simple realistic address
	streetNumber := int(math.Abs(lat*1000))%999 + 1
	if streetNumber <= 0 {
		streetNumber = 1
	}

	streets := []string{"Rua da República", "Avenida dos Aliados", "Rua de Santa Catarina",
		"Rua do Almada", "Rua Formosa", "Rua das Flores", "Avenida da Boavista"}
	streetIndex := int(math.Abs(lon*1000)) % len(streets)

	// Extra safety check to ensure valid index
	if streetIndex < 0 || streetIndex >= len(streets) {
		streetIndex = 0
	}

	return fmt.Sprintf("%s, %d, %s, Portugal", streets[streetIndex], streetNumber, city)
}

// generateRealisticTags creates relevant tags based on category and description
func (l *ServiceImpl) generateRealisticTags(category, description string) []string {
	tags := []string{}
	categoryLower := strings.ToLower(category)
	descLower := strings.ToLower(description)

	// Add category-based tags
	switch categoryLower {
	case "museum", "gallery":
		tags = append(tags, "Culture", "Art", "History", "Educational")
	case "restaurant":
		tags = append(tags, "Dining", "Food", "Local Cuisine")
		if strings.Contains(descLower, "seafood") {
			tags = append(tags, "Seafood")
		}
		if strings.Contains(descLower, "traditional") {
			tags = append(tags, "Traditional")
		}
	case "cafe":
		tags = append(tags, "Coffee", "Casual", "Local")
	case "bar":
		tags = append(tags, "Nightlife", "Drinks", "Social")
	case "park", "garden":
		tags = append(tags, "Nature", "Outdoor", "Relaxing", "Family-friendly")
	case "historical", "landmark":
		tags = append(tags, "History", "Architecture", "Sightseeing", "Cultural")
	case "shopping", "market":
		tags = append(tags, "Shopping", "Local Products", "Souvenirs")
	case "entertainment":
		tags = append(tags, "Entertainment", "Fun", "Activities")
	}

	// Add description-based tags
	if strings.Contains(descLower, "family") {
		tags = append(tags, "Family-friendly")
	}
	if strings.Contains(descLower, "romantic") {
		tags = append(tags, "Romantic")
	}
	if strings.Contains(descLower, "outdoor") {
		tags = append(tags, "Outdoor")
	}
	if strings.Contains(descLower, "traditional") {
		tags = append(tags, "Traditional")
	}
	if strings.Contains(descLower, "modern") {
		tags = append(tags, "Modern")
	}
	if strings.Contains(descLower, "view") || strings.Contains(descLower, "panoramic") {
		tags = append(tags, "Great Views")
	}

	// Ensure we have at least 2-3 tags
	if len(tags) < 2 {
		tags = append(tags, "Popular", "Recommended")
	}

	return tags
}
