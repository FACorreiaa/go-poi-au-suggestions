
# WanderWiseAI: Integrating Google Gemini API with go-genai

Below is a concise, actionable guide for integrating the Google Gemini API using the go-genai SDK within your generativeAI package for WanderWiseAI. This guide focuses on enabling the AI to read user interests from your database and generate personalized itineraries, as well as supporting multi-turn or chained prompts for an interactive chat experience (e.g., "Berlin, 3km around me" → "Change to 5km around me and display more info"). This is tailored to your project’s goals and database schema.

**Overview**

Your WanderWiseAI application leverages the Gemini API to:

 

 - Read User Interests: Fetch user preferences from the interests  
   or user_profile_interests tables and use them to craft personalized  
   itineraries.
   
  - Generate Itineraries: Use Gemini to suggest points of interest (POIs)
   based on interests, location, and constraints, integrating with your
   PostgreSQL/PostGIS database.
   
  -   Enable Multi-Turn Dialogues: Allow users to refine itineraries
   interactively within a chat session.

The go-genai SDK is sufficient for these tasks, acting as the bridge between your Go backend and Gemini’s capabilities. You won’t need an MPC server or a separate AI agent framework initially—your Go backend will orchestrate everything.

Steps to Integrate go-genai and Enable Multi-Turn Prompts

1. Set Up the go-genai Client

Install the SDK and initialize the client with your API key.

    package generativeAI
    
      
    
    import (
    
    "context"
    
    "log"
    
    "os"
    
      
    
    "github.com/google/generative-ai-go/genai"
    
    "google.golang.org/api/option"
    
    )
    
      
    
    type AIClient struct {
    
    client *genai.Client
    
    model *genai.GenerativeModel
    
    }
    
      
    
    func NewAIClient(ctx context.Context) (*AIClient, error) {
    
    apiKey := os.Getenv("GEMINI_API_KEY")
    
    if apiKey == "" {
    
    log.Fatal("GEMINI_API_KEY not set")
    
    }
    
    client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
    
    if err != nil {
    
    return nil, err
    
    }
    
    return &AIClient{
    
    client: client,
    
    model: client.GenerativeModel("gemini-1.5-pro"), // Adjust model as needed
    
    }, nil
    
    }
    
      
    
    func (c *AIClient) Close() {
    
    c.client.Close()
    
    }

Where: Place this in your generativeAI package (e.g., ai_client.go).

Why: This sets up a reusable client for all Gemini interactions.

2. Fetch User Interests and Construct the Initial Prompt

Use your database to retrieve user interests and build a prompt for Gemini.

Database Query

    Assuming you’re using pgx:
    
    go
    
    package generativeAI
    
      
    
    import (
    
    "context"
    
    "fmt"
    
      
    
    "github.com/jackc/pgx/v5"
    
    )
    
      
    
    // Assuming a DB connection is passed or managed elsewhere
    
    type UserService struct {
    
    db *pgx.Conn // Or your preferred DB interface
    
    }
    
      
    
    func (s *UserService) Getinterestss(ctx context.Context, userID string) ([]string, error) {
    
    rows, err := s.db.Query(ctx, `
    
    SELECT i.name
    
    FROM interests ui
    
    JOIN interests i ON ui.interest_id = i.id
    
    WHERE ui.user_id = $1`, userID)
    
    if err != nil {
    
    return nil, err
    
    }
    
    defer rows.Close()
    
      
    
    var interests []string
    
    for rows.Next() {
    
    var interest string
    
    if err := rows.Scan(&interest); err != nil {
    
    return nil, err
    
    }
    
    interests = append(interests, interest)
    
    }
    
    return interests, rows.Err()
    
    }
    
    Alternative: Use user_profile_interests if leveraging user_preference_profiles:
    
    sql
    
    SELECT i.name
    
    FROM user_profile_interests upi
    
    JOIN interests i ON upi.interest_id = i.id
    
    WHERE upi.profile_id = (SELECT id FROM user_preference_profiles WHERE user_id = $1 AND is_default = TRUE)
    
    Prompt Construction
    
    go
    
    func (c *AIClient) GenerateItinerary(ctx context.Context, userID, location string, radiusKm float64) (string, error) {
    
    userSvc := UserService{db: /* your DB connection */}
    
    interests, err := userSvc.Getinterestss(ctx, userID)
    
    if err != nil {
    
    return "", err
    
    }

 

    prompt := fmt.Sprintf(
    
    "Generate a personalized itinerary for a user in %s within a %f km radius. "+
    
    "The user is interested in: %s. Include 3-4 points of interest with brief descriptions.",
    
    location, radiusKm, strings.Join(interests, ", "),
    
    )
    
    resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
    
    if err != nil {
    
    return "", err
    
    }
    
    return string(resp.Candidates[0].Content.Parts[0].(genai.Text)), nil
    
    }

Where: Add this to your AIClient struct in generativeAI.

Why: This fetches interests from your DB and feeds them into Gemini to tailor the itinerary.

3. Enhance with Function Calling for Precise POI Data

Use function calling to fetch accurate POI data (e.g., coordinates) from your points_of_interest table instead of relying on Gemini to generate them.

Define a Function

    type POI struct {
    
    Name string `json:"name"`
    
    Latitude float64 `json:"lat"`
    
    Longitude float64 `json:"lon"`
    
    Description string `json:"description"`
    
    }
    
      
    
    func (s *UserService) FindPOIs(ctx context.Context, cityName string, radiusKm float64, interests []string) ([]POI, error) {
    
    query := `
    
    SELECT p.name, ST_Y(p.location) AS lat, ST_X(p.location) AS lon, p.description
    
    FROM points_of_interest p
    
    JOIN cities c ON p.city_id = c.id
    
    WHERE c.name = $1
    
    AND ST_DWithin(p.location, c.center_location, $2 * 1000)
    
    AND p.tags && $3::text[]`
    
    rows, err := s.db.Query(ctx, query, cityName, radiusKm, pq.Array(interests))
    
    if err != nil {
    
    return nil, err
    
    }
    
    defer rows.Close()
    
      
    
    var pois []POI
    
    for rows.Next() {
    
    var poi POI
    
    if err := rows.Scan(&poi.Name, &poi.Latitude, &poi.Longitude, &poi.Description); err != nil {
    
    return nil, err
    
    }
    
    pois = append(pois, poi)
    
    }
    
    return pois, rows.Err()
    
    }
    
    Register with Gemini
    
    go
    
    func (c *AIClient) SetupFunctionCalling() {
    
    poiFunc := &genai.FunctionDeclaration{
    
    Name: "findPOIs",
    
    Description: "Find points of interest in a city within a radius, filtered by user interests.",
    
    Parameters: &genai.Schema{
    
    Type: genai.TypeObject,
    
    Properties: map[string]*genai.Schema{
    
    "cityName": {Type: genai.TypeString, Description: "City name"},
    
    "radiusKm": {Type: genai.TypeNumber, Description: "Radius in kilometers"},
    
    "interests": {Type: genai.TypeArray, Description: "List of user interests", Items: &genai.Schema{Type: genai.TypeString}},
    
    },
    
    Required: []string{"cityName", "radiusKm", "interests"},
    
    },
    
    }
    
    c.model.Tools = []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{poiFunc}}}
    
    }

**Handle Function Calls**

    func (c *AIClient) GenerateItineraryWithFunctions(ctx context.Context, userID, location string, radiusKm float64) (string, error) {
    
    c.SetupFunctionCalling()
    
    userSvc := UserService{db: /* your DB connection */}
    
    interests, err := userSvc.Getinterestss(ctx, userID)
    
    if err != nil {
    
    return "", err
    
    }
    
      
    
    chat := c.model.StartChat()
    
    prompt := fmt.Sprintf("Plan an itinerary in %s within %f km, interests: %s.", location, radiusKm, strings.Join(interests, ", "))
    
    resp, err := chat.SendMessage(ctx, genai.Text(prompt))
    
    if err != nil {
    
    return "", err
    
    }
    
      
    
    for {
    
    if len(resp.Candidates) == 0 {
    
    break
    
    }
    
    candidate := resp.Candidates[0]
    
    if fc, ok := candidate.Content.Parts[0].(genai.FunctionCall); ok && fc.Name == "findPOIs" {
    
    cityName := fc.Args["cityName"].(string)
    
    radius := fc.Args["radiusKm"].(float64)
    
    intfInterests := fc.Args["interests"].([]interface{})
    
    interestList := make([]string, len(intfInterests))
    
    for i, v := range intfInterests {
    
    interestList[i] = v.(string)
    
    }
    
      
    
    pois, err := userSvc.FindPOIs(ctx, cityName, radius, interestList)
    
    if err != nil {
    
    return "", err
    
    }
    
    resp, err = chat.SendMessage(ctx, genai.FunctionResponse{Name: "findPOIs", Response: pois})
    
    if err != nil {
    
    return "", err
    
    }
    
    continue
    
    }
    
    return string(candidate.Content.Parts[0].(genai.Text)), nil
    
    }
    
    return "", fmt.Errorf("no valid response")
    
    }

Where: Add to your AIClient in generativeAI.

Why: Ensures Gemini uses your authoritative POI data, enhancing accuracy.

4. Enable Multi-Turn Dialogues (Chained Prompts)

**Support interactive refinements like "Change to 5km around me and display more info" within a chat session.**

Session Management

    go
    
    type SessionState struct {
    
    UserID string
    
    Location string
    
    RadiusKm float64
    
    Itinerary []POI
    
    History []Turn
    
    }
    
      
    
    type Turn struct {
    
    Role string
    
    Content string
    
    }
    
      
    
    type SessionManager struct {
    
    sessions map[string]*SessionState // In-memory; use Redis/Postgres for persistence
    
    }
    
      
    
    func NewSessionManager() *SessionManager {
    
    return &SessionManager{sessions: make(map[string]*SessionState)}
    
    }
    
      
    
    func (sm *SessionManager) StartSession(userID, location string, radiusKm float64) string {
    
    sessionID := uuid.New().String()
    
    sm.sessions[sessionID] = &SessionState{
    
    UserID: userID,
    
    Location: location,
    
    RadiusKm: radiusKm,
    
    }
    
    return sessionID
    
    }
    
      
    
    func (sm *SessionManager) GetSession(sessionID string) (*SessionState, bool) {
    
    session, exists := sm.sessions[sessionID]
    
    return session, exists
    
    }
    
    Handle Chat Refinements
    
    go
    
    func (c *AIClient) ChatWithItinerary(ctx context.Context, sessionID, message string, sm *SessionManager) (string, error) {
    
    session, exists := sm.GetSession(sessionID)
    
    if !exists {
    
    return "", fmt.Errorf("session not found")
    
    }
    
      
    
    chat := c.model.StartChat()
    
    for _, turn := range session.History {
    
    chat.History = append(chat.History, &genai.Content{Role: turn.Role, Parts: []genai.Part{genai.Text(turn.Content)}})
    
    }
    
      
    
    prompt := fmt.Sprintf("Current context: Location=%s, Radius=%f km, Itinerary=%v\nUser says: %s",
    
    session.Location, session.RadiusKm, session.Itinerary, message)
    
    resp, err := chat.SendMessage(ctx, genai.Text(prompt))
    
    if err != nil {
    
    return "", err
    
    }

  

// Update session state based on response (simplified)

    responseText := string(resp.Candidates[0].Content.Parts[0].(genai.Text))
    
    session.History = append(session.History, Turn{Role: "user", Content: message}, Turn{Role: "model", Content: responseText})
    
      
    
    // Example: Parse "Change to 5km" and update
    
    if strings.Contains(message, "5km") {
    
    session.RadiusKm = 5.0
    
    pois, err := UserService{db: /* your DB */}.FindPOIs(ctx, session.Location, session.RadiusKm, /* interests */)
    
    if err != nil {
    
    return "", err
    
    }
    
    session.Itinerary = pois
    
    }
    
      
    
    return responseText, nil
    
    }

Usage Example:

    sm := NewSessionManager()
    
    sessionID := sm.StartSession("user123", "Berlin", 3.0)
    
    resp, _ := c.GenerateItineraryWithFunctions(ctx, "user123", "Berlin", 3.0) // Initial itinerary
    
    fmt.Println(resp)
    
    resp, _ = c.ChatWithItinerary(ctx, sessionID, "Change to 5km around me and display more info", sm)
    
    fmt.Println(resp)

Where: Add to generativeAI package.

Why: Maintains context across turns, allowing dynamic refinements.

5. Store Results in Database

Save itineraries and AI-generated descriptions for reuse.
 
    INSERT INTO itineraries (id, user_id, city_id) VALUES ($1, $2, (SELECT id FROM cities WHERE name = $3));
    
    INSERT INTO itinerary_pois (itinerary_id, poi_id, order_index, ai_description)
    
    VALUES ($1, $2, $3, $4);

Go Implementation: Add a method to save session.Itinerary and Gemini’s response text to these tables.

Key Considerations

Structured Output: For cleaner parsing, instruct Gemini to return JSON:


    prompt += "\nReturn the itinerary as a JSON array with name, coordinates (lat, lon), and description for each POI."

Persistence: Replace in-memory sessions with Redis or PostgreSQL (conversations table) for production.

Performance: Cache frequent queries (e.g., POIs) in Redis.

Error Handling: Add robust error checks and retries for API calls.

Conclusion

With these steps, your generativeAI package will:

Read user interests from your database to personalize itineraries.

Use go-genai with function calling to integrate precise POI data.

Support multi-turn dialogues for an interactive chat experience.

This approach keeps your stack Go-centric, leverages your existing schema, and meets your project’s MVP needs while setting the stage for future enhancements like advanced filtering or premium features. Start implementing and iterate based on user feedbac



