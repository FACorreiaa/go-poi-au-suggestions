package llmChat

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/api/auth"
	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type StreamingHandler struct {
	llmService LlmInteractiontService
	logger     *slog.Logger
}

func NewStreamingHandler(llmService LlmInteractiontService, logger *slog.Logger) *StreamingHandler {
	return &StreamingHandler{
		llmService: llmService,
		logger:     logger,
	}
}

func (h *HandlerImpl) StartChatSessionStream(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		api.ErrorResponse(w, r, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	ctx := r.Context()

	// Authentication
	userIDStr, ok := auth.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		api.ErrorResponse(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	profileIDStr := chi.URLParam(r, "profileID")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		h.writeSSEError(w, "Invalid profile ID format")
		return
	}

	var req struct {
		CityName string `json:"city_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeSSEError(w, "Invalid request body")
		return
	}

	userLocation := &types.UserLocation{
		UserLat: 41.3851,
		UserLon: 2.1734,
	}

	// Start streaming
	streamResp, err := h.llmInteractionService.StartNewSessionStreamed(ctx, userID, profileID, req.CityName, "", userLocation)
	if err != nil {
		h.writeSSEError(w, fmt.Sprintf("Failed to start session: %v", err))
		return
	}
	defer streamResp.Cancel()

	h.logger.InfoContext(ctx, "Started streaming session",
		slog.String("session_id", streamResp.SessionID.String()),
		slog.String("city_name", req.CityName))

	// Stream events
	for {
		select {

		case event, ok := <-streamResp.Stream:
			if !ok {
				h.logger.InfoContext(ctx, "Stream closed", slog.String("session_id", streamResp.SessionID.String()))
				return
			}

			data, err := json.Marshal(event)
			if err != nil {
				h.logger.ErrorContext(ctx, "Failed to marshal event", slog.Any("error", err))
				continue
			}

			fmt.Fprintf(w, "id: %s\n", event.EventID)
			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case <-ctx.Done():
			h.logger.InfoContext(ctx, "Client disconnected", slog.String("session_id", streamResp.SessionID.String()))
			return
		}
	}
}

func (h *HandlerImpl) writeSSEError(w http.ResponseWriter, errorMsg string) {
	event := types.StreamEvent{
		Type:      types.EventTypeError,
		Error:     errorMsg,
		Timestamp: time.Now(),
		EventID:   uuid.New().String(),
	}
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "id: %s\n", event.EventID)
	fmt.Fprintf(w, "event: %s\n", event.Type)
	fmt.Fprintf(w, "data: %s\n\n", data)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// ContinueSessionStreamHandler handles streaming requests for continuing a session
func (h *HandlerImpl) ContinueSessionStreamHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("Handler").Start(r.Context(), "ContinueSessionStreamHandler")
	defer span.End()

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable buffering for some proxies

	// Parse request body
	var req struct {
		SessionID    string              `json:"session_id"`
		Message      string              `json:"message"`
		UserLocation *types.UserLocation `json:"user_location"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.ErrorContext(ctx, "Failed to decode request body", slog.Any("error", err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		span.RecordError(err)
		return
	}

	// Validate and parse sessionID
	sessionIDStr := chi.URLParam(r, "sessionID")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		h.logger.ErrorContext(ctx, "Invalid session ID", slog.String("sessionID", sessionIDStr), slog.Any("error", err))
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		span.RecordError(err)
		return
	}

	// Validate message
	if req.Message == "" {
		h.logger.ErrorContext(ctx, "Message is empty")
		http.Error(w, "Message cannot be empty", http.StatusBadRequest)
		span.RecordError(fmt.Errorf("message is empty"))
		return
	}

	// Validate userLocation (if provided)
	if req.UserLocation != nil && (req.UserLocation.UserLat == 0 || req.UserLocation.UserLon == 0) {
		h.logger.WarnContext(ctx, "Invalid user location, ignoring", slog.Any("userLocation", req.UserLocation))
		req.UserLocation = nil // Ignore invalid location
	}

	// Create channel for streaming events
	eventCh := make(chan types.StreamEvent)
	defer close(eventCh) // Ensure channel is closed when handler exits

	// Start the service in a goroutine
	go func() {
		err := h.llmInteractionService.ContinueSessionStreamed(ctx, sessionID, req.Message, req.UserLocation, eventCh)
		if err != nil {
			h.logger.ErrorContext(ctx, "ContinueSessionStreamed failed", slog.Any("error", err))
			span.RecordError(err)
			// Send error event if the channel is still open
			select {
			case eventCh <- types.StreamEvent{
				Type:      string(types.TypeError),
				Error:     err.Error(),
				IsFinal:   true,
				EventID:   uuid.New().String(),
				Timestamp: time.Now(),
			}:
			case <-ctx.Done():
			}
		}
	}()

	// Stream events to the client
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.ErrorContext(ctx, "ResponseWriter does not support flushing")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case event := <-eventCh:
			// Marshal event to JSON
			data, err := json.Marshal(event)
			if err != nil {
				h.logger.WarnContext(ctx, "Failed to marshal event", slog.Any("error", err))
				continue
			}

			// Write SSE event
			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			span.AddEvent("Sent SSE event", trace.WithAttributes(
				attribute.String("event.type", event.Type),
				attribute.String("event.id", event.EventID),
			))

			if event.IsFinal {
				return // Exit after final event
			}

		case <-ctx.Done():
			h.logger.InfoContext(ctx, "Client disconnected or context cancelled")
			span.AddEvent("Client disconnected")
			return
		}
	}
}
