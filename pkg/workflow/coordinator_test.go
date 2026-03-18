// Package workflow provides testing for the agent orchestration logic.
package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/iuriikogan/Audit-Agent/pkg/agent"
	"github.com/iuriikogan/Audit-Agent/pkg/core"
)

// Ensure MockAgent strictly fulfills the agent.Agent interface.
var _ agent.Agent = (*MockAgent)(nil)

// MockAgent facilitates controlled testing of the coordinator pipeline.
type MockAgent struct {
	NameFunc  func() string
	RoleFunc  func() string
	ChatFunc  func(ctx context.Context, input string) (string, error)
	CloseFunc func() error
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

func (m *MockAgent) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// TestCoordinator_ProcessStream_Success validates the full sequential pipeline
// of a single resource assessment under ideal conditions.
func TestCoordinator_ProcessStream_Success(t *testing.T) {
	// Setup mock agent with stage-aware responses.
	successAgent := &MockAgent{
		ChatFunc: func(ctx context.Context, input string) (string, error) {
			switch {
			case strings.Contains(input, "Ingest"):
				return "config-data", nil
			case strings.Contains(input, "Model"):
				return "cra-model", nil
			case strings.Contains(input, "Validate"):
				return "passed-validation", nil
			case strings.Contains(input, "Review"):
				return "Approved", nil
			case strings.Contains(input, "Suggest GCP tags"):
				return "env:prod", nil
			default:
				return "unknown", nil
			}
		},
	}

	coordinator := NewCoordinator(successAgent, successAgent, successAgent, successAgent, successAgent, 2)

	inputChan := make(chan core.GCPResource, 1)
	inputChan <- core.GCPResource{ID: "res-1", Name: "TestInstance", Type: "GCE", ProjectID: "test-proj"}
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
	if res.Error != nil {
		t.Errorf("unexpected pipeline error: %v", res.Error)
	}
	if res.ComplianceModel != "cra-model" {
		t.Errorf("unexpected model: %s", res.ComplianceModel)
	}
	if res.ApprovalStatus != "Approved" {
		t.Errorf("unexpected approval: %s", res.ApprovalStatus)
	}
}

// TestCoordinator_ProcessStream_Failure validates the error handling logic
// when an individual agent in the pipeline returns an error.
func TestCoordinator_ProcessStream_Failure(t *testing.T) {
	failAgent := &MockAgent{
		ChatFunc: func(ctx context.Context, input string) (string, error) {
			return "", errors.New("simulated agent crash")
		},
	}
	successAgent := &MockAgent{}

	// Aggregator fails immediately.
	coordinator := NewCoordinator(failAgent, successAgent, successAgent, successAgent, successAgent, 1)

	inputChan := make(chan core.GCPResource, 1)
	inputChan <- core.GCPResource{ID: "res-fail", Name: "BrokenInstance"}
	close(inputChan)

	resultsChan := coordinator.ProcessStream(context.Background(), inputChan)
	res := <-resultsChan

	if res.Error == nil {
		t.Fatal("expected pipeline error, got nil")
	}
	if !strings.Contains(res.Error.Error(), "aggregation failed") {
		t.Errorf("unexpected error message: %v", res.Error)
	}
}

// TestCoordinator_ProcessStream_Concurrency ensures that the coordinator correctly
// utilizes multiple workers to process resources in parallel.
func TestCoordinator_ProcessStream_Concurrency(t *testing.T) {
	slowAgent := &MockAgent{
		ChatFunc: func(ctx context.Context, input string) (string, error) {
			time.Sleep(50 * time.Millisecond) // Simulate latency
			return "done", nil
		},
	}

	count := 10
	workers := 5
	coordinator := NewCoordinator(slowAgent, slowAgent, slowAgent, slowAgent, slowAgent, workers)

	inputChan := make(chan core.GCPResource, count)
	for i := 0; i < count; i++ {
		inputChan <- core.GCPResource{ID: fmt.Sprintf("r%d", i), Name: "Resource"}
	}
	close(inputChan)

	start := time.Now()
	resultsChan := coordinator.ProcessStream(context.Background(), inputChan)

	var results []core.AssessmentResult
	for res := range resultsChan {
		results = append(results, res)
	}
	duration := time.Since(start)

	if len(results) != count {
		t.Fatalf("expected %d results, got %d", count, len(results))
	}

	// 10 items / 5 workers * ~250ms (5 stages * 50ms) = ~500ms minimum expected.
	// We allow for some overhead, but verify it didn't run serially (~2.5s).
	if duration > 2*time.Second {
		t.Errorf("processing too slow (%v), concurrency might be broken", duration)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ResourceID < results[j].ResourceID
	})

	for i, res := range results {
		expectedID := fmt.Sprintf("r%d", i)
		if res.ResourceID != expectedID {
			t.Errorf("missing resource %s", expectedID)
		}
	}
}
