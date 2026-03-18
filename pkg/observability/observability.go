// Package observability provides distributed tracing and telemetry exports for the multi-agent system.
//
// Rationale: Asynchronous workflows involving Multiple Agents across Pub/Sub require
// end-to-end trace propagation to understand latency and identify bottlenecks.
// This package integrates OpenTelemetry with Google Cloud Trace.
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

var (
	tp   *sdktrace.TracerProvider
	once sync.Once
)

// InitTrace initializes the global OpenTelemetry TracerProvider with a Google Cloud Trace exporter.
// It ensures that initialization logic is thread-safe and executed exactly once.
//
// Parameters:
//   - ctx: The context for the resource configuration.
//   - projectID: The Google Cloud Project ID for the trace exporter.
func InitTrace(ctx context.Context, projectID string) error {
	var err error
	once.Do(func() {
		// Initialize the Google Cloud Trace exporter.
		exporter, e := texporter.New(texporter.WithProjectID(projectID))
		if e != nil {
			err = fmt.Errorf("observability: failed to create Google Cloud Trace exporter: %w", e)
			return
		}

		// Define the resource attributes for the Audit-Agent service.
		res, e := resource.New(ctx,
			resource.WithAttributes(
				semconv.ServiceNameKey.String("Audit-Agent"),
			),
			resource.WithHost(),
			resource.WithTelemetrySDK(),
			resource.WithProcess(),
		)
		if e != nil {
			err = fmt.Errorf("observability: failed to create trace resource: %w", e)
			return
		}

		// Create a TracerProvider with batch exporting and sampling.
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.AlwaysSample()), // Sample every trace (adjust for production as needed).
		)

		// Set global tracer and propagator for context injection across Pub/Sub and HTTP.
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

		slog.Info("OpenTelemetry Trace Provider successfully initialized with Google Cloud Trace exporter")
	})
	return err
}

// Shutdown ensures all remaining spans in the trace buffer are flushed to the exporter
// before the application exits. It should be called during a graceful shutdown sequence.
func Shutdown(ctx context.Context) {
	if tp != nil {
		if err := tp.Shutdown(ctx); err != nil {
			slog.Error("observability: failed to shutdown TracerProvider gracefully", "error", err)
		} else {
			slog.Info("observability: TracerProvider shutdown successfully")
		}
	}
}
