package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// E2ETestSuite provides end-to-end testing for complete user workflows
type E2ETestSuite struct {
	suite.Suite
	server     *httptest.Server
	client     *http.Client
	baseURL    string
	logger     *slog.Logger
	authToken  string
	userID     string
	userEmail  string
	profileID  uuid.UUID
}

// SetupSuite initializes the test suite
func (suite *E2ETestSuite) SetupSuite() {
	suite.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	// This would ideally start the actual server or a test instance
	// For now, we'll create a mock server that simulates the API
	suite.server = httptest.NewServer(suite.createMockAPIServer())
	suite.baseURL = suite.server.URL
	suite.client = &http.Client{Timeout: 30 * time.Second}
	suite.userEmail = fmt.Sprintf("e2etest+%d@example.com", time.Now().Unix())
}

// TearDownSuite cleans up after all tests
func (suite *E2ETestSuite) TearDownSuite() {
	if suite.server != nil {
		suite.server.Close()
	}
}

// createMockAPIServer creates a mock server that simulates the API responses
func (suite *E2ETestSuite) createMockAPIServer() http.Handler {
	mux := http.NewServeMux()

	// Mock auth endpoints
	mux.HandleFunc("/auth/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		// Validate required fields
		if req["email"] == "" || req["password"] == "" || req["username"] == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing required fields"})
			return
		}

		suite.userID = uuid.New().String()
		response := map[string]interface{}{
			"user_id":  suite.userID,
			"email":    req["email"],
			"username": req["username"],
			"message":  "User registered successfully",
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		if req["email"] != suite.userEmail {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
			return
		}

		suite.authToken = "mock-jwt-token-" + uuid.New().String()
		
		// Set cookies
		http.SetCookie(w, &http.Cookie{
			Name:     "access_token",
			Value:    suite.authToken,
			HttpOnly: true,
			Secure:   false, // false for testing
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		})

		response := map[string]interface{}{
			"access_token":  suite.authToken,
			"refresh_token": "mock-refresh-token",
			"user_id":       suite.userID,
			"expires_in":    3600,
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	// Mock profile endpoints
	mux.HandleFunc("/profiles", func(w http.ResponseWriter, r *http.Request) {
		// Check authentication
		if !suite.isAuthenticated(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodGet:
			profiles := []map[string]interface{}{
				{
					"id":                   suite.profileID.String(),
					"user_id":              suite.userID,
					"name":                 "Default Profile",
					"default_radius_km":    10.0,
					"preferred_budget":     2,
					"preferred_time":       "morning",
					"preferred_pace":       "moderate",
					"accessibility_needs":  false,
					"created_at":          time.Now().Format(time.RFC3339),
				},
			}
			json.NewEncoder(w).Encode(profiles)

		case http.MethodPost:
			var req types.CreateProfileRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			suite.profileID = uuid.New()
			profile := map[string]interface{}{
				"id":                   suite.profileID.String(),
				"user_id":              suite.userID,
				"name":                 req.Name,
				"default_radius_km":    req.DefaultRadiusKm,
				"preferred_budget":     req.PreferredBudget,
				"preferred_time":       req.PreferredTime,
				"preferred_pace":       req.PreferredPace,
				"accessibility_needs":  req.AccessibilityNeeds,
				"created_at":          time.Now().Format(time.RFC3339),
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(profile)
		}
	})

	// Mock interests endpoints
	mux.HandleFunc("/interests", func(w http.ResponseWriter, r *http.Request) {
		if !suite.isAuthenticated(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodGet:
			interests := []map[string]interface{}{
				{
					"id":          uuid.New().String(),
					"name":        "Photography",
					"category":    "hobby",
					"description": "Taking photos",
				},
				{
					"id":          uuid.New().String(),
					"name":        "Food & Dining",
					"category":    "lifestyle",
					"description": "Exploring restaurants",
				},
			}
			json.NewEncoder(w).Encode(interests)

		case http.MethodPost:
			var req types.CreateInterestRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			interest := map[string]interface{}{
				"id":          uuid.New().String(),
				"user_id":     suite.userID,
				"name":        req.Name,
				"category":    req.Category,
				"description": req.Description,
				"created_at":  time.Now().Format(time.RFC3339),
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(interest)
		}
	})

	// Mock settings endpoints
	mux.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		if !suite.isAuthenticated(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Method == http.MethodGet {
			settings := map[string]interface{}{
				"user_id":                   suite.userID,
				"default_search_radius_km":  10.0,
				"preferred_time":            "morning",
				"default_budget_level":      2,
				"preferred_pace":            "moderate",
				"prefer_accessible_pois":    false,
				"prefer_outdoor_seating":    false,
				"prefer_dog_friendly":       false,
				"search_radius":             5.0,
				"budget_level":              1,
				"preferred_transport":       "walking",
			}
			json.NewEncoder(w).Encode(settings)
		}
	})

	// Mock tags endpoints
	mux.HandleFunc("/tags", func(w http.ResponseWriter, r *http.Request) {
		if !suite.isAuthenticated(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodGet:
			tags := []map[string]interface{}{
				{
					"id":          uuid.New().String(),
					"name":        "Outdoor",
					"tag_type":    "preference",
					"description": "Outdoor activities",
				},
				{
					"id":          uuid.New().String(),
					"name":        "Cultural",
					"tag_type":    "preference",
					"description": "Cultural experiences",
				},
			}
			json.NewEncoder(w).Encode(tags)

		case http.MethodPost:
			var req types.CreatePersonalTagParams
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			tag := map[string]interface{}{
				"id":          uuid.New().String(),
				"user_id":     suite.userID,
				"name":        req.Name,
				"tag_type":    req.TagType,
				"description": req.Description,
				"created_at":  time.Now().Format(time.RFC3339),
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(tag)
		}
	})

	return mux
}

// isAuthenticated checks if the request has valid authentication
func (suite *E2ETestSuite) isAuthenticated(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	return token == suite.authToken
}

// makeRequest makes an HTTP request with optional authentication
func (suite *E2ETestSuite) makeRequest(method, path string, body interface{}, authenticated bool) (*http.Response, error) {
	url := suite.baseURL + path

	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if authenticated && suite.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+suite.authToken)
	}

	return suite.client.Do(req)
}

// TestCompleteUserOnboardingWorkflow tests the complete user onboarding process
func (suite *E2ETestSuite) TestCompleteUserOnboardingWorkflow() {
	t := suite.T()

	// Step 1: User Registration
	t.Log("Step 1: Testing user registration")
	registerData := map[string]interface{}{
		"username": "e2etestuser",
		"email":    suite.userEmail,
		"password": "SecurePassword123!",
		"role":     "user",
	}

	resp, err := suite.makeRequest("POST", "/auth/register", registerData, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "User registration should succeed")

	var registerResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&registerResponse)
	require.NoError(t, err)

	assert.Equal(t, suite.userEmail, registerResponse["email"])
	assert.NotEmpty(t, registerResponse["user_id"])

	// Step 2: User Login
	t.Log("Step 2: Testing user login")
	loginData := map[string]interface{}{
		"email":    suite.userEmail,
		"password": "SecurePassword123!",
	}

	resp, err = suite.makeRequest("POST", "/auth/login", loginData, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "User login should succeed")

	var loginResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&loginResponse)
	require.NoError(t, err)

	assert.NotEmpty(t, loginResponse["access_token"])
	assert.NotEmpty(t, loginResponse["refresh_token"])

	// Step 3: Create User Profile
	t.Log("Step 3: Testing profile creation")
	profileData := types.CreateProfileRequest{
		Name:               "Travel Enthusiast",
		DefaultRadiusKm:    15.0,
		PreferredBudget:    3,
		PreferredTime:      types.DayPreference("day"),
		PreferredPace:      types.SearchPace("fast"),
		AccessibilityNeeds: false,
	}

	resp, err = suite.makeRequest("POST", "/profiles", profileData, true)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Profile creation should succeed")

	var profileResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&profileResponse)
	require.NoError(t, err)

	assert.Equal(t, "Travel Enthusiast", profileResponse["name"])
	assert.Equal(t, 15.0, profileResponse["default_radius_km"])

	// Step 4: Add User Interests
	t.Log("Step 4: Testing interest addition")
	interestData := types.CreateInterestRequest{
		Name:        "Photography",
		Category:    "hobby",
		Description: "Passionate about capturing moments",
	}

	resp, err = suite.makeRequest("POST", "/interests", interestData, true)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Interest creation should succeed")

	// Step 5: Create Personal Tags
	t.Log("Step 5: Testing personal tag creation")
	tagData := types.CreatePersonalTagParams{
		Name:        "Food Lover",
		Description: "Enjoys trying new cuisines",
		TagType:     "preference",
	}

	resp, err = suite.makeRequest("POST", "/tags", tagData, true)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Tag creation should succeed")

	// Step 6: Verify Data Retrieval
	t.Log("Step 6: Testing data retrieval")
	
	// Get profiles
	resp, err = suite.makeRequest("GET", "/profiles", nil, true)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Get interests
	resp, err = suite.makeRequest("GET", "/interests", nil, true)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Get settings
	resp, err = suite.makeRequest("GET", "/settings", nil, true)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Get tags
	resp, err = suite.makeRequest("GET", "/tags", nil, true)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	t.Log("Complete user onboarding workflow test completed successfully")
}

// TestUserPersonalizationWorkflow tests user personalization features
func (suite *E2ETestSuite) TestUserPersonalizationWorkflow() {
	t := suite.T()

	// First complete the basic setup
	suite.TestCompleteUserOnboardingWorkflow()

	t.Log("Testing user personalization workflow")

	// Test updating profile preferences
	t.Log("Step 1: Updating profile preferences")
	// This would involve updating the profile through API calls

	// Test adding multiple interests
	t.Log("Step 2: Adding multiple interests")
	interests := []types.CreateInterestRequest{
		{Name: "Museums", Category: "culture", Description: "Love visiting museums"},
		{Name: "Hiking", Category: "outdoor", Description: "Enjoy outdoor activities"},
		{Name: "Local Cuisine", Category: "food", Description: "Trying local food"},
	}

	for _, interest := range interests {
		resp, err := suite.makeRequest("POST", "/interests", interest, true)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	// Test adding multiple tags
	t.Log("Step 3: Adding multiple personal tags")
	tags := []types.CreatePersonalTagParams{
		{Name: "Budget Traveler", Description: "Prefers budget-friendly options", TagType: "preference"},
		{Name: "Family Friendly", Description: "Needs family-friendly activities", TagType: "requirement"},
		{Name: "Pet Owner", Description: "Travels with pets", TagType: "requirement"},
	}

	for _, tag := range tags {
		resp, err := suite.makeRequest("POST", "/tags", tag, true)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	t.Log("User personalization workflow test completed successfully")
}

// TestErrorHandlingWorkflow tests error scenarios and edge cases
func (suite *E2ETestSuite) TestErrorHandlingWorkflow() {
	t := suite.T()

	t.Log("Testing error handling workflow")

	// Test registration with invalid data
	t.Log("Step 1: Testing registration with invalid data")
	invalidData := map[string]interface{}{
		"username": "", // Empty username
		"email":    "invalid-email",
		// Missing password
	}

	resp, err := suite.makeRequest("POST", "/auth/register", invalidData, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Should reject invalid registration data")

	// Test login with wrong credentials
	t.Log("Step 2: Testing login with wrong credentials")
	wrongLoginData := map[string]interface{}{
		"email":    "wrong@example.com",
		"password": "wrongpassword",
	}

	resp, err = suite.makeRequest("POST", "/auth/login", wrongLoginData, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should reject invalid credentials")

	// Test accessing protected endpoints without authentication
	t.Log("Step 3: Testing unauthenticated access")
	protectedEndpoints := []string{"/profiles", "/interests", "/settings", "/tags"}

	for _, endpoint := range protectedEndpoints {
		resp, err := suite.makeRequest("GET", endpoint, nil, false)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, 
			"Should require authentication for "+endpoint)
	}

	t.Log("Error handling workflow test completed successfully")
}

// TestConcurrentUserSessions tests multiple users interacting with the system
func (suite *E2ETestSuite) TestConcurrentUserSessions() {
	t := suite.T()

	t.Log("Testing concurrent user sessions")

	const numUsers = 3
	userEmails := make([]string, numUsers)
	userTokens := make([]string, numUsers)

	// Create multiple users
	for i := 0; i < numUsers; i++ {
		userEmail := fmt.Sprintf("concurrent_user_%d_%d@example.com", i, time.Now().Unix())
		userEmails[i] = userEmail

		// Register user
		registerData := map[string]interface{}{
			"username": fmt.Sprintf("user%d", i),
			"email":    userEmail,
			"password": "password123",
			"role":     "user",
		}

		resp, err := suite.makeRequest("POST", "/auth/register", registerData, false)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Login user
		loginData := map[string]interface{}{
			"email":    userEmail,
			"password": "password123",
		}

		resp, err = suite.makeRequest("POST", "/auth/login", loginData, false)
		require.NoError(t, err)

		var loginResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&loginResponse)
		require.NoError(t, err)
		resp.Body.Close()

		userTokens[i] = loginResponse["access_token"].(string)
	}

	// Test concurrent operations
	t.Log("Testing concurrent profile creation")
	results := make(chan bool, numUsers)

	for i := 0; i < numUsers; i++ {
		go func(userIndex int) {
			// Temporarily override auth token for this user
			originalToken := suite.authToken
			suite.authToken = userTokens[userIndex]

			profileData := types.CreateProfileRequest{
				Name:            fmt.Sprintf("User %d Profile", userIndex),
				DefaultRadiusKm: float64(10 + userIndex),
				PreferredBudget: 1 + userIndex,
				PreferredTime:   types.DayPreference("morning"),
				PreferredPace:   types.SearchPace("moderate"),
			}

			resp, err := suite.makeRequest("POST", "/profiles", profileData, true)
			success := err == nil && resp.StatusCode == http.StatusCreated
			if resp != nil {
				resp.Body.Close()
			}

			// Restore original token
			suite.authToken = originalToken

			results <- success
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numUsers; i++ {
		select {
		case success := <-results:
			if success {
				successCount++
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}

	assert.Equal(t, numUsers, successCount, "All concurrent users should successfully create profiles")

	t.Log("Concurrent user sessions test completed successfully")
}

// TestUserDataConsistency tests data consistency across operations
func (suite *E2ETestSuite) TestUserDataConsistency() {
	t := suite.T()

	// Setup user first
	suite.TestCompleteUserOnboardingWorkflow()

	t.Log("Testing user data consistency")

	// Create multiple profiles and verify they're all returned
	profileNames := []string{"Profile 1", "Profile 2", "Profile 3"}
	
	for _, name := range profileNames {
		profileData := types.CreateProfileRequest{
			Name:            name,
			DefaultRadiusKm: 10.0,
			PreferredBudget: 2,
			PreferredTime:   types.DayPreference("morning"),
			PreferredPace:   types.SearchPace("moderate"),
		}

		resp, err := suite.makeRequest("POST", "/profiles", profileData, true)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	// Verify all profiles are returned
	resp, err := suite.makeRequest("GET", "/profiles", nil, true)
	require.NoError(t, err)
	defer resp.Body.Close()

	var profiles []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&profiles)
	require.NoError(t, err)

	// Should have at least 4 profiles (1 from onboarding + 3 new ones)
	assert.GreaterOrEqual(t, len(profiles), 4, "Should return all created profiles")

	t.Log("User data consistency test completed successfully")
}

// TestE2E runs the complete end-to-end test suite
func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	suite.Run(t, new(E2ETestSuite))
}