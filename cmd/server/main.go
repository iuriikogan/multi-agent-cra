// Package main serves as the entry point for the compliance dashboard server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/iuriikogan/multi-agent-cra/internal/server"
	"github.com/iuriikogan/multi-agent-cra/pkg/config"
	"github.com/iuriikogan/multi-agent-cra/pkg/logger"
	"github.com/iuriikogan/multi-agent-cra/pkg/queue"
	"github.com/iuriikogan/multi-agent-cra/pkg/store"
)

// main initializes dependencies and starts the HTTP server.
func main() {
	cfg := config.Load()
	logger.Setup(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("Starting Multi-Agent CRA Server", "project_id", cfg.ProjectID)

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
	defer func() {
		if err := storeClient.Close(); err != nil {
			slog.Error("Failed to close store client", "error", err)
		}
	}()

	hub := server.NewHub()
	go hub.Run(ctx)
	
	go func() {
		err := pubsubClient.Subscribe(ctx, cfg.PubSub.SubMonitoring, func(ctx context.Context, data []byte) error {
			hub.Broadcast <- string(data)
			return nil
		})
		if err != nil && ctx.Err() == nil {
			slog.Error("Monitoring subscription error", "error", err)
		}
	}()

	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(ctx, cfg, pubsubClient, storeClient, hub); err != nil {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()
	select {
	case <-ctx.Done():
		slog.Info("Shutting down processes...")
	case err := <-errChan:
		slog.Error("Process failed", "error", err)
		os.Exit(1)
	}
}
