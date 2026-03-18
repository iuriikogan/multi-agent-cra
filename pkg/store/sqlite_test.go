package store

import (
	"context"
	"testing"
)

func TestSQLiteStore_GetScan_BeforeUpdate(t *testing.T) {
	ctx := context.Background()

	s, err := NewSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	defer func() { _ = s.Close() }()

	jobID := "test-job-uuid-2"
	scope := "projects/test-project"
	reg := "DORA"

	if err := s.CreateScan(ctx, jobID, scope, reg); err != nil {
		t.Fatalf("failed to create scan: %v", err)
	}

	_, err = s.GetScan(ctx, jobID)
	if err != nil {
		t.Fatalf("failed to get scan: %v", err)
	}
}
