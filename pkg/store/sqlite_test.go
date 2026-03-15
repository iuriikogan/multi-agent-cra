package store

import (
	"context"
	"testing"
)

func TestSQLiteStore(t *testing.T) {
	ctx := context.Background()
	s, err := NewSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Errorf("failed to close sqlite store: %v", err)
		}
	}()

	jobID := "test-job"
	scope := "projects/test"

	if err := s.CreateScan(ctx, jobID, scope, "CRA"); err != nil {
		t.Fatalf("failed to create scan: %v", err)
	}

	if err := s.UpdateScanStatus(ctx, jobID, "completed"); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	finding := Finding{
		ResourceName: "res1",
		Status:       "compliant",
		Details:      "all good",
	}
	if err := s.AddFinding(ctx, jobID, finding); err != nil {
		t.Fatalf("failed to add finding: %v", err)
	}

	res, err := s.GetScan(ctx, jobID)
	if err != nil {
		t.Fatalf("failed to get scan: %v", err)
	}

	if res.JobID != jobID {
		t.Errorf("expected job ID %s, got %s", jobID, res.JobID)
	}
	if res.Status != "completed" {
		t.Errorf("expected status completed, got %s", res.Status)
	}
	if len(res.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(res.Findings))
	}
}
