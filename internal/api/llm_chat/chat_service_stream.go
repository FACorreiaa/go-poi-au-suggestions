package llmChat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genai"
)

// StreamEvent represents different types of streaming events
type StreamEvent struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	EventID   string      `json:"event_id"`
	IsFinal   bool        `json:"is_final,omitempty"`
}

// StreamEventType constants
const (
	EventTypeStart           = "start"
	EventTypeProgress        = "progress"
	EventTypeCityData        = "city_data"
	EventTypeGeneralPOI      = "general_poi"
	EventTypePersonalizedPOI = "personalized_poi"
	EventTypeItinerary       = "itinerary"
	EventTypeMessage         = "message"
	EventTypeError           = "error"
	EventTypeComplete        = "complete"
)

// StreamingResponse wraps the streaming channel and metadata
type StreamingResponse struct {
	SessionID uuid.UUID
	Stream    <-chan StreamEvent
	Cancel    context.CancelFunc
}

func (l *LlmInteractiontServiceImpl) sendEvent(ctx context.Context, ch chan<- StreamEvent, event StreamEvent) (sent bool) {
	// Add EventID and Timestamp if not already set
	if event.EventID == "" {
		event.EventID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	select {
	case <-ctx.Done(): // Check if the request context is cancelled FIRST
		l.logger.WarnContext(ctx, "Context cancelled, not sending stream event", slog.String("eventType", event.Type))
		return false
	default:
		// Context not done, try to send
		select {
		case ch <- event:
			return true // Successfully sent
		case <-ctx.Done(): // Check again, in case it was cancelled while trying to send
			l.logger.WarnContext(ctx, "Context cancelled while trying to send stream event", slog.String("eventType", event.Type))
			return false
		case <-time.After(2 * time.Second): // Timeout for sending
			l.logger.WarnContext(ctx, "Dropped stream event due to slow consumer or blocked channel (timeout)", slog.String("eventType", event.Type))
			return false
		}
	}
}

// getCityDescriptionPrompt generates a prompt for city data
func getCityDescriptionPrompt(cityName string) string {
	return fmt.Sprintf(`
        Provide detailed information about the city %s in JSON format with the following structure:
        {
            "city_name": "%s",
            "country": "Country name",
            "state_province": "State or province, if applicable",
            "description": "A detailed description of the city",
            "center_latitude": float64,
            "center_longitude": float64
        }
    `, cityName, cityName)
}

// getGeneralPOI generates a prompt for general POIs
func getGeneralPOI(cityName string) string {
	return fmt.Sprintf(`
        Provide a list of general points of interest for %s in JSON format with the following structure:
        {
            "points_of_interest": [
                {
                    "name": "POI name",
                    "category": "Category (e.g., Historical Site, Museum)",
                    "coordinates": {
                        "latitude": float64,
                        "longitude": float64
                    }
                }
            ]
        }
    `, cityName)
}

// getPersonalizedPOI generates a prompt for personalized POIs
func getPersonalizedPOI(interestNames []string, cityName, tagsPromptPart, userPrefs string) string {
	prompt := fmt.Sprintf(`
        Generate a personalized trip itinerary for %s, tailored to user interests [%s]. Include:
        1. An itinerary name.
        2. An overall description.
        3. A list of points of interest with name, category, coordinates, and detailed description.
        Format the response in JSON with the following structure:
        {
            "itinerary_name": "Name of the itinerary",
            "overall_description": "Description of the itinerary",
            "points_of_interest": [
                {
                    "name": "POI name",
                    "category": "Category",
                    "coordinates": {
                        "latitude": float64,
                        "longitude": float64
                    },
                    "description": "Detailed description of why this POI matches the user's interests"
                }
            ]
        }
    `, cityName, strings.Join(interestNames, ", "))
	if tagsPromptPart != "" {
		prompt += "\n" + tagsPromptPart
	}
	if userPrefs != "" {
		prompt += "\n" + userPrefs
	}
	return prompt
}

// streamingCityDataWorker generates city data with streaming updates
func (l *LlmInteractiontServiceImpl) streamingCityDataWorker(wg *sync.WaitGroup, ctx context.Context, cityName string, resultCh chan<- types.GenAIResponse, eventCh chan<- StreamEvent) {
	ctxWorker, span := otel.Tracer("LlmInteractionService").Start(ctx, "streamingCityDataWorker", trace.WithAttributes(
		attribute.String("city.name", cityName),
	))
	defer span.End()
	if wg != nil {
		defer wg.Done()
	}

	if !l.sendEvent(ctxWorker, eventCh, StreamEvent{Type: EventTypeProgress, Data: map[string]interface{}{"status": "generating_city_data", "progress": 10}}) {
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("context cancelled before sending initial progress for city data")} // Send to resultCh
		return
	}

	prompt := getCityDescriptionPrompt(cityName)
	startTime := time.Now()
	var responseText strings.Builder

	// Try streaming
	iter, err := l.aiClient.GenerateContentStream(ctx, prompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
	if err == nil {
		for resp, err := range iter {
			if err != nil {
				span.RecordError(err)
				l.sendEvent(ctx, eventCh, StreamEvent{
					Type:      EventTypeError,
					Error:     fmt.Sprintf("streaming city data error: %v", err),
					Timestamp: time.Now(),
					EventID:   uuid.New().String(),
				})
				resultCh <- types.GenAIResponse{Err: err}
				return
			}
			for _, cand := range resp.Candidates {
				if cand.Content != nil {
					for _, part := range cand.Content.Parts {
						if part.Text != "" {
							responseText.WriteString(string(part.Text))
							l.sendEvent(ctx, eventCh, StreamEvent{
								Type:      EventTypeCityData,
								Data:      map[string]string{"partial_city_data": responseText.String()},
								Timestamp: time.Now(),
								EventID:   uuid.New().String(),
							})
						}
					}
				}
			}
		}
	} else {
		// Fallback to non-streaming
		l.logger.WarnContext(ctx, "Streaming city data failed, falling back to non-streaming", slog.Any("error", err))
		response, err := l.aiClient.GenerateResponse(ctx, prompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
		if err != nil {
			span.RecordError(err)
			l.sendEvent(ctx, eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     fmt.Sprintf("failed to generate city data: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			resultCh <- types.GenAIResponse{Err: err}
			return
		}
		for _, cand := range response.Candidates {
			if cand.Content != nil {
				for _, part := range cand.Content.Parts {
					if part.Text != "" {
						responseText.WriteString(string(part.Text))
					}
				}
			}
		}
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeCityData,
			Data:      map[string]string{"partial_city_data": responseText.String()},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
	}
	if ctxWorker.Err() != nil {
		resultCh <- types.GenAIResponse{Err: fmt.Errorf("context cancelled during city data generation: %w", ctxWorker.Err())}
		return
	}

	fullText := responseText.String()
	if fullText == "" {
		err := fmt.Errorf("empty city data response")
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeError,
			Error:     err.Error(),
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		resultCh <- types.GenAIResponse{Err: err}
		return
	}

	// Save LLM interaction
	latencyMs := int(time.Since(startTime).Milliseconds())
	interaction := types.LlmInteraction{
		UserID:       uuid.Nil, // No specific user for city data
		Prompt:       prompt,
		ResponseText: fullText,
		ModelUsed:    model,
		LatencyMs:    latencyMs,
	}
	_, err = l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeError,
			Error:     fmt.Sprintf("failed to save city data interaction: %v", err),
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		resultCh <- types.GenAIResponse{Err: err}
		return
	}

	cleanTxt := cleanJSONResponse(fullText)
	var cityData struct {
		CityName        string  `json:"city_name"`
		StateProvince   *string `json:"state_province"`
		Country         string  `json:"country"`
		CenterLatitude  float64 `json:"center_latitude"`
		CenterLongitude float64 `json:"center_longitude"`
		Description     string  `json:"description"`
	}
	if err := json.Unmarshal([]byte(cleanTxt), &cityData); err != nil {
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeError,
			Error:     fmt.Sprintf("failed to parse city data JSON: %v", err),
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		resultCh <- types.GenAIResponse{Err: err}
		return
	}

	stateProvince := ""
	if cityData.StateProvince != nil {
		stateProvince = *cityData.StateProvince
	}

	result := types.GenAIResponse{
		City:            cityData.CityName,
		Country:         cityData.Country,
		StateProvince:   stateProvince,
		CityDescription: cityData.Description,
		Latitude:        cityData.CenterLatitude,
		Longitude:       cityData.CenterLongitude,
	}

	l.sendEvent(ctx, eventCh, StreamEvent{
		Type:      EventTypeCityData,
		Data:      result,
		Timestamp: time.Now(),
		EventID:   uuid.New().String(),
	})
	resultCh <- result
}

// streamingGeneralPOIWorker generates general POIs with streaming updates
func (l *LlmInteractiontServiceImpl) streamingGeneralPOIWorker(wg *sync.WaitGroup, ctx context.Context, cityName string, resultCh chan<- types.GenAIResponse, eventCh chan<- StreamEvent) {
	defer wg.Done()

	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "streamingGeneralPOIWorker", trace.WithAttributes(
		attribute.String("city.name", cityName),
	))
	defer span.End()

	l.sendEvent(ctx, eventCh, StreamEvent{
		Type:      EventTypeProgress,
		Data:      map[string]interface{}{"status": "generating_general_pois", "progress": 30},
		Timestamp: time.Now(),
		EventID:   uuid.New().String(),
	})

	prompt := getGeneralPOI(cityName)
	startTime := time.Now()
	var responseText strings.Builder

	// Try streaming
	iter, err := l.aiClient.GenerateContentStream(ctx, prompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
	if err == nil {
		for resp, err := range iter {
			if err != nil {
				span.RecordError(err)
				l.sendEvent(ctx, eventCh, StreamEvent{
					Type:      EventTypeError,
					Error:     fmt.Sprintf("streaming general POI error: %v", err),
					Timestamp: time.Now(),
					EventID:   uuid.New().String(),
				})
				resultCh <- types.GenAIResponse{Err: err}
				return
			}
			for _, cand := range resp.Candidates {
				if cand.Content != nil {
					for _, part := range cand.Content.Parts {
						if part.Text != "" {
							responseText.WriteString(string(part.Text))
							l.sendEvent(ctx, eventCh, StreamEvent{
								Type:      EventTypeGeneralPOI,
								Data:      map[string]string{"partial_poi_data": responseText.String()},
								Timestamp: time.Now(),
								EventID:   uuid.New().String(),
							})
						}
					}
				}
			}
		}
	} else {
		// Fallback to non-streaming
		l.logger.WarnContext(ctx, "Streaming general POIs failed, falling back to non-streaming", slog.Any("error", err))
		response, err := l.aiClient.GenerateResponse(ctx, prompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
		if err != nil {
			span.RecordError(err)
			l.sendEvent(ctx, eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     fmt.Sprintf("failed to generate general POIs: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			resultCh <- types.GenAIResponse{Err: err}
			return
		}
		for _, cand := range response.Candidates {
			if cand.Content != nil {
				for _, part := range cand.Content.Parts {
					if part.Text != "" {
						responseText.WriteString(string(part.Text))
					}
				}
			}
		}
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeGeneralPOI,
			Data:      map[string]string{"partial_poi_data": responseText.String()},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
	}

	fullText := responseText.String()
	if fullText == "" {
		err := fmt.Errorf("empty general POI response")
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeError,
			Error:     err.Error(),
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		resultCh <- types.GenAIResponse{Err: err}
		return
	}

	// Save LLM interaction
	latencyMs := int(time.Since(startTime).Milliseconds())
	interaction := types.LlmInteraction{
		UserID:       uuid.Nil, // No specific user for general POIs
		Prompt:       prompt,
		ResponseText: fullText,
		ModelUsed:    model,
		LatencyMs:    latencyMs,
	}
	_, err = l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeError,
			Error:     fmt.Sprintf("failed to save general POI interaction: %v", err),
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		resultCh <- types.GenAIResponse{Err: err}
		return
	}

	cleanTxt := cleanJSONResponse(fullText)
	var poiData struct {
		PointsOfInterest []types.POIDetail `json:"points_of_interest"`
	}
	if err := json.Unmarshal([]byte(cleanTxt), &poiData); err != nil {
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeError,
			Error:     fmt.Sprintf("failed to parse general POI JSON: %v", err),
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		resultCh <- types.GenAIResponse{Err: err}
		return
	}

	result := types.GenAIResponse{
		GeneralPOI: poiData.PointsOfInterest,
	}

	l.sendEvent(ctx, eventCh, StreamEvent{
		Type:      EventTypeGeneralPOI,
		Data:      result.GeneralPOI,
		Timestamp: time.Now(),
		EventID:   uuid.New().String(),
	})
	resultCh <- result
}

// streamingPersonalizedPOIWorker generates personalized POIs with streaming updates
func (l *LlmInteractiontServiceImpl) streamingPersonalizedPOIWorker(wg *sync.WaitGroup, ctx context.Context, cityName string, userID, profileID uuid.UUID, resultCh chan<- types.GenAIResponse, eventCh chan<- StreamEvent, interestNames []string, tagsPromptPart, userPrefs string) {
	defer wg.Done()

	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "streamingPersonalizedPOIWorker", trace.WithAttributes(
		attribute.String("city.name", cityName),
		attribute.String("user.id", userID.String()),
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l.sendEvent(ctx, eventCh, StreamEvent{
		Type:      EventTypeProgress,
		Data:      map[string]interface{}{"status": "generating_personalized_pois", "progress": 50},
		Timestamp: time.Now(),
		EventID:   uuid.New().String(),
	})

	startTime := time.Now()
	prompt := getPersonalizedPOI(interestNames, cityName, tagsPromptPart, userPrefs)
	var responseText strings.Builder

	// Try streaming
	iter, err := l.aiClient.GenerateContentStream(ctx, prompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
	if err == nil {
		for resp, err := range iter {
			if err != nil {
				span.RecordError(err)
				l.sendEvent(ctx, eventCh, StreamEvent{
					Type:      EventTypeError,
					Error:     fmt.Sprintf("streaming personalized POI error: %v", err),
					Timestamp: time.Now(),
					EventID:   uuid.New().String(),
				})
				resultCh <- types.GenAIResponse{Err: err}
				return
			}
			for _, cand := range resp.Candidates {
				if cand.Content != nil {
					for _, part := range cand.Content.Parts {
						if part.Text != "" {
							responseText.WriteString(string(part.Text))
							l.sendEvent(ctx, eventCh, StreamEvent{
								Type:      EventTypePersonalizedPOI,
								Data:      map[string]string{"partial_poi_data": responseText.String()},
								Timestamp: time.Now(),
								EventID:   uuid.New().String(),
							})
						}
					}
				}
			}
		}
	} else {
		// Fallback to non-streaming
		l.logger.WarnContext(ctx, "Streaming personalized POIs failed, falling back to non-streaming", slog.Any("error", err))
		response, err := l.aiClient.GenerateResponse(ctx, prompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
		if err != nil {
			span.RecordError(err)
			l.sendEvent(ctx, eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     fmt.Sprintf("failed to generate personalized POIs: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			resultCh <- types.GenAIResponse{Err: err}
			return
		}
		for _, cand := range response.Candidates {
			if cand.Content != nil {
				for _, part := range cand.Content.Parts {
					if part.Text != "" {
						responseText.WriteString(string(part.Text))
					}
				}
			}
		}
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypePersonalizedPOI,
			Data:      map[string]string{"partial_poi_data": responseText.String()},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
	}

	fullText := responseText.String()
	if fullText == "" {
		err := fmt.Errorf("empty personalized POI response")
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeError,
			Error:     err.Error(),
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		resultCh <- types.GenAIResponse{Err: err}
		return
	}

	// Save LLM interaction
	latencyMs := int(time.Since(startTime).Milliseconds())
	interaction := types.LlmInteraction{
		UserID:       userID,
		Prompt:       prompt,
		ResponseText: fullText,
		ModelUsed:    model,
		LatencyMs:    latencyMs,
	}
	savedInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeError,
			Error:     fmt.Sprintf("failed to save personalized POI interaction: %v", err),
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		resultCh <- types.GenAIResponse{Err: err}
		return
	}

	cleanTxt := cleanJSONResponse(fullText)
	var itineraryData struct {
		ItineraryName      string            `json:"itinerary_name"`
		OverallDescription string            `json:"overall_description"`
		PointsOfInterest   []types.POIDetail `json:"points_of_interest"`
	}
	if err := json.Unmarshal([]byte(cleanTxt), &itineraryData); err != nil {
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeError,
			Error:     fmt.Sprintf("failed to parse personalized POI JSON: %v", err),
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		resultCh <- types.GenAIResponse{Err: err}
		return
	}

	result := types.GenAIResponse{
		ItineraryName:        itineraryData.ItineraryName,
		ItineraryDescription: itineraryData.OverallDescription,
		PersonalisedPOI:      itineraryData.PointsOfInterest,
		LlmInteractionID:     savedInteractionID,
	}

	l.sendEvent(ctx, eventCh, StreamEvent{
		Type:      EventTypePersonalizedPOI,
		Data:      result,
		Timestamp: time.Now(),
		EventID:   uuid.New().String(),
	})
	resultCh <- result
}

func (l *LlmInteractiontServiceImpl) StartNewSessionStreamed(ctx context.Context, userID, profileID uuid.UUID, cityName, message string, userLocation *types.UserLocation) (*StreamingResponse, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "StartNewSessionStreamed", trace.WithAttributes(
		attribute.String("city.name", cityName),
		attribute.String("user.id", userID.String()),
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Starting new streamed chat session", slog.String("cityName", cityName), slog.String("userID", userID.String()))

	if message == "" {
		message = fmt.Sprintf("Plan a trip to %s", cityName)
	}

	// Create session early to persist partial data
	sessionID := uuid.New()
	eventCh := make(chan StreamEvent, 100)
	ctx, cancel := context.WithCancel(ctx)

	// Initialize session
	session := types.ChatSession{
		ID:     sessionID,
		UserID: userID,
		ConversationHistory: []types.ConversationMessage{
			{Role: "user", Content: message, Timestamp: time.Now()},
		},
		SessionContext: types.SessionContext{
			CityName:            cityName,
			ConversationSummary: fmt.Sprintf("Trip plan for %s", cityName),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    "active",
	}
	if err := l.llmInteractionRepo.CreateSession(ctx, session); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	go func() {
		defer close(eventCh)

		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeStart,
			Data:      map[string]string{"session_id": sessionID.String()},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})

		// Fetch user data
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeProgress,
			Data:      map[string]interface{}{"status": "fetching_user_data", "progress": 5},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		interests, searchProfile, tags, err := l.FetchUserData(ctx, userID, profileID)
		if err != nil {
			span.RecordError(err)
			l.sendEvent(ctx, eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     fmt.Sprintf("failed to fetch user data: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		interestNames, tagsPromptPart, userPrefs := l.PreparePromptData(interests, tags, searchProfile)

		// Channels for worker results
		cityDataCh := make(chan types.GenAIResponse, 1)
		generalPOICh := make(chan types.GenAIResponse, 1)
		personalizedPOICh := make(chan types.GenAIResponse, 1)

		var wg sync.WaitGroup
		wg.Add(3)

		go l.streamingCityDataWorker(&wg, ctx, cityName, cityDataCh, eventCh)
		go l.streamingGeneralPOIWorker(&wg, ctx, cityName, generalPOICh, eventCh)
		go l.streamingPersonalizedPOIWorker(&wg, ctx, cityName, userID, profileID, personalizedPOICh, eventCh, interestNames, tagsPromptPart, userPrefs)

		go func() {
			wg.Wait()
			close(cityDataCh)
			close(generalPOICh)
			close(personalizedPOICh)
		}()

		var itinerary types.AiCityResponse
		var llmInteractionID uuid.UUID
		var rawPersonalisedPOIs []types.POIDetail
		var errors []error

		for i := 0; i < 3; i++ {
			select {
			case result, ok := <-cityDataCh:
				if ok && result.Err != nil {
					errors = append(errors, result.Err)
				} else if ok {
					itinerary.GeneralCityData = types.GeneralCityData{
						City:            result.City,
						Country:         result.Country,
						StateProvince:   result.StateProvince,
						Description:     result.CityDescription,
						CenterLatitude:  result.Latitude,
						CenterLongitude: result.Longitude,
					}
				}
			case result, ok := <-generalPOICh:
				if ok && result.Err != nil {
					errors = append(errors, result.Err)
				} else if ok {
					itinerary.PointsOfInterest = result.GeneralPOI
				}
			case result, ok := <-personalizedPOICh:
				if ok && result.Err != nil {
					errors = append(errors, result.Err)
				} else if ok {
					itinerary.AIItineraryResponse = types.AIItineraryResponse{
						ItineraryName:      result.ItineraryName,
						OverallDescription: result.ItineraryDescription,
						PointsOfInterest:   result.PersonalisedPOI,
					}
					llmInteractionID = result.LlmInteractionID
					rawPersonalisedPOIs = result.PersonalisedPOI
				}
			case <-ctx.Done():
				l.sendEvent(ctx, eventCh, StreamEvent{
					Type:      EventTypeError,
					Error:     ctx.Err().Error(),
					Timestamp: time.Now(),
					EventID:   uuid.New().String(),
				})
				return
			}
		}

		if len(errors) > 0 {
			err := fmt.Errorf("worker errors: %v", errors)
			span.RecordError(err)
			l.sendEvent(ctx, eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     err.Error(),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		// Save data
		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeProgress,
			Data:      map[string]interface{}{"status": "saving_data", "progress": 80},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})

		cityID, err := l.HandleCityData(ctx, itinerary.GeneralCityData)
		if err != nil {
			span.RecordError(err)
			l.sendEvent(ctx, eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     fmt.Sprintf("failed to save city data: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		l.HandleGeneralPOIs(ctx, itinerary.PointsOfInterest, cityID)
		sortedPOIs, err := l.HandlePersonalisedPOIs(ctx, rawPersonalisedPOIs, cityID, userLocation, llmInteractionID, userID, profileID)
		if err != nil {
			span.RecordError(err)
			l.sendEvent(ctx, eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     fmt.Sprintf("failed to save personalised POIs: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}
		itinerary.AIItineraryResponse.PointsOfInterest = sortedPOIs

		// Update session with itinerary
		session.CurrentItinerary = &itinerary
		session.ConversationHistory = append(session.ConversationHistory, types.ConversationMessage{
			Role:        "assistant",
			Content:     fmt.Sprintf("Here's your personalized itinerary for %s", cityName),
			Timestamp:   time.Now(),
			MessageType: types.TypeItineraryResponse,
		})
		session.UpdatedAt = time.Now()
		if err := l.llmInteractionRepo.UpdateSession(ctx, session); err != nil {
			span.RecordError(err)
			l.sendEvent(ctx, eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     fmt.Sprintf("failed to update session: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeItinerary,
			Data:      itinerary,
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})

		l.sendEvent(ctx, eventCh, StreamEvent{
			Type:      EventTypeComplete,
			Data:      map[string]interface{}{"session_id": sessionID.String(), "progress": 100},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
			IsFinal:   true,
		})

		l.logger.InfoContext(ctx, "New session created and streamed",
			slog.String("session_id", sessionID.String()),
			slog.String("city_name", cityName))
		span.SetStatus(codes.Ok, "Session started successfully")
	}()

	return &StreamingResponse{
		SessionID: sessionID,
		Stream:    eventCh,
		Cancel:    cancel,
	}, nil
}
