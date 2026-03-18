// Package logger provides a structured logging wrapper using slog with Google Cloud Trace correlation.
//
// Rationale: Structured logging in JSON format is essential for ingestion by Google Cloud Logging.
// Injecting trace and span IDs enables seamless log-trace correlation in the Cloud Console,
// which is critical for debugging distributed, asynchronous multi-agent workflows.
package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// Setup initializes the global default slog logger with the specified log level
// and configures it to output JSON logs compatible with Google Cloud Logging.
//
// Parameters:
//   - level: The desired log level (DEBUG, INFO, WARN, ERROR). Defaults to INFO.
//   - projectID: The Google Cloud Project ID, used to format the trace resource names.
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
		AddSource: true, // Includes file and line number in the log output.
	}

	// Use JSON handler as the base for structured logging.
	jsonHandler := slog.NewJSONHandler(os.Stdout, opts)

	// Wrap with our custom Cloud Logging handler for trace correlation.
	wrappedHandler := &cloudLoggingHandler{
		Handler:   jsonHandler,
		projectID: projectID,
	}

	logger := slog.New(wrappedHandler)
	slog.SetDefault(logger)
}

// cloudLoggingHandler is a custom slog.Handler that intercepts log records to
// inject Google Cloud-specific trace correlation fields if a trace context is present.
type cloudLoggingHandler struct {
	slog.Handler
	projectID string
}

// Handle extracts OpenTelemetry trace information from the context and adds it to the log record
// using the special field names recognized by Google Cloud Logging.
func (h *cloudLoggingHandler) Handle(ctx context.Context, r slog.Record) error {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.HasTraceID() {
		// Inject special fields for Google Cloud Logging log-trace correlation.
		// Reference: https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
		r.AddAttrs(
			slog.String("logging.googleapis.com/trace", fmt.Sprintf("projects/%s/traces/%s", h.projectID, spanCtx.TraceID().String())),
			slog.String("logging.googleapis.com/spanId", spanCtx.SpanID().String()),
			slog.Bool("logging.googleapis.com/trace_sampled", spanCtx.IsSampled()),
		)
	}
	return h.Handler.Handle(ctx, r)
}
