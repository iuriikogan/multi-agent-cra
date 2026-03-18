// Package store provides a SQLite implementation of the Store interface for local environments.
//
// Rationale: SQLite offers a zero-dependency, transactional relational store perfect for
// development, CI/CD, and lightweight edge deployments of the multi-agent system.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// Register pure Go SQLite driver (CGO-free for easier cross-compilation)
	_ "modernc.org/sqlite"
)

// SQLiteStore implements the Store interface using a local or in-memory SQLite database.
type SQLiteStore struct {
	db *sql.DB // Underlying database connection pool
}

// NewSQLite initializes a new SQLiteStore with the provided DSN (Data Source Name).
// If DSN is empty, it defaults to an in-memory database.
//
// Parameters:
//   - ctx: The context for the initialization operations.
//   - dsn: The path to the SQLite file (or ":memory:").
func NewSQLite(ctx context.Context, dsn string) (Store, error) {
	if dsn == "" || dsn == ":memory:" {
		dsn = "file:audit.db?cache=shared"
	}

	// Enforce foreign key constraints by default.
	if !strings.Contains(dsn, "_pragma=foreign_keys") {
		if strings.Contains(dsn, "?") {
			dsn += "&_pragma=foreign_keys(1)"
		} else {
			dsn += "?_pragma=foreign_keys(1)"
		}
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: failed to open database: %w", err)
	}

	// Optimization for in-memory databases to ensure all goroutines see the same data.
	if strings.Contains(dsn, ":memory:") || strings.Contains(dsn, "mode=memory") {
		db.SetMaxOpenConns(1)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("sqlite: failed to ping database: %w", err)
	}

	s := &SQLiteStore{db: db}

	// Initialize tables.
	if err := s.initSchema(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

// initSchema ensures the required 'scans' and 'findings' tables are present.
func (s *SQLiteStore) initSchema(ctx context.Context) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS scans (
			job_id TEXT PRIMARY KEY,
			scope TEXT NOT NULL,
			status TEXT NOT NULL,
			regulation TEXT NOT NULL DEFAULT 'CRA',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS findings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT REFERENCES scans(job_id),
			resource_name TEXT NOT NULL,
			status TEXT NOT NULL,
			details TEXT NOT NULL,
			regulation TEXT NOT NULL DEFAULT 'CRA'
		);`,
		`CREATE INDEX IF NOT EXISTS idx_findings_job_id ON findings(job_id);`,
	}

	for _, stmt := range schema {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("sqlite: schema initialization failed: %w", err)
		}
	}
	return nil
}

// CreateScan registers a new scan job if it does not already exist.
func (s *SQLiteStore) CreateScan(ctx context.Context, jobID, scope, regulation string) error {
	query := "INSERT INTO scans (job_id, scope, status, regulation) VALUES (?, ?, ?, ?) ON CONFLICT (job_id) DO NOTHING"
	_, err := s.db.ExecContext(ctx, query, jobID, scope, "running", regulation)
	if err != nil {
		return fmt.Errorf("sqlite: failed to create scan: %w", err)
	}
	return nil
}

// UpdateScanStatus updates the lifecycle state and records the completion time if applicable.
func (s *SQLiteStore) UpdateScanStatus(ctx context.Context, jobID, status string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		now := time.Now()
		completedAt = &now
	}
	query := "UPDATE scans SET status = ?, completed_at = ? WHERE job_id = ?"
	_, err := s.db.ExecContext(ctx, query, status, completedAt, jobID)
	if err != nil {
		return fmt.Errorf("sqlite: failed to update status: %w", err)
	}
	return nil
}

// AddFinding saves a single compliance observation for a job.
func (s *SQLiteStore) AddFinding(ctx context.Context, jobID string, f Finding) error {
	detailsJSON, err := json.Marshal(f.Details)
	if err != nil {
		return fmt.Errorf("sqlite: failed to marshal finding details: %w", err)
	}

	query := "INSERT INTO findings (job_id, resource_name, status, details, regulation) VALUES (?, ?, ?, ?, ?)"
	_, err = s.db.ExecContext(ctx, query, jobID, f.ResourceName, f.Status, string(detailsJSON), f.Regulation)
	if err != nil {
		return fmt.Errorf("sqlite: failed to insert finding: %w", err)
	}
	return nil
}

// GetScan retrieves a scan header and compiles all associated findings.
func (s *SQLiteStore) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	row := s.db.QueryRowContext(ctx, "SELECT job_id, scope, status, regulation, created_at, completed_at FROM scans WHERE job_id = ?", jobID)

	res := &ScanResult{Findings: make([]Finding, 0)}
	if err := row.Scan(&res.JobID, &res.Scope, &res.Status, &res.Regulation, &res.CreatedAt, &res.CompletedAt); err != nil {
		return nil, fmt.Errorf("sqlite: failed to find scan %s: %w", jobID, err)
	}

	rows, err := s.db.QueryContext(ctx, "SELECT resource_name, status, details, regulation FROM findings WHERE job_id = ?", jobID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: failed to query findings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var f Finding
		var detailsStr string
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsStr, &f.Regulation); err != nil {
			return nil, err
		}
		f.Details = detailsStr // We return as raw string/any for JSON decoding by consumers.
		res.Findings = append(res.Findings, f)
	}
	return res, nil
}

// GetAllFindings retrieves all findings across all jobs for global dashboard reporting.
func (s *SQLiteStore) GetAllFindings(ctx context.Context) ([]Finding, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT resource_name, status, details, regulation FROM findings")
	if err != nil {
		return nil, fmt.Errorf("sqlite: failed to query all findings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	findings := make([]Finding, 0)
	for rows.Next() {
		var f Finding
		var detailsStr string
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsStr, &f.Regulation); err != nil {
			return nil, err
		}
		f.Details = detailsStr
		findings = append(findings, f)
	}
	return findings, nil
}

// Close gracefully releases the underlying database connection pool.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
