// Package workflow implements the core orchestration logic for specialized agents.
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/iuriikogan/Audit-Agent/pkg/agent"
	"github.com/iuriikogan/Audit-Agent/pkg/core"
)

// Coordinator orchestrates the sequential execution of specialized agents.
type Coordinator struct {
	aggregator  agent.Agent // Agent responsible for data gathering
	modeler     agent.Agent // Agent responsible for compliance modeling
	validator   agent.Agent // Agent responsible for rule validation
	reviewer    agent.Agent // Agent responsible for assessment review
	tagger      agent.Agent // Agent responsible for resource tagging
	concurrency int         // Number of concurrent worker goroutines
}

// NewCoordinator initializes a new workflow coordinator with specialized agents and worker count.
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

// ProcessStream concurrently assesses a stream of resources and returns results.
// It takes a context and an input channel of resources, returning an output channel of assessment results.
func (c *Coordinator) ProcessStream(ctx context.Context, input <-chan core.GCPResource) <-chan core.AssessmentResult {
	results := make(chan core.AssessmentResult)

	go func() {
		defer close(results)
		var wg sync.WaitGroup

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

// workerLoop processes resources from the input channel until closed or context is cancelled.
func (c *Coordinator) workerLoop(ctx context.Context, id int, input <-chan core.GCPResource, output chan<- core.AssessmentResult) {
	slog.Debug("Worker started", "worker_id", id)
	defer slog.Debug("Worker stopped", "worker_id", id)

	for r := range input {
		select {
		case <-ctx.Done():
			return
		default:
		}

		res := c.analyzeResource(ctx, r)
		output <- res
	}
}

// analyzeResource executes the full agent pipeline for a single GCP resource.
func (c *Coordinator) analyzeResource(ctx context.Context, r core.GCPResource) core.AssessmentResult {
	slog.Info("Analyzing resource", "resource", r.Name, "type", r.Type)

	res := core.AssessmentResult{
		ResourceID:   r.ID,
		ResourceName: r.Name,
		ResourceType: r.Type,
		ProjectID:    r.ProjectID,
	}

	// 1. Resource Aggregation
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

	// 2. CRA Modeling
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

	// 3. Compliance Validation
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

	// 4. Assessment Review
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

	// 5. Resource Tagging
	stepCtx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	slog.Debug("Step 5: Resource Tagger", "resource", r.Name)
	tags, err := c.tagger.Chat(stepCtx, fmt.Sprintf("Generate GCP labels/tags for resource based on report: %s", complianceReport))
	if err != nil {
		slog.Error("Tagging failed", "resource", r.Name, "error", err)
		res.Error = fmt.Errorf("tagging failed: %w", err)
		return res
	}
	res.Tags = tags

	return res
}
