package llmChat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
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

func (l *LlmInteractiontServiceImpl) sendEvent(ctx context.Context, ch chan<- types.StreamEvent, event types.StreamEvent) (sent bool) {
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
func (l *LlmInteractiontServiceImpl) streamingCityDataWorker(wg *sync.WaitGroup,
	ctx context.Context,
	cityName string, resultCh chan<- types.GenAIResponse,
	eventCh chan<- types.StreamEvent, userID uuid.UUID) {
	ctxWorker, span := otel.Tracer("LlmInteractionService").Start(ctx, "streamingCityDataWorker", trace.WithAttributes(
		attribute.String("city.name", cityName),
	))
	defer span.End()
	if wg != nil {
		defer wg.Done()
	}

	if !l.sendEvent(ctxWorker, eventCh, types.StreamEvent{Type: types.EventTypeProgress, Data: map[string]interface{}{"status": "generating_city_data", "progress": 10}}) {
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
				l.sendEvent(ctx, eventCh, types.StreamEvent{
					Type:      types.EventTypeError,
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
							l.sendEvent(ctx, eventCh, types.StreamEvent{
								Type:      types.EventTypeCityData,
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
			l.sendEvent(ctx, eventCh, types.StreamEvent{
				Type:      types.EventTypeError,
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
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeCityData,
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
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeError,
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
	_, err = l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeError,
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
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeError,
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

	l.sendEvent(ctx, eventCh, types.StreamEvent{
		Type:      types.EventTypeCityData,
		Data:      result,
		Timestamp: time.Now(),
		EventID:   uuid.New().String(),
	})
	resultCh <- result
}

// streamingGeneralPOIWorker generates general POIs with streaming updates
func (l *LlmInteractiontServiceImpl) streamingGeneralPOIWorker(wg *sync.WaitGroup,
	ctx context.Context, cityName string,
	resultCh chan<- types.GenAIResponse,
	eventCh chan<- types.StreamEvent,
	userID uuid.UUID) {
	defer wg.Done()

	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "streamingGeneralPOIWorker", trace.WithAttributes(
		attribute.String("city.name", cityName),
	))
	defer span.End()

	l.sendEvent(ctx, eventCh, types.StreamEvent{
		Type:      types.EventTypeProgress,
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
				l.sendEvent(ctx, eventCh, types.StreamEvent{
					Type:      types.EventTypeError,
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
							l.sendEvent(ctx, eventCh, types.StreamEvent{
								Type:      types.EventTypeGeneralPOI,
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
			l.sendEvent(ctx, eventCh, types.StreamEvent{
				Type:      types.EventTypeError,
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
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeGeneralPOI,
			Data:      map[string]string{"partial_poi_data": responseText.String()},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
	}

	fullText := responseText.String()
	if fullText == "" {
		err := fmt.Errorf("empty general POI response")
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeError,
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
		UserID:       userID, // No specific user for general POIs
		Prompt:       prompt,
		ResponseText: fullText,
		ModelUsed:    model,
		LatencyMs:    latencyMs,
	}
	_, err = l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeError,
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
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeError,
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

	l.sendEvent(ctx, eventCh, types.StreamEvent{
		Type:      types.EventTypeGeneralPOI,
		Data:      result.GeneralPOI,
		Timestamp: time.Now(),
		EventID:   uuid.New().String(),
	})
	resultCh <- result
}

// streamingPersonalizedPOIWorker generates personalized POIs with streaming updates
func (l *LlmInteractiontServiceImpl) streamingPersonalizedPOIWorker(wg *sync.WaitGroup, ctx context.Context, cityName string, userID, profileID uuid.UUID, resultCh chan<- types.GenAIResponse, eventCh chan<- types.StreamEvent, interestNames []string, tagsPromptPart, userPrefs string) {
	defer wg.Done()

	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "streamingPersonalizedPOIWorker", trace.WithAttributes(
		attribute.String("city.name", cityName),
		attribute.String("user.id", userID.String()),
		attribute.String("profile.id", profileID.String()),
	))
	defer span.End()

	l.sendEvent(ctx, eventCh, types.StreamEvent{
		Type:      types.EventTypeProgress,
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
				l.sendEvent(ctx, eventCh, types.StreamEvent{
					Type:      types.EventTypeError,
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
							l.sendEvent(ctx, eventCh, types.StreamEvent{
								Type:      types.EventTypePersonalizedPOI,
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
			l.sendEvent(ctx, eventCh, types.StreamEvent{
				Type:      types.EventTypeError,
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
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypePersonalizedPOI,
			Data:      map[string]string{"partial_poi_data": responseText.String()},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
	}

	fullText := responseText.String()
	if fullText == "" {
		err := fmt.Errorf("empty personalized POI response")
		span.RecordError(err)
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeError,
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
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeError,
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
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeError,
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

	l.sendEvent(ctx, eventCh, types.StreamEvent{
		Type:      types.EventTypePersonalizedPOI,
		Data:      result,
		Timestamp: time.Now(),
		EventID:   uuid.New().String(),
	})
	resultCh <- result
}

func (l *LlmInteractiontServiceImpl) StartNewSessionStreamed(ctx context.Context, userID, profileID uuid.UUID, cityName, message string, userLocation *types.UserLocation) (*types.StreamingResponse, error) {
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
	eventCh := make(chan types.StreamEvent, 100)
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

		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeStart,
			Data:      map[string]string{"session_id": sessionID.String()},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})

		// Fetch user data
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeProgress,
			Data:      map[string]interface{}{"status": "fetching_user_data", "progress": 5},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		interests, searchProfile, tags, err := l.FetchUserData(ctx, userID, profileID)
		if err != nil {
			span.RecordError(err)
			l.sendEvent(ctx, eventCh, types.StreamEvent{
				Type:      types.EventTypeError,
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

		go l.streamingCityDataWorker(&wg, ctx, cityName, cityDataCh, eventCh, userID)
		go l.streamingGeneralPOIWorker(&wg, ctx, cityName, generalPOICh, eventCh, userID)
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
				l.sendEvent(ctx, eventCh, types.StreamEvent{
					Type:      types.EventTypeError,
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
			l.sendEvent(ctx, eventCh, types.StreamEvent{
				Type:      types.EventTypeError,
				Error:     err.Error(),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		// Save data
		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeProgress,
			Data:      map[string]interface{}{"status": "saving_data", "progress": 80},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})

		cityID, err := l.HandleCityData(ctx, itinerary.GeneralCityData)
		if err != nil {
			span.RecordError(err)
			l.sendEvent(ctx, eventCh, types.StreamEvent{
				Type:      types.EventTypeError,
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
			l.sendEvent(ctx, eventCh, types.StreamEvent{
				Type:      types.EventTypeError,
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
			l.sendEvent(ctx, eventCh, types.StreamEvent{
				Type:      types.EventTypeError,
				Error:     fmt.Sprintf("failed to update session: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeItinerary,
			Data:      itinerary,
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})

		l.sendEvent(ctx, eventCh, types.StreamEvent{
			Type:      types.EventTypeComplete,
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

	return &types.StreamingResponse{
		SessionID: sessionID,
		Stream:    eventCh,
		Cancel:    cancel,
	}, nil
}

// ContinueSessionStreamed handles subsequent messages in an existing session and streams responses/updates.
func (l *LlmInteractiontServiceImpl) ContinueSessionStreamed(
	ctx context.Context, sessionID uuid.UUID,
	message string, userLocation *types.UserLocation,
	eventCh chan<- types.StreamEvent, // Output channel for events
) error { // Only returns error for critical setup failures
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "ContinueSessionStreamed", trace.WithAttributes(
		attribute.String("session.id", sessionID.String()),
		attribute.String("message", message),
	))
	defer span.End()

	l.logger.DebugContext(ctx, "Continuing streamed chat session", slog.String("sessionID", sessionID.String()), slog.String("message", message))

	// Helper to safely send events on the channel, respecting context cancellation
	sendEvt := func(event types.StreamEvent) bool { // Returns true if sent, false if context cancelled/timeout
		if event.EventID == "" {
			event.EventID = uuid.New().String()
		}
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}

		select {
		case <-ctx.Done():
			l.logger.WarnContext(ctx, "Context cancelled, not sending stream event", slog.String("eventType", event.Type))
			return false
		case eventCh <- event:
			return true
		case <-time.After(3 * time.Second):
			l.logger.WarnContext(ctx, "Timeout sending event to channel", slog.String("eventType", event.Type))
			return false
		}
	}

	// --- 1. Fetch Session & Basic Validation ---
	session, err := l.llmInteractionRepo.GetSession(ctx, sessionID)
	if err != nil {
		err = fmt.Errorf("failed to get session %s: %w", sessionID, err)
		sendEvt(types.StreamEvent{Type: types.EventTypeError, Error: err.Error(), IsFinal: true})
		return err
	}
	if session.Status != types.StatusActive {
		err = fmt.Errorf("session %s is not active (status: %s)", sessionID, session.Status)
		sendEvt(types.StreamEvent{Type: types.EventTypeError, Error: err.Error(), IsFinal: true})
		return err
	}
	sendEvt(types.StreamEvent{Type: "session_validated", Data: map[string]string{"status": "active"}})

	// --- 2. Fetch City ID ---
	cityData, err := l.cityRepo.FindCityByNameAndCountry(ctx, session.SessionContext.CityName, "")
	if err != nil || cityData == nil {
		if err == nil {
			err = fmt.Errorf("city '%s' not found for session %s", session.SessionContext.CityName, sessionID)
		} else {
			err = fmt.Errorf("failed to find city '%s' for session %s: %w", session.SessionContext.CityName, sessionID, err)
		}
		sendEvt(types.StreamEvent{Type: types.EventTypeError, Error: err.Error(), IsFinal: true})
		return err
	}
	cityID := cityData.ID
	sendEvt(types.StreamEvent{Type: types.EventTypeProgress, Data: map[string]interface{}{"status": "context_loaded", "city_id": cityID.String()}})

	// --- 3. Add User Message to History ---
	userMessage := types.ConversationMessage{
		ID: uuid.New(), Role: types.RoleUser, Content: message, Timestamp: time.Now(), MessageType: types.TypeModificationRequest,
	}
	if err := l.llmInteractionRepo.AddMessageToSession(ctx, sessionID, userMessage); err != nil {
		l.logger.WarnContext(ctx, "Failed to persist user message, continuing with in-memory history", slog.Any("error", err))
		span.RecordError(err, trace.WithAttributes(attribute.String("warning", "User message DB save failed")))
	}
	session.ConversationHistory = append(session.ConversationHistory, userMessage)

	// --- 4. Classify Intent ---
	intentStr, err := l.intentClassifier.Classify(ctx, message)
	if err != nil {
		err = fmt.Errorf("failed to classify intent for message '%s': %w", message, err)
		sendEvt(types.StreamEvent{Type: types.EventTypeError, Error: err.Error(), IsFinal: true})
		return err
	}
	l.logger.InfoContext(ctx, "Intent classified", slog.String("intent", intentStr))
	sendEvt(types.StreamEvent{Type: "intent_classified", Data: map[string]string{"intent": intentStr}})

	// --- 5. Handle Intent and Generate Response ---
	var finalResponseMessage string
	var assistantMessageType types.MessageType = types.TypeResponse
	itineraryModifiedByThisTurn := false

	switch intentStr { // Align with ContinueSession's string-based intents
	case "add_poi":
		sendEvt(types.StreamEvent{Type: types.EventTypeProgress, Data: "Processing: Adding Point of Interest..."})
		poiName := extractPOIName(message)
		isDuplicate := false
		for _, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
			if strings.EqualFold(poi.Name, poiName) {
				finalResponseMessage = fmt.Sprintf("It looks like '%s' is already in your itinerary.", poiName)
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			if poiName == "Unknown POI" || poiName == "" {
				finalResponseMessage = "I couldn't identify a specific place to add. Could you tell me the name of the POI?"
				assistantMessageType = types.TypeClarification
			} else {
				newPOI, genErr := l.generatePOIDataStream(ctx, poiName, session.SessionContext.CityName, userLocation, session.UserID, cityID, eventCh)
				if genErr != nil {
					finalResponseMessage = fmt.Sprintf("I had trouble finding details for '%s'. You might want to try rephrasing or checking the name. Error: %v", poiName, genErr)
					assistantMessageType = types.TypeError
				} else {
					session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = append(session.CurrentItinerary.AIItineraryResponse.PointsOfInterest, newPOI)
					finalResponseMessage = fmt.Sprintf("Okay, I've added '%s' to your itinerary for %s!", newPOI.Name, session.SessionContext.CityName)
					itineraryModifiedByThisTurn = true
				}
			}
		}

	case "remove_poi":
		sendEvt(types.StreamEvent{Type: types.EventTypeProgress, Data: "Processing: Removing Point of Interest..."})
		poiName := extractPOIName(message)
		var updatedPOIs []types.POIDetail
		found := false
		for _, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
			if !strings.Contains(strings.ToLower(poi.Name), strings.ToLower(poiName)) {
				updatedPOIs = append(updatedPOIs, poi)
			} else {
				found = true
			}
		}
		if found {
			session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = updatedPOIs
			finalResponseMessage = fmt.Sprintf("I've removed instances matching '%s' from your itinerary.", poiName)
			itineraryModifiedByThisTurn = true
		} else {
			finalResponseMessage = fmt.Sprintf("I couldn't find '%s' in your current itinerary.", poiName)
		}

	case "ask_question":
		sendEvt(types.StreamEvent{Type: types.EventTypeProgress, Data: "Processing: Answering your question..."})
		finalResponseMessage = "I’m here to help! For now, I’ll assume you’re asking about your trip. What specifically would you like to know?"

		// questionPrompt := fmt.Sprintf(
		// 	"The user is continuing a conversation about their trip to %s. Their current itinerary summary is: '%s'. Their current question/message is: '%s'. Provide a helpful and concise answer in a conversational tone. If the message isn't a clear question, infer what information they might be seeking or offer to help with itinerary modifications.",
		// 	session.SessionContext.CityName,
		// 	l.summarizeCurrentItinerary(session.CurrentItinerary),
		// 	message,
		// )
		// iter, err := l.aiClient.GenerateContentStream(ctx, questionPrompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.6)})
		// if err != nil {
		// 	finalResponseMessage = "I'm sorry, I had trouble processing your question right now."
		// 	assistantMessageType = types.TypeError
		// 	sendEvt(types.StreamEvent{Type: types.EventTypeError, Error: err.Error()})
		// } else {
		// 	var answerBuilder strings.Builder
		// 	iterErr := iter(func(chunk *genai.GenerateContentResponse) bool {
		// 		select {
		// 		case <-ctx.Done():
		// 			return false
		// 		default:
		// 		}
		// 		chunkContent := ""
		// 		for _, cand := range chunk.Candidates {
		// 			if cand.Content != nil {
		// 				for _, part := range cand.Content.Parts {
		// 					if textPart, ok := part.(genai.Text); ok {
		// 						chunkContent = string(textPart)
		// 					}
		// 				}
		// 			}
		// 		}
		// 		if chunkContent != "" {
		// 			answerBuilder.WriteString(chunkContent)
		// 			sendEvt(types.StreamEvent{Type: EventTypeMessage, Data: chunkContent})
		// 		}
		// 		return true
		// 	})
		// 	if iterErr != nil && iterErr != iterator.Done {
		// 		l.logger.WarnContext(ctx, "Error streaming question response", slog.Any("error", iterErr))
		// 		sendEvt(types.StreamEvent{Type: types.EventTypeError, Error: iterErr.Error()})
		// 	}
		// 	finalResponseMessage = answerBuilder.String()
		// 	if finalResponseMessage == "" {
		// 		finalResponseMessage = "I'm not sure how to respond to that. Could you rephrase or ask something else about your trip?"
		// 		assistantMessageType = types.TypeClarification
		// 	}
		//}

	case "replace_poi":
		sendEvt(types.StreamEvent{Type: types.EventTypeProgress, Data: "Processing: Replacing Point of Interest..."})
		if matches := regexp.MustCompile(`replace\s+(.+?)\s+with\s+(.+?)(?:\s+in\s+my\s+itinerary)?`).FindStringSubmatch(strings.ToLower(message)); len(matches) == 3 {
			oldPOI := matches[1]
			newPOIName := matches[2]
			for i, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
				if strings.Contains(strings.ToLower(poi.Name), oldPOI) {
					newPOI, err := l.generatePOIDataStream(ctx, newPOIName, session.SessionContext.CityName, userLocation, session.UserID, cityID, eventCh)
					if err != nil {
						finalResponseMessage = fmt.Sprintf("Could not replace %s with %s due to an error: %v", oldPOI, newPOIName, err)
						assistantMessageType = types.TypeError
					} else {
						session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[i] = newPOI
						finalResponseMessage = fmt.Sprintf("I've replaced %s with %s in your itinerary.", oldPOI, newPOIName)
						itineraryModifiedByThisTurn = true
					}
					break
				}
			}
			if finalResponseMessage == "" {
				finalResponseMessage = fmt.Sprintf("Could not find %s in your itinerary.", oldPOI)
			}
		} else {
			finalResponseMessage = "Please specify the replacement clearly (e.g., 'replace X with Y')."
			assistantMessageType = types.TypeClarification
		}

	default: // modify_itinerary
		sendEvt(types.StreamEvent{Type: types.EventTypeProgress, Data: "Processing: Updating itinerary..."})
		if matches := regexp.MustCompile(`replace\s+(.+?)\s+with\s+(.+?)(?:\s+in\s+my\s+itinerary)?`).FindStringSubmatch(strings.ToLower(message)); len(matches) == 3 {
			oldPOI := matches[1]
			newPOIName := matches[2]
			for i, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
				if strings.Contains(strings.ToLower(poi.Name), oldPOI) {
					newPOI, err := l.generatePOIData(ctx, newPOIName, session.SessionContext.CityName, userLocation, session.UserID, cityID)
					if err != nil {
						l.logger.ErrorContext(ctx, "Failed to generate POI data", slog.Any("error", err))
						span.RecordError(err)
						finalResponseMessage = fmt.Sprintf("Could not replace %s with %s due to an error.", oldPOI, newPOIName)
					} else {
						session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[i] = newPOI
						finalResponseMessage = fmt.Sprintf("I’ve replaced %s with %s in your itinerary.", oldPOI, newPOIName)
					}
					break
				}
			}
			if finalResponseMessage == "" {
				finalResponseMessage = fmt.Sprintf("Could not find %s in your itinerary.", oldPOI)
			}
		} else {
			finalResponseMessage = "I’ve noted your request to modify the itinerary. Please specify the changes (e.g., 'replace X with Y')."
		}
	}
	// 	conversationContext := l.buildConversationContext(session.ConversationHistory)
	// 	currentItinerarySummary := l.summarizeCurrentItinerary(session.CurrentItinerary)
	// 	updatePrompt := l.buildUpdatePrompt(session.SessionContext.CityName, message, conversationContext, currentItinerarySummary, types.Intent{Type: types.IntentModifyItinerary})

	// 	iter, err := l.aiClient.GenerateContentStream(ctx, updatePrompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)})
	// 	if err != nil {
	// 		finalResponseMessage = "I had trouble processing that itinerary update."
	// 		assistantMessageType = types.TypeError
	// 		sendEvt(types.StreamEvent{Type: types.EventTypeError, Error: err.Error()})
	// 	} else {
	// 		var fullResponseBuilder strings.Builder
	// 		iterErr := iter(func(chunk *genai.GenerateContentResponse) bool {
	// 			select {
	// 			case <-ctx.Done():
	// 				return false
	// 			default:
	// 			}
	// 			chunkContent := ""
	// 			for _, cand := range chunk.Candidates {
	// 				if cand.Content != nil {
	// 					for _, part := range cand.Content.Parts {
	// 						if textPart, ok := part.(genai.Text); ok {
	// 							chunkContent = string(textPart)
	// 						}
	// 					}
	// 				}
	// 			}
	// 			if chunkContent != "" {
	// 				fullResponseBuilder.WriteString(chunkContent)
	// 				sendEvt(types.StreamEvent{Type: EventTypeMessage, Data: chunkContent})
	// 			}
	// 			return true
	// 		})
	// 		if iterErr != nil && iterErr != iterator.Done {
	// 			l.logger.WarnContext(ctx, "Error streaming itinerary update", slog.Any("error", iterErr))
	// 			sendEvt(types.StreamEvent{Type: types.EventTypeError, Error: iterErr.Error()})
	// 		}
	// 		if ctx.Err() != nil {
	// 			return ctx.Err()
	// 		}

	// 		parsedItinerary, parsedMsg, parseErr := l.parseUpdateResponse(fullResponseBuilder.String(), session.CurrentItinerary)
	// 		if parseErr != nil {
	// 			finalResponseMessage = "I received an update, but had trouble structuring it: " + fullResponseBuilder.String()
	// 			assistantMessageType = types.TypeError
	// 		} else {
	// 			session.CurrentItinerary = parsedItinerary
	// 			finalResponseMessage = parsedMsg
	// 			itineraryModifiedByThisTurn = true
	// 		}
	// 	}
	// }

	// --- 6. Post-Modification Processing (Sorting, Saving Session) ---
	if itineraryModifiedByThisTurn && userLocation != nil && userLocation.UserLat != 0 && userLocation.UserLon != 0 && session.CurrentItinerary != nil {
		sendEvt(types.StreamEvent{Type: types.EventTypeProgress, Data: "Sorting updated POIs by distance..."})
		// Save new POIs to DB to ensure they have valid IDs
		for i, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
			if poi.ID == uuid.Nil {
				dbPoiID, saveErr := l.llmInteractionRepo.SaveSinglePOI(ctx, poi, session.UserID, cityID, poi.LlmInteractionID)
				if saveErr != nil {
					l.logger.WarnContext(ctx, "Failed to save new POI", slog.String("name", poi.Name), slog.Any("error", saveErr))
					continue
				}
				session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[i].ID = dbPoiID
			}
		}

		var currentPOIIDs []uuid.UUID
		for _, p := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
			if p.ID != uuid.Nil {
				currentPOIIDs = append(currentPOIIDs, p.ID)
			}
		}
		if (intentStr == "add_poi" || intentStr == "modify_itinerary") && userLocation != nil && userLocation.UserLat != 0 && userLocation.UserLon != 0 {
			sortedPOIs, err := l.llmInteractionRepo.GetPOIsBySessionSortedByDistance(ctx, sessionID, cityID, *userLocation)
			if err != nil {
				l.logger.WarnContext(ctx, "Failed to sort POIs by distance", slog.Any("error", err))
				span.RecordError(err)
			} else {
				session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = sortedPOIs
				l.logger.InfoContext(ctx, "POIs sorted by distance",
					slog.Int("poi_count", len(sortedPOIs)))
				span.SetAttributes(attribute.Int("sorted_pois.count", len(sortedPOIs)))
			}
		}
	}

	// Add assistant's final response to history
	assistantMessage := types.ConversationMessage{
		ID: uuid.New(), Role: types.RoleAssistant, Content: finalResponseMessage, Timestamp: time.Now(), MessageType: assistantMessageType,
	}
	if err := l.llmInteractionRepo.AddMessageToSession(ctx, sessionID, assistantMessage); err != nil {
		l.logger.WarnContext(ctx, "Failed to save assistant message", slog.Any("error", err))
	}
	session.ConversationHistory = append(session.ConversationHistory, assistantMessage)

	// Update session in the database
	session.UpdatedAt = time.Now()
	session.ExpiresAt = time.Now().Add(24 * time.Hour)
	if err := l.llmInteractionRepo.UpdateSession(ctx, *session); err != nil {
		err = fmt.Errorf("failed to update session %s: %w", sessionID, err)
		sendEvt(types.StreamEvent{Type: types.EventTypeError, Error: err.Error(), IsFinal: true})
		return err
	}

	// --- 7. Send Final Itinerary and Completion Event ---
	sendEvt(types.StreamEvent{
		Type:      types.EventTypeItinerary,
		Data:      session.CurrentItinerary,
		Message:   finalResponseMessage,
		Timestamp: time.Now(),
	})
	sendEvt(types.StreamEvent{Type: types.EventTypeComplete, Data: "Turn completed.", IsFinal: true})

	l.logger.InfoContext(ctx, "Streamed session continued", slog.String("sessionID", sessionID.String()), slog.String("intent", intentStr))
	return nil
}

// generatePOIDataStream queries the LLM for POI details and streams updates
func (l *LlmInteractiontServiceImpl) generatePOIDataStream(
	ctx context.Context, poiName, cityName string,
	userLocation *types.UserLocation, userID, cityID uuid.UUID,
	eventCh chan<- types.StreamEvent,
) (types.POIDetail, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "generatePOIDataStream",
		trace.WithAttributes(attribute.String("poi.name", poiName), attribute.String("city.name", cityName)))
	defer span.End()

	// Use the same prompt structure as ContinueSession
	prompt := generatedContinuedConversationPrompt(poiName, cityName)
	config := &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.2)} // Lower temp for factual data

	_, err := l.aiClient.GenerateContentStream(ctx, prompt, config)
	if err != nil {
		l.sendEvent(ctx, eventCh, types.StreamEvent{Type: types.EventTypeError, Error: fmt.Sprintf("Failed to generate POI data for '%s': %v", poiName, err)})
		return types.POIDetail{}, fmt.Errorf("AI stream init failed for POI '%s': %w", poiName, err)
	}

	var responseTextBuilder strings.Builder
	l.sendEvent(ctx, eventCh, types.StreamEvent{Type: types.EventTypeProgress, Data: map[string]string{"status": fmt.Sprintf("Getting details for %s...", poiName)}})

	// iterErr := iter(func(chunk *genai.GenerateContentResponse) bool {
	// 	select {
	// 	case <-ctx.Done():
	// 		return false
	// 	default:
	// 	}
	// 	chunkContent := ""
	// 	for _, cand := range chunk.Candidates {
	// 		if cand.Content != nil {
	// 			for _, part := range cand.Content.Parts {
	// 				if txtPart, ok := part.(genai.Text); ok {
	// 					chunkContent += string(txtPart)
	// 				}
	// 			}
	// 		}
	// 	}
	// 	if chunkContent != "" {
	// 		responseTextBuilder.WriteString(chunkContent)
	// 		l.sendEvent(ctx, eventCh, types.StreamEvent{Type: "poi_detail_chunk", Data: map[string]string{"poi_name": poiName, "chunk": chunkContent}})
	// 	}
	// 	return true
	// })

	// if err != nil && err != iter.Done {
	// 	l.sendEvent(ctx, eventCh, types.StreamEvent{Type: types.EventTypeError, Error: fmt.Sprintf("Streaming failed for POI '%s': %v", poiName, iterErr)})
	// 	return types.POIDetail{}, fmt.Errorf("streaming POI details for '%s' failed: %w", poiName, iterErr)
	// }
	if ctx.Err() != nil {
		l.sendEvent(ctx, eventCh, types.StreamEvent{Type: types.EventTypeError, Error: ctx.Err().Error()})
		return types.POIDetail{}, fmt.Errorf("context cancelled during POI detail generation: %w", ctx.Err())
	}

	fullText := responseTextBuilder.String()
	if fullText == "" {
		l.sendEvent(ctx, eventCh, types.StreamEvent{Type: types.EventTypeError, Error: fmt.Sprintf("Empty response for POI '%s'", poiName)})
		return types.POIDetail{Name: poiName, DescriptionPOI: "Details not found."}, fmt.Errorf("empty response for POI details '%s'", poiName)
	}

	// Save LLM interaction
	interaction := types.LlmInteraction{
		UserID:       userID,
		Prompt:       prompt,
		ResponseText: fullText,
		ModelUsed:    model,
	}
	llmInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to save LLM interaction", slog.Any("error", err))
		span.RecordError(err)
	}

	// Parse response
	cleanJSON := cleanJSONResponse(fullText)
	var poiData types.POIDetail
	if err := json.Unmarshal([]byte(cleanJSON), &poiData); err != nil || poiData.Name == "" {
		l.logger.WarnContext(ctx, "Invalid POI data from LLM", slog.String("response", fullText), slog.Any("error", err))
		poiData = types.POIDetail{
			ID:             uuid.New(),
			Name:           poiName,
			Category:       "Attraction",
			DescriptionPOI: fmt.Sprintf("Added %s based on user request, but detailed data not available.", poiName),
		}
	}
	if poiData.ID == uuid.Nil {
		poiData.ID = uuid.New()
	}
	poiData.LlmInteractionID = llmInteractionID
	poiData.City = cityName

	// Save POI to database
	dbPoiID, err := l.llmInteractionRepo.SaveSinglePOI(ctx, poiData, userID, cityID, llmInteractionID)
	if err != nil {
		l.logger.WarnContext(ctx, "Failed to save POI to database", slog.Any("error", err))
		span.RecordError(err)
	} else {
		poiData.ID = dbPoiID
	}

	// Calculate distance
	if userLocation != nil && userLocation.UserLat != 0 && userLocation.UserLon != 0 && poiData.Latitude != 0 && poiData.Longitude != 0 {
		distance, err := l.llmInteractionRepo.CalculateDistancePostGIS(ctx, userLocation.UserLat, userLocation.UserLon, poiData.Latitude, poiData.Longitude)
		if err != nil {
			l.logger.WarnContext(ctx, "Failed to calculate distance", slog.Any("error", err))
			span.RecordError(err)
		} else {
			poiData.Distance = distance
		}
	}

	l.sendEvent(ctx, eventCh, types.StreamEvent{Type: "poi_detail_complete", Data: poiData})
	return poiData, nil
}
