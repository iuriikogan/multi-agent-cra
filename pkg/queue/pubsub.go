// Package queue provides a wrapper for Google Cloud Pub/Sub operations.
package queue

import (
	"context"
	"fmt"
	"log/slog"

	"cloud.google.com/go/pubsub/v2"
)

// Client wraps the official Google Cloud Pub/Sub client for simplified interactions.
type Client struct {
	projectID string
	client    *pubsub.Client
}

// NewClient initializes and returns a connection to Google Cloud Pub/Sub.
func NewClient(ctx context.Context, projectID string) (*Client, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub client: %w", err)
	}
	return &Client{projectID: projectID, client: client}, nil
}

// Publish sends data to the specified Pub/Sub topic.
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

// Subscribe starts receiving messages from a subscription and executes the provided handler.
// This is a blocking operation until the context is cancelled.
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

// Close releases the underlying Pub/Sub client resources.
func (c *Client) Close() error {
	return c.client.Close()
}
