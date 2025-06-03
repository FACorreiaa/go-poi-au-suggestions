package generativeAI

import (
	"context"
	"flag"
	"fmt"
	"iter"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genai"
)

var model = flag.String("model", "gemini-2.0-flash", "the model name, e.g. gemini-2.0-flash")

type AIClient struct {
	client *genai.Client
	model  string
}

type ChatSession struct {
	chat *genai.Chat
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
