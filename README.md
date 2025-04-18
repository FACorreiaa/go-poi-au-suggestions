# **WanderWiseAI** – Personalized City Discovery 🗺️✨

A smart, mobile-first web application providing personalized recommendations for city exploration based on user interests, time constraints, location, and an evolving AI engine.

---

## 📑 Table of Contents

- [🚀 Elevator Pitch](#-elevator-pitch)  
- [🌟 Core Features](#-core-features)  
- [💰 Business Model & Monetization](#-business-model--monetization)  
- [🛠 Technology Stack](#-technology-stack)  
- [🧪 Getting Started](#-getting-started)  
- [🗺️ Roadmap Highlights](#-roadmap-highlights)  
- [🤝 Contributing](#-contributing)  
- [📄 License](#-license)  

---

## 🚀 Elevator Pitch

Tired of generic city guides? **WanderWise** learns what you love—be it history, food, art, nightlife, or hidden gems—and combines it with your available time and location to suggest the perfect spots, activities, and restaurants.

Whether you're a tourist on a tight schedule or a local looking for something new, discover your city like never before with hyper-personalized, intelligent recommendations.

---

## 🌟 Core Features

- **🧠 AI-Powered Personalization**  
  Recommendations adapt based on explicit user preferences and learned behavior over time.

- **🔍 Contextual Filtering**  
  Filters results by:
  - Distance / Location
  - Available Time (e.g., “things to do in the next 2 hours”)
  - Opening Hours
  - User Interests (e.g., "art", "foodie", "outdoors", "history")
  - Budget (coming soon)

- **🗺 Interactive Map Integration**  
  Visualize recommendations, your location, and potential routes.

- **📌 Save & Organize**  
  Bookmark favorites, create custom lists or simple itineraries (enhanced in Premium).

- **📱 Mobile-First Design**  
  Optimized for on-the-go browsing via web browser.

---

## 💰 Business Model & Monetization

### Freemium Model

- **Free Tier**:
  - Access to core recommendation engine
  - Basic preference filters
  - Limited saves/lists
  - Non-intrusive contextual ads

- **Premium Tier (Monthly/Annual Subscription)**:
  - Enhanced AI recommendations
  - Advanced filters (cuisine, accessibility, niche tags, specific hours)
  - Unlimited saves & lists
  - Offline access
  - Exclusive curated content & themed tours
  - Ad-free experience

### Partnerships & Commissions

- **Booking Referrals**  
  Earn commission via integrations with platforms like GetYourGuide, Booking.com, OpenTable, etc.

- **Featured Listings (Transparent)**  
  Local businesses can pay for premium visibility in relevant results.

- **Exclusive Deals**  
  Offer users special discounts via business partnerships (potentially Premium-only).

### Future Monetization Options

- One-time in-app purchases (premium guides, city packs)
- Aggregated anonymized trend data (for tourism boards, researchers)

---

## 🛠 Technology Stack

### Backend
- **Language:** Go (Golang)
- **Router/Framework:** Chi or Gin Gonic
- **Database:** PostgreSQL + PostGIS (for geospatial queries)
- **ORM/DB Tooling:** `pgx`, `database/sql`, or `sqlc`

### Frontend
- **Framework:** SvelteKit *or* Next.js (React)
- **Styling:** Tailwind CSS
- **Maps:** Leaflet or Mapbox GL JS

### AI / Recommendation Engine
- **Initial:** Built-in Go logic (content-based filtering, etc.)
- **Future:** Dedicated Python microservice using FastAPI + Scikit-learn/TensorFlow/PyTorch

### Infrastructure
- **Containers:** Docker, Docker Compose
- **Cloud:** AWS / GCP / Azure (Managed Postgres, Kubernetes, Fargate, or Cloud Run)
- **CI/CD:** GitHub Actions or GitLab CI

---

## 🧪 Getting Started

> 🔧 _Instructions for local setup coming soon._

This will include:
- Cloning the repo
- Setting environment variables
- Running backend/frontend services
- Connecting to the database

---

## 🗺️ Roadmap Highlights

### Phase 1 (MVP)
- Core recommendation engine
- User accounts
- Map view
- Launch in one pilot city

### Phase 2
- Premium subscription tier
- Enhanced AI
- User reviews & ratings
- Booking/restaurant platform partnerships

### Phase 3
- Expansion to more cities
- Curated content
- Native app exploration (iOS/Android)

---

### 🔤 Name Suggestions

| Name | Meaning / Vibe |
|------|----------------|
| **WanderWise** | Smart way to explore cities (my top pick) |
| **CityMuse** | Your personal city inspiration |
| **Roamly** | Friendly, mobile, casual name (like travel "calmly") |
| **Spotlight** | Puts the spotlight on things you'll love |
| **TerraCurio** | “Curious Earth” vibe – for explorers |
| **Loci** | Latin for “places”; short and sharp |
| **Driftly** | Evokes free-flowing, casual exploration |
| **UrbanNest** | Cozy feeling of finding “your spot” in a city |
| **ScenIQ** | Scene + IQ — smart discovery of local scenes |
| **ViaNova** | Latin-inspired “new way” |

## 🛠 Technology Stack & Design Choices

This project aims for a high-performance, personalized user experience, integrating AI and social features, with considerations for future mobile expansion. The technology stack reflects these goals:

### Core Stack

*   **Backend Language:** **Go (Golang)**
    *   *Why:* Chosen for its excellent performance, concurrency handling, static typing, and suitability for building robust APIs (both HTTP and gRPC).

*   **Backend Framework/Router:** **Chi** or **Gin Gonic**
    *   *Why:* Lightweight, high-performance HTTP routers/micro-frameworks for Go, well-suited for building the primary API layer. (See API Layer section below).

*   **Database:** **PostgreSQL** with **PostGIS** extension
    *   *Why:* Powerful relational database combined with robust geospatial capabilities (PostGIS) essential for location-based queries (finding nearby points of interest, calculating distances).

*   **Database Interaction (Go):** **`pgx`** (recommended) or `sqlc`
    *   *Why:* `pgx` offers high performance and better type handling compared to the standard `database/sql` for PostgreSQL. `sqlc` can generate type-safe Go code from SQL queries, reducing boilerplate.

*   **Frontend Framework:** **SvelteKit** or **Next.js (React)**
    *   *Why:* Both are modern, powerful frameworks offering Server-Side Rendering (SSR) or Static Site Generation (SSG) capabilities. **SSR is crucial for SEO** and can improve perceived performance (faster first contentful paint). The choice depends on team preference (Svelte vs. React). Using **Tanstack Query (React Query) / Svelte Query** is recommended for efficient data fetching, caching, and state management on the frontend.

*   **Maps (Frontend):** **Mapbox GL JS**, **MapLibre GL JS**, or **Leaflet** (potentially **CesiumJS** for advanced 3D)
    *   *Why:* Provide interactive map experiences. Mapbox/MapLibre offer excellent performance and customization (including 3D potential). Leaflet is simpler for basic 2D maps. This is primarily a frontend implementation detail, consuming location data from the backend API.

*   **AI Engine Integration:** **Direct Google Gemini API via `google/generative-ai-go`**
    *   *Why:* To leverage the latest models (like Gemini 1.5 Pro with its large context window) directly for maximum control and capability. This allows feeding rich contextual information (user profile, time, location, *data fetched from PostGIS about nearby POIs*) into the prompt for deeply personalized recommendations. Using the official Go SDK avoids reliance on third-party gateways (like MCP), potential model availability lag, and potentially extra costs, while giving full control over prompt engineering.

*   **Authentication:** **Standard JWT + `Goth` package**
    *   *Why:* JWT (JSON Web Tokens) for managing user sessions via the API. Goth provides a straightforward way to integrate multiple social media logins (Google, Facebook, etc.) on the backend.

### API Layer: HTTP vs. gRPC

*   **Decision:** Start with a primary **HTTP/REST API** (using Chi or Gin).
*   **Rationale:**
    *   **Frontend Simplicity:** Standard HTTP/REST APIs are significantly easier to consume directly from web frameworks (SvelteKit, Next.js/React) using the native Fetch API or libraries like Axios/Tanstack Query, simplifying frontend development and debugging.
    *   **Performance:** While gRPC offers potential performance benefits (binary protocol, multiplexing), a well-designed Go HTTP API with efficient database queries, appropriate caching, and potentially HTTP/2 support is typically **performant enough** for this type of application's initial needs. Performance bottlenecks are often in database access or complex business logic, not necessarily the HTTP transport itself.
    *   **Ecosystem & Tooling:** The ecosystem for HTTP APIs, including testing tools (like Postman/Insomnia), browser debugging, and standard libraries, is more mature and widely understood for web development.
    *   **Third-Party Integrations:** Social sharing and other external services often rely on standard HTTP callbacks or webhooks.
    *   **gRPC Complexity:** Implementing gRPC for direct browser communication requires gRPC-Web proxies (like Envoy or the built-in Go proxy) or specific frontend libraries, adding setup and operational complexity.
*   **Future Consideration:** gRPC *could* be introduced later for specific backend-to-backend communication between microservices if the architecture evolves that way, or if profiling identifies the HTTP API layer itself as a critical performance bottleneck for specific high-throughput operations. However, starting with HTTP simplifies the initial development and frontend integration significantly.

### Social Features

*   **Sharing:** The backend API will provide endpoints to fetch POI details. The frontend will implement sharing functionality using standard web share APIs or direct links formatted for WhatsApp, Discord, etc.
*   **Login:** Handled by the Go backend using the `Goth` library integrating with the chosen frontend framework's authentication flow.

### Mobile / Cross-Platform

*   **Strategy:** Build the web application (PWA potentially) first using SvelteKit or Next.js.
*   **Future:** If native mobile apps are required:
    *   If using **Next.js/React**, **React Native** (potentially with shared components/logic via React Native Web) offers a path to target iOS, Android, and Web with significant code reuse, all consuming the same backend API.
    *   If using **SvelteKit**, options include Capacitor/Ionic for wrapping the web app or building native apps separately (Swift/Kotlin) consuming the API.

### Summary

This stack prioritizes **performance** (Go, PostGIS, efficient API), **personalization** (direct Gemini 1.5 integration via `go-genai`), **SEO** (SSR via SvelteKit/Next.js), and **developer experience**. Starting with an HTTP API simplifies frontend integration while retaining the option to introduce gRPC later if needed. The chosen components provide a solid foundation for current features and future expansion into mobile and enhanced social integration.

---

# Integrating Google Gemini (`go-genai`) with PostgreSQL

This document outlines how the application leverages the `google/generative-ai-go` SDK (`go-genai`) to interact with Google's Gemini models and how data related to these interactions, as well as application data queried via Gemini, is handled using PostgreSQL.

## Using Gemini to Query/Interact with PostgreSQL Data (via Function Calling)

The `go-genai` SDK itself does not directly interact with the database. Instead, it enables the Gemini model to request that our Go backend application perform specific database actions using **Function Calling**.

**Roles:**

*   **`go-genai` SDK:** Manages communication with the Gemini API, sending prompts, function definitions, receiving generated text, and receiving function call requests from the model.
*   **Go Backend Application (WanderWiseAI):** Acts as the "tool user". It defines the available database tools, executes database operations using standard Go drivers (`pgx`) when requested by Gemini via a function call, and sends the results back to Gemini.

**Workflow:**

1.  **Define "Tools" in Go:** Within the Go backend (e.g., in relevant service or platform layers), create functions that perform specific, well-defined database operations. Examples:
    ```go
    // Example: Get schema for a specific table
    func getTableSchema(tableName string) (string, error) {
        // ... connect to Postgres using pgx ...
        // ... query information_schema for the table ...
        // ... format and return schema details ...
    }

    // Example: Execute a safe, read-only SQL query
    // WARNING: Ensure extreme caution regarding SQL injection if query isn't fully controlled/validated.
    func executeReadOnlySQL(query string) (string, error) {
        // ... connect to Postgres using pgx ...
        // ... execute the read-only query ...
        // ... format and return results (e.g., as JSON or formatted text) ...
    }

    // Example: More specific, safer function
    func findPOIsNearLocation(lat, lon float64, radiusMeters int) ([]domain.POI, error) {
        // ... connect to Postgres using pgx ...
        // ... construct and execute a specific PostGIS query using parameters ...
        // ... return structured POI data ...
    }
    ```

2.  **Declare Functions to Gemini:** Using `go-genai`, declare these Go functions as callable "tools" available to the Gemini model. This involves providing the function name, a clear description of its purpose, and the expected parameters.

3.  **User Interaction & Prompting:**
    *   A user request (e.g., "Show me coffee shops near me") triggers the Go backend.
    *   The backend constructs a prompt for Gemini, including the user's query and the definitions of the available tools (like `findPOIsNearLocation`).
    *   The prompt and tools are sent to the Gemini API via `go-genai`.

4.  **Gemini Requests Function Call:** Gemini analyzes the prompt. If it determines it needs data from the database, it doesn't run SQL itself. Instead, it generates a `FunctionCall` response (sent back via `go-genai`) asking the Go backend to execute a specific declared function (e.g., call `findPOIsNearLocation` with specific lat/lon/radius).

5.  **Go Backend Executes Function:** The Go backend receives the `FunctionCall`, parses it, identifies the requested local Go function (e.g., `findPOIsNearLocation`), and executes it using `pgx` against the PostgreSQL database.

6.  **Send Results Back to Gemini:** The Go backend takes the result from the executed function (e.g., a list of nearby POIs) and sends it back to the Gemini API as a `FunctionResponse` via `go-genai`.

7.  **Gemini Generates Final Response:** Gemini incorporates the data received from the function execution into its context and generates a final, user-friendly natural language response (e.g., "Okay, here are some coffee shops near you: Cafe Central, The Daily Grind...").

8.  **Response to User:** The Go backend relays Gemini's final text response to the frontend.

**In essence:** The Go application performs the actual database work, guided by Gemini's requests facilitated through the `go-genai` function calling mechanism.

## Storing Gemini-Related Data in PostgreSQL

PostgreSQL is well-suited for storing data generated by or related to interactions with the Gemini models.

**1. Standard Data Types (No Special Extensions Needed):**

For most common use cases, standard PostgreSQL data types suffice:

*   **`TEXT` / `VARCHAR`:** Store generated text (recommendations, summaries, chat responses), prompts sent to the API, model names, etc. `TEXT` is generally suitable for variable, potentially long strings.
*   **`JSONB`:** **Highly recommended** for storing:
    *   Structured data requested from the LLM (e.g., asking it to output JSON).
    *   Complete request/response logs from `go-genai` interactions (capturing prompts, function calls, responses, safety ratings, etc.).
    *   Function call details (arguments, results).
    *   *Benefit:* `JSONB` is indexed efficiently and allows querying nested data within the JSON object directly using SQL.
*   **`TIMESTAMP WITH TIME ZONE` (`timestamptz`):** Record the time of interactions.
*   **`INTEGER`, `BIGINT`, `FLOAT`, `BOOLEAN`:** Store metadata like token counts, user IDs associated with interactions, latency measurements, flags, etc.

**2. Vector Embeddings (`pgvector` Extension Recommended):**

If you plan to work with **vector embeddings** (numerical representations of text capturing semantic meaning, often generated by models like `text-embedding-004`), using a specialized extension is crucial for performance.

*   **Use Case:** Semantic search (finding similar POI descriptions), content-based recommendations, clustering based on meaning.
*   **Recommended Extension:** [`pgvector`](https://github.com/pgvector/pgvector)
*   **`pgvector` Features:**
    *   Adds a `vector` data type for storing embedding arrays.
    *   Provides efficient index types (e.g., **HNSW**, IVFFlat) for fast Approximate Nearest Neighbor (ANN) searches. *This is essential for performance.*
    *   Includes vector distance operators (`<=>` for cosine distance, `<->` for L2, `<#*>` for inner product) usable directly in SQL queries.

**Recommended Storage Practices:**

*   **Thoughtful Schema Design:** Create appropriate tables (`llm_interactions`, `points_of_interest`, etc.) using the standard types mentioned above.
*   **Interaction Logging:** Consider a dedicated table (e.g., `llm_interactions`) with `JSONB` columns for `request_payload` and `response_payload` to capture full interaction details for debugging, analysis, or fine-tuning. Include relevant metadata like `user_id`, `timestamp`, `model_used`, `token_counts`.
*   **Embeddings Strategy (if using `pgvector`):**
    1.  **Install:** Add the `pgvector` extension to your PostgreSQL database.
    2.  **Add Column:** Add a `vector(<dimension>)` column (e.g., `vector(768)`) to relevant tables (e.g., `points_of_interest`).
    3.  **Generate & Store:** Use the appropriate embedding model (via API call) to generate vectors for your text data and store them in the `vector` column.
    4.  **Index:** Create an HNSW or IVFFlat index on the `vector` column for efficient querying.
    5.  **Query:** Use `pgvector`'s distance operators in your SQL queries (e.g., `ORDER BY embedding <=> $1 LIMIT 10`) to find semantically similar items.

**Summary:** Rely on PostgreSQL's robust standard features (`TEXT`, `JSONB`, etc.) for most data storage related to `go-genai`. Integrate the `pgvector` extension specifically when you need to store and perform efficient similarity searches on vector embeddings.
GCP, GKE, Terraform, Vault, GO, Ansible
---

## 🤝 Contributing

> 🛠 _Contribution guidelines and code of conduct coming soon._

---

## 📄 License

> 📃 _License type to be defined (MIT, Apache 2.0, or Proprietary)._