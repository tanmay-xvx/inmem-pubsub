package ringbuffer

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/tanmay-xvx/inmem-pubsub/internals/models"
)

func TestNewRingBuffer(t *testing.T) {
	rb := NewRingBuffer(10)
	if rb == nil {
		t.Fatal("NewRingBuffer returned nil")
	}
	if rb.Capacity() != 10 {
		t.Errorf("Expected capacity 10, got %d", rb.Capacity())
	}
	if rb.Size() != 0 {
		t.Errorf("Expected size 0, got %d", rb.Size())
	}
	if !rb.IsEmpty() {
		t.Error("New buffer should be empty")
	}
}

func TestRingBuffer_Push(t *testing.T) {
	rb := NewRingBuffer(3)

	// Test basic push
	msg1 := models.Message{ID: "1", Payload: json.RawMessage(`{"test": "data1"}`)}
	rb.Push(msg1)

	if rb.Size() != 1 {
		t.Errorf("Expected size 1, got %d", rb.Size())
	}

	// Test multiple pushes
	msg2 := models.Message{ID: "2", Payload: json.RawMessage(`{"test": "data2"}`)}
	msg3 := models.Message{ID: "3", Payload: json.RawMessage(`{"test": "data3"}`)}
	rb.Push(msg2)
	rb.Push(msg3)

	if rb.Size() != 3 {
		t.Errorf("Expected size 3, got %d", rb.Size())
	}
	if !rb.IsFull() {
		t.Error("Buffer should be full")
	}

	// Test overwriting (circular behavior)
	msg4 := models.Message{ID: "4", Payload: json.RawMessage(`{"test": "data4"}`)}
	rb.Push(msg4)

	if rb.Size() != 3 {
		t.Errorf("Expected size 3 after overwrite, got %d", rb.Size())
	}
}

func TestRingBuffer_LastN(t *testing.T) {
	rb := NewRingBuffer(5)

	// Add 5 messages
	for i := 1; i <= 5; i++ {
		msg := models.Message{
			ID:      string(rune('0' + i)),
			Payload: json.RawMessage(`{"value": "test"}`),
		}
		rb.Push(msg)
	}

	// Test LastN with various values
	testCases := []struct {
		n        int
		expected int
	}{
		{0, 0},
		{1, 1},
		{3, 3},
		{5, 5},
		{10, 5}, // More than available
		{-1, 0}, // Negative
	}

	for _, tc := range testCases {
		result := rb.LastN(tc.n)
		if len(result) != tc.expected {
			t.Errorf("LastN(%d) expected %d messages, got %d", tc.n, tc.expected, len(result))
		}
	}

	// Test chronological order (oldest to newest)
	result := rb.LastN(5)
	if len(result) != 5 {
		t.Fatalf("Expected 5 messages, got %d", len(result))
	}

	// First message should be oldest (ID "1")
	if result[0].ID != "1" {
		t.Errorf("First message should have ID '1', got '%s'", result[0].ID)
	}

	// Last message should be newest (ID "5")
	if result[4].ID != "5" {
		t.Errorf("Last message should have ID '5', got '%s'", result[4].ID)
	}
}

func TestRingBuffer_ThreadSafety(t *testing.T) {
	rb := NewRingBuffer(1000)
	var wg sync.WaitGroup

	// Start multiple goroutines pushing messages
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				msg := models.Message{
					ID:      fmt.Sprintf("goroutine-%d-msg-%d", id, j),
					Payload: json.RawMessage(`{"goroutine": "test"}`),
				}
				rb.Push(msg)
			}
		}(i)
	}

	// Start multiple goroutines reading
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				rb.LastN(10)
				rb.Size()
				rb.IsEmpty()
				rb.IsFull()
			}
		}(i)
	}

	wg.Wait()

	// Verify final state is consistent
	finalSize := rb.Size()
	if finalSize < 0 || finalSize > rb.Capacity() {
		t.Errorf("Invalid final size: %d", finalSize)
	}
}

func TestRingBuffer_EdgeCases(t *testing.T) {
	// Test zero capacity (should use default)
	rb := NewRingBuffer(0)
	if rb.Capacity() != 100 {
		t.Errorf("Expected default capacity 100, got %d", rb.Capacity())
	}

	// Test negative capacity (should use default)
	rb = NewRingBuffer(-5)
	if rb.Capacity() != 100 {
		t.Errorf("Expected default capacity 100, got %d", rb.Capacity())
	}

	// Test empty buffer operations
	rb = NewRingBuffer(5)
	if !rb.IsEmpty() {
		t.Error("New buffer should be empty")
	}

	result := rb.LastN(10)
	if len(result) != 0 {
		t.Errorf("Empty buffer should return empty slice, got %d items", len(result))
	}
}

func BenchmarkRingBuffer_Push(b *testing.B) {
	rb := NewRingBuffer(1000)
	msg := models.Message{
		ID:      "benchmark",
		Payload: json.RawMessage(`{"benchmark": "test"}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Push(msg)
	}
}

func BenchmarkRingBuffer_LastN(b *testing.B) {
	rb := NewRingBuffer(1000)

	// Pre-populate buffer
	for i := 0; i < 1000; i++ {
		msg := models.Message{
			ID:      fmt.Sprintf("msg-%d", i),
			Payload: json.RawMessage(`{"value": "test"}`),
		}
		rb.Push(msg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.LastN(100)
	}
}
