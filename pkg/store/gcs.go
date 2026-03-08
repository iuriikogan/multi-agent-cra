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

type Store struct {
	client     *storage.Client
	bucketName string
}

type ScanResult struct {
	JobID       string     `json:"job_id"`
	Scope       string     `json:"scope"`
	Status      string     `json:"status"`
	Findings    []Finding  `json:"findings"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Finding struct {
	ResourceName string `json:"resource_name"`
	Status       string `json:"status"`
	Details      string `json:"details"`
}

func NewGCS(ctx context.Context, bucketName string) (*Store, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}
	return &Store{
		client:     client,
		bucketName: bucketName,
	}, nil
}

// metadataPath returns the path for the scan metadata
func metadataPath(jobID string) string {
	return fmt.Sprintf("scans/%s/metadata.json", jobID)
}

// findingPath returns the path for a specific finding
func findingPath(jobID, resourceName string) string {
	// Encode resource name to avoid path issues
	return fmt.Sprintf("scans/%s/findings/%s.json", jobID, resourceName)
}

func (s *Store) CreateScan(ctx context.Context, jobID, scope string) error {
	scan := ScanResult{
		JobID:     jobID,
		Scope:     scope,
		Status:    "running",
		CreatedAt: time.Now(),
		Findings:  []Finding{},
	}
	return s.writeJSON(ctx, metadataPath(jobID), scan)
}

func (s *Store) UpdateScanStatus(ctx context.Context, jobID, status string) error {
	// Read-Modify-Write metadata
	// Note: In a high-concurrency scenario, generation preconditions should be used.
	// For this worker, we assume single-writer ownership of the job.
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

func (s *Store) AddFinding(ctx context.Context, jobID string, f Finding) error {
	// Write finding as a separate object to avoid contention on metadata file
	// and to allow for listing.
	return s.writeJSON(ctx, findingPath(jobID, f.ResourceName), f)
}

func (s *Store) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	// 1. Get Metadata
	scan, err := s.getScanMetadata(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// 2. List and Get Findings
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

// Helper: Get just the metadata object
func (s *Store) getScanMetadata(ctx context.Context, jobID string) (*ScanResult, error) {
	var scan ScanResult
	if err := s.readJSON(ctx, metadataPath(jobID), &scan); err != nil {
		return nil, err
	}
	return &scan, nil
}

// Helper: Write interface to JSON object in GCS
func (s *Store) writeJSON(ctx context.Context, object string, data interface{}) error {
	w := s.client.Bucket(s.bucketName).Object(object).NewWriter(ctx)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// Helper: Read JSON object from GCS to interface
func (s *Store) readJSON(ctx context.Context, object string, dest interface{}) error {
	r, err := s.client.Bucket(s.bucketName).Object(object).NewReader(ctx)
	if err != nil {
		return err
	}
	defer r.Close()
	return json.NewDecoder(r).Decode(dest)
}

func (s *Store) Close() error {
	return s.client.Close()
}
