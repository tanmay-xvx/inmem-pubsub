package registry

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/tanmay-xvx/inmem-pubsub/internals/config"
	"github.com/tanmay-xvx/inmem-pubsub/internals/metrics"
	"github.com/tanmay-xvx/inmem-pubsub/internals/models"
)

func TestNewRegistry(t *testing.T) {
	cfg := config.NewConfig()
	metrics := metrics.NewMetrics()

	registry := NewRegistry(cfg, metrics)
	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if registry.GetTopicCount() != 0 {
		t.Errorf("Expected 0 topics, got %d", registry.GetTopicCount())
	}

	if registry.GetTotalSubscriberCount() != 0 {
		t.Errorf("Expected 0 total subscribers, got %d", registry.GetTotalSubscriberCount())
	}
}

func TestRegistry_CreateTopic(t *testing.T) {
	cfg := config.NewConfig()
	metrics := metrics.NewMetrics()
	registry := NewRegistry(cfg, metrics)

	// Test creating a valid topic
	err := registry.CreateTopic("test-topic")
	if err != nil {
		t.Errorf("Failed to create topic: %v", err)
	}

	if registry.GetTopicCount() != 1 {
		t.Errorf("Expected 1 topic, got %d", registry.GetTopicCount())
	}

	// Test creating topic with empty name
	err = registry.CreateTopic("")
	if err != ErrInvalidTopicName {
		t.Errorf("Expected ErrInvalidTopicName, got %v", err)
	}

	// Test creating duplicate topic
	err = registry.CreateTopic("test-topic")
	if err != ErrTopicAlreadyExists {
		t.Errorf("Expected ErrTopicAlreadyExists, got %v", err)
	}
}

func TestRegistry_DeleteTopic(t *testing.T) {
	cfg := config.NewConfig()
	metrics := metrics.NewMetrics()
	registry := NewRegistry(cfg, metrics)

	// Create a topic first
	err := registry.CreateTopic("test-topic")
	if err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}

	if registry.GetTopicCount() != 1 {
		t.Error("Topic not created properly")
	}

	// Test deleting existing topic
	err = registry.DeleteTopic("test-topic")
	if err != nil {
		t.Errorf("Failed to delete topic: %v", err)
	}

	if registry.GetTopicCount() != 0 {
		t.Errorf("Expected 0 topics after deletion, got %d", registry.GetTopicCount())
	}

	// Test deleting non-existent topic
	err = registry.DeleteTopic("non-existent")
	if err != ErrTopicNotFound {
		t.Errorf("Expected ErrTopicNotFound, got %v", err)
	}

	// Test deleting topic with empty name
	err = registry.DeleteTopic("")
	if err != ErrInvalidTopicName {
		t.Errorf("Expected ErrInvalidTopicName, got %v", err)
	}
}

func TestRegistry_GetTopic(t *testing.T) {
	cfg := config.NewConfig()
	metrics := metrics.NewMetrics()
	registry := NewRegistry(cfg, metrics)

	// Create a topic
	err := registry.CreateTopic("test-topic")
	if err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}

	// Test getting existing topic
	topic, exists := registry.GetTopic("test-topic")
	if !exists {
		t.Error("Topic not found")
	}
	if topic == nil {
		t.Error("Topic is nil")
	}
	if topic.Name != "test-topic" {
		t.Errorf("Expected topic name 'test-topic', got '%s'", topic.Name)
	}

	// Test getting non-existent topic
	topic, exists = registry.GetTopic("non-existent")
	if exists {
		t.Error("Topic should not exist")
	}
	if topic != nil {
		t.Error("Topic should be nil")
	}

	// Test getting topic with empty name
	topic, exists = registry.GetTopic("")
	if exists {
		t.Error("Topic should not exist")
	}
}

func TestRegistry_ListTopics(t *testing.T) {
	cfg := config.NewConfig()
	metrics := metrics.NewMetrics()
	registry := NewRegistry(cfg, metrics)

	// Create multiple topics
	topics := []string{"topic-1", "topic-2", "topic-3"}
	for _, name := range topics {
		err := registry.CreateTopic(name)
		if err != nil {
			t.Fatalf("Failed to create topic %s: %v", name, err)
		}
	}

	// List topics
	topicList := registry.ListTopics()
	if len(topicList) != 3 {
		t.Errorf("Expected 3 topics, got %d", len(topicList))
	}

	// Check that all topic names are present
	found := make(map[string]bool)
	for _, topic := range topicList {
		found[topic.Name] = true
	}

	for _, name := range topics {
		if !found[name] {
			t.Errorf("Topic %s not found in list", name)
		}
	}
}

func TestRegistry_Stats(t *testing.T) {
	cfg := config.NewConfig()
	metrics := metrics.NewMetrics()
	registry := NewRegistry(cfg, metrics)

	// Create a topic
	err := registry.CreateTopic("test-topic")
	if err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}

	// Get stats
	stats := registry.Stats()
	if len(stats) != 1 {
		t.Errorf("Expected 1 topic in stats, got %d", len(stats))
	}

	topicStats, exists := stats["test-topic"]
	if !exists {
		t.Fatal("Topic stats not found")
	}

	if topicStats.Name != "test-topic" {
		t.Errorf("Expected topic name 'test-topic', got '%s'", topicStats.Name)
	}

	if topicStats.Subscribers != 0 {
		t.Errorf("Expected 0 subscribers, got %d", topicStats.Subscribers)
	}

	if topicStats.Messages != 0 {
		t.Errorf("Expected 0 messages, got %d", topicStats.Messages)
	}

	if topicStats.Dropped != 0 {
		t.Errorf("Expected 0 dropped messages, got %d", topicStats.Dropped)
	}
}

func TestRegistry_PublishMessage(t *testing.T) {
	cfg := config.NewConfig()
	metrics := metrics.NewMetrics()
	registry := NewRegistry(cfg, metrics)

	// Create a topic
	err := registry.CreateTopic("test-topic")
	if err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}

	// Publish message to existing topic
	msg := models.Message{
		ID:      "msg-1",
		Payload: json.RawMessage(`{"test": "data"}`),
	}

	delivered, dropped, err := registry.PublishMessage("test-topic", msg)
	if err != nil {
		t.Errorf("Failed to publish message: %v", err)
	}

	if delivered != 0 {
		t.Errorf("Expected 0 delivered (no subscribers), got %d", delivered)
	}

	if dropped != 0 {
		t.Errorf("Expected 0 dropped, got %d", dropped)
	}

	// Publish message to non-existent topic
	delivered, dropped, err = registry.PublishMessage("non-existent", msg)
	if err != ErrTopicNotFound {
		t.Errorf("Expected ErrTopicNotFound, got %v", err)
	}

	if delivered != 0 || dropped != 0 {
		t.Errorf("Expected 0 delivered/dropped for non-existent topic, got %d/%d", delivered, dropped)
	}
}

func TestRegistry_GetOrCreateTopic(t *testing.T) {
	cfg := config.NewConfig()
	metrics := metrics.NewMetrics()
	registry := NewRegistry(cfg, metrics)

	// Test getting non-existent topic (should create it)
	topic, err := registry.GetOrCreateTopic("new-topic")
	if err != nil {
		t.Errorf("Failed to get or create topic: %v", err)
	}

	if topic == nil {
		t.Error("Topic is nil")
	}

	if topic.Name != "new-topic" {
		t.Errorf("Expected topic name 'new-topic', got '%s'", topic.Name)
	}

	if registry.GetTopicCount() != 1 {
		t.Errorf("Expected 1 topic, got %d", registry.GetTopicCount())
	}

	// Test getting existing topic (should return existing)
	topic2, err := registry.GetOrCreateTopic("new-topic")
	if err != nil {
		t.Errorf("Failed to get existing topic: %v", err)
	}

	if topic2 != topic {
		t.Error("Should return same topic instance")
	}

	if registry.GetTopicCount() != 1 {
		t.Errorf("Expected 1 topic, got %d", registry.GetTopicCount())
	}

	// Test with empty name
	topic, err = registry.GetOrCreateTopic("")
	if err != ErrInvalidTopicName {
		t.Errorf("Expected ErrInvalidTopicName, got %v", err)
	}

	if topic != nil {
		t.Error("Topic should be nil for invalid name")
	}
}

func TestRegistry_Close(t *testing.T) {
	cfg := config.NewConfig()
	metrics := metrics.NewMetrics()
	registry := NewRegistry(cfg, metrics)

	// Create some topics
	topics := []string{"topic-1", "topic-2"}
	for _, name := range topics {
		err := registry.CreateTopic(name)
		if err != nil {
			t.Fatalf("Failed to create topic %s: %v", name, err)
		}
	}

	if registry.GetTopicCount() != 2 {
		t.Error("Topics not created properly")
	}

	// Close registry
	registry.Close()

	if registry.GetTopicCount() != 0 {
		t.Errorf("Expected 0 topics after close, got %d", registry.GetTopicCount())
	}
}

func TestRegistry_Concurrency(t *testing.T) {
	cfg := config.NewConfig()
	metrics := metrics.NewMetrics()
	registry := NewRegistry(cfg, metrics)

	// Test concurrent topic creation
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			topicName := fmt.Sprintf("concurrent-topic-%d", id)
			err := registry.CreateTopic(topicName)
			if err != nil {
				t.Errorf("Failed to create topic %s: %v", topicName, err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	if registry.GetTopicCount() != 10 {
		t.Errorf("Expected 10 topics, got %d", registry.GetTopicCount())
	}
}
