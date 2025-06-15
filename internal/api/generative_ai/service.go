package generativeAI

import (
	"context"
	"flag"
	"fmt"
	"iter"
	"log"
	"log/slog"
	"math"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genai"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
)

var model = flag.String("model", "gemini-2.0-flash", "the model name, e.g. gemini-2.0-flash")

type AIClient struct {
	client *genai.Client
	model  string
}

type ChatSession struct {
	chat *genai.Chat
}

type RAGService struct {
	aiClient         *AIClient
	embeddingService *EmbeddingService
	logger           *slog.Logger
}

type RAGContext struct {
	Query               string                 `json:"query"`
	RelevantPOIs        []types.POIDetail      `json:"relevant_pois"`
	UserPreferences     map[string]interface{} `json:"user_preferences,omitempty"`
	CityContext         string                 `json:"city_context,omitempty"`
	ConversationHistory []ConversationTurn     `json:"conversation_history,omitempty"`
}

type ConversationTurn struct {
	Role      string                 `json:"role"` // "user" or "assistant"
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type RAGResponse struct {
	Answer      string            `json:"answer"`
	SourcePOIs  []types.POIDetail `json:"source_pois"`
	Confidence  float64           `json:"confidence"`
	Suggestions []string          `json:"suggestions,omitempty"`
}

func NewAIClient(ctx context.Context) (*AIClient, error) {
	ctx, span := otel.Tracer("GenerativeAI").Start(ctx, "NewAIClient")
	defer span.End()

	apiKey := os.Getenv("GOOGLE_GEMINI_API_KEY")
	if apiKey == "" {
		err := fmt.Errorf("GOOGLE_GEMINI_API_KEY environment variable is not set")
		span.RecordError(err)
		span.SetStatus(codes.Error, "API key not set")
		log.Fatal(err)
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create Gemini client")
		return nil, err
	}

	span.SetStatus(codes.Ok, "AI client created successfully")
	return &AIClient{
		client: client,
		model:  *model,
	}, nil
}

// func (ai *AIClient) SetupFunctionCalling() {
// 	poiFunc := &genai.FunctionDeclaration{
// 		Name:        "findPOIs",
// 		Description: "Find points of interest in a city within a radius.",
// 		Parameters: &genai.Schema{
// 			Type: genai.TypeObject,
// 			Properties: map[string]*genai.Schema{
// 				"city":   {Type: genai.TypeString, Description: "City name"},
// 				"radius": {Type: genai.TypeNumber, Description: "Radius in km"},
// 			},
// 			Required: []string{"city", "radius"},
// 		},
// 	}
// 	ai.model.Tools = []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{poiFunc}}}
// }

func (ai *AIClient) GenerateContent(ctx context.Context, prompt string, config *genai.GenerateContentConfig) (string, error) {
	ctx, span := otel.Tracer("GenerativeAI").Start(ctx, "GenerateContent", trace.WithAttributes(
		attribute.String("prompt.length", fmt.Sprintf("%d", len(prompt))),
		attribute.String("model", *model),
	))
	defer span.End()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  os.Getenv("GOOGLE_GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create client")
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	result, err := client.Models.GenerateContent(ctx, *model, genai.Text(prompt), config)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate content")
		log.Fatal(err)
	}

	responseText := result.Text()
	span.SetAttributes(attribute.Int("response.length", len(responseText)))
	span.SetStatus(codes.Ok, "Content generated successfully")
	return responseText, nil
}

func (ai *AIClient) GenerateResponse(ctx context.Context, prompt string, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	ctx, span := otel.Tracer("GenerativeAI").Start(ctx, "GenerateResponse", trace.WithAttributes(
		attribute.String("prompt.length", fmt.Sprintf("%d", len(prompt))),
		attribute.String("model", ai.model),
	))
	defer span.End()

	chat, err := ai.client.Chats.Create(ctx, ai.model, config, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create chat")
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}

	response, err := chat.SendMessage(ctx, genai.Part{Text: prompt})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to send message")
		return nil, err
	}

	span.SetStatus(codes.Ok, "Response generated successfully")
	return response, nil
}

func (ai *AIClient) StartChatSession(ctx context.Context, config *genai.GenerateContentConfig) (*ChatSession, error) {
	ctx, span := otel.Tracer("GenerativeAI").Start(ctx, "StartChatSession", trace.WithAttributes(
		attribute.String("model", ai.model),
	))
	defer span.End()

	//config = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}
	chat, err := ai.client.Chats.Create(ctx, ai.model, config, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create chat session")
		return nil, err
	}

	span.SetStatus(codes.Ok, "Chat session created successfully")
	return &ChatSession{chat: chat}, nil
}

func (cs *ChatSession) SendMessage(ctx context.Context, message string) (string, error) {
	ctx, span := otel.Tracer("GenerativeAI").Start(ctx, "SendMessage", trace.WithAttributes(
		attribute.String("message.length", fmt.Sprintf("%d", len(message))),
	))
	defer span.End()

	result, err := cs.chat.SendMessage(ctx, genai.Part{Text: message})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to send message")
		return "", err
	}

	responseText := result.Text()
	span.SetAttributes(attribute.Int("response.length", len(responseText)))
	span.SetStatus(codes.Ok, "Message sent successfully")
	return responseText, nil
}

//func GenerateEmbedding(ctx context.Context, client *genai.Client, text string) ([]float32, error) {
//	req := &genaipb.EmbedContentRequest{
//		Model: "models/embedding-001", // replace with actual embedding model name
//		Content: &genaipb.Content{
//			Parts: []*genaipb.Part{
//				{Value: &genaipb.Part_Text{Text: text}},
//			},
//		},
//	}
//	resp, err := client.EmbedContent(ctx, req)
//	if err != nil {
//		return nil, err
//	}
//	return resp.Embedding.Values, nil
//}

// GenerateContentStream initiates a streaming content generation process.
func (ai *AIClient) GenerateContentStream(
	ctx context.Context,
	prompt string,
	config *genai.GenerateContentConfig,
) (iter.Seq2[*genai.GenerateContentResponse, error], error) {
	ctx, span := otel.Tracer("GenerativeAI").Start(ctx, "GenerateContentStream", trace.WithAttributes(
		attribute.String("prompt.length", fmt.Sprintf("%d", len(prompt))),
		attribute.String("model", ai.model),
	))
	defer span.End()

	if ai.client == nil {
		err := fmt.Errorf("AIClient's internal genai.Client is not initialized")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Client not initialized for stream")
		return nil, err
	}

	// Assuming the genai library has a method like this; adjust as needed
	stream := ai.client.Models.GenerateContentStream(ctx, ai.model, genai.Text(prompt), config)

	span.SetStatus(codes.Ok, "Content stream initiated")
	return stream, nil
}

// func (cs *ChatSession) SendMessageStream(ctx context.Context, message string) (iter.Seq2[*genai.GenerateContentResponse, error], error) {
// 	ctx, span := otel.Tracer("GenerativeAI").Start(ctx, "SendMessage", trace.WithAttributes(
// 		attribute.String("message.length", fmt.Sprintf("%d", len(message))),
// 	))
// 	defer span.End()

// 	iter := cs.chat.SendMessageStream(ctx, genai.Part{Text: message})

// 	span.SetStatus(codes.Ok, "Content stream initiated")
// 	return iter, nil
// }

// Add SendMessageStream to ChatSession
func (cs *ChatSession) SendMessageStream(ctx context.Context, message string) iter.Seq2[*genai.GenerateContentResponse, error] {
	return cs.chat.SendMessageStream(ctx, genai.Part{Text: message})
}

// NewRAGService creates a new RAG service instance
func NewRAGService(ctx context.Context, logger *slog.Logger) (*RAGService, error) {
	ctx, span := otel.Tracer("RAGService").Start(ctx, "NewRAGService")
	defer span.End()

	aiClient, err := NewAIClient(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create AI client")
		return nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	embeddingService, err := NewEmbeddingService(ctx, logger)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create embedding service")
		return nil, fmt.Errorf("failed to create embedding service: %w", err)
	}

	span.SetStatus(codes.Ok, "RAG service created successfully")
	return &RAGService{
		aiClient:         aiClient,
		embeddingService: embeddingService,
		logger:           logger,
	}, nil
}

// GenerateRAGResponse generates a response using retrieved context from similar POIs
func (rs *RAGService) GenerateRAGResponse(ctx context.Context, ragContext RAGContext) (*RAGResponse, error) {
	ctx, span := otel.Tracer("RAGService").Start(ctx, "GenerateRAGResponse", trace.WithAttributes(
		attribute.String("query", ragContext.Query),
		attribute.Int("relevant_pois.count", len(ragContext.RelevantPOIs)),
	))
	defer span.End()

	l := rs.logger.With(slog.String("method", "GenerateRAGResponse"))

	if ragContext.Query == "" {
		err := fmt.Errorf("query cannot be empty")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Empty query provided")
		return nil, err
	}

	// Build context from relevant POIs
	contextInfo := rs.buildContextFromPOIs(ragContext.RelevantPOIs)

	// Build conversation history context
	conversationContext := rs.buildConversationContext(ragContext.ConversationHistory)

	// Build user preferences context
	userContext := rs.buildUserPreferencesContext(ragContext.UserPreferences)

	// Construct the RAG prompt
	prompt := rs.buildRAGPrompt(ragContext.Query, contextInfo, conversationContext, userContext, ragContext.CityContext)

	l.DebugContext(ctx, "Generated RAG prompt", slog.String("prompt_preview", prompt[:min(200, len(prompt))]))

	// Generate response using AI
	response, err := rs.aiClient.GenerateContent(ctx, prompt, &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0.7),
		MaxOutputTokens: *genai.Ptr[int32](1000),
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate AI response")
		l.ErrorContext(ctx, "Failed to generate AI response", slog.Any("error", err))
		return nil, fmt.Errorf("failed to generate AI response: %w", err)
	}

	// Calculate confidence based on number of relevant POIs and response quality
	confidence := rs.calculateConfidence(ragContext.RelevantPOIs, response)

	// Generate suggestions for follow-up questions
	suggestions := rs.generateSuggestions(ragContext.RelevantPOIs, ragContext.Query)

	ragResponse := &RAGResponse{
		Answer:      strings.TrimSpace(response),
		SourcePOIs:  ragContext.RelevantPOIs,
		Confidence:  confidence,
		Suggestions: suggestions,
	}

	span.SetAttributes(
		attribute.Float64("response.confidence", confidence),
		attribute.Int("response.suggestions.count", len(suggestions)),
		attribute.Int("response.length", len(response)),
	)
	span.SetStatus(codes.Ok, "RAG response generated successfully")

	l.InfoContext(ctx, "RAG response generated",
		slog.Float64("confidence", confidence),
		slog.Int("source_pois", len(ragContext.RelevantPOIs)),
		slog.Int("suggestions", len(suggestions)))

	return ragResponse, nil
}

// buildContextFromPOIs creates a structured context string from relevant POIs
func (rs *RAGService) buildContextFromPOIs(pois []types.POIDetail) string {
	if len(pois) == 0 {
		return "No relevant points of interest found."
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("Relevant Points of Interest:\n")

	for i, poi := range pois {
		contextBuilder.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, poi.Name))
		if poi.Category != "" {
			contextBuilder.WriteString(fmt.Sprintf("   Category: %s\n", poi.Category))
		}
		if poi.DescriptionPOI != "" {
			contextBuilder.WriteString(fmt.Sprintf("   Description: %s\n", poi.DescriptionPOI))
		}
		if poi.Distance > 0 {
			contextBuilder.WriteString(fmt.Sprintf("   Distance: %.2f km\n", poi.Distance))
		}
		contextBuilder.WriteString("\n")
	}

	return contextBuilder.String()
}

// buildConversationContext creates context from conversation history
func (rs *RAGService) buildConversationContext(history []ConversationTurn) string {
	if len(history) == 0 {
		return ""
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("Recent Conversation:\n")

	// Include last few turns to maintain context
	start := max(0, len(history)-5)
	for i := start; i < len(history); i++ {
		turn := history[i]
		contextBuilder.WriteString(fmt.Sprintf("%s: %s\n", strings.Title(turn.Role), turn.Message))
	}

	return contextBuilder.String()
}

// buildUserPreferencesContext creates context from user preferences
func (rs *RAGService) buildUserPreferencesContext(preferences map[string]interface{}) string {
	if len(preferences) == 0 {
		return ""
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("User Preferences:\n")

	for key, value := range preferences {
		contextBuilder.WriteString(fmt.Sprintf("- %s: %v\n", strings.Title(key), value))
	}

	return contextBuilder.String()
}

// buildRAGPrompt constructs the complete RAG prompt
func (rs *RAGService) buildRAGPrompt(query, contextInfo, conversationContext, userContext, cityContext string) string {
	var promptBuilder strings.Builder

	promptBuilder.WriteString("You are Loci, an AI travel assistant specialized in providing personalized recommendations for points of interest. ")
	promptBuilder.WriteString("Use the provided context to answer the user's question accurately and helpfully.\n\n")

	if cityContext != "" {
		promptBuilder.WriteString(fmt.Sprintf("City Context: %s\n\n", cityContext))
	}

	if userContext != "" {
		promptBuilder.WriteString(userContext)
		promptBuilder.WriteString("\n")
	}

	if conversationContext != "" {
		promptBuilder.WriteString(conversationContext)
		promptBuilder.WriteString("\n")
	}

	promptBuilder.WriteString(contextInfo)
	promptBuilder.WriteString("\n")

	promptBuilder.WriteString(fmt.Sprintf("User Question: %s\n\n", query))

	promptBuilder.WriteString("Instructions:\n")
	promptBuilder.WriteString("1. Provide a helpful and accurate answer based on the relevant points of interest\n")
	promptBuilder.WriteString("2. Reference specific POIs from the context when relevant\n")
	promptBuilder.WriteString("3. Consider the user's preferences and conversation history\n")
	promptBuilder.WriteString("4. If the context doesn't contain enough information, acknowledge this limitation\n")
	promptBuilder.WriteString("5. Keep your response conversational and engaging\n")
	promptBuilder.WriteString("6. Include practical information like distances when relevant\n\n")

	promptBuilder.WriteString("Response:")

	return promptBuilder.String()
}

// calculateConfidence estimates the confidence of the response
func (rs *RAGService) calculateConfidence(relevantPOIs []types.POIDetail, response string) float64 {
	baseConfidence := 0.5

	// Increase confidence based on number of relevant POIs
	poiBonus := math.Min(float64(len(relevantPOIs))*0.1, 0.3)

	// Increase confidence based on response length (longer responses often indicate more context)
	lengthBonus := math.Min(float64(len(response))/1000.0*0.1, 0.1)

	// Check if response contains specific POI names (indicates good use of context)
	contextBonus := 0.0
	for _, poi := range relevantPOIs {
		if strings.Contains(strings.ToLower(response), strings.ToLower(poi.Name)) {
			contextBonus += 0.05
		}
	}
	contextBonus = math.Min(contextBonus, 0.2)

	finalConfidence := math.Min(baseConfidence+poiBonus+lengthBonus+contextBonus, 1.0)
	return finalConfidence
}

// generateSuggestions creates follow-up question suggestions
func (rs *RAGService) generateSuggestions(relevantPOIs []types.POIDetail, originalQuery string) []string {
	suggestions := []string{}

	if len(relevantPOIs) > 0 {
		// Suggest questions about specific POIs
		for i, poi := range relevantPOIs {
			if i >= 2 { // Limit to first 2 POIs
				break
			}
			suggestions = append(suggestions, fmt.Sprintf("Tell me more about %s", poi.Name))
		}

		// Suggest questions about categories
		categories := make(map[string]bool)
		for _, poi := range relevantPOIs {
			if poi.Category != "" {
				categories[poi.Category] = true
			}
		}

		for category := range categories {
			suggestions = append(suggestions, fmt.Sprintf("What other %s places do you recommend?", category))
			break // Only add one category suggestion
		}

		// Add distance-based suggestions
		suggestions = append(suggestions, "What's within walking distance?")
	}

	// Add general follow-up suggestions
	if !strings.Contains(strings.ToLower(originalQuery), "opening") {
		suggestions = append(suggestions, "What are the opening hours?")
	}
	if !strings.Contains(strings.ToLower(originalQuery), "cost") && !strings.Contains(strings.ToLower(originalQuery), "price") {
		suggestions = append(suggestions, "How much does it cost to visit?")
	}

	// Limit total suggestions
	if len(suggestions) > 4 {
		suggestions = suggestions[:4]
	}

	return suggestions
}

// StoreConversationTurn stores a conversation turn for future context
func (rs *RAGService) StoreConversationTurn(ctx context.Context, userID string, role, message string, metadata map[string]interface{}) error {
	ctx, span := otel.Tracer("RAGService").Start(ctx, "StoreConversationTurn")
	defer span.End()

	// This would typically store to a database or cache
	// For now, we'll just log it
	rs.logger.InfoContext(ctx, "Storing conversation turn",
		slog.String("user_id", userID),
		slog.String("role", role),
		slog.String("message", message[:min(100, len(message))]),
		slog.Any("metadata", metadata))

	span.SetStatus(codes.Ok, "Conversation turn stored")
	return nil
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
