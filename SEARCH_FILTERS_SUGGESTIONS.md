# Enhanced Search Filters and Parameters

## Current Profile Structure Analysis
Your current profile includes basic filters like radius, time preferences, budget, pace, accessibility, and transport preferences. Here are comprehensive additions to create a robust search system:

## Travel & Accommodation Filters

### Hotels/Hostels/Accommodations
- `accommodation_type`: ["hotel", "hostel", "apartment", "guesthouse", "resort", "boutique"]
- `star_rating`: 1-5 stars
- `price_range_per_night`: {"min": 0, "max": 1000}
- `amenities`: ["wifi", "parking", "pool", "gym", "spa", "breakfast", "pet_friendly", "business_center", "concierge"]
- `room_type`: ["single", "double", "suite", "dorm", "private_bathroom", "shared_bathroom"]
- `chain_preference`: ["independent", "major_chains", "boutique_chains"]
- `cancellation_policy`: ["free_cancellation", "partial_refund", "non_refundable"]
- `booking_flexibility`: ["instant_book", "request_only"]

### Location & Geography
- `neighborhood_types`: ["city_center", "historic_district", "business_district", "residential", "waterfront", "mountainous", "suburban"]
- `proximity_to_landmarks`: {"landmark_name": "distance_km"}
- `elevation_preference`: ["sea_level", "moderate_altitude", "high_altitude"]
- `climate_preference`: ["tropical", "temperate", "arid", "continental", "mediterranean"]
- `safety_level_required`: 1-5 scale
- `noise_tolerance`: ["quiet", "moderate", "lively", "any"]

## Restaurant & Dining Filters

### Cuisine & Food
- `cuisine_types`: ["italian", "asian", "mediterranean", "mexican", "indian", "french", "american", "local_specialty"]
- `meal_types`: ["breakfast", "brunch", "lunch", "dinner", "late_night", "snacks"]
- `service_style`: ["fine_dining", "casual", "fast_casual", "street_food", "buffet", "takeaway"]
- `price_range_per_person`: {"min": 0, "max": 200}
- `michelin_rated`: boolean
- `local_recommendations`: boolean
- `chain_vs_local`: ["local_only", "chains_ok", "chains_preferred"]

### Dietary & Health
- `allergen_free`: ["gluten", "nuts", "dairy", "shellfish", "soy"]
- `organic_preference`: boolean
- `locally_sourced`: boolean
- `sustainable_practices`: boolean

## Activity & Experience Filters

### Itinerary Planning
- `trip_duration`: {"min_days": 1, "max_days": 30}
- `group_size`: {"adults": 1, "children": 0, "infants": 0}
- `group_type`: ["solo", "couple", "family", "friends", "business", "large_group"]
- `physical_activity_level`: ["low", "moderate", "high", "extreme"]
- `cultural_immersion_level`: ["tourist", "moderate", "deep_local"]
- `planning_style`: ["structured", "flexible", "spontaneous"]
- `must_see_vs_hidden_gems`: ["popular_attractions", "off_beaten_path", "mixed"]

### Activities & Interests
- `activity_categories`: ["museums", "nightlife", "shopping", "nature", "sports", "arts", "history", "food_tours", "adventure"]
- `indoor_outdoor_preference`: ["indoor", "outdoor", "mixed", "weather_dependent"]
- `season_specific_activities`: ["summer_only", "winter_sports", "year_round"]
- `educational_preference`: boolean
- `photography_opportunities`: boolean
- `instagram_worthy_spots`: boolean

## Temporal & Scheduling

### Timing Preferences
- `preferred_seasons`: ["spring", "summer", "fall", "winter"]
- `avoid_peak_season`: boolean
- `weekend_vs_weekday`: ["weekends", "weekdays", "any"]
- `morning_person_vs_night_owl`: ["early_bird", "night_owl", "flexible"]
- `time_flexibility`: ["strict_schedule", "loose_schedule", "completely_flexible"]

### Event-Based
- `local_events_interest`: ["festivals", "concerts", "sports", "cultural_events", "food_events"]
- `avoid_crowds`: boolean
- `holiday_preference`: ["during_holidays", "avoid_holidays", "neutral"]

## Social & Personal Preferences

### Social Aspects
- `social_interaction_level`: ["minimal", "moderate", "high_interaction"]
- `language_requirements`: ["english_speaking", "local_language_immersion", "translator_needed"]
- `solo_female_friendly`: boolean (for safety considerations)
- `lgbtq_friendly`: boolean
- `family_friendly_required`: boolean

### Personal Style
- `luxury_vs_budget`: ["luxury", "mid_range", "budget", "backpacker"]
- `authenticity_vs_comfort`: ["authentic_local", "comfortable_familiar", "balanced"]
- `adventure_vs_relaxation`: ["adventure_focused", "relaxation_focused", "balanced"]
- `spontaneous_vs_planned`: ["highly_planned", "semi_planned", "spontaneous"]

## Technical & Accessibility

### Accessibility (Enhanced)
- `mobility_assistance_needed`: ["wheelchair", "walking_aid", "limited_mobility"]
- `sensory_accommodations`: ["hearing_impaired", "vision_impaired", "sensory_sensitive"]
- `cognitive_accessibility`: boolean
- `service_animal_friendly`: boolean

### Technology
- `wifi_importance`: ["essential", "preferred", "not_important"]
- `charging_stations_needed`: boolean
- `mobile_payment_preferred`: boolean
- `english_menus_required`: boolean

## Weather & Environmental

### Weather Sensitivity
- `weather_flexibility`: ["any_weather", "good_weather_only", "seasonal_preferences"]
- `temperature_range`: {"min_celsius": -10, "max_celsius": 40}
- `rain_tolerance`: ["rain_ok", "covered_activities_only", "sunny_days_preferred"]
- `humidity_tolerance`: ["low", "moderate", "high", "any"]

## Budget & Financial

### Enhanced Budget Controls
- `total_trip_budget`: {"min": 0, "max": 10000}
- `daily_spending_limit`: {"accommodation": 100, "food": 50, "activities": 75, "transport": 25}
- `currency_preference`: string
- `payment_methods_accepted`: ["cash", "card", "mobile_payment", "crypto"]
- `tipping_culture_comfort`: boolean
- `bargaining_comfort`: boolean

## Filter Separation Strategy

### Core Architecture: Multi-Domain Filter System

#### 1. Shared/Global Filters (Base Profile)
These filters apply across all domains and should remain in the main user profile:
```json
{
  "user_id": "uuid",
  "search_radius_km": 5,
  "preferred_time": "any",
  "budget_level": 0-5,
  "preferred_transport": "public",
  "prefer_accessible_pois": false,
  "safety_level_required": 1-5,
  "language_requirements": ["english_speaking"],
  "group_size": {"adults": 1, "children": 0},
  "trip_duration": {"min_days": 1, "max_days": 30}
}
```

#### 2. Domain-Specific Filter Tables

##### A. Accommodation Filters (`user_accommodation_preferences`)
```json
{
  "user_id": "uuid",
  "accommodation_type": ["hotel", "hostel", "apartment"],
  "star_rating": {"min": 1, "max": 5},
  "price_range_per_night": {"min": 0, "max": 1000},
  "amenities": ["wifi", "parking", "pool", "gym"],
  "room_type": ["single", "double", "suite"],
  "chain_preference": "independent",
  "cancellation_policy": ["free_cancellation"],
  "booking_flexibility": "instant_book"
}
```

##### B. Dining Filters (`user_dining_preferences`)
```json
{
  "user_id": "uuid",
  "cuisine_types": ["italian", "asian", "local_specialty"],
  "meal_types": ["breakfast", "lunch", "dinner"],
  "service_style": ["fine_dining", "casual", "street_food"],
  "price_range_per_person": {"min": 0, "max": 200},
  "dietary_needs": ["vegetarian", "gluten_free"],
  "allergen_free": ["nuts", "dairy"],
  "michelin_rated": false,
  "local_recommendations": true,
  "chain_vs_local": "local_preferred",
  "organic_preference": false,
  "outdoor_seating_preferred": true
}
```

##### C. Activity/POI Filters (`user_activity_preferences`)
```json
{
  "user_id": "uuid",
  "activity_categories": ["museums", "nature", "nightlife"],
  "physical_activity_level": "moderate",
  "indoor_outdoor_preference": "mixed",
  "cultural_immersion_level": "moderate",
  "must_see_vs_hidden_gems": "mixed",
  "educational_preference": true,
  "photography_opportunities": true,
  "season_specific_activities": ["year_round"],
  "avoid_crowds": false,
  "local_events_interest": ["festivals", "cultural_events"]
}
```

##### D. Itinerary Filters (`user_itinerary_preferences`)
```json
{
  "user_id": "uuid",
  "planning_style": "flexible",
  "preferred_pace": "moderate",
  "time_flexibility": "loose_schedule",
  "morning_vs_evening": "flexible",
  "weekend_vs_weekday": "any",
  "preferred_seasons": ["spring", "summer"],
  "avoid_peak_season": false,
  "adventure_vs_relaxation": "balanced",
  "spontaneous_vs_planned": "semi_planned"
}
```

### Database Implementation Strategy

#### Option 1: Separate Tables (Recommended)
```sql
-- Base user preferences (shared filters)
CREATE TABLE user_preferences (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    profile_name VARCHAR(255),
    is_default BOOLEAN,
    search_radius_km INTEGER,
    preferred_transport TEXT,
    budget_level INTEGER,
    safety_level_required INTEGER,
    accessibility_needs JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- Domain-specific preference tables
CREATE TABLE user_accommodation_preferences (
    user_preference_id UUID REFERENCES user_preferences(id),
    accommodation_filters JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE TABLE user_dining_preferences (
    user_preference_id UUID REFERENCES user_preferences(id),
    dining_filters JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE TABLE user_activity_preferences (
    user_preference_id UUID REFERENCES user_preferences(id),
    activity_filters JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE TABLE user_itinerary_preferences (
    user_preference_id UUID REFERENCES user_preferences(id),
    itinerary_filters JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

#### Option 2: Single Table with JSONB (Alternative)
```sql
CREATE TABLE user_preferences_v2 (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    profile_name VARCHAR(255),
    is_default BOOLEAN,
    
    -- Shared filters
    shared_filters JSONB,
    
    -- Domain-specific filters
    accommodation_filters JSONB,
    dining_filters JSONB,
    activity_filters JSONB,
    itinerary_filters JSONB,
    
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

### API Endpoint Strategy

#### Domain-Specific Endpoints
```
GET    /api/users/{userId}/preferences/accommodation
POST   /api/users/{userId}/preferences/accommodation
PUT    /api/users/{userId}/preferences/accommodation
DELETE /api/users/{userId}/preferences/accommodation

GET    /api/users/{userId}/preferences/dining
POST   /api/users/{userId}/preferences/dining
PUT    /api/users/{userId}/preferences/dining
DELETE /api/users/{userId}/preferences/dining

GET    /api/users/{userId}/preferences/activities
POST   /api/users/{userId}/preferences/activities
PUT    /api/users/{userId}/preferences/activities
DELETE /api/users/{userId}/preferences/activities

GET    /api/users/{userId}/preferences/itinerary
POST   /api/users/{userId}/preferences/itinerary
PUT    /api/users/{userId}/preferences/itinerary
DELETE /api/users/{userId}/preferences/itinerary
```

#### Composite Endpoints for Search
```
POST /api/search/accommodations
POST /api/search/restaurants
POST /api/search/activities
POST /api/search/itineraries

-- With combined filters
POST /api/search/comprehensive
```

### Filter Application Logic

#### Search Request Processing
1. **Retrieve Base Preferences**: Get shared filters from main profile
2. **Retrieve Domain Preferences**: Get specific filters based on search type
3. **Merge Filters**: Combine shared + domain-specific filters
4. **Apply Defaults**: Use system defaults for missing preferences
5. **Execute Search**: Apply merged filter set to search algorithm

#### Filter Hierarchy (Override Logic)
```
Domain-Specific > Profile-Specific > User Default > System Default
```

### Code Structure Recommendations

#### Service Layer Organization
```go
// Shared preference service
type PreferenceService struct {
    baseRepo BasePreferenceRepository
    accommRepo AccommodationPreferenceRepository
    diningRepo DiningPreferenceRepository
    activityRepo ActivityPreferenceRepository
    itineraryRepo ItineraryPreferenceRepository
}

// Domain-specific methods
func (s *PreferenceService) GetAccommodationPreferences(userID, profileID string) (*AccommodationPreferences, error)
func (s *PreferenceService) GetDiningPreferences(userID, profileID string) (*DiningPreferences, error)
func (s *PreferenceService) GetActivityPreferences(userID, profileID string) (*ActivityPreferences, error)
func (s *PreferenceService) GetItineraryPreferences(userID, profileID string) (*ItineraryPreferences, error)

// Composite methods
func (s *PreferenceService) GetSearchPreferences(userID, profileID, domain string) (*CombinedPreferences, error)
```

### Benefits of This Approach

1. **Separation of Concerns**: Each domain has its own filter logic
2. **Scalability**: Easy to add new domains (shopping, transportation, etc.)
3. **Performance**: Query only relevant filters for specific searches
4. **Maintenance**: Domain experts can manage their specific filters
5. **Flexibility**: Users can have different preferences per domain
6. **API Clarity**: Clear endpoints for each use case

### Migration Strategy

1. **Phase 1**: Keep existing structure, add new domain tables
2. **Phase 2**: Migrate existing filters to appropriate domains
3. **Phase 3**: Update API endpoints to use new structure
4. **Phase 4**: Remove deprecated fields from original table

## Implementation Recommendations

### Priority Levels
1. **High Priority**: accommodation_type, cuisine_types, group_size, trip_duration, activity_categories
2. **Medium Priority**: price_ranges, amenities, neighborhood_types, physical_activity_level
3. **Low Priority**: instagram_worthy_spots, cryptocurrency_payment, extreme_weather_tolerance

### Database Considerations
- Use JSONB fields for array-type preferences
- Consider separate tables for complex nested objects
- Implement proper indexing for frequently queried fields
- Use enums for constrained value sets

### API Design
- Allow partial filtering (not all fields required)
- Implement filter combination logic with weights
- Support "OR" and "AND" operations between filter groups
- Provide filter suggestion endpoints based on location/season

## AI Chat Integration Strategy

### Current Chat System Analysis

The existing `chat_prompt` package implements a sophisticated conversational AI system with:

1. **Intent Classification**: Already classifies user intents like `IntentFindHotels`, `IntentFindRestaurants`, `IntentAddPOI`
2. **Context Management**: Maintains conversation history and session state
3. **RAG Implementation**: Uses semantic search with PostGIS for location-aware recommendations
4. **Streaming Support**: Real-time responses via Server-Sent Events
5. **Multi-domain Handlers**: Separate handlers for POIs, hotels, restaurants

### Single-Prompt Multi-Domain Strategy

#### 1. Enhanced Intent Classification with Domain Detection

```go
type EnhancedIntentClassifier struct {
    // Add domain detection alongside intent classification
}

type ClassificationResult struct {
    Intent     IntentType     `json:"intent"`
    Domain     DomainType     `json:"domain"`
    Entities   []Entity       `json:"entities"`
    Confidence float64        `json:"confidence"`
}

const (
    DomainGeneral        DomainType = "general"
    DomainAccommodation  DomainType = "accommodation"  
    DomainDining         DomainType = "dining"
    DomainActivities     DomainType = "activities"
    DomainItinerary      DomainType = "itinerary"
    DomainTransport      DomainType = "transport"
)
```

#### 2. Context-Aware Filter Resolution

```go
// Enhanced session context with active filters
type EnhancedSessionContext struct {
    // Existing fields...
    ActiveFilters        map[DomainType]DomainFilters `json:"active_filters"`
    FilterHistory        []FilterChange               `json:"filter_history"`
    LastDomainContext    DomainType                   `json:"last_domain_context"`
    InferredPreferences  map[string]interface{}       `json:"inferred_preferences"`
}

// Dynamic filter resolution based on conversation
func (s *LlmInteractiontServiceImpl) ResolveFiltersFromContext(
    ctx context.Context,
    message string,
    sessionID uuid.UUID,
    userID uuid.UUID,
    domain DomainType,
) (*CombinedFilters, error) {
    // 1. Get base user preferences
    basePrefs := s.getBaseUserPreferences(userID)
    
    // 2. Get domain-specific preferences  
    domainPrefs := s.getDomainPreferences(userID, domain)
    
    // 3. Extract filters from natural language
    inferredFilters := s.extractFiltersFromMessage(message, domain)
    
    // 4. Apply conversation context
    contextFilters := s.getContextualFilters(sessionID, domain)
    
    // 5. Merge with hierarchy: Inferred > Context > Domain > Base
    return s.mergeFilters(inferredFilters, contextFilters, domainPrefs, basePrefs)
}
```

#### 3. Natural Language Filter Extraction

```go
type FilterExtractor struct {
    nlpProcessor    NLPProcessor
    entityRecognizer EntityRecognizer
}

// Examples of natural language to filter extraction:
// "Find a quiet Italian restaurant under $50 near downtown"
// → DomainDining + {cuisine: "italian", price_max: 50, noise_level: "quiet", location: "downtown"}

// "Show me family-friendly hotels with pools in Paris"  
// → DomainAccommodation + {family_friendly: true, amenities: ["pool"], location: "Paris"}

func (fe *FilterExtractor) ExtractFilters(message string, domain DomainType) (*DomainFilters, error) {
    entities := fe.entityRecognizer.Extract(message)
    
    switch domain {
    case DomainDining:
        return fe.extractDiningFilters(entities, message)
    case DomainAccommodation:
        return fe.extractAccommodationFilters(entities, message)
    case DomainActivities:
        return fe.extractActivityFilters(entities, message)
    default:
        return fe.extractGeneralFilters(entities, message)
    }
}
```

### RAG Enhancement Strategy

#### 1. Filter-Aware Vector Search

```go
// Enhanced semantic search with filter pre-filtering
func (s *LlmInteractiontServiceImpl) EnhancedSemanticSearch(
    ctx context.Context,
    query string,
    filters *CombinedFilters,
    location *types.UserLocation,
    limit int,
) ([]types.EnhancedPOI, error) {
    
    // 1. Pre-filter database using structured filters
    preFilteredPOIs := s.applyStructuredFilters(ctx, filters, location)
    
    // 2. Generate query embedding
    queryEmbedding := s.generateEmbedding(query)
    
    // 3. Semantic search within pre-filtered results
    semanticResults := s.vectorSearch(queryEmbedding, preFilteredPOIs, limit)
    
    // 4. Post-process with conversation context
    return s.enhanceWithContext(semanticResults, filters)
}
```

#### 2. Dynamic Context Building

```go
type RAGContext struct {
    UserProfile      *UserProfile              `json:"user_profile"`
    ActiveFilters    map[DomainType]DomainFilters `json:"active_filters"`
    LocationContext  *LocationContext          `json:"location_context"`
    ConversationSummary string                 `json:"conversation_summary"`
    RecentPOIs       []types.POIDetail         `json:"recent_pois"`
    FilterRationale  map[string]string         `json:"filter_rationale"`
}

func (s *LlmInteractiontServiceImpl) BuildRAGContext(
    ctx context.Context,
    sessionID uuid.UUID,
    domain DomainType,
    query string,
) (*RAGContext, error) {
    
    session := s.getSession(sessionID)
    
    return &RAGContext{
        UserProfile:     s.getUserProfile(session.UserID),
        ActiveFilters:   session.Context.ActiveFilters,
        LocationContext: s.buildLocationContext(session.Context.CityName),
        ConversationSummary: s.summarizeConversation(session.ConversationHistory),
        RecentPOIs:     s.getRecentlyMentionedPOIs(session, domain),
        FilterRationale: s.explainActiveFilters(session.Context.ActiveFilters[domain]),
    }
}
```

#### 3. Multi-Domain Prompt Engineering

```go
func (s *LlmInteractiontServiceImpl) BuildUnifiedPrompt(
    ctx context.Context,
    query string,
    classification *ClassificationResult,
    ragContext *RAGContext,
    candidatePOIs []types.EnhancedPOI,
) string {
    
    var promptBuilder strings.Builder
    
    // System role with domain awareness
    promptBuilder.WriteString(fmt.Sprintf(`
You are a travel AI assistant specialized in %s recommendations. 

ACTIVE FILTERS: %s
USER CONTEXT: %s
LOCATION: %s
CONVERSATION CONTEXT: %s

Based on the user's query "%s", provide personalized recommendations from the filtered candidates below.

FILTERED CANDIDATES:
%s

Respond in JSON format appropriate for %s domain:
`, 
    classification.Domain,
    s.formatFilters(ragContext.ActiveFilters[classification.Domain]),
    s.formatUserContext(ragContext.UserProfile),
    ragContext.LocationContext.Name,
    ragContext.ConversationSummary,
    query,
    s.formatCandidates(candidatePOIs),
    classification.Domain))
    
    // Domain-specific response format
    switch classification.Domain {
    case DomainDining:
        promptBuilder.WriteString(s.getDiningResponseFormat())
    case DomainAccommodation:
        promptBuilder.WriteString(s.getAccommodationResponseFormat())
    case DomainActivities:
        promptBuilder.WriteString(s.getActivityResponseFormat())
    }
    
    return promptBuilder.String()
}
```

### Implementation Roadmap

#### Phase 1: Enhanced Classification (2-3 weeks)
- Implement domain detection alongside intent classification
- Add natural language filter extraction for common patterns
- Enhance session context to store active filters per domain

#### Phase 2: Filter-Aware RAG (3-4 weeks)  
- Implement pre-filtering in semantic search
- Build multi-domain prompt templates
- Add filter explanation and rationale generation

#### Phase 3: Conversation Flow (2-3 weeks)
- Add filter negotiation and clarification flows
- Implement filter persistence across conversation turns
- Add filter recommendation based on context

#### Phase 4: Advanced Features (4-5 weeks)
- Machine learning-based filter inference
- Preference learning from user interactions  
- Cross-domain filter suggestions

### Benefits of This Approach

1. **Unified User Experience**: Single input handles all domains intelligently
2. **Context Preservation**: Filters and preferences persist across conversation
3. **Semantic Enhancement**: RAG provides relevant context for better recommendations
4. **Scalable Architecture**: Easy to add new domains and filter types
5. **Learning Capability**: System improves recommendations over time
6. **Explainable AI**: Users understand why certain recommendations are made

### Technical Considerations

1. **Performance**: Pre-filtering reduces semantic search space
2. **Accuracy**: Multi-stage filtering improves recommendation relevance  
3. **Flexibility**: Natural language allows complex filter combinations
4. **Maintainability**: Clear separation between domain logic and AI components
5. **Observability**: Comprehensive logging of filter application and results

This integration strategy leverages the existing sophisticated chat system while adding the multi-domain filter capabilities, creating a powerful and intuitive travel recommendation system.

---

## Testing Endpoints for Enhanced Multi-Domain Filtering System

### Base URL
```
http://localhost:8000/api/v1/user/search-profile
```

### Authentication
All endpoints require Bearer token authentication:
```
Authorization: Bearer <your_jwt_token>
```

### 1. Get Combined Filters (with Domain Support)

**Endpoint:**
```
GET /api/v1/user/search-profile/{profileID}/filters?domain={domain}
```

**Query Parameters:**
- `domain` (optional): "accommodation", "dining", "activities", "itinerary", "general" (default: "general")

**Example Request:**
```
GET http://localhost:8000/api/v1/user/search-profile/30d93077-7aa7-4fb0-adec-7519448ba824/filters?domain=accommodation
```

**Expected Response:**
```json
{
  "profile_id": "30d93077-7aa7-4fb0-adec-7519448ba824",
  "domain": "accommodation",
  "base_preferences": {
    "search_radius_km": 5,
    "budget_level": 3,
    "preferred_transport": "public"
  },
  "accommodation_preferences": {
    "accommodation_type": ["hotel", "apartment"],
    "star_rating": {"min": 3, "max": 5},
    "price_range_per_night": {"min": 50, "max": 200},
    "amenities": ["wifi", "parking", "gym"]
  }
}
```

### 2. Accommodation Preferences Endpoints

#### Get Accommodation Preferences
**Endpoint:**
```
GET /api/v1/user/search-profile/{profileID}/accommodation
```

**Example Request:**
```
GET http://localhost:8000/api/v1/user/search-profile/30d93077-7aa7-4fb0-adec-7519448ba824/accommodation
```

#### Update Accommodation Preferences
**Endpoint:**
```
PUT /api/v1/user/search-profile/{profileID}/accommodation
```

**Request Body:**
```json
{
  "accommodation_type": ["hotel", "boutique", "apartment"],
  "star_rating": {"min": 4, "max": 5},
  "price_range_per_night": {"min": 100, "max": 300},
  "amenities": ["wifi", "parking", "pool", "gym", "spa"],
  "room_type": ["double", "suite"],
  "chain_preference": "boutique_chains",
  "cancellation_policy": ["free_cancellation"],
  "booking_flexibility": "instant_book"
}
```

### 3. Dining Preferences Endpoints

#### Get Dining Preferences
**Endpoint:**
```
GET /api/v1/user/search-profile/{profileID}/dining
```

#### Update Dining Preferences
**Endpoint:**
```
PUT /api/v1/user/search-profile/{profileID}/dining
```

**Request Body:**
```json
{
  "cuisine_types": ["italian", "french", "local_specialty"],
  "meal_types": ["lunch", "dinner"],
  "service_style": ["fine_dining", "casual"],
  "price_range_per_person": {"min": 25, "max": 100},
  "dietary_needs": ["vegetarian"],
  "allergen_free": ["nuts", "shellfish"],
  "michelin_rated": true,
  "local_recommendations": true,
  "chain_vs_local": "local_preferred",
  "organic_preference": false,
  "outdoor_seating_preferred": true
}
```

### 4. Activity Preferences Endpoints

#### Get Activity Preferences
**Endpoint:**
```
GET /api/v1/user/search-profile/{profileID}/activities
```

#### Update Activity Preferences
**Endpoint:**
```
PUT /api/v1/user/search-profile/{profileID}/activities
```

**Request Body:**
```json
{
  "activity_categories": ["museums", "nature", "arts", "history"],
  "physical_activity_level": "moderate",
  "indoor_outdoor_preference": "mixed",
  "cultural_immersion_level": "deep_local",
  "must_see_vs_hidden_gems": "mixed",
  "educational_preference": true,
  "photography_opportunities": true,
  "season_specific_activities": ["year_round"],
  "avoid_crowds": false,
  "local_events_interest": ["festivals", "cultural_events", "food_events"]
}
```

### 5. Itinerary Preferences Endpoints

#### Get Itinerary Preferences
**Endpoint:**
```
GET /api/v1/user/search-profile/{profileID}/itinerary
```

#### Update Itinerary Preferences
**Endpoint:**
```
PUT /api/v1/user/search-profile/{profileID}/itinerary
```

**Request Body:**
```json
{
  "planning_style": "flexible",
  "preferred_pace": "moderate",
  "time_flexibility": "loose_schedule",
  "morning_vs_evening": "flexible",
  "weekend_vs_weekday": "any",
  "preferred_seasons": ["spring", "summer", "fall"],
  "avoid_peak_season": false,
  "adventure_vs_relaxation": "balanced",
  "spontaneous_vs_planned": "semi_planned"
}
```

### 6. Enhanced LLM Chat Endpoint (Using New Filters)

**Endpoint:**
```
POST /api/v1/llm/prompt-response/chat/sessions/{profileID}
```

**Request Body:**
```json
{
  "city_name": "Paris",
  "message": "Find me a romantic Italian restaurant with outdoor seating near the Eiffel Tower"
}
```

**Expected Enhanced Response:**
```json
{
  "session_id": "1f0841de-8929-49d4-a9e7-1cea4eb67664",
  "domain_detected": "dining",
  "applied_filters": {
    "cuisine_types": ["italian"],
    "outdoor_seating_preferred": true,
    "ambiance": "romantic",
    "location_proximity": "eiffel_tower"
  },
  "data": {
    "restaurants": [
      {
        "name": "Bistrot de la Tour Eiffel",
        "cuisine_type": "Italian",
        "outdoor_seating": true,
        "price_range": "$$",
        "rating": 4.5,
        "distance_km": 0.3
      }
    ]
  }
}
```

### Testing Workflow

1. **Setup**: Create a user profile with ID `30d93077-7aa7-4fb0-adec-7519448ba824`
2. **Test Base Functionality**: Get combined filters for different domains
3. **Test Domain Updates**: Update preferences for each domain (accommodation, dining, activities, itinerary)
4. **Test Integration**: Use the enhanced chat endpoint to see filter application
5. **Test Filter Combinations**: Request combined filters to see merged preferences

### Example Test Sequence

1. **Get general filters:**
   ```
   GET /api/v1/user/search-profile/30d93077-7aa7-4fb0-adec-7519448ba824/filters
   ```

2. **Update accommodation preferences:**
   ```
   PUT /api/v1/user/search-profile/30d93077-7aa7-4fb0-adec-7519448ba824/accommodation
   ```

3. **Get accommodation-specific combined filters:**
   ```
   GET /api/v1/user/search-profile/30d93077-7aa7-4fb0-adec-7519448ba824/filters?domain=accommodation
   ```

4. **Test chat with enhanced filters:**
   ```
   POST /api/v1/llm/prompt-response/chat/sessions/30d93077-7aa7-4fb0-adec-7519448ba824
   Body: {"city_name": "Paris", "message": "Find luxury hotels with spa amenities"}
   ```

### Notes
- Replace `{profileID}` with actual profile UUID (e.g., `30d93077-7aa7-4fb0-adec-7519448ba824`)
- All endpoints return JSON responses with appropriate HTTP status codes
- Error responses include detailed error messages for debugging
- The system maintains backward compatibility with existing profile structures

 Looking at your pasted content and
  understanding your architecture, here's a
   good client-side approach:

  1. Route-based separation:
  /chat/hotels/[cityId] - Hotel-specific
  chat
  /chat/restaurants/[cityId] -
  Restaurant-specific chat
  /chat/itineraries/[cityId] -
  Itinerary-specific chat

  2. Context-aware API calls:
  - Pass the route context to your LLM
  endpoint
  - Modify your chat service to include the
   content type:

  // In your chat API call
  const response = await
  fetch('/api/v1/llm/prompt-response/chat/s
  essions/{sessionId}', {
    body: JSON.stringify({
      city_name: "Paris",
      message: "Find luxury hotels with spa
   amenities",
      context_type: "hotels" // Add this
    })
  })

  3. Server-side routing enhancement:
  Create separate endpoints or use query
  parameters:
  - /api/v1/llm/hotels/chat/sessions/{id}
  - /api/v1/llm/restaurants/chat/sessions/{
  id}
  - /api/v1/llm/itineraries/chat/sessions/{
  id}

  This keeps your chat sessions
  contextually aware and allows for
  specialized prompting per content type.