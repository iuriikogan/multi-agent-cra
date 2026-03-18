// Package queue provides a wrapper for Google Cloud Pub/Sub with OpenTelemetry instrumentation.
//
// Rationale: Asynchronous multi-agent communication relies on Pub/Sub. Ensuring
// end-to-end trace propagation across message publish/subscribe boundaries is
// essential for distributed observability in the Google Cloud ecosystem.
package queue

import (
	"context"
	"fmt"
	"log/slog"

	"cloud.google.com/go/pubsub/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Client wraps the official Google Cloud Pub/Sub v2 client to simplify common operations.
type Client struct {
	projectID string
	client    *pubsub.Client
}

// NewClient initializes a connection to the Google Cloud Pub/Sub service for the specified project.
//
// Parameters:
//   - ctx: The context for the initialization of the client.
//   - projectID: The Google Cloud Project ID.
func NewClient(ctx context.Context, projectID string) (*Client, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub: failed to initialize client: %w", err)
	}
	return &Client{projectID: projectID, client: client}, nil
}

// Publish sends data to a specified Pub/Sub topic and injects the current trace context into the attributes.
//
// Parameters:
//   - ctx: The context containing current trace information.
//   - topicID: The target Pub/Sub topic ID.
//   - data: The byte payload to be published.
func (c *Client) Publish(ctx context.Context, topicID string, data []byte) error {
	tracer := otel.Tracer("pubsub")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("publish/%s", topicID), trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()

	msg := &pubsub.Message{
		Data:       data,
		Attributes: make(map[string]string),
	}

	// Inject the current trace context into the message attributes for propagation.
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(msg.Attributes))

	publisher := c.client.Publisher(topicID)
	result := publisher.Publish(ctx, msg)
	id, err := result.Get(ctx)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("pubsub: failed to publish message to %s: %w", topicID, err)
	}

	slog.Debug("pubsub: published message", "msg_id", id, "topic", topicID)
	return nil
}

// Subscribe starts an asynchronous listener on a specified subscription and executes the handler on message receipt.
// It automatically extracts trace context from message attributes to maintain trace continuity.
//
// Parameters:
//   - ctx: The parent context for the subscription.
//   - subID: The target Pub/Sub subscription ID.
//   - handler: Function to process the message payload.
func (c *Client) Subscribe(ctx context.Context, subID string, handler func(ctx context.Context, data []byte) error) error {
	subscription := c.client.Subscriber(subID)
	slog.Info("pubsub: subscribing to topic", "sub_id", subID)

	// Pull and process messages asynchronously.
	return subscription.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		// Extract the trace context from the message attributes.
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(msg.Attributes))

		tracer := otel.Tracer("pubsub")
		ctx, span := tracer.Start(ctx, fmt.Sprintf("receive/%s", subID), trace.WithSpanKind(trace.SpanKindConsumer))
		defer span.End()

		slog.Debug("pubsub: received message", "msg_id", msg.ID)

		// Execute the specialized business logic handler.
		if err := handler(ctx, msg.Data); err != nil {
			slog.Error("pubsub: message handling failed", "error", err, "msg_id", msg.ID)
			span.RecordError(err)
			msg.Nack() // Ensure the message is retried according to the subscription policy.
			return
		}

		msg.Ack() // Mark message as successfully processed.
	})
}

// Close gracefully releases any resources held by the underlying Pub/Sub client.
func (c *Client) Close() error {
	return c.client.Close()
}
