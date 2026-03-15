package store

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCloudSQLStore(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	s := &CloudSQLStore{db: db}
	ctx := context.Background()
	jobID := "test-job"
	scope := "projects/test"

	// Test CreateScan
	mock.ExpectExec("INSERT IGNORE INTO scans").
		WithArgs(jobID, scope, "running", "CRA").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := s.CreateScan(ctx, jobID, scope, "CRA"); err != nil {
		t.Errorf("CreateScan failed: %v", err)
	}

	// Test UpdateScanStatus
	mock.ExpectExec("UPDATE scans SET status = \\?, completed_at = \\? WHERE job_id = \\?").
		WithArgs("completed", sqlmock.AnyArg(), jobID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := s.UpdateScanStatus(ctx, jobID, "completed"); err != nil {
		t.Errorf("UpdateScanStatus failed: %v", err)
	}

	// Test AddFinding
	finding := Finding{
		ResourceName: "res1",
		Status:       "compliant",
		Details:      "all good",
		Regulation:   "CRA",
	}
	mock.ExpectExec("INSERT INTO findings").
		WithArgs(jobID, finding.ResourceName, finding.Status, sqlmock.AnyArg(), finding.Regulation).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := s.AddFinding(ctx, jobID, finding); err != nil {
		t.Errorf("AddFinding failed: %v", err)
	}

	// Test GetScan
	mock.ExpectQuery("SELECT job_id, scope, status, regulation, created_at, completed_at FROM scans").
		WithArgs(jobID).
		WillReturnRows(sqlmock.NewRows([]string{"job_id", "scope", "status", "regulation", "created_at", "completed_at"}).
			AddRow(jobID, scope, "completed", "CRA", time.Now(), time.Now()))

	mock.ExpectQuery("SELECT resource_name, status, details, regulation FROM findings").
		WithArgs(jobID).
		WillReturnRows(sqlmock.NewRows([]string{"resource_name", "status", "details", "regulation"}).
			AddRow("res1", "compliant", []byte("\"all good\""), "CRA"))

	res, err := s.GetScan(ctx, jobID)
	if err != nil {
		t.Errorf("GetScan failed: %v", err)
	}
	if res.JobID != jobID {
		t.Errorf("expected job ID %s, got %s", jobID, res.JobID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
