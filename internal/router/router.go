// internal/api/router.go
package api

import (
	"net/http"

	// --- OR if handlers/services are directly under internal ---
	// "github.com/FACorreiaa/WanderWiseAI/internal/auth"
	// appMiddleware "github.com/FACorreiaa/WanderWiseAI/internal/middleware"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors" // Import CORS middleware if needed
)

// Config contains dependencies needed for the router setup
type Config struct {
	AuthHandler            *auth.AuthHandler
	AuthenticateMiddleware func(http.Handler) http.Handler // Function signature for auth middleware
	// Add other handlers here as needed
	// POIHandler *poi.Handler
	// UserHandler *user.Handler
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
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	// Group API routes, potentially versioning them
	r.Route("/api/v1", func(r chi.Router) {

		// --- Public Auth Routes ---
		// Routes that DO NOT require JWT authentication
		r.Group(func(r chi.Router) {
			// Example: Mount auth routes that don't need the JWT check
			r.Post("/auth/register", cfg.AuthHandler.Register) // Assuming Register handler exists
			r.Post("/auth/login", cfg.AuthHandler.Login)
			r.Post("/auth/refresh", cfg.AuthHandler.RefreshSession) // Assuming RefreshSession exists
		})

		// --- Protected Routes ---
		// Routes under this group WILL require JWT authentication
		r.Group(func(r chi.Router) {
			// Apply the authentication middleware passed from main.go
			r.Use(cfg.AuthenticateMiddleware)

			// Mount protected auth routes
			r.Post("/auth/logout", cfg.AuthHandler.Logout)
			r.Get("/auth/session/{sessionID}", cfg.AuthHandler.GetSession)                    // Needs Auth? Usually yes.
			r.Get("/auth/validate-session", cfg.AuthHandler.ValidateSession)                  // Needs Auth
			r.Post("/auth/verify-password", cfg.AuthHandler.VerifyPassword)                   // Needs Auth
			r.Put("/auth/update-password", cfg.AuthHandler.UpdatePassword)                    // Needs Auth (use PUT for update)
			r.Post("/auth/invalidate-tokens", cfg.AuthHandler.InvalidateAllUserRefreshTokens) // Needs Auth

			// Mount other protected resource routes
			// r.Mount("/users", UserRoutes(cfg.UserHandler)) // Example for user routes
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
	})

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
