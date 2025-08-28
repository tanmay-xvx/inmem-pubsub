// Package metrics provides metrics collection and reporting for the Pub/Sub system.
package metrics

import (
	"sync"
	"sync/atomic"
)

// Metrics tracks various metrics for the Pub/Sub system.
type Metrics struct {
	// Global counters
	totalTopics      uint64
	totalSubscribers uint64
	totalMessages    uint64
	totalDropped     uint64

	// Per-topic metrics
	mu     sync.RWMutex
	topics map[string]*TopicMetrics
}

// TopicMetrics tracks metrics for a specific topic.
type TopicMetrics struct {
	Name        string
	Published   uint64
	Delivered   uint64
	Dropped     uint64
	Subscribers uint64
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{
		topics: make(map[string]*TopicMetrics),
	}
}

// IncPublished increments the published message counter for a topic.
func (m *Metrics) IncPublished(topic string) {
	atomic.AddUint64(&m.totalMessages, 1)

	m.mu.Lock()
	if m.topics[topic] == nil {
		m.topics[topic] = &TopicMetrics{Name: topic}
	}
	m.topics[topic].Published++
	m.mu.Unlock()
}

// IncDelivered increments the delivered message counter for a topic.
func (m *Metrics) IncDelivered(topic string, n int) {
	if n <= 0 {
		return
	}

	atomic.AddUint64(&m.totalMessages, uint64(n))

	m.mu.Lock()
	if m.topics[topic] == nil {
		m.topics[topic] = &TopicMetrics{Name: topic}
	}
	m.topics[topic].Delivered += uint64(n)
	m.mu.Unlock()
}

// IncDropped increments the dropped message counter for a topic.
func (m *Metrics) IncDropped(topic string, n int) {
	if n <= 0 {
		return
	}

	atomic.AddUint64(&m.totalDropped, uint64(n))

	m.mu.Lock()
	if m.topics[topic] == nil {
		m.topics[topic] = &TopicMetrics{Name: topic}
	}
	m.topics[topic].Dropped += uint64(n)
	m.mu.Unlock()
}

// IncTopics increments the total topics counter.
func (m *Metrics) IncTopics() {
	atomic.AddUint64(&m.totalTopics, 1)
}

// DecTopics decrements the total topics counter.
func (m *Metrics) DecTopics() {
	atomic.AddUint64(&m.totalTopics, ^uint64(0))
}

// IncSubscribers increments the total subscribers counter.
func (m *Metrics) IncSubscribers() {
	atomic.AddUint64(&m.totalSubscribers, 1)
}

// DecSubscribers decrements the total subscribers counter.
func (m *Metrics) DecSubscribers() {
	atomic.AddUint64(&m.totalSubscribers, ^uint64(0))
}

// UpdateTopicSubscriberCount updates the subscriber count for a specific topic.
func (m *Metrics) UpdateTopicSubscriberCount(topic string, count int) {
	if count < 0 {
		count = 0
	}

	m.mu.Lock()
	if m.topics[topic] == nil {
		m.topics[topic] = &TopicMetrics{Name: topic}
	}
	m.topics[topic].Subscribers = uint64(count)
	m.mu.Unlock()
}

// RemoveTopic removes metrics for a specific topic.
func (m *Metrics) RemoveTopic(topic string) {
	m.mu.Lock()
	delete(m.topics, topic)
	m.mu.Unlock()
}

// Snapshot returns a copy of the current metrics suitable for JSON serialization.
func (m *Metrics) Snapshot() map[string]interface{} {
	snapshot := make(map[string]interface{})

	// Global metrics
	snapshot["global"] = map[string]interface{}{
		"topics":      atomic.LoadUint64(&m.totalTopics),
		"subscribers": atomic.LoadUint64(&m.totalSubscribers),
		"messages":    atomic.LoadUint64(&m.totalMessages),
		"dropped":     atomic.LoadUint64(&m.totalDropped),
	}

	// Per-topic metrics
	m.mu.RLock()
	topics := make(map[string]map[string]interface{})
	for name, tm := range m.topics {
		topics[name] = map[string]interface{}{
			"published":   tm.Published,
			"delivered":   tm.Delivered,
			"dropped":     tm.Dropped,
			"subscribers": tm.Subscribers,
		}
	}
	m.mu.RUnlock()

	snapshot["topics"] = topics
	return snapshot
}

// GetTopicMetrics returns metrics for a specific topic.
func (m *Metrics) GetTopicMetrics(topic string) *TopicMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if tm, exists := m.topics[topic]; exists {
		// Return a copy to avoid race conditions
		return &TopicMetrics{
			Name:        tm.Name,
			Published:   tm.Published,
			Delivered:   tm.Delivered,
			Dropped:     tm.Dropped,
			Subscribers: tm.Subscribers,
		}
	}
	return nil
}

// GetAllTopicMetrics returns all topic metrics.
func (m *Metrics) GetAllTopicMetrics() map[string]*TopicMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*TopicMetrics)
	for name, tm := range m.topics {
		result[name] = &TopicMetrics{
			Name:        tm.Name,
			Published:   tm.Published,
			Delivered:   tm.Delivered,
			Dropped:     tm.Dropped,
			Subscribers: tm.Subscribers,
		}
	}
	return result
}

// Reset resets all metrics to zero.
func (m *Metrics) Reset() {
	atomic.StoreUint64(&m.totalTopics, 0)
	atomic.StoreUint64(&m.totalSubscribers, 0)
	atomic.StoreUint64(&m.totalMessages, 0)
	atomic.StoreUint64(&m.totalDropped, 0)

	m.mu.Lock()
	for _, tm := range m.topics {
		tm.Published = 0
		tm.Delivered = 0
		tm.Dropped = 0
		tm.Subscribers = 0
	}
	m.mu.Unlock()
}
