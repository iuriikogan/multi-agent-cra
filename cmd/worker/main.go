// Package main serves as the entry point for the background worker service.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/iuriikogan/Audit-Agent/internal/worker"
	"github.com/iuriikogan/Audit-Agent/pkg/config"
	"github.com/iuriikogan/Audit-Agent/pkg/logger"
	"github.com/iuriikogan/Audit-Agent/pkg/observability"
	"github.com/iuriikogan/Audit-Agent/pkg/queue"
	"github.com/iuriikogan/Audit-Agent/pkg/store"
)

// main initializes dependencies and starts the worker process.
func main() {
	cfg := config.Load()

	flag.StringVar(&cfg.Models.Aggregator, "model-aggregator", cfg.Models.Aggregator, "Model for ResourceAggregator agent")
	flag.StringVar(&cfg.Models.Modeler, "model-modeler", cfg.Models.Modeler, "Model for CRAModeler agent")
	flag.StringVar(&cfg.Models.Validator, "model-validator", cfg.Models.Validator, "Model for ComplianceValidator agent")
	flag.StringVar(&cfg.Models.Reviewer, "model-reviewer", cfg.Models.Reviewer, "Model for Reviewer agent")
	flag.StringVar(&cfg.Models.Tagger, "model-tagger", cfg.Models.Tagger, "Model for ResourceTagger agent")
	flag.Parse()

	logger.Setup(cfg.LogLevel, cfg.ProjectID)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := observability.InitTrace(ctx, cfg.ProjectID); err != nil {
		slog.Error("Failed to initialize tracing", "error", err)
	}
	defer observability.Shutdown(context.Background())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "Worker is running")
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "OK")
	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

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
	case "SQLITE_MEM":
		storeClient, err = store.NewSQLite(ctx, "file:audit.db?cache=shared")
	default:
		if cfg.StoreType == "cloudsql" {
			storeClient, err = store.NewCloudSQL(ctx, cfg.DatabaseURL)
		} else if cfg.StoreType == "sqlite" {
			storeClient, err = store.NewSQLite(ctx, cfg.DatabaseURL)
		} else {
			storeClient, err = store.NewGCS(ctx, cfg.GCSBucketName)
		}
	}
	if err != nil {
		slog.Error("Failed to init Store", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := storeClient.Close(); err != nil {
			slog.Error("Failed to close store client", "error", err)
		}
	}()

	cleanupWorker, err := worker.RegisterRoutes(ctx, mux, cfg, pubsubClient, storeClient)
	if err != nil {
		slog.Error("Failed to register worker routes", "error", err)
		os.Exit(1)
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		slog.Info("Starting combined health check and worker server", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		slog.Info("Shutting down worker and health server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("Server shutdown failed", "error", err)
		}
		if cleanupWorker != nil {
			cleanupWorker()
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		slog.Error("Worker termination", "error", err)
		os.Exit(1)
	}
	slog.Info("Worker exited gracefully")
}
