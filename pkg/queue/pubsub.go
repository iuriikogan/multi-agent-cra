package queue

import (
	"context"
	"fmt"
	"log/slog"

	"cloud.google.com/go/pubsub/v2"
)

// Client wraps the Google Cloud Pub/Sub client.
type Client struct {
	projectID string
	client    *pubsub.Client
}

// NewClient initializes a new Pub/Sub client.
func NewClient(ctx context.Context, projectID string) (*Client, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub client: %w", err)
	}
	return &Client{projectID: projectID, client: client}, nil
}

// Publish sends a message to the specified topic.
func (c *Client) Publish(ctx context.Context, topicID string, data []byte) error {
	t := c.client.Publisher(topicID)
	result := t.Publish(ctx, &pubsub.Message{Data: data})
	id, err := result.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	slog.Debug("Published message", "id", id, "topic", topicID)
	return nil
}

// Subscribe listens for messages on the specified subscription.
// It blocks until the context is cancelled.
func (c *Client) Subscribe(ctx context.Context, subID string, handler func(ctx context.Context, data []byte) error) error {
	sub := c.client.Subscriber(subID)
	slog.Info("Subscribing to topic", "subscription", subID)
	return sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		slog.Debug("Received message", "id", msg.ID)
		if err := handler(ctx, msg.Data); err != nil {
			slog.Error("Failed to handle message", "error", err)
			msg.Nack()
			return
		}
		msg.Ack()
	})
}

// Close closes the underlying Pub/Sub client.
func (c *Client) Close() error {
	return c.client.Close()
}
