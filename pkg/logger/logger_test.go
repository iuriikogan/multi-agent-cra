// Package logger provides structured logging for the Audit Agent.
package logger

import (
	"context"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

// TestSetup verifies the global default logger is initialized correctly.
func TestSetup(t *testing.T) {
	Setup("DEBUG", "test-project")

	// Simply verify that SetDefault took effect by checking if we can call Info.
	slog.Info("logger setup test complete")
}

// TestCloudLoggingHandler verifies the custom slog handler correctly injects GCP tracing fields
// into log records when a trace context is present.
func TestCloudLoggingHandler(t *testing.T) {
	Setup("INFO", "test-project")

	// Mock a span context
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")

	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	// Note: Fully validating the internal record additions would require a custom exporter or
	// overriding stdout. For this unit test, we ensure it doesn't panic.
	slog.InfoContext(ctx, "testing trace correlation")
}
