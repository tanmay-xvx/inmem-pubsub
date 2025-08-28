package topicManagerService

import (
	"errors"
	"testing"

	"github.com/tanmay-xvx/inmem-pubsub/internals/topic"
)

// Error constants for testing
var (
	ErrInvalidTopicName   = errors.New("invalid topic name")
	ErrTopicAlreadyExists = errors.New("topic already exists")
	ErrTopicNotFound      = errors.New("topic not found")
)

// mockTopicManager is a mock implementation for testing the interface
type mockTopicManager struct {
	topics map[string]bool
}

func (m *mockTopicManager) CreateTopic(name string) error {
	if name == "" {
		return ErrInvalidTopicName
	}
	if m.topics[name] {
		return ErrTopicAlreadyExists
	}
	m.topics[name] = true
	return nil
}

func (m *mockTopicManager) DeleteTopic(name string) error {
	if name == "" {
		return ErrInvalidTopicName
	}
	if !m.topics[name] {
		return ErrTopicNotFound
	}
	delete(m.topics, name)
	return nil
}

func (m *mockTopicManager) ListTopics() []TopicInfo {
	topics := make([]TopicInfo, 0, len(m.topics))
	for name := range m.topics {
		topics = append(topics, TopicInfo{
			Name:        name,
			Subscribers: 0,
			Messages:    0,
			Dropped:     0,
		})
	}
	return topics
}

func (m *mockTopicManager) GetTopic(name string) (*topic.Topic, bool) {
	if m.topics[name] {
		// Return nil for mock, but indicate it exists
		return nil, true
	}
	return nil, false
}

func (m *mockTopicManager) Stats() map[string]TopicStats {
	stats := make(map[string]TopicStats)
	for name := range m.topics {
		stats[name] = TopicStats{
			Name:        name,
			Subscribers: 0,
			Messages:    0,
			Dropped:     0,
		}
	}
	return stats
}

// TestTopicManagerInterface tests that the interface can be implemented and used
func TestTopicManagerInterface(t *testing.T) {
	var tm TopicManager = &mockTopicManager{
		topics: make(map[string]bool),
	}

	// Test CreateTopic
	err := tm.CreateTopic("test-topic")
	if err != nil {
		t.Errorf("Failed to create topic: %v", err)
	}

	// Test ListTopics
	topics := tm.ListTopics()
	if len(topics) != 1 {
		t.Errorf("Expected 1 topic, got %d", len(topics))
	}

	if topics[0].Name != "test-topic" {
		t.Errorf("Expected topic name 'test-topic', got '%s'", topics[0].Name)
	}

	// Test GetTopic
	_, exists := tm.GetTopic("test-topic")
	if !exists {
		t.Error("Topic should exist")
	}

	// Test Stats
	stats := tm.Stats()
	if len(stats) != 1 {
		t.Errorf("Expected 1 topic in stats, got %d", len(stats))
	}

	if stats["test-topic"].Name != "test-topic" {
		t.Errorf("Expected topic name 'test-topic', got '%s'", stats["test-topic"].Name)
	}

	// Test DeleteTopic
	err = tm.DeleteTopic("test-topic")
	if err != nil {
		t.Errorf("Failed to delete topic: %v", err)
	}

	// Verify deletion
	topics = tm.ListTopics()
	if len(topics) != 0 {
		t.Errorf("Expected 0 topics after deletion, got %d", len(topics))
	}
}

// TestTopicInfoStructure tests the TopicInfo struct
func TestTopicInfoStructure(t *testing.T) {
	info := TopicInfo{
		Name:        "test-topic",
		Subscribers: 5,
		Messages:    100,
		Dropped:     2,
	}

	if info.Name != "test-topic" {
		t.Errorf("Expected name 'test-topic', got '%s'", info.Name)
	}

	if info.Subscribers != 5 {
		t.Errorf("Expected 5 subscribers, got %d", info.Subscribers)
	}

	if info.Messages != 100 {
		t.Errorf("Expected 100 messages, got %d", info.Messages)
	}

	if info.Dropped != 2 {
		t.Errorf("Expected 2 dropped, got %d", info.Dropped)
	}
}

// TestTopicStatsStructure tests the TopicStats struct
func TestTopicStatsStructure(t *testing.T) {
	stats := TopicStats{
		Name:        "test-topic",
		Subscribers: 10,
		Messages:    500,
		Dropped:     5,
	}

	if stats.Name != "test-topic" {
		t.Errorf("Expected name 'test-topic', got '%s'", stats.Name)
	}

	if stats.Subscribers != 10 {
		t.Errorf("Expected 10 subscribers, got %d", stats.Subscribers)
	}

	if stats.Messages != 500 {
		t.Errorf("Expected 500 messages, got %d", stats.Messages)
	}

	if stats.Dropped != 5 {
		t.Errorf("Expected 5 dropped, got %d", stats.Dropped)
	}
}

// TestInterfaceCompliance ensures the mock implements all required methods
func TestInterfaceCompliance(t *testing.T) {
	var _ TopicManager = (*mockTopicManager)(nil)
}
