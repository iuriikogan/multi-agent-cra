// Package store provides a SQLite implementation of the Store interface for local use.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// Register pure Go SQLite driver (CGO-free)
	_ "modernc.org/sqlite"
)

// SQLiteStore implements the Store interface using a local or in-memory SQLite database.
type SQLiteStore struct {
	db *sql.DB // Underlying database connection
}

// NewSQLite initializes a new SQLiteStore with the provided DSN.
// It returns a Store implementation and an error if initialization fails.
func NewSQLite(ctx context.Context, dsn string) (Store, error) {
	if dsn == "" {
		dsn = ":memory:"
	}
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

	// For in-memory databases, we must limit the pool to a single connection
	// to ensure all goroutines see the same database.
	if strings.Contains(dsn, ":memory:") || strings.Contains(dsn, "mode=memory") {
		db.SetMaxOpenConns(1)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS scans (
			job_id TEXT PRIMARY KEY,
			scope TEXT NOT NULL,
			status TEXT NOT NULL,
			regulation TEXT NOT NULL DEFAULT 'CRA',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		);`); err != nil {
		return nil, fmt.Errorf("failed to initialize scans table: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS findings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT REFERENCES scans(job_id),
			resource_name TEXT NOT NULL,
			status TEXT NOT NULL,
			details TEXT NOT NULL,
			regulation TEXT NOT NULL DEFAULT 'CRA'
		);
	`); err != nil {
		return nil, fmt.Errorf("failed to initialize findings table: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// CreateScan registers a new scan job if it does not already exist.
// It takes a context, jobID, and scope, returning an error if the operation fails.
func (s *SQLiteStore) CreateScan(ctx context.Context, jobID, scope, regulation string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO scans (job_id, scope, status, regulation) VALUES (?, ?, ?, ?) ON CONFLICT (job_id) DO NOTHING", jobID, scope, "running", regulation)
	return err
}

// UpdateScanStatus updates the status of a specific scan job.
// It takes a context, jobID, and status, returning an error if the operation fails.
func (s *SQLiteStore) UpdateScanStatus(ctx context.Context, jobID, status string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		now := time.Now()
		completedAt = &now
	}
	_, err := s.db.ExecContext(ctx, "UPDATE scans SET status = ?, completed_at = ? WHERE job_id = ?", status, completedAt, jobID)
	return err
}

// AddFinding saves a single compliance finding for a job to SQLite.
// It takes a context, jobID, and finding object, returning an error if the operation fails.
func (s *SQLiteStore) AddFinding(ctx context.Context, jobID string, f Finding) error {
	details, _ := json.Marshal(f.Details)
	_, err := s.db.ExecContext(ctx, "INSERT INTO findings (job_id, resource_name, status, details, regulation) VALUES (?, ?, ?, ?, ?)", jobID, f.ResourceName, f.Status, string(details), f.Regulation)
	return err
}

// GetScan retrieves scan header data and all associated findings from SQLite.
// It takes a context and jobID, returning a ScanResult pointer and an error if the operation fails.
func (s *SQLiteStore) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	row := s.db.QueryRowContext(ctx, "SELECT job_id, scope, status, regulation, created_at, completed_at FROM scans WHERE job_id = ?", jobID)
	var res ScanResult
	res.Findings = make([]Finding, 0)
	if err := row.Scan(&res.JobID, &res.Scope, &res.Status, &res.Regulation, &res.CreatedAt, &res.CompletedAt); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, "SELECT resource_name, status, details, regulation FROM findings WHERE job_id = ?", jobID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var f Finding
		var detailsStr string
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsStr, &f.Regulation); err != nil {
			return nil, err
		}
		f.Details = detailsStr
		res.Findings = append(res.Findings, f)
	}
	return &res, nil
}

// GetAllFindings retrieves all findings across all jobs for global dashboard views.
// It takes a context and returns a slice of findings and an error if the query fails.
func (s *SQLiteStore) GetAllFindings(ctx context.Context) ([]Finding, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT resource_name, status, details, regulation FROM findings")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

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

// Close closes the underlying SQLite database connection.
// It returns an error if the connection closure fails.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
