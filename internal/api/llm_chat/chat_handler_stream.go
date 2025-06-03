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
		h.writeSSEError(w, "Authentication required")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeSSEError(w, "Invalid user ID format")
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
	event := StreamEvent{
		Type:      EventTypeError,
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
