// Package knowledge provides semantic search capabilities over embedded regulatory frameworks.
//
// Rationale: By embedding the CRA and DORA legislation as vectorized JSON, agents can
// perform Retrieval-Augmented Generation (RAG) to ground their compliance findings in
// actual legal requirements without requiring an external vector database.
package knowledge

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"

	"google.golang.org/genai"
)

//go:embed cra_kb.json dora_kb.json
var f embed.FS

// Regulation defines the supported regulatory frameworks for compliance assessment.
type Regulation string

const (
	RegulationCRA  Regulation = "CRA"  // Cyber Resilience Act
	RegulationDORA Regulation = "DORA" // Digital Operational Resilience Act
)

// Chunk represents a discrete unit of regulatory text and its pre-computed vector embedding.
type Chunk struct {
	Text      string    `json:"text"`            // The raw text content of the article or requirement.
	Embedding []float32 `json:"embedding"`       // The vector representation for semantic search.
	Score     float32   `json:"score,omitempty"` // The similarity score populated during Search.
}

var (
	knowledgeBases = make(map[Regulation][]Chunk)
	initOnce       sync.Once
)

// Init triggers the loading and unmarshaling of embedded knowledge bases into memory.
// It is designed to be thread-safe and executes exactly once.
func Init() error {
	var err error
	initOnce.Do(func() {
		if e := loadKB(RegulationCRA, "cra_kb.json"); e != nil {
			err = fmt.Errorf("knowledge: failed to load CRA: %w", e)
			return
		}
		if e := loadKB(RegulationDORA, "dora_kb.json"); e != nil {
			err = fmt.Errorf("knowledge: failed to load DORA: %w", e)
			return
		}
	})
	return err
}

// loadKB reads an embedded JSON file and populates the internal memory store.
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

// Search performs a cosine similarity search against the specified regulatory knowledge base.
// It leverages the Google GenAI SDK to generate a query embedding and compares it to the stored chunks.
func Search(ctx context.Context, client *genai.Client, query string, reg Regulation, topN int) ([]Chunk, error) {
	// Ensure KB is initialized
	if len(knowledgeBases[reg]) == 0 {
		if err := Init(); err != nil {
			return nil, err
		}
	}

	kb := knowledgeBases[reg]
	if len(kb) == 0 {
		return nil, fmt.Errorf("knowledge: knowledge base for %s is empty or not found", reg)
	}

	if client == nil {
		return nil, fmt.Errorf("knowledge: genai client is required for query embedding")
	}

	// Generate embedding for the user's search query
	res, err := client.Models.EmbedContent(ctx, "text-embedding-004", genai.Text(query), nil)
	if err != nil {
		return nil, fmt.Errorf("knowledge: failed to generate query embedding: %w", err)
	}

	if len(res.Embeddings) == 0 {
		return nil, fmt.Errorf("knowledge: received empty embedding response")
	}

	queryEmb := res.Embeddings[0].Values

	// Pre-allocate results slice to improve performance
	results := make([]Chunk, len(kb))
	for i, c := range kb {
		results[i] = c
		results[i].Score = cosineSimilarity(queryEmb, c.Embedding)
	}

	// Rank results by similarity score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Truncate to the requested number of results
	if len(results) > topN {
		results = results[:topN]
	}

	return results, nil
}

// cosineSimilarity calculates the mathematical cosine similarity between two float vectors.
// Rationale: This enables semantic matching where text with similar meanings are mathematically close.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
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
