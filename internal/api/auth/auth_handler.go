package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
)

type AuthHandler struct {
	authService AuthService
	logger      *slog.Logger
}

func NewAuthHandler(authService AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		logger:      logger,
		authService: authService,
	}
}

// Login godoc
// @Summary      User Login
// @Description  Authenticates a user and returns JWT access and refresh tokens.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        credentials body LoginRequest true "Login Credentials"
// @Success      200 {object} LoginResponse "Successful Login"
// @Failure      400 {object} Response "Invalid Input"
// @Failure      401 {object} Response "Authentication Failed"
// @Failure      500 {object} Response "Internal Server Error"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := h.logger.With(slog.String("handler", "Login"))

	var req api.LoginRequest
	if err := api.DecodeJSONBody(w, r, &req); err != nil {
		l.WarnContext(ctx, "Failed to decode request", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request format")
		return
	}
	if req.Email == "" || req.Password == "" {
		api.ErrorResponse(w, r, http.StatusBadRequest, "Email and password are required")
		return
	}

	accessToken, refreshToken, err := h.authService.Login(ctx, req.Email, req.Password)
	if err != nil {
		l.WarnContext(ctx, "Service login failed", slog.Any("error", err), slog.String("email", req.Email))
		if errors.Is(err, api.ErrUnauthenticated) {
			api.ErrorResponse(w, r, http.StatusUnauthorized, "Invalid email or password")
		} else {
			api.ErrorResponse(w, r, http.StatusInternalServerError, "Login failed")
		}
		return
	}

	// Set refresh token in HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Set to true if using HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()), // Match your refresh token TTL
	})

	// Respond with access token only
	resp := api.LoginResponse{
		AccessToken: accessToken,
		Message:     "Login successful",
	}
	l.InfoContext(ctx, "Login successful", slog.String("email", req.Email))
	api.WriteJSONResponse(w, r, http.StatusOK, resp)
}

// Logout godoc
// @Summary      User Logout
// @Description  Invalidates the user's current session/refresh token. Typically uses Refresh Token from HttpOnly cookie. Body might be empty or contain refresh token if not using cookies.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        token body LogoutRequest false "Logout Request (only needed if sending refresh_token in body)"
// @Success      200 {object} Response "Logout Successful"
// @Failure      400 {object} Response "Bad Request (e.g., malformed body if used)"
// @Failure      500 {object} Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := h.logger.With(slog.String("handler", "Logout"))

	// Extract refresh token from cookie
	refreshCookie, err := r.Cookie("refreshToken")
	if err != nil {
		if err == http.ErrNoCookie {
			l.WarnContext(ctx, "No refresh token cookie present for logout")
			// Still proceed to clear cookie and succeed
		} else {
			l.ErrorContext(ctx, "Error reading refresh token cookie", slog.Any("error", err))
			api.ErrorResponse(w, r, http.StatusInternalServerError, "Internal server error")
			return
		}
	}
	refreshToken := ""
	if refreshCookie != nil {
		refreshToken = refreshCookie.Value
	}

	// Invalidate refresh token if present
	if refreshToken != "" {
		err = h.authService.Logout(ctx, refreshToken)
		if err != nil {
			l.ErrorContext(ctx, "Service logout failed", slog.Any("error", err))
			// Proceed anyway, as cookie will be cleared
		}
	}

	// Clear the refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1, // Delete cookie
	})

	l.InfoContext(ctx, "Logout processed")
	api.WriteJSONResponse(w, r, http.StatusOK, api.Response{Success: true, Message: "Logged out successfully"})
}

// RefreshToken godoc
// @Summary      Refresh Access Token
// @Description  Provides a new access token using a valid refresh token (typically sent via HttpOnly cookie). Body might be empty or contain refresh token if not using cookies.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        token body RefreshTokenRequest false "Refresh Token Request (only needed if sending refresh_token in body)"
// @Success      200 {object} TokenResponse "New Access Token (Refresh Token set in cookie)"
// @Failure      400 {object} Response "Bad Request (e.g., missing token)"
// @Failure      401 {object} Response "Invalid or Expired Refresh Token"
// @Failure      500 {object} Response "Internal Server Error"
// @Router       /auth/refresh [post]
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := h.logger.With(slog.String("handler", "RefreshToken"))

	// Extract refresh token from cookie
	refreshCookie, err := r.Cookie("refreshToken")
	if err != nil {
		if err == http.ErrNoCookie {
			h.respondWithError(w, http.StatusBadRequest, "Refresh token cookie missing")
			return
		}
		l.Error("Error reading cookie", "error", err)
		h.respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	refreshToken := refreshCookie.Value

	if refreshToken == "" {
		h.respondWithError(w, http.StatusBadRequest, "Refresh token is required")
		return
	}

	// Call service
	accessToken, newRefreshToken, err := h.authService.RefreshSession(ctx, refreshToken)
	if err != nil {
		l.Error("Token refresh failed", "error", err)
		h.respondWithError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	// Set new refresh token in cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    newRefreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})

	// Respond with access token only
	h.respondWithJSON(w, http.StatusOK, api.TokenResponse{
		AccessToken: accessToken,
	})
}

// Register godoc
// @Summary      Register New User
// @Description  Creates a new user account.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        user body RegisterRequest true "User Registration Details"
// @Success      201 {object} Response "User Registered Successfully"
// @Failure      400 {object} Response "Invalid Input"
// @Failure      409 {object} Response "Email or Username already exists"
// @Failure      500 {object} Response "Internal Server Error"
// @Router       /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("RegisterHandler").Start(r.Context(), "RegisterHandler", trace.WithAttributes(
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.HTTPRouteKey.String("/register"),
	))
	defer span.End()

	l := h.logger.With(slog.String("handler", "Register"))

	// Record start time for duration metric
	//startTime := time.Now()

	var req api.RegisterRequest
	if err := api.DecodeJSONBody(w, r, &req); err != nil {
		l.WarnContext(ctx, "Failed to decode request", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request format")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request format")
		return
	}

	if req.Email == "" || req.Password == "" || req.Username == "" {
		span.SetStatus(codes.Error, "Missing required fields")
		api.ErrorResponse(w, r, http.StatusBadRequest, "Username, email, and password are required")
		return
	}

	// Call the service layer with the traced context
	err := h.authService.Register(ctx, req.Username, req.Email, req.Password, req.Role)
	if err != nil {
		l.ErrorContext(ctx, "Service registration failed", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Registration failed")
		if errors.Is(err, api.ErrConflict) {
			api.ErrorResponse(w, r, http.StatusConflict, "Email or username already exists")
		} else {
			api.ErrorResponse(w, r, http.StatusInternalServerError, "Registration failed")
		}
		return
	}

	// Record metrics
	// duration := time.Since(startTime).Seconds()
	// registerRequestsTotal.Add(ctx, 1)
	// registerDurationSeconds.Record(ctx, duration)

	l.InfoContext(ctx, "User registered successfully", slog.String("email", req.Email))
	span.SetStatus(codes.Ok, "User registered successfully")
	api.WriteJSONResponse(w, r, http.StatusCreated, api.Response{Success: true, Message: "User registered successfully"})
}

// ValidateSession checks if a session is valid
func (h *AuthHandler) ValidateSession(w http.ResponseWriter, r *http.Request) {
	var req api.ValidateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if req.SessionID == "" {
		h.respondWithError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	// Call service
	userID, err := h.authService.ValidateRefreshToken(r.Context(), req.SessionID)
	if err != nil {
		h.logger.Error("Session validation failed", "error", err)
		h.respondWithJSON(w, http.StatusOK, api.ValidateSessionResponse{
			Valid: false,
		})
		return
	}

	user, err := h.authService.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user details", "error", err)
		h.respondWithJSON(w, http.StatusOK, api.ValidateSessionResponse{
			Valid: false,
		})
		return
	}

	// Respond with session info
	h.respondWithJSON(w, http.StatusOK, api.ValidateSessionResponse{
		Valid:    true,
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
	})
}

// ChangePassword godoc
// @Summary      Change User Password
// @Description  Allows an authenticated user to change their own password.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        passwords body ChangePasswordRequest true "Old and New Passwords"
// @Success      200 {object} Response "Password Updated Successfully"
// @Failure      400 {object} Response "Invalid Input"
// @Failure      401 {object} Response "Unauthorized (Invalid old password or bad token)"
// @Failure      500 {object} Response "Internal Server Error"
// @Security     BearerAuth
// @Router       /auth/password [put]
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := h.logger.With(slog.String("handler", "ChangePassword"))

	// Get UserID from context (set by Authenticate middleware)
	userID, ok := GetUserIDFromContext(ctx) // Use actual helper
	if !ok || userID == "" {
		l.ErrorContext(ctx, "User ID not found in context for ChangePassword")
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}
	l = l.With(slog.String("userID", userID))

	var req api.ChangePasswordRequest
	if err := api.DecodeJSONBody(w, r, &req); err != nil {
		l.WarnContext(ctx, "Failed to decode request", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid request format")
		return
	}
	// Add validation for passwords
	if req.OldPassword == "" || req.NewPassword == "" {
		api.ErrorResponse(w, r, http.StatusBadRequest, "Old and new passwords are required")
		return
	}
	if req.OldPassword == req.NewPassword {
		api.ErrorResponse(w, r, http.StatusBadRequest, "New password must be different from old password")
		return
	}

	l.DebugContext(ctx, "Attempting password change")
	err := h.authService.UpdatePassword(ctx, userID, req.OldPassword, req.NewPassword)
	if err != nil {
		l.ErrorContext(ctx, "Service password update failed", slog.Any("error", err))
		if errors.Is(err, api.ErrUnauthenticated) { // Check if service returned this
			api.ErrorResponse(w, r, http.StatusUnauthorized, "Incorrect old password")
		} else {
			api.ErrorResponse(w, r, http.StatusInternalServerError, "Failed to update password")
		}
		return
	}

	l.InfoContext(ctx, "Password updated successfully")
	api.WriteJSONResponse(w, r, http.StatusOK, api.Response{Success: true, Message: "Password updated successfully"})
}

// ChangeEmail updates a user's email
func (h *AuthHandler) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	var req api.ChangeEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if req.Password == "" || req.NewEmail == "" {
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
	err := h.authService.VerifyPassword(r.Context(), userID, req.Password)
	if err != nil {
		h.logger.Error("Password verification failed", "error", err)
		h.respondWithError(w, http.StatusUnauthorized, "Invalid password")
		return
	}

	// Call service to update email
	// Note: In a real implementation, you would have a dedicated method for this
	// For now, we'll assume there's an UpdateEmail method on the user repository
	// that would be called by the service
	h.respondWithError(w, http.StatusNotImplemented, "Email change not implemented")
}

// Helper functions for response handling

func (h *AuthHandler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, api.Response{
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

func (h *AuthHandler) AuthenticateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Implement your authentication logic here
		next.ServeHTTP(w, r)
	})
}

func (h *AuthHandler) RefreshSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := h.logger.With(slog.String("handler", "RefreshSession"))

	var req api.RefreshTokenRequest
	if err := api.DecodeJSONBody(w, r, &req); err != nil || req.RefreshToken == "" {
		l.WarnContext(ctx, "Missing refresh token for refresh", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusBadRequest, "Refresh token required")
		return
	}
	refreshToken := req.RefreshToken

	l.DebugContext(ctx, "Attempting token refresh")
	newAccessToken, newRefreshToken, err := h.authService.RefreshSession(ctx, refreshToken)
	if err != nil {
		l.WarnContext(ctx, "Service token refresh failed", slog.Any("error", err))
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	// Set new refresh token in HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    newRefreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Use true for HTTPS in production
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()), // Match your refresh token TTL
	})

	resp := api.TokenResponse{
		AccessToken: newAccessToken,
	}
	l.InfoContext(ctx, "Token refresh successful")
	api.WriteJSONResponse(w, r, http.StatusOK, resp)
}
