// Package server implements the HTTP API and SSE (Server-Sent Events) streaming for the dashboard.
//
// Rationale: This package centralizes the interaction between the frontend UI and the
// backend system, providing real-time telemetry and access to historical findings.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iuriikogan/Audit-Agent/pkg/config"
	"github.com/iuriikogan/Audit-Agent/pkg/queue"
	"github.com/iuriikogan/Audit-Agent/pkg/store"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Hub manages a set of active SSE (Server-Sent Events) client channels.
// It acts as a fan-out broadcaster for real-time telemetry events.
type Hub struct {
	Clients   map[chan string]bool // Active registered client channels.
	Broadcast chan string          // Channel for inbound messages to be broadcast.
	mu        sync.Mutex           // Mutex to ensure thread-safe access to the Clients map.
}

// NewHub initializes and returns a new thread-safe SSE Hub.
func NewHub() *Hub {
	return &Hub{
		Clients:   make(map[chan string]bool),
		Broadcast: make(chan string),
	}
}

// Run starts the hub's broadcast loop. It listens for messages on the Broadcast
// channel and sends them to all registered clients.
func (h *Hub) Run(ctx context.Context) {
	slog.Info("server: sse hub started")
	for {
		select {
		case msg := <-h.Broadcast:
			h.mu.Lock()
			for client := range h.Clients {
				select {
				case client <- msg:
				default:
					// If the client's channel is full or blocked, drop the client.
					close(client)
					delete(h.Clients, client)
				}
			}
			h.mu.Unlock()
		case <-ctx.Done():
			slog.Info("server: sse hub shutting down")
			return
		}
	}
}

// NewAppHandler constructs the primary HTTP handler for the application.
// It mounts API routes, SSE streaming, and serves the static Next.js frontend.
func NewAppHandler(ctx context.Context, cfg *config.Config, pubsubClient *queue.Client, db store.Store, hub *Hub, staticFS http.FileSystem) http.Handler {
	apiMux := http.NewServeMux()

	// Health check endpoint for Cloud Run and GCLB.
	apiMux.HandleFunc("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// SSE endpoint for real-time log and status updates.
	apiMux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("X-Accel-Buffering", "no")

		// Register a new client channel with the hub.
		clientChan := make(chan string, 20)
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
			http.Error(w, "server: streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Send periodic keep-alive events to prevent connection timeouts.
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-clientChan:
				if !ok {
					return
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
					return
				}
				flusher.Flush()
			case <-ticker.C:
				if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
					return
				}
				flusher.Flush()
			case <-r.Context().Done():
				return
			case <-ctx.Done():
				return
			}
		}
	})

	// Retrieve all historical findings.
	apiMux.HandleFunc("/api/findings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		findings, err := db.GetAllFindings(r.Context())
		if err != nil {
			slog.Error("server: failed to get findings", "error", err)
			http.Error(w, "Failed to retrieve findings", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(findings)
	})

	// Dispatch or retrieve specific scan jobs.
	apiMux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleScanCreate(w, r, pubsubClient, cfg, db)
		case http.MethodGet:
			handleGetScan(w, r, db)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Serve the static Next.js frontend with SPA-aware routing.
	fileServer := http.FileServer(staticFS)
	apiMux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Skip redirection for API and Next.js internal assets.
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/_next/") {
			fileServer.ServeHTTP(w, r)
			return
		}

		f, err := staticFS.Open(path)
		if err != nil {
			// Fallback to index.html for Single Page Application routing.
			r.URL.Path = "/"
		} else {
			_ = f.Close()
		}
		fileServer.ServeHTTP(w, r)
	}))

	return otelhttp.NewHandler(corsMiddleware(apiMux), "api-server")
}

// Start launches the HTTP server and manages its graceful shutdown.
func Start(ctx context.Context, cfg *config.Config, pubsubClient *queue.Client, db store.Store, hub *Hub, staticFS http.FileSystem) error {
	handler := NewAppHandler(ctx, cfg, pubsubClient, db, hub, staticFS)

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		slog.Info("server: gracefully shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("server: listening", "port", cfg.Server.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: failed to start: %w", err)
	}
	return nil
}

// handleScanCreate processes a request to initiate a new multi-agent compliance scan.
func handleScanCreate(w http.ResponseWriter, r *http.Request, pubsubClient *queue.Client, cfg *config.Config, db store.Store) {
	var req struct {
		Scope      string `json:"scope"`
		Regulation string `json:"regulation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	jobID := uuid.New().String()
	reg := req.Regulation
	if reg == "" {
		reg = "CRA"
	}

	// Persist the scan initialization.
	if err := db.CreateScan(r.Context(), jobID, req.Scope, reg); err != nil {
		slog.Error("server: database error during scan creation", "error", err)
		http.Error(w, "Internal persistence failure", http.StatusInternalServerError)
		return
	}

	// Publish the initial task to Pub/Sub to trigger the aggregator agent.
	msg, _ := json.Marshal(map[string]string{
		"job_id":     jobID,
		"scope":      req.Scope,
		"regulation": reg,
	})
	if err := pubsubClient.Publish(r.Context(), cfg.PubSub.TopicScanRequests, msg); err != nil {
		slog.Error("server: failed to dispatch scan request to pubsub", "error", err)
		http.Error(w, "Failed to enqueue assessment job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"job_id": jobID,
		"status": "queued",
	})
}

// handleGetScan retrieves the current status and findings for a specific job ID.
func handleGetScan(w http.ResponseWriter, r *http.Request, db store.Store) {
	w.Header().Set("Cache-Control", "no-cache")
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "Missing required 'id' parameter", http.StatusBadRequest)
		return
	}

	res, err := db.GetScan(r.Context(), jobID)
	if err != nil {
		slog.Warn("server: scan lookup failed", "job_id", jobID, "error", err)
		http.Error(w, "Scan job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// corsMiddleware injects necessary headers for cross-origin resource sharing.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
