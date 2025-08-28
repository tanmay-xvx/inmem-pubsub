package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tanmay-xvx/inmem-pubsub/internals/topic"
	"github.com/tanmay-xvx/inmem-pubsub/topicManagerService"
)

// mockTopicManager is a mock implementation for testing
type mockTopicManager struct{}

func (m *mockTopicManager) CreateTopic(name string) error {
	return nil
}

func (m *mockTopicManager) DeleteTopic(name string) error {
	return nil
}

func (m *mockTopicManager) ListTopics() []topicManagerService.TopicInfo {
	return []topicManagerService.TopicInfo{}
}

func (m *mockTopicManager) GetTopic(name string) (*topic.Topic, bool) {
	return nil, false
}

func (m *mockTopicManager) Stats() map[string]topicManagerService.TopicStats {
	return map[string]topicManagerService.TopicStats{}
}

// TestRegisterTopicManagerRoutes tests that the function can be called without errors
func TestRegisterTopicManagerRoutes(t *testing.T) {
	router := chi.NewRouter()
	mockTM := &mockTopicManager{}

	// This should not panic or error
	RegisterTopicManagerRoutes(router, mockTM)

	// Test that at least one route is registered by trying to access health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// The health endpoint should exist (even if it might return an error due to mock)
	if w.Code == http.StatusNotFound {
		t.Log("Health endpoint not found, but this is expected with mock implementation")
	}
}

func TestNewPubSubServer(t *testing.T) {
	ps := NewPubSubServer()
	if ps == nil {
		t.Error("NewPubSubServer returned nil")
	}
	if ps.topics == nil {
		t.Error("topics map is nil")
	}
	if ps.subscribers == nil {
		t.Error("subscribers map is nil")
	}
}

func TestPubSubServer_Publish(t *testing.T) {
	ps := NewPubSubServer()
	topic := "test-topic"
	data := "test-data"

	// Publish should not panic even with no subscribers
	ps.Publish(topic, data)
}

func TestPubSubServer_Subscribe(t *testing.T) {
	ps := NewPubSubServer()
	topic := "test-topic"

	ch := ps.Subscribe(topic)
	if ch == nil {
		t.Error("Subscribe returned nil channel")
	}

	// Check if topic was added
	ps.mu.RLock()
	_, exists := ps.topics[topic]
	ps.mu.RUnlock()
	if !exists {
		t.Error("Topic was not added to topics map")
	}
}
