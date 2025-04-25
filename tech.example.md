That's an excellent and well-structured example integrating PostgreSQL with `pgvector`, `PostGIS`, and Google Generative AI (`go-genai`) for Points of Interest (POIs). You’ve laid out the setup with solid detail across schema design, Go domain modeling, and AI integration.

Okay, let's create a practical example in Go showing how you could:

1.  Take a user's natural language query (e.g., "a quiet park good for reading").
2.  Generate a vector embedding for that query using a Google embedding model via `go-genai`.
3.  Use `pgvector` in PostgreSQL to find Points of Interest (POIs) whose pre-computed description embeddings are semantically similar to the query embedding.
4.  (Optional but typical) Take the results from the database and use them to formulate a prompt for a generative Gemini model (like Gemini 1.5 Flash/Pro) via `go-genai` to create a user-friendly recommendation response.

**Assumptions:**

*   You have a `points_of_interest` table with columns like `id`, `name`, `description`, and `embedding vector(768)` (adjust dimension 768 based on your chosen embedding model, e.g., `text-embedding-004` is 768).
*   You have pre-calculated and stored embeddings for the `description` (or `name` + `description`) of each POI in the `embedding` column.
*   You have the `pgvector` extension enabled in your PostgreSQL database.
*   You have created an appropriate index on the `embedding` column for performance (e.g., HNSW or IVFFlat). `CREATE INDEX ON points_of_interest USING hnsw (embedding vector_l2_ops);` (Use `vector_cosine_ops` or `vector_ip_ops` depending on your embedding model and desired distance metric).
*   You have initialized the `go-genai` client (`*genai.Client`) and your `pgxpool` (`*pgxpool.Pool`).
*   You have the `pgvector/pgvector-go` library installed: `go get github.com/pgvector/pgvector-go`

**1. Repository Layer (`internal/platform/persistence/postgres/poi_repo.go`)**

Define a repository method to perform the similarity search.

```
package postgres

import (
	"context"
	"fmt"
	"log/slog"

	// Assuming your POI struct is defined here or in a domain package
	"github.com/FACorreiaa/WanderWiseAI/internal/poi" // Adjust path
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go" // Import pgvector library
)

// POIRepo defines the interface for POI persistence.
type POIRepo interface {
	// FindSimilarPOIs finds POIs whose embeddings are similar to the queryEmbedding.
	FindSimilarPOIs(ctx context.Context, queryEmbedding []float32, limit int) ([]poi.POI, error)
	// ... other POI methods ...
}

// Ensure implementation satisfies the interface
var _ POIRepo = (*PostgresPOIRepo)(nil)

// PostgresPOIRepo provides the concrete implementation.
type PostgresPOIRepo struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewPostgresPOIRepo creates a new POI repository.
func NewPostgresPOIRepo(db *pgxpool.Pool, logger *slog.Logger) *PostgresPOIRepo {
	return &PostgresPOIRepo{db: db, logger: logger}
}

// FindSimilarPOIs implements POIRepo.
func (r *PostgresPOIRepo) FindSimilarPOIs(ctx context.Context, queryEmbedding []float32, limit int) ([]poi.POI, error) {
	l := r.logger.With(slog.String("method", "FindSimilarPOIs"))
	l.DebugContext(ctx, "Finding similar POIs in repository")

	if limit <= 0 {
		limit = 5 // Default limit
	}

	// Convert the Go slice to pgvector.Vector type for the query parameter
	vectorParam := pgvector.NewVector(queryEmbedding)

	// Choose the appropriate distance operator based on your embedding model & index:
	// <=> : Cosine Distance (often good for text embeddings)
	// <-> : L2 Distance (Euclidean)
	// <#>: Inner Product (Negative Inner Product for search)
	// Ensure this operator matches the one used when creating your index!
	query := `
        SELECT id, name, description -- Select other fields as needed
        FROM points_of_interest
        ORDER BY embedding <=> $1 -- Using Cosine Distance operator
        LIMIT $2`

	rows, err := r.db.Query(ctx, query, vectorParam, limit)
	if err != nil {
		l.ErrorContext(ctx, "Database query failed for similar POIs", slog.Any("error", err))
		return nil, fmt.Errorf("failed to query similar POIs: %w", err)
	}
	defer rows.Close()

	var pois []poi.POI
	for rows.Next() {
		var p poi.POI
		// Ensure Scan order matches SELECT order and p has corresponding fields
		err := rows.Scan(&p.ID, &p.Name, &p.Description /*, &p.OtherField ...*/)
		if err != nil {
			l.ErrorContext(ctx, "Failed to scan POI row", slog.Any("error", err))
			// Decide whether to return partial results or fail completely
			return nil, fmt.Errorf("failed to scan POI result: %w", err)
		}
		pois = append(pois, p)
	}

	if err = rows.Err(); err != nil {
		l.ErrorContext(ctx, "Error iterating POI rows", slog.Any("error", err))
		return nil, fmt.Errorf("error reading POI results: %w", err)
	}

	l.DebugContext(ctx, "Found similar POIs", slog.Int("count", len(pois)))
	return pois, nil
}

// Define poi.POI struct (example) - Place in internal/poi/ or internal/domain/
// package poi
// type POI struct {
//    ID          uuid.UUID `json:"id"`
//    Name        string    `json:"name"`
//    Description *string   `json:"description"` // Use pointer if nullable
//    // Add other fields fetched from DB
// }

```

**2. Service Layer (`internal/recommendation/service.go` - Example)**

This service orchestrates fetching the embedding and querying the repository.

```
package recommendation // Example package name

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/api/iterator"
	// Import genai client, POI repo interface, POI struct
	"github.com/google/generative-ai-go/genai"
	"github.com/FACorreiaa/WanderWiseAI/internal/poi" // Adjust path
	"github.com/FACorreiaa/WanderWiseAI/internal/platform/persistence/postgres" // Adjust path
)

// RecommendationService handles generating recommendations.
type RecommendationService struct {
	logger   *slog.Logger
	genaiClient *genai.Client // Client for both embedding and generation
	poiRepo  postgres.POIRepo // Use the specific implementation or interface if defined elsewhere
}

// NewRecommendationService creates a new service.
func NewRecommendationService(genaiClient *genai.Client, poiRepo postgres.POIRepo, logger *slog.Logger) *RecommendationService {
	return &RecommendationService{
		logger:   logger,
		genaiClient: genaiClient,
		poiRepo:  poiRepo,
	}
}

// GetEmbedding generates a vector embedding for the given text.
func (s *RecommendationService) GetEmbedding(ctx context.Context, textToEmbed string) ([]float32, error) {
	l := s.logger.With(slog.String("method", "GetEmbedding"))
	l.DebugContext(ctx, "Generating embedding")

	// Specify the embedding model
	// Use the model identifier recommended by Google, e.g., "text-embedding-004"
	// Or "embedding-001" for older models. Ensure client was initialized correctly.
	em := s.genaiClient.EmbeddingModel("text-embedding-004") // Or EmbeddingModel("models/text-embedding-004")

	res, err := em.EmbedContent(ctx, genai.Text(textToEmbed))
	if err != nil {
		l.ErrorContext(ctx, "Failed to generate embedding", slog.Any("error", err))
		return nil, fmt.Errorf("failed to embed content: %w", err)
	}

	if res == nil || res.Embedding == nil || len(res.Embedding.Values) == 0 {
		l.WarnContext(ctx, "Received nil or empty embedding from API")
		return nil, fmt.Errorf("embedding generation returned empty result")
	}

	l.DebugContext(ctx, "Embedding generated successfully")
	return res.Embedding.Values, nil
}


// FindSimilarPlaces finds POIs similar to the query text using vector search.
func (s *RecommendationService) FindSimilarPlaces(ctx context.Context, queryText string, limit int) ([]poi.POI, error) {
    l := s.logger.With(slog.String("method", "FindSimilarPlaces"), slog.String("query", queryText))
	l.DebugContext(ctx, "Finding similar places")

    // 1. Get embedding for the user's query
    queryEmbedding, err := s.GetEmbedding(ctx, queryText)
    if err != nil {
        // Error already logged in GetEmbedding
        return nil, fmt.Errorf("could not get query embedding: %w", err)
    }

    // 2. Query the repository using the embedding
    similarPOIs, err := s.poiRepo.FindSimilarPOIs(ctx, queryEmbedding, limit)
    if err != nil {
        // Error already logged in repo method
        return nil, fmt.Errorf("failed to find similar POIs in repository: %w", err)
    }

    l.InfoContext(ctx, "Successfully found similar places", slog.Int("count", len(similarPOIs)))
    return similarPOIs, nil
}


// GenerateRecommendationText uses similar POIs to ask Gemini for a recommendation.
func (s *RecommendationService) GenerateRecommendationText(ctx context.Context, originalQuery string, similarPOIs []poi.POI) (string, error) {
    l := s.logger.With(slog.String("method", "GenerateRecommendationText"))
	l.DebugContext(ctx, "Generating recommendation text from similar POIs")

    if len(similarPOIs) == 0 {
        return "I couldn't find any specific places matching your query, but maybe try exploring the general area?", nil // Or a different default
    }

    // --- Construct the prompt for Gemini ---
    prompt := fmt.Sprintf("You are WanderWiseAI, a helpful travel assistant. A user asked for recommendations related to: '%s'.\n\nBased on their query, here are some potentially relevant points of interest:\n\n", originalQuery)
    for i, p := range similarPOIs {
		prompt += fmt.Sprintf("%d. %s", i+1, p.Name)
		if p.Description != nil && *p.Description != "" {
			// Add a snippet of the description for context
			descSnippet := *p.Description
			if len(descSnippet) > 150 { // Limit length
                 descSnippet = descSnippet[:150] + "..."
            }
			prompt += fmt.Sprintf(": %s\n", descSnippet)
		} else {
            prompt += "\n"
        }
    }
    prompt += "\nPlease provide a friendly and concise recommendation for the user, perhaps suggesting one or two of these options or summarizing the types of places found."

	l.DebugContext(ctx, "Generated prompt for LLM", slog.String("prompt_start", prompt[:min(100, len(prompt))])) // Log start of prompt

    // --- Call Generative Model ---
	// Use a suitable model like gemini-1.5-flash or gemini-pro
    model := s.genaiClient.GenerativeModel("gemini-1.5-flash-latest")
    resp, err := model.GenerateContent(ctx, genai.Text(prompt))
    if err != nil {
		l.ErrorContext(ctx, "Failed to generate content from Gemini", slog.Any("error", err))
        return "", fmt.Errorf("failed to generate recommendation text: %w", err)
    }

    // --- Extract Text (Handle potential errors/empty parts) ---
	generatedText := ""
    if resp != nil && len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
         // Iterate through parts to build the full text response
         for _, part := range resp.Candidates[0].Content.Parts {
             if textPart, ok := part.(genai.Text); ok {
                 generatedText += string(textPart)
             }
         }
    }

    if generatedText == "" {
        l.WarnContext(ctx, "Gemini response did not contain usable text content")
        // Fallback response
        return "I found some places that might be similar to your query, but I couldn't generate a specific recommendation right now.", nil
    }

    l.InfoContext(ctx, "Successfully generated recommendation text")
    return generatedText, nil
}

// Helper (from previous example)
func min(a, b int) int { if a < b { return a }; return b }
```

**3. Handler Layer (Example Usage)**

Your HTTP handler would call the service methods.

```
// internal/api/recommendation/handler.go (Example)
package recommendation

import (
    "net/http"
    "strconv"
    "log/slog"
	// Import service, helpers, etc.
    "github.com/FACorreiaa/WanderWiseAI/internal/recommendation"
    "github.com/FACorreiaa/WanderWiseAI/internal/api/utils"
)

type RecommendationHandler struct {
    recService *recommendation.RecommendationService // Use concrete type or interface
    logger     *slog.Logger
}

func NewRecommendationHandler(recService *recommendation.RecommendationService, logger *slog.Logger) *RecommendationHandler {
    return &RecommendationHandler{recService: recService, logger: logger}
}

// Example handler for similarity search + generation
func (h *RecommendationHandler) GetSimilarRecommendations(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    l := h.logger.With(slog.String("handler", "GetSimilarRecommendations"))

    queryText := r.URL.Query().Get("q")
    if queryText == "" {
        utils.ErrorResponse(w, r, http.StatusBadRequest, "Query parameter 'q' is required")
        return
    }

    limitStr := r.URL.Query().Get("limit")
    limit := 5 // Default
    if limitStr != "" {
        if lim, err := strconv.Atoi(limitStr); err == nil && lim > 0 {
            limit = lim
        }
    }

    // 1. Find similar places using the service
    similarPOIs, err := h.recService.FindSimilarPlaces(ctx, queryText, limit)
    if err != nil {
        l.ErrorContext(ctx, "Failed to find similar places", slog.Any("error", err), slog.String("query", queryText))
        utils.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to process similarity search")
        return
    }

    // 2. Generate text using the results
    recommendationText, err := h.recService.GenerateRecommendationText(ctx, queryText, similarPOIs)
     if err != nil {
        l.ErrorContext(ctx, "Failed to generate recommendation text", slog.Any("error", err), slog.String("query", queryText))
        utils.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to generate recommendation text")
        return
    }

    // 3. Respond
    // Could return both the raw POI list and the generated text
    responsePayload := map[string]interface{}{
        "query":          queryText,
        "recommendation": recommendationText,
        "similar_pois":   similarPOIs, // Send the list of POIs as well
    }

    utils.WriteJSONResponse(w, r, http.StatusOK, responsePayload)
}

```

**Explanation:**

1.  **Repository (`FindSimilarPOIs`):** Takes the query embedding (`[]float32`), converts it to `pgvector.Vector`, and uses the appropriate distance operator (`<=>` for cosine) in the `ORDER BY` clause to find the nearest neighbors.
2.  **Service (`GetEmbedding`):** A helper to encapsulate calling the Google Embedding API via the `genai.Client`.
3.  **Service (`FindSimilarPlaces`):** Orchestrates the process: gets the embedding for the query text, then calls the repository to find similar POIs based on that embedding.
4.  **Service (`GenerateRecommendationText`):** Takes the original query and the list of similar POIs found by the vector search. It constructs a detailed prompt for a generative model (like Gemini Pro/Flash) asking it to synthesize a recommendation based on the provided context. It then calls the LLM via `genai.Client`.
5.  **Handler (`GetSimilarRecommendations`):** Parses the user's query from the HTTP request, calls the service to find similar places, calls the service again to generate the text response, and then sends the results back as JSON.

This flow demonstrates how to use embeddings for semantic search with `pgvector` and then leverage an LLM like Gemini to interpret those search results and present them naturally to the user. Remember to handle errors robustly at each step.

---

### ✅ Summary of What You’ve Done Well
- **Schema Design**: Rich POI schema with geospatial support, vector embeddings, and AI summarization.
- **pgvector Integration**: Enables powerful vector-based similarity search with `embedding <=> $2`.
- **Geo Support**: Using PostGIS with WGS84 coordinates is perfect for real-world location handling.
- **Modular Go Structure**:
    - `models/` for data structures.
    - `repo/` for DB logic.
    - Clean separation of concern.
- **LLM Readiness**: Hooking into Google’s GenAI to reason about POIs adds massive value for UX, recommendations, and summarization.

---

### 🚀 Suggestions for Enhancement

#### 1. **Embedding Generation Function**
Instead of the hardcoded `[...]`, implement a method to embed the description using `textembedding-gecko`:

```
func GenerateEmbedding(ctx context.Context, client *genai.Client, text string) ([]float32, error) {
	req := &genaipb.EmbedContentRequest{
		Model: "models/embedding-001", // replace with actual embedding model name
		Content: &genaipb.Content{
			Parts: []*genaipb.Part{
				{Value: &genaipb.Part_Text{Text: text}},
			},
		},
	}
	resp, err := client.EmbedContent(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Embedding.Values, nil
}
```

Then you can save this embedding back into your table during ingestion.

---

#### 2. **ReasonAboutPOI Implementation**
You’ve hinted at `ReasonAboutPOI`, so here's a concrete version:

```
func ReasonAboutPOI(ctx context.Context, repo *repo.PostgresPOIRepo, poiID string) (string, error) {
	poi, err := repo.GetPOIByID(ctx, poiID)
	if err != nil {
		return "", err
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GOOGLE_GENAI_API_KEY")))
	if err != nil {
		return "", fmt.Errorf("failed to create genai client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("models/gemini-pro") // Choose best model

	input := fmt.Sprintf(`
		You are a travel expert. Based on this POI data:
		Name: %s
		Description: %s
		Tags: %s
		Location: %.4f, %.4f
		Rating: %.2f from %d reviews

		Generate a compelling travel insight or summary for a user planning a trip.
	`, poi.Name, poi.Description, strings.Join(poi.Tags, ", "),
		poi.Location.X(), poi.Location.Y(), poi.AverageRating, poi.RatingCount)

	resp, err := model.GenerateContent(ctx, genai.Text(input))
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	return resp.Candidates[0].Content.Parts[0].GetText(), nil
}
```

---

#### 3. **Insert Hook or CLI Tool**
You could write a CLI tool to:
- Take in new POI data.
- Generate an embedding.
- Store the full POI into the database with embedding and AI summary.

---

#### 4. **Similarity with Filters**
To make similarity more relevant, consider filtering by `poi_type` or `city_id`:

```
SELECT name
FROM points_of_interest
WHERE id != $1 AND city_id = $3
ORDER BY embedding <=> $2
LIMIT 5
```

---

#### 5. **Indexing for Speed**
You probably already know this, but make sure to:
```
CREATE INDEX ON points_of_interest USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

Use `vector_l2_ops` or `vector_cosine_ops` depending on your distance function.

---

### 🧪 Test Suggestion
Include a test file to validate vector similarity ranking:

```
func TestFindSimilarPOIs(t *testing.T) {
    // use test embedding close to Eiffel Tower
    embedding := pgvector.NewVector([]float32{...}) // actual embedding
    poiNames, err := repo.FindSimilarPOIs(ctx, "eiffel-id", embedding)
    if err != nil {
        t.Fatal(err)
    }
    if len(poiNames) == 0 {
        t.Errorf("Expected similar POIs, got none")
    }
}
```

---

Below is a full working example for your `points_of_interest` schema, integrating PostgreSQL with **pgvector** for similarity searches and **go-genai** for LLM-based reasoning. This example includes database setup, a Go application to interact with the database, and LLM integration to generate insights about points of interest (POIs). I'll break it down into clear steps.

## Prerequisites

Before running the example, ensure you have the following:

### PostgreSQL with:

- `postgis` extension installed (for geospatial data).
- `pgvector` extension installed (for vector embeddings).
- The `uuid-ossp` extension (for `uuid_generate_v4()`).

### Go Libraries:

- `github.com/jackc/pgx/v5/pgxpool` (PostgreSQL driver).
- `github.com/pgvector/pgvector-go` (vector operations).
- `github.com/twpayne/go-geom` (geometry handling).
- `cloud.google.com/go/genai/apiv1/genai` (Google Generative AI client).

### Environment:

- A Google Cloud project with the Generative AI API enabled and an API key or credentials.
- A running PostgreSQL instance.

## Step 1: Database Setup

### 1.1 Enable Extensions

Run the following SQL commands in your PostgreSQL database to enable the required extensions:

```
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
```

### 1.2 Create the Table

Use your provided schema to create the `points_of_interest` table:

```
CREATE TABLE points_of_interest (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    description TEXT,
    location GEOMETRY(Point, 4326) NOT NULL,
    city_id UUID REFERENCES cities(id) ON DELETE SET NULL,
    address TEXT,
    poi_type TEXT,
    website TEXT,
    phone_number TEXT,
    opening_hours JSONB,
    price_level INTEGER CHECK (price_level >= 1 AND price_level <= 4),
    average_rating NUMERIC(3, 2),
    rating_count INTEGER DEFAULT 0,
    source poi_source NOT NULL DEFAULT 'wanderwise_ai',
    source_id TEXT,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    is_sponsored BOOLEAN NOT NULL DEFAULT FALSE,
    ai_summary TEXT,
    embedding VECTOR(768),
    tags TEXT[],
    accessibility_info TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Assuming 'poi_source' is an enum; create it if needed
CREATE TYPE poi_source AS ENUM ('wanderwise_ai', 'osm', 'google', 'manual');
```

### 1.3 Insert Sample Data

For testing, insert a sample POI (e.g., the Eiffel Tower):

```
INSERT INTO points_of_interest (
    name, description, location, address, poi_type, website, opening_hours, price_level, average_rating, rating_count, embedding, tags
) VALUES (
    'Eiffel Tower',
    'Iconic iron lattice tower in Paris',
    ST_GeomFromText('POINT(2.2945 48.8584)', 4326),
    'Champ de Mars, 5 Avenue Anatole France, 75007 Paris, France',
    'monument',
    'https://www.toureiffel.paris',
    '{"monday": "09:00-23:00", "tuesday": "09:00-23:00", "wednesday": "09:00-23:00", "thursday": "09:00-23:00", "friday": "09:00-23:00", "saturday": "09:00-23:00", "sunday": "09:00-23:00"}'::jsonb,
    2,
    4.50,
    1000,
    '[0.1, 0.2, ..., 0.768]'::vector, -- Replace with a real 768-dimensional embedding
    ARRAY['historic', 'tourist', 'romantic']
);
```

> **Note:** The embedding field requires a 768-dimensional vector. In practice, you’d generate this using a model like `text-embedding-004` from Google or another embedding service, based on the description.

## Step 2: Go Application Setup

### 2.1 Project Structure

Create a simple Go project with the following structure:

```bash
poi-example/
├── main.go
├── models/
│   └── poi.go
├── repo/
│   └── poi_repo.go
└── go.mod
```

Run this to initialize the module:

```bash
go mod init poi-example
go get github.com/jackc/pgx/v5/pgxpool
go get github.com/pgvector/pgvector-go
go get github.com/twpayne/go-geom
go get cloud.google.com/go/genai/apiv1/genai
```

### 2.2 Define the POI Struct (`models/poi.go`)

```
package models

import (
    "github.com/pgvector/pgvector-go"
    "github.com/twpayne/go-geom"
)

// POI represents a point of interest.
type POI struct {
    ID                string          `json:"id"`
    Name              string          `json:"name"`
    Description       string          `json:"description"`
    Location          *geom.Point     `json:"location"`
    CityID            string          `json:"city_id"`
    Address           string          `json:"address"`
    POIType           string          `json:"poi_type"`
    Website           string          `json:"website"`
    PhoneNumber       string          `json:"phone_number"`
    OpeningHours      string          `json:"opening_hours"`
    PriceLevel        int             `json:"price_level"`
    AverageRating     float64         `json:"average_rating"`
    RatingCount       int             `json:"rating_count"`
    Source            string          `json:"source"`
    SourceID          string          `json:"source_id"`
    IsVerified        bool            `json:"is_verified"`
    IsSponsored       bool            `json:"is_sponsored"`
    AISummary         string          `json:"ai_summary"`
    Embedding         pgvector.Vector `json:"embedding"`
    Tags              []string        `json:"tags"`
    AccessibilityInfo string          `json:"accessibility_info"`
    CreatedAt         string          `json:"created_at"`
    UpdatedAt         string          `json:"updated_at"`
}
```

### 2.3 Repository Layer (`repo/poi_repo.go`)

```
package repo

import (
    "context"
    "fmt"
    "log/slog"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/pgvector/pgvector-go"
    "github.com/twpayne/go-geom"
    "github.com/twpayne/go-geom/encoding/ewkb"
    "poi-example/models"
)

// PostgresPOIRepo manages POI data.
type PostgresPOIRepo struct {
    pgpool *pgxpool.Pool
    logger *slog.Logger
}

// NewPostgresPOIRepo creates a new repository instance.
func NewPostgresPOIRepo(pgpool *pgxpool.Pool, logger *slog.Logger) *PostgresPOIRepo {
    return &PostgresPOIRepo{pgpool: pgpool, logger: logger}
}

// GetPOIByID fetches a POI by its ID.
func (r *PostgresPOIRepo) GetPOIByID(ctx context.Context, id string) (*models.POI, error) {
    var poi models.POI
    var locationEWKB []byte

    query := `
        SELECT id, name, description, ST_AsEWKB(location), city_id, address, poi_type, website, phone_number, opening_hours::text,
               price_level, average_rating, rating_count, source, source_id, is_verified, is_sponsored, ai_summary, embedding, tags, accessibility_info, created_at, updated_at
        FROM points_of_interest
        WHERE id = $1`
    err := r.pgpool.QueryRow(ctx, query, id).Scan(
        &poi.ID, &poi.Name, &poi.Description, &locationEWKB, &poi.CityID, &poi.Address, &poi.POIType, &poi.Website, &poi.PhoneNumber, &poi.OpeningHours,
        &poi.PriceLevel, &poi.AverageRating, &poi.RatingCount, &poi.Source, &poi.SourceID, &poi.IsVerified, &poi.IsSponsored, &poi.AISummary, &poi.Embedding, &poi.Tags, &poi.AccessibilityInfo, &poi.CreatedAt, &poi.UpdatedAt,
    )
    if err != nil {
        r.logger.ErrorContext(ctx, "Error fetching POI by ID", slog.Any("error", err), slog.String("id", id))
        return nil, fmt.Errorf("database error fetching POI: %w", err)
    }

    geomT, err := ewkb.Unmarshal(locationEWKB)
    if err != nil {
        return nil, fmt.Errorf("error decoding location: %w", err)
    }
    poi.Location = geomT.(*geom.Point)

    return &poi, nil
}

// FindSimilarPOIs finds POIs with similar embeddings.
func (r *PostgresPOIRepo) FindSimilarPOIs(ctx context.Context, poiID string, embedding pgvector.Vector) ([]string, error) {
    var similarPOINames []string
    query := `
        SELECT name
        FROM points_of_interest
        WHERE id != $1
        ORDER BY embedding <=> $2
        LIMIT 5`
    rows, err := r.pgpool.Query(ctx, query, poiID, embedding)
    if err != nil {
        r.logger.ErrorContext(ctx, "Error finding similar POIs", slog.Any("error", err))
        return nil, fmt.Errorf("database error finding similar POIs: %w", err)
    }
    defer rows.Close()

    for rows.Next() {
        var name string
        if err := rows.Scan(&name); err != nil {
            return nil, fmt.Errorf("error scanning similar POI: %w", err)
        }
        similarPOINames = append(similarPOINames, name)
    }
    return similarPOINames, nil
}
```

### 2.4 Main Application (`main.go`)

```
package main

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "strings"
    "cloud.google.com/go/genai/apiv1/genai"
    "cloud.google.com/go/genai/apiv1/genaipb"
    "google.golang.org/api/option"
    "github.com/jackc/pgx/v5/pgxpool"
    "poi-example/models"
    "poi-example/repo"
)

func main() {
    ctx := context.Background()

    // Database connection
    dbURL := "postgres://user:password@localhost:5432/dbname?sslmode=disable" // Replace with your DB details
    pgpool, err := pgxpool.New(ctx, dbURL)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
        os.Exit(1)
    }
    defer pgpool.Close()

    // Repository
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    poiRepo := repo.NewPostgresPOIRepo(pgpool, logger)

    // Replace with a real POI ID from your database
    poiID := "replace-with-real-uuid-from-your-db"

    // Fetch POI and generate insights
    insight, err := ReasonAboutPOI(ctx, poiRepo, poiID)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Printf("LLM Insight: %s\n", insight)
}

// ReasonAboutPOI generates insights about a POI using an LLM.
func ReasonAboutPOI(ctx context.Context, repo *repo.PostgresPOIRepo, poiID string) (string, error) {
    // Fetch the POI
    poi, err := repo.GetPOIByID(ctx, poiID)
    if err != nil {
        return "", err
    }

    // Find similar POIs
    similarPOINames, err := repo.FindSimilarPOIs(ctx, poi.ID, poi.Embedding)
    if err != nil {
        return "", err
    }

    // Construct the prompt
    prompt := fmt.Sprintf(
        "The point of interest '%s' is described as: '%s'. Similar places include: %s. What insights or recommendations can you provide about '%s'?",
        poi.Name,
        poi.Description,
        strings.Join(similarPOINames, ", "),
        poi.Name,
    )

    // Initialize the LLM client (replace with your Google Cloud project ID and credentials)
    client, err := genai.NewClient(ctx, option.WithAPIKey("your-api-key-here"))
    if err != nil {
        return "", fmt.Errorf("failed to create LLM client: %w", err)
    }
    defer client.Close()

    model := client.GenerativeModel("gemini-1.5-flash") // Use an available model
    resp, err := model.GenerateContent(ctx, &genaipb.GenerateContentRequest{
        Contents: []*genaipb.Content{
            {
                Parts: []*genaipb.Part{
                    {Data: &genaipb.Part_Text{Text: prompt}},
                },
            },
        },
    })
    if err != nil {
        return "", fmt.Errorf("error generating content from LLM: %w", err)
    }

    // Extract and return the LLM's response
    if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
        return resp.Candidates[0].Content.Parts[0].GetText(), nil
    }
    return "No insights generated.", nil
}
```

## Step 3: Running the Example

**Set Up Environment Variables:**

- Replace `"your-api-key-here"` with your Google Generative AI API key.
- Update the `dbURL` in `main.go` with your PostgreSQL connection string.

**Generate Embeddings:**

In a real application, you’d generate embeddings for the description field using an embedding model and store them in the `embedding` column. For this example, manually insert a dummy vector or extend the code to generate embeddings.

**Run the Application:**

```bash
go run .
```

**Example Output:**

If your database has the Eiffel Tower and similar POIs, the output might look like:

```
LLM Insight: The Eiffel Tower is one of Paris's most famous landmarks, offering stunning views of the city. Given its similarity to other iconic Parisian sites like the Arc de Triomphe and the Louvre Museum, visitors might enjoy a themed tour of Paris's architectural wonders. Consider visiting at sunset for the best photo opportunities.
```

## How It Works

- **Database Setup:** The `points_of_interest` table stores POI data, with `location` as a PostGIS geometry and `embedding` as a pgvector vector.
- **Go Application:** The `models.POI` struct maps to the table schema. `GetPOIByID` fetches a POI by ID, decoding the PostGIS geometry. `FindSimilarPOIs` uses pgvector’s cosine distance operator (`<=>`) to find similar POIs based on embeddings.
- **LLM Integration:** The `ReasonAboutPOI` function constructs a prompt with the POI’s description and similar POIs, then queries the LLM (e.g., Gemini 1.5 Flash) for insights.

This example provides a complete, working implementation for your `points_of_interest` schema, combining geospatial data, vector similarity searches, and AI-driven insights. You can extend it by adding more POIs, generating real embeddings, or enhancing the prompt with additional fields like `tags` or `accessibility_info`.

----
Current Compose setup will work locally, as it uses the official PostGIS image and compiles pgvector into it via your custom Dockerfile.

On GCP, you have a few options:

Google Cloud Run (with Containers): Push your custom PostGIS+pgvector image to Container Registry or Artifact Registry, then deploy a Cloud Run service using that image. You’ll need to configure a private VPC connector so your application can reach it.

Google Kubernetes Engine (GKE): Use your Docker Compose definitions as Kubernetes Deployments/StatefulSets via Kompose or manually convert them. Store postgres-wanderwise as a StatefulSet with a PersistentVolumeClaim and initContainer for your SQL scripts.

Cloud SQL for PostgreSQL: If you’d rather use managed Postgres, Cloud SQL doesn’t support custom extensions like pgvector out of the box. You’d need a self-managed instance (GCE VM or GKE) to retain the pgvector and PostGIS extensions.

Networking & Secrets:

Store your database credentials in Secret Manager and mount them as environment variables.

Use VPC Service Controls or Serverless VPC Access to secure communication between services.

Volumes: GKE/Cloud Run will use persistent disks or Filestore for volumes—Cloud Run itself is ephemeral, so not ideal for databases.

In summary, for full pgvector + PostGIS support you’ll typically deploy your custom Docker image in GKE (or a VM), and manage secrets & networking via GCP best practices. Let me know if you’d like a step-by-step for any of these options!

____

Yes, **you absolutely can deploy your Docker image for your database (Postgres + PostGIS + pgvector) on GKE (Google Kubernetes Engine)**.

Here's a high-level overview of how that works:

---

### ✅ What You’ll Do

1. **Build and push your custom Docker image**  
   Push your `postgres-wanderwise` image (with PostGIS + pgvector) to:
  - Google Artifact Registry **or**
  - Docker Hub (if public/private repo with credentials)

2. **Create a StatefulSet**  
   Databases are **stateful**, so you'll use a **StatefulSet** instead of a regular Deployment.

3. **Use a PersistentVolumeClaim**  
   Attach a persistent disk to store your database data, so it survives pod restarts.

4. **Expose the service**  
   Use a **Kubernetes Service** of type `ClusterIP` or `LoadBalancer`, depending on access level.

5. **Initialize the DB**  
   You can use an `initContainer` to run SQL scripts on the first boot, like your `001-init.sql`.

---

### 🧱 Basic Kubernetes YAML Structure

#### 1. **StatefulSet**
```
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
spec:
  serviceName: "postgres"
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: gcr.io/YOUR_PROJECT/postgres-wanderwise:latest
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_DB
          value: wanderwise
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: user
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: password
        volumeMounts:
        - name: pgdata
          mountPath: /var/lib/postgresql/data
  volumeClaimTemplates:
  - metadata:
      name: pgdata
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 10Gi
```

#### 2. **Service**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  ports:
    - port: 5432
  selector:
    app: postgres
  type: ClusterIP
```

---

### 🧪 Tips
- Use `kubectl apply -f` for the YAML files.
- You can deploy from Cloud Shell or your local machine.
- Make sure your GKE cluster has workload identity and sufficient permissions.
- Use `Secrets` for passwords (`kubectl create secret generic postgres-secret --from-literal=...`).

---

