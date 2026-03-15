package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"cloud.google.com/go/pubsub/v2/pstest"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/iuriikogan/Audit-Agent/pkg/config"
	"github.com/iuriikogan/Audit-Agent/pkg/queue"
	"github.com/iuriikogan/Audit-Agent/pkg/store"
)

type mockStore struct {
	db *sql.DB
}

func (m *mockStore) CreateScan(ctx context.Context, jobID, scope, regulation string) error {
	_, err := m.db.ExecContext(ctx, "INSERT INTO scans (job_id, scope, status, regulation) VALUES ($1, $2, $3, $4)", jobID, scope, "running", regulation)
	return err
}

func (m *mockStore) UpdateScanStatus(ctx context.Context, jobID, status string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		now := time.Now()
		completedAt = &now
	}
	_, err := m.db.ExecContext(ctx, "UPDATE scans SET status = $1, completed_at = $2 WHERE job_id = $3", status, completedAt, jobID)
	return err
}

func (m *mockStore) AddFinding(ctx context.Context, jobID string, f store.Finding) error {
	details, _ := json.Marshal(f.Details)
	_, err := m.db.ExecContext(ctx, "INSERT INTO findings (job_id, resource_name, status, details, regulation) VALUES ($1, $2, $3, $4, $5)", jobID, f.ResourceName, f.Status, details, f.Regulation)
	return err
}

func (m *mockStore) GetScan(ctx context.Context, jobID string) (*store.ScanResult, error) {
	row := m.db.QueryRowContext(ctx, "SELECT job_id, scope, status, regulation, created_at, completed_at FROM scans WHERE job_id = $1", jobID)
	var res store.ScanResult
	if err := row.Scan(&res.JobID, &res.Scope, &res.Status, &res.Regulation, &res.CreatedAt, &res.CompletedAt); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, "SELECT resource_name, status, details, regulation FROM findings WHERE job_id = $1", jobID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var f store.Finding
		var detailsStr string
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsStr, &f.Regulation); err != nil {
			return nil, err
		}
		f.Details = detailsStr
		res.Findings = append(res.Findings, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *mockStore) GetAllFindings(ctx context.Context) ([]store.Finding, error) {
	rows, err := m.db.QueryContext(ctx, "SELECT resource_name, status, details, regulation FROM findings")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var findings []store.Finding
	for rows.Next() {
		var f store.Finding
		var detailsStr string
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsStr, &f.Regulation); err != nil {
			return nil, err
		}
		f.Details = detailsStr
		findings = append(findings, f)
	}
	return findings, nil
}

func (m *mockStore) Close() error {
	return m.db.Close()
}

func setupTestEnv(t *testing.T) (http.Handler, sqlmock.Sqlmock, *pstest.Server, *sql.DB) {
	srv := pstest.NewServer()
	t.Cleanup(func() {
		if err := srv.Close(); err != nil {
			t.Fatalf("Failed to close pubsub server: %v", err)
		}
	})
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", srv.Addr); err != nil {
		t.Fatalf("Failed to set PUBSUB_EMULATOR_HOST: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Unsetenv("PUBSUB_EMULATOR_HOST"); err != nil {
			t.Fatalf("Failed to unset PUBSUB_EMULATOR_HOST: %v", err)
		}
	})
	ctx := context.Background()
	queueClient, err := queue.NewClient(ctx, "test-project")
	if err != nil {
		t.Fatalf("Failed to create pubsub client: %v", err)
	}
	_, err = srv.GServer.CreateTopic(ctx, &pubsubpb.Topic{
		Name: "projects/test-project/topics/scan-requests",
	})
	if err != nil {
		t.Fatalf("Failed to create test topic: %v", err)
	}
	db, sqlMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}

	storeClient := &mockStore{db: db}
	cfg := &config.Config{
		ProjectID: "test-project",
		PubSub: config.PubSubConfig{
			TopicScanRequests: "scan-requests",
		},
	}
	hub := NewHub()
	handler := NewAppHandler(ctx, cfg, queueClient, storeClient, hub, http.Dir("."))
	return handler, sqlMock, srv, db
}

func TestHealthCheckHandler(t *testing.T) {
	handler, mock, _, db := setupTestEnv(t)
	mock.ExpectClose()

	req := httptest.NewRequest("GET", "/api/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, w.Code)
	}
	if w.Body.String() != "OK" {
		t.Errorf("expected body OK, got %v", w.Body.String())
	}
	_ = db.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestPostScanEndpoint(t *testing.T) {
	handler, mock, _, db := setupTestEnv(t)

	mock.ExpectExec("INSERT INTO scans").
		WithArgs(sqlmock.AnyArg(), "projects/test-project", "running", "CRA").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectClose()

	reqBody := `{"scope":"projects/test-project"}`
	req := httptest.NewRequest("POST", "/api/scan", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "queued" {
		t.Errorf("expected status queued, got %v", resp["status"])
	}
	if resp["job_id"] == "" {
		t.Errorf("expected non-empty job_id")
	}

	_ = db.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestGetScanEndpoint(t *testing.T) {
	handler, mock, _, db := setupTestEnv(t)

	mock.ExpectQuery("SELECT job_id, scope, status, regulation, created_at, completed_at FROM scans WHERE job_id = \\$1").
		WithArgs("test-job-id").
		WillReturnRows(sqlmock.NewRows([]string{"job_id", "scope", "status", "regulation", "created_at", "completed_at"}).
			AddRow("test-job-id", "projects/test-project", "running", "CRA", time.Now(), nil))

	mock.ExpectQuery(`SELECT resource_name, status, details, regulation FROM findings WHERE job_id = \$1`).
		WithArgs("test-job-id").
		WillReturnRows(sqlmock.NewRows([]string{"resource_name", "status", "details", "regulation"}).
			AddRow("res1", "compliant", `{"some":"detail"}`, "CRA"))

	mock.ExpectClose()

	req := httptest.NewRequest("GET", "/api/scan?id=test-job-id", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Code)
	}
	var res store.ScanResult
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.JobID != "test-job-id" {
		t.Errorf("expected test-job-id, got %v", res.JobID)
	}
	_ = db.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestFindingsEndpoint(t *testing.T) {
	handler, mock, _, db := setupTestEnv(t)

	mock.ExpectQuery("SELECT resource_name, status, details, regulation FROM findings").
		WillReturnRows(sqlmock.NewRows([]string{"resource_name", "status", "details", "regulation"}).
			AddRow("res1", "compliant", `{"detail":"1"}`, "CRA").
			AddRow("res2", "non-compliant", `{"detail":"2"}`, "CRA"))

	mock.ExpectClose()

	req := httptest.NewRequest("GET", "/api/findings", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Code)
	}
	var findings []store.Finding
	if err := json.NewDecoder(w.Body).Decode(&findings); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(findings) != 2 {
		t.Errorf("expected 2 findings, got %v", len(findings))
	}
	_ = db.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestSPARouting(t *testing.T) {
	handler, mock, _, db := setupTestEnv(t)
	mock.ExpectClose()

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{
			name:           "API path should not fallback",
			path:           "/api/nonexistent",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Next.js asset path should not fallback",
			path:           "/_next/static/chunks/main.js",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Random path should fallback to index.html",
			path:           "/random-route",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("%s: expected status %v, got %v", tt.name, tt.expectedStatus, w.Code)
			}
		})
	}

	_ = db.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}
