// Package subscriberService provides subscriber management functionality for the Pub/Sub system.
package subscriberService

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/tanmay-xvx/inmem-pubsub/internals/config"
	"github.com/tanmay-xvx/inmem-pubsub/internals/models"
	"github.com/tanmay-xvx/inmem-pubsub/internals/registry"
	"github.com/tanmay-xvx/inmem-pubsub/internals/subscriber"
	"github.com/tanmay-xvx/inmem-pubsub/topicManagerService"
)

// SubscriberServiceImpl implements the SubscriberService interface.
type SubscriberServiceImpl struct {
	registry *registry.Registry
	cfg      *config.Config
	topicMgr topicManagerService.TopicManager

	// Connection management
	activeConnsMu sync.RWMutex
	activeConns   map[*websocket.Conn]struct{}

	// Client to subscribers mapping for cleanup
	clientSubsMu sync.RWMutex
	clientSubs   map[string]map[string]*subscriber.Subscriber // clientID -> topic -> subscriber
}

// NewSubscriberService creates a new subscriber service with the specified dependencies.
func NewSubscriberService(registry *registry.Registry, cfg *config.Config, topicMgr topicManagerService.TopicManager) *SubscriberServiceImpl {
	return &SubscriberServiceImpl{
		registry:    registry,
		cfg:         cfg,
		topicMgr:    topicMgr,
		activeConns: make(map[*websocket.Conn]struct{}),
		clientSubs:  make(map[string]map[string]*subscriber.Subscriber),
	}
}

// Start initializes the service and prepares resources for operation.
func (s *SubscriberServiceImpl) Start() error {
	log.Println("Starting subscriber service...")
	// No specific initialization needed for now
	return nil
}

// Shutdown gracefully shuts down the service, closing all active connections
// and cleaning up resources.
func (s *SubscriberServiceImpl) Shutdown(ctx context.Context) error {
	log.Println("Shutting down subscriber service...")

	// Clean up all client subscriptions
	s.clientSubsMu.Lock()
	for clientID, topicSubs := range s.clientSubs {
		for topicName, sub := range topicSubs {
			// Remove from topic
			if topic, exists := s.topicMgr.GetTopic(topicName); exists {
				topic.RemoveSubscriber(clientID)
			}

			// Close subscriber
			sub.Close()
		}
		delete(s.clientSubs, clientID)
	}
	s.clientSubsMu.Unlock()

	// Close all active connections
	s.activeConnsMu.Lock()
	for conn := range s.activeConns {
		conn.Close()
		delete(s.activeConns, conn)
	}
	s.activeConnsMu.Unlock()

	log.Println("Subscriber service shutdown complete")
	return nil
}

// Publish sends a message to a specific topic.
func (s *SubscriberServiceImpl) Publish(topic string, msg models.Message) error {
	// Get the topic from the registry
	topicObj, exists := s.topicMgr.GetTopic(topic)
	if !exists {
		return fmt.Errorf("topic '%s' not found", topic)
	}

	// Publish the message
	delivered, dropped := topicObj.Publish(msg, "DROP_OLDEST", 100)

	log.Printf("Published message to topic '%s': %d delivered, %d dropped", topic, delivered, dropped)
	return nil
}

// GetTopicManager returns the topic manager used by this service.
func (s *SubscriberServiceImpl) GetTopicManager() topicManagerService.TopicManager {
	return s.topicMgr
}

// RegisterConnection registers a new WebSocket connection.
func (s *SubscriberServiceImpl) RegisterConnection(conn *websocket.Conn) {
	s.activeConnsMu.Lock()
	s.activeConns[conn] = struct{}{}
	s.activeConnsMu.Unlock()
}

// UnregisterConnection removes a WebSocket connection.
func (s *SubscriberServiceImpl) UnregisterConnection(conn *websocket.Conn) {
	s.activeConnsMu.Lock()
	delete(s.activeConns, conn)
	s.activeConnsMu.Unlock()
}

// RegisterClientSubscriber registers a subscriber for a specific client and topic.
func (s *SubscriberServiceImpl) RegisterClientSubscriber(clientID, topic string, sub *subscriber.Subscriber) {
	s.clientSubsMu.Lock()
	defer s.clientSubsMu.Unlock()

	if s.clientSubs[clientID] == nil {
		s.clientSubs[clientID] = make(map[string]*subscriber.Subscriber)
	}
	s.clientSubs[clientID][topic] = sub
}

// UnregisterClientSubscriber removes a subscriber for a specific client and topic.
func (s *SubscriberServiceImpl) UnregisterClientSubscriber(clientID, topic string) {
	s.clientSubsMu.Lock()
	defer s.clientSubsMu.Unlock()

	if clientSubs, exists := s.clientSubs[clientID]; exists {
		delete(clientSubs, topic)
		if len(clientSubs) == 0 {
			delete(s.clientSubs, clientID)
		}
	}
}

// GetActiveConnectionCount returns the number of active WebSocket connections.
func (s *SubscriberServiceImpl) GetActiveConnectionCount() int {
	s.activeConnsMu.RLock()
	defer s.activeConnsMu.RUnlock()
	return len(s.activeConns)
}

// GetClientSubscriberCount returns the number of subscribers for a specific client.
func (s *SubscriberServiceImpl) GetClientSubscriberCount(clientID string) int {
	s.clientSubsMu.RLock()
	defer s.clientSubsMu.RUnlock()

	if clientSubs, exists := s.clientSubs[clientID]; exists {
		return len(clientSubs)
	}
	return 0
}
