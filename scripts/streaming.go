package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"google.golang.org/genai"
)

var model = flag.String("model", "gemini-2.0-flash", "the model name, e.g. gemini-2.0-flash")

func chatStream(ctx context.Context) {
	apiKey := os.Getenv("GOOGLE_GEMINI_API_KEY")
	fmt.Println("GOOGLE_GEMINI_API_KEY:", apiKey)
	if apiKey == "" {
		fmt.Println("GOOGLE_GEMINI_API_KEY environment variable is not set")
		return
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})

	if err != nil {
		fmt.Println("Failed to create client:", err)
		return
	}
	if client.ClientConfig().Backend == genai.BackendVertexAI {
		fmt.Println("Calling VertexAI Backend...")
	} else {
		fmt.Println("Calling GeminiAPI Backend...")
	}

	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}

	// Create a new Chat.
	chat, err := client.Chats.Create(ctx, *model, config, nil)

	part := genai.Part{Text: "Give me a very very long text so I can evaluate if streaming works."}
	p := make([]genai.Part, 1)
	p[0] = part

	// Send first chat message.
	for result, err := range chat.SendMessageStream(ctx, p...) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Result text: %s\n", result.Text())
	}

	// Send second chat message.
	part = genai.Part{Text: "Add more text to see if streaming works."}

	for result, err := range chat.SendMessageStream(ctx, part) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Result text: %s\n", result.Text())
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found or error loading:", err)
	}
	ctx := context.Background()
	flag.Parse()
	chatStream(ctx)
}
