package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/iuriikogan/multi-agent-cra/pkg/agent"
	"github.com/iuriikogan/multi-agent-cra/pkg/core"
	"github.com/iuriikogan/multi-agent-cra/pkg/queue"
	"github.com/iuriikogan/multi-agent-cra/pkg/store"
)

// AgentTask is the payload passed between agent stages in the Pub/Sub pipeline.
type AgentTask struct {
	JobID    string            `json:"job_id"`
	Scope    string            `json:"scope"`
	Resource core.GCPResource  `json:"resource"`
	Result   core.AssessmentResult `json:"result"`
}

type PubSubWorkflow struct {
	client          *queue.Client
	db              store.Store
	monitoringTopic string
}

func NewPubSubWorkflow(client *queue.Client, db store.Store, monitoringTopic string) *PubSubWorkflow {
	return &PubSubWorkflow{client: client, db: db, monitoringTopic: monitoringTopic}
}

// PushMessage represents the Pub/Sub push HTTP payload
type PushMessage struct {
	Message struct {
		Data []byte `json:"data"`
	} `json:"message"`
}

// RegisterPushHandler creates an HTTP handler for a specific agent stage.
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
			// Return 200 to ack unparseable messages so they don't loop endlessly
			rw.WriteHeader(http.StatusOK)
			return
		}

		ctx := r.Context()
		slog.Info("Agent processing push stage", "agent", a.Name(), "job_id", task.JobID, "resource", task.Resource.Name)

		w.emitMonitoring(ctx, task.JobID, task.Resource.Name, a.Name(), "started", "")

		if err := processor(ctx, a, &task); err != nil {
			slog.Error("Agent processing failed", "agent", a.Name(), "error", err)
			w.emitMonitoring(ctx, task.JobID, task.Resource.Name, a.Name(), "failed", err.Error())
			// Return 500 so Pub/Sub retries this task
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
			// Final stage: Save finding to DB
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
	if err := w.client.Publish(ctx, w.monitoringTopic, data); err != nil {
		slog.Error("Failed to publish monitoring event", "error", err)
	}
}

// Helper Processors for each stage

func ProcessAggregation(ctx context.Context, a agent.Agent, task *AgentTask) error {
	prompt := fmt.Sprintf("Ingest configuration and IAM policies for GCP resource: %s (Type: %s, Project: %s)", task.Resource.Name, task.Resource.Type, task.Resource.ProjectID)
	_, err := a.Chat(ctx, prompt)
	// For now, we just simulate the chat. In a real scenario, the agent would return the data.
	return err
}

func ProcessModeling(ctx context.Context, a agent.Agent, task *AgentTask) error {
	model, err := a.Chat(ctx, fmt.Sprintf("Model CRA compliance for GCP resource: %s", task.Resource.Name))
	task.Result.ComplianceModel = model
	return err
}

func ProcessValidation(ctx context.Context, a agent.Agent, task *AgentTask) error {
	report, err := a.Chat(ctx, fmt.Sprintf("Validate CRA compliance for model: %s", task.Result.ComplianceModel))
	task.Result.ComplianceReport = report
	return err
}

func ProcessReview(ctx context.Context, a agent.Agent, task *AgentTask) error {
	approval, err := a.Chat(ctx, fmt.Sprintf("Review compliance report: %s", task.Result.ComplianceReport))
	task.Result.ApprovalStatus = approval
	return err
}

func ProcessTagging(ctx context.Context, a agent.Agent, task *AgentTask) error {
	tags, err := a.Chat(ctx, fmt.Sprintf("Suggest tags for resource based on report: %s", task.Result.ComplianceReport))
	task.Result.Tags = tags
	return err
}

func ProcessReporting(ctx context.Context, a agent.Agent, task *AgentTask) error {
	report, err := a.Chat(ctx, fmt.Sprintf("Generate a CRA compliance report for resource: %s, with compliance status: %s and details: %s", task.Resource.Name, task.Result.ApprovalStatus, task.Result.ComplianceReport))
	if err != nil {
		return err
	}
	var finding store.Finding
	if err := json.Unmarshal([]byte(report), &finding); err != nil {
		return fmt.Errorf("failed to unmarshal finding from report: %w", err)
	}
	task.Result.ApprovalStatus = finding.Status
	task.Result.ComplianceReport = finding.Details
	return nil
}
