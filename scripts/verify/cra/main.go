package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"github.com/iuriikogan/Audit-Agent/pkg/knowledge"
	"google.golang.org/api/option"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable is required")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer func() {
		_ = client.Close()
	}()

	fmt.Println("Searching for: 'reporting obligations' in CRA...")
	chunks, err := knowledge.Search(ctx, client, "reporting obligations", knowledge.RegulationCRA, 2)
	if err != nil {
		log.Fatalf("search failed: %v", err)
	}

	for _, c := range chunks {
		fmt.Printf("[Score: %.4f]\n%s\n\n", c.Score, c.Text)
	}
}
