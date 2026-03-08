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
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"google.golang.org/api/option"

	"multi-agent-cra/pkg/agent"
	"multi-agent-cra/pkg/config"
	"multi-agent-cra/pkg/core"
	"multi-agent-cra/pkg/logger"
	"multi-agent-cra/pkg/queue"
	"multi-agent-cra/pkg/store"
	"multi-agent-cra/pkg/tools"
	"multi-agent-cra/pkg/workflow"
)

//go:embed out/*
var staticFiles embed.FS

type contextKey string

const userContextKey contextKey = "user"

func main() {
	cfg := config.Load()
	logger.Setup(cfg.LogLevel)

	ctx := context.Background()

	// --- Cloud Run Health Check Requirement ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Worker is running")
		})
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK")
		})
		
		slog.Info("Starting health check server", "port", port)
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			slog.Error("Health check server failed", "error", err)
			os.Exit(1)
		}
	}()
	// ------------------------------------------

	// 1. Initialize Pub/Sub Client
	pubsubClient, err := queue.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		slog.Error("Failed to initialize Pub/Sub client", "error", err)
		os.Exit(1)
	}
	defer pubsubClient.Close()

	// 2. Initialize GCS Store (Persistence)
	bucketName := os.Getenv("GCS_BUCKET_NAME")
	if bucketName == "" {
		// Fallback for dev/test if env var not set, though build.sh sets it
		bucketName = fmt.Sprintf("cra-data-%s", cfg.ProjectID)
	}
	
	storeClient, err := store.NewGCS(ctx, bucketName)
	if err != nil {
		slog.Error("Failed to init GCS Store", "error", err)
		os.Exit(1)
	}
	defer storeClient.Close()
	slog.Info("GCS Store initialized", "bucket", bucketName)

	// 3. Initialize AI Agents (Worker Logic)
	startWorker(ctx, cfg, pubsubClient, storeClient)

	// 4. Start API Server
	startServer(cfg, pubsubClient, storeClient)
}

// --- Worker Logic ---

func startWorker(ctx context.Context, cfg *config.Config, pubsubClient *queue.Client, db *store.Store) {
	genaiClient, err := genai.NewClient(ctx, option.WithAPIKey(cfg.APIKey))
	if err != nil {
		slog.Error("Failed to create GenAI client", "error", err)
		return
	}

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

	coordinator := workflow.NewCoordinator(aggregatorAgent, modelerAgent, validatorAgent, reviewerAgent, taggerAgent, 5)

	// Subscribe in a background goroutine
	go func() {
		slog.Info("Worker started: Listening for scan requests...")
		err := pubsubClient.Subscribe(ctx, cfg.PubSub.SubScanRequests, func(ctx context.Context, data []byte) error {
			var job struct {
				JobID string `json:"job_id"`
				Scope string `json:"scope"`
			}
			if err := json.Unmarshal(data, &job); err != nil {
				return fmt.Errorf("failed to parse job: %w", err)
			}

			slog.Info("Processing scan request", "job_id", job.JobID)
			
			// Initialize scan record in GCS
			if err := db.CreateScan(ctx, job.JobID, job.Scope); err != nil {
				slog.Error("Failed to create scan record", "error", err)
				return err // Retry
			}

			err := runScan(ctx, coordinator, aggregatorAgent, job.Scope, job.JobID, db)
			
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
		if err != nil {
			slog.Error("Subscription error", "error", err)
		}
	}()
}

func runScan(ctx context.Context, coordinator *workflow.Coordinator, aggregator agent.Agent, scope, jobID string, db *store.Store) error {
	discoveryCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	listResp, err := aggregator.Chat(discoveryCtx, fmt.Sprintf("List all GCP assets in %s", scope))
	if err != nil {
		return err
	}

	// Clean JSON markdown
	jsonStr := listResp
	if start := strings.Index(jsonStr, "```"); start != -1 {
		if end := strings.LastIndex(jsonStr, "```"); end > start {
			contentStart := start + 3
			if newline := strings.Index(jsonStr[contentStart:], "\n"); newline != -1 {
				contentStart += newline + 1
			}
			jsonStr = jsonStr[contentStart:end]
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

	var resources []core.GCPResource
	for i, a := range assets {
		resources = append(resources, core.GCPResource{
			ID: fmt.Sprintf("r%d", i), Name: a.Name, Type: a.AssetType, Region: a.Location, ProjectID: scope,
		})
	}

	inputChan := make(chan core.GCPResource, len(resources))
	for _, r := range resources {
		inputChan <- r
	}
	close(inputChan)

	resultsChan := coordinator.ProcessStream(ctx, inputChan)
	for res := range resultsChan {
		if res.Error != nil {
			slog.Error("Assessment error", "resource", res.ResourceName, "error", res.Error)
			continue
		}
		// Save finding to GCS
		err := db.AddFinding(ctx, jobID, store.Finding{
			ResourceName: res.ResourceName,
			Status:       fmt.Sprintf("%v", res.ApprovalStatus),
			Details:      "Processed by AI agents",
		})
		if err != nil {
			slog.Error("Failed to save finding", "error", err)
		}
	}
	return nil
}

// --- Server Logic ---

func startServer(cfg *config.Config, pubsubClient *queue.Client, db *store.Store) {
	apiMux := http.NewServeMux()
	
	apiMux.HandleFunc("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	apiMux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handleScanCreate(w, r, pubsubClient, cfg)
		} else if r.Method == http.MethodGet {
			handleGetScan(w, r, db)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Static Files
	contentStatic, _ := fs.Sub(staticFiles, "out")
	fileServer := http.FileServer(http.FS(contentStatic))

	mux := http.NewServeMux()
	mux.Handle("/api/", apiMux)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := contentStatic.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err == nil {
			f.Close()
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
	if port == "" { port = "8080" }
	slog.Info("Server listening", "port", port)
	http.ListenAndServe(":"+port, mux)
}

func handleScanCreate(w http.ResponseWriter, r *http.Request, pubsubClient *queue.Client, cfg *config.Config) {
	var req struct { Scope string `json:"scope"` } 
	json.NewDecoder(r.Body).Decode(&req)
	
	jobID := uuid.New().String()
	msg, _ := json.Marshal(map[string]string{"job_id": jobID, "scope": req.Scope})
	
	pubsubClient.Publish(r.Context(), cfg.PubSub.TopicScanRequests, msg)
	
	json.NewEncoder(w).Encode(map[string]string{"job_id": jobID, "status": "queued"})
}

func handleGetScan(w http.ResponseWriter, r *http.Request, db *store.Store) {
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
	json.NewEncoder(w).Encode(res)
}