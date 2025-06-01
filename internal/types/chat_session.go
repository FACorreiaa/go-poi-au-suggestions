package types

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ChatSession struct {
	ID                  uuid.UUID             `json:"id"`
	UserID              uuid.UUID             `json:"user_id"`
	ProfileID           uuid.UUID             `json:"profile_id"`
	CityName            string                `json:"city_name"`
	CurrentItinerary    *AiCityResponse       `json:"current_itinerary,omitempty"`
	ConversationHistory []ConversationMessage `json:"conversation_history"`
	SessionContext      SessionContext        `json:"session_context"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
	ExpiresAt           time.Time             `json:"expires_at"`
	Status              SessionStatus         `json:"status"`
}

type ConversationMessage struct {
	ID          uuid.UUID       `json:"id"`
	Role        MessageRole     `json:"role"` // user, assistant, system
	Content     string          `json:"content"`
	MessageType MessageType     `json:"message_type"` // initial_request, modification_request, response
	Timestamp   time.Time       `json:"timestamp"`
	Metadata    MessageMetadata `json:"metadata,omitempty"`
}

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

type MessageType string

const (
	TypeInitialRequest      MessageType = "initial_request"
	TypeModificationRequest MessageType = "modification_request"
	TypeResponse            MessageType = "response"
	TypeClarification       MessageType = "clarification"
)

type MessageMetadata struct {
	LlmInteractionID *uuid.UUID `json:"llm_interaction_id,omitempty"`
	ModifiedPOICount int        `json:"modified_poi_count,omitempty"`
	RequestType      string     `json:"request_type,omitempty"` // add_poi, remove_poi, modify_preferences, etc.
}

type SessionContext struct {
	LastCityID          uuid.UUID                      `json:"last_city_id"`
	UserPreferences     *UserPreferenceProfileResponse `json:"user_preferences"`
	ActiveInterests     []string                       `json:"active_interests"`
	ActiveTags          []string                       `json:"active_tags"`
	ConversationSummary string                         `json:"conversation_summary"`
	ModificationHistory []ModificationRecord           `json:"modification_history"`
}

type ModificationRecord struct {
	Type        string    `json:"type"` // add_poi, remove_poi, change_preferences
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	Applied     bool      `json:"applied"`
}

type SessionStatus string

const (
	StatusActive  SessionStatus = "active"
	StatusExpired SessionStatus = "expired"
	StatusClosed  SessionStatus = "closed"
)

// Request/Response types for chat API
type ChatRequest struct {
	SessionID *uuid.UUID `json:"session_id,omitempty"` // nil for new session
	Message   string     `json:"message"`
	CityName  string     `json:"city_name,omitempty"` // required for new session
}

type ChatResponse struct {
	SessionID             uuid.UUID       `json:"session_id"`
	Message               string          `json:"message"`
	UpdatedItinerary      *AiCityResponse `json:"updated_itinerary,omitempty"`
	IsNewSession          bool            `json:"is_new_session"`
	RequiresClarification bool            `json:"requires_clarification"`
	SuggestedActions      []string        `json:"suggested_actions,omitempty"`
}

// Session Repository Interface
type ChatSessionRepository interface {
	CreateSession(ctx context.Context, session ChatSession) error
	GetSession(ctx context.Context, sessionID uuid.UUID) (*ChatSession, error)
	UpdateSession(ctx context.Context, session ChatSession) error
	AddMessageToSession(ctx context.Context, sessionID uuid.UUID, message ConversationMessage) error
	GetActiveSessionsForUser(ctx context.Context, userID uuid.UUID) ([]ChatSession, error)
	ExpireSession(ctx context.Context, sessionID uuid.UUID) error
	CleanupExpiredSessions(ctx context.Context) error
}

// Chat Service Interface
type ChatService interface {
	StartNewSession(ctx context.Context, userID, profileID uuid.UUID, cityName, initialMessage string) (*ChatResponse, error)
	ContinueSession(ctx context.Context, sessionID uuid.UUID, message string) (*ChatResponse, error)
	GetSessionHistory(ctx context.Context, sessionID uuid.UUID) (*ChatSession, error)
	EndSession(ctx context.Context, sessionID uuid.UUID) error
}
