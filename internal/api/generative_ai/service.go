package generativeAI

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

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
	apiKey := os.Getenv("GOOGLE_GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GOOGLE_GEMINI_API_KEY environment variable is not set")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}
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
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  os.Getenv("GOOGLE_GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}
	result, err := client.Models.GenerateContent(ctx, *model, genai.Text(prompt), config)
	if err != nil {
		log.Fatal(err)
	}
	return result.Text(), nil
}

func (ai *AIClient) GenerateResponse(ctx context.Context, prompt string, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	chat, err := ai.client.Chats.Create(ctx, ai.model, config, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}
	return chat.SendMessage(ctx, genai.Part{Text: prompt})
}

func (ai *AIClient) StartChatSession(ctx context.Context, config *genai.GenerateContentConfig) (*ChatSession, error) {
	//config = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}
	chat, err := ai.client.Chats.Create(ctx, ai.model, config, nil)
	if err != nil {
		return nil, err
	}
	return &ChatSession{chat: chat}, nil
}

func (cs *ChatSession) SendMessage(ctx context.Context, message string) (string, error) {
	result, err := cs.chat.SendMessage(ctx, genai.Part{Text: message})
	if err != nil {
		return "", err
	}
	return result.Text(), nil
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
