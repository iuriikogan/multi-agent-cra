// Package store defines the persistence layer for scan metadata and findings.
package store

import (
	"context"
	"time"
)

// ScanResult holds metadata and compliance findings for a specific scan.
type ScanResult struct {
	JobID       string     `json:"job_id"`
	Scope       string     `json:"scope"`
	Status      string     `json:"status"`
	Findings    []Finding  `json:"findings"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Finding holds security violation or compliance details for a specific resource.
type Finding struct {
	ResourceName string `json:"resource_name"`
	Status       string `json:"status"`
	Details      string `json:"details"`
}

// Store defines persistent storage operations for compliance scan data.
type Store interface {
	// CreateScan initializes a new scan record.
	CreateScan(ctx context.Context, jobID, scope string) error
	// UpdateScanStatus updates the lifecycle state of a scan.
	UpdateScanStatus(ctx context.Context, jobID, status string) error
	// AddFinding records a single assessment result.
	AddFinding(ctx context.Context, jobID string, f Finding) error
	// GetScan retrieves a scan and its associated findings.
	GetScan(ctx context.Context, jobID string) (*ScanResult, error)
	// GetAllFindings retrieves all historical findings for aggregate reporting.
	GetAllFindings(ctx context.Context) ([]Finding, error)
	// Close releases underlying storage resources.
	Close() error
}
