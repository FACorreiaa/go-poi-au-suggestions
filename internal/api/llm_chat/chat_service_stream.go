package llmChat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"google.golang.org/genai"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StreamEvent represents different types of streaming events
type StreamEvent struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	EventID   string      `json:"event_id"`
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

// func (l *LlmInteractiontServiceImpl) sendEvent(eventCh chan<- StreamEvent, event StreamEvent) {
// 	select {
// 	case eventCh <- event:
// 		// Event sent successfully
// 	default:
// 		l.logger.Warn("streaming channel closed or full, event not sent", slog.Any("event", event))
// 	}
// }

// // streamingCityDataWorker generates city data with streaming updates
// func (l *LlmInteractiontServiceImpl) streamingCityDataWorker(wg *sync.WaitGroup, ctx context.Context, cityName string, resultCh chan<- types.GenAIResponse, eventCh chan<- StreamEvent) {
// 	defer wg.Done()

// 	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "streamingCityDataWorker", trace.WithAttributes(
// 		attribute.String("city.name", cityName),
// 	))
// 	defer span.End()

// 	l.sendEvent(eventCh, StreamEvent{
// 		Type:      EventTypeProgress,
// 		Data:      map[string]string{"status": "generating_city_data"},
// 		Timestamp: time.Now(),
// 		EventID:   uuid.New().String(),
// 	})

// 	prompt := getCityDescriptionPrompt(cityName)
// 	iter, err := l.aiClient.GenerateContentStream(ctx, prompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
// 	if err != nil {
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{
// 			Type:      EventTypeError,
// 			Error:     fmt.Sprintf("failed to stream city data: %v", err),
// 			Timestamp: time.Now(),
// 			EventID:   uuid.New().String(),
// 		})
// 		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to stream city data: %w", err)}
// 		return
// 	}

// 	var responseText strings.Builder
// 	for {
// 		chunk, err := iter.Next()
// 		if err == iterator.Done {
// 			break
// 		}
// 		if err != nil {
// 			span.RecordError(err)
// 			l.sendEvent(eventCh, StreamEvent{
// 				Type:      EventTypeError,
// 				Error:     fmt.Sprintf("streaming error: %v", err),
// 				Timestamp: time.Now(),
// 				EventID:   uuid.New().String(),
// 			})
// 			resultCh <- types.GenAIResponse{Err: err}
// 			return
// 		}

// 		for _, cand := range chunk.Candidates {
// 			if cand.Content != nil {
// 				for _, part := range cand.Content.Parts {
// 					if txt, ok := part.(genai.Text); ok {
// 						responseText.WriteString(string(txt))
// 						l.sendEvent(eventCh, StreamEvent{
// 							Type:      EventTypeCityData,
// 							Data:      map[string]string{"partial_city_data": responseText.String()},
// 							Timestamp: time.Now(),
// 							EventID:   uuid.New().String(),
// 						})
// 					}
// 				}
// 			}
// 		}
// 	}

// 	fullText := responseText.String()
// 	if fullText == "" {
// 		err := fmt.Errorf("empty city data response")
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 		resultCh <- types.GenAIResponse{Err: err}
// 		return
// 	}

// 	cleanTxt := cleanJSONResponse(fullText)
// 	var cityData struct {
// 		CityName        string  `json:"city_name"`
// 		StateProvince   *string `json:"state_province"`
// 		Country         string  `json:"country"`
// 		CenterLatitude  float64 `json:"center_latitude"`
// 		CenterLongitude float64 `json:"center_longitude"`
// 		Description     string  `json:"description"`
// 	}
// 	if err := json.Unmarshal([]byte(cleanTxt), &cityData); err != nil {
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 		resultCh <- types.GenAIResponse{Err: err}
// 		return
// 	}

// 	stateProvince := ""
// 	if cityData.StateProvince != nil {
// 		stateProvince = *cityData.StateProvince
// 	}

// 	result := types.GenAIResponse{
// 		City:            cityData.CityName,
// 		Country:         cityData.Country,
// 		StateProvince:   stateProvince,
// 		CityDescription: cityData.Description,
// 		Latitude:        cityData.CenterLatitude,
// 		Longitude:       cityData.CenterLongitude,
// 	}

// 	l.sendEvent(eventCh, StreamEvent{
// 		Type:      EventTypeCityData,
// 		Data:      result,
// 		Timestamp: time.Now(),
// 		EventID:   uuid.New().String(),
// 	})
// 	resultCh <- result
// }

// // streamingGeneralPOIWorker generates general POIs with streaming updates
// func (l *LlmInteractiontServiceImpl) streamingCityDataWorker(wg *sync.WaitGroup, ctx context.Context, cityName string, resultCh chan<- types.GenAIResponse, eventCh chan<- StreamEvent) {
// 	defer wg.Done()

// 	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "streamingCityDataWorker", trace.WithAttributes(
// 		attribute.String("city.name", cityName),
// 	))
// 	defer span.End()

// 	l.sendEvent(eventCh, StreamEvent{
// 		Type:      EventTypeProgress,
// 		Data:      map[string]string{"status": "generating_city_data"},
// 		Timestamp: time.Now(),
// 		EventID:   uuid.New().String(),
// 	})

// 	prompt := getCityDescriptionPrompt(cityName)
// 	iter, err := l.aiClient.GenerateContentStream(ctx, prompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
// 	if err != nil {
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{
// 			Type:      EventTypeError,
// 			Error:     fmt.Sprintf("failed to stream city data: %v", err),
// 			Timestamp: time.Now(),
// 			EventID:   uuid.New().String(),
// 		})
// 		resultCh <- types.GenAIResponse{Err: fmt.Errorf("failed to stream city data: %w", err)}
// 		return
// 	}

// 	var responseText strings.Builder
// 	for {
// 		chunk, err := iter.Next()
// 		if err == iterator.Done {
// 			break
// 		}
// 		if err != nil {
// 			span.RecordError(err)
// 			l.sendEvent(eventCh, StreamEvent{
// 				Type:      EventTypeError,
// 				Error:     fmt.Sprintf("streaming error: %v", err),
// 				Timestamp: time.Now(),
// 				EventID:   uuid.New().String(),
// 			})
// 			resultCh <- types.GenAIResponse{Err: err}
// 			return
// 		}

// 		for _, cand := range chunk.Candidates {
// 			if cand.Content != nil {
// 				for _, part := range cand.Content.Parts {
// 					if txt, ok := part.(genai.Text); ok {
// 						responseText.WriteString(string(txt))
// 						l.sendEvent(eventCh, StreamEvent{
// 							Type:      EventTypeCityData,
// 							Data:      map[string]string{"partial_city_data": responseText.String()},
// 							Timestamp: time.Now(),
// 							EventID:   uuid.New().String(),
// 						})
// 					}
// 				}
// 			}
// 		}
// 	}

// 	fullText := responseText.String()
// 	if fullText == "" {
// 		err := fmt.Errorf("empty city data response")
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 		resultCh <- types.GenAIResponse{Err: err}
// 		return
// 	}

// 	cleanTxt := cleanJSONResponse(fullText)
// 	var cityData struct {
// 		CityName        string  `json:"city_name"`
// 		StateProvince   *string `json:"state_province"`
// 		Country         string  `json:"country"`
// 		CenterLatitude  float64 `json:"center_latitude"`
// 		CenterLongitude float64 `json:"center_longitude"`
// 		Description     string  `json:"description"`
// 	}
// 	if err := json.Unmarshal([]byte(cleanTxt), &cityData); err != nil {
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 		resultCh <- types.GenAIResponse{Err: err}
// 		return
// 	}

// 	stateProvince := ""
// 	if cityData.StateProvince != nil {
// 		stateProvince = *cityData.StateProvince
// 	}

// 	result := types.GenAIResponse{
// 		City:            cityData.CityName,
// 		Country:         cityData.Country,
// 		StateProvince:   stateProvince,
// 		CityDescription: cityData.Description,
// 		Latitude:        cityData.CenterLatitude,
// 		Longitude:       cityData.CenterLongitude,
// 	}

// 	l.sendEvent(eventCh, StreamEvent{
// 		Type:      EventTypeCityData,
// 		Data:      result,
// 		Timestamp: time.Now(),
// 		EventID:   uuid.New().String(),
// 	})
// 	resultCh <- result
// }

// func (l *LlmInteractiontServiceImpl) streamingGeneralPOIWorker(wg *sync.WaitGroup, ctx context.Context, cityName string, resultCh chan<- types.GenAIResponse, eventCh chan<- StreamEvent) {
// 	defer wg.Done()

// 	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "streamingGeneralPOIWorker", trace.WithAttributes(
// 		attribute.String("city.name", cityName),
// 	))
// 	defer span.End()

// 	l.sendEvent(eventCh, StreamEvent{
// 		Type:      EventTypeProgress,
// 		Data:      map[string]string{"status": "generating_general_pois"},
// 		Timestamp: time.Now(),
// 		EventID:   uuid.New().String(),
// 	})

// 	prompt := getGeneralPOI(cityName)
// 	iter, err := l.aiClient.GenerateContentStream(ctx, prompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
// 	if err != nil {
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 		resultCh <- types.GenAIResponse{Err: err}
// 		return
// 	}

// 	var responseText strings.Builder
// 	for {
// 		chunk, err := iter.Next()
// 		if err == iterator.Done {
// 			break
// 		}
// 		if err != nil {
// 			span.RecordError(err)
// 			l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 			resultCh <- types.GenAIResponse{Err: err}
// 			return
// 		}

// 		for _, cand := range chunk.Candidates {
// 			if cand.Content != nil {
// 				for _, part := range cand.Content.Parts {
// 					if txt, ok := part.(genai.Text); ok {
// 						responseText.WriteString(string(txt))
// 						l.sendEvent(eventCh, StreamEvent{
// 							Type:      EventTypeGeneralPOI,
// 							Data:      map[string]string{"partial_poi_data": responseText.String()},
// 							Timestamp: time.Now(),
// 							EventID:   uuid.New().String(),
// 						})
// 					}
// 				}
// 			}
// 		}
// 	}

// 	fullText := responseText.String()
// 	if fullText == "" {
// 		err := fmt.Errorf("empty general POI response")
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 		resultCh <- types.GenAIResponse{Err: err}
// 		return
// 	}

// 	cleanTxt := cleanJSONResponse(fullText)
// 	var poiData struct {
// 		PointsOfInterest []types.POIDetail `json:"points_of_interest"`
// 	}
// 	if err := json.Unmarshal([]byte(cleanTxt), &poiData); err != nil {
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 		resultCh <- types.GenAIResponse{Err: err}
// 		return
// 	}

// 	result := types.GenAIResponse{
// 		GeneralPOI: poiData.PointsOfInterest,
// 	}

// 	l.sendEvent(eventCh, StreamEvent{
// 		Type:      EventTypeGeneralPOI,
// 		Data:      result.GeneralPOI,
// 		Timestamp: time.Now(),
// 		EventID:   uuid.New().String(),
// 	})
// 	resultCh <- result
// }

// func (l *LlmInteractiontServiceImpl) streamingPersonalizedPOIWorker(wg *sync.WaitGroup, ctx context.Context, cityName string, userID, profileID uuid.UUID, resultCh chan<- types.GenAIResponse, eventCh chan<- StreamEvent, interestNames []string, tagsPromptPart, userPrefs string) {
// 	defer wg.Done()

// 	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "streamingPersonalizedPOIWorker", trace.WithAttributes(
// 		attribute.String("city.name", cityName),
// 		attribute.String("user.id", userID.String()),
// 		attribute.String("profile.id", profileID.String()),
// 	))
// 	defer span.End()

// 	l.sendEvent(eventCh, StreamEvent{
// 		Type:      EventTypeProgress,
// 		Data:      map[string]string{"status": "generating_personalized_pois"},
// 		Timestamp: time.Now(),
// 		EventID:   uuid.New().String(),
// 	})

// 	startTime := time.Now()
// 	prompt := getPersonalizedPOI(interestNames, cityName, tagsPromptPart, userPrefs)
// 	iter, err := l.aiClient.GenerateContentStream(ctx, prompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
// 	if err != nil {
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 		resultCh <- types.GenAIResponse{Err: err}
// 		return
// 	}

// 	var responseText strings.Builder
// 	for {
// 		chunk, err := iter.Next()
// 		if err == iterator.Done {
// 			break
// 		}
// 		if err != nil {
// 			span.RecordError(err)
// 			l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 			resultCh <- types.GenAIResponse{Err: err}
// 			return
// 		}

// 		for _, cand := range chunk.Candidates {
// 			if cand.Content != nil {
// 				for _, part := range cand.Content.Parts {
// 					if txt, ok := part.(genai.Text); ok {
// 						responseText.WriteString(string(txt))
// 						l.sendEvent(eventCh, StreamEvent{
// 							Type:      EventTypePersonalizedPOI,
// 							Data:      map[string]string{"partial_poi_data": responseText.String()},
// 							Timestamp: time.Now(),
// 							EventID:   uuid.New().String(),
// 						})
// 					}
// 				}
// 			}
// 		}
// 	}

// 	fullText := responseText.String()
// 	if fullText == "" {
// 		err := fmt.Errorf("empty personalized POI response")
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 		resultCh <- types.GenAIResponse{Err: err}
// 		return
// 	}

// 	cleanTxt := cleanJSONResponse(fullText)
// 	var itineraryData struct {
// 		ItineraryName      string            `json:"itinerary_name"`
// 		OverallDescription string            `json:"overall_description"`
// 		PointsOfInterest   []types.POIDetail `json:"points_of_interest"`
// 	}
// 	if err := json.Unmarshal([]byte(cleanTxt), &itineraryData); err != nil {
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 		resultCh <- types.GenAIResponse{Err: err}
// 		return
// 	}

// 	interaction := types.LlmInteraction{
// 		UserID:       userID,
// 		Prompt:       prompt,
// 		ResponseText: fullText,
// 		ModelUsed:    model,
// 		LatencyMs:    int(time.Since(startTime).Milliseconds()),
// 	}
// 	savedInteractionID, err := l.llmInteractionRepo.SaveInteraction(ctx, interaction)
// 	if err != nil {
// 		span.RecordError(err)
// 		l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 		resultCh <- types.GenAIResponse{Err: err}
// 		return
// 	}

// 	result := types.GenAIResponse{
// 		ItineraryName:        itineraryData.ItineraryName,
// 		ItineraryDescription: itineraryData.OverallDescription,
// 		PersonalisedPOI:      itineraryData.PointsOfInterest,
// 		LlmInteractionID:     savedInteractionID,
// 	}

// 	l.sendEvent(eventCh, StreamEvent{
// 		Type:      EventTypePersonalizedPOI,
// 		Data:      result,
// 		Timestamp: time.Now(),
// 		EventID:   uuid.New().String(),
// 	})
// 	resultCh <- result
// }

// func (l *LlmInteractiontServiceImpl) StartNewSessionStreamed(ctx context.Context, userID, profileID uuid.UUID, cityName, message string, userLocation *types.UserLocation) (*StreamingResponse, error) {
// 	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "StartNewSessionStreamed", trace.WithAttributes(
// 		attribute.String("city.name", cityName),
// 		attribute.String("user.id", userID.String()),
// 	))
// 	defer span.End()

// 	if message == "" {
// 		message = fmt.Sprintf("Plan a trip to %s", cityName)
// 	}

// 	sessionID := uuid.New()
// 	eventCh := make(chan StreamEvent, 100) // Buffered channel
// 	ctx, cancel := context.WithCancel(ctx)

// 	go func() {
// 		defer close(eventCh)

// 		l.sendEvent(eventCh, StreamEvent{
// 			Type:      EventTypeStart,
// 			Data:      map[string]string{"session_id": sessionID.String()},
// 			Timestamp: time.Now(),
// 			EventID:   uuid.New().String(),
// 		})

// 		// Fetch user data
// 		interests, searchProfile, tags, err := l.FetchUserData(ctx, userID, profileID)
// 		if err != nil {
// 			l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 			return
// 		}

// 		interestNames, tagsPromptPart, userPrefs := l.PreparePromptData(interests, tags, searchProfile)

// 		// Channels for worker results
// 		cityDataCh := make(chan types.GenAIResponse, 1)
// 		generalPOICh := make(chan types.GenAIResponse, 1)
// 		personalizedPOICh := make(chan types.GenAIResponse, 1)

// 		var wg sync.WaitGroup
// 		wg.Add(3)

// 		go l.streamingCityDataWorker(&wg, ctx, cityName, cityDataCh, eventCh)
// 		go l.streamingGeneralPOIWorker(&wg, ctx, cityName, generalPOICh, eventCh)
// 		go l.streamingPersonalizedPOIWorker(&wg, ctx, cityName, userID, profileID, personalizedPOICh, eventCh, interestNames, tagsPromptPart, userPrefs)

// 		go func() {
// 			wg.Wait()
// 			close(cityDataCh)
// 			close(generalPOICh)
// 			close(personalizedPOICh)
// 		}()

// 		var itinerary types.AiCityResponse
// 		var llmInteractionID uuid.UUID
// 		var rawPersonalisedPOIs []types.POIDetail
// 		var errors []error

// 		for i := 0; i < 3; i++ {
// 			select {
// 			case result, ok := <-cityDataCh:
// 				if ok && result.Err != nil {
// 					errors = append(errors, result.Err)
// 				} else if ok {
// 					itinerary.GeneralCityData = types.GeneralCityData{
// 						City:            result.City,
// 						Country:         result.Country,
// 						StateProvince:   result.StateProvince,
// 						Description:     result.CityDescription,
// 						CenterLatitude:  result.Latitude,
// 						CenterLongitude: result.Longitude,
// 					}
// 				}
// 			case result, ok := <-generalPOICh:
// 				if ok && result.Err != nil {
// 					errors = append(errors, result.Err)
// 				} else if ok {
// 					itinerary.PointsOfInterest = result.GeneralPOI
// 				}
// 			case result, ok := <-personalizedPOICh:
// 				if ok && result.Err != nil {
// 					errors = append(errors, result.Err)
// 				} else if ok {
// 					itinerary.AIItineraryResponse = types.AIItineraryResponse{
// 						ItineraryName:      result.ItineraryName,
// 						OverallDescription: result.ItineraryDescription,
// 						PointsOfInterest:   result.PersonalisedPOI,
// 					}
// 					llmInteractionID = result.LlmInteractionID
// 					rawPersonalisedPOIs = result.PersonalisedPOI
// 				}
// 			case <-ctx.Done():
// 				l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: ctx.Err().Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 				return
// 			}
// 		}

// 		if len(errors) > 0 {
// 			l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: fmt.Sprintf("errors: %v", errors), Timestamp: time.Now(), EventID: uuid.New().String()})
// 			return
// 		}

// 		cityID, err := l.HandleCityData(ctx, itinerary.GeneralCityData)
// 		if err != nil {
// 			l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 			return
// 		}

// 		l.HandleGeneralPOIs(ctx, itinerary.PointsOfInterest, cityID)
// 		sortedPOIs, err := l.HandlePersonalisedPOIs(ctx, rawPersonalisedPOIs, cityID, userLocation, llmInteractionID, userID, profileID)
// 		if err != nil {
// 			l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 			return
// 		}
// 		itinerary.AIItineraryResponse.PointsOfInterest = sortedPOIs

// 		session := types.ChatSession{
// 			ID:               sessionID,
// 			UserID:           userID,
// 			CurrentItinerary: &itinerary,
// 			ConversationHistory: []types.ConversationMessage{
// 				{Role: "user", Content: message, Timestamp: time.Now()},
// 				{Role: "assistant", Content: fmt.Sprintf("Here's your trip plan for %s", cityName), Timestamp: time.Now()},
// 			},
// 			SessionContext: types.SessionContext{
// 				CityName:            cityName,
// 				ConversationSummary: fmt.Sprintf("Trip plan for %s", cityName),
// 			},
// 			CreatedAt: time.Now(),
// 			UpdatedAt: time.Now(),
// 			ExpiresAt: time.Now().Add(24 * time.Hour),
// 			Status:    "active",
// 		}

// 		if err := l.llmInteractionRepo.CreateSession(ctx, session); err != nil {
// 			l.sendEvent(eventCh, StreamEvent{Type: EventTypeError, Error: err.Error(), Timestamp: time.Now(), EventID: uuid.New().String()})
// 			return
// 		}

// 		l.sendEvent(eventCh, StreamEvent{
// 			Type:      EventTypeItinerary,
// 			Data:      itinerary,
// 			Timestamp: time.Now(),
// 			EventID:   uuid.New().String(),
// 		})

// 		l.sendEvent(eventCh, StreamEvent{
// 			Type:      EventTypeComplete,
// 			Data:      map[string]string{"session_id": sessionID.String()},
// 			Timestamp: time.Now(),
// 			EventID:   uuid.New().String(),
// 		})
// 	}()

// 	return &StreamingResponse{
// 		SessionID: sessionID,
// 		Stream:    eventCh,
// 		Cancel:    cancel,
// 	}, nil
// }

// func (l *LlmInteractiontServiceImpl) sendEvent(eventCh chan<- StreamEvent, event StreamEvent) {
// 	select {
// 	case eventCh <- event:
// 		// Event sent successfully
// 	default:
// 		l.logger.Warn("Streaming channel closed or full, event not sent", slog.Any("event", event))
// 	}
// }

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

	// Fetch user data
	interests, searchProfile, tags, err := l.FetchUserData(ctx, userID, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user data: %w", err)
	}

	interestNames, tagsPromptPart, userPrefs := l.PreparePromptData(interests, tags, searchProfile)

	// Construct a single prompt for the entire itinerary
	prompt := fmt.Sprintf("Please provide a trip plan for %s, including:\n"+
		"1. City data: name, country, description, coordinates.\n"+
		"2. General points of interest: list of POIs with name, category, coordinates.\n"+
		"3. Personalized points of interest based on user interests [%s]: list of POIs with name, category, coordinates, description.\n"+
		"Format the response as JSON with sections for city_data, general_pois, and personalized_pois.",
		cityName, strings.Join(interestNames, ", "))

	if tagsPromptPart != "" {
		prompt += "\n" + tagsPromptPart
	}
	if userPrefs != "" {
		prompt += "\n" + userPrefs
	}

	// Start chat session
	chatSession, err := l.aiClient.StartChatSession(ctx, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](defaultTemperature)})
	if err != nil {
		return nil, fmt.Errorf("failed to start chat session: %w", err)
	}

	sessionID := uuid.New()
	eventCh := make(chan StreamEvent, 100)
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(eventCh)

		l.sendEvent(eventCh, StreamEvent{
			Type:      EventTypeStart,
			Data:      map[string]string{"session_id": sessionID.String()},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})

		// Stream the response
		stream := chatSession.SendMessageStream(ctx, prompt)
		var responseText strings.Builder

		for chunk, err := range stream {
			if err != nil {
				l.sendEvent(eventCh, StreamEvent{
					Type:      EventTypeError,
					Error:     fmt.Sprintf("streaming error: %v", err),
					Timestamp: time.Now(),
					EventID:   uuid.New().String(),
				})
				return
			}
			for _, cand := range chunk.Candidates {
				if cand.Content != nil {
					for _, part := range cand.Content.Parts {
						if part.Text != "" {
							responseText.WriteString(part.Text)
							l.sendEvent(eventCh, StreamEvent{
								Type:      EventTypeMessage,
								Data:      part.Text,
								Timestamp: time.Now(),
								EventID:   uuid.New().String(),
							})
						}
					}
				}
			}
		}

		fullText := responseText.String()
		if fullText == "" {
			l.sendEvent(eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     "empty response from AI",
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		cleanTxt := cleanJSONResponse(fullText)
		var itineraryData struct {
			CityData         types.GeneralCityData `json:"city_data"`
			GeneralPOIs      []types.POIDetail     `json:"general_pois"`
			PersonalizedPOIs []types.POIDetail     `json:"personalized_pois"`
		}
		if err := json.Unmarshal([]byte(cleanTxt), &itineraryData); err != nil {
			l.sendEvent(eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     fmt.Sprintf("failed to parse itinerary JSON: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		itinerary := types.AiCityResponse{
			GeneralCityData:  itineraryData.CityData,
			PointsOfInterest: itineraryData.GeneralPOIs,
			AIItineraryResponse: types.AIItineraryResponse{
				ItineraryName:      "Personalized Itinerary",
				OverallDescription: "Based on your interests",
				PointsOfInterest:   itineraryData.PersonalizedPOIs,
			},
		}

		// Process and save data
		cityID, err := l.HandleCityData(ctx, itinerary.GeneralCityData)
		if err != nil {
			l.sendEvent(eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     err.Error(),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		l.HandleGeneralPOIs(ctx, itinerary.PointsOfInterest, cityID)
		sortedPOIs, err := l.HandlePersonalisedPOIs(ctx, itineraryData.PersonalizedPOIs, cityID, userLocation, uuid.New(), userID, profileID)
		if err != nil {
			l.sendEvent(eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     err.Error(),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}
		itinerary.AIItineraryResponse.PointsOfInterest = sortedPOIs

		// Save session
		session := types.ChatSession{
			ID:               sessionID,
			UserID:           userID,
			CurrentItinerary: &itinerary,
			ConversationHistory: []types.ConversationMessage{
				{Role: "user", Content: message, Timestamp: time.Now()},
				{Role: "assistant", Content: fullText, Timestamp: time.Now()},
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
			l.sendEvent(eventCh, StreamEvent{
				Type:      EventTypeError,
				Error:     err.Error(),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		l.sendEvent(eventCh, StreamEvent{
			Type:      EventTypeItinerary,
			Data:      itinerary,
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})

		l.sendEvent(eventCh, StreamEvent{
			Type:      EventTypeComplete,
			Data:      map[string]string{"session_id": sessionID.String()},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
	}()

	return &StreamingResponse{
		SessionID: sessionID,
		Stream:    eventCh,
		Cancel:    cancel,
	}, nil
}

// ContinueSessionStreamed handles subsequent messages in an existing session with streaming responses
func (l *LlmInteractiontServiceImpl) ContinueSessionStreamed(ctx context.Context, sessionID uuid.UUID, message string, userLocation *types.UserLocation) (*StreamingResponse, error) {
	ctx, span := otel.Tracer("LlmInteractionService").Start(ctx, "ContinueSessionStreamed", trace.WithAttributes(
		attribute.String("session.id", sessionID.String()),
		attribute.String("message", message),
	))
	defer span.End()

	// Set up streaming channel and context
	eventCh := make(chan StreamEvent, 100) // Buffer to prevent blocking
	ctx, cancel := context.WithCancel(ctx)

	// Process in a goroutine to stream events asynchronously
	go func() {
		defer close(eventCh)

		// Fetch session
		session, err := l.llmInteractionRepo.GetSession(ctx, sessionID)
		if err != nil || session.Status != "active" {
			l.logger.ErrorContext(ctx, "Invalid or inactive session", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Invalid or inactive session")
			l.sendEvent(eventCh, StreamEvent{
				Type:      "error",
				Error:     fmt.Sprintf("invalid or inactive session: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}
		l.sendEvent(eventCh, StreamEvent{
			Type:      "session_fetched",
			Data:      map[string]string{"session_id": sessionID.String()},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})

		// Fetch city ID
		city, err := l.cityRepo.FindCityByNameAndCountry(ctx, session.SessionContext.CityName, "")
		if err != nil || city == nil {
			l.logger.ErrorContext(ctx, "Failed to find city", slog.Any("error", err))
			span.RecordError(err)
			l.sendEvent(eventCh, StreamEvent{
				Type:      "error",
				Error:     fmt.Sprintf("city %s not found: %v", session.SessionContext.CityName, err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}
		cityID := city.ID

		// Add user message
		userMessage := types.ConversationMessage{
			ID:          uuid.New(),
			Role:        "user",
			Content:     message,
			Timestamp:   time.Now(),
			MessageType: types.TypeModificationRequest,
		}
		if err := l.llmInteractionRepo.AddMessageToSession(ctx, sessionID, userMessage); err != nil {
			span.RecordError(err)
			l.sendEvent(eventCh, StreamEvent{
				Type:      "error",
				Error:     fmt.Sprintf("failed to add message to session: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		// Classify intent
		intent, err := l.intentClassifier.Classify(ctx, message)
		if err != nil {
			l.logger.ErrorContext(ctx, "Failed to classify intent", slog.Any("error", err))
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to classify intent")
			l.sendEvent(eventCh, StreamEvent{
				Type:      "error",
				Error:     fmt.Sprintf("failed to classify intent: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}
		l.sendEvent(eventCh, StreamEvent{
			Type:      "intent_classified",
			Data:      map[string]string{"intent": intent},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})

		var responseText string
		switch intent {
		case "add_poi":
			poiName := extractPOIName(message)
			for _, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
				if strings.EqualFold(poi.Name, poiName) {
					responseText = fmt.Sprintf("%s is already in your itinerary.", poiName)
					break
				}
			}
			if responseText == "" {
				l.sendEvent(eventCh, StreamEvent{
					Type:      "generating_poi",
					Data:      map[string]string{"poi_name": poiName},
					Timestamp: time.Now(),
					EventID:   uuid.New().String(),
				})
				newPOI, err := l.generatePOIDataStream(ctx, poiName, session.SessionContext.CityName, userLocation, session.UserID, cityID, eventCh)
				if err != nil {
					l.logger.ErrorContext(ctx, "Failed to generate POI data", slog.Any("error", err))
					span.RecordError(err)
					responseText = fmt.Sprintf("Could not add %s due to an error.", poiName)
				} else {
					session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = append(
						session.CurrentItinerary.AIItineraryResponse.PointsOfInterest, newPOI)
					responseText = fmt.Sprintf("I’ve added %s to your itinerary.", poiName)
				}
			}
		case "remove_poi":
			poiName := extractPOIName(message)
			for i, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
				if strings.Contains(strings.ToLower(poi.Name), strings.ToLower(poiName)) {
					session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = append(
						session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[:i],
						session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[i+1:]...,
					)
					responseText = fmt.Sprintf("I’ve removed %s from your itinerary.", poiName)
					break
				}
			}
			if responseText == "" {
				responseText = fmt.Sprintf("Could not find %s in your itinerary.", poiName)
			}
		case "ask_question":
			responseText = "I’m here to help! For now, I’ll assume you’re asking about your trip. What specifically would you like to know?"
		default: // modify_itinerary
			if matches := regexp.MustCompile(`replace\s+(.+?)\s+with\s+(.+?)(?:\s+in\s+my\s+itinerary)?`).FindStringSubmatch(strings.ToLower(message)); len(matches) == 3 {
				oldPOI := matches[1]
				newPOIName := matches[2]
				for i, poi := range session.CurrentItinerary.AIItineraryResponse.PointsOfInterest {
					if strings.Contains(strings.ToLower(poi.Name), oldPOI) {
						l.sendEvent(eventCh, StreamEvent{
							Type:      "generating_poi",
							Data:      map[string]string{"poi_name": newPOIName},
							Timestamp: time.Now(),
							EventID:   uuid.New().String(),
						})
						newPOI, err := l.generatePOIDataStream(ctx, newPOIName, session.SessionContext.CityName, userLocation, session.UserID, cityID, eventCh)
						if err != nil {
							l.logger.ErrorContext(ctx, "Failed to generate POI data", slog.Any("error", err))
							span.RecordError(err)
							responseText = fmt.Sprintf("Could not replace %s with %s due to an error.", oldPOI, newPOIName)
						} else {
							session.CurrentItinerary.AIItineraryResponse.PointsOfInterest[i] = newPOI
							responseText = fmt.Sprintf("I’ve replaced %s with %s in your itinerary.", oldPOI, newPOIName)
						}
						break
					}
				}
				if responseText == "" {
					responseText = fmt.Sprintf("Could not find %s in your itinerary.", oldPOI)
				}
			} else {
				responseText = "I’ve noted your request to modify the itinerary. Please specify the changes (e.g., 'replace X with Y')."
			}
		}

		// Sort POIs by distance if applicable
		if (intent == "add_poi" || intent == "modify_itinerary") && userLocation != nil && userLocation.UserLat != 0 && userLocation.UserLon != 0 {
			sortedPOIs, err := l.llmInteractionRepo.GetPOIsBySessionSortedByDistance(ctx, sessionID, cityID, *userLocation)
			if err != nil {
				l.logger.WarnContext(ctx, "Failed to sort POIs by distance", slog.Any("error", err))
				span.RecordError(err)
			} else {
				session.CurrentItinerary.AIItineraryResponse.PointsOfInterest = sortedPOIs
				l.logger.InfoContext(ctx, "POIs sorted by distance", slog.Int("poi_count", len(sortedPOIs)))
				span.SetAttributes(attribute.Int("sorted_pois.count", len(sortedPOIs)))
			}
		}

		// Add assistant response
		assistantMessage := types.ConversationMessage{
			ID:          uuid.New(),
			Role:        "assistant",
			Content:     responseText,
			Timestamp:   time.Now(),
			MessageType: types.TypeModificationRequest,
		}
		if err := l.llmInteractionRepo.AddMessageToSession(ctx, sessionID, assistantMessage); err != nil {
			span.RecordError(err)
			l.sendEvent(eventCh, StreamEvent{
				Type:      "error",
				Error:     fmt.Sprintf("failed to add assistant message: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		// Update session
		session.UpdatedAt = time.Now()
		session.ExpiresAt = time.Now().Add(24 * time.Hour)
		if err := l.llmInteractionRepo.UpdateSession(ctx, *session); err != nil {
			l.logger.ErrorContext(ctx, "Failed to update session", slog.Any("error", err))
			span.RecordError(err)
			l.sendEvent(eventCh, StreamEvent{
				Type:      "error",
				Error:     fmt.Sprintf("failed to update session: %v", err),
				Timestamp: time.Now(),
				EventID:   uuid.New().String(),
			})
			return
		}

		// Send final itinerary and completion events
		l.sendEvent(eventCh, StreamEvent{
			Type:      "itinerary",
			Data:      session.CurrentItinerary,
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})
		l.sendEvent(eventCh, StreamEvent{
			Type:      "complete",
			Data:      map[string]string{"session_id": sessionID.String()},
			Timestamp: time.Now(),
			EventID:   uuid.New().String(),
		})

		l.logger.InfoContext(ctx, "Session continued",
			slog.String("sessionID", sessionID.String()),
			slog.String("intent", intent))
		span.SetStatus(codes.Ok, "Session continued successfully")
	}()

	return &StreamingResponse{
		SessionID: sessionID,
		Stream:    eventCh,
		Cancel:    cancel,
	}, nil
}

// sendEvent is a helper to send events to the stream (assumed to exist or implement as needed)
func (l *LlmInteractiontServiceImpl) sendEvent(ch chan<- StreamEvent, event StreamEvent) {
	select {
	case ch <- event:
	case <-time.After(1 * time.Second): // Timeout to avoid blocking indefinitely
		l.logger.WarnContext(context.Background(), "Dropped stream event due to slow consumer", slog.Any("event", event))
	}
}

func (l *LlmInteractiontServiceImpl) generatePOIDataStream(ctx context.Context, poiName, cityName string, userLocation *types.UserLocation, userID, cityID uuid.UUID, eventCh chan<- StreamEvent) (types.POIDetail, error) {
	prompt := generatedContinuedConversationPrompt(poiName, cityName) // Assume this exists
	stream, err := l.aiClient.GenerateContentStream(ctx, prompt, &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.7)})
	if err != nil {
		return types.POIDetail{}, err
	}

	var responseText strings.Builder
	for chunk := range stream {
		// if chunk.Err != nil {
		// 	return types.POIDetail{}, chunk.Error
		// }
		for _, cand := range chunk.Candidates {
			if cand.Content != nil {
				for _, part := range cand.Content.Parts {
					// if txt, ok := part.(genai.Text); ok {
					// 	responseText.WriteString(string(txt))
					// 	l.sendEvent(eventCh, StreamEvent{
					// 		Type:      "poi_generation",
					// 		Data:      string(txt),
					// 		Timestamp: time.Now(),
					// 		EventID:   uuid.New().String(),
					// 	})
					// }
					if part.Text != "" {
						responseText.WriteString(string(part.Text))
						l.sendEvent(eventCh, StreamEvent{
							Type:      "poi_generation",
							Data:      string(part.Text),
							Timestamp: time.Now(),
							EventID:   uuid.New().String(),
						})
					}
				}
			}
		}
	}

	fullText := responseText.String()
	cleanResponse := cleanJSONResponse(fullText) // Assume this exists
	var poiData types.POIDetail
	if err := json.Unmarshal([]byte(cleanResponse), &poiData); err != nil {
		return types.POIDetail{}, err
	}
	return poiData, nil
}
