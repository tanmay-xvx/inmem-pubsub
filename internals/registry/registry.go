// Package registry provides topic registry management for the Pub/Sub system.
package registry

import (
	"log"
	"sync"
	"time"

	"github.com/tanmay-xvx/inmem-pubsub/internals/config"
	"github.com/tanmay-xvx/inmem-pubsub/internals/metrics"
	"github.com/tanmay-xvx/inmem-pubsub/internals/models"
	"github.com/tanmay-xvx/inmem-pubsub/internals/topic"
)

// TopicInfo provides information about a topic for listing and monitoring.
type TopicInfo struct {
	Name           string `json:"name"`
	Subscribers    int    `json:"subscribers"`
	Messages       uint64 `json:"messages"`
	Dropped        uint64 `json:"dropped"`
	RingBufferSize int    `json:"ring_buffer_size"`
}

// TopicStats provides detailed statistics for a topic.
type TopicStats struct {
	Name           string `json:"name"`
	Subscribers    int    `json:"subscribers"`
	Messages       uint64 `json:"messages"`
	Dropped        uint64 `json:"dropped"`
	RingBufferSize int    `json:"ring_buffer_size"`
	LastPublish    string `json:"last_publish,omitempty"`
	LastActivity   string `json:"last_activity,omitempty"`
}

// Registry manages all topics in the Pub/Sub system with thread-safe operations.
type Registry struct {
	topics  map[string]*topic.Topic
	mu      sync.RWMutex
	cfg     *config.Config
	metrics *metrics.Metrics
}

// NewRegistry creates a new topic registry with the specified configuration and metrics.
func NewRegistry(cfg *config.Config, metrics *metrics.Metrics) *Registry {
	return &Registry{
		topics:  make(map[string]*topic.Topic),
		cfg:     cfg,
		metrics: metrics,
	}
}

// CreateTopic creates a new topic with the specified name.
// Returns an error if the topic already exists.
func (r *Registry) CreateTopic(name string) error {
	if name == "" {
		return ErrInvalidTopicName
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if topic already exists
	if _, exists := r.topics[name]; exists {
		return ErrTopicAlreadyExists
	}

	// Create new topic with configured ring buffer size
	newTopic := topic.NewTopic(name, r.cfg.DefaultRingBufferSize)
	r.topics[name] = newTopic

	// Update metrics
	r.metrics.IncTopics()

	log.Printf("Created topic: %s", name)
	return nil
}

// DeleteTopic deletes a topic and notifies all subscribers with a "topic_deleted" message.
// All subscribers are closed and removed.
func (r *Registry) DeleteTopic(name string) error {
	if name == "" {
		return ErrInvalidTopicName
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	topic, exists := r.topics[name]
	if !exists {
		return ErrTopicNotFound
	}

	// Notify all subscribers about topic deletion
	subscriberIDs := topic.ListSubscriberIDs()
	for _, clientID := range subscriberIDs {
		// Send topic deleted notification
		notification := models.ServerMsg{
			Type:  "topic_deleted",
			Topic: name,
			Ts:    time.Now(),
		}

		// Try to send notification (non-blocking)
		if sub, found := topic.GetSubscriber(clientID); found {
			select {
			case sub.Send <- notification:
				// Notification sent successfully
			default:
				// Can't send notification, continue with cleanup
			}
		}
	}

	// Close the topic (this will close all subscribers)
	topic.Close()

	// Remove from registry
	delete(r.topics, name)

	// Update metrics
	r.metrics.DecTopics()

	log.Printf("Deleted topic: %s (closed %d subscribers)", name, len(subscriberIDs))
	return nil
}

// GetTopic retrieves a topic by name.
// Returns the topic and a boolean indicating if it was found.
func (r *Registry) GetTopic(name string) (*topic.Topic, bool) {
	if name == "" {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	topic, exists := r.topics[name]
	return topic, exists
}

// ListTopics returns information about all topics in the registry.
func (r *Registry) ListTopics() []TopicInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	topics := make([]TopicInfo, 0, len(r.topics))
	for name, t := range r.topics {
		topics = append(topics, TopicInfo{
			Name:           name,
			Subscribers:    t.GetSubscriberCount(),
			Messages:       t.GetMessageCount(),
			Dropped:        t.GetDroppedCount(),
			RingBufferSize: t.GetRingBufferSize(),
		})
	}

	return topics
}

// Stats returns detailed statistics for all topics.
func (r *Registry) Stats() map[string]TopicStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]TopicStats)
	for name, t := range r.topics {
		// Get metrics from metrics system
		if topicMetrics := r.metrics.GetTopicMetrics(name); topicMetrics != nil {
			stats[name] = TopicStats{
				Name:           name,
				Subscribers:    t.GetSubscriberCount(),
				Messages:       t.GetMessageCount(),
				Dropped:        t.GetDroppedCount(),
				RingBufferSize: t.GetRingBufferSize(),
			}
		} else {
			// Fallback to topic-only stats if metrics not available
			stats[name] = TopicStats{
				Name:           name,
				Subscribers:    t.GetSubscriberCount(),
				Messages:       t.GetMessageCount(),
				Dropped:        t.GetDroppedCount(),
				RingBufferSize: t.GetRingBufferSize(),
			}
		}
	}

	return stats
}

// PublishMessage publishes a message to a topic and updates metrics.
// This is a convenience method that combines topic retrieval and publishing.
func (r *Registry) PublishMessage(topicName string, msg models.Message) (delivered int, dropped int, err error) {
	topic, exists := r.GetTopic(topicName)
	if !exists {
		return 0, 0, ErrTopicNotFound
	}

	// Publish message using configured policy and buffer size
	delivered, dropped = topic.Publish(msg, r.cfg.DefaultPublishPolicy, r.cfg.DefaultWSBufferSize)

	// Update metrics
	r.metrics.IncDelivered(topicName, delivered)
	r.metrics.IncDropped(topicName, dropped)

	return delivered, dropped, nil
}

// GetOrCreateTopic retrieves a topic by name, creating it if it doesn't exist.
// This is useful for ensuring topics exist before publishing.
func (r *Registry) GetOrCreateTopic(name string) (*topic.Topic, error) {
	if name == "" {
		return nil, ErrInvalidTopicName
	}

	// Try to get existing topic first
	if topic, exists := r.GetTopic(name); exists {
		return topic, nil
	}

	// Create new topic
	if err := r.CreateTopic(name); err != nil {
		return nil, err
	}

	// Return the newly created topic
	topic, _ := r.GetTopic(name)
	return topic, nil
}

// Close closes all topics and cleans up the registry.
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("Closing registry with %d topics", len(r.topics))

	for name, t := range r.topics {
		log.Printf("Closing topic: %s", name)
		t.Close()
	}

	// Clear topics map
	r.topics = make(map[string]*topic.Topic)

	log.Printf("Registry closed")
}

// GetTopicCount returns the total number of topics in the registry.
func (r *Registry) GetTopicCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.topics)
}

// GetTotalSubscriberCount returns the total number of subscribers across all topics.
func (r *Registry) GetTotalSubscriberCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := 0
	for _, t := range r.topics {
		total += t.GetSubscriberCount()
	}
	return total
}
