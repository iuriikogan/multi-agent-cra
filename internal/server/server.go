package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iuriikogan/multi-agent-cra/pkg/config"
	"github.com/iuriikogan/multi-agent-cra/pkg/queue"
	"github.com/iuriikogan/multi-agent-cra/pkg/store"
)

// Hub manages Server-Sent Events (SSE) connections.
type Hub struct {
	Clients   map[chan string]bool
	Broadcast chan string
	mu        sync.Mutex
}

// NewHub creates a synchronization point for managing active SSE client subscriptions
func NewHub() *Hub {
	return &Hub{
		Clients:   make(map[chan string]bool),
		Broadcast: make(chan string),
	}
}

// Run continually listens for new broadcast messages and pushes them to all active client channels.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case msg := <-h.Broadcast:
			h.mu.Lock()
			for client := range h.Clients {
				select {
				case client <- msg:
				default:
					close(client)
					delete(h.Clients, client)
				}
			}
			h.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

// NewAppHandler constructs the main server handler
func NewAppHandler(ctx context.Context, cfg *config.Config, pubsubClient *queue.Client, db store.Store, hub *Hub) http.Handler {
	apiMux := http.NewServeMux()

	apiMux.HandleFunc("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	apiMux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("X-Accel-Buffering", "no")

		clientChan := make(chan string, 10)
		hub.mu.Lock()
		hub.Clients[clientChan] = true
		hub.mu.Unlock()

		defer func() {
			hub.mu.Lock()
			if _, ok := hub.Clients[clientChan]; ok {
				delete(hub.Clients, clientChan)
				close(clientChan)
			}
			hub.mu.Unlock()
		}()

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-clientChan:
				if !ok {
					return
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			case <-ticker.C:
				_, _ = fmt.Fprintf(w, ": keepalive\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			case <-ctx.Done():
				return
			}
		}
	})

	apiMux.HandleFunc("/api/findings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		findings, err := db.GetAllFindings(r.Context())
		if err != nil {
			slog.Error("GetAllFindings error", "error", err)
			http.Error(w, "Failed to retrieve findings", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(findings); err != nil {
			slog.Error("Failed to encode findings response", "error", err)
		}
	})

	apiMux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleScanCreate(w, r, pubsubClient, cfg)
		case http.MethodGet:
			handleGetScan(w, r, db)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return apiMux
}

// Start initiates the HTTP server process
func Start(ctx context.Context, cfg *config.Config, pubsubClient *queue.Client, db store.Store, hub *Hub) error {
	mux := NewAppHandler(ctx, cfg, pubsubClient, db, hub)

	port := cfg.Server.Port
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		slog.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("Server shutdown failed", "error", err)
		}
	}()

	slog.Info("Server listening", "port", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}
	slog.Info("Server stopped")
	return nil
}

func handleScanCreate(w http.ResponseWriter, r *http.Request, pubsubClient *queue.Client, cfg *config.Config) {
	var req struct {
		Scope string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	jobID := uuid.New().String()
	msg, _ := json.Marshal(map[string]string{"job_id": jobID, "scope": req.Scope})

	if err := pubsubClient.Publish(r.Context(), cfg.PubSub.TopicScanRequests, msg); err != nil {
		slog.Error("Failed to publish scan request", "error", err)
		http.Error(w, "Failed to queue scan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"job_id": jobID, "status": "queued"}); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

func handleGetScan(w http.ResponseWriter, r *http.Request, db store.Store) {
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "Missing job_id", http.StatusBadRequest)
		return
	}
	res, err := db.GetScan(r.Context(), jobID)
	if err != nil {
		slog.Error("GetScan error", "error", err)
		http.Error(w, "Scan not found or error", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}
