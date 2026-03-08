package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"multi-agent-cra/pkg/agent"
	"multi-agent-cra/pkg/core"
)

// Ensure MockAgent implements the interface
var _ agent.Agent = (*MockAgent)(nil)

// MockAgent implements agent.Agent for testing purposes.
type MockAgent struct {
	NameFunc func() string
	RoleFunc func() string
	ChatFunc func(ctx context.Context, input string) (string, error)
}

func (m *MockAgent) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-agent"
}

func (m *MockAgent) Role() string {
	if m.RoleFunc != nil {
		return m.RoleFunc()
	}
	return "mock-role"
}

func (m *MockAgent) Chat(ctx context.Context, input string) (string, error) {
	if m.ChatFunc != nil {
		return m.ChatFunc(ctx, input)
	}
	return "mock-response", nil
}

func TestCoordinator_ProcessStream_Success(t *testing.T) {
	// Setup mock agents that succeed
	successAgent := &MockAgent{
		ChatFunc: func(ctx context.Context, input string) (string, error) {
			if strings.Contains(input, "Ingest") {
				return "data-repo", nil
			}
			if strings.Contains(input, "Model") {
				return "compliance-model", nil
			}
			if strings.Contains(input, "Validate") {
				return "compliance-report", nil
			}
			if strings.Contains(input, "Review") {
				return "approved", nil
			}
			if strings.Contains(input, "Generate GCP labels") {
				return "tags-applied", nil
			}
			return "unknown", nil
		},
	}

	coordinator := NewCoordinator(successAgent, successAgent, successAgent, successAgent, successAgent, 2)

	inputChan := make(chan core.GCPResource, 1)
	inputChan <- core.GCPResource{ID: "r1", Name: "Test Resource", Type: "Compute", ProjectID: "p1"}
	close(inputChan)

	ctx := context.Background()
	resultsChan := coordinator.ProcessStream(ctx, inputChan)

	// Collect results
	var results []core.AssessmentResult
	for res := range resultsChan {
		results = append(results, res)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	res := results[0]
	if res.Error != nil {
		t.Errorf("unexpected error: %v", res.Error)
	}
	if res.ComplianceReport != "compliance-report" {
		t.Errorf("expected compliance report 'compliance-report', got %s", res.ComplianceReport)
	}
	if res.ApprovalStatus != "approved" {
		t.Errorf("expected approval status 'approved', got %s", res.ApprovalStatus)
	}
	if res.Tags != "tags-applied" {
		t.Errorf("expected tags 'tags-applied', got %s", res.Tags)
	}
}

func TestCoordinator_ProcessStream_Failure(t *testing.T) {
	// Setup agents where aggregator fails
	failAgent := &MockAgent{
		ChatFunc: func(ctx context.Context, input string) (string, error) {
			return "", errors.New("simulated error")
		},
	}
	successAgent := &MockAgent{}

	// Aggregator fails, others succeed (but shouldn't be called for that item)
	coordinator := NewCoordinator(failAgent, successAgent, successAgent, successAgent, successAgent, 1)

	inputChan := make(chan core.GCPResource, 1)
	inputChan <- core.GCPResource{ID: "r1", Name: "Fail Resource"}
	close(inputChan)

	ctx := context.Background()
	resultsChan := coordinator.ProcessStream(ctx, inputChan)

	var results []core.AssessmentResult
	for res := range resultsChan {
		results = append(results, res)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	res := results[0]
	if res.Error == nil {
		t.Error("expected error, got nil")
	}
	if !strings.Contains(res.Error.Error(), "aggregation failed") {
		t.Errorf("expected aggregation error, got %v", res.Error)
	}
}

func TestCoordinator_ProcessStream_Concurrency(t *testing.T) {
	// Setup a slow agent to verify concurrency
	slowAgent := &MockAgent{
		ChatFunc: func(ctx context.Context, input string) (string, error) {
			time.Sleep(100 * time.Millisecond)
			return "done", nil
		},
	}

	// 5 workers, 10 items. Should take roughly 2 * 100ms = 200ms (plus overhead)
	// Each item passes through 5 stages. Total time ~ 1s.
	coordinator := NewCoordinator(slowAgent, slowAgent, slowAgent, slowAgent, slowAgent, 5)

	count := 10
	inputChan := make(chan core.GCPResource, count)
	for i := 0; i < count; i++ {
		inputChan <- core.GCPResource{ID: fmt.Sprintf("r%d", i), Name: "Resource"}
	}
	close(inputChan)

	start := time.Now()
	ctx := context.Background()
	resultsChan := coordinator.ProcessStream(ctx, inputChan)

	var results []core.AssessmentResult
	for res := range resultsChan {
		results = append(results, res)
	}
	duration := time.Since(start)

	if len(results) != count {
		t.Fatalf("expected %d results, got %d", count, len(results))
	}

	if duration > 4*time.Second {
		t.Errorf("processing took too long (%v), concurrency might be broken", duration)
	}

	// Sort results by ID to ensure all processed
	sort.Slice(results, func(i, j int) bool {
		return results[i].ResourceID < results[j].ResourceID
	})

	for i, res := range results {
		expectedID := fmt.Sprintf("r%d", i)
		if res.ResourceID != expectedID {
			t.Errorf("expected result %d to be %s, got %s", i, expectedID, res.ResourceID)
		}
	}
}
