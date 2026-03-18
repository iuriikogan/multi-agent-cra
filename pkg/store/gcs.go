// Package store provides a Google Cloud Storage (GCS) implementation of the Store interface.
//
// Rationale: GCS offers durable and cost-effective object storage, suitable for
// storing massive volumes of compliance artifacts and logs for historical audits.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// GCSStore implements the Store interface using Google Cloud Storage objects.
type GCSStore struct {
	client     *storage.Client // GCS client for object operations.
	bucketName string          // The target GCS bucket for data persistence.
}

// NewGCS initializes a new GCSStore with the provided bucket name.
//
// Parameters:
//   - ctx: The context for the initialization operations.
//   - bucketName: The name of the target GCS bucket.
func NewGCS(ctx context.Context, bucketName string) (Store, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs: failed to create storage client: %w", err)
	}
	return &GCSStore{
		client:     client,
		bucketName: bucketName,
	}, nil
}

// metadataPath returns the GCS object path for scan metadata.
func (s *GCSStore) metadataPath(jobID string) string {
	return fmt.Sprintf("scans/%s/metadata.json", jobID)
}

// findingPath returns the GCS object path for a specific compliance finding.
func (s *GCSStore) findingPath(jobID, resourceName string) string {
	return fmt.Sprintf("scans/%s/findings/%s.json", jobID, resourceName)
}

// CreateScan initializes a new scan metadata object in GCS.
func (s *GCSStore) CreateScan(ctx context.Context, jobID, scope, regulation string) error {
	scan := ScanResult{
		JobID:      jobID,
		Scope:      scope,
		Status:     "running",
		Regulation: regulation,
		CreatedAt:  time.Now(),
		Findings:   []Finding{},
	}
	return s.writeJSON(ctx, s.metadataPath(jobID), scan)
}

// UpdateScanStatus updates the lifecycle state in the scan's metadata object.
func (s *GCSStore) UpdateScanStatus(ctx context.Context, jobID, status string) error {
	scan, err := s.getScanMetadata(ctx, jobID)
	if err != nil {
		return fmt.Errorf("gcs: failed to get metadata for status update: %w", err)
	}

	scan.Status = status
	if status == "completed" || status == "failed" {
		now := time.Now()
		scan.CompletedAt = &now
	}

	return s.writeJSON(ctx, s.metadataPath(jobID), scan)
}

// AddFinding records an individual compliance observation as a standalone GCS object.
func (s *GCSStore) AddFinding(ctx context.Context, jobID string, f Finding) error {
	return s.writeJSON(ctx, s.findingPath(jobID, f.ResourceName), f)
}

// GetScan compiles a full scan result by retrieving metadata and listing all finding objects.
func (s *GCSStore) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	scan, err := s.getScanMetadata(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("gcs: failed to get scan header: %w", err)
	}

	it := s.client.Bucket(s.bucketName).Objects(ctx, &storage.Query{
		Prefix: fmt.Sprintf("scans/%s/findings/", jobID),
	})

	var findings []Finding
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gcs: failed to list findings: %w", err)
		}

		var f Finding
		if err := s.readJSON(ctx, attrs.Name, &f); err != nil {
			slog.Warn("gcs: failed to decode finding", "object", attrs.Name, "error", err)
			continue
		}
		findings = append(findings, f)
	}

	scan.Findings = findings
	return scan, nil
}

// GetAllFindings is not efficiently supported in GCS and returns an empty list.
// For global reporting, a relational database or BigQuery is recommended.
func (s *GCSStore) GetAllFindings(ctx context.Context) ([]Finding, error) {
	slog.Debug("gcs: GetAllFindings is not supported efficiently on object storage")
	return []Finding{}, nil
}

// getScanMetadata retrieves the core metadata object for a specific scan job.
func (s *GCSStore) getScanMetadata(ctx context.Context, jobID string) (*ScanResult, error) {
	var scan ScanResult
	if err := s.readJSON(ctx, s.metadataPath(jobID), &scan); err != nil {
		return nil, fmt.Errorf("gcs: failed to read metadata object: %w", err)
	}
	return &scan, nil
}

// writeJSON encodes a Go structure as JSON and writes it to a GCS object.
func (s *GCSStore) writeJSON(ctx context.Context, object string, data any) error {
	w := s.client.Bucket(s.bucketName).Object(object).NewWriter(ctx)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		_ = w.Close()
		return fmt.Errorf("gcs: failed to encode JSON: %w", err)
	}
	return w.Close()
}

// readJSON reads a GCS object and decodes its JSON payload into the provided destination.
func (s *GCSStore) readJSON(ctx context.Context, object string, dest any) error {
	r, err := s.client.Bucket(s.bucketName).Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("gcs: failed to open reader: %w", err)
	}
	defer func() { _ = r.Close() }()

	if err := json.NewDecoder(r).Decode(dest); err != nil {
		return fmt.Errorf("gcs: failed to decode JSON: %w", err)
	}
	return nil
}

// Close releases the underlying storage client resources.
func (s *GCSStore) Close() error {
	return s.client.Close()
}
