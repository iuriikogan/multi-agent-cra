// Package workflow implements the multi-agent assessment pipeline with Google Pub/Sub distribution.
//
// Rationale: This asynchronous implementation decouples specialized agents into
// independent, stateless processing stages. This allows individual components (aggregator,
// modeler, validator) to scale horizontally and persist findings via cloud storage/database.
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/iuriikogan/Audit-Agent/pkg/agent"
	"github.com/iuriikogan/Audit-Agent/pkg/core"
	"github.com/iuriikogan/Audit-Agent/pkg/queue"
	"github.com/iuriikogan/Audit-Agent/pkg/store"
)

// AgentTask represents the structured payload propagated through the agent pipeline topics.
type AgentTask struct {
	JobID      string                `json:"job_id"`     // Unique ID for the scan job.
	Scope      string                `json:"scope"`      // Resource scope currently assessed.
	Resource   core.GCPResource      `json:"resource"`   // Target resource information.
	Result     core.AssessmentResult `json:"result"`     // Accumulated findings from previous agent stages.
	Regulation string                `json:"regulation"` // The regulatory framework being assessed (CRA or DORA).
}

// PubSubWorkflow provides the messaging and persistence layer for distributed agent orchestration.
type PubSubWorkflow struct {
	client          *queue.Client // Google Cloud Pub/Sub wrapper.
	db              store.Store   // Persistent store for assessment findings.
	monitoringTopic string        // Topic for broadcasting real-time monitoring events.
}

// NewPubSubWorkflow initializes a new Pub/Sub-driven workflow with required dependencies.
//
// Parameters:
//   - client: An initialized Pub/Sub client.
//   - db: A store implementation for findings persistence.
//   - monitoringTopic: A topic ID for internal monitoring/SSE log streams.
func NewPubSubWorkflow(client *queue.Client, db store.Store, monitoringTopic string) *PubSubWorkflow {
	return &PubSubWorkflow{
		client:          client,
		db:              db,
		monitoringTopic: monitoringTopic,
	}
}

// PushMessage defines the envelope structure received from a Google Cloud Pub/Sub push subscription.
type PushMessage struct {
	Message struct {
		Data []byte `json:"data"` // Base64 decoded bytes from the Pub/Sub payload.
	}
}

// RegisterPushHandler creates and mounts an HTTP handler for an individual agent stage in the pipeline.
//
// Parameters:
//   - mux: The HTTP multiplexer to register the handler on.
//   - pattern: The URL pattern (route) for this agent stage.
//   - nextTopic: The Pub/Sub topic to publish the task to after processing (empty if final stage).
//   - a: The specialized agent instance for this processing stage.
//   - processor: The function defining the specific agent logic for this stage.
func (w *PubSubWorkflow) RegisterPushHandler(mux *http.ServeMux, pattern, nextTopic string, a agent.Agent, processor func(ctx context.Context, a agent.Agent, task *AgentTask) (string, string, error)) {
	mux.HandleFunc(pattern, func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Ensure the body is closed to prevent memory leaks during high throughput.
		defer func() { _ = r.Body.Close() }()

		var req PushMessage
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("workflow: failed to decode push request", "error", err)
			http.Error(rw, fmt.Sprintf("workflow: failed to decode push request: %v", err), http.StatusBadRequest)
			return
		}

		var task AgentTask
		if err := json.Unmarshal(req.Message.Data, &task); err != nil {
			slog.Error("workflow: failed to unmarshal agent task", "error", err)
			// Return 200 to acknowledge and stop retries on corrupted payloads.
			rw.WriteHeader(http.StatusOK)
			return
		}

		ctx := r.Context()
		slog.Info("workflow: agent stage processing", "agent", a.Name(), "job_id", task.JobID, "resource", task.Resource.Name)

		// Broadcast a monitoring event for the dashboard SSE stream.
		w.emitMonitoring(ctx, task.JobID, task.Resource.Name, a.Name(), "started", "Stage initiated")

		// Execute the agent-specific processing logic.
		reqPrompt, resText, err := processor(ctx, a, &task)

		// Create concise summaries for monitoring.
		reqShort := truncateStr(reqPrompt, 120)
		resShort := truncateStr(resText, 120)

		if err != nil {
			slog.Error("workflow: agent processing failed", "agent", a.Name(), "error", err)
			errDetails := fmt.Sprintf("FAILED | Error: %v | Prompt: %s", err, reqShort)
			w.emitMonitoring(ctx, task.JobID, task.Resource.Name, a.Name(), "failed", errDetails)
			http.Error(rw, "internal agent error", http.StatusInternalServerError)
			return
		}

		// Broadcast completion.
		details := fmt.Sprintf("OK | Size: %d bytes | Response: %s", len(resText), resShort)
		w.emitMonitoring(ctx, task.JobID, task.Resource.Name, a.Name(), "completed", details)

		// Hand-off the task to the next stage or finalize findings in the database.
		if nextTopic != "" {
			nextData, _ := json.Marshal(task)
			if err := w.client.Publish(ctx, nextTopic, nextData); err != nil {
				slog.Error("workflow: failed to hand-off task", "topic", nextTopic, "error", err)
				http.Error(rw, "pipeline hand-off failed", http.StatusInternalServerError)
				return
			}
		} else {
			// Terminal stage: persist the final assessment result.
			finding := store.Finding{
				ResourceName: task.Resource.Name,
				Status:       task.Result.ApprovalStatus,
				Details:      task.Result.ComplianceReport,
				Regulation:   task.Regulation,
			}
			if err := w.db.AddFinding(ctx, task.JobID, finding); err != nil {
				slog.Error("workflow: failed to save final finding", "error", err)
				http.Error(rw, "database error", http.StatusInternalServerError)
				return
			}
		}

		rw.WriteHeader(http.StatusOK)
	})
}

// emitMonitoring publishes an internal workflow telemetry event to the monitoring topic.
// Rationale: This event is consumed by the server and broadcast via SSE to the dashboard UI.
func (w *PubSubWorkflow) emitMonitoring(ctx context.Context, jobID, resourceName, agentName, status, details string) {
	if w.monitoringTopic == "" {
		return
	}

	event := map[string]string{
		"job_id":        jobID,
		"resource_name": resourceName,
		"agent_name":    agentName,
		"status":        status,
		"details":       details,
		"timestamp":     time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(event)

	// Use a background context that is NOT cancelled by the HTTP request lifecycle.
	publishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	if err := w.client.Publish(publishCtx, w.monitoringTopic, data); err != nil {
		slog.Error("workflow: failed to publish monitoring event", "error", err)
	}
}

// truncateStr simplifies agent conversation logs for visualization in the monitoring dashboard.
func truncateStr(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

// Specialized processing functions for each agent stage in the compliance pipeline.

func ProcessAggregation(ctx context.Context, a agent.Agent, task *AgentTask) (string, string, error) {
	prompt := fmt.Sprintf("Analyze GCP resource: %s (Type: %s) and retrieve relevant configuration and IAM metadata.", task.Resource.Name, task.Resource.Type)
	res, err := a.Chat(ctx, prompt)
	return prompt, res, err
}

func ProcessModeling(ctx context.Context, a agent.Agent, task *AgentTask) (string, string, error) {
	prompt := fmt.Sprintf("Model regulatory compliance (%s) based on configuration: %s", task.Regulation, task.Resource.Name)
	model, err := a.Chat(ctx, prompt)
	task.Result.ComplianceModel = model
	return prompt, model, err
}

func ProcessValidation(ctx context.Context, a agent.Agent, task *AgentTask) (string, string, error) {
	prompt := fmt.Sprintf("Evaluate compliance against regulatory rules for model: %s", task.Result.ComplianceModel)
	report, err := a.Chat(ctx, prompt)
	task.Result.ComplianceReport = report
	return prompt, report, err
}

func ProcessReview(ctx context.Context, a agent.Agent, task *AgentTask) (string, string, error) {
	prompt := fmt.Sprintf("Peer-review the compliance findings: %s", task.Result.ComplianceReport)
	approval, err := a.Chat(ctx, prompt)
	task.Result.ApprovalStatus = approval
	return prompt, approval, err
}

func ProcessTagging(ctx context.Context, a agent.Agent, task *AgentTask) (string, string, error) {
	prompt := fmt.Sprintf("Generate governance tags/labels for resource based on report: %s", task.Result.ComplianceReport)
	tags, err := a.Chat(ctx, prompt)
	task.Result.Tags = tags
	return prompt, tags, err
}

func ProcessReporting(ctx context.Context, a agent.Agent, task *AgentTask) (string, string, error) {
	prompt := fmt.Sprintf("Summarize compliance findings for resource %s. Findings: %s | Status: %s", task.Resource.Name, task.Result.ComplianceReport, task.Result.ApprovalStatus)
	summary, err := a.Chat(ctx, prompt)
	return prompt, summary, err
}
