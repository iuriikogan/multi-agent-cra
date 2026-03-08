package logger

import (
	"log/slog"
	"testing"
)

func TestSetup(t *testing.T) {
	// Just verify it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Setup panicked: %v", r)
		}
	}()

	Setup("DEBUG")
	slog.Info("Test log message")
}
