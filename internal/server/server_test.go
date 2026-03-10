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
	"github.com/iuriikogan/multi-agent-cra/pkg/config"
	"github.com/iuriikogan/multi-agent-cra/pkg/queue"
	"github.com/iuriikogan/multi-agent-cra/pkg/store"
)

// mockStore uses go-sqlmock to simulate database interactions.
type mockStore struct {
	db *sql.DB
}

func (m *mockStore) CreateScan(ctx context.Context, jobID, scope string) error {
	_, err := m.db.ExecContext(ctx, "INSERT INTO scans (job_id, scope, status) VALUES ($1, $2, $3)", jobID, scope, "running")
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
	_, err := m.db.ExecContext(ctx, "INSERT INTO findings (job_id, resource_name, status, details) VALUES ($1, $2, $3, $4)", jobID, f.ResourceName, f.Status, details)
	return err
}

func (m *mockStore) GetScan(ctx context.Context, jobID string) (*store.ScanResult, error) {
	row := m.db.QueryRowContext(ctx, "SELECT job_id, scope, status, created_at, completed_at FROM scans WHERE job_id = $1", jobID)
	var res store.ScanResult
	if err := row.Scan(&res.JobID, &res.Scope, &res.Status, &res.CreatedAt, &res.CompletedAt); err != nil {
		return nil, err
	}

	rows, err := m.db.QueryContext(ctx, "SELECT resource_name, status, details FROM findings WHERE job_id = $1", jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var f store.Finding
		var detailsStr string
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsStr); err != nil {
			return nil, err
		}
		f.Details = detailsStr
		res.Findings = append(res.Findings, f)
	}
	return &res, nil
}

func (m *mockStore) GetAllFindings(ctx context.Context) ([]store.Finding, error) {
	rows, err := m.db.QueryContext(ctx, "SELECT resource_name, status, details FROM findings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []store.Finding
	for rows.Next() {
		var f store.Finding
		var detailsStr string
		if err := rows.Scan(&f.ResourceName, &f.Status, &detailsStr); err != nil {
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

func setupTestEnv(t *testing.T) (http.Handler, sqlmock.Sqlmock, *pstest.Server) {
	// Setup Pub/Sub Mock
	srv := pstest.NewServer()
	t.Cleanup(func() { srv.Close() })

	os.Setenv("PUBSUB_EMULATOR_HOST", srv.Addr)
	t.Cleanup(func() { os.Unsetenv("PUBSUB_EMULATOR_HOST") })

	ctx := context.Background()

	// Create pubsub client via newQueue proxy
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

	// Setup SQL Mock
	db, sqlMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	storeClient := &mockStore{db: db}

	cfg := &config.Config{
		ProjectID: "test-project",
		PubSub: config.PubSubConfig{
			TopicScanRequests: "scan-requests",
		},
	}

	hub := NewHub()
	handler := NewAppHandler(ctx, cfg, queueClient, storeClient, hub, nil)

	return handler, sqlMock, srv
}

func TestHealthCheckHandler(t *testing.T) {
	handler, _, _ := setupTestEnv(t)

	tests := []struct {
		name           string
		method         string
		url            string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Health Check GET",
			method:         "GET",
			url:            "/api/healthz",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %v, got %v", tt.expectedStatus, w.Code)
			}
			if tt.expectedBody != "" && w.Body.String() != tt.expectedBody {
				t.Errorf("expected body %v, got %v", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestScanEndpoints(t *testing.T) {
	handler, mock, _ := setupTestEnv(t)

	t.Run("POST /api/scan", func(t *testing.T) {
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
	})

	t.Run("GET /api/scan", func(t *testing.T) {
		mock.ExpectQuery(`SELECT job_id, scope, status, created_at, completed_at FROM scans WHERE job_id = \$1`).
			WithArgs("test-job-id").
			WillReturnRows(sqlmock.NewRows([]string{"job_id", "scope", "status", "created_at", "completed_at"}).
				AddRow("test-job-id", "projects/test-project", "running", time.Now(), nil))

		mock.ExpectQuery(`SELECT resource_name, status, details FROM findings WHERE job_id = \$1`).
			WithArgs("test-job-id").
			WillReturnRows(sqlmock.NewRows([]string{"resource_name", "status", "details"}).
				AddRow("res1", "compliant", `{"some":"detail"}`))

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
	})
}

func TestFindingsEndpoint(t *testing.T) {
	handler, mock, _ := setupTestEnv(t)

	t.Run("GET /api/findings", func(t *testing.T) {
		mock.ExpectQuery("SELECT resource_name, status, details FROM findings").
			WillReturnRows(sqlmock.NewRows([]string{"resource_name", "status", "details"}).
				AddRow("res1", "compliant", `{"detail":"1"}`).
				AddRow("res2", "non-compliant", `{"detail":"2"}`))

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
	})
}
