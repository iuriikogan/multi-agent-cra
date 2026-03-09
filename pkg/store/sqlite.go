package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// Use pure Go SQLite driver to avoid CGO complications during CI/CD and cross-compilation.
	// This ensures greater portability and easier builds in containerized environments.
	_ "modernc.org/sqlite"
)

// SQLiteStore provides a lightweight, local relational store. It is primarily
// used for development, testing, or standalone deployments where managing CloudSQL is overhead.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLite initializes a SQLite database connection. Defaults to an in-memory db
// if no DSN is provided, ensuring zero-configuration setups work out of the box,
// and providing a clean, stateless environment for testing and development.
func NewSQLite(ctx context.Context, dsn string) (Store, error) {
	if dsn == "" {
		dsn = ":memory:" // Use ephemeral memory to guarantee clean state across restarts
	}
	// Enable foreign key support for data integrity
	if !strings.Contains(dsn, "_pragma=foreign_keys") {
		if strings.Contains(dsn, "?") {
			dsn += "&_pragma=foreign_keys(1)"
		} else {
			dsn += "?_pragma=foreign_keys(1)"
		}
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Verify the database is ready for queries.
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	// Auto-create schema. SQLite uses standard SQL but maps types internally (e.g., DATETIME, TEXT).
	// This ensures the application starts without requiring external migration scripts.
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS scans (
			job_id TEXT PRIMARY KEY,
			scope TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		);
		CREATE TABLE IF NOT EXISTS findings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT REFERENCES scans(job_id),
			resource_name TEXT NOT NULL,
			status TEXT NOT NULL,
			details TEXT NOT NULL
		);
	`); err != nil {
		return nil, fmt.Errorf("failed to initialize sqlite schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// CreateScan initializes tracking for a new workflow.
func (s *SQLiteStore) CreateScan(ctx context.Context, jobID, scope string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO scans (job_id, scope, status) VALUES (?, ?, ?)", jobID, scope, "running")
	return err
}

// UpdateScanStatus tracks job progression and timestamps completion.
func (s *SQLiteStore) UpdateScanStatus(ctx context.Context, jobID, status string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		now := time.Now()
		completedAt = &now
	}
	_, err := s.db.ExecContext(ctx, "UPDATE scans SET status = ?, completed_at = ? WHERE job_id = ?", status, completedAt, jobID)
	return err
}

// AddFinding persists individual rules matched against resources.
// JSON is stored as raw TEXT since SQLite lacks native JSONB support, 
// which might limit advanced querying compared to PostgreSQL but is simpler for local use.
func (s *SQLiteStore) AddFinding(ctx context.Context, jobID string, f Finding) error {
	details, _ := json.Marshal(f.Details)
	_, err := s.db.ExecContext(ctx, "INSERT INTO findings (job_id, resource_name, status, details) VALUES (?, ?, ?, ?)", jobID, f.ResourceName, f.Status, string(details))
	return err
}

// GetScan builds a complete view of a specific execution run.
// It performs a query to fetch the scan header, then iterates through associated findings, 
// effectively joining data from multiple tables for a comprehensive result.
func (s *SQLiteStore) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	row := s.db.QueryRowContext(ctx, "SELECT job_id, scope, status, created_at, completed_at FROM scans WHERE job_id = ?", jobID)
	var res ScanResult
	if err := row.Scan(&res.JobID, &res.Scope, &res.Status, &res.CreatedAt, &res.CompletedAt); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, "SELECT resource_name, status, details FROM findings WHERE job_id = ?", jobID)
	if err != nil {
		return nil, err
	}
	defer func() {
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

// GetAllFindings retrieves the un-partitioned list of findings required for organization-wide dashboards.
// It queries the findings table directly, optimized for displaying aggregated data to users.
func (s *SQLiteStore) GetAllFindings(ctx context.Context) ([]Finding, error) {
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

// Close gracefully releases the local file lock or memory mapping, 
// and importantly, closes the underlying database connection pool to prevent resource leaks.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
