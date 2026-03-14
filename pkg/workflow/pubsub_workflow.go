// Package workflow implements the multi-agent assessment pipeline using Google Pub/Sub.
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/iuriikogan/multi-agent-cra/pkg/agent"
	"github.com/iuriikogan/multi-agent-cra/pkg/core"
	"github.com/iuriikogan/multi-agent-cra/pkg/queue"
	"github.com/iuriikogan/multi-agent-cra/pkg/store"
)

// AgentTask holds the data transferred between assessment agents in the pipeline.
type AgentTask struct {
	JobID    string                `json:"job_id"`    // Unique ID for the scan job
	Scope    string                `json:"scope"`     // Resource scope being assessed
	Resource core.GCPResource      `json:"resource"`  // Target resource details
	Result   core.AssessmentResult `json:"result"`    // Cumulative results from agents
}

// PubSubWorkflow manages agent task distribution and status monitoring via Pub/Sub.
type PubSubWorkflow struct {
	client          *queue.Client // Pub/Sub messaging client
	db              store.Store   // Persistence layer for final findings
	monitoringTopic string        // Optional topic for real-time monitoring events
}

// NewPubSubWorkflow initializes a coordinate workflow with required dependencies.
func NewPubSubWorkflow(client *queue.Client, db store.Store, monitoringTopic string) *PubSubWorkflow {
	return &PubSubWorkflow{client: client, db: db, monitoringTopic: monitoringTopic}
}

// PushMessage represents the structured payload received from a Pub/Sub push subscription.
type PushMessage struct {
	Message struct {
		Data []byte `json:"data"` // Base64 decoded bytes from Pub/Sub
	} `json:"message"`
}

// RegisterPushHandler creates and registers an HTTP handler for an agent stage in the pipeline.
// It takes a mux, route pattern, next topic for chaining, the agent instance, and a processor function.
func (w *PubSubWorkflow) RegisterPushHandler(mux *http.ServeMux, pattern string, nextTopic string, a agent.Agent, processor func(ctx context.Context, a agent.Agent, task *AgentTask) error) {
	mux.HandleFunc(pattern, func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req PushMessage
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("Failed to decode push request", "error", err)
			http.Error(rw, "bad request", http.StatusBadRequest)
			return
		}

		var task AgentTask
		if err := json.Unmarshal(req.Message.Data, &task); err != nil {
			slog.Error("Failed to unmarshal task from push data", "error", err)
			rw.WriteHeader(http.StatusOK)
			return
		}

		ctx := r.Context()
		slog.Info("Agent processing push stage", "agent", a.Name(), "job_id", task.JobID, "resource", task.Resource.Name)

		w.emitMonitoring(ctx, task.JobID, task.Resource.Name, a.Name(), "started", "")

		if err := processor(ctx, a, &task); err != nil {
			slog.Error("Agent processing failed", "agent", a.Name(), "error", err)
			w.emitMonitoring(ctx, task.JobID, task.Resource.Name, a.Name(), "failed", err.Error())
			http.Error(rw, "processing failed", http.StatusInternalServerError)
			return
		}

		w.emitMonitoring(ctx, task.JobID, task.Resource.Name, a.Name(), "completed", "")

		if nextTopic != "" {
			nextData, _ := json.Marshal(task)
			if err := w.client.Publish(ctx, nextTopic, nextData); err != nil {
				slog.Error("Failed to publish to next topic", "error", err)
				http.Error(rw, "publish failed", http.StatusInternalServerError)
				return
			}
		} else {
			finding := store.Finding{
				ResourceName: task.Resource.Name,
				Status:       fmt.Sprintf("%v", task.Result.ApprovalStatus),
				Details:      task.Result.ComplianceReport,
			}
			if err := w.db.AddFinding(ctx, task.JobID, finding); err != nil {
				slog.Error("Failed to save final finding", "error", err)
				http.Error(rw, "db error", http.StatusInternalServerError)
				return
			}
		}

		rw.WriteHeader(http.StatusOK)
	})
}

// emitMonitoring publishes a workflow event to the monitoring topic.
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
	
	publishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	if err := w.client.Publish(publishCtx, w.monitoringTopic, data); err != nil {
		slog.Error("Failed to publish monitoring event", "error", err)
	}
}

// ProcessAggregation simulates resource discovery and data gathering.
func ProcessAggregation(ctx context.Context, a agent.Agent, task *AgentTask) error {
	prompt := fmt.Sprintf("Ingest configuration and IAM policies for GCP resource: %s (Type: %s, Project: %s)", task.Resource.Name, task.Resource.Type, task.Resource.ProjectID)
	_, err := a.Chat(ctx, prompt)
	return err
}

// ProcessModeling generates a compliance model for the resource via the LLM agent.
func ProcessModeling(ctx context.Context, a agent.Agent, task *AgentTask) error {
	model, err := a.Chat(ctx, fmt.Sprintf("Model CRA compliance for GCP resource: %s", task.Resource.Name))
	task.Result.ComplianceModel = model
	return err
}

// ProcessValidation generates a detailed compliance report based on the model.
func ProcessValidation(ctx context.Context, a agent.Agent, task *AgentTask) error {
	report, err := a.Chat(ctx, fmt.Sprintf("Validate CRA compliance for model: %s", task.Result.ComplianceModel))
	task.Result.ComplianceReport = report
	return err
}

// ProcessReview evaluates the compliance report to determine approval status.
func ProcessReview(ctx context.Context, a agent.Agent, task *AgentTask) error {
	approval, err := a.Chat(ctx, fmt.Sprintf("Review compliance report: %s", task.Result.ComplianceReport))
	task.Result.ApprovalStatus = approval
	return err
}

// ProcessTagging suggests resource tags based on the compliance findings.
func ProcessTagging(ctx context.Context, a agent.Agent, task *AgentTask) error {
	tags, err := a.Chat(ctx, fmt.Sprintf("Suggest tags for resource based on report: %s", task.Result.ComplianceReport))
	task.Result.Tags = tags
	return err
}

// ProcessReporting converts agent output into a structured finding object.
func ProcessReporting(ctx context.Context, a agent.Agent, task *AgentTask) error {
	report, err := a.Chat(ctx, fmt.Sprintf("Generate a CRA compliance report for resource: %s, with compliance status: %s and details: %s", task.Resource.Name, task.Result.ApprovalStatus, task.Result.ComplianceReport))
	if err != nil {
		return err
	}
	
	cleanReport := sanitizeJSON(report)

	var finding store.Finding
	if err := json.Unmarshal([]byte(cleanReport), &finding); err != nil {
		return fmt.Errorf("failed to unmarshal finding from report: %w (raw report: %s)", err, report)
	}
	task.Result.ApprovalStatus = finding.Status
	task.Result.ComplianceReport = finding.Details
	return nil
}

// sanitizeJSON cleans markdown formatting from LLM responses to ensure valid JSON.
func sanitizeJSON(input string) string {
	res := strings.TrimSpace(input)
	if strings.HasPrefix(res, "```json") {
		res = strings.TrimPrefix(res, "```json")
		res = strings.TrimSuffix(res, "```")
	} else if strings.HasPrefix(res, "```") {
		res = strings.TrimPrefix(res, "```")
		res = strings.TrimSuffix(res, "```")
	}
	return strings.TrimSpace(res)
}
