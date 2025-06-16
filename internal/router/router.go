// internal/router/router.go
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors" // Import CORS middleware if needed

	appMiddleware "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	llmChat "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/chat_prompt"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/interests"
	itineraryList "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/list"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/poi"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/profiles"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/tags"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user"
)

// Config contains dependencies needed for the router setup
type Config struct {
	AuthHandler             *appMiddleware.HandlerImpl
	AuthenticateMiddleware  func(http.Handler) http.Handler // Function signature for auth middleware
	Logger                  *slog.Logger
	UserHandler             *user.HandlerImpl
	InterestHandler         *interests.HandlerImpl
	SearchProfileHandler    *profiles.HandlerImpl
	TagsHandler             *tags.HandlerImpl
	LLMInteractionHandler   *llmChat.HandlerImpl
	PointsOfInterestHandler *poi.HandlerImpl
	ItineraryListHandler    *itineraryList.HandlerImpl
}

// SetupRouter initializes and configures the main application router.
// Server-wide middleware (like logger, requestID, recoverer) are expected
// to be applied *before* mounting this router in main.go.
func SetupRouter(cfg *Config) chi.Router {
	r := chi.NewRouter()

	// Optional: Add CORS middleware if your frontend is on a different origin
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:3000", "https://your-frontend-domain.com"}, // Adjust origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any major browsers
	}))

	// Optional: Heartbeat/Health check endpoint (often public)

	// Group API routes, potentially versioning them
	r.Route("/api/v1", func(r chi.Router) {

		// --- Public Auth Routes ---
		// Routes that DO NOT require JWT authentication
		r.Group(func(r chi.Router) {
			// Example: Mount auth routes that don't need the JWT check
			r.Post("/auth/register", cfg.AuthHandler.Register) // Assuming Register HandlerImpl exists
			r.Post("/auth/login", cfg.AuthHandler.Login)
			r.Get("/auth/google", cfg.AuthHandler.LoginWithGoogle)
			r.Get("/auth/google/callback", cfg.AuthHandler.GoogleCallback)
			r.Post("/auth/refresh", cfg.AuthHandler.RefreshToken) // Refresh tokens via HttpOnly cookie
		})

		// --- Protected Routes ---
		// Routes under this group WILL require JWT authentication
		r.Group(func(r chi.Router) {
			// Apply the authentication middleware passed from main.go
			r.Use(cfg.AuthenticateMiddleware)

			// Mount protected auth routes
			r.Post("/auth/logout", cfg.AuthHandler.Logout)
			//r.Get("/auth/session/{sessionID}", cfg.AuthHandlerImpl.GetSession)                    // Needs Auth? Usually yes.
			r.Post("/auth/validate-session", cfg.AuthHandler.ValidateSession) // Needs Auth
			//r.Post("/auth/verify-password", cfg.AuthHandlerImpl.VerifyPassword)                   // Needs Auth
			r.Put("/auth/update-password", cfg.AuthHandler.ChangePassword) // Needs Auth (use PUT for update)
			//r.Post("/auth/invalidate-tokens", cfg.AuthHandlerImpl.InvalidateAllUserRefreshTokens) // Needs Auth

			// Mount other protected resource routes
			r.Mount("/user", UserRoutes(cfg.UserHandler)) // User routes
			r.Mount("/user/interests", interestsRoutes(cfg.InterestHandler))
			r.Mount("/user/search-profile", profilesRoutes(cfg.SearchProfileHandler))
			r.Mount("/user/tags", tagsRoutes(cfg.TagsHandler))
			r.Mount("/llm", LLMInteractionRoutes(cfg.LLMInteractionHandler))
			r.Mount("/pois", POIRoutes(cfg.PointsOfInterestHandler)) // Points of Interest routes
			r.Mount("/itineraries", ItineraryListRoutes(cfg.ItineraryListHandler))
			// r.Mount("/pois", POIRoutes(cfg.HandlerImpl))   // Example for POI routes
		})

		// --- Admin Routes (Example) ---
		// Nested group for routes requiring admin role (add specific admin middleware)
		// r.Group(func(r chi.Router) {
		// 	r.Use(cfg.AuthenticateMiddleware)
		// 	r.Use(middleware.RequireRole("admin"))                         // Example: you'd create this middleware
		// 	r.Get("/auth/user-role/{userID}", cfg.AuthHandlerImpl.GetUserRole) // Example Admin action
		// 	// r.Mount("/admin", AdminRoutes(cfg.AdminHandlerImpl))
		// })
		// --- Premium Routes (Require active premium subscription) ---
		r.Group(func(r chi.Router) {
			r.Use(cfg.AuthenticateMiddleware) // Must be authenticated
			// Apply premium check middleware
			r.Use(appMiddleware.RequirePlanStatus(
				cfg.Logger,
				[]string{"premium_monthly", "premium_annual"}, // List of allowed plans
				"active", // Required status
			))

			// Add routes specific to premium users
			// e.g., advanced filtering endpoint, unlimited list creation, curated guides
			// r.Get("/pois/advanced-search", cfg.HandlerImpl.AdvancedSearch)
			// r.Get("/guides/exclusive/{guideID}", cfg.GuideHandlerImpl.GetExclusiveGuide)
		})

		// --- Admin Routes ---
		// ... (Apply Authenticate + Admin role check middleware) ...

	})

	return r
}

// UserRoutes creates a router for user-related endpoints
func UserRoutes(HandlerImpl *user.HandlerImpl) http.Handler {
	r := chi.NewRouter()

	// All user routes require authentication, handled at the parent router level

	// User profile routes
	r.Get("/profile", HandlerImpl.GetUserProfile)    // GET http://localhost:8000/api/v1/user/profile
	r.Put("/profile", HandlerImpl.UpdateUserProfile) // PUT http://localhost:8000/api/v1/user/profile
	return r
}

func tagsRoutes(HandlerImpl *tags.HandlerImpl) http.Handler {
	r := chi.NewRouter()

	r.Get("/", HandlerImpl.GetTags) // GET http://localhost:8000/api/v1/user/tags
	r.Get("/{tagID}", HandlerImpl.GetTag)
	r.Delete("/{tagID}", HandlerImpl.DeleteTag)
	r.Put("/{tagID}", HandlerImpl.UpdateTag)
	r.Post("/", HandlerImpl.CreateTag)

	return r
}

// interestsRoutes TODO
func interestsRoutes(HandlerImpl *interests.HandlerImpl) http.Handler {
	r := chi.NewRouter()
	// Interest routes
	r.Get("/", HandlerImpl.GetAllInterests) // GET http://localhost:8000/api/v1/user/interests
	r.Post("/create", HandlerImpl.CreateInterest)
	r.Put("/{interestID}", HandlerImpl.Updateinterests)    // POST http://localhost:8000/api/v1/user/interests/create
	r.Delete("/{interestID}", HandlerImpl.Removeinterests) // DELETE http://localhost:8000/api/v1/user/interests/{interestID}
	return r
}

func profilesRoutes(HandlerImpl *profiles.HandlerImpl) http.Handler {
	r := chi.NewRouter()
	r.Get("/{profileID}", HandlerImpl.GetSearchProfile)
	r.Get("/default", HandlerImpl.GetDefaultSearchProfile)             // GET http://localhost:8000/api/v1/user/search-profile/default
	r.Put("/default/{profileID}", HandlerImpl.SetDefaultSearchProfile) // PUT http://localhost:8000/api/v1/user/search-profile/default
	r.Put("/{profileID}", HandlerImpl.UpdateSearchProfile)             // PUT http://localhost:8000/api/v1/user/search-profile/{profileID}
	r.Delete("/{profileID}", HandlerImpl.DeleteSearchProfile)          // DELETE http://localhost:8000/api/v1/user/search-profile/{profileID}
	r.Get("/", HandlerImpl.GetSearchProfiles)                          // GET http://localhost:8000/api/v1/user/search-profile
	r.Post("/", HandlerImpl.CreateSearchProfile)                       // POST http://localhost:8000/api/v1/user/search-profile
	return r
}

func LLMInteractionRoutes(HandlerImpl *llmChat.HandlerImpl) http.Handler {
	r := chi.NewRouter()

	// Legacy chat endpoints (maintain backward compatibility)
	r.Post("/prompt-response/chat/sessions/{profileID}", HandlerImpl.StartChatSessionHandler)
	r.Post("/prompt-response/chat/sessions/stream/{profileID}", HandlerImpl.StartChatSessionStreamHandler)
	r.Post("/prompt-response/chat/sessions/{sessionID}/messages", HandlerImpl.ContinueChatSessionHandler)
	r.Post("/prompt-response/chat/sessions/{sessionID}/messages/stream", HandlerImpl.ContinueSessionStreamHandler)

	// Unified chat endpoints
	r.Post("/prompt-response/chat/sessions/unified-chat/{profileID}", HandlerImpl.ProcessUnifiedChatMessage)
	r.Post("/prompt-response/chat/sessions/unified-chat/stream/{profileID}", HandlerImpl.ProcessUnifiedChatMessageStream)

	// LLM interaction routes
	//r.Post("/prompt-response/profile/{profileID}", HandlerImpl.GetPrompResponse)        // GET http://localhost:8000/api/v1/user/interests
	r.Get("/prompt-response/poi/details", HandlerImpl.GetPOIDetails)                    // GET http://localhost:8000/api/v1/llm/prompt-response/{interactionID}
	r.Get("/prompt-response/poi/nearby", HandlerImpl.GetPOIsByDistance)                 // GET http://localhost:8000/api/v1/llm/prompt-response/poi/nearby
	r.Post("/prompt-response/bookmark", HandlerImpl.SaveItenerary)                      // POST http://localhost:8000/api/v1/llm/prompt-response
	r.Delete("/prompt-response/bookmark/{itineraryID}", HandlerImpl.RemoveItenerary)    // DELETE http://localhost:8000/api/v1/llm/bookmark/{bookmarkID}
	r.Get("/prompt-response/city/hotel/preferences", HandlerImpl.GetHotelsByPreference) // GET http://localhost:8000/api/v1/pois/city/hotel/preferences
	r.Get("/prompt-response/city/hotel/nearby", HandlerImpl.GetHotelsNearby)            // GET http://localhost:8000/api/v1/pois/city/restaurant/preferences
	r.Get("/prompt-response/city/hotel/{hotelID}", HandlerImpl.GetHotelByID)
	r.Get("/prompt-response/city/restaurants/preferences", HandlerImpl.GetRestaurantsByPreferences)
	r.Get("/prompt-response/city/restaurants/nearby", HandlerImpl.GetRestaurantsNearby)
	// TODO save on the db
	r.Get("/prompt-response/city/restaurants/{restaurantID}", HandlerImpl.GetRestaurantDetails) // GET http://localhost:8000/api/v1/pois/city/poi/nearby

	return r
}

func POIRoutes(HandlerImpl *poi.HandlerImpl) http.Handler {
	r := chi.NewRouter()
	// Points of Interest routes
	r.Get("/favourites", HandlerImpl.GetFavouritePOIsByUserID)   // GET http://localhost:8000/api/v1/pois/favourites
	r.Post("/favourites", HandlerImpl.AddPoiToFavourites)        // POST http://localhost:8000/api/v1/pois/favourites
	r.Delete("/favourites", HandlerImpl.RemovePoiFromFavourites) // DELETE http://localhost:8000/api/v1/pois/favourites/{poiID}
	r.Get("/city/{cityID}", HandlerImpl.GetPOIsByCityID)
	r.Get("/itineraries", HandlerImpl.GetItineraries)                           // GET /api/v1/itineraries?page=1&page_size=20
	r.Get("/itineraries/itinerary/{itinerary_id}", HandlerImpl.GetItinerary)    // GET /api/v1/itineraries/{uuid}
	r.Put("/itineraries/itinerary/{itinerary_id}", HandlerImpl.UpdateItinerary) // PUT /api/v1/itineraries/{uuid}

	// Traditional search
	r.Get("/search", HandlerImpl.GetPOIs) // GET http://localhost:8000/api/v1/pois/search

	// Semantic search routes
	r.Route("/search", func(r chi.Router) {
		r.Get("/semantic", HandlerImpl.SearchPOIsSemantic)            // GET http://localhost:8000/api/v1/pois/search/semantic?query=romantic%20restaurants
		r.Get("/semantic/city", HandlerImpl.SearchPOIsSemanticByCity) // GET http://localhost:8000/api/v1/pois/search/semantic/city?query=museums&city_id={uuid}
		r.Get("/hybrid", HandlerImpl.SearchPOIsHybrid)                // GET http://localhost:8000/api/v1/pois/search/hybrid?query=outdoor%20activities&latitude=40.7128&longitude=-74.0060&radius=5.0
	})

	// Embedding management routes (for admin/maintenance)
	r.Route("/embeddings", func(r chi.Router) {
		r.Post("/generate", HandlerImpl.GenerateEmbeddingsForPOIs) // POST http://localhost:8000/api/v1/pois/embeddings/generate?batch_size=20
	})

	return r
}

func ItineraryListRoutes(h *itineraryList.HandlerImpl) http.Handler {
	r := chi.NewRouter()

	r.Post("/lists", h.CreateTopLevelListHandler)                                // Create a new top-level list
	r.Get("/lists", h.GetUserListsHandler)                                       // Get all top-level lists for a user
	r.Get("/lists/{listID}", h.GetListDetailsHandler)                            // Get details of a specific list
	r.Put("/lists/{listID}", h.UpdateListDetailsHandler)                         // Update a specific list
	r.Delete("/lists/{listID}", h.DeleteListHandler)                             // Delete a specific list
	r.Post("/lists/{parentListID}/itineraries", h.CreateItineraryForListHandler) // Create an itinerary within a parent list
	r.Post("/{itineraryID}/items", h.AddPOIListItemHandler)                      // Add a POI to an itinerary
	r.Put("/{itineraryID}/items/{poiID}", h.UpdatePOIListItemHandler)            // Update a POI in an itinerary
	r.Delete("/{itineraryID}/items/{poiID}", h.RemovePOIListItemHandler)         // Remove a POI from an itinerary
	return r
}
