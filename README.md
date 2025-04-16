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

## 🤝 Contributing

> 🛠 _Contribution guidelines and code of conduct coming soon._

---

## 📄 License

> 📃 _License type to be defined (MIT, Apache 2.0, or Proprietary)._