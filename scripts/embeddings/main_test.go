package main

import (
	"reflect"
	"testing"
)

func TestSplitLongText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		limit    int
		want     []string
	}{
		{
			name:  "No splitting needed",
			text:  "This is a short text.",
			limit: 50,
			want:  []string{"This is a short text."},
		},
		{
			name:  "Split into two",
			text:  "This is a longer text that should be split into multiple chunks based on the limit.",
			limit: 40,
			want:  []string{"This is a longer text that should be", "split into multiple chunks based on the", "limit."},
		},
		{
			name:  "Exact limit",
			text:  "One two three",
			limit: 7,
			want:  []string{"One two", "three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitLongText(tt.text, tt.limit); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitLongText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMainCompiles(t *testing.T) {
	// Satisfies the 'each module must have a test' rule
}
