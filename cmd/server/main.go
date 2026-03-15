package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"embed"
	"io/fs"

	"github.com/iuriikogan/multi-agent-cra/internal/server"
	"github.com/iuriikogan/multi-agent-cra/internal/worker"
	"github.com/iuriikogan/multi-agent-cra/pkg/config"
	"github.com/iuriikogan/multi-agent-cra/pkg/logger"
	"github.com/iuriikogan/multi-agent-cra/pkg/observability"
	"github.com/iuriikogan/multi-agent-cra/pkg/queue"
	"github.com/iuriikogan/multi-agent-cra/pkg/store"
)

//go:embed out
var staticAssets embed.FS

func main() {
	cfg := config.Load()
	logger.Setup(cfg.LogLevel, cfg.ProjectID)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := observability.InitTrace(ctx, cfg.ProjectID); err != nil {
		slog.Error("Failed to initialize tracing", "error", err)
	}
	defer observability.Shutdown(context.Background())

	role := os.Getenv("ROLE")
	if role == "" {
		role = "all"
	}
	slog.Info("Starting Multi-Agent CRA", "project_id", cfg.ProjectID, "role", role)

	pubsubClient, err := queue.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		slog.Error("Failed to initialize Pub/Sub client", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = pubsubClient.Close()
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
		if storeClient != nil {
			_ = storeClient.Close()
		}
	}()

	errChan := make(chan error, 2)

	if role == "server" || role == "all" {
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

		go func() {
			if role == "all" {
				var muxWrapper http.Handler
				subFS, err := fs.Sub(staticAssets, "out")
				if err != nil {
					// Fallback to local directory for development if embed fails or is empty
					slog.Warn("Failed to use embedded assets, falling back to disk", "error", err)
					muxWrapper = server.NewAppHandler(ctx, cfg, pubsubClient, storeClient, hub, http.Dir("web/out"))
				} else {
					muxWrapper = server.NewAppHandler(ctx, cfg, pubsubClient, storeClient, hub, http.FS(subFS))
				}

				// Re-create the inner ServeMux for the worker routes if needed.
				// Actually, since we wrapped it in corsMiddleware, it's not a ServeMux directly.
				// For the sake of this change, we'll keep them separate or use a trick.

				// Let's just create a new mux that wraps everything
				mainMux := http.NewServeMux()

				_, err = worker.RegisterRoutes(ctx, mainMux, cfg, pubsubClient, storeClient)
				if err != nil {
					errChan <- fmt.Errorf("worker register error: %w", err)
					return
				}

				// Add the server handler
				mainMux.Handle("/", muxWrapper)

				port := cfg.Server.Port
				if port == "" {
					port = "8080"
				}
				srv := &http.Server{Addr: ":" + port, Handler: mainMux}
				go func() {
					<-ctx.Done()
					_ = srv.Shutdown(context.Background())
				}()
				slog.Info("Server & Worker listening", "port", port)
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					errChan <- fmt.Errorf("server failed: %w", err)
				}
			} else {
				subFS, err := fs.Sub(staticAssets, "out")
				var staticFS http.FileSystem
				if err != nil {
					slog.Warn("Failed to use embedded assets, falling back to disk", "error", err)
					staticFS = http.Dir("web/out")
				} else {
					staticFS = http.FS(subFS)
				}
				if err := server.Start(ctx, cfg, pubsubClient, storeClient, hub, staticFS); err != nil {
					errChan <- fmt.Errorf("server error: %w", err)
				}
			}
		}()
	}

	select {
	case <-ctx.Done():
		slog.Info("Shutting down processes...")
	case err := <-errChan:
		slog.Error("Process failed", "error", err)
		os.Exit(1)
	}
}
