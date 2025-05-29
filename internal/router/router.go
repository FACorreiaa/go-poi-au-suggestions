// internal/router/router.go
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors" // Import CORS middleware if needed

	appMiddleware "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	llmInteraction "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/llm_interaction"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/poi"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user"
	userInterest "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_interests"
	userProfiles "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_search_profiles"
	userSettings "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_settings"
	userTags "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user_tags"
)

// Config contains dependencies needed for the router setup
type Config struct {
	AuthHandler              *appMiddleware.AuthHandler
	AuthenticateMiddleware   func(http.Handler) http.Handler // Function signature for auth middleware
	Logger                   *slog.Logger
	UserHandler              *user.HandlerUser
	UserInterestHandler      *userInterest.UserInterestHandler
	UserSettingsHandler      *userSettings.SettingsHandler
	UserSearchProfileHandler *userProfiles.UserSearchProfileHandler
	UserTagsHandler          *userTags.UserTagsHandler
	LLMInteractionHandler    *llmInteraction.LlmInteractionHandler
	PointsOfInterestHandler  *poi.POIHandler
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
			r.Post("/auth/register", cfg.AuthHandler.Register) // Assuming Register handler exists
			r.Post("/auth/login", cfg.AuthHandler.Login)
			//r.Post("/auth/refresh", cfg.AuthHandler.RefreshSession) // Assuming RefreshSession exists
		})

		// --- Protected Routes ---
		// Routes under this group WILL require JWT authentication
		r.Group(func(r chi.Router) {
			// Apply the authentication middleware passed from main.go
			r.Use(cfg.AuthenticateMiddleware)

			// Mount protected auth routes
			r.Post("/auth/logout", cfg.AuthHandler.Logout)
			//r.Get("/auth/session/{sessionID}", cfg.AuthHandler.GetSession)                    // Needs Auth? Usually yes.
			r.Get("/auth/validate-session", cfg.AuthHandler.ValidateSession) // Needs Auth
			//r.Post("/auth/verify-password", cfg.AuthHandler.VerifyPassword)                   // Needs Auth
			r.Put("/auth/update-password", cfg.AuthHandler.ChangePassword) // Needs Auth (use PUT for update)
			//r.Post("/auth/invalidate-tokens", cfg.AuthHandler.InvalidateAllUserRefreshTokens) // Needs Auth

			// Mount other protected resource routes
			//r.Mount("/user", UserRoutes(cfg.UserHandler)) // User routes
			r.Mount("/user/interests", UserInterestRoutes(cfg.UserInterestHandler))
			r.Mount("/user/preferences", UserPreferencesRoutes(cfg.UserSettingsHandler))
			r.Mount("/user/search-profile", UserSearchProfileRoutes(cfg.UserSearchProfileHandler))
			r.Mount("/user/tags", UserTagsRoutes(cfg.UserTagsHandler))
			r.Mount("/llm", LLMInteractionRoutes(cfg.LLMInteractionHandler))
			r.Mount("/pois", POIRoutes(cfg.PointsOfInterestHandler)) // Points of Interest routes
			// r.Mount("/pois", POIRoutes(cfg.POIHandler))   // Example for POI routes
		})

		// --- Admin Routes (Example) ---
		// Nested group for routes requiring admin role (add specific admin middleware)
		// r.Group(func(r chi.Router) {
		// 	r.Use(cfg.AuthenticateMiddleware)
		// 	r.Use(middleware.RequireRole("admin"))                         // Example: you'd create this middleware
		// 	r.Get("/auth/user-role/{userID}", cfg.AuthHandler.GetUserRole) // Example Admin action
		// 	// r.Mount("/admin", AdminRoutes(cfg.AdminHandler))
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
			// r.Get("/pois/advanced-search", cfg.POIHandler.AdvancedSearch)
			// r.Get("/guides/exclusive/{guideID}", cfg.GuideHandler.GetExclusiveGuide)
		})

		// --- Admin Routes ---
		// ... (Apply Authenticate + Admin role check middleware) ...

	})

	return r
}

// UserRoutes creates a router for user-related endpoints
//func UserRoutes(handler *user.HandlerUser) http.Handler {
//	r := chi.NewRouter()
//
//	// All user routes require authentication, handled at the parent router level
//
//	// User profile routes
//	r.Get("/profile", handler.GetUserProfile)    // GET http://localhost:8000/api/v1/user/profile
//	r.Put("/profile", handler.UpdateUserProfile) // PUT http://localhost:8000/api/v1/user/profile
//	return r
//}

func UserTagsRoutes(handler *userTags.UserTagsHandler) http.Handler {
	r := chi.NewRouter()

	r.Get("/", handler.GetTags) // GET http://localhost:8000/api/v1/user/tags
	r.Get("/{tagID}", handler.GetTag)
	r.Delete("/{tagID}", handler.DeleteTag)
	r.Put("/{tagID}", handler.DeleteTag)
	r.Post("/", handler.CreateTag)

	return r
}

// TODO
func UserInterestRoutes(handler *userInterest.UserInterestHandler) http.Handler {
	r := chi.NewRouter()
	// Interest routes
	r.Get("/", handler.GetAllInterests) // GET http://localhost:8000/api/v1/user/interests
	r.Post("/create", handler.CreateInterest)
	r.Put("/{interestID}", handler.UpdateUserInterest)    // POST http://localhost:8000/api/v1/user/interests/create
	r.Delete("/{interestID}", handler.RemoveUserInterest) // DELETE http://localhost:8000/api/v1/user/interests/{interestID}
	return r
}

// UserPreferencesRoutes ..
func UserPreferencesRoutes(handler *userSettings.SettingsHandler) http.Handler {
	r := chi.NewRouter()
	// User preferences routes

	r.Get("/", handler.GetUserSettings) // GET http://localhost:8000/api/v1/user/preferences
	r.Put("/{profileID}", handler.UpdateUserSettings)

	return r
}

func UserSearchProfileRoutes(handler *userProfiles.UserSearchProfileHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/{profileID}", handler.GetSearchProfile)
	r.Get("/default", handler.GetDefaultSearchProfile)             // GET http://localhost:8000/api/v1/user/search-profile/default
	r.Put("/default/{profileID}", handler.SetDefaultSearchProfile) // PUT http://localhost:8000/api/v1/user/search-profile/default
	r.Put("/{profileID}", handler.UpdateSearchProfile)             // PUT http://localhost:8000/api/v1/user/search-profile/{profileID}
	r.Delete("/{profileID}", handler.DeleteSearchProfile)          // DELETE http://localhost:8000/api/v1/user/search-profile/{profileID}
	r.Get("/", handler.GetSearchProfiles)                          // GET http://localhost:8000/api/v1/user/search-profile
	r.Post("/", handler.CreateSearchProfile)                       // POST http://localhost:8000/api/v1/user/search-profile

	return r
}

func LLMInteractionRoutes(handler *llmInteraction.LlmInteractionHandler) http.Handler {
	r := chi.NewRouter()
	// LLM interaction routes
	r.Post("/prompt-response/profile/{profileID}", handler.GetPrompResponse)        // GET http://localhost:8000/api/v1/user/interests
	r.Get("/prompt-response/poi/details", handler.GetPOIDetails)                    // GET http://localhost:8000/api/v1/llm/prompt-response/{interactionID}
	r.Post("/prompt-response/bookmark", handler.SaveItenerary)                      // POST http://localhost:8000/api/v1/llm/prompt-response
	r.Delete("/prompt-response/bookmark/{itineraryID}", handler.RemoveItenerary)    // DELETE http://localhost:8000/api/v1/llm/bookmark/{bookmarkID}
	r.Get("/prompt-response/city/hotel/preferences", handler.GetHotelsByPreference) // GET http://localhost:8000/api/v1/pois/city/hotel/preferences
	r.Get("/prompt-response/city/hotel/nearby", handler.GetHotelsNearby)            // GET http://localhost:8000/api/v1/pois/city/restaurant/preferences
	r.Get("/prompt-response/city/hotel/{hotelID}", handler.GetHotelByID)
	r.Get("/prompt-response/city/restaurants/preferences", handler.GetRestaurantsByPreferences)
	r.Get("/prompt-response/city/restaurants/nearby", handler.GetRestaurantsNearby)
	// TODO save on the db
	r.Get("/prompt-response/city/restaurants/{restaurantID}", handler.GetRestaurantDetails) // GET http://localhost:8000/api/v1/pois/city/poi/nearby
	return r
}

func POIRoutes(handler *poi.POIHandler) http.Handler {
	r := chi.NewRouter()
	// Points of Interest routes
	r.Get("/favourites", handler.GetFavouritePOIsByUserID)   // GET http://localhost:8000/api/v1/pois/favourites
	r.Post("/favourites", handler.AddPoiToFavourites)        // POST http://localhost:8000/api/v1/pois/favourites
	r.Delete("/favourites", handler.RemovePoiFromFavourites) // DELETE http://localhost:8000/api/v1/pois/favourites/{poiID}
	r.Get("/city/{cityID}", handler.GetPOIsByCityID)
	return r
}

// TODO
//func UserPreferencesTags(handler *user.HandlerUser) http.Handler {
//	r := chi.NewRouter()
//	// User preferences routes
//	r.Get("/preferences", handler.GetUserPreferences) // GET http://localhost:8000/api/v1/user/preferences
//
//	return r
//}

// Example of how you might structure feature-specific routes (optional)
// func AuthRoutes(handler *auth.AuthHandler, authMiddleware func(http.Handler) http.Handler) http.Handler {
// 	r := chi.NewRouter()
// 	// Public auth routes
// 	r.Post("/register", handler.Register)
// 	r.Post("/login", handler.Login)
//  r.Post("/refresh", handler.RefreshSession)
//
// 	// Protected auth routes
//  r.Group(func(r chi.Router){
//      r.Use(authMiddleware)
// 		r.Post("/logout", handler.Logout)
//      // ... other protected auth routes
//  })
// 	return r
// }

// Note: You still need to implement the actual logic within your handler methods
// (e.g., AuthHandler.Login, AuthHandler.Register, etc.)
