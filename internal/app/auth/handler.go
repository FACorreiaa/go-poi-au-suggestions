package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
)

// Response is a generic response structure for all API endpoints
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// AuthHandler handles HTTP requests for authentication operations
type AuthHandler struct {
	authService AuthService
	logger      *slog.Logger
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(authService AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}

// RegisterRoutes registers all auth routes to the provided router
func (h *AuthHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/auth/login", h.Login).Methods("POST")
	r.HandleFunc("/auth/logout", h.Logout).Methods("POST")
	r.HandleFunc("/auth/refresh", h.RefreshToken).Methods("POST")
	r.HandleFunc("/auth/register", h.Register).Methods("POST")
	r.HandleFunc("/auth/validate", h.ValidateSession).Methods("POST")
	r.HandleFunc("/auth/password", h.ChangePassword).Methods("PUT")
	r.HandleFunc("/auth/email", h.ChangeEmail).Methods("PUT")
	r.HandleFunc("/auth/password/admin", h.AdminResetUserPassword).Methods("PUT")
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Tenant   string `json:"tenant"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents the login response body
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Message      string `json:"message"`
}

// Login handles user authentication and returns access and refresh tokens
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if req.Tenant == "" || req.Email == "" || req.Password == "" {
		h.respondWithError(w, http.StatusBadRequest, "Tenant, email, and password are required")
		return
	}

	// Call service
	accessToken, refreshToken, err := h.authService.Login(r.Context(), req.Tenant, req.Email, req.Password)
	if err != nil {
		h.logger.Error("Login failed", "error", err, "email", req.Email, "tenant", req.Tenant)
		h.respondWithError(w, http.StatusUnauthorized, "Authentication failed")
		return
	}

	// Respond with tokens
	h.respondWithJSON(w, http.StatusOK, LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Message:      "Login successful",
	})
}

// LogoutRequest represents the logout request body
type LogoutRequest struct {
	Tenant    string `json:"tenant"`
	SessionID string `json:"session_id"`
}

// Logout invalidates a user session
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if req.Tenant == "" || req.SessionID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Tenant and session ID are required")
		return
	}

	// Call service
	err := h.authService.Logout(r.Context(), req.Tenant, req.SessionID)
	if err != nil {
		h.logger.Error("Logout failed", "error", err, "tenant", req.Tenant)
		h.respondWithError(w, http.StatusInternalServerError, "Failed to logout")
		return
	}

	// Respond with success
	h.respondWithJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Logged out successfully",
	})
}

// RefreshTokenRequest represents the refresh token request body
type RefreshTokenRequest struct {
	Tenant       string `json:"tenant"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenResponse represents the refresh token response body
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshToken generates new access and refresh tokens using a valid refresh token
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if req.Tenant == "" || req.RefreshToken == "" {
		h.respondWithError(w, http.StatusBadRequest, "Tenant and refresh token are required")
		return
	}

	// Call service
	accessToken, refreshToken, err := h.authService.RefreshSession(r.Context(), req.Tenant, req.RefreshToken)
	if err != nil {
		h.logger.Error("Token refresh failed", "error", err, "tenant", req.Tenant)
		h.respondWithError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	// Respond with new tokens
	h.respondWithJSON(w, http.StatusOK, RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// RegisterRequest represents the register request body
type RegisterRequest struct {
	Tenant   string `json:"tenant"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// Register creates a new user
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if req.Tenant == "" || req.Username == "" || req.Email == "" || req.Password == "" || req.Role == "" {
		h.respondWithError(w, http.StatusBadRequest, "All fields are required")
		return
	}

	// Call service
	err := h.authService.Register(r.Context(), req.Tenant, req.Username, req.Email, req.Password, req.Role)
	if err != nil {
		h.logger.Error("Registration failed", "error", err, "email", req.Email, "tenant", req.Tenant)
		h.respondWithError(w, http.StatusInternalServerError, "Failed to register user")
		return
	}

	// Respond with success
	h.respondWithJSON(w, http.StatusCreated, Response{
		Success: true,
		Message: "User registered successfully",
	})
}

// ValidateSessionRequest represents the validate session request body
type ValidateSessionRequest struct {
	Tenant    string `json:"tenant"`
	SessionID string `json:"session_id"`
}

// ValidateSessionResponse represents the validate session response body
type ValidateSessionResponse struct {
	Valid    bool   `json:"valid"`
	UserID   string `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
}

// ValidateSession checks if a session is valid
func (h *AuthHandler) ValidateSession(w http.ResponseWriter, r *http.Request) {
	var req ValidateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if req.Tenant == "" || req.SessionID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Tenant and session ID are required")
		return
	}

	// Call service
	session, err := h.authService.GetSession(r.Context(), req.Tenant, req.SessionID)
	if err != nil {
		h.logger.Error("Session validation failed", "error", err, "tenant", req.Tenant)
		h.respondWithJSON(w, http.StatusOK, ValidateSessionResponse{
			Valid: false,
		})
		return
	}

	// Respond with session info
	h.respondWithJSON(w, http.StatusOK, ValidateSessionResponse{
		Valid:    true,
		UserID:   session.ID,
		Username: session.Username,
		Email:    session.Email,
	})
}

// ChangePasswordRequest represents the change password request body
type ChangePasswordRequest struct {
	Tenant      string `json:"tenant"`
	Username    string `json:"username"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword updates a user's password
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if req.Tenant == "" || req.Username == "" || req.OldPassword == "" || req.NewPassword == "" {
		h.respondWithError(w, http.StatusBadRequest, "All fields are required")
		return
	}

	// Get user ID from context (assuming middleware has set it)
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		h.respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Verify old password
	err := h.authService.VerifyPassword(r.Context(), req.Tenant, userID, req.OldPassword)
	if err != nil {
		h.logger.Error("Password verification failed", "error", err, "username", req.Username, "tenant", req.Tenant)
		h.respondWithError(w, http.StatusUnauthorized, "Invalid old password")
		return
	}

	// Hash new password
	// Note: In a real implementation, you might want to do this in the service layer
	hashedPassword, err := hashPassword(req.NewPassword)
	if err != nil {
		h.logger.Error("Password hashing failed", "error", err)
		h.respondWithError(w, http.StatusInternalServerError, "Failed to process new password")
		return
	}

	// Update password
	err = h.authService.UpdatePassword(r.Context(), req.Tenant, userID, hashedPassword)
	if err != nil {
		h.logger.Error("Password update failed", "error", err, "username", req.Username, "tenant", req.Tenant)
		h.respondWithError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	// Invalidate all refresh tokens
	err = h.authService.InvalidateAllUserRefreshTokens(r.Context(), req.Tenant, userID)
	if err != nil {
		h.logger.Warn("Failed to invalidate refresh tokens", "error", err, "username", req.Username, "tenant", req.Tenant)
		// Continue anyway, as the password was updated successfully
	}

	// Respond with success
	h.respondWithJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Password changed successfully",
	})
}

// ChangeEmailRequest represents the change email request body
type ChangeEmailRequest struct {
	Tenant    string `json:"tenant"`
	Password  string `json:"password"`
	NewEmail  string `json:"new_email"`
}

// ChangeEmail updates a user's email
func (h *AuthHandler) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	var req ChangeEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if req.Tenant == "" || req.Password == "" || req.NewEmail == "" {
		h.respondWithError(w, http.StatusBadRequest, "All fields are required")
		return
	}

	// Get user ID from context (assuming middleware has set it)
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		h.respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Verify password
	err := h.authService.VerifyPassword(r.Context(), req.Tenant, userID, req.Password)
	if err != nil {
		h.logger.Error("Password verification failed", "error", err, "tenant", req.Tenant)
		h.respondWithError(w, http.StatusUnauthorized, "Invalid password")
		return
	}

	// Call service to update email
	// Note: In a real implementation, you would have a dedicated method for this
	// For now, we'll assume there's an UpdateEmail method on the user repository
	// that would be called by the service
	h.respondWithError(w, http.StatusNotImplemented, "Email change not implemented")
}

// AdminResetUserPasswordRequest represents the admin reset user password request body
type AdminResetUserPasswordRequest struct {
	Tenant       string `json:"tenant"`
	TargetUserID string `json:"target_user_id"`
	NewPassword  string `json:"new_password"`
}

// AdminResetUserPassword allows an admin to reset another user's password
func (h *AuthHandler) AdminResetUserPassword(w http.ResponseWriter, r *http.Request) {
	var req AdminResetUserPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if req.Tenant == "" || req.TargetUserID == "" || req.NewPassword == "" {
		h.respondWithError(w, http.StatusBadRequest, "All fields are required")
		return
	}

	// Get admin user ID from context (assuming middleware has set it)
	adminID, ok := r.Context().Value("user_id").(string)
	if !ok || adminID == "" {
		h.respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Verify admin role
	role, err := h.authService.GetUserRole(r.Context(), req.Tenant, adminID)
	if err != nil {
		h.logger.Error("Failed to get user role", "error", err, "tenant", req.Tenant)
		h.respondWithError(w, http.StatusInternalServerError, "Failed to verify admin status")
		return
	}

	// Check if user has admin role
	isAdmin := role == "OWNER" || role == "ADMIN"
	if !isAdmin {
		h.respondWithError(w, http.StatusForbidden, "Action requires administrative privileges")
		return
	}

	// Hash new password
	hashedPassword, err := hashPassword(req.NewPassword)
	if err != nil {
		h.logger.Error("Password hashing failed", "error", err)
		h.respondWithError(w, http.StatusInternalServerError, "Failed to process new password")
		return
	}

	// Update password
	err = h.authService.UpdatePassword(r.Context(), req.Tenant, req.TargetUserID, hashedPassword)
	if err != nil {
		h.logger.Error("Password update failed", "error", err, "target_user_id", req.TargetUserID, "tenant", req.Tenant)
		h.respondWithError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	// Invalidate all refresh tokens for the target user
	err = h.authService.InvalidateAllUserRefreshTokens(r.Context(), req.Tenant, req.TargetUserID)
	if err != nil {
		h.logger.Warn("Failed to invalidate refresh tokens", "error", err, "target_user_id", req.TargetUserID, "tenant", req.Tenant)
		// Continue anyway, as the password was updated successfully
	}

	// Respond with success
	h.respondWithJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "User password reset successfully by admin",
	})
}

// Helper functions for response handling

func (h *AuthHandler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, Response{
		Success: false,
		Error:   message,
	})
}

func (h *AuthHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("Failed to marshal JSON response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err = w.Write(response)
	if err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}
}

// Helper function to hash passwords
func hashPassword(password string) (string, error) {
	// In a real implementation, you would use bcrypt or another secure hashing algorithm
	// For now, we'll return a placeholder
	return "hashed_" + password, nil
}
