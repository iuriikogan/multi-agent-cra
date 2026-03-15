// Package store provides a Cloud SQL (MySQL) implementation of the Store interface.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	// Register the MySQL driver for CloudSQL connections
	_ "github.com/go-sql-driver/mysql"
)

// CloudSQLStore implements the Store interface using a MySQL backend.
type CloudSQLStore struct {
	db *sql.DB // Underlying database connection pool
}

// NewCloudSQL initializes a new CloudSQLStore with the provided DSN.
// It returns a Store implementation and an error if initialization fails.
func NewCloudSQL(ctx context.Context, dsn string) (Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if _, err := db.ExecContext(pingCtx, `
		CREATE TABLE IF NOT EXISTS scans (
			job_id VARCHAR(255) PRIMARY KEY,
			scope TEXT NOT NULL,
			status VARCHAR(50) NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)
	`); err != nil {
		return nil, fmt.Errorf("failed to initialize scans schema: %w", err)
	}

	if _, err := db.ExecContext(pingCtx, `
		CREATE TABLE IF NOT EXISTS findings (
			id INT AUTO_INCREMENT PRIMARY KEY,
			job_id VARCHAR(255),
			resource_name TEXT NOT NULL,
			status VARCHAR(50) NOT NULL,
			details JSON NOT NULL,
			INDEX (job_id),
			FOREIGN KEY (job_id) REFERENCES scans(job_id)
		)
	`); err != nil {
		return nil, fmt.Errorf("failed to initialize findings schema: %w", err)
	}

	return &CloudSQLStore{db: db}, nil
}

// CreateScan registers a new scan job.
// It takes a context, jobID, and scope, returning an error if the operation fails.
func (s *CloudSQLStore) CreateScan(ctx context.Context, jobID, scope string) error {
	_, err := s.db.ExecContext(ctx, "INSERT IGNORE INTO scans (job_id, scope, status) VALUES (?, ?, ?)", jobID, scope, "running")
	return err
}

// UpdateScanStatus updates the status and completion time of a scan job.
// It takes a context, jobID, and status, returning an error if the operation fails.
func (s *CloudSQLStore) UpdateScanStatus(ctx context.Context, jobID, status string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		now := time.Now()
		completedAt = &now
	}
	_, err := s.db.ExecContext(ctx, "UPDATE scans SET status = ?, completed_at = ? WHERE job_id = ?", status, completedAt, jobID)
	return err
}

// AddFinding saves a single compliance finding linked to a job.
// It takes a context, jobID, and finding object, returning an error if the operation fails.
func (s *CloudSQLStore) AddFinding(ctx context.Context, jobID string, f Finding) error {
	details, _ := json.Marshal(f.Details)
	_, err := s.db.ExecContext(ctx, "INSERT INTO findings (job_id, resource_name, status, details) VALUES (?, ?, ?, ?)", jobID, f.ResourceName, f.Status, details)
	return err
}

// GetScan retrieves scan metadata and all linked findings.
// It takes a context and jobID, returning a ScanResult pointer and an error if the operation fails.
func (s *CloudSQLStore) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
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
		var detailsRaw []byte
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsRaw); err != nil {
			return nil, err
		}
		f.Details = string(detailsRaw)
		res.Findings = append(res.Findings, f)
	}
	return &res, nil
}

// GetAllFindings retrieves all findings from the database for global reporting.
// It takes a context and returns a slice of findings and an error if the query fails.
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
		var detailsRaw []byte
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsRaw); err != nil {
			return nil, err
		}
		f.Details = string(detailsRaw)
		findings = append(findings, f)
	}
	return findings, nil
}

// Close closes the underlying database connection pool.
// It returns an error if the connection closure fails.
func (s *CloudSQLStore) Close() error {
	return s.db.Close()
}
