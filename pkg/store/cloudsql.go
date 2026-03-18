// Package store provides a Cloud SQL (MySQL) implementation of the Store interface for production.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// Register the MySQL driver for Cloud SQL connections
	_ "github.com/go-sql-driver/mysql"
)

// CloudSQLStore implements the Store interface using a MySQL backend.
type CloudSQLStore struct {
	db *sql.DB // Underlying database connection pool
}

// NewCloudSQL initializes a new CloudSQLStore with the provided DSN (Data Source Name).
//
// Parameters:
//   - ctx: The context for the initialization operations.
//   - dsn: The MySQL connection string.
func NewCloudSQL(ctx context.Context, dsn string) (Store, error) {
	// The MySQL driver requires parseTime=true to correctly scan DATETIME into Go's time.Time.
	if !strings.Contains(dsn, "parseTime=true") {
		if strings.Contains(dsn, "?") {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("cloudsql: failed to open database: %w", err)
	}

	// Recommended connection pool settings for Cloud SQL to prevent exhaustion and handle idle timeouts.
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("cloudsql: failed to ping database: %w", err)
	}

	s := &CloudSQLStore{db: db}

	// Initialize tables using MySQL-specific syntax.
	if err := s.initSchema(pingCtx); err != nil {
		return nil, err
	}

	return s, nil
}

// initSchema ensures the required 'scans' and 'findings' tables are present in MySQL.
func (s *CloudSQLStore) initSchema(ctx context.Context) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS scans (
			job_id VARCHAR(255) PRIMARY KEY,
			scope TEXT NOT NULL,
			status VARCHAR(50) NOT NULL,
			regulation VARCHAR(50) NOT NULL DEFAULT 'CRA',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS findings (
			id INT AUTO_INCREMENT PRIMARY KEY,
			job_id VARCHAR(255),
			resource_name TEXT NOT NULL,
			status VARCHAR(50) NOT NULL,
			details JSON NOT NULL,
			regulation VARCHAR(50) NOT NULL DEFAULT 'CRA',
			INDEX (job_id),
			FOREIGN KEY (job_id) REFERENCES scans(job_id)
		)`,
	}

	for _, stmt := range schema {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("cloudsql: schema initialization failed: %w", err)
		}
	}
	return nil
}

// CreateScan registers a new scan job.
func (s *CloudSQLStore) CreateScan(ctx context.Context, jobID, scope, regulation string) error {
	query := "INSERT IGNORE INTO scans (job_id, scope, status, regulation) VALUES (?, ?, ?, ?)"
	_, err := s.db.ExecContext(ctx, query, jobID, scope, "running", regulation)
	if err != nil {
		return fmt.Errorf("cloudsql: failed to create scan: %w", err)
	}
	return nil
}

// UpdateScanStatus updates the lifecycle state and records completion time.
func (s *CloudSQLStore) UpdateScanStatus(ctx context.Context, jobID, status string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		now := time.Now()
		completedAt = &now
	}
	query := "UPDATE scans SET status = ?, completed_at = ? WHERE job_id = ?"
	_, err := s.db.ExecContext(ctx, query, status, completedAt, jobID)
	if err != nil {
		return fmt.Errorf("cloudsql: failed to update status: %w", err)
	}
	return nil
}

// AddFinding saves a single compliance observation linked to a scan job.
func (s *CloudSQLStore) AddFinding(ctx context.Context, jobID string, f Finding) error {
	detailsJSON, err := json.Marshal(f.Details)
	if err != nil {
		return fmt.Errorf("cloudsql: failed to marshal finding details: %w", err)
	}

	query := "INSERT INTO findings (job_id, resource_name, status, details, regulation) VALUES (?, ?, ?, ?, ?)"
	_, err = s.db.ExecContext(ctx, query, jobID, f.ResourceName, f.Status, detailsJSON, f.Regulation)
	if err != nil {
		return fmt.Errorf("cloudsql: failed to insert finding: %w", err)
	}
	return nil
}

// GetScan retrieves scan metadata and all linked findings.
func (s *CloudSQLStore) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	row := s.db.QueryRowContext(ctx, "SELECT job_id, scope, status, regulation, created_at, completed_at FROM scans WHERE job_id = ?", jobID)

	res := &ScanResult{Findings: make([]Finding, 0)}
	if err := row.Scan(&res.JobID, &res.Scope, &res.Status, &res.Regulation, &res.CreatedAt, &res.CompletedAt); err != nil {
		return nil, fmt.Errorf("cloudsql: failed to find scan %s: %w", jobID, err)
	}

	rows, err := s.db.QueryContext(ctx, "SELECT resource_name, status, details, regulation FROM findings WHERE job_id = ?", jobID)
	if err != nil {
		return nil, fmt.Errorf("cloudsql: failed to query findings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var f Finding
		var detailsRaw []byte
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsRaw, &f.Regulation); err != nil {
			return nil, err
		}
		f.Details = string(detailsRaw)
		res.Findings = append(res.Findings, f)
	}
	return res, nil
}

// GetAllFindings retrieves all findings from the database for global dashboard reporting.
func (s *CloudSQLStore) GetAllFindings(ctx context.Context) ([]Finding, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT resource_name, status, details, regulation FROM findings")
	if err != nil {
		return nil, fmt.Errorf("cloudsql: failed to query all findings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	findings := make([]Finding, 0)
	for rows.Next() {
		var f Finding
		var detailsRaw []byte
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsRaw, &f.Regulation); err != nil {
			return nil, err
		}
		f.Details = string(detailsRaw)
		findings = append(findings, f)
	}
	return findings, nil
}

// Close gracefully releases the underlying database connection pool.
func (s *CloudSQLStore) Close() error {
	return s.db.Close()
}
