You've got a fantastic foundation for your MVP! The core loop of fetching user context, feeding it to the AI, and getting a structured response for a city is in place.

For an MVP, the goal is to deliver core value quickly and get user feedback. Let's look at what "core value" means for WanderWiseAI and what small additions would significantly enhance that.

**Current MVP Strengths (Based on your code and description):**

*   User authentication.
*   Fetching user's interests, search preferences, and tags.
*   AI integration to generate:
    *   General city information (name, country, description).
    *   General POIs for the city.
    *   Personalized itinerary name, description, and POIs based on user context.
*   Persistence of cities and general POIs.

**Suggestions to Enhance the MVP (Focus on User Experience & Core Loop):**

Based on your "Elevator Pitch" and "Core Features," here are some high-impact additions/refinements for the MVP:

1.  **Visualizing Recommendations on a Map (Crucial for a Travel App):**
    *   **Why:** "Interactive Map Integration" is a core feature. Users expect to see where things are.
    *   **Implementation:**
        *   Ensure your `types.POIDetail` consistently has `latitude` and `longitude`.
        *   Frontend: Integrate Leaflet, Mapbox GL JS, or MapLibre GL JS.
        *   API: Your `/api/v1/recommendations` (or similar endpoint that calls `GetPromptResponse`) should return the `AiCityResponse` with POI coordinates.
        *   Frontend displays markers for the POIs in `AiCityResponse.PointsOfInterest` and `AiCityResponse.AIItineraryResponse.PointsOfInterest`.
        *   **MVP Level:** Simple markers. Clicking a marker could show the POI name and description.

2.  **Basic Filtering on the Frontend (Leveraging Backend Capabilities):**
    *   **Why:** "Contextual Filtering" is core. Even if the *AI* considers preferences, users often want to tweak or see results based on one or two *explicit* filters.
    *   **Implementation:**
        *   **Backend Already Supports It (Implicitly):** Your AI prompts already take `userPrefs` which includes radius, time, budget, etc.
        *   **Frontend:**
            *   When the user enters a city, after Stage 1 & 2 (general city info & POIs) are displayed, show simple filter controls (e.g., a dropdown for primary interest category, maybe a "Open Now" toggle if your POI data supports it yet - probably not for MVP if AI doesn't give opening hours reliably).
            *   When these filters change, the frontend re-requests the *personalized itinerary* (Stage 3) from the backend, passing the new filter values. The backend then uses these in the `userPrefs` for the AI prompt.
        *   **MVP Level:** Start with 1-2 key filters like "Primary Interest Category" or "Search Radius" that the frontend sends to the backend to influence the *personalized AI prompt*. The AI then does the "filtering" by generating relevant content. Don't overcomplicate frontend filtering logic itself if the AI can handle it via prompting.

3.  **Saving/Bookmarking Favorite POIs (Simple Version):**
    *   **Why:** "Save & Organize" is core. Users want to remember places.
    *   **Implementation:**
        *   **Database:** You'll need a `user_saved_pois` table (e.g., `user_id UUID, poi_id UUID, created_at TIMESTAMPTZ, PRIMARY KEY (user_id, poi_id)`).
        *   **Backend API:**
            *   `POST /api/v1/users/me/saved-pois` (body: `{ "poi_id": "..." }`)
            *   `DELETE /api/v1/users/me/saved-pois/{poi_id}`
            *   `GET /api/v1/users/me/saved-pois` (to list saved POIs)
        *   **Frontend:**
            *   A "save" icon on each POI card.
            *   A separate "My Saved Places" page/section that lists POIs fetched from the GET endpoint.
        *   **MVP Level:** Just saving individual POIs. Custom lists/itineraries can come in Phase 2.

4.  **Clearer Separation of General POIs vs. Personalized Itinerary POIs in UI:**
    *   **Why:** To manage user expectations and guide them.
    *   **Implementation:**
        *   After the user enters a city, your UI first shows `AiCityResponse.GeneralCityData` and `AiCityResponse.PointsOfInterest` (the general ones). This is "Explore [CityName]".
        *   Then, have a clear Call to Action (CTA) like "Get Personalized Suggestions" or "Build My Itinerary." Clicking this triggers the backend to generate/return the `AiCityResponse.AIItineraryResponse` part (which involves the personalized AI prompt).
        *   This makes the 3-stage AI interaction more explicit to the user.

5.  **Refine User Onboarding for Preferences:**
    *   **Why:** To get the initial data for personalization.
    *   **Implementation:**
        *   A simple onboarding flow after signup where users can select their top interests (from a predefined list that maps to what your AI understands).
        *   These are saved via your existing `interests.interestsRepo`.
        *   The `searchProfile` can have sensible defaults initially, with a separate "Settings" page for users to adjust radius, pace, vibes, etc., later.

**What to Defer from the MVP (based on your current plan & complexity):**

*   **Complex User-Created Itineraries (Ordering, Day Planning):** Simple saving of POIs is enough for MVP.
*   **Advanced AI (Function Calling, Vector Search for POIs):** Your current prompt-based generation is a great start. Defer RAG or complex function calling unless absolutely necessary for core functionality.
*   **User Reviews & Ratings:** Adds significant complexity.
*   **Social Features (Following, Sharing Lists):** Definitely post-MVP.
*   **Rich POI Data Beyond AI Output (Opening Hours, Prices, etc.):** For MVP, what the AI provides for POIs is likely sufficient. Data enrichment from other sources (OSM, Google Places API directly) is a Phase 2+ item.
*   **Offline Access.**
*   **Ads / Premium Tier Mechanics:** Focus on delivering free value first.

**Multi-Session Chat / Editing Preferences & Updating Response:**

*   **For MVP: Simpler is Better - Regenerate Response.**
    *   If a user types "Barcelona," gets results, then edits preferences (e.g., adds "Art Galleries" as an interest on the frontend filter), the simplest MVP approach is for the frontend to make a *new complete request* to your backend `/api/v1/recommendations` endpoint.
    *   Your backend `GetPromptResponse` will then re-fetch user context (including the now-updated preferences if they were saved globally, or just use the request-specific filters) and re-run the necessary AI prompts (likely just the personalized itinerary part if general city info is already fetched/cached).
    *   **Why this is okay for MVP:**
        *   It's much simpler to implement on both frontend and backend.
        *   Guarantees the AI has the latest full context.
        *   Token costs are a concern but for an MVP, functionality and speed of development often outweigh minor cost optimizations.

*   **"Editing" the Response (More Complex - Post-MVP):**
    *   This implies a more conversational AI interaction where you'd send the *previous AI output* and the *user's change* back to the AI, asking it to "modify the previous itinerary to now also include art galleries."
    *   **Challenges:**
        *   **Prompt Engineering:** Crafting prompts for modification is harder than for generation from scratch.
        *   **State Management:** You need to maintain the "session" or "chat history" (your commented-out `sessions` map hints at this). This state needs to be passed back and forth with the AI.
        *   **AI Capability:** The AI needs to be good at understanding diffs and making targeted edits without ruining the rest of the itinerary. Gemini's chat models (`model.StartChat()`) are designed for this, but it adds complexity to your backend logic.
        *   **Token Usage:** Sending history increases token count, potentially negating savings from not regenerating everything.

    *   **How `genai.ChatSession` would fit:** If you go this route, your `LlmInteractiontServiceImpl` would:
        1.  When a user starts interacting with a city, create or retrieve a `genai.ChatSession` (e.g., keyed by `userID` + `cityID`).
        2.  The first prompt (e.g., for personalized POIs) is sent via `chatSession.SendMessage()`.
        3.  If the user then changes a preference, you construct a new message like, "Okay, now also consider my interest in 'Art Galleries' and update the previous suggestions for Barcelona." You send this new message using the *same* `chatSession.SendMessage()`. The SDK handles sending the relevant history.
        4.  The AI's response should ideally be an updated itinerary.
        5.  You'd need to persist this chat history if you want it to survive server restarts or be multi-device.

**Recommendation for Chat/Editing:** For the MVP, stick to regenerating the personalized part of the response when preferences change. It's robust and easier to implement. You can explore true chat-based refinement in Phase 2. Your current 3-prompt structure is *not* inherently a "chat" with the AI; it's three distinct (though related) requests.

By focusing on a strong map visualization, basic but useful filtering that influences AI prompts, and simple POI saving, your MVP will provide significant value and a solid platform for user feedback and future iterations.

---

You're asking excellent questions that get to the heart of designing a good user experience and a maintainable backend! Let's break this down.

**1. Sorting Personalized POIs: User Location in HandlerImpl vs. Separate Service?**

*   **Passing User Location via HandlerImpl (Recommended for this use case):**
    *   **How it works:** The frontend client (web browser, mobile app) determines the user's current latitude and longitude (e.g., using the browser's Geolocation API). It then includes these coordinates as parameters in the API request to your backend endpoint that triggers `GetPromptResponse`.
    *   Example API call: `GET /api/v1/city-discovery?city=Barcelona&user_lat=41.3851&user_lon=2.1734`
    *   Your HTTP HandlerImpl in Go would parse `user_lat` and `user_lon` from the query parameters and pass them down to your `LlmInteractiontServiceImpl.GetPromptResponse` method.
    *   **Pros:**
        *   **Direct and Contextual:** The user's current location is most relevant *at the moment they are requesting* the itinerary.
        *   **Stateless (for this specific parameter):** The backend doesn't need to store or guess the user's "current" location for this specific request; it's provided.
        *   **Standard Practice:** This is a very common way to handle location-aware requests.
    *   **Cons:**
        *   The frontend needs to be ableto obtain and send the location.
        *   If location services are off, you need a fallback (e.g., use city center, or don't sort by distance).

*   **Separate Location Service (Less ideal for *this specific sorting task*, but useful for other things):**
    *   **How it might work:** You could have a service that tries to determine or store a user's "last known location" or "home location."
    *   **Pros:**
        *   Useful if you want to send push notifications based on location or have a default location if the frontend can't provide one.
    *   **Cons for Itinerary Sorting:**
        *   **Staleness:** A stored "last known location" might not be their *current* location when they request an itinerary.
        *   **Complexity:** Adds another service call and state management.
        *   **User Intent:** For sorting an itinerary *right now*, their *current* location is usually what's intended.

    **Recommendation:** For sorting the *personalized itinerary POIs immediately after AI generation and for the current user request*, **pass the user's current latitude and longitude from the frontend through the API HandlerImpl to your `GetPromptResponse` service method.** This is the most direct and contextually relevant approach.

    Your `LlmInteractiontServiceImpl.GetPromptResponse` method signature would then be:
    ```go
    func (l *LlmInteractiontServiceImpl) GetPromptResponse(
        ctx context.Context,
        cityName string,
        userID, profileID uuid.UUID,
        userLat float64, // from HandlerImpl
        userLon float64, // from HandlerImpl
    ) (*types.AiCityResponse, error)
    ```

**2. "AI should return everything on one click"**

Your current 3-goroutine approach within `GetPromptResponse` already aims to achieve this. The user makes one logical request (e.g., "Show me Barcelona"), and your backend orchestrates multiple AI calls concurrently to assemble the complete `AiCityResponse`. This is a good design. The PostGIS sorting becomes another step in this backend orchestration *before* the final consolidated response is sent back.

**MVP Features Refinement & Implementation Strategy:**

Let's map your desired MVP features to your existing structure and the new sorting requirement.

**A. Use PostGIS to sort itinerary:**

*   **Done (Conceptually):** We've outlined how to integrate this into `GetPromptResponse` by adding a new method to `POIRepository` (`FindAndSortPOIsByNamesAndDistance`) and calling it after the AI generates personalized POIs and the `cityID` is known.
*   **Key:** Ensure `userLat` and `userLon` are passed into `GetPromptResponse`.

**B. CRUD Itinerary, City, POI:**

You already have the "C" (Create) and parts of "R" (Read - `Find...`) for City and POI. Let's expand.

*   **City CRUD:**
    *   **Create:** `cityRepo.SaveCity` (You have this, make sure it handles updates if `FindCityByNameAndCountry` returns an existing city and the `AiSummary` or `center_location` needs updating - this might mean `SaveCity` becomes an "Upsert" or you have a separate `UpdateCity` method).
    *   **Read:**
        *   `cityRepo.FindCityByNameAndCountry` (You have this).
        *   `cityRepo.GetCityByID(ctx context.Context, id uuid.UUID) (*types.CityDetail, error)` (Good to have).
        *   `cityRepo.ListCities(ctx context.Context, /* pagination params */) ([]types.CityDetail, error)` (For browsing).
    *   **Update:** `cityRepo.UpdateCity(ctx context.Context, city types.CityDetail) error` (To update summary, location, etc.).
    *   **Delete:** `cityRepo.DeleteCity(ctx context.Context, id uuid.UUID) error` (Consider soft deletes).

*   **POI CRUD:**
    *   **Create:** `poiRepo.SavePoi` (You have this. It saves the general POI details).
    *   **Read:**
        *   `poiRepo.FindPoiByNameAndCity` (You have this).
        *   `poiRepo.GetPoiByID(ctx context.Context, id uuid.UUID) (*types.POIDetail, error)`.
        *   `poiRepo.ListPOIsByCity(ctx context.Context, cityID uuid.UUID, /* filters, pagination */) ([]types.POIDetail, error)`.
        *   Your new `poiRepo.FindAndSortPOIsByNamesAndDistance` is also a specialized read.
    *   **Update:** `poiRepo.UpdatePOI(ctx context.Context, poi types.POIDetail) error`.
    *   **Delete:** `poiRepo.DeletePOI(ctx context.Context, id uuid.UUID) error`.

*   **Itinerary CRUD (This is NEW for explicit user-saved itineraries):**
    *   **Schema (as discussed before):**
        *   `itineraries (id UUID PK, user_id UUID FK, city_id UUID FK, name TEXT, overall_description_ai TEXT, created_at, updated_at)`
        *   `itinerary_pois (itinerary_id UUID FK, poi_id UUID FK, order_index INT, personalized_reasoning_ai TEXT, PK(itinerary_id, poi_id))`
    *   **Repository (`ItineraryRepository`):**
        *   `SaveItinerary(ctx context.Context, itineraryHeader types.ItineraryHeader, pois []types.ItineraryPOIDetailForSave) (uuid.UUID, error)`:
            *   `types.ItineraryHeader` would contain `UserID`, `CityID`, `Name`, `OverallDescriptionAI`.
            *   `types.ItineraryPOIDetailForSave` would contain `POIID` (global ID), `OrderIndex`, `PersonalizedReasoningAI`.
            *   This method would create a record in `itineraries` and multiple records in `itinerary_pois` within a transaction.
        *   `GetItineraryByID(ctx context.Context, itineraryID uuid.UUID, userID uuid.UUID) (*types.SavedItinerary, error)`: Fetches the header and its associated POIs (with personalized reasoning).
        *   `ListItinerariesByUser(ctx context.Context, userID uuid.UUID) ([]types.ItineraryHeader, error)`.
        *   `UpdateItineraryHeader(ctx context.Context, itineraryHeader types.ItineraryHeader) error`.
        *   `UpdateItineraryPOIs(ctx context.Context, itineraryID uuid.UUID, pois []types.ItineraryPOIDetailForSave) error` (more complex: delete old, insert new).
        *   `DeleteItinerary(ctx context.Context, itineraryID uuid.UUID, userID uuid.UUID) error`.
    *   **Service (`ItineraryService`):**
        *   `CreateUserItinerary(ctx, userID, cityID, name, description, []PersonalizedPOIDetailFromAI)`: This would be called when the user clicks "Save Itinerary" on the AI-generated response. It would take the relevant data from `AiCityResponse.AIItineraryResponse`, ensure global POIs are saved (to get their IDs), and then call `ItineraryRepository.SaveItinerary`.
        *   Methods for listing, getting details, updating, deleting user's saved itineraries.

**C. Create Markdown system to save itineraries. (New table schema)**

*   **Concept:** Instead of (or in addition to) saving structured `itinerary_pois` with personalized reasons, you want to save the *entire AI-generated itinerary* as a Markdown document for display, sharing, or simpler storage.
*   **Schema:**
    *   `saved_markdown_itineraries (id UUID PK, user_id UUID FK, city_id UUID FK, itinerary_name TEXT, markdown_content TEXT, created_at, updated_at)`
*   **Implementation:**
    *   When the AI generates the `AIItineraryResponse`, your backend service (`LlmInteractiontServiceImpl` or the new `ItineraryOrchestrationService`) could format parts of this response (itinerary name, overall description, list of personalized POIs with their descriptions and reasons) into a Markdown string.
    *   When the user clicks "Save Itinerary (as Markdown)", this Markdown string is saved to the new table.
*   **Pros:** Simple to display, easy to share as text.
*   **Cons:** Less structured for querying specific POIs within it or re-using components. It's more of a "snapshot."
*   **Suggestion:** You could offer *both*. Save the structured itinerary (for in-app use, map display, future editing) AND generate/save a Markdown version (for export, simple display). For MVP, pick one. Saving the structured itinerary (B) is generally more powerful for in-app features. Markdown is a nice-to-have for export.

**D. Create Favourite system to save city and Itineraries. (New table schema)**

*   **Save City (Favourites):**
    *   **Schema:** `user_favourite_cities (user_id UUID FK, city_id UUID FK, created_at TIMESTAMPTZ, PRIMARY KEY (user_id, city_id))`
    *   **Repo/Service:** Simple methods to add/remove/list favourite cities for a user.

*   **Save Itineraries (Favourites):**
    *   This is essentially what "Save Itinerary" (Point B above) achieves by creating records in the `itineraries` table linked to a `user_id`. You are saving the itinerary itself. "Favouriting" an itinerary *is* saving it.
    *   If you want a separate "quick access" favourite flag on *already saved itineraries*, you could add an `is_favourite BOOLEAN` column to your `itineraries` table.

**E. Inside the prompt, click on POI (personalised and general) and have the AI return extra details.**

*   **Concept:** User clicks on a POI name shown in the UI. You want to get more details *about that specific POI* from the AI.
*   **Implementation:**
    1.  **Frontend:** When a POI is clicked, it sends a request to a new backend endpoint, e.g., `GET /api/v1/poi-details-ai?poi_name=[NAME]&city=[CITY_NAME]&[optional_user_context_params]`.
    2.  **Backend (New Service Method, e.g., in `POIDiscoveryService`):**
        *   `GetExtraPOIDetailsFromAI(ctx context.Context, poiName, cityName, countryName string, userContext *UserAIContext /* optional */) (*types.ExtraPOIDetail, error)`
        *   **Prompt Engineering:** Craft a new prompt specifically for this.
            *   `"Provide extensive details for the Point of Interest: '[POI Name]' in '[City Name], [Country Name]'. Include its history, what makes it special, visitor tips, typical visiting duration, and any interesting anecdotes. If relevant to the user's profile (Interests: [User Interests]), highlight those aspects. Return as JSON: {"extended_description": "...", "history": "...", "visitor_tips": "...", "typical_duration": "...", "personalized_notes": "..."}"`
        *   Call the AI client.
        *   Parse the response.
        *   Return the extra details.
    *   **UI:** Displays these extended details in a modal or a dedicated POI detail view.
*   **Considerations:**
    *   **Token Cost:** Each click is another AI call.
    *   **Latency:** User waits for AI response.
    *   **Caching:** Cache responses for `(poi_name, city_name)` to reduce AI calls if the general details don't change frequently. Personalized notes would be harder to cache unless user context is also part of the cache key.
    *   **Alternative - Database First:** Before hitting the AI, you could first check your `points_of_interest` table. If you have a decent `description` or `ai_summary` already stored there, show that first. The "get more from AI" could be an explicit button for "Deeper Dive" or if your stored info is minimal.

**MVP Prioritization for "Rest of MVP":**

1.  **Core: PostGIS sorting of the AI-generated personalized itinerary.** (Integrate into current `GetPromptResponse`).
2.  **Core: Save/List individual POIs (Favourites).** (New `UserSavedPOIRepository` and service methods).
3.  **High Value: Save AI-Generated Itineraries (Structured).** (New `ItineraryRepository` and service methods for `itineraries` and `itinerary_pois`). This is a key differentiator.
4.  **Useful: Save Cities (Favourites).** (New `UserFavouriteCityRepository`).
5.  **Nice-to-have / Phase 2: Markdown Itineraries.**
6.  **Nice-to-have / Phase 2: Click POI for *more AI details*.** Start by just displaying the details the AI *already provided* in the initial generation. On-demand extra AI detail is an enhancement.

Focus on making the primary "one-click" experience (getting a sorted, personalized itinerary and general city info) solid, and allow users to save the valuable outputs (POIs and the full AI itinerary). CRUD operations for these saved items are then natural follow-ons.