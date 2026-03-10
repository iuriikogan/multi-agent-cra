package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/iuriikogan/multi-agent-cra/internal/batch"
	"github.com/iuriikogan/multi-agent-cra/pkg/logger"
)

func main() {
	mode := flag.String("mode", "batch", "The execution mode (batch or server)")
	project := flag.String("project", "", "GCP Project ID")
	folder := flag.String("folder", "", "GCP Folder ID")
	org := flag.String("org", "", "GCP Organization ID")
	logLevel := flag.String("log-level", "INFO", "Log level (DEBUG, INFO, WARN, ERROR)")

	flag.Parse()

	logger.Setup(*logLevel)
	ctx := context.Background()

	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if _, err := fmt.Fprintln(w, "OK"); err != nil {
				slog.Error("Failed to write health check response", "error", err)
			}
		})
		slog.Info("Starting health check server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			slog.Error("Health check server failed", "error", err)
		}
	}()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		slog.Error("GEMINI_API_KEY is not set")
		os.Exit(1)
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		slog.Error("Failed to create GenAI client", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := client.Close(); err != nil {
			slog.Error("Failed to close GenAI client", "error", err)
		}
	}()

	if *mode == "server" {
		slog.Warn("Running in SERVER mode is deprecated for batch. Use cmd/server instead.")
	}

	scope := "projects/demo-project"
	if *project != "" {
		scope = fmt.Sprintf("projects/%s", *project)
	} else if envProject := os.Getenv("PROJECT_ID"); envProject != "" {
		scope = fmt.Sprintf("projects/%s", envProject)
	} else if *folder != "" {
		scope = fmt.Sprintf("folders/%s", *folder)
	} else if *org != "" {
		scope = fmt.Sprintf("organizations/%s", *org)
	}

	batch.Run(ctx, client, apiKey, scope)
}
