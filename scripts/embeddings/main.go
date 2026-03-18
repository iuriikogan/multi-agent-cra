package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/genai"
)

type Chunk struct {
	Text      string    `json:"text"`
	Embedding []float32 `json:"embedding"`
}

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable is not set")
	}

	inputFile := os.Getenv("EMBEDDING_INPUT_FILE")
	if inputFile == "" {
		log.Fatal("EMBEDDING_INPUT_FILE environment variable is not set")
	}

	outputFile := os.Getenv("EMBEDDING_OUTPUT_FILE")
	if outputFile == "" {
		log.Fatal("EMBEDDING_OUTPUT_FILE environment variable is not set")
	}

	modelName := os.Getenv("EMBEDDING_MODEL")
	if modelName == "" {
		modelName = "text-embedding-004"
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		log.Fatalf("Failed to create genai client: %v", err)
	}

	content, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file %s: %v", inputFile, err)
	}

	text := string(content)
	// Simple chunking strategy: split by double newline or roughly 1000 characters
	rawChunks := strings.Split(text, "\n\n")
	var chunks []string
	for _, c := range rawChunks {
		c = strings.TrimSpace(c)
		if len(c) < 50 {
			continue
		}
		// Further split large chunks
		if len(c) > 2000 {
			subChunks := splitLongText(c, 1500)
			chunks = append(chunks, subChunks...)
		} else {
			chunks = append(chunks, c)
		}
	}

	fmt.Printf("Generating embeddings for %d chunks using model %s...\n", len(chunks), modelName)

	var knowledgeBase []Chunk

	for i, c := range chunks {
		fmt.Printf("Embedding chunk %d/%d...\n", i+1, len(chunks))
		res, err := client.Models.EmbedContent(ctx, modelName, genai.Text(c), nil)
		if err != nil {
			log.Fatalf("Failed to embed chunk %d: %v", i, err)
		}
		knowledgeBase = append(knowledgeBase, Chunk{
			Text:      c,
			Embedding: res.Embeddings[0].Values,
		})
	}

	data, err := json.MarshalIndent(knowledgeBase, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal knowledge base: %v", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		log.Fatalf("Failed to write to %s: %v", outputFile, err)
	}

	fmt.Printf("Successfully generated %s\n", outputFile)
}

func splitLongText(text string, limit int) []string {
	var result []string
	words := strings.Fields(text)
	var current strings.Builder
	for _, word := range words {
		if current.Len()+len(word)+1 > limit {
			result = append(result, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString(" ")
		}
		current.WriteString(word)
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}
