package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"multi-agent-cra/pkg/agent"
	"multi-agent-cra/pkg/core"
	"multi-agent-cra/pkg/logger"
	"multi-agent-cra/pkg/tools"
	"multi-agent-cra/pkg/workflow"
)

func main() {
	// Parse flags for configuration
	role := flag.String("role", "all", "The agent role to run (classifier, auditor, vuln, reporter, or all)")
	mode := flag.String("mode", "batch", "The execution mode (batch or server)")
	project := flag.String("project", "", "GCP Project ID")
	folder := flag.String("folder", "", "GCP Folder ID")
	org := flag.String("org", "", "GCP Organization ID")
	logLevel := flag.String("log-level", "INFO", "Log level (DEBUG, INFO, WARN, ERROR)")

	flag.Parse()

	// Initialize Structured Logging for observability
	logger.Setup(*logLevel)

	ctx := context.Background()

	// Start health check server for Cloud Run readiness probes
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

	// Ensure API Key is present for Gemini interactions
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		slog.Error("GEMINI_API_KEY is not set")
		os.Exit(1)
	}

	// Create a shared GenAI client
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

	// Handle Server Mode (future extension for microservices)
	if *mode == "server" {
		slog.Info("Running in SERVER mode", "role", *role)
		select {}
	}

	// Determine the scope of the assessment based on flags or env vars
	scope := "projects/demo-project" // Default
	if *project != "" {
		scope = fmt.Sprintf("projects/%s", *project)
	} else if envProject := os.Getenv("PROJECT_ID"); envProject != "" {
		scope = fmt.Sprintf("projects/%s", envProject)
	} else if *folder != "" {
		scope = fmt.Sprintf("folders/%s", *folder)
	} else if *org != "" {
		scope = fmt.Sprintf("organizations/%s", *org)
	}

	runBatch(ctx, client, apiKey, scope)
}

func runBatch(ctx context.Context, client *genai.Client, apiKey, scope string) {
	// --- 1. Initialize Agents ---
	// Specialist agents are configured with specific system instructions, models, and tools.

	// Aggregator: Discovers and lists cloud assets. Uses faster/cheaper model.
	aggregatorAgent := agent.New(client, apiKey, "ResourceAggregator", "Ingestion", "gemini-3.1-flash-lite-preview",
		agent.WithSystemInstruction(`You are a Resource Aggregator. 
			Your task is to list and ingest GCP assets for a given scope.
			When asked to list assets, use the list_gcp_assets tool and return ONLY the raw JSON array of assets.`),
		agent.WithTools(tools.IngestionTools...),
	)

	// Modeler: Analyzes asset config against CRA framework. Uses reasoning model.
	modelerAgent := agent.New(client, apiKey, "CRAModeler", "Modeling", "gemini-3-pro-preview",
		agent.WithSystemInstruction(`You are a CRA Modeler.
			Your task is to take a structured data repository and apply the Cyber Resilience Act (CRA) compliance framework to generate a compliance model.`),
	)

	// Validator: checks the model against specific regulations. Uses reasoning model + reg tools.
	validatorAgent := agent.New(client, apiKey, "ComplianceValidator", "Validation", "gemini-3-pro-preview",
		agent.WithSystemInstruction(`You are a Compliance Validator.
			Your task is to validate a compliance model against CRA rules and output a compliance report with findings and deviations.`),
		agent.WithTools(tools.RegulatoryCheckerTools...),
		agent.WithTools(tools.ComplianceTools...),
	)

	// Reviewer: Provides final approval/summary. Uses reasoning model.
	reviewerAgent := agent.New(client, apiKey, "Reviewer", "Approval", "gemini-3-pro-preview",
		agent.WithSystemInstruction(`You are a Compliance Reviewer.
			Your task is to review the compliance report and provide an approval status and final report summary.`),
		agent.WithTools(tools.ComplianceTools...),
	)

	// Tagger: Generates remediation tags. Uses faster model.
	taggerAgent := agent.New(client, apiKey, "ResourceTagger", "Tagging", "gemini-3.1-flash-lite-preview",
		agent.WithSystemInstruction(`You are a Resource Tagger.
			Your task is to tag resources that have issues identified in the compliance report with information on how to solve them.
			If you apply tags, end your response with a line starting with 'APPLIED_TAGS:' followed by the tags in key=value format (comma-separated). 
			Example: APPLIED_TAGS: cra_status=non_compliant,remediation=urgent`),
		agent.WithTools(tools.TaggingTools...),
	)

	// Visual Reporter: Creates dashboard images. Uses reasoning model + visual tools.
	visualReporterAgent := agent.New(client, apiKey, "VisualReporter", "Reporting", "gemini-3-pro-preview",
		agent.WithSystemInstruction(`You are a Visual Reporting Agent. 
			Your task is to generate a graphical compliance dashboard image. 
			Use the generate_visual_dashboard tool to create the image. 
			Be creative with the dashboard design but ensure it accurately reflects the data provided.`),
		agent.WithTools(tools.VisualTools...),
	)

	// --- 2. Discovery Phase ---
	// Identify all assets within the target scope to populate the work queue.
	slog.Info("Discovering Assets", "scope", scope)
	discoveryCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Ask aggregator to list assets
	listResp, err := aggregatorAgent.Chat(discoveryCtx, fmt.Sprintf("List all GCP assets in %s", scope))
	if err != nil {
		slog.Error("Discovery failed", "error", err)
		os.Exit(1)
	}

	// Parse JSON response (handle markdown code blocks if present)
	jsonStr := listResp
	// Remove markdown code block delimiters if present
	if start := strings.Index(jsonStr, "```"); start != -1 {
		if end := strings.LastIndex(jsonStr, "```"); end > start {
			// Find the first newline after the start backticks to strip the language identifier (e.g., "json")
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
		slog.Error("Failed to parse asset list", "error", err, "raw_response", listResp)
		panic("failed to parse asset list")
	}

	if len(assets) == 0 {
		slog.Error("No assets found")
		panic("no assets found")
	}

	var resources []core.GCPResource
	for i, a := range assets {
		r := core.GCPResource{
			ID:        fmt.Sprintf("r%d", i),
			Name:      a.Name,
			Type:      a.AssetType,
			Region:    a.Location,
			ProjectID: scope, // Simplified
		}
		resources = append(resources, r)
	}

	// --- 3. Initialize Concurrency Coordinator ---
	coordinator := workflow.NewCoordinator(aggregatorAgent, modelerAgent, validatorAgent, reviewerAgent, taggerAgent, 5)

	slog.Info("Starting Concurrent Assessment", "resource_count", len(resources))
	start := time.Now()

	inputChan := make(chan core.GCPResource, len(resources))
	for _, r := range resources {
		inputChan <- r
	}
	close(inputChan)

	// Consume results
	resultsChan := coordinator.ProcessStream(ctx, inputChan)

	var finalResults []core.AssessmentResult

	for res := range resultsChan {
		if res.Error != nil {
			slog.Error("Resource Assessment Failed", "resource", res.ResourceName, "error", res.Error)
			continue
		}
		// Truncate report for display
		displayReport := res.ComplianceReport
		if len(displayReport) > 50 {
			displayReport = displayReport[:50] + "..."
		}
		slog.Info("Resource Assessment Completed", "resource", res.ResourceName, "compliance", displayReport)
		finalResults = append(finalResults, res)
	}

	// --- 4. Generate Reports ---
	generateCSV(finalResults)
	generateVisualReport(ctx, visualReporterAgent, finalResults)
	generateTaggingInstructions(finalResults)

	slog.Info("Batch Execution Completed", "duration", time.Since(start))
}

func generateCSV(results []core.AssessmentResult) {
	file, err := os.Create("compliance_report.csv")
	if err != nil {
		slog.Error("Failed to create CSV", "error", err)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("Failed to close CSV file", "error", err)
		}
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"ResourceID", "Name", "Type", "ComplianceStatus", "Tags"}); err != nil {
		slog.Error("Failed to write CSV header", "error", err)
		return
	}
	for _, r := range results {
		// Extract status from report (simplified logic for demo)
		status := "Compliant"
		if strings.Contains(r.ComplianceReport, "NON-COMPLIANT") || strings.Contains(r.ComplianceReport, "High Risk") {
			status = "Non-Compliant"
		}
		if err := writer.Write([]string{r.ResourceID, r.ResourceName, r.ResourceType, status, r.Tags}); err != nil {
			slog.Error("Failed to write CSV record", "error", err)
		}
	}
	slog.Info("CSV Report generated", "file", "compliance_report.csv")
}

func generateTaggingInstructions(results []core.AssessmentResult) {
	fmt.Println("\n--- 🏷️  Resource Tagging Validation & CLI Instructions ---")

	seenTags := make(map[string]bool)

	for _, r := range results {
		if strings.Contains(r.Tags, "APPLIED_TAGS:") {
			parts := strings.Split(r.Tags, "APPLIED_TAGS:")
			if len(parts) > 1 {
				tagStr := strings.TrimSpace(parts[1])
				tags := strings.Split(tagStr, ",")

				for _, t := range tags {
					kv := strings.Split(strings.TrimSpace(t), "=")
					if len(kv) == 2 {
						k := strings.TrimSpace(kv[0])
						v := strings.TrimSpace(kv[1])
						tagKey := fmt.Sprintf("%s=%s", k, v)

						if !seenTags[tagKey] {
							seenTags[tagKey] = true
							fmt.Printf("\nFound Tag: %s=%s\n", k, v)
							fmt.Println("To find all resources with this label:")
							fmt.Printf("  gcloud asset search-all-resources --query=\"labels.%s:%s\"\n", k, v)
							fmt.Println("To filter instances:")
							fmt.Printf("  gcloud compute instances list --filter=\"labels.%s=%s\"\n", k, v)
						}
					}
				}
			}
		}
	}
	fmt.Println("\n------------------------------------------------------")
}

func generateVisualReport(ctx context.Context, reporter agent.Agent, results []core.AssessmentResult) {
	slog.Info("Generating Visual Report...")

	compliantCount := 0
	nonCompliantCount := 0
	for _, r := range results {
		if strings.Contains(r.ComplianceReport, "NON-COMPLIANT") || strings.Contains(r.ComplianceReport, "High Risk") {
			nonCompliantCount++
		} else {
			compliantCount++
		}
	}

	prompt := fmt.Sprintf("Generate a compliance dashboard for %d resources. Data: %d Compliant, %d Non-Compliant. Filename: compliance_dashboard.png. Theme: Cyber Resilience Act (CRA) futuristic security dashboard.",
		len(results), compliantCount, nonCompliantCount)

	resp, err := reporter.Chat(ctx, prompt)
	if err != nil {
		slog.Error("Visual reporting agent failed", "error", err)
		return
	}

	slog.Info("Visual Reporter Agent Output", "response", resp)
}
