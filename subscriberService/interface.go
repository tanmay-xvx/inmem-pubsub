// Package subscriberService provides subscriber management functionality for the Pub/Sub system.
package subscriberService

import (
	"context"

	"github.com/tanmay-xvx/inmem-pubsub/internals/models"
	"github.com/tanmay-xvx/inmem-pubsub/topicManagerService"
)

// SubscriberService defines the interface for managing subscribers and handling WebSocket connections.
// The service depends on TopicManager (registry) to locate topics for subscription and publishing operations.
type SubscriberService interface {
	// Start initializes the service and prepares resources for operation.
	Start() error

	// Shutdown gracefully shuts down the service, closing all active connections
	// and cleaning up resources. The context can be used to set a timeout.
	Shutdown(ctx context.Context) error

	// Publish sends a message to a specific topic. The service will locate the topic
	// via the TopicManager and publish the message to all subscribers of that topic.
	Publish(topic string, msg models.Message) error

	// GetTopicManager returns the topic manager used by this service.
	// This is needed for WebSocket handlers to access topic operations.
	GetTopicManager() topicManagerService.TopicManager
}
