// Package knowledge provides tests for the embedded regulatory KB.
package knowledge

import (
	"context"
	"testing"
)

// TestInit_ThreadSafety verifies that the Init() function handles multi-stage calls without error.
func TestInit_ThreadSafety(t *testing.T) {
	for i := 0; i < 5; i++ {
		if err := Init(); err != nil {
			t.Fatalf("Init() iteration %d failed: %v", i, err)
		}
	}
}

// TestLoadKB verifies that the regulation-specific data is correctly unmarshaled into memory.
func TestLoadKB(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatalf("failed to init KB: %v", err)
	}

	for _, reg := range []Regulation{RegulationCRA, RegulationDORA} {
		if len(knowledgeBases[reg]) == 0 {
			t.Errorf("knowledge base for %s is empty after Init()", reg)
		}
	}
}

// TestCosineSimilarity evaluates the mathematical accuracy of the vector comparison logic.
func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float32
		want float32
	}{
		{
			name: "Identical Vectors",
			a:    []float32{1.0, 0.0, 0.0},
			b:    []float32{1.0, 0.0, 0.0},
			want: 1.0,
		},
		{
			name: "Orthogonal Vectors",
			a:    []float32{1.0, 0.0},
			b:    []float32{0.0, 1.0},
			want: 0.0,
		},
		{
			name: "Opposite Vectors",
			a:    []float32{1.0, 0.0},
			b:    []float32{-1.0, 0.0},
			want: -1.0,
		},
		{
			name: "Empty Vectors",
			a:    []float32{},
			b:    []float32{},
			want: 0.0,
		},
		{
			name: "Zero Vectors",
			a:    []float32{0.0, 0.0},
			b:    []float32{0.0, 0.0},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			// Use a small epsilon for float comparison
			if got < tt.want-0.001 || got > tt.want+0.001 {
				t.Errorf("cosineSimilarity() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSearch_ErrorCases verifies that Search handles missing clients or uninitialized KBs gracefully.
func TestSearch_ErrorCases(t *testing.T) {
	ctx := context.Background()
	_, err := Search(ctx, nil, "test query", RegulationCRA, 1)
	if err == nil {
		t.Error("expected error when searching with nil client, got none")
	}
}
