package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/iuriikogan/multi-agent-cra/internal/server"
	"github.com/iuriikogan/multi-agent-cra/internal/worker"
	"github.com/iuriikogan/multi-agent-cra/pkg/config"
	"github.com/iuriikogan/multi-agent-cra/pkg/logger"
	"github.com/iuriikogan/multi-agent-cra/pkg/queue"
	"github.com/iuriikogan/multi-agent-cra/pkg/store"
)

//go:embed all:out
var staticFiles embed.FS

func main() {
	cfg := config.Load()
	logger.Setup(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	role := os.Getenv("ROLE")
	if role == "" {
		role = "all"
	}

	slog.Info("Starting Multi-Agent CRA", "role", role, "project_id", cfg.ProjectID)

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

	hub := server.NewHub()
	go hub.Run(ctx)
	
	if role == "server" || role == "all" {
		go func() {
			err := pubsubClient.Subscribe(ctx, cfg.PubSub.SubMonitoring, func(ctx context.Context, data []byte) error {
				hub.Broadcast <- string(data)
				return nil
			})
			if err != nil && ctx.Err() == nil {
				slog.Error("Monitoring subscription error", "error", err)
			}
		}()
	}

	contentStatic, _ := fs.Sub(staticFiles, "out")

	switch role {
	case "worker":
		if err := worker.Start(ctx, cfg, pubsubClient, storeClient); err != nil {
			slog.Error("Worker failed", "error", err)
			os.Exit(1)
		}
	case "server":
		if err := server.Start(ctx, cfg, pubsubClient, storeClient, hub, contentStatic); err != nil {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	case "all":
		errChan := make(chan error, 1)
		go func() {
			if err := worker.Start(ctx, cfg, pubsubClient, storeClient); err != nil {
				errChan <- fmt.Errorf("worker error: %w", err)
			}
		}()
		go func() {
			if err := server.Start(ctx, cfg, pubsubClient, storeClient, hub, contentStatic); err != nil {
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