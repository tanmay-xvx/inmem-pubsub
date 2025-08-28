// Package http provides HTTP handlers for the subscriber service.
package http

import (
	"github.com/go-chi/chi/v5"
	"github.com/tanmay-xvx/inmem-pubsub/subscriberService"
)

// RegisterSubscriberRoutes registers all subscriber service HTTP routes with the provided chi router.
// This function mounts the following endpoints:
//   - GET /ws - WebSocket endpoint for subscriber connections
//
// The function creates a new WebSocket handler with the provided subscriber service and
// registers the WebSocket route.
func RegisterSubscriberRoutes(r chi.Router, svc subscriberService.SubscriberService) {
	// Get the topic manager from the service
	topicManager := svc.GetTopicManager()

	// Create a new WebSocket handler
	handler := NewWebSocketHandler(topicManager)
	r.Get("/ws", handler.HandleWebSocket)
}
