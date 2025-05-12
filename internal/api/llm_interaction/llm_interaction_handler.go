package llmInteraction

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	generativeAI "github.com/FACorreiaa/go-poi-au-suggestions/internal/api/generative_ai"

	"google.golang.org/genai"
)

func RunLLM(ctx context.Context) {
	aiClient, err := generativeAI.NewAIClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	config := &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}
	response, err := aiClient.GenerateResponse(ctx, "return only the json format. be more precise as possible. Points of interest in Berlin in a format like:"+"{'name':lallala, 'latitude':'', longitude: ''}", config)
	if err != nil {
		log.Fatal(err)
	}
	for _, candidate := range response.Candidates {
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			log.Println("Candidate has no content or parts.")
			continue
		}

		part := candidate.Content.Parts[0]
		txt := part.Text

		if txt != "" {
			log.Printf("Extracted text: [%s]\n", txt)
			pois := []string{"give me some POIs in Berlin"}
			if err := json.Unmarshal([]byte(txt), &pois); err != nil {
				log.Printf("Failed to unmarshal AI response text into POIs: %v. Text was: %s\n", err, txt)
			} else {
				fmt.Println("POIs (successfully unmarshalled):", pois)
			}
		} else {
			log.Println("Part's text was empty.")
		}
	}
}
