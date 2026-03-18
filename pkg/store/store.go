// Package store defines the persistence layer for scan metadata and compliance findings.
//
// Rationale: Abstracting storage operations through an interface allows the system to
// seamlessly switch between local development (SQLite), production cloud databases (Cloud SQL),
// or cost-effective archival storage (GCS) without changing business logic.
package store

import (
	"context"
	"time"
)

// ScanResult holds the high-level metadata and aggregated compliance findings for a specific job.
type ScanResult struct {
	JobID       string     `json:"job_id"`                 // Unique identifier for the scan job.
	Scope       string     `json:"scope"`                  // The GCP scope (project/folder/org) assessed.
	Status      string     `json:"status"`                 // Current lifecycle state (running, completed, failed).
	Findings    []Finding  `json:"findings"`               // Collection of individual resource assessments.
	Regulation  string     `json:"regulation"`             // Regulatory framework (e.g., "CRA", "DORA").
	CreatedAt   time.Time  `json:"created_at"`             // Timestamp when the scan was initiated.
	CompletedAt *time.Time `json:"completed_at,omitempty"` // Timestamp when the scan finished (optional).
}

// Finding represents a discrete compliance observation for a specific cloud resource.
type Finding struct {
	ResourceName string `json:"resource_name"` // Name of the evaluated GCP resource.
	Status       string `json:"status"`        // Compliance status (e.g., "Compliant", "Non-Compliant").
	Details      any    `json:"details"`       // Detailed rationale or evidence (stored as JSON in DB).
	Regulation   string `json:"regulation"`    // Regulatory context for this specific finding.
}

// Store defines the contract for persistent storage operations within the compliance system.
type Store interface {
	// CreateScan initializes a new scan record in the underlying storage.
	CreateScan(ctx context.Context, jobID, scope, regulation string) error

	// UpdateScanStatus transitions the lifecycle state of a scan (e.g., from 'running' to 'completed').
	UpdateScanStatus(ctx context.Context, jobID, status string) error

	// AddFinding records a single assessment result linked to a specific job.
	AddFinding(ctx context.Context, jobID string, f Finding) error

	// GetScan retrieves the full scan result, including all associated findings, for a given job.
	GetScan(ctx context.Context, jobID string) (*ScanResult, error)

	// GetAllFindings retrieves all historical findings across all jobs for global dashboard reporting.
	GetAllFindings(ctx context.Context) ([]Finding, error)

	// Close gracefully releases any underlying storage resources (e.g., database connections).
	Close() error
}
