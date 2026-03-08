package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Set environment variables for testing
	os.Setenv("PROJECT_ID", "test-project")
	os.Setenv("REGION", "us-west1")
	os.Setenv("LOG_LEVEL", "DEBUG")
	os.Setenv("GEMINI_API_KEY", "test-key")
	os.Setenv("PUBSUB_TOPIC_SCAN_REQUESTS", "test-topic")
	os.Setenv("PORT", "9090")
	defer func() {
		os.Unsetenv("PROJECT_ID")
		os.Unsetenv("REGION")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("GEMINI_API_KEY")
		os.Unsetenv("PUBSUB_TOPIC_SCAN_REQUESTS")
		os.Unsetenv("PORT")
	}()

	cfg := Load()

	if cfg.ProjectID != "test-project" {
		t.Errorf("expected ProjectID 'test-project', got %s", cfg.ProjectID)
	}
	if cfg.Region != "us-west1" {
		t.Errorf("expected Region 'us-west1', got %s", cfg.Region)
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("expected LogLevel 'DEBUG', got %s", cfg.LogLevel)
	}
	if cfg.PubSub.TopicScanRequests != "test-topic" {
		t.Errorf("expected PubSub topic 'test-topic', got %s", cfg.PubSub.TopicScanRequests)
	}
	if cfg.Server.Port != "9090" {
		t.Errorf("expected Server port '9090', got %s", cfg.Server.Port)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Ensure environment is clean
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("PORT")

	cfg := Load()

	if cfg.LogLevel != "INFO" {
		t.Errorf("expected default LogLevel 'INFO', got %s", cfg.LogLevel)
	}
	if cfg.Server.Port != "8080" {
		t.Errorf("expected default Server port '8080', got %s", cfg.Server.Port)
	}
}
