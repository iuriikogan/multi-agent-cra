// Package logger provides a structured logging wrapper using slog with Google Cloud Trace correlation.
package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// Setup initializes the global default logger with the given level and project ID for tracing.
func Setup(level string, projectID string) {
	var logLevel slog.Level
	switch level {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true,
	}

	// Use JSON handler for structured logs suitable for Google Cloud Logging.
	jsonHandler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(&cloudLoggingHandler{Handler: jsonHandler, projectID: projectID})
	slog.SetDefault(logger)
}

// cloudLoggingHandler is a custom slog handler that injects Google Cloud Trace correlation fields.
type cloudLoggingHandler struct {
	slog.Handler
	projectID string
}

func (h *cloudLoggingHandler) Handle(ctx context.Context, r slog.Record) error {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.HasTraceID() {
		// Use Google Cloud Logging's special fields for trace correlation.
		// https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
		r.AddAttrs(
			slog.String("logging.googleapis.com/trace", fmt.Sprintf("projects/%s/traces/%s", h.projectID, spanCtx.TraceID().String())),
			slog.String("logging.googleapis.com/spanId", spanCtx.SpanID().String()),
			slog.Bool("logging.googleapis.com/trace_sampled", spanCtx.IsSampled()),
		)
	}
	return h.Handler.Handle(ctx, r)
}
