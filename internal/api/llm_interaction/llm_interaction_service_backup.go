package llmInteraction

// const (
// 	model              = "gemini-2.0-flash"
// 	defaultTemperature = 0.5
// )

// var sessions = make(map[string]*genai.Chat) // In-memory session store
// var sessionsMu sync.Mutex                   // Mutex for thread-safe access

// // Ensure implementation satisfies the interface
// var _ LlmInteractiontService = (*LlmInteractiontServiceImpl)(nil)

// // LlmInteractiontService defines the business logic contract for user operations.
// type LlmInteractiontService interface {
// 	GetPromptResponse(ctx context.Context,
// 		sessionIDInput *string,
// 		cityName string,
// 		userID,
// 		profileID uuid.UUID,
// 		userLocation *types.UserLocation,
// 		userRequest UserRequest, // This is not used in the current implementation, but can be used for future enhancements
// 		isNewConversation bool,
// 	) (*types.AiCityResponse, string, error)
// 	SaveItenerary(ctx context.Context, userID uuid.UUID, req types.BookmarkRequest) (uuid.UUID, error)
// 	RemoveItenerary(ctx context.Context, userID, itineraryID uuid.UUID) error
// }

// // LlmInteractiontServiceImpl provides the implementation for LlmInteractiontService.
// type LlmInteractiontServiceImpl struct {
// 	logger             *slog.Logger
// 	interestRepo       userInterest.UserInterestRepo
// 	searchProfileRepo  userSearchProfile.UserSearchProfilesRepo
// 	tagsRepo           userTags.UserTagsRepo
// 	aiClient           *generativeAI.AIClient
// 	llmInteractionRepo LLmInteractionRepository
// 	cityRepo           city.CityRepository
// 	poiRepo            poi.POIRepository
// }

// // NewLlmInteractiontService creates a new user service instance.
// func NewLlmInteractiontService(interestRepo userInterest.UserInterestRepo,
// 	searchProfileRepo userSearchProfile.UserSearchProfilesRepo,
// 	tagsRepo userTags.UserTagsRepo,
// 	llmInteractionRepo LLmInteractionRepository,
// 	cityRepo city.CityRepository,
// 	poiRepo poi.POIRepository,
// 	logger *slog.Logger) *LlmInteractiontServiceImpl {
// 	ctx := context.Background()
// 	aiClient, err := generativeAI.NewAIClient(ctx)
// 	if err != nil {
// 		log.Fatalf("Failed to create AI client: %v", err) // Terminate if initialization fails
// 	}
// 	return &LlmInteractiontServiceImpl{
// 		logger:             logger,
// 		tagsRepo:           tagsRepo,
// 		interestRepo:       interestRepo,
// 		searchProfileRepo:  searchProfileRepo,
// 		aiClient:           aiClient,
// 		llmInteractionRepo: llmInteractionRepo,
// 		cityRepo:           cityRepo,
// 		poiRepo:            poiRepo,
// 	}
// }

// func (l *LlmInteractiontServiceImpl) generateGeneralCityData(wg *sync.WaitGroup,
// 	ctx context.Context,
// 	cityName string,
// 	resultCh chan<- types.GenAIResponse,
// 	config *genai.GenerateContentConfig) {
// 	go func() {
// 		defer wg.Done()
// 		prompt := GetCityDescriptionPrompt(cityName)
// 		response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
// 		if err != nil {
// 			resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to generate city data: %w", err)}
// 			return
// 		}

// 		var txt string
// 		for _, candidate := range response.Candidates {
// 			if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
// 				txt = candidate.Content.Parts[0].Text
// 				break
// 			}
// 		}
// 		if txt == "" {
// 			resultCh <- types.GenAIResponse{Err: fmt.Errorf("no valid city data content from AI")}
// 			return
// 		}

// 		jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
// 		var cityDataFromAI struct {
// 			CityName        string  `json:"city_name"`
// 			StateProvince   *string `json:"state_province"` // Use pointer for nullable string
// 			Country         string  `json:"country"`
// 			CenterLatitude  float64 `json:"center_latitude"`
// 			CenterLongitude float64 `json:"center_longitude"`
// 			Description     string  `json:"description"`
// 			// BoundingBox     string  `json:"bounding_box,omitempty"` // If trying to get BBox string
// 		}
// 		if err := json.Unmarshal([]byte(jsonStr), &cityDataFromAI); err != nil {
// 			resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse city data JSON: %w", err)}
// 			return
// 		}

// 		stateProvinceValue := ""
// 		if cityDataFromAI.StateProvince != nil {
// 			stateProvinceValue = *cityDataFromAI.StateProvince
// 		}

// 		resultCh <- types.GenAIResponse{
// 			City:            cityDataFromAI.CityName,
// 			Country:         cityDataFromAI.Country,
// 			StateProvince:   stateProvinceValue,
// 			CityDescription: cityDataFromAI.Description,
// 			Latitude:        cityDataFromAI.CenterLatitude,
// 			Longitude:       cityDataFromAI.CenterLongitude,
// 			// BoundingBoxWKT: cityDataFromAI.BoundingBox, // TODO
// 		}
// 	}()
// }

// func (l *LlmInteractiontServiceImpl) generateGeneralPOI(wg *sync.WaitGroup,
// 	ctx context.Context,
// 	cityName string,
// 	resultCh chan<- types.GenAIResponse,
// 	config *genai.GenerateContentConfig) {
// 	defer wg.Done()
// 	prompt := GetGeneralPOI(cityName)
// 	//startTime := time.Now()
// 	response, err := l.aiClient.GenerateResponse(ctx, prompt, config)
// 	//latencyMs := int(time.Since(startTime).Milliseconds())
// 	if err != nil {
// 		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to generate general POIs: %w", err)}
// 		return
// 	}

// 	var txt string
// 	for _, candidate := range response.Candidates {
// 		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
// 			txt = candidate.Content.Parts[0].Text
// 			break
// 		}
// 	}
// 	if txt == "" {
// 		resultCh <- types.GenAIResponse{Err: fmt.Errorf("no valid general POI content from AI")}
// 		return
// 	}

// 	jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
// 	var poiData struct {
// 		PointsOfInterest []types.POIDetail `json:"points_of_interest"`
// 	}
// 	if err := json.Unmarshal([]byte(jsonStr), &poiData); err != nil {
// 		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse general POI JSON: %w", err)}
// 		return
// 	}

// 	resultCh <- types.GenAIResponse{GeneralPOI: poiData.PointsOfInterest}
// }

// func (l *LlmInteractiontServiceImpl) generatePersonalisedPOI(
// 	wg *sync.WaitGroup,
// 	ctx context.Context,
// 	userID, profileID uuid.UUID,
// 	resultCh chan<- types.GenAIResponse,
// 	prompt string,
// 	chatObj *genai.Chat,
// ) {
// 	defer wg.Done()
// 	startTime := time.Now()

// 	l.logger.DebugContext(ctx, "Sending message to chat object", slog.String("prompt_summary", TruncateString(prompt, 100)))

// 	part := genai.Part{Text: prompt}
// 	p := make([]genai.Part, 1)
// 	p[0] = part
// 	response, err := chatObj.SendMessage(ctx, part)
// 	if err != nil {
// 		l.logger.ErrorContext(ctx, "Chat SendMessage failed", slog.Any("error", err))
// 		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to generate personalized itinerary via chat: %w", err)}
// 		return
// 	}

// 	var txt string
// 	for _, candidate := range response.Candidates {
// 		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
// 			txt = candidate.Content.Parts[0].Text
// 			break
// 		}
// 	}
// 	if txt == "" {
// 		l.logger.WarnContext(ctx, "No valid text content from AI chat response for personalized POI")
// 		resultCh <- types.GenAIResponse{Err: fmt.Errorf("no valid personalized itinerary content from AI chat")}
// 		return
// 	}

// 	jsonStr := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(txt), "```"), "```json")
// 	var itineraryData struct {
// 		ItineraryName      string            `json:"itinerary_name"`
// 		OverallDescription string            `json:"overall_description"`
// 		PointsOfInterest   []types.POIDetail `json:"points_of_interest"`
// 	}
// 	if err := json.Unmarshal([]byte(jsonStr), &itineraryData); err != nil {
// 		l.logger.ErrorContext(ctx, "Failed to parse personalized itinerary JSON from chat", slog.Any("error", err))
// 		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to parse personalized itinerary JSON: %w", err)}
// 		return
// 	}

// 	latencyMs := int(time.Since(startTime).Milliseconds())
// 	interaction := types.LlmInteraction{
// 		UserID:       userID,
// 		Prompt:       prompt,
// 		ResponseText: txt,
// 		ModelUsed:    "your-model-name", // Replace with actual model name
// 		LatencyMs:    latencyMs,
// 	}
// 	savedInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
// 	if err != nil {
// 		l.logger.ErrorContext(ctx, "Failed to save LLM interaction", slog.Any("error", err))
// 	}

// 	resultCh <- types.GenAIResponse{
// 		ItineraryName:        itineraryData.ItineraryName,
// 		ItineraryDescription: itineraryData.OverallDescription,
// 		PersonalisedPOI:      itineraryData.PointsOfInterest,
// 		LlmInteractionID:     savedInteractionID,
// 	}
// }

// type UserRequest struct {
// 	Interests  []string // e.g., ["art", "history"]
// 	Tags       []string // e.g., ["family-friendly", "outdoor"]
// 	Categories []string // e.g., ["restaurants", "museums"]
// 	// Add other fields as needed, e.g., Budget, TimeOfDay, etc.
// }

// func (l *LlmInteractiontServiceImpl) GetPromptResponse(ctx context.Context,
// 	sessionIDInput *string,
// 	cityName string,
// 	userID,
// 	profileID uuid.UUID,
// 	userLocation *types.UserLocation,
// 	userRequest UserRequest, // This is not used in the current implementation, but can be used for future enhancements
// 	isNewConversation bool,
// ) (*types.AiCityResponse, string, error) {
// 	var currentSessionID string
// 	var chatObj *genai.Chat

// 	// --- Chat Session Management ---
// 	sessionsMu.Lock() // Lock for writing/reading map
// 	if sessionIDInput != nil && *sessionIDInput != "" {
// 		var ok bool
// 		chatObj, ok = sessions[*sessionIDInput]
// 		if ok {
// 			l.logger.InfoContext(ctx, "Continuing existing chat session", slog.String("sessionID", *sessionIDInput))
// 			currentSessionID = *sessionIDInput
// 		} else {
// 			l.logger.WarnContext(ctx, "Session ID provided but not found, starting new session", slog.String("provided_sessionID", *sessionIDInput))
// 			isNewConversation = true // Force new conversation if ID is invalid
// 		}
// 	} else {
// 		isNewConversation = true // No session ID means new conversation
// 	}

// 	if isNewConversation || chatObj == nil {
// 		// EDIT This
// 		client, err := genai.NewClient(ctx, &genai.ClientConfig{
// 			APIKey:  os.Getenv("GOOGLE_GEMINI_API_KEY"),
// 			Backend: genai.BackendGeminiAPI,
// 		})

// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		newUUID, _ := uuid.NewRandom() // Generate a new session ID
// 		currentSessionID = newUUID.String()

// 		// Configuration for creating the chat
// 		// This config (e.g., temperature) applies to the entire chat session.
// 		var chatConfig *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}

// 		// Create a new chat session using the underlying genai.Client from your wrapper
// 		// The history (`nil` here) is for starting a chat with pre-existing messages.
// 		// For a brand new chat, it's typically nil.
// 		newChatObj, err := client.Chats.Create(ctx, model, chatConfig, nil)
// 		if err != nil {
// 			sessionsMu.Unlock()
// 			l.logger.ErrorContext(ctx, "Failed to create new genai.Chat object", slog.Any("error", err))
// 			return nil, "", fmt.Errorf("failed to create chat session: %w", err)
// 		}
// 		chatObj = newChatObj
// 		sessions[currentSessionID] = chatObj
// 		l.logger.InfoContext(ctx, "Started new chat session", slog.String("sessionID", currentSessionID))
// 	}
// 	sessionsMu.Unlock()

// 	// Fetch user profile data
// 	config := &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)}
// 	l.logger.DebugContext(ctx, "Starting itinerary generation", slog.String("cityName", cityName),
// 		slog.String("userID", userID.String()),
// 		slog.String("profileID", profileID.String()))

// 	interests, err := l.interestRepo.GetInterestsForProfile(ctx, profileID)
// 	if err != nil {
// 		return nil, "", fmt.Errorf("failed to fetch user interests: %w", err)
// 	}

// 	// Inside GetPromptResponse
// 	var promptParts []string

// 	// Add interests to the prompt if provided
// 	if len(userRequest.Interests) > 0 {
// 		promptParts = append(promptParts, fmt.Sprintf("Interests: %s", strings.Join(userRequest.Interests, ", ")))
// 	}

// 	// Add tags to the prompt if provided
// 	if len(userRequest.Tags) > 0 {
// 		promptParts = append(promptParts, fmt.Sprintf("Tags: %s", strings.Join(userRequest.Tags, ", ")))
// 	}

// 	// Add categories to the prompt if provided
// 	if len(userRequest.Categories) > 0 {
// 		promptParts = append(promptParts, fmt.Sprintf("Categories: %s", strings.Join(userRequest.Categories, ", ")))
// 	}

// 	// Construct the prompt based on available inputs
// 	var promptForPersonalizedPOITurn string

// 	if len(promptParts) == 0 {
// 		// Fallback for when no specific inputs are provided
// 		promptForPersonalizedPOITurn = fmt.Sprintf("Generate a general itinerary for %s.", cityName)
// 	} else {
// 		if isNewConversation {
// 			promptForPersonalizedPOITurn = fmt.Sprintf("Generate a personalized itinerary for %s based on the following: %s", cityName, strings.Join(promptParts, "; "))
// 		} else {
// 			promptForPersonalizedPOITurn = fmt.Sprintf("Continuing our discussion about %s. Update the itinerary based on: %s", cityName, strings.Join(promptParts, "; "))
// 		}
// 	}

// 	searchProfile, err := l.searchProfileRepo.GetSearchProfile(ctx, userID, profileID)
// 	if err != nil {
// 		return nil, "", fmt.Errorf("failed to fetch search profile: %w", err)
// 	}
// 	tags, err := l.tagsRepo.GetTagsForProfile(ctx, profileID)
// 	if err != nil {
// 		return nil, "", fmt.Errorf("failed to fetch user tags: %w", err)
// 	}

// 	var interestNames []string
// 	var tagInfoForPrompt []string
// 	if len(interests) == 0 {
// 		l.logger.WarnContext(ctx, "No interests found for profile, using fallback.", slog.String("profileID", profileID.String()))
// 		interestNames = []string{"general sightseeing", "local experiences"}
// 	} else {
// 		for _, interest := range interests {
// 			if interest != nil {
// 				interestNames = append(interestNames, interest.Name)
// 			}
// 		}
// 	}
// 	if len(tags) > 0 {
// 		for _, tag := range tags {
// 			if tag != nil {
// 				tagDetail := tag.Name
// 				if tag.Description != nil && *tag.Description != "" {
// 					tagDetail += fmt.Sprintf(" (meaning: %s)", *tag.Description)
// 				}
// 				tagInfoForPrompt = append(tagInfoForPrompt, tagDetail)
// 			}
// 		}
// 	}
// 	tagsPromptPart := ""
// 	if len(tagInfoForPrompt) > 0 {
// 		tagsPromptPart = fmt.Sprintf("\n    - Additionally, consider these specific user tags/preferences: [%s].", strings.Join(tagInfoForPrompt, "; "))
// 	}

// 	// Common user preferences for prompts
// 	userPrefs := GetUserPreferencesPrompt(searchProfile)

// 	//

// 	if isNewConversation {
// 		promptForPersonalizedPOITurn = GetPersonalizedPOI(interestNames, cityName, tagsPromptPart, userPrefs) // Your existing initial prompt
// 	} else {
// 		// Construct a follow-up prompt. This needs to guide the LLM.
// 		// Example:
// 		promptForPersonalizedPOITurn = fmt.Sprintf("Okay, continuing our discussion about %s. My interests for this request are: [%s]. %s Please provide an updated JSON list of points of interest based on these refined interests, using the same JSON structure as before. Ensure the response is ONLY the JSON object, enclosed in triple backticks (```json ... ```).",
// 			cityName, // Good to remind the city context if it might be ambiguous
// 			strings.Join(interestNames, ", "),
// 			tagsPromptPart, // Can include tags/user prefs again if they are global
// 		)
// 		if len(promptForPersonalizedPOITurn) == 0 {
// 			promptForPersonalizedPOITurn = fmt.Sprintf("Okay, continuing our discussion about %s. I don't have any specific new interests for this turn, please suggest some general POIs or refine based on our previous conversation. %s Use the standard JSON output format.", cityName, tagsPromptPart)
// 		}
// 	}

// 	if searchProfile.UserLatitude != nil && searchProfile.UserLongitude != nil {
// 		userLocation = &types.UserLocation{
// 			UserLat: *searchProfile.UserLatitude,
// 			UserLon: *searchProfile.UserLongitude,
// 		}
// 	} else {
// 		l.logger.WarnContext(ctx, "User location not available in search profile, cannot sort personalised POIs by distance.")
// 		userLocation = nil
// 	}

// 	// Define result struct to collect data from goroutines
// 	// resultCh := make(chan types.GenAIResponse, 3)
// 	// var wg sync.WaitGroup
// 	// wg.Add(3)

// 	// // **Goroutine 1: Generate city, country, and description**
// 	// go l.generateGeneralCityData(&wg, ctx, cityName, resultCh, config)
// 	// // **Goroutine 2: Generate general points of interest**
// 	// go l.generateGeneralPOI(&wg, ctx, cityName, resultCh, config)

// 	// // **Goroutine 3: Generate itinerary name, description, and personalized POIs**
// 	// go l.generatePersonalisedPOI(&wg, ctx, cityName, userID, profileID, resultCh, interestNames, tagsPromptPart, userPrefs, config)

// 	// // Close result channel after goroutines complete
// 	// go func() {
// 	// 	wg.Wait()
// 	// 	close(resultCh)
// 	// }()

// 	resultCh := make(chan types.GenAIResponse, 3)
// 	var wg sync.WaitGroup

// 	// Personalized POI generation now uses the chatObj
// 	wg.Add(1)
// 	go l.generatePersonalisedPOI(&wg, ctx, userID, profileID, resultCh, promptForPersonalizedPOITurn, chatObj)

// 	// General city data and POIs - typically for new conversations or if city changes.
// 	// These still use the one-shot GenerateResponse method from your aiClient.
// 	if isNewConversation {
// 		wg.Add(2)
// 		go l.generateGeneralCityData(&wg, ctx, cityName, resultCh, config)
// 		go l.generateGeneralPOI(&wg, ctx, cityName, resultCh, config)
// 	}

// 	go func() {
// 		wg.Wait()
// 		close(resultCh)
// 	}()

// 	// Collect results from goroutines
// 	var itinerary types.AiCityResponse
// 	var errors []error
// 	var llmInteractionIDForPersonalisedPOIs uuid.UUID
// 	var rawPersonalisedPOIs []types.POIDetail

// 	for res := range resultCh {
// 		if res.Err != nil {
// 			errors = append(errors, res.Err)
// 			continue
// 		}
// 		if res.City != "" {
// 			itinerary.GeneralCityData.City = res.City
// 			itinerary.GeneralCityData.Country = res.Country
// 			itinerary.GeneralCityData.Description = res.CityDescription
// 			itinerary.GeneralCityData.StateProvince = res.StateProvince
// 			itinerary.GeneralCityData.CenterLatitude = res.Latitude
// 			itinerary.GeneralCityData.CenterLongitude = res.Longitude
// 			// itinerary.GeneralCityData.BoundingBoxWKT = res.BoundingBoxWKT // TODO
// 		}
// 		if res.ItineraryName != "" {
// 			itinerary.AIItineraryResponse.ItineraryName = res.ItineraryName
// 			itinerary.AIItineraryResponse.OverallDescription = res.ItineraryDescription
// 		}
// 		if len(res.GeneralPOI) > 0 {
// 			itinerary.PointsOfInterest = res.GeneralPOI
// 		}
// 		if len(res.PersonalisedPOI) > 0 {
// 			itinerary.AIItineraryResponse.PointsOfInterest = res.PersonalisedPOI
// 		}
// 		if res.LlmInteractionID != uuid.Nil {
// 			llmInteractionIDForPersonalisedPOIs = res.LlmInteractionID
// 			itinerary.AIItineraryResponse.ItineraryName = res.ItineraryName
// 			itinerary.AIItineraryResponse.OverallDescription = res.ItineraryDescription
// 			rawPersonalisedPOIs = res.PersonalisedPOI // Store raw POIs for now
// 		}
// 	}

// 	// Handle any errors from goroutines
// 	if len(errors) > 0 {
// 		l.logger.ErrorContext(ctx, "Errors during itinerary generation", slog.Any("errors", errors))
// 		return nil, "", fmt.Errorf("failed to generate itinerary: %v", errors)
// 	}

// 	// Validate that the itinerary has a name and personalized POIs
// 	if itinerary.AIItineraryResponse.ItineraryName == "" || len(itinerary.AIItineraryResponse.PointsOfInterest) == 0 {
// 		l.logger.ErrorContext(ctx, "Incomplete itinerary generated")
// 		return nil, "", fmt.Errorf("incomplete itinerary: missing name or personalized POIs")
// 	}

// 	l.logger.InfoContext(ctx, "Successfully generated itinerary",
// 		slog.String("itinerary_name", itinerary.AIItineraryResponse.ItineraryName),
// 		slog.Int("poi_count", len(itinerary.AIItineraryResponse.PointsOfInterest)))

// 	// Check if city exists in the database and save if not
// 	// city, err := l.cityRepo.FindCityByNameAndCountry(ctx, itinerary.GeneralCityData.City, itinerary.GeneralCityData.Country)
// 	// if err != nil {
// 	// 	return nil, "", fmt.Errorf("failed to check city existence: %w", err)
// 	// }

// 	var cityID uuid.UUID
// 	// if city == nil {
// 	// 	cityDetail := types.CityDetail{
// 	// 		Name:            itinerary.GeneralCityData.City,
// 	// 		Country:         itinerary.GeneralCityData.Country,
// 	// 		StateProvince:   itinerary.GeneralCityData.StateProvince,
// 	// 		AiSummary:       itinerary.GeneralCityData.Description,
// 	// 		CenterLatitude:  itinerary.GeneralCityData.CenterLatitude,  // Pass these to SaveCity
// 	// 		CenterLongitude: itinerary.GeneralCityData.CenterLongitude, // Pass these to SaveCity
// 	// 	}
// 	// 	cityID, err = l.cityRepo.SaveCity(ctx, cityDetail)
// 	// 	if err != nil {
// 	// 		l.logger.ErrorContext(ctx, "Failed to save city", slog.Any("error", err))
// 	// 		return nil, fmt.Errorf("failed to save city: %w", err)
// 	// 	}
// 	// } else {
// 	// 	cityID = city.ID
// 	// }
// 	if isNewConversation && itinerary.GeneralCityData.City != "" {
// 		// ... (your existing city saving logic) ...
// 		dbCity, err := l.cityRepo.FindCityByNameAndCountry(ctx, itinerary.GeneralCityData.City, itinerary.GeneralCityData.Country)
// 		if err != nil {
// 			return nil, currentSessionID, fmt.Errorf("failed to check city existence: %w", err)
// 		}
// 		if dbCity == nil {
// 			cityDetail := types.CityDetail{
// 				Name:            itinerary.GeneralCityData.City,
// 				Country:         itinerary.GeneralCityData.Country,
// 				StateProvince:   itinerary.GeneralCityData.StateProvince,
// 				AiSummary:       itinerary.GeneralCityData.Description,
// 				CenterLatitude:  itinerary.GeneralCityData.CenterLatitude,
// 				CenterLongitude: itinerary.GeneralCityData.CenterLongitude,
// 			}
// 			cityID, err = l.cityRepo.SaveCity(ctx, cityDetail)
// 			if err != nil {
// 				l.logger.ErrorContext(ctx, "Failed to save city", slog.Any("error", err))
// 				return nil, currentSessionID, fmt.Errorf("failed to save city: %w", err)
// 			}
// 		} else {
// 			cityID = dbCity.ID
// 		}

// 		for _, poi := range itinerary.PointsOfInterest { // General POIs
// 			existingPoi, err := l.poiRepo.FindPoiByNameAndCity(ctx, poi.Name, cityID)
// 			if err != nil {
// 				l.logger.WarnContext(ctx, "Failed to check POI existence", slog.String("poi_name", poi.Name), slog.Any("error", err))
// 				continue
// 			}
// 			if existingPoi == nil {
// 				_, err = l.poiRepo.SavePoi(ctx, poi, cityID)
// 				if err != nil {
// 					l.logger.WarnContext(ctx, "Failed to save POI", slog.String("poi_name", poi.Name), slog.Any("error", err))
// 					continue
// 				}
// 			}
// 		}
// 	} else if !isNewConversation {
// 		// For follow-up, ensure cityID is known. This might involve fetching it if not stored/passed.
// 		// Simplification: assume cityName is consistent and try to find cityID.
// 		if cityName != "" { // cityName is an input to GetPromptResponse
// 			//dbCity, err := l.cityRepo.FindCityByNameAndCountry(ctx, cityName, "") // Country might be needed for accuracy
// 			dbCity, err := l.cityRepo.FindCityByNameAndCountry(ctx, itinerary.GeneralCityData.City, itinerary.GeneralCityData.Country)

// 			if err == nil && dbCity != nil {
// 				cityID = dbCity.ID
// 			} else {
// 				l.logger.WarnContext(ctx, "Could not determine cityID for follow-up POI operations", slog.String("cityName", cityName))
// 				// Decide how to handle if cityID is crucial for DB ops for personalized POIs
// 			}
// 		}
// 	}

// 	// Save general POIs to the database if they don’t exist
// 	for _, poi := range itinerary.PointsOfInterest {
// 		existingPoi, err := l.poiRepo.FindPoiByNameAndCity(ctx, poi.Name, cityID)
// 		if err != nil {
// 			l.logger.WarnContext(ctx, "Failed to check POI existence", slog.String("poi_name", poi.Name), slog.Any("error", err))
// 			continue
// 		}
// 		if existingPoi == nil {
// 			_, err = l.poiRepo.SavePoi(ctx, poi, cityID)
// 			if err != nil {
// 				l.logger.WarnContext(ctx, "Failed to save POI", slog.String("poi_name", poi.Name), slog.Any("error", err))
// 				continue
// 			}
// 		}
// 	}

// 	// TODO Sort the POIs by distance from user location
// 	if userLocation != nil && cityID != uuid.Nil && len(itinerary.AIItineraryResponse.PointsOfInterest) > 0 {
// 		l.logger.InfoContext(ctx, "Attempting to save and sort personalised POIs by distance.")
// 		err := l.llmInteractionRepo.SaveLlmSuggestedPOIsBatch(ctx, rawPersonalisedPOIs, userID, profileID, llmInteractionIDForPersonalisedPOIs, cityID)
// 		if err != nil {
// 			l.logger.ErrorContext(ctx, "Failed to save personalised POIs", slog.Any("error", err))
// 			return nil, "", fmt.Errorf("failed to save personalised POIs: %w", err)
// 		} else {
// 			l.logger.InfoContext(ctx, "Successfully saved personalised POIs to the database.")
// 			l.logger.DebugContext(ctx, "Fetching LLM suggested POIs sorted by distance",
// 				slog.String("llm_interaction_id", llmInteractionIDForPersonalisedPOIs.String()),
// 				slog.String("cityID", cityID.String()))

// 			sortedPois, sortErr := l.llmInteractionRepo.GetLlmSuggestedPOIsByInteractionSortedByDistance(
// 				ctx, llmInteractionIDForPersonalisedPOIs, cityID, *userLocation,
// 			)
// 			if sortErr != nil {
// 				l.logger.ErrorContext(ctx, "Failed to fetch sorted LLM suggested POIs, using unsorted (but saved).", slog.Any("error", sortErr))
// 				itinerary.AIItineraryResponse.PointsOfInterest = rawPersonalisedPOIs
// 			} else {
// 				l.logger.InfoContext(ctx, "Successfully fetched and sorted LLM suggested POIs.", slog.Int("count", len(sortedPois)))
// 				itinerary.AIItineraryResponse.PointsOfInterest = sortedPois
// 			}

// 		}
// 		var personalisedPoiNamesToQuery []string
// 		tempPersonalisedPois := make([]types.POIDetail, 0, len(itinerary.AIItineraryResponse.PointsOfInterest))

// 		for _, pPoi := range itinerary.AIItineraryResponse.PointsOfInterest { // These are personalised POIs from LLM
// 			// Check if POI exists, save if not.
// 			// This step ensures that the POIs are in the DB before attempting to sort them via a DB query.
// 			existingPersPoi, err := l.poiRepo.FindPoiByNameAndCity(ctx, pPoi.Name, cityID)
// 			if err != nil && err != sql.ErrNoRows {
// 				l.logger.WarnContext(ctx, "Error checking personalised POI for saving", slog.String("name", pPoi.Name), slog.Any("error", err))
// 				tempPersonalisedPois = append(tempPersonalisedPois, pPoi) // Add unsaved POI to list to keep it
// 				continue
// 			}

// 			var dbPoiID uuid.UUID
// 			var dbPoiLat, dbPoiLon float64

// 			if existingPersPoi == nil {
// 				// POI doesn't exist, save it
// 				// The SavePoi function should ideally handle setting the location GEOMETRY from pPoi.Latitude and pPoi.Longitude
// 				savedID, saveErr := l.poiRepo.SavePoi(ctx, pPoi, cityID)
// 				if saveErr != nil {
// 					l.logger.WarnContext(ctx, "Failed to save new personalised POI", slog.String("name", pPoi.Name), slog.Any("error", saveErr))
// 					tempPersonalisedPois = append(tempPersonalisedPois, pPoi) // Add unsaved POI
// 					continue
// 				}
// 				dbPoiID = savedID
// 				dbPoiLat = pPoi.Latitude // Use original LLM lat/lon for the newly saved POI
// 				dbPoiLon = pPoi.Longitude
// 				l.logger.DebugContext(ctx, "Saved new personalised POI", slog.String("name", pPoi.Name), slog.String("id", dbPoiID.String()))
// 			} else {
// 				// POI already exists
// 				dbPoiID = existingPersPoi.ID
// 				dbPoiLat = existingPersPoi.Latitude // Use DB lat/lon for existing POI
// 				dbPoiLon = existingPersPoi.Longitude
// 				l.logger.DebugContext(ctx, "Found existing personalised POI", slog.String("name", pPoi.Name), slog.String("id", dbPoiID.String()))
// 			}

// 			// Add name for querying sorted list, and update the POI detail for potential use if sorting fails
// 			pPoi.ID = dbPoiID        // Update the POI in memory with its database ID
// 			pPoi.Latitude = dbPoiLat // Ensure using consistent lat/lon (from DB if exists, from LLM if new)
// 			pPoi.Longitude = dbPoiLon
// 			tempPersonalisedPois = append(tempPersonalisedPois, pPoi)
// 			personalisedPoiNamesToQuery = append(personalisedPoiNamesToQuery, pPoi.Name)
// 		}

// 		// Update the itinerary with POIs that have now been processed (saved/found and IDs updated)
// 		itinerary.AIItineraryResponse.PointsOfInterest = tempPersonalisedPois

// 		// If there are any names to query (meaning some POIs were processed successfully)
// 		if len(personalisedPoiNamesToQuery) > 0 {
// 			l.logger.DebugContext(ctx, "Fetching personalised POIs sorted by distance",
// 				slog.Any("names", personalisedPoiNamesToQuery),
// 				slog.String("cityID", cityID.String()),
// 				slog.Any("user_location", *userLocation),
// 			)
// 			sortedPersonalisedPois, sortErr := l.poiRepo.GetPOIsByNamesAndCitySortedByDistance(ctx, personalisedPoiNamesToQuery, cityID, *userLocation)
// 			if sortErr != nil {
// 				l.logger.ErrorContext(ctx, "Failed to fetch sorted personalised POIs, returning them unsorted (but potentially saved/updated).", slog.Any("error", sortErr))
// 				// itinerary.AIItineraryResponse.PointsOfInterest will remain as tempPersonalisedPois (unsorted but processed)
// 			} else {
// 				l.logger.InfoContext(ctx, "Successfully fetched and sorted personalised POIs by distance.", slog.Int("count", len(sortedPersonalisedPois)))
// 				// We need to be careful here. GetPOIsByNamesAndCitySortedByDistance returns a new list.
// 				// We should map the sorted POIs back to the itinerary's list or replace it carefully,
// 				// ensuring all relevant data (like LLM description) is preserved if the DB version is minimal.
// 				// For simplicity, if the sorted list contains all necessary fields, we can replace it.
// 				// If `types.POIDetail` from `GetPOIsByNamesAndCitySortedByDistance` is complete, this is fine:
// 				itinerary.AIItineraryResponse.PointsOfInterest = sortedPersonalisedPois
// 			}
// 		} else {
// 			l.logger.InfoContext(ctx, "No personalised POIs were successfully processed for sorting by distance.")
// 		}

// 	} else {
// 		if userLocation == nil {
// 			l.logger.InfoContext(ctx, "Skipping sorting of personalised POIs: user location not available.")
// 		}
// 		if cityID == uuid.Nil {
// 			l.logger.InfoContext(ctx, "Skipping sorting of personalised POIs: cityID is nil (city not found/saved).")
// 		}
// 		if len(itinerary.AIItineraryResponse.PointsOfInterest) == 0 {
// 			l.logger.InfoContext(ctx, "Skipping sorting of personalised POIs: no personalised POIs to sort.")
// 		}
// 	}

// 	l.logger.InfoContext(ctx, "Final itinerary ready",
// 		slog.String("itinerary_name", itinerary.AIItineraryResponse.ItineraryName),
// 		slog.Int("final_personalised_poi_count", len(itinerary.AIItineraryResponse.PointsOfInterest)))

// 	return &itinerary, *sessionIDInput, nil
// }

// func TruncateString(str string, num int) string {
// 	if len(str) > num {
// 		return str[0:num] + "..."
// 	}
// 	return str
// }

// func (l *LlmInteractiontServiceImpl) SaveItenerary(ctx context.Context, userID uuid.UUID, req types.BookmarkRequest) (uuid.UUID, error) {
// 	l.logger.InfoContext(ctx, "Attempting to bookmark interaction",
// 		slog.String("userID", userID.String()),
// 		slog.String("llmInteractionID", req.LlmInteractionID.String()),
// 		slog.String("title", req.Title))

// 	originalInteraction, err := l.llmInteractionRepo.GetInteractionByID(ctx, req.LlmInteractionID)
// 	if err != nil {
// 		l.logger.ErrorContext(ctx, "Failed to fetch original LLM interaction for bookmarking", slog.Any("error", err))
// 		return uuid.Nil, fmt.Errorf("could not retrieve original interaction: %w", err)
// 	}
// 	if originalInteraction == nil { // Or however you check for not found
// 		l.logger.WarnContext(ctx, "LLM interaction not found for bookmarking", slog.String("llmInteractionID", req.LlmInteractionID.String()))
// 		return uuid.Nil, fmt.Errorf("original interaction %s not found", req.LlmInteractionID)
// 	}
// 	markdownContent := originalInteraction.ResponseText

// 	//markdownContent := "Placeholder: Content from LLM Interaction " + req.LlmInteractionID.String() + ". This should be fetched from the DB."
// 	l.logger.WarnContext(ctx, "Using placeholder markdownContent for bookmark. Implement fetching original interaction text.",
// 		slog.String("llmInteractionID", req.LlmInteractionID.String()))

// 	var description sql.NullString
// 	if req.Description != nil {
// 		description.String = *req.Description
// 		description.Valid = true
// 	}

// 	isPublic := false // Default
// 	if req.IsPublic != nil {
// 		isPublic = *req.IsPublic
// 	}

// 	newBookmark := &types.UserSavedItinerary{
// 		UserID:                 userID,
// 		SourceLlmInteractionID: uuid.NullUUID{UUID: req.LlmInteractionID, Valid: true},
// 		Title:                  req.Title,
// 		Description:            description,
// 		MarkdownContent:        markdownContent,
// 		Tags:                   req.Tags, // pgx handles nil []string as NULL for TEXT[]
// 		IsPublic:               isPublic,
// 	}

// 	savedID, err := l.llmInteractionRepo.AddChatToBookmark(ctx, newBookmark)
// 	if err != nil {
// 		return uuid.Nil, err // Error already logged by repo
// 	}

// 	l.logger.InfoContext(ctx, "Successfully bookmarked interaction", slog.String("savedItineraryID", savedID.String()))
// 	return savedID, nil

// 	// Save the itinerary using the repository
// 	// itineraryID, err := l.llmInteractionRepo.SaveItinerary(ctx, itinerary)
// 	// if err != nil {
// 	// 	return uuid.Nil, fmt.Errorf("failed to save itinerary: %w", err)
// 	// }

// 	// l.logger.InfoContext(ctx, "Itinerary saved successfully", slog.String("itinerary_id", itineraryID.String()))
// 	// return itineraryID, nil
// }

// func (l *LlmInteractiontServiceImpl) RemoveItenerary(ctx context.Context, userID, itineraryID uuid.UUID) error {
// 	l.logger.InfoContext(ctx, "Attempting to remove chat from bookmark",
// 		slog.String("itineraryID", itineraryID.String()))
// 	if err := l.llmInteractionRepo.RemoveChatFromBookmark(ctx, userID, itineraryID); err != nil {
// 		l.logger.ErrorContext(ctx, "Failed to remove chat from bookmark", slog.Any("error", err))
// 		return fmt.Errorf("failed to remove chat from bookmark: %w", err)
// 	}
// 	l.logger.InfoContext(ctx, "Successfully removed chat from bookmark", slog.String("itineraryID", itineraryID.String()))
// 	return nil
// }
