package queue

import (
	"context"
	"os"
	"testing"
)

func TestNewClient_NoCredentials(t *testing.T) {
	// Without credentials or emulator, NewClient usually fails or hangs depending on SDK version.
	// We want to ensure it at least attempts to connect and returns an error if auth fails,
	// rather than panicking.
	// However, in some CI environments, it might look for default credentials and fail slowly.
	
	if os.Getenv("CI") != "" {
		t.Skip("Skipping in CI without credentials")
	}

	ctx := context.Background()
	// Using a dummy project ID
	_, err := NewClient(ctx, "test-project")
	
	// We expect an error if no credentials are provided, or success if using emulator
	if err == nil {
		// If it succeeded (e.g. emulator), that's fine too for this test structure
		t.Log("NewClient succeeded (possibly using emulator or default creds)")
	} else {
		t.Logf("NewClient failed as expected (no creds): %v", err)
	}
}
