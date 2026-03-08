package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"multi-agent-cra/pkg/agent"
	"multi-agent-cra/pkg/config"
	"multi-agent-cra/pkg/core"
	"multi-agent-cra/pkg/logger"
	"multi-agent-cra/pkg/queue"
	"multi-agent-cra/pkg/tools"
	"multi-agent-cra/pkg/workflow"
)

func main() {
	cfg := config.Load()
	logger.Setup(cfg.LogLevel)

	ctx := context.Background()

	// --- Cloud Run Health Check Requirement ---
	// Start a dummy HTTP server to satisfy Cloud Run's container startup probe.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Worker is running")
		})
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK")
		})
		
		slog.Info("Starting health check server", "port", port)
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			slog.Error("Health check server failed", "error", err)
			os.Exit(1)
		}
	}()
	// ------------------------------------------

	// Initialize Pub/Sub Client
	pubsubClient, err := queue.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		slog.Error("Failed to initialize Pub/Sub client", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := pubsubClient.Close(); err != nil {
			slog.Error("Failed to close Pub/Sub client", "error", err)
		}
	}()

	// Initialize GenAI Client
	genaiClient, err := genai.NewClient(ctx, option.WithAPIKey(cfg.APIKey))
	if err != nil {
		slog.Error("Failed to create GenAI client", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := genaiClient.Close(); err != nil {
			slog.Error("Failed to close GenAI client", "error", err)
		}
	}()

	// Initialize Agents
	aggregatorAgent := agent.New(genaiClient, cfg.APIKey, "ResourceAggregator", "Ingestion", "gemini-3.1-flash-lite-preview",
		agent.WithSystemInstruction(`You are a Resource Aggregator. 
			Your task is to list and ingest GCP assets for a given scope.
			When asked to list assets, use the list_gcp_assets tool and return ONLY the raw JSON array of assets.`),
		agent.WithTools(tools.IngestionTools...),
	)

	modelerAgent := agent.New(genaiClient, cfg.APIKey, "CRAModeler", "Modeling", "gemini-3-pro-preview",
		agent.WithSystemInstruction(`You are a CRA Modeler.
			Your task is to take a structured data repository and apply the Cyber Resilience Act (CRA) compliance framework to generate a compliance model.`),
	)

	validatorAgent := agent.New(genaiClient, cfg.APIKey, "ComplianceValidator", "Validation", "gemini-3-pro-preview",
		agent.WithSystemInstruction(`You are a Compliance Validator.
			Your task is to validate a compliance model against CRA rules and output a compliance report with findings and deviations.`),
		agent.WithTools(tools.RegulatoryCheckerTools...),
		agent.WithTools(tools.ComplianceTools...),
	)

	reviewerAgent := agent.New(genaiClient, cfg.APIKey, "Reviewer", "Approval", "gemini-3-pro-preview",
		agent.WithSystemInstruction(`You are a Compliance Reviewer.
			Your task is to review the compliance report and provide an approval status and final report summary.`),
		agent.WithTools(tools.ComplianceTools...),
	)

	taggerAgent := agent.New(genaiClient, cfg.APIKey, "ResourceTagger", "Tagging", "gemini-3.1-flash-lite-preview",
		agent.WithSystemInstruction(`You are a Resource Tagger.
			Your task is to tag resources that have issues identified in the compliance report with information on how to solve them.
			If you apply tags, end your response with a line starting with 'APPLIED_TAGS:' followed by the tags in key=value format (comma-separated). 
			Example: APPLIED_TAGS: cra_status=non_compliant,remediation=urgent`),
		agent.WithTools(tools.TaggingTools...),
	)

	coordinator := workflow.NewCoordinator(aggregatorAgent, modelerAgent, validatorAgent, reviewerAgent, taggerAgent, 5)

	// Semaphore to limit concurrent scans
	sem := make(chan struct{}, 10)

	// Subscribe to scan requests
	err = pubsubClient.Subscribe(ctx, cfg.PubSub.SubScanRequests, func(ctx context.Context, data []byte) error {
		// Acquire semaphore
		sem <- struct{}{}
		defer func() { <-sem }()

		var job struct {
			JobID string `json:"job_id"`
			Scope string `json:"scope"`
		}
		if err := json.Unmarshal(data, &job); err != nil {
			return fmt.Errorf("failed to parse job: %w", err)
		}

		slog.Info("Processing scan request", "job_id", job.JobID, "scope", job.Scope)
		
		// Run discovery and workflow
		return runScan(ctx, coordinator, aggregatorAgent, job.Scope)
	})

	if err != nil {
		slog.Error("Subscription failed", "error", err)
		os.Exit(1)
	}
}

func runScan(ctx context.Context, coordinator *workflow.Coordinator, aggregator agent.Agent, scope string) error {
	// 1. Discovery
	slog.Info("Discovering Assets", "scope", scope)
	discoveryCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	listResp, err := aggregator.Chat(discoveryCtx, fmt.Sprintf("List all GCP assets in %s", scope))
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	// Parse JSON
	jsonStr := listResp
	if start := strings.Index(jsonStr, "```"); start != -1 {
		if end := strings.LastIndex(jsonStr, "```"); end > start {
			contentStart := start + 3
			if newline := strings.Index(jsonStr[contentStart:], "\n"); newline != -1 {
				contentStart += newline + 1
			}
			jsonStr = jsonStr[contentStart:end]
		}
	}
	jsonStr = strings.TrimSpace(jsonStr)

	type Asset struct {
		Name      string `json:"name"`
		AssetType string `json:"asset_type"`
		Location  string `json:"location"`
	}
	var assets []Asset
	if err := json.Unmarshal([]byte(jsonStr), &assets); err != nil {
		return fmt.Errorf("failed to parse asset list: %w", err)
	}

	if len(assets) == 0 {
		return fmt.Errorf("no assets found")
	}

	var resources []core.GCPResource
	for i, a := range assets {
		r := core.GCPResource{
			ID:        fmt.Sprintf("r%d", i),
			Name:      a.Name,
			Type:      a.AssetType,
			Region:    a.Location,
			ProjectID: scope,
		}
		resources = append(resources, r)
	}

	// 2. Process
	inputChan := make(chan core.GCPResource, len(resources))
	for _, r := range resources {
		inputChan <- r
	}
	close(inputChan)

	resultsChan := coordinator.ProcessStream(ctx, inputChan)

	for res := range resultsChan {
		if res.Error != nil {
			slog.Error("Resource Assessment Failed", "resource", res.ResourceName, "error", res.Error)
			continue
		}
		// TODO: Publish result to 'validation-results' or save to Firestore
		slog.Info("Resource Assessment Completed", "resource", res.ResourceName, "status", res.ApprovalStatus)
	}

	return nil
}
