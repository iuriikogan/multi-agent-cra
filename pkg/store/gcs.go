// Package store provides a Google Cloud Storage (GCS) implementation of the Store interface.
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

// GCSStore implements the Store interface using Google Cloud Storage.
type GCSStore struct {
	client     *storage.Client // GCS client for storage operations
	bucketName string          // Target GCS bucket for data persistence
}

// NewGCS initializes a new GCSStore with the specified bucket name.
// It returns a Store implementation and an error if client initialization fails.
func NewGCS(ctx context.Context, bucketName string) (Store, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}
	return &GCSStore{
		client:     client,
		bucketName: bucketName,
	}, nil
}

// metadataPath returns the GCS object path for scan metadata.
func metadataPath(jobID string) string {
	return fmt.Sprintf("scans/%s/metadata.json", jobID)
}

// findingPath returns the GCS object path for a specific compliance finding.
func findingPath(jobID, resourceName string) string {
	return fmt.Sprintf("scans/%s/findings/%s.json", jobID, resourceName)
}

// CreateScan initializes a new scan metadata object in GCS.
func (s *GCSStore) CreateScan(ctx context.Context, jobID, scope string) error {
	scan := ScanResult{
		JobID:     jobID,
		Scope:     scope,
		Status:    "running",
		CreatedAt: time.Now(),
		Findings:  []Finding{},
	}
	return s.writeJSON(ctx, metadataPath(jobID), scan)
}

// UpdateScanStatus updates the status field in the scan's metadata object.
func (s *GCSStore) UpdateScanStatus(ctx context.Context, jobID, status string) error {
	scan, err := s.getScanMetadata(ctx, jobID)
	if err != nil {
		return err
	}
	
	scan.Status = status
	now := time.Now()
	if status == "completed" || status == "failed" {
		scan.CompletedAt = &now
	}

	return s.writeJSON(ctx, metadataPath(jobID), scan)
}

// AddFinding writes an individual finding as a standalone object in GCS.
func (s *GCSStore) AddFinding(ctx context.Context, jobID string, f Finding) error {
	return s.writeJSON(ctx, findingPath(jobID, f.ResourceName), f)
}

// GetScan retrieves scan metadata and compiles findings by listing GCS objects.
func (s *GCSStore) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	scan, err := s.getScanMetadata(ctx, jobID)
	if err != nil {
		return nil, err
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
			return nil, err
		}

		var f Finding
		if err := s.readJSON(ctx, attrs.Name, &f); err != nil {
			slog.Warn("Failed to read finding", "object", attrs.Name, "error", err)
			continue
		}
		findings = append(findings, f)
	}

	scan.Findings = findings
	return scan, nil
}

// getScanMetadata retrieves the core metadata object for a scan job.
func (s *GCSStore) getScanMetadata(ctx context.Context, jobID string) (*ScanResult, error) {
	var scan ScanResult
	if err := s.readJSON(ctx, metadataPath(jobID), &scan); err != nil {
		return nil, err
	}
	return &scan, nil
}

// writeJSON encodes an interface to JSON and writes it to a GCS object.
func (s *GCSStore) writeJSON(ctx context.Context, object string, data interface{}) error {
	w := s.client.Bucket(s.bucketName).Object(object).NewWriter(ctx)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

// readJSON reads a GCS object and decodes its JSON content into an interface.
func (s *GCSStore) readJSON(ctx context.Context, object string, dest interface{}) error {
	r, err := s.client.Bucket(s.bucketName).Object(object).NewReader(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			slog.Warn("Failed to close storage reader", "error", err)
		}
	}()
	return json.NewDecoder(r).Decode(dest)
}

// GetAllFindings returns an empty list as aggregate querying is not efficient in GCS.
func (s *GCSStore) GetAllFindings(ctx context.Context) ([]Finding, error) {
	return []Finding{}, nil
}

// Close closes the GCS storage client.
func (s *GCSStore) Close() error {
	return s.client.Close()
}
