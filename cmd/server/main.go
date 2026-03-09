package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"google.golang.org/api/option"

	"github.com/iuriikogan/multi-agent-cra/pkg/agent"
	"github.com/iuriikogan/multi-agent-cra/pkg/config"
	"github.com/iuriikogan/multi-agent-cra/pkg/core"
	"github.com/iuriikogan/multi-agent-cra/pkg/logger"
	"github.com/iuriikogan/multi-agent-cra/pkg/queue"
	"github.com/iuriikogan/multi-agent-cra/pkg/store"
	"github.com/iuriikogan/multi-agent-cra/pkg/tools"
	"github.com/iuriikogan/multi-agent-cra/pkg/workflow"
)

//go:embed all:out
var staticFiles embed.FS

// Hub manages Server-Sent Events (SSE) connections.
// It acts as a fan-out mechanism, broadcasting internal Pub/Sub monitoring events
// (like agent status and findings) to all connected browser clients for real-time dashboard updates.
type Hub struct {
	clients   map[chan string]bool
	broadcast chan string
	mu        sync.Mutex
}

// newHub creates a synchronization point for managing active SSE client subscriptions
// and ensuring timely message delivery to all connected web clients.
func newHub() *Hub {
	return &Hub{
		clients:   make(map[chan string]bool),
		broadcast: make(chan string),
	}
}

// run continually listens for new broadcast messages and pushes them to all active client channels.
func (h *Hub) run(ctx context.Context) {
	for {
		select {
		case msg := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client <- msg:
				default:
					// If the client channel is blocked, they disconnected ungracefully.
					// Close and remove them to prevent memory leaks and blocking the broadcaster.
					close(client)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	// Factor III: Config (strictly from env vars via config package).
	// Avoids hardcoded secrets or environment-specific files in source control.
	cfg := config.Load()
	logger.Setup(cfg.LogLevel)

	// Factor IX: Disposability (Graceful Shutdown).
	// Ensures in-flight database transactions or agent API calls are cleanly aborted or allowed to finish
	// when Cloud Run or Kubernetes sends a SIGTERM during autoscaling or updates.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hub := newHub()
	go hub.run(ctx)

	// Factor VI: Processes (Role-based execution).
	// The codebase contains both the HTTP API ('server') and background event processors ('worker').
	// Using the ROLE variable allows us to build a single Docker container but scale the API
	// independently from the heavy background worker processes on infrastructure like Cloud Run, optimizing resource usage.
	role := os.Getenv("ROLE")
	if role == "" {
		role = "all" // Defaults to running both logic paths for local dev ease.
	}

	slog.Info("Starting Multi-Agent CRA", "role", role, "project_id", cfg.ProjectID)

	// Initialize backing services (Factor IV)
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

	var storeClient store.Store
	switch cfg.DatabaseType {
	case "CLOUD_SQL":
		if cfg.DatabaseURL == "" {
			slog.Error("DATABASE_URL is required for CLOUD_SQL")
			os.Exit(1)
		}
		storeClient, err = store.NewCloudSQL(ctx, cfg.DatabaseURL)
		if err != nil {
			slog.Error("Failed to init CloudSQL Store", "error", err)
			os.Exit(1)
		}
	case "SQLITE_MEM":
		storeClient, err = store.NewSQLite(ctx, ":memory:")
		if err != nil {
			slog.Error("Failed to init SQLite Store", "error", err)
			os.Exit(1)
		}
	default:
		// Fallback to GCS or existing StoreType logic for backward compatibility 
		// or if DATABASE_TYPE is not explicitly set.
		if cfg.StoreType == "cloudsql" {
			storeClient, err = store.NewCloudSQL(ctx, cfg.DatabaseURL)
			if err != nil {
				slog.Error("Failed to init CloudSQL Store", "error", err)
				os.Exit(1)
			}
		} else {
			storeClient, err = store.NewGCS(ctx, cfg.GCSBucketName)
			if err != nil {
				slog.Error("Failed to init GCS Store", "error", err, "bucket", cfg.GCSBucketName)
				os.Exit(1)
			}
		}
	}
	defer func() {
		if err := storeClient.Close(); err != nil {
			slog.Error("Failed to close Store", "error", err)
		}
	}()

	// Subscribe to monitoring events if role is server or all
	if role == "server" || role == "all" {
		go func() {
			err := pubsubClient.Subscribe(ctx, cfg.PubSub.SubMonitoring, func(ctx context.Context, data []byte) error {
				hub.broadcast <- string(data)
				return nil
			})
			if err != nil && ctx.Err() == nil {
				slog.Error("Monitoring subscription error", "error", err)
			}
		}()
	}

	// Process roles
	switch role {
	case "worker":
		if err := startWorker(ctx, cfg, pubsubClient, storeClient); err != nil {
			slog.Error("Worker failed", "error", err)
			os.Exit(1)
		}
	case "server":
		if err := startServer(ctx, cfg, pubsubClient, storeClient, hub); err != nil {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	case "all":
		// For local development or monolithic deployment
		errChan := make(chan error, 1)
		go func() {
			if err := startWorker(ctx, cfg, pubsubClient, storeClient); err != nil {
				errChan <- fmt.Errorf("worker error: %w", err)
			}
		}()
		go func() {
			if err := startServer(ctx, cfg, pubsubClient, storeClient, hub); err != nil {
				errChan <- fmt.Errorf("server error: %w", err)
			}
		}()
		select {
		case <-ctx.Done():
			slog.Info("Shutting down all processes...")
		case err := <-errChan:
			slog.Error("Process failed", "error", err)
			os.Exit(1)
		}
	default:
		slog.Error("Unknown ROLE", "role", role)
		os.Exit(1)
	}
}

// --- Worker Logic ---

func startWorker(ctx context.Context, cfg *config.Config, pubsubClient *queue.Client, db store.Store) error {
	genaiClient, err := genai.NewClient(ctx, option.WithAPIKey(cfg.APIKey))
	if err != nil {
		return fmt.Errorf("failed to create GenAI client: %w", err)
	}

	// Agent definitions (to be moved to factory if needed)
	aggregatorAgent := agent.New(genaiClient, cfg.APIKey, "ResourceAggregator", "Ingestion", "gemini-3.1-flash-lite-preview",
		agent.WithSystemInstruction(`You are a Resource Aggregator. Use the list_gcp_assets tool. Return ONLY raw JSON array.`),
		agent.WithTools(tools.IngestionTools...),
	)

	modelerAgent := agent.New(genaiClient, cfg.APIKey, "CRAModeler", "Modeling", "gemini-3-pro-preview",
		agent.WithSystemInstruction(`You are a CRA Modeler. Apply CRA framework.`),
	)
	validatorAgent := agent.New(genaiClient, cfg.APIKey, "ComplianceValidator", "Validation", "gemini-3-pro-preview",
		agent.WithSystemInstruction(`Validate compliance model against CRA rules.`),
		agent.WithTools(tools.RegulatoryCheckerTools...),
		agent.WithTools(tools.ComplianceTools...),
	)
	reviewerAgent := agent.New(genaiClient, cfg.APIKey, "Reviewer", "Approval", "gemini-3-pro-preview",
		agent.WithSystemInstruction(`Review compliance report.`),
		agent.WithTools(tools.ComplianceTools...),
	)
	taggerAgent := agent.New(genaiClient, cfg.APIKey, "ResourceTagger", "Tagging", "gemini-3.1-flash-lite-preview",
		agent.WithSystemInstruction(`Tag resources.`),
		agent.WithTools(tools.TaggingTools...),
	)

	// Gracefully close all agents to release resources like Asset Inventory clients
	defer func() {
		_ = aggregatorAgent.Close()
		_ = modelerAgent.Close()
		_ = validatorAgent.Close()
		_ = reviewerAgent.Close()
		_ = taggerAgent.Close()
	}()

	wf := workflow.NewPubSubWorkflow(pubsubClient, db, cfg.PubSub.TopicMonitoring)

	// Register agent stages
	go func() {
		_ = wf.StartStage(ctx, cfg.PubSub.SubAggregator, cfg.PubSub.TopicModeler, aggregatorAgent, workflow.ProcessAggregation)
	}()
	go func() {
		_ = wf.StartStage(ctx, cfg.PubSub.SubModeler, cfg.PubSub.TopicValidator, modelerAgent, workflow.ProcessModeling)
	}()
	go func() {
		_ = wf.StartStage(ctx, cfg.PubSub.SubValidator, cfg.PubSub.TopicReviewer, validatorAgent, workflow.ProcessValidation)
	}()
	go func() {
		_ = wf.StartStage(ctx, cfg.PubSub.SubReviewer, cfg.PubSub.TopicTagger, reviewerAgent, workflow.ProcessReview)
	}()
	go func() {
		_ = wf.StartStage(ctx, cfg.PubSub.SubTagger, "", taggerAgent, workflow.ProcessTagging)
	}()

	slog.Info("Worker started: Listening for scan requests...")
	err = pubsubClient.Subscribe(ctx, cfg.PubSub.SubScanRequests, func(ctx context.Context, data []byte) error {
		var job struct {
			JobID string `json:"job_id"`
			Scope string `json:"scope"`
		}
		if err := json.Unmarshal(data, &job); err != nil {
			return fmt.Errorf("failed to parse job: %w", err)
		}

		slog.Info("Processing scan request", "job_id", job.JobID)

		if err := db.CreateScan(ctx, job.JobID, job.Scope); err != nil {
			slog.Error("Failed to create scan record", "error", err)
			return err
		}

		err := runScan(ctx, cfg, pubsubClient, aggregatorAgent, job.Scope, job.JobID, db)

		status := "completed"
		if err != nil {
			slog.Error("Scan failed", "error", err)
			status = "failed"
		}

		if err := db.UpdateScanStatus(ctx, job.JobID, status); err != nil {
			slog.Error("Failed to update status", "error", err)
		}
		return nil
	})
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("subscription error: %w", err)
	}
	slog.Info("Worker stopped")
	return nil
}

func runScan(ctx context.Context, cfg *config.Config, pubsubClient *queue.Client, aggregator agent.Agent, scope, jobID string, db store.Store) error {
	discoveryCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	listResp, err := aggregator.Chat(discoveryCtx, fmt.Sprintf("List all GCP assets in %s", scope))
	if err != nil {
		return err
	}

	jsonStr := listResp
	// Robust JSON extraction from markdown blocks
	if start := strings.IndexAny(jsonStr, "[{"); start != -1 {
		if end := strings.LastIndexAny(jsonStr, "}]"); end != -1 && end > start {
			jsonStr = jsonStr[start : end+1]
		}
	}
	jsonStr = strings.TrimSpace(jsonStr)

	type Asset struct {
		Name      string `json:"name"`
		AssetType string `json:"asset_type"`
		Location  string `json:"location"`
	}
	var assets []Asset
	if err := json.Unmarshal([]byte(jsonStr), &assets); err != nil {
		return fmt.Errorf("failed to parse assets: %w", err)
	}

	for i, a := range assets {
		task := workflow.AgentTask{
			JobID: jobID,
			Scope: scope,
			Resource: core.GCPResource{
				ID: fmt.Sprintf("r%d", i), Name: a.Name, Type: a.AssetType, Region: a.Location, ProjectID: scope,
			},
		}
		taskData, _ := json.Marshal(task)
		if err := pubsubClient.Publish(ctx, cfg.PubSub.TopicAggregator, taskData); err != nil {
			slog.Error("Failed to publish aggregator task", "error", err)
		}
	}
	return nil
}

// --- Server Logic ---

func startServer(ctx context.Context, cfg *config.Config, pubsubClient *queue.Client, db store.Store, hub *Hub) error {
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
		w.Header().Set("X-Accel-Buffering", "no") // Disable buffering in some proxies

		clientChan := make(chan string, 10) // Use a small buffer to avoid blocking the hub
		hub.mu.Lock()
		hub.clients[clientChan] = true
		hub.mu.Unlock()

		        defer func() {
		            hub.mu.Lock()
		            if _, ok := hub.clients[clientChan]; ok {
		                delete(hub.clients, clientChan)
		                close(clientChan)
		            }
		            hub.mu.Unlock()
		        }()
		
		        flusher, ok := w.(http.Flusher)
		        if !ok {
		            http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		            return
		        }
		
		        // Keep connection alive with heartbeats
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
		                // Send a comment to keep the connection alive
		                _, _ = fmt.Fprintf(w, ": keepalive\n\n")
		                flusher.Flush()
		            case <-r.Context().Done():
		                return
		            case <-ctx.Done():
		                return
		            }
		        }	})

	apiMux.HandleFunc("/api/findings", func(w http.ResponseWriter, r *http.Request) {
		// This endpoint serves historical CRA findings data, primarily for the dashboard's detailed view.
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

	contentStatic, _ := fs.Sub(staticFiles, "out")
	fileServer := http.FileServer(http.FS(contentStatic))

	mux := http.NewServeMux()
	mux.Handle("/api/", apiMux)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve the embedded Next.js frontend application for the dashboard UI.
		// This handler catches requests that are not API endpoints and attempts to serve them as static files.
		f, err := contentStatic.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	}))

	port := cfg.Server.Port
	if port == "" {
		port = "8080"
	}
	
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Graceful shutdown
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

	jobID := uuid.New().String() // Generate a unique ID for each scan request for tracking and idempotency.
	msg, _ := json.Marshal(map[string]string{"job_id": jobID, "scope": req.Scope})

	if err := pubsubClient.Publish(r.Context(), cfg.PubSub.TopicScanRequests, msg); err != nil {
		slog.Error("Failed to publish scan request", "error", err)
		http.Error(w, "Failed to queue scan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// Respond synchronously indicating the scan request has been queued.
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