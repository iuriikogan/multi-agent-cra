package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iuriikogan/Audit-Agent/pkg/agent"
	"github.com/iuriikogan/Audit-Agent/pkg/core"
	"github.com/iuriikogan/Audit-Agent/pkg/store"
)

// MockStore facilitates testing of the workflow persistence logic.
type MockStore struct {
	store.Store
	AddFindingFunc func(ctx context.Context, jobID string, f store.Finding) error
}

func (m *MockStore) AddFinding(ctx context.Context, jobID string, f store.Finding) error {
	if m.AddFindingFunc != nil {
		return m.AddFindingFunc(ctx, jobID, f)
	}
	return nil
}

func TestPubSubWorkflow_RegisterPushHandler(t *testing.T) {
	mockAgent := &MockAgent{
		NameFunc: func() string { return "test-agent" },
	}
	mockStore := &MockStore{}
	
	wf := NewPubSubWorkflow(nil, mockStore, "")
	mux := http.NewServeMux()

	processor := func(ctx context.Context, a agent.Agent, task *AgentTask) (string, string, error) {
		return "prompt", "response", nil
	}

	wf.RegisterPushHandler(mux, "/test", "", mockAgent, processor)

	t.Run("ValidRequest", func(t *testing.T) {
		task := AgentTask{
			JobID: "job-1",
			Resource: core.GCPResource{Name: "res-1"},
		}
		data, _ := json.Marshal(task)
		
		pushMsg := PushMessage{}
		pushMsg.Message.Data = data
		msgBytes, _ := json.Marshal(pushMsg)

		req := httptest.NewRequest("POST", "/test", bytes.NewReader(msgBytes))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("InvalidMethod", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", rr.Code)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", strings.NewReader("invalid"))
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})
}
