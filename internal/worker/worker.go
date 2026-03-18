// Package worker implements the initialization and routing for background agent processes.
//
// Rationale: Workers act as the processing engine of the system. They consume Pub/Sub
// messages, initialize specialized Gemini agents, and execute the multi-stage
// compliance pipeline asynchronously.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/iuriikogan/Audit-Agent/pkg/agent"
	"github.com/iuriikogan/Audit-Agent/pkg/config"
	"github.com/iuriikogan/Audit-Agent/pkg/core"
	"github.com/iuriikogan/Audit-Agent/pkg/queue"
	"github.com/iuriikogan/Audit-Agent/pkg/store"
	"github.com/iuriikogan/Audit-Agent/pkg/tools"
	"github.com/iuriikogan/Audit-Agent/pkg/workflow"
	"google.golang.org/genai"
)

// RegisterRoutes configures HTTP handlers for Pub/Sub push subscriptions and initializes agents.
// It returns a cleanup function to gracefully close agent resources.
func RegisterRoutes(ctx context.Context, mux *http.ServeMux, cfg *config.Config, pubsubClient *queue.Client, db store.Store) (func(), error) {
	// Initialize the shared Google GenAI client.
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: cfg.APIKey})
	if err != nil {
		return nil, fmt.Errorf("worker: failed to create GenAI client: %w", err)
	}

	// 1. Aggregator Agent: Discovers GCP assets.
	aggregatorAgent := agent.New(genaiClient, cfg.APIKey, "ResourceAggregator", "Ingestion", cfg.Models.Aggregator,
		agent.WithSystemInstruction(`You are an expert Google Cloud Resource Aggregator. 
Fetch all relevant assets in the scope using the list_gcp_assets tool. 
Output MUST be a raw JSON array.`),
		agent.WithTools(tools.IngestionTools...),
	)

	// 2. Modeler Agent: Creates regulatory threat models.
	modelerAgent := agent.New(genaiClient, cfg.APIKey, "CRAModeler", "Modeling", cfg.Models.Modeler,
		agent.WithSystemInstruction(`You are a Cyber Resilience Act (CRA) Modeler. 
Analyze configurations and produce a structured threat model.`),
	)

	// 3. Validator Agent: Performs regulatory checks.
	validatorAgent := agent.New(genaiClient, cfg.APIKey, "ComplianceValidator", "Validation", cfg.Models.Validator,
		agent.WithSystemInstruction(`You are a Compliance Validator. 
Query the regulatory knowledge base and validate resource security postures.`),
		agent.WithTools(tools.RegulatoryCheckerTools...),
	)

	// 4. Reviewer Agent: Peer reviews findings.
	reviewerAgent := agent.New(genaiClient, cfg.APIKey, "Reviewer", "Approval", cfg.Models.Reviewer,
		agent.WithSystemInstruction(`You are a Compliance Reviewer. 
Issue a final verdict (APPROVED/REJECTED) based on the validation report.`),
	)

	// 5. Tagger Agent: Automated governance enforcement.
	taggerAgent := agent.New(genaiClient, cfg.APIKey, "ResourceTagger", "Tagging", cfg.Models.Tagger,
		agent.WithSystemInstruction(`You are a Resource Tagger. 
Apply GCP labels to identified resources using TaggingTools.`),
		agent.WithTools(tools.TaggingTools...),
	)

	// 6. Reporter Agent: Final summary generation.
	reporterAgent := agent.New(genaiClient, cfg.APIKey, "Reporter", "Reporting", cfg.Models.Reporter,
		agent.WithSystemInstruction(`You are a Compliance Reporter. 
Generate a structured JSON finding for the database.`),
	)

	cleanup := func() {
		_ = aggregatorAgent.Close()
		_ = modelerAgent.Close()
		_ = validatorAgent.Close()
		_ = reviewerAgent.Close()
		_ = taggerAgent.Close()
		_ = reporterAgent.Close()
	}

	wf := workflow.NewPubSubWorkflow(pubsubClient, db, cfg.PubSub.TopicMonitoring)

	// Register internal pipeline stages as Push handlers.
	wf.RegisterPushHandler(mux, "/pubsub/aggregator", cfg.PubSub.TopicModeler, aggregatorAgent, workflow.ProcessAggregation)
	wf.RegisterPushHandler(mux, "/pubsub/modeler", cfg.PubSub.TopicValidator, modelerAgent, workflow.ProcessModeling)
	wf.RegisterPushHandler(mux, "/pubsub/validator", cfg.PubSub.TopicReviewer, validatorAgent, workflow.ProcessValidation)
	wf.RegisterPushHandler(mux, "/pubsub/reviewer", cfg.PubSub.TopicTagger, reviewerAgent, workflow.ProcessReview)
	wf.RegisterPushHandler(mux, "/pubsub/tagger", cfg.PubSub.TopicReporter, taggerAgent, workflow.ProcessTagging)
	wf.RegisterPushHandler(mux, "/pubsub/reporter", "", reporterAgent, workflow.ProcessReporting)

	// Primary entrypoint for scan requests from the API.
	mux.HandleFunc("/pubsub/scan-requests", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer func() { _ = r.Body.Close() }()

		var req workflow.PushMessage
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		var job struct {
			JobID      string `json:"job_id"`
			Scope      string `json:"scope"`
			Regulation string `json:"regulation"`
		}
		if err := json.Unmarshal(req.Message.Data, &job); err != nil {
			slog.Error("worker: failed to parse job payload", "error", err)
			w.WriteHeader(http.StatusOK)
			return
		}

		slog.Info("worker: processing new scan request", "job_id", job.JobID, "scope", job.Scope)

		if err := db.CreateScan(r.Context(), job.JobID, job.Scope, job.Regulation); err != nil {
			slog.Error("worker: failed to register scan", "error", err)
			http.Error(w, "internal db error", http.StatusInternalServerError)
			return
		}

		// Discovery Phase: List resources and fan-out tasks to Pub/Sub.
		err = runDiscovery(r.Context(), cfg, pubsubClient, aggregatorAgent, job.Scope, job.JobID, job.Regulation)

		status := "completed"
		if err != nil {
			slog.Error("worker: discovery phase failed", "job_id", job.JobID, "error", err)
			status = "failed"
		}

		if err := db.UpdateScanStatus(r.Context(), job.JobID, status); err != nil {
			slog.Error("worker: failed to finalize scan status", "error", err)
		}

		w.WriteHeader(http.StatusOK)
	})

	return cleanup, nil
}

// runDiscovery lists GCP resources and publishes individual assessment tasks to the aggregator topic.
func runDiscovery(ctx context.Context, cfg *config.Config, pubsubClient *queue.Client, aggregator agent.Agent, scope, jobID, regulation string) error {
	discoveryCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	// Ask the aggregator agent to list assets.
	listResp, err := aggregator.Chat(discoveryCtx, fmt.Sprintf("List all GCP assets in scope: %s", scope))
	if err != nil {
		return fmt.Errorf("discovery chat failed: %w", err)
	}

	// Robustly parse the JSON array from the agent response.
	type Asset struct {
		Name      string `json:"name"`
		AssetType string `json:"asset_type"`
		Location  string `json:"location"`
	}
	var assets []Asset

	// Strip potential markdown artifacts.
	cleanJSON := listResp
	if start := strings.Index(listResp, "["); start != -1 {
		if end := strings.LastIndex(listResp, "]"); end > start {
			cleanJSON = listResp[start : end+1]
		}
	}

	if err := json.Unmarshal([]byte(cleanJSON), &assets); err != nil {
		return fmt.Errorf("failed to unmarshal discovered assets: %w | raw: %s", err, listResp)
	}

	// Demo optimization: limit breadth of scan.
	if len(assets) > 10 {
		assets = assets[:10]
	}

	for i, a := range assets {
		task := workflow.AgentTask{
			JobID:      jobID,
			Scope:      scope,
			Regulation: regulation,
			Resource: core.GCPResource{
				ID:        fmt.Sprintf("r-%d", i),
				Name:      a.Name,
				Type:      a.AssetType,
				Region:    a.Location,
				ProjectID: scope,
			},
		}
		taskData, _ := json.Marshal(task)

		// Dispatch the individual resource task into the distributed pipeline.
		if err := pubsubClient.Publish(ctx, cfg.PubSub.TopicAggregator, taskData); err != nil {
			slog.Error("worker: task dispatch failed", "resource", a.Name, "error", err)
		}
	}

	return nil
}
