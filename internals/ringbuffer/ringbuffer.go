// Package ringbuffer provides a thread-safe circular buffer for storing messages.
package ringbuffer

import (
	"sync"

	"github.com/tanmay-xvx/inmem-pubsub/internals/models"
)

// RingBuffer implements a thread-safe circular buffer for storing messages.
// It maintains a fixed capacity and overwrites oldest messages when full.
type RingBuffer struct {
	buf  []models.Message
	cap  int
	head int
	size int
	mu   sync.RWMutex
}

// NewRingBuffer creates a new ring buffer with the specified capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 100 // Default capacity
	}
	return &RingBuffer{
		buf:  make([]models.Message, capacity),
		cap:  capacity,
		head: 0,
		size: 0,
	}
}

// Push adds a new message to the ring buffer.
// If the buffer is full, it overwrites the oldest message.
func (r *RingBuffer) Push(m models.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Add message at current head position
	r.buf[r.head] = m

	// Move head to next position (circular)
	r.head = (r.head + 1) % r.cap

	// Update size (don't exceed capacity)
	if r.size < r.cap {
		r.size++
	}
}

// LastN returns the last n messages in chronological order (oldest to newest).
// If n is greater than the number of messages stored, returns all available messages.
// If n is 0 or negative, returns an empty slice.
func (r *RingBuffer) LastN(n int) []models.Message {
	if n <= 0 {
		return []models.Message{}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Determine how many messages to return
	count := n
	if count > r.size {
		count = r.size
	}

	if count == 0 {
		return []models.Message{}
	}

	// Create result slice
	result := make([]models.Message, count)

	// Calculate starting position (oldest message)
	start := (r.head - count + r.cap) % r.cap

	// Copy messages in chronological order
	for i := 0; i < count; i++ {
		pos := (start + i) % r.cap
		result[i] = r.buf[pos]
	}

	return result
}

// Size returns the current number of messages in the buffer.
func (r *RingBuffer) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size
}

// Capacity returns the maximum capacity of the buffer.
func (r *RingBuffer) Capacity() int {
	return r.cap
}

// IsEmpty returns true if the buffer contains no messages.
func (r *RingBuffer) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size == 0
}

// IsFull returns true if the buffer is at maximum capacity.
func (r *RingBuffer) IsFull() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size == r.cap
}
