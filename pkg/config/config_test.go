// Package config provides application settings for the Audit Agent.
package config

import (
	"os"
	"testing"
)

// TestLoad verifies that environment variables are correctly mapped to the Config object.
func TestLoad(t *testing.T) {
	// Mock environment
	t.Setenv("PROJECT_ID", "test-id")
	t.Setenv("REGION", "europe-west1")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("GEMINI_API_KEY", "fake-key")
	t.Setenv("DATABASE_URL", "mysql://...")
	t.Setenv("PORT", "9090")

	cfg := Load()

	if cfg.ProjectID != "test-id" {
		t.Errorf("expected ProjectID test-id, got %s", cfg.ProjectID)
	}
	if cfg.Region != "europe-west1" {
		t.Errorf("expected Region europe-west1, got %s", cfg.Region)
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("expected LogLevel DEBUG, got %s", cfg.LogLevel)
	}
	if cfg.APIKey != "fake-key" {
		t.Errorf("expected APIKey fake-key, got %s", cfg.APIKey)
	}
	if cfg.Server.Port != "9090" {
		t.Errorf("expected Port 9090, got %s", cfg.Server.Port)
	}
}

// TestLoad_Defaults verifies the presence of fallback values for optional environment variables.
func TestLoad_Defaults(t *testing.T) {
	// Unset environment
	_ = os.Unsetenv("PROJECT_ID")
	_ = os.Unsetenv("LOG_LEVEL")
	_ = os.Unsetenv("PORT")

	cfg := Load()

	// LOG_LEVEL should default to INFO
	if cfg.LogLevel != "INFO" {
		t.Errorf("expected default LogLevel INFO, got %s", cfg.LogLevel)
	}

	// Port should default to 8080
	if cfg.Server.Port != "8080" {
		t.Errorf("expected default Port 8080, got %s", cfg.Server.Port)
	}

	// Models should have defaults
	if cfg.Models.Aggregator == "" {
		t.Errorf("expected default model for aggregator, got empty")
	}
}
