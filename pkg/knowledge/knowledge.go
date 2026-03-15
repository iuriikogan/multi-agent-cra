// Package knowledge provides vector search capabilities over the embedded CRA knowledge base.
package knowledge

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/google/generative-ai-go/genai"
)

//go:embed cra_kb.json dora_kb.json
var f embed.FS

type Regulation string

const (
	RegulationCRA  Regulation = "CRA"
	RegulationDORA Regulation = "DORA"
)

// Chunk represents a single piece of text and its vector embedding.
type Chunk struct {
	Text      string    `json:"text"`
	Embedding []float32 `json:"embedding"`
	Score     float32   `json:"score,omitempty"`
}

var knowledgeBases = make(map[Regulation][]Chunk)

// Init loads the embedded knowledge bases into memory.
func Init() error {
	if err := loadKB(RegulationCRA, "cra_kb.json"); err != nil {
		return err
	}
	return loadKB(RegulationDORA, "dora_kb.json")
}

func loadKB(reg Regulation, filename string) error {
	data, err := f.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read embedded %s: %w", filename, err)
	}
	var kb []Chunk
	if err := json.Unmarshal(data, &kb); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", filename, err)
	}
	knowledgeBases[reg] = kb
	return nil
}

// Search performs a cosine similarity search against the specified knowledge base for the given query.
func Search(ctx context.Context, client *genai.Client, query string, reg Regulation, topN int) ([]Chunk, error) {
	if len(knowledgeBases[reg]) == 0 {
		if err := Init(); err != nil {
			return nil, err
		}
	}

	kb := knowledgeBases[reg]
	if len(kb) == 0 {
		return nil, fmt.Errorf("knowledge base for %s is empty or not found", reg)
	}

	if client == nil {
		return nil, fmt.Errorf("genai client is nil")
	}

	model := client.EmbeddingModel("gemini-embedding-001")
	res, err := model.EmbedContent(ctx, genai.Text(query))
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	queryEmb := res.Embedding.Values
	results := make([]Chunk, len(kb))
	copy(results, kb)

	for i := range results {
		results[i].Score = cosineSimilarity(queryEmb, results[i].Embedding)
	}

	// Sort by similarity score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topN {
		results = results[:topN]
	}

	return results, nil
}

// cosineSimilarity calculates the cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
