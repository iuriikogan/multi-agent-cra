package observability

import (
	"context"
	"fmt"
	"log/slog"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

var tp *sdktrace.TracerProvider

// InitTrace initializes an OpenTelemetry trace provider with a Google Cloud Trace exporter.
func InitTrace(ctx context.Context, projectID string) error {
	exporter, err := texporter.New(texporter.WithProjectID(projectID))
	if err != nil {
		return fmt.Errorf("failed to create Google Cloud Trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("multi-agent-cra"),
		),
		resource.WithHost(),
		resource.WithTelemetrySDK(),
		resource.WithProcess(),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	tp = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // In production, consider using a lower sampling rate.
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	slog.Info("OpenTelemetry Trace Provider initialized with Google Cloud Trace exporter")
	return nil
}

// Shutdown flushes all remaining spans to the exporter.
func Shutdown(ctx context.Context) {
	if tp != nil {
		if err := tp.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown Trace Provider", "error", err)
		}
	}
}
