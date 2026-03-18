// Package store provides testing for the Cloud SQL MySQL implementation.
package store

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestCloudSQLStore verifies the CloudSQLStore implementation using a mocked database connection.
func TestCloudSQLStore(t *testing.T) {
	// Create a new SQL mock instance.
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to initialize sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	s := &CloudSQLStore{db: db}
	ctx := context.Background()
	jobID := "test-job-uuid"
	scope := "projects/test-id"
	reg := "CRA"

	// 1. Test CreateScan expectation
	t.Run("CreateScanMock", func(t *testing.T) {
		mock.ExpectExec("INSERT IGNORE INTO scans").
			WithArgs(jobID, scope, "running", reg).
			WillReturnResult(sqlmock.NewResult(1, 1))

		if err := s.CreateScan(ctx, jobID, scope, reg); err != nil {
			t.Errorf("CreateScan failed: %v", err)
		}
	})

	// 2. Test UpdateScanStatus expectation
	t.Run("UpdateScanStatusMock", func(t *testing.T) {
		mock.ExpectExec(`UPDATE scans SET status = \?, completed_at = \? WHERE job_id = \?`).
			WithArgs("completed", sqlmock.AnyArg(), jobID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		if err := s.UpdateScanStatus(ctx, jobID, "completed"); err != nil {
			t.Errorf("UpdateScanStatus failed: %v", err)
		}
	})

	// 3. Test AddFinding expectation
	t.Run("AddFindingMock", func(t *testing.T) {
		finding := Finding{
			ResourceName: "test-compute-instance",
			Status:       "Compliant",
			Details:      "all good",
			Regulation:   reg,
		}
		// Expect insertion with a JSON-encoded details string.
		mock.ExpectExec("INSERT INTO findings").
			WithArgs(jobID, finding.ResourceName, finding.Status, sqlmock.AnyArg(), finding.Regulation).
			WillReturnResult(sqlmock.NewResult(1, 1))

		if err := s.AddFinding(ctx, jobID, finding); err != nil {
			t.Errorf("AddFinding failed: %v", err)
		}
	})

	// 4. Test GetScan expectation
	t.Run("GetScanMock", func(t *testing.T) {
		mock.ExpectQuery("SELECT job_id, scope, status, regulation, created_at, completed_at FROM scans").
			WithArgs(jobID).
			WillReturnRows(sqlmock.NewRows([]string{"job_id", "scope", "status", "regulation", "created_at", "completed_at"}).
				AddRow(jobID, scope, "completed", reg, time.Now(), time.Now()))

		mock.ExpectQuery("SELECT resource_name, status, details, regulation FROM findings").
			WithArgs(jobID).
			WillReturnRows(sqlmock.NewRows([]string{"resource_name", "status", "details", "regulation"}).
				AddRow("test-compute-instance", "Compliant", []byte("\"all good\""), reg))

		res, err := s.GetScan(ctx, jobID)
		if err != nil {
			t.Errorf("GetScan failed: %v", err)
		}
		if res.JobID != jobID {
			t.Errorf("expected job ID %s, got %s", jobID, res.JobID)
		}
	})

	// Ensure all SQL mock expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled sqlmock expectations: %s", err)
	}
}
