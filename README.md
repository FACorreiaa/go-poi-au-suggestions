Okay, here's a more concise version of your README, aiming to reduce repetition while retaining the essential information for WanderWiseAI.

---

# **WanderWiseAI** – Personalized City Discovery 🗺️✨

WanderWiseAI is a smart, mobile-first web application delivering hyper-personalized city exploration recommendations based on user interests, time, location, and an evolving AI engine. It starts with an HTTP/REST API, utilizing WebSockets/SSE for real-time features.

## 🚀 Elevator Pitch & Core Features

Tired of generic city guides? WanderWise learns your preferences (history, food, art, etc.) and combines them with your available time and location to suggest the perfect spots.

*   **🧠 AI-Powered Personalization:** Recommendations adapt to explicit preferences and learned behavior.
*   **🔍 Contextual Filtering:** Filter by distance, time, opening hours, interests, and soon, budget.
*   **🗺 Interactive Map Integration:** Visualize recommendations and routes.
*   **📌 Save & Organize:** Bookmark favorites and create lists/itineraries (enhanced in Premium).
*   **📱 Mobile-First Design:** Optimized for on-the-go web browsing.

## 💰 Business Model & Monetization

WanderWiseAI uses a **Freemium Model**:



*   **Free Tier:** Core recommendations, basic filters, limited saves, non-intrusive ads.
*   **Premium Tier (Subscription):** Enhanced/Advanced AI recommendations & filters (niche tags, cuisine, accessibility), unlimited saves, offline access, exclusive content, ad-free.

**Monetization Avenues:**

*   Premium Subscriptions
*   **Partnerships & Commissions:** Booking referrals (GetYourGuide, Booking.com, OpenTable), transparent featured listings, exclusive deals.
*   **Future:** One-time purchases (guides), aggregated anonymized trend data.

## 🛠 Technology Stack & Design Choices

The stack prioritizes performance, personalization, SEO, and developer experience.

*   **Backend:** **Go (Golang)** with **Chi/Gin Gonic**, **PostgreSQL + PostGIS** (for geospatial queries), `pgx` or `sqlc`.
    *   *Rationale:* Go for performance and concurrency; PostGIS for essential location features.
*   **Frontend:** **SvelteKit** *or* **Next.js (React)** with **Tailwind CSS**, **Mapbox GL JS/MapLibre GL JS/Leaflet**.
    *   *Rationale:* Modern SSR frameworks for SEO and performance.
*   **AI / Recommendation Engine:**

Direct Google Gemini API integration via `google/generative-ai-go` SDK.**
        *   *Rationale:* Leverage latest models (e.g., Gemini 1.5 Pro) for deep personalization via rich prompts and function calling to access PostgreSQL data (e.g., nearby POIs from PostGIS).
    *   **Vector Embeddings:** PostgreSQL with `pgvector` extension for semantic search and advanced recommendations.
*   **API Layer:** Primary **HTTP/REST API**.
    *   *Rationale:* Simplicity for frontend integration and broad compatibility. gRPC considered for future backend-to-backend needs.
*   **Authentication:** Standard JWT + `Goth` package for social logins.
*   **Infrastructure:** Docker, Docker Compose; Cloud (AWS/GCP/Azure for managed services like Postgres, Kubernetes/Fargate/Cloud Run); CI/CD (GitHub Actions/GitLab CI).

## 🗺️ Roadmap Highlights

*   **Phase 1 (MVP):** Core recommendation engine (Gemini-powered), user accounts, map view, itinerary personalisation. 
*   **Phase 2:** Premium tier, enhanced AI (embeddings, `pgvector`), add more gemini features like
- speech to text
- itinerary download to different formats (pdf/markdown)
- itinerary uploads
- 24/7 agent more personalised agent

 reviews/ratings, booking partnerships.
*   **Phase 3:** Multi-city expansion, curated content, native app exploration.

## 🧪 Getting Started

> 🔧 _Instructions for local setup coming soon._

## 🤝 Contributing

> 🛠 _Contribution guidelines and code of conduct coming soon._

## 📄 License

> 📃 _License type to be defined (MIT, Apache 2.0, or Proprietary)._

---
