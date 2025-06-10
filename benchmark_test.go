package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/interests"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/profiles"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/settings"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/tags"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/user"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/router"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// BenchmarkSuite provides benchmark testing for the API
type BenchmarkSuite struct {
	router    *chi.Mux
	logger    *slog.Logger
	authToken string
	userID    string
}

// setupBenchmarkSuite initializes the benchmark test suite
func setupBenchmarkSuite() *BenchmarkSuite {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	
	// Create optimized mock services for benchmarking
	authService := &auth.MockAuthService{}
	userService := &user.MockUserService{}
	interestsService := &interests.MockinterestsService{}
	profileService := &profiles.MockProfileService{}
	settingsService := &settings.MockSettingsService{}
	tagsService := &tags.MocktagsService{}

	// Setup fast mock responses
	testUserID := uuid.New().String()
	testToken := "benchmark-jwt-token"

	// Setup mock expectations for minimal overhead
	authService.On("Register", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&types.UserAuth{ID: testUserID}, nil)
	
	authService.On("Login", mock.Anything, mock.Anything, mock.Anything).
		Return(&types.LoginResponse{
			AccessToken:  testToken,
			RefreshToken: "refresh-token",
			User:         &types.UserAuth{ID: testUserID},
		}, nil)

	// Setup profile service mocks
	profileService.On("CreateProfile", mock.Anything, mock.Anything, mock.Anything).
		Return(&types.UserProfile{ID: uuid.New(), UserID: uuid.MustParse(testUserID)}, nil)
	
	profileService.On("GetProfiles", mock.Anything, mock.Anything).
		Return([]*types.UserProfile{}, nil)

	// Setup interests service mocks
	interestsService.On("CreateInterest", mock.Anything, mock.Anything, mock.Anything).
		Return(&types.Interest{ID: uuid.New()}, nil)
	
	interestsService.On("GetInterests", mock.Anything, mock.Anything).
		Return([]*types.Interest{}, nil)

	// Create handlers
	authHandler := auth.NewAuthHandlerImpl(authService, logger)
	userHandler := user.NewUserHandlerImpl(userService, logger)
	interestsHandler := interests.NewinterestsHandlerImpl(interestsService, logger)
	profileHandler := profiles.NewProfileHandlerImpl(profileService, logger)
	settingsHandler := settings.NewSettingsHandlerImpl(settingsService, logger)
	tagsHandler := tags.NewtagsHandlerImpl(tagsService, logger)

	// Create router
	r := router.NewRouter(
		authHandler,
		userHandler,
		interestsHandler,
		profileHandler,
		settingsHandler,
		tagsHandler,
		nil, // poi handler
		nil, // chat handler
		nil, // city handler
		nil, // admin handler
		nil, // list handler
		logger,
	)

	return &BenchmarkSuite{
		router:    r,
		logger:    logger,
		authToken: testToken,
		userID:    testUserID,
	}
}

// makeAuthenticatedRequest helper for benchmark tests
func (suite *BenchmarkSuite) makeAuthenticatedRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+suite.authToken)
	
	// Add user context for auth middleware
	ctx := context.WithValue(req.Context(), "userID", suite.userID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	
	return w
}

// BenchmarkUserRegistration benchmarks user registration endpoint
func BenchmarkUserRegistration(b *testing.B) {
	suite := setupBenchmarkSuite()
	
	registerData := map[string]interface{}{
		"username": "benchmarkuser",
		"email":    "benchmark@example.com",
		"password": "password123",
		"role":     "user",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		registerData["email"] = "benchmark" + string(rune(i)) + "@example.com"
		
		jsonBody, _ := json.Marshal(registerData)
		req := httptest.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
	}
}

// BenchmarkUserLogin benchmarks user login endpoint
func BenchmarkUserLogin(b *testing.B) {
	suite := setupBenchmarkSuite()
	
	loginData := map[string]interface{}{
		"email":    "benchmark@example.com",
		"password": "password123",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		jsonBody, _ := json.Marshal(loginData)
		req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
	}
}

// BenchmarkProfileCreation benchmarks profile creation endpoint
func BenchmarkProfileCreation(b *testing.B) {
	suite := setupBenchmarkSuite()
	
	profileData := types.CreateProfileRequest{
		Name:               "Benchmark Profile",
		DefaultRadiusKm:    10.0,
		PreferredBudget:    2,
		PreferredTime:      types.DayPreference("morning"),
		PreferredPace:      types.SearchPace("moderate"),
		AccessibilityNeeds: false,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		suite.makeAuthenticatedRequest("POST", "/profiles", profileData)
	}
}

// BenchmarkProfileRetrieval benchmarks profile retrieval endpoint
func BenchmarkProfileRetrieval(b *testing.B) {
	suite := setupBenchmarkSuite()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		suite.makeAuthenticatedRequest("GET", "/profiles", nil)
	}
}

// BenchmarkInterestCreation benchmarks interest creation endpoint
func BenchmarkInterestCreation(b *testing.B) {
	suite := setupBenchmarkSuite()
	
	interestData := types.CreateInterestRequest{
		Name:        "Photography",
		Category:    "hobby",
		Description: "Love taking photos",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		suite.makeAuthenticatedRequest("POST", "/interests", interestData)
	}
}

// BenchmarkInterestRetrieval benchmarks interest retrieval endpoint
func BenchmarkInterestRetrieval(b *testing.B) {
	suite := setupBenchmarkSuite()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		suite.makeAuthenticatedRequest("GET", "/interests", nil)
	}
}

// BenchmarkTagCreation benchmarks tag creation endpoint
func BenchmarkTagCreation(b *testing.B) {
	suite := setupBenchmarkSuite()
	
	tagData := types.CreatePersonalTagParams{
		Name:        "Spicy Food",
		Description: "Love spicy food",
		TagType:     "preference",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		suite.makeAuthenticatedRequest("POST", "/tags", tagData)
	}
}

// BenchmarkConcurrentRequests benchmarks concurrent requests handling
func BenchmarkConcurrentRequests(b *testing.B) {
	suite := setupBenchmarkSuite()

	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			suite.makeAuthenticatedRequest("GET", "/profiles", nil)
		}
	})
}

// BenchmarkJSONSerialization benchmarks JSON serialization/deserialization
func BenchmarkJSONSerialization(b *testing.B) {
	profileData := types.CreateProfileRequest{
		Name:               "Test Profile",
		DefaultRadiusKm:    10.0,
		PreferredBudget:    2,
		PreferredTime:      types.DayPreference("morning"),
		PreferredPace:      types.SearchPace("moderate"),
		AccessibilityNeeds: false,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Serialize
		data, _ := json.Marshal(profileData)
		
		// Deserialize
		var result types.CreateProfileRequest
		json.Unmarshal(data, &result)
	}
}

// BenchmarkRequestRouting benchmarks the router performance
func BenchmarkRequestRouting(b *testing.B) {
	suite := setupBenchmarkSuite()

	routes := []string{
		"/auth/login",
		"/profiles",
		"/interests",
		"/settings",
		"/tags",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		route := routes[i%len(routes)]
		req := httptest.NewRequest("GET", route, nil)
		req.Header.Set("Authorization", "Bearer "+suite.authToken)
		
		ctx := context.WithValue(req.Context(), "userID", suite.userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
	}
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	suite := setupBenchmarkSuite()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate memory-intensive operation
		profiles := make([]*types.UserProfile, 100)
		for j := 0; j < 100; j++ {
			profiles[j] = &types.UserProfile{
				ID:              uuid.New(),
				UserID:          uuid.New(),
				Name:            "Profile " + string(rune(j)),
				DefaultRadiusKm: 10.0,
				PreferredBudget: 2,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}
		}
		
		// Serialize to JSON
		_, _ = json.Marshal(profiles)
	}
}

// BenchmarkUUIDGeneration benchmarks UUID generation performance
func BenchmarkUUIDGeneration(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = uuid.New()
	}
}

// BenchmarkTimeOperations benchmarks time operations
func BenchmarkTimeOperations(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		now := time.Now()
		_ = now.Format(time.RFC3339)
		_ = now.Add(1 * time.Hour)
		_ = now.Unix()
	}
}

// BenchmarkCompleteWorkflow benchmarks a complete user workflow
func BenchmarkCompleteWorkflow(b *testing.B) {
	suite := setupBenchmarkSuite()

	// Prepare test data
	registerData := map[string]interface{}{
		"username": "workflowuser",
		"email":    "workflow@example.com",
		"password": "password123",
		"role":     "user",
	}

	loginData := map[string]interface{}{
		"email":    "workflow@example.com",
		"password": "password123",
	}

	profileData := types.CreateProfileRequest{
		Name:               "Workflow Profile",
		DefaultRadiusKm:    10.0,
		PreferredBudget:    2,
		PreferredTime:      types.DayPreference("morning"),
		PreferredPace:      types.SearchPace("moderate"),
		AccessibilityNeeds: false,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Register user
		registerData["email"] = "workflow" + string(rune(i)) + "@example.com"
		jsonBody, _ := json.Marshal(registerData)
		req := httptest.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		// Login user  
		jsonBody, _ = json.Marshal(loginData)
		req = httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		// Create profile
		suite.makeAuthenticatedRequest("POST", "/profiles", profileData)

		// Get profiles
		suite.makeAuthenticatedRequest("GET", "/profiles", nil)
	}
}

// BenchmarkLargeJSONPayload benchmarks handling of large JSON payloads
func BenchmarkLargeJSONPayload(b *testing.B) {
	suite := setupBenchmarkSuite()

	// Create a large payload
	largeProfile := types.CreateProfileRequest{
		Name:               "Large Profile with very long name that contains lots of text and information about the user's preferences and details",
		DefaultRadiusKm:    10.0,
		PreferredBudget:    2,
		PreferredTime:      types.DayPreference("morning"),
		PreferredPace:      types.SearchPace("moderate"),
		AccessibilityNeeds: false,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		suite.makeAuthenticatedRequest("POST", "/profiles", largeProfile)
	}
}