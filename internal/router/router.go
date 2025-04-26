// internal/router/router.go
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors" // Import CORS middleware if needed

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	appMiddleware "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user"
)

// Config contains dependencies needed for the router setup
type Config struct {
	AuthHandler            *auth.AuthHandler
	AuthenticateMiddleware func(http.Handler) http.Handler // Function signature for auth middleware
	Logger                 *slog.Logger
	UserHandler            *user.UserHandler
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
			r.Mount("/user", UserRoutes(cfg.UserHandler)) // User routes
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
func UserRoutes(handler *user.UserHandler) http.Handler {
	r := chi.NewRouter()

	// All user routes require authentication, handled at the parent router level

	// User profile routes
	r.Get("/profile", handler.GetUserProfile)    // GET http://localhost:8000/api/v1/user/profile
	r.Put("/profile", handler.UpdateUserProfile) // PUT http://localhost:8000/api/v1/user/profile

	// User preferences routes
	r.Get("/preferences", handler.GetUserPreferences) // GET http://localhost:8000/api/v1/user/preferences

	// Interest routes
	r.Get("/interests", handler.GetAllInterests)                                                 // GET http://localhost:8000/api/v1/user/interests
	r.Post("/interests", handler.AddUserInterest)                                                // POST http://localhost:8000/api/v1/user/interests
	r.Post("/interests/create", handler.CreateInterest)                                          // POST http://localhost:8000/api/v1/user/interests/create
	r.Delete("/interests/{interestID}", handler.RemoveUserInterest)                              // DELETE http://localhost:8000/api/v1/user/interests/{interestID}
	r.Put("/interests/{interestID}/preference-level", handler.UpdateUserInterestPreferenceLevel) // PUT http://localhost:8000/api/v1/user/interests/{interestID}/preference-level

	// Enhanced interests route
	r.Get("/enhanced-interests", handler.GetUserEnhancedInterests) // GET http://localhost:8000/api/v1/user/enhanced-interests

	return r
}

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
