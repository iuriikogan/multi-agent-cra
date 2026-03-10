package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/iuriikogan/multi-agent-cra/internal/worker"
	"github.com/iuriikogan/multi-agent-cra/pkg/config"
	"github.com/iuriikogan/multi-agent-cra/pkg/logger"
	"github.com/iuriikogan/multi-agent-cra/pkg/queue"
	"github.com/iuriikogan/multi-agent-cra/pkg/store"
)

func main() {
	cfg := config.Load()
	logger.Setup(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	defer pubsubClient.Close()

	var storeClient store.Store
	switch cfg.DatabaseType {
	case "CLOUD_SQL":
		if cfg.DatabaseURL == "" {
			slog.Error("DATABASE_URL is required for CLOUD_SQL")
			os.Exit(1)
		}
		storeClient, err = store.NewCloudSQL(ctx, cfg.DatabaseURL)
	case "SQLITE_MEM":
		storeClient, err = store.NewSQLite(ctx, ":memory:")
	default:
		if cfg.StoreType == "cloudsql" {
			storeClient, err = store.NewCloudSQL(ctx, cfg.DatabaseURL)
		} else {
			storeClient, err = store.NewGCS(ctx, cfg.GCSBucketName)
		}
	}
	if err != nil {
		slog.Error("Failed to init Store", "error", err)
		os.Exit(1)
	}
	defer storeClient.Close()

	g, gCtx := errgroup.WithContext(ctx)

	// Start Health Server
	g.Go(func() error {
		slog.Info("Starting health check server", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("health check server failed: %w", err)
		}
		return nil
	})

	// Start Worker
	g.Go(func() error {
		slog.Info("Starting worker process")
		if err := worker.Start(gCtx, cfg, pubsubClient, storeClient); err != nil {
			return fmt.Errorf("worker process failed: %w", err)
		}
		return nil
	})

	// Graceful shutdown observer
	g.Go(func() error {
		<-gCtx.Done()
		slog.Info("Shutting down worker and health server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("Health server shutdown failed", "error", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		slog.Error("Worker termination", "error", err)
		os.Exit(1)
	}
	slog.Info("Worker exited gracefully")
}
