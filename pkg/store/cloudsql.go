package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	// Register the PostgreSQL driver for CloudSQL connections
	_ "github.com/lib/pq"
)

// CloudSQLStore provides a persistent, structured backend for scan results,
// enabling complex queries and reporting that are difficult with flat files (GCS).
type CloudSQLStore struct {
	db *sql.DB
}

// NewCloudSQL initializes a connection pool to a PostgreSQL instance.
func NewCloudSQL(ctx context.Context, dsn string) (Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Ensure the database is actually reachable before returning the store instance.
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Bootstrap schema if missing. In a mature environment, this should be handled
	// by an external migration tool (e.g., golang-migrate or Flyway) to avoid race conditions.
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS scans (
			job_id TEXT PRIMARY KEY,
			scope TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP WITH TIME ZONE
		);
		CREATE TABLE IF NOT EXISTS findings (
			id SERIAL PRIMARY KEY,
			job_id TEXT REFERENCES scans(job_id),
			resource_name TEXT NOT NULL,
			status TEXT NOT NULL,
			details JSONB NOT NULL
		);
	`); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &CloudSQLStore{db: db}, nil
}

// CreateScan registers a new scan job so downstream workers can link their findings to it.
func (s *CloudSQLStore) CreateScan(ctx context.Context, jobID, scope string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO scans (job_id, scope, status) VALUES ($1, $2, $3) ON CONFLICT (job_id) DO NOTHING", jobID, scope, "running")
	return err
}

// UpdateScanStatus tracks the lifecycle of a scan. It records completion time to measure SLA adherence.
func (s *CloudSQLStore) UpdateScanStatus(ctx context.Context, jobID, status string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		now := time.Now()
		completedAt = &now
	}
	_, err := s.db.ExecContext(ctx, "UPDATE scans SET status = $1, completed_at = $2 WHERE job_id = $3", status, completedAt, jobID)
	return err
}

// AddFinding stores individual compliance rule violations or passes.
// Details are stored as JSONB to allow schema-less querying of varying CRA rules later.
func (s *CloudSQLStore) AddFinding(ctx context.Context, jobID string, f Finding) error {
	details, _ := json.Marshal(f.Details)
	_, err := s.db.ExecContext(ctx, "INSERT INTO findings (job_id, resource_name, status, details) VALUES ($1, $2, $3, $4)", jobID, f.ResourceName, f.Status, details)
	return err
}

// GetScan aggregates a scan's metadata and all its findings for the frontend detailed view.
func (s *CloudSQLStore) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	row := s.db.QueryRowContext(ctx, "SELECT job_id, scope, status, created_at, completed_at FROM scans WHERE job_id = $1", jobID)
	var res ScanResult
	if err := row.Scan(&res.JobID, &res.Scope, &res.Status, &res.CreatedAt, &res.CompletedAt); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, "SELECT resource_name, status, details FROM findings WHERE job_id = $1", jobID)
	if err != nil {
		return nil, err
	}
	defer func() {
		// Ensure resources are freed even if the loop panics or returns early.
		_ = rows.Close()
	}()

	for rows.Next() {
		var f Finding
		var detailsStr string
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsStr); err != nil {
			return nil, err
		}
		f.Details = detailsStr
		res.Findings = append(res.Findings, f)
	}
	return &res, nil
}

// GetAllFindings fetches raw findings independently of their job, feeding the global CRA compliance dashboard.
// It leverages Cloud SQL's efficient querying of the findings table, making it suitable for aggregate reporting.
func (s *CloudSQLStore) GetAllFindings(ctx context.Context) ([]Finding, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT resource_name, status, details FROM findings")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var findings []Finding
	for rows.Next() {
		var f Finding
		var detailsStr string
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsStr); err != nil {
			return nil, err
		}
		f.Details = detailsStr
		findings = append(findings, f)
	}
	return findings, nil
}

// Close gracefully releases the database connection pool.
func (s *CloudSQLStore) Close() error {
	return s.db.Close()
}
