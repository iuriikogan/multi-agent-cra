// Package workflow implements the core orchestration logic for specialized agents in the compliance system.
//
// Rationale: This package provides both a synchronous/concurrent local coordinator and an
// asynchronous Pub/Sub-driven workflow engine. This flexibility allows the system to scale
// from local CLI assessments to high-throughput cloud-native pipelines.
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

// Coordinator orchestrates the sequential execution of specialized agents for local processing.
// It manages a pool of worker goroutines to process resource assessments in parallel.
type Coordinator struct {
	aggregator  agent.Agent // Agent responsible for gathering resource configuration and IAM data.
	modeler     agent.Agent // Agent responsible for mapping configuration to regulatory requirements.
	validator   agent.Agent // Agent responsible for evaluating compliance rules and generating reports.
	reviewer    agent.Agent // Agent responsible for peer-reviewing the validator's findings.
	tagger      agent.Agent // Agent responsible for suggesting resource tags based on compliance.
	concurrency int         // The number of concurrent worker goroutines in the pool.
}

// NewCoordinator initializes a new workflow coordinator with a specialized agent swarm.
//
// Parameters:
//   - aggregator, modeler, validator, reviewer, tagger: Instances of specialized agents.
//   - workers: The number of concurrent assessment pipelines to run. Defaults to 1 if <= 0.
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

// ProcessStream facilitates the concurrent assessment of a stream of GCP resources.
// It returns a channel that emits results as they are completed by the agent swarm.
//
// Parameters:
//   - ctx: Context to manage the lifecycle of the entire stream processing.
//   - input: A read-only channel of GCP resources to be assessed.
func (c *Coordinator) ProcessStream(ctx context.Context, input <-chan core.GCPResource) <-chan core.AssessmentResult {
	results := make(chan core.AssessmentResult)

	// Spawn the worker group.
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

// workerLoop continuously pulls resources from the input channel and pipes them through the agent swarm.
func (c *Coordinator) workerLoop(ctx context.Context, id int, input <-chan core.GCPResource, output chan<- core.AssessmentResult) {
	slog.Debug("workflow: worker started", "worker_id", id)
	defer slog.Debug("workflow: worker stopped", "worker_id", id)

	for {
		select {
		case <-ctx.Done():
			return
		case r, ok := <-input:
			if !ok {
				return
			}
			// Analyze the resource and push the result to the output channel.
			output <- c.analyzeResource(ctx, r)
		}
	}
}

// analyzeResource executes the 5-stage agent pipeline for a single GCP resource.
// Each stage is given a timeout to prevent an individual agent from blocking the pipeline.
func (c *Coordinator) analyzeResource(ctx context.Context, r core.GCPResource) core.AssessmentResult {
	slog.Info("workflow: analyzing resource", "resource", r.Name, "type", r.Type)

	// Initialize the result with resource metadata.
	res := core.AssessmentResult{
		ResourceID:   r.ID,
		ResourceName: r.Name,
		ResourceType: r.Type,
		ProjectID:    r.ProjectID,
	}

	// Define assessment timeout (2 minutes per agent stage).
	stageTimeout := 2 * time.Minute

	// Stage 1: Resource Aggregator (Discovery & Ingestion)
	{
		stepCtx, cancel := context.WithTimeout(ctx, stageTimeout)
		defer cancel()

		slog.Debug("workflow: step 1 (Aggregator)", "resource", r.Name)
		prompt := fmt.Sprintf("Ingest configuration and IAM policies for GCP resource: %s (Type: %s, Project: %s, Region: %s)",
			r.Name, r.Type, r.ProjectID, r.Region)
		dataRepo, err := c.aggregator.Chat(stepCtx, prompt)
		if err != nil {
			slog.Error("workflow: aggregation failed", "resource", r.Name, "error", err)
			res.Error = fmt.Errorf("aggregation failed: %w", err)
			return res
		}

		// Stage 2: Compliance Modeler (Context Mapping)
		slog.Debug("workflow: step 2 (Modeler)", "resource", r.Name)
		complianceModel, err := c.modeler.Chat(stepCtx, fmt.Sprintf("Model regulatory compliance requirements for resource configuration: %s", dataRepo))
		if err != nil {
			slog.Error("workflow: modeling failed", "resource", r.Name, "error", err)
			res.Error = fmt.Errorf("modeling failed: %w", err)
			return res
		}
		res.ComplianceModel = complianceModel

		// Stage 3: Compliance Validator (Rule Evaluation)
		slog.Debug("workflow: step 3 (Validator)", "resource", r.Name)
		complianceReport, err := c.validator.Chat(stepCtx, fmt.Sprintf("Validate compliance against regulatory rules for model: %s", complianceModel))
		if err != nil {
			slog.Error("workflow: validation failed", "resource", r.Name, "error", err)
			res.Error = fmt.Errorf("validation failed: %w", err)
			return res
		}
		res.ComplianceReport = complianceReport

		// Stage 4: Assessment Reviewer (Verification)
		slog.Debug("workflow: step 4 (Reviewer)", "resource", r.Name)
		approval, err := c.reviewer.Chat(stepCtx, fmt.Sprintf("Review compliance findings for resource: %s", complianceReport))
		if err != nil {
			slog.Error("workflow: review failed", "resource", r.Name, "error", err)
			res.Error = fmt.Errorf("review failed: %w", err)
			return res
		}
		res.ApprovalStatus = approval

		// Stage 5: Resource Tagger (Governance)
		slog.Debug("workflow: step 5 (Tagger)", "resource", r.Name)
		tags, err := c.tagger.Chat(stepCtx, fmt.Sprintf("Suggest GCP tags/labels for resource based on final report: %s", complianceReport))
		if err != nil {
			slog.Error("workflow: tagging failed", "resource", r.Name, "error", err)
			res.Error = fmt.Errorf("tagging failed: %w", err)
			return res
		}
		res.Tags = tags
	}

	return res
}
