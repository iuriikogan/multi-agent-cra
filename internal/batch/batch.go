package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/iuriikogan/multi-agent-cra/pkg/agent"
	"github.com/iuriikogan/multi-agent-cra/pkg/config"
	"github.com/iuriikogan/multi-agent-cra/pkg/core"
	"github.com/iuriikogan/multi-agent-cra/pkg/tools"
	"github.com/iuriikogan/multi-agent-cra/pkg/workflow"
)

// Run executes the batch analysis workflow.
func Run(ctx context.Context, client *genai.Client, apiKey, scope string, models config.ModelsConfig) {
	aggregatorAgent := agent.New(client, apiKey, "ResourceAggregator", "Ingestion", models.Aggregator,
		agent.WithSystemInstruction(`You are a Resource Aggregator. 
			Your task is to list and ingest GCP assets for a given scope.
			When asked to list assets, use the list_gcp_assets tool and return ONLY the raw JSON array of assets.`),
		agent.WithTools(tools.IngestionTools...),
	)

	modelerAgent := agent.New(client, apiKey, "CRAModeler", "Modeling", models.Modeler,
		agent.WithSystemInstruction(`You are a CRA Modeler.
			Your task is to take a structured data repository and apply the Cyber Resilience Act (CRA) compliance framework to generate a compliance model.`),
	)

	validatorAgent := agent.New(client, apiKey, "ComplianceValidator", "Validation", models.Validator,
		agent.WithSystemInstruction(`You are a Compliance Validator.
			Your task is to validate a compliance model against CRA rules and output a compliance report with findings and deviations.`),
		agent.WithTools(tools.RegulatoryCheckerTools...),
		agent.WithTools(tools.ComplianceTools...),
	)

	reviewerAgent := agent.New(client, apiKey, "Reviewer", "Approval", models.Reviewer,
		agent.WithSystemInstruction(`You are a Compliance Reviewer.
			Your task is to review the compliance report and provide an approval status and final report summary.`),
		agent.WithTools(tools.ComplianceTools...),
	)

	taggerAgent := agent.New(client, apiKey, "ResourceTagger", "Tagging", models.Tagger,
		agent.WithSystemInstruction(`You are a Resource Tagger.
			Your task is to tag resources that have issues identified in the compliance report with information on how to solve them.
			If you apply tags, end your response with a line starting with 'APPLIED_TAGS:' followed by the tags in key=value format (comma-separated). 
			Example: APPLIED_TAGS: cra_status=non_compliant,remediation=urgent`),
		agent.WithTools(tools.TaggingTools...),
	)

	visualReporterAgent := agent.New(client, apiKey, "VisualReporter", "Reporting", models.VisualReporter,
		agent.WithSystemInstruction(`You are a Visual Reporting Agent. 
			Your task is to generate a graphical compliance dashboard image. 
			Use the generate_visual_dashboard tool to create the image. 
			Be creative with the dashboard design but ensure it accurately reflects the data provided.`),
		agent.WithTools(tools.VisualTools...),
	)

	slog.Info("Discovering Assets", "scope", scope)
	discoveryCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	listResp, err := aggregatorAgent.Chat(discoveryCtx, fmt.Sprintf("List all GCP assets in %s", scope))
	if err != nil {
		slog.Error("Discovery failed", "error", err)
		os.Exit(1)
	}

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
			ProjectID: scope,
		}
		resources = append(resources, r)
	}

	coordinator := workflow.NewCoordinator(aggregatorAgent, modelerAgent, validatorAgent, reviewerAgent, taggerAgent, 5)

	slog.Info("Starting Concurrent Assessment", "resource_count", len(resources))
	start := time.Now()

	inputChan := make(chan core.GCPResource, len(resources))
	for _, r := range resources {
		inputChan <- r
	}
	close(inputChan)

	resultsChan := coordinator.ProcessStream(ctx, inputChan)

	var finalResults []core.AssessmentResult

	for res := range resultsChan {
		if res.Error != nil {
			slog.Error("Resource Assessment Failed", "resource", res.ResourceName, "error", res.Error)
			continue
		}
		displayReport := res.ComplianceReport
		if len(displayReport) > 50 {
			displayReport = displayReport[:50] + "..."
		}
		slog.Info("Resource Assessment Completed", "resource", res.ResourceName, "compliance", displayReport)
		finalResults = append(finalResults, res)
	}

	GenerateCSV(finalResults)
	GenerateVisualReport(ctx, visualReporterAgent, finalResults)
	GenerateTaggingInstructions(finalResults)

	slog.Info("Batch Execution Completed", "duration", time.Since(start))
}
