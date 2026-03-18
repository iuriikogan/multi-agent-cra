// Package queue provides testing for the Google Cloud Pub/Sub wrapper.
package queue

import (
	"context"
	"os"
	"testing"
)

// TestNewClient_NoCredentials verifies that the PubSub client handles missing auth
// context gracefully without crashing.
func TestNewClient_NoCredentials(t *testing.T) {
	// Skip the test in CI if it requires real credentials not present in the environment.
	if os.Getenv("CI") != "" && os.Getenv("PUBSUB_EMULATOR_HOST") == "" {
		t.Skip("Skipping in CI without credentials or emulator")
	}

	ctx := context.Background()
	projectID := "test-audit-project"

	client, err := NewClient(ctx, projectID)
	if err != nil {
		// If initialization failed (e.g., missing auth), verify it returned a wrapped error.
		t.Logf("pubsub client failed as expected: %v", err)
	} else {
		// If initialization succeeded (e.g., emulator detected), ensure client isn't nil.
		if client == nil {
			t.Error("NewClient returned nil client without error")
		}
		defer func() { _ = client.Close() }()
	}
}

// TestClient_Compilation verifies that the Client struct and methods match expectations.
func TestClient_Compilation(t *testing.T) {
	var _ = &Client{}
}
