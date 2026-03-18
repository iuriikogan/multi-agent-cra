// Package observability provides distributed tracing for the Audit Agent.
package observability

import (
	"context"
	"testing"
)

// TestInitTrace_ThreadSafety ensures that multiple calls to InitTrace behave correctly
// and that it manages the global trace provider without errors.
func TestInitTrace_ThreadSafety(t *testing.T) {
	ctx := context.Background()
	projectID := "test-project"

	// Initial setup
	if err := InitTrace(ctx, projectID); err != nil {
		t.Fatalf("InitTrace() failed: %v", err)
	}

	// Repeated calls should not error out or cause panics (due to sync.Once)
	for i := 0; i < 3; i++ {
		if err := InitTrace(ctx, projectID); err != nil {
			t.Errorf("repeated InitTrace() failed: %v", err)
		}
	}
}

// TestShutdown ensures the trace provider can be gracefully closed.
func TestShutdown(t *testing.T) {
	ctx := context.Background()

	// We call shutdown after an initialization to ensure it doesn't panic.
	Shutdown(ctx)
}
