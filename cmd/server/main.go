package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"multi-agent-cra/pkg/config"
	"multi-agent-cra/pkg/logger"
	"multi-agent-cra/pkg/queue"

	"github.com/google/uuid"
)

type contextKey string

const userContextKey contextKey = "user"

func main() {
	cfg := config.Load()
	logger.Setup(cfg.LogLevel)

	ctx := context.Background()

	// Initialize Pub/Sub Client
	pubsubClient, err := queue.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		slog.Error("Failed to initialize Pub/Sub client", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := pubsubClient.Close(); err != nil {
			slog.Error("Failed to close Pub/Sub client", "error", err)
		}
	}()

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			slog.Error("Failed to write health check response", "error", err)
		}
	})

	http.HandleFunc("/api/scan", authMiddleware(auditMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// RBAC Check (Example: only admins can scan)
		// user := r.Context().Value("user").(string)
		// if !isAdmin(user) { http.Error(w, "Forbidden", http.StatusForbidden); return }

		var req struct {
			Scope string `json:"scope"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		jobID := uuid.New().String()
		msg := map[string]string{
			"job_id": jobID,
			"scope":  req.Scope,
		}
		msgBytes, _ := json.Marshal(msg)

		if err := pubsubClient.Publish(ctx, cfg.PubSub.TopicScanRequests, msgBytes); err != nil {
			slog.Error("Failed to publish scan request", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		resp := map[string]string{
			"job_id": jobID,
			"status": "queued",
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("Failed to write response", "error", err)
		}
	})))

	slog.Info("Server listening", "port", cfg.Server.Port)
	if err := http.ListenAndServe(":"+cfg.Server.Port, nil); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Simulate IAP header check
		user := r.Header.Get("X-Goog-Authenticated-User-Email")
		if user == "" {
			user = "anonymous"
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

func auditMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next(w, r)
		
		user := r.Context().Value(userContextKey)
		slog.Info("Audit Log", 
			"method", r.Method, 
			"path", r.URL.Path, 
			"user", user,
			"duration", time.Since(start),
		)
	}
}
