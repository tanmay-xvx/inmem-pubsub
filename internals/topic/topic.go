// Package topic provides topic management for the Pub/Sub system.
package topic

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/tanmay-xvx/inmem-pubsub/internals/models"
	"github.com/tanmay-xvx/inmem-pubsub/internals/ringbuffer"
	"github.com/tanmay-xvx/inmem-pubsub/internals/subscriber"
)

const (
	// PolicyDropOldest drops the oldest message when subscriber buffer is full
	PolicyDropOldest = "DROP_OLDEST"
	// PolicyDisconnect disconnects the subscriber when buffer is full
	PolicyDisconnect = "DISCONNECT"
)

// Topic represents a named channel for publishing and subscribing to messages.
// It manages subscribers and maintains a ring buffer of recent messages.
type Topic struct {
	Name     string
	subs     map[string]*subscriber.Subscriber
	subsMu   sync.RWMutex
	ring     *ringbuffer.RingBuffer
	messages uint64 // atomic counter for total messages published
	dropped  uint64 // atomic counter for dropped messages
}

// NewTopic creates a new topic with the specified name and ring buffer capacity.
func NewTopic(name string, ringCap int) *Topic {
	if ringCap <= 0 {
		ringCap = 1000 // Default ring buffer capacity
	}

	return &Topic{
		Name: name,
		subs: make(map[string]*subscriber.Subscriber),
		ring: ringbuffer.NewRingBuffer(ringCap),
	}
}

// AddSubscriber adds a subscriber to the topic.
// If a subscriber with the same ClientID already exists, it will be replaced.
func (t *Topic) AddSubscriber(s *subscriber.Subscriber) {
	if s == nil {
		return
	}

	t.subsMu.Lock()
	defer t.subsMu.Unlock()

	// If subscriber already exists, close it first
	if existing, exists := t.subs[s.ClientID]; exists {
		existing.Close()
	}

	t.subs[s.ClientID] = s
}

// RemoveSubscriber removes a subscriber from the topic by client ID.
// Returns true if the subscriber was found and removed.
func (t *Topic) RemoveSubscriber(clientID string) bool {
	t.subsMu.Lock()
	defer t.subsMu.Unlock()

	if sub, exists := t.subs[clientID]; exists {
		sub.Close()
		delete(t.subs, clientID)
		return true
	}

	return false
}

// Publish publishes a message to the topic and delivers it to all subscribers.
// The policy parameter determines how to handle subscriber buffer overflow.
// Returns the number of messages delivered and dropped for metrics.
func (t *Topic) Publish(msg models.Message, policy string, wsBuf int) (delivered int, dropped int) {
	// Push message to ring buffer
	t.ring.Push(msg)

	// Atomically increment message counter
	atomic.AddUint64(&t.messages, 1)

	// Get all active subscribers
	t.subsMu.RLock()
	subscribers := make([]*subscriber.Subscriber, 0, len(t.subs))
	for _, sub := range t.subs {
		if sub.IsActive() {
			subscribers = append(subscribers, sub)
		}
	}
	t.subsMu.RUnlock()

	// Deliver message to all subscribers
	for _, sub := range subscribers {
		if t.deliverToSubscriber(sub, msg, policy, wsBuf) {
			delivered++
		} else {
			dropped++
		}
	}

	return delivered, dropped
}

// deliverToSubscriber attempts to deliver a message to a single subscriber.
// Returns true if message was delivered, false if it was dropped.
func (t *Topic) deliverToSubscriber(sub *subscriber.Subscriber, msg models.Message, policy string, wsBuf int) bool {
	// Convert Message to ServerMsg
	serverMsg := models.ServerMsg{
		Type:    "message",
		Topic:   t.Name,
		Message: &msg,
		Ts:      time.Now(),
	}

	// Try to send message non-blocking
	select {
	case sub.Send <- serverMsg:
		return true
	default:
		// Buffer is full, handle according to policy
		return t.handleBufferOverflow(sub, msg, policy, wsBuf)
	}
}

// handleBufferOverflow handles subscriber buffer overflow according to the specified policy.
// Returns true if message was delivered, false if it was dropped.
func (t *Topic) handleBufferOverflow(sub *subscriber.Subscriber, msg models.Message, policy string, wsBuf int) bool {
	switch policy {
	case PolicyDropOldest:
		return t.dropOldestAndSend(sub, msg)

	case PolicyDisconnect:
		return t.disconnectAndSend(sub, msg)

	default:
		// Default to drop oldest
		return t.dropOldestAndSend(sub, msg)
	}
}

// dropOldestAndSend implements DROP_OLDEST policy.
// Drains one message from subscriber buffer and sends the new message.
func (t *Topic) dropOldestAndSend(sub *subscriber.Subscriber, msg models.Message) bool {
	// Try to drain one message from the buffer (non-blocking)
	select {
	case <-sub.Send:
		// Successfully drained one message, now try to send the new one
		// Convert Message to ServerMsg
		serverMsg := models.ServerMsg{
			Type:    "message",
			Topic:   t.Name,
			Message: &msg,
			Ts:      time.Now(),
		}
		select {
		case sub.Send <- serverMsg:
			atomic.AddUint64(&t.dropped, 1)
			return true
		default:
			// Still can't send, increment dropped counter
			atomic.AddUint64(&t.dropped, 1)
			return false
		}
	default:
		// Can't drain, increment dropped counter
		atomic.AddUint64(&t.dropped, 1)
		return false
	}
}

// disconnectAndSend implements DISCONNECT policy.
// Sends an error message and closes the subscriber.
func (t *Topic) disconnectAndSend(sub *subscriber.Subscriber, msg models.Message) bool {
	// Try to send error message before disconnecting
	errorMsg := models.NewServerError("", "BUFFER_OVERFLOW", "Subscriber buffer overflow, disconnecting")
	select {
	case sub.Send <- *errorMsg:
		// Error message sent successfully
	default:
		// Can't even send error message
	}

	// Close the subscriber
	sub.Close()

	// Remove from topic
	t.RemoveSubscriber(sub.GetClientID())

	atomic.AddUint64(&t.dropped, 1)
	return false
}

// ListSubscriberIDs returns a slice of all active subscriber client IDs.
func (t *Topic) ListSubscriberIDs() []string {
	t.subsMu.RLock()
	defer t.subsMu.RUnlock()

	ids := make([]string, 0, len(t.subs))
	for clientID, sub := range t.subs {
		if sub.IsActive() {
			ids = append(ids, clientID)
		}
	}

	return ids
}

// GetSubscriberCount returns the number of active subscribers.
func (t *Topic) GetSubscriberCount() int {
	t.subsMu.RLock()
	defer t.subsMu.RUnlock()

	count := 0
	for _, sub := range t.subs {
		if sub.IsActive() {
			count++
		}
	}

	return count
}

// GetMessageCount returns the total number of messages published to this topic.
func (t *Topic) GetMessageCount() uint64 {
	return atomic.LoadUint64(&t.messages)
}

// GetDroppedCount returns the total number of messages dropped due to buffer overflow.
func (t *Topic) GetDroppedCount() uint64 {
	return atomic.LoadUint64(&t.dropped)
}

// GetLastN returns the last n messages from the ring buffer.
func (t *Topic) GetLastN(n int) []models.Message {
	return t.ring.LastN(n)
}

// GetRingBufferSize returns the capacity of the ring buffer.
func (t *Topic) GetRingBufferSize() int {
	return t.ring.Capacity()
}

// GetSubscriber returns a subscriber by client ID.
func (t *Topic) GetSubscriber(clientID string) (*subscriber.Subscriber, bool) {
	t.subsMu.RLock()
	defer t.subsMu.RUnlock()

	sub, exists := t.subs[clientID]
	return sub, exists
}

// Close closes all subscribers and cleans up the topic.
func (t *Topic) Close() {
	t.subsMu.Lock()
	defer t.subsMu.Unlock()

	for _, sub := range t.subs {
		sub.Close()
	}

	// Clear the subscribers map
	t.subs = make(map[string]*subscriber.Subscriber)
}
