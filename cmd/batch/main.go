// Package main serves as the entry point for the offline batch assessment tool.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/iuriikogan/multi-agent-cra/internal/batch"
	"github.com/iuriikogan/multi-agent-cra/pkg/config"
	"github.com/iuriikogan/multi-agent-cra/pkg/logger"
	"github.com/iuriikogan/multi-agent-cra/pkg/observability"
)

// main initializes dependencies and executes the batch analysis workflow.
func main() {
	cfg := config.Load()

	mode := flag.String("mode", "batch", "The execution mode (batch or server)")
	project := flag.String("project", "", "GCP Project ID")
	folder := flag.String("folder", "", "GCP Folder ID")
	org := flag.String("org", "", "GCP Organization ID")
	logLevel := flag.String("log-level", cfg.LogLevel, "Log level (DEBUG, INFO, WARN, ERROR)")

	flag.StringVar(&cfg.Models.Aggregator, "model-aggregator", cfg.Models.Aggregator, "Model for ResourceAggregator agent")
	flag.StringVar(&cfg.Models.Modeler, "model-modeler", cfg.Models.Modeler, "Model for CRAModeler agent")
	flag.StringVar(&cfg.Models.Validator, "model-validator", cfg.Models.Validator, "Model for ComplianceValidator agent")
	flag.StringVar(&cfg.Models.Reviewer, "model-reviewer", cfg.Models.Reviewer, "Model for Reviewer agent")
	flag.StringVar(&cfg.Models.Tagger, "model-tagger", cfg.Models.Tagger, "Model for ResourceTagger agent")
	flag.StringVar(&cfg.Models.VisualReporter, "model-reporter", cfg.Models.VisualReporter, "Model for VisualReporter agent")

	flag.Parse()

	logger.Setup(*logLevel, cfg.ProjectID)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := observability.InitTrace(ctx, cfg.ProjectID); err != nil {
		slog.Error("Failed to initialize tracing", "error", err)
	}
	defer observability.Shutdown(context.Background())

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

	apiKey := cfg.APIKey
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
	} else if envProject := cfg.ProjectID; envProject != "" {
		scope = fmt.Sprintf("projects/%s", envProject)
	} else if *folder != "" {
		scope = fmt.Sprintf("folders/%s", *folder)
	} else if *org != "" {
		scope = fmt.Sprintf("organizations/%s", *org)
	}

	batch.Run(ctx, client, apiKey, scope, cfg.Models)
}
