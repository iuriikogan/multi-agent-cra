package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"github.com/iuriikogan/multi-agent-cra/pkg/knowledge"
	"google.golang.org/api/option"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY not set")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	if err := knowledge.Init(); err != nil {
		log.Fatalf("knowledge.Init failed: %v", err)
	}

	query := "ICT risk management framework requirements"
	fmt.Printf("Searching DORA for: %s\n", query)
	chunks, err := knowledge.Search(ctx, client, query, knowledge.RegulationDORA, 3)
	if err != nil {
		log.Fatalf("knowledge.Search failed: %v", err)
	}

	fmt.Printf("Found %d results:\n", len(chunks))
	for i, chunk := range chunks {
		fmt.Printf("\n--- Result %d (Score: %.4f) ---\n", i+1, chunk.Score)
		fmt.Printf("%s\n", chunk.Text)
	}
}
