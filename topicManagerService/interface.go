// Package topicManagerService provides the interface for topic management operations.
package topicManagerService

import (
	"github.com/tanmay-xvx/inmem-pubsub/internals/topic"
)

// TopicInfo provides basic information about a topic for listing and monitoring.
type TopicInfo struct {
	Name        string `json:"name"`
	Subscribers int    `json:"subscribers"`
	Messages    uint64 `json:"messages"`
	Dropped     uint64 `json:"dropped"`
}

// TopicStats provides detailed statistics for a topic.
type TopicStats struct {
	Name        string `json:"name"`
	Subscribers int    `json:"subscribers"`
	Messages    uint64 `json:"messages"`
	Dropped     uint64 `json:"dropped"`
}

// TopicManager defines the interface for topic management operations.
// Implementations should provide thread-safe topic creation, deletion,
// listing, and statistics gathering capabilities.
type TopicManager interface {
	// CreateTopic creates a new topic with the specified name.
	// Returns an error if the topic already exists or if the name is invalid.
	CreateTopic(name string) error

	// DeleteTopic deletes a topic and notifies all subscribers.
	// All subscribers are closed and removed from the topic.
	// Returns an error if the topic doesn't exist or if the name is invalid.
	DeleteTopic(name string) error

	// ListTopics returns information about all topics in the system.
	// The returned slice contains basic topic information for monitoring.
	ListTopics() []TopicInfo

	// GetTopic retrieves a topic by name.
	// Returns the topic and a boolean indicating if it was found.
	// Returns (nil, false) if the topic doesn't exist.
	GetTopic(name string) (*topic.Topic, bool)

	// Stats returns detailed statistics for all topics.
	// The returned map contains comprehensive topic statistics
	// indexed by topic name.
	Stats() map[string]TopicStats
}
