package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"multi-agent-cra/pkg/agent"
	"multi-agent-cra/pkg/core"
)

// Coordinator acts as the Concurrency Agent, orchestrating the flow of data
// between specialized agents using Go channels and goroutines.
type Coordinator struct {
	aggregator agent.Agent
	modeler    agent.Agent
	validator  agent.Agent
	reviewer   agent.Agent
	tagger     agent.Agent
	concurrency int
}

// NewCoordinator initializes the workflow manager with specific agents.
func NewCoordinator(aggregator, modeler, validator, reviewer, tagger agent.Agent, workers int) *Coordinator {
	if workers <= 0 {
		workers = 1
	}
	return &Coordinator{
		aggregator:  aggregator,
		modeler:     modeler,
		validator:   validator,
		reviewer:    reviewer,
		tagger:      tagger,
		concurrency: workers,
	}
}

// ProcessStream takes a stream of products and returns a stream of results.
// It manages a worker pool to process items concurrently.
func (c *Coordinator) ProcessStream(ctx context.Context, input <-chan core.GCPResource) <-chan core.AssessmentResult {
	results := make(chan core.AssessmentResult)

	go func() {
		defer close(results)
		var wg sync.WaitGroup

		// Launch worker pool
		for i := 0; i < c.concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				c.workerLoop(ctx, workerID, input, results)
			}(i)
		}
		wg.Wait()
	}()
	return results
}

// workerLoop consumes products and runs the agent pipeline for each.
func (c *Coordinator) workerLoop(ctx context.Context, id int, input <-chan core.GCPResource, output chan<- core.AssessmentResult) {
	slog.Debug("Worker started", "worker_id", id)
	defer slog.Debug("Worker stopped", "worker_id", id)

	for r := range input {
		// Respect context cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		res := c.analyzeResource(ctx, r)
		output <- res
	}
}

// analyzeResource executes the sequential logic for a single item:
// Aggregator -> Modeler -> Validator -> Reviewer -> Tagger
func (c *Coordinator) analyzeResource(ctx context.Context, r core.GCPResource) core.AssessmentResult {
	slog.Info("Analyzing resource", "resource", r.Name, "type", r.Type)

	res := core.AssessmentResult{
		ResourceID:   r.ID,
		ResourceName: r.Name,
		ResourceType: r.Type,
		ProjectID:    r.ProjectID,
	}

	// 1. Resource Aggregator: Collects configuration details.
	stepCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)

	defer cancel()

	slog.Debug("Step 1: Resource Aggregator", "resource", r.Name)
	prompt := fmt.Sprintf("Ingest configuration and IAM policies for GCP resource: %s (Type: %s, Project: %s, Region: %s)", r.Name, r.Type, r.ProjectID, r.Region)
	dataRepo, err := c.aggregator.Chat(stepCtx, prompt)

	if err != nil {
		slog.Error("Aggregation failed", "resource", r.Name, "error", err)
		res.Error = fmt.Errorf("aggregation failed: %w", err)

		return res

	}

	// 2. CRA Modeler: Applies compliance framework to the data.
	stepCtx, cancel = context.WithTimeout(ctx, 2*time.Minute)

	defer cancel()

	slog.Debug("Step 2: CRA Modeler", "resource", r.Name)
	complianceModel, err := c.modeler.Chat(stepCtx, fmt.Sprintf("Model CRA compliance for GCP resource configuration: %s", dataRepo))

	if err != nil {
		slog.Error("Modeling failed", "resource", r.Name, "error", err)
		res.Error = fmt.Errorf("modeling failed: %w", err)

		return res

	}
	res.ComplianceModel = complianceModel

	// 3. Compliance Validator: Checks against specific regulations.
	stepCtx, cancel = context.WithTimeout(ctx, 2*time.Minute)

	defer cancel()

	slog.Debug("Step 3: Compliance Validator", "resource", r.Name)
	complianceReport, err := c.validator.Chat(stepCtx, fmt.Sprintf("Validate GCP resource compliance against CRA rules: %s", complianceModel))

	if err != nil {
		slog.Error("Validation failed", "resource", r.Name, "error", err)
		res.Error = fmt.Errorf("validation failed: %w", err)

		return res

	}

	res.ComplianceReport = complianceReport

	// 4. Reviewer: Provides final verdict and summary.
	stepCtx, cancel = context.WithTimeout(ctx, 2*time.Minute)

	defer cancel()

	slog.Debug("Step 4: Reviewer", "resource", r.Name)
	approval, err := c.reviewer.Chat(stepCtx, fmt.Sprintf("Review compliance report for GCP resource: %s", complianceReport))

	if err != nil {
		slog.Error("Review failed", "resource", r.Name, "error", err)
		res.Error = fmt.Errorf("review failed: %w", err)

		return res

	}

	res.ApprovalStatus = approval

	// 5. Resource Tagger: Suggests remediation tags.
	stepCtx, cancel = context.WithTimeout(ctx, 2*time.Minute)

	defer cancel()

	slog.Debug("Step 5: Resource Tagger", "resource", r.Name)
	tags, err := c.tagger.Chat(stepCtx, fmt.Sprintf("Generate GCP labels/tags for resource based on report: %s", complianceReport))

	if err != nil {
		slog.Error("Tagging failed", "resource", r.Name, "error", err)
		// Treat tagging failure as an error for strict compliance

		res.Error = fmt.Errorf("tagging failed: %w", err)

		return res

	}

	res.Tags = tags

	return res

}
