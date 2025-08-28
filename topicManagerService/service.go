// Package topicManagerService provides topic management functionality for the Pub/Sub system.
package topicManagerService

import (
	"github.com/tanmay-xvx/inmem-pubsub/internals/config"
	"github.com/tanmay-xvx/inmem-pubsub/internals/metrics"
	"github.com/tanmay-xvx/inmem-pubsub/internals/registry"
	"github.com/tanmay-xvx/inmem-pubsub/internals/topic"
)

// TopicManagerServiceImpl implements the TopicManager interface using the registry.
type TopicManagerServiceImpl struct {
	registry *registry.Registry
	cfg      *config.Config
	metrics  *metrics.Metrics
}

// NewTopicManagerService creates a new topic manager service with the specified dependencies.
func NewTopicManagerService(registry *registry.Registry, cfg *config.Config, metrics *metrics.Metrics) *TopicManagerServiceImpl {
	return &TopicManagerServiceImpl{
		registry: registry,
		cfg:      cfg,
		metrics:  metrics,
	}
}

// CreateTopic creates a new topic with the specified name.
func (s *TopicManagerServiceImpl) CreateTopic(name string) error {
	return s.registry.CreateTopic(name)
}

// DeleteTopic deletes a topic with the specified name.
func (s *TopicManagerServiceImpl) DeleteTopic(name string) error {
	return s.registry.DeleteTopic(name)
}

// ListTopics returns a list of all topics with their information.
func (s *TopicManagerServiceImpl) ListTopics() []TopicInfo {
	registryTopics := s.registry.ListTopics()
	topics := make([]TopicInfo, len(registryTopics))
	for i, rt := range registryTopics {
		topics[i] = TopicInfo{
			Name:        rt.Name,
			Subscribers: rt.Subscribers,
			Messages:    rt.Messages,
			Dropped:     rt.Dropped,
		}
	}
	return topics
}

// GetTopic returns a topic with the specified name and a boolean indicating if it exists.
func (s *TopicManagerServiceImpl) GetTopic(name string) (*topic.Topic, bool) {
	return s.registry.GetTopic(name)
}

// Stats returns statistics for all topics.
func (s *TopicManagerServiceImpl) Stats() map[string]TopicStats {
	registryStats := s.registry.Stats()
	stats := make(map[string]TopicStats)
	for name, rs := range registryStats {
		stats[name] = TopicStats{
			Name:        rs.Name,
			Subscribers: rs.Subscribers,
			Messages:    rs.Messages,
			Dropped:     rs.Dropped,
		}
	}
	return stats
}
