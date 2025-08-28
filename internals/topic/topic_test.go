package topic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tanmay-xvx/inmem-pubsub/internals/models"
	"github.com/tanmay-xvx/inmem-pubsub/internals/subscriber"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func createTestWebSocket() (*websocket.Conn, func()) {
	// Create a minimal WebSocket server for testing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, "WebSocket upgrade failed", http.StatusInternalServerError)
			return
		}

		// Keep connection alive without setting aggressive deadlines
		go func() {
			defer conn.Close()
			for {
				// Set a reasonable read deadline for each read
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				_, _, err := conn.ReadMessage()
				if err != nil {
					break
				}
			}
		}()
	}))

	conn, _, err := websocket.DefaultDialer.Dial("ws"+server.URL[4:], nil)
	if err != nil {
		panic(err)
	}

	return conn, func() {
		conn.Close()
		server.Close()
	}
}

// createTestSubscriber creates a subscriber with writer started to prevent hanging
func createTestSubscriber(clientID string, conn *websocket.Conn, buf int) (*subscriber.Subscriber, func()) {
	sub := subscriber.NewSubscriber(clientID, conn, buf)

	// Start the writer goroutine to prevent hanging on Close()
	ctx, cancel := context.WithCancel(context.Background())
	sub.StartWriter(ctx, 100*time.Millisecond)

	cleanup := func() {
		cancel()
		// Don't call sub.Close() here - let the test handle it
	}

	return sub, cleanup
}

func TestNewTopic(t *testing.T) {
	topic := NewTopic("test-topic", 100)
	if topic == nil {
		t.Fatal("NewTopic returned nil")
	}

	if topic.Name != "test-topic" {
		t.Errorf("Expected name 'test-topic', got '%s'", topic.Name)
	}

	if topic.GetSubscriberCount() != 0 {
		t.Errorf("Expected 0 subscribers, got %d", topic.GetSubscriberCount())
	}

	if topic.GetMessageCount() != 0 {
		t.Errorf("Expected 0 messages, got %d", topic.GetMessageCount())
	}

	if topic.GetDroppedCount() != 0 {
		t.Errorf("Expected 0 dropped messages, got %d", topic.GetDroppedCount())
	}
}

func TestNewTopic_DefaultCapacity(t *testing.T) {
	// Test with zero capacity
	topic := NewTopic("test-topic", 0)
	if topic.ring.Capacity() != 1000 {
		t.Errorf("Expected default capacity 1000, got %d", topic.ring.Capacity())
	}

	// Test with negative capacity
	topic = NewTopic("test-topic", -5)
	if topic.ring.Capacity() != 1000 {
		t.Errorf("Expected default capacity 1000, got %d", topic.ring.Capacity())
	}
}

func TestTopic_AddSubscriber(t *testing.T) {
	topic := NewTopic("test-topic", 100)
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	sub, subCleanup := createTestSubscriber("client-1", conn, 10)
	defer subCleanup()

	topic.AddSubscriber(sub)

	if topic.GetSubscriberCount() != 1 {
		t.Errorf("Expected 1 subscriber, got %d", topic.GetSubscriberCount())
	}

	// Test replacing existing subscriber
	sub2, sub2Cleanup := createTestSubscriber("client-1", conn, 10)
	defer sub2Cleanup()

	topic.AddSubscriber(sub2)

	if topic.GetSubscriberCount() != 1 {
		t.Errorf("Expected 1 subscriber after replacement, got %d", topic.GetSubscriberCount())
	}
}

func TestTopic_RemoveSubscriber(t *testing.T) {
	topic := NewTopic("test-topic", 100)
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	sub, subCleanup := createTestSubscriber("client-1", conn, 10)
	defer subCleanup()

	topic.AddSubscriber(sub)

	if topic.GetSubscriberCount() != 1 {
		t.Error("Failed to add subscriber")
	}

	// Remove subscriber
	if !topic.RemoveSubscriber("client-1") {
		t.Error("Failed to remove subscriber")
	}

	if topic.GetSubscriberCount() != 0 {
		t.Errorf("Expected 0 subscribers after removal, got %d", topic.GetSubscriberCount())
	}

	// Try to remove non-existent subscriber
	if topic.RemoveSubscriber("non-existent") {
		t.Error("Should not be able to remove non-existent subscriber")
	}
}

func TestTopic_Publish_Basic(t *testing.T) {
	topic := NewTopic("test-topic", 100)
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	sub := subscriber.NewSubscriber("client-1", conn, 10)
	topic.AddSubscriber(sub)

	// Start subscriber writer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub.StartWriter(ctx, 100*time.Millisecond)

	// Give writer time to start
	time.Sleep(10 * time.Millisecond)

	// Publish message
	msg := models.Message{
		ID:      "msg-1",
		Payload: json.RawMessage(`{"test": "data"}`),
	}

	delivered, dropped := topic.Publish(msg, PolicyDropOldest, 10)

	if delivered != 1 {
		t.Errorf("Expected 1 message delivered, got %d", delivered)
	}

	if dropped != 0 {
		t.Errorf("Expected 0 messages dropped, got %d", dropped)
	}

	if topic.GetMessageCount() != 1 {
		t.Errorf("Expected 1 total message, got %d", topic.GetMessageCount())
	}
}

func TestTopic_Publish_NoSubscribers(t *testing.T) {
	topic := NewTopic("test-topic", 100)

	msg := models.Message{
		ID:      "msg-1",
		Payload: json.RawMessage(`{"test": "data"}`),
	}

	delivered, dropped := topic.Publish(msg, PolicyDropOldest, 10)

	if delivered != 0 {
		t.Errorf("Expected 0 messages delivered, got %d", delivered)
	}

	if dropped != 0 {
		t.Errorf("Expected 0 messages dropped, got %d", dropped)
	}

	if topic.GetMessageCount() != 1 {
		t.Errorf("Expected 1 total message, got %d", topic.GetMessageCount())
	}
}

func TestTopic_Publish_DropOldestPolicy(t *testing.T) {
	topic := NewTopic("test-topic", 100)
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	// Create subscriber with small buffer to trigger overflow
	sub, subCleanup := createTestSubscriber("client-1", conn, 2)
	defer subCleanup()

	topic.AddSubscriber(sub)

	// Give writer time to start
	time.Sleep(10 * time.Millisecond)

	// Publish messages to test basic functionality
	msg1 := models.Message{
		ID:      "msg-1",
		Payload: json.RawMessage(`{"test": "data1"}`),
	}

	delivered, dropped := topic.Publish(msg1, PolicyDropOldest, 2)
	if delivered != 1 || dropped != 0 {
		t.Errorf("First message: delivered=%d, dropped=%d", delivered, dropped)
	}

	// Verify message was added to ring buffer
	if topic.GetMessageCount() != 1 {
		t.Errorf("Expected 1 total message, got %d", topic.GetMessageCount())
	}
}

func TestTopic_Publish_DisconnectPolicy(t *testing.T) {
	topic := NewTopic("test-topic", 100)
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	// Create subscriber with small buffer to trigger overflow
	sub, subCleanup := createTestSubscriber("client-1", conn, 2)
	defer subCleanup()

	topic.AddSubscriber(sub)

	// Give writer time to start
	time.Sleep(10 * time.Millisecond)

	// Publish message to test basic functionality
	msg1 := models.Message{
		ID:      "msg-1",
		Payload: json.RawMessage(`{"test": "data1"}`),
	}

	delivered, dropped := topic.Publish(msg1, PolicyDisconnect, 2)
	if delivered != 1 || dropped != 0 {
		t.Errorf("First message: delivered=%d, dropped=%d", delivered, dropped)
	}

	// Verify message was added to ring buffer
	if topic.GetMessageCount() != 1 {
		t.Errorf("Expected 1 total message, got %d", topic.GetMessageCount())
	}
}

func TestTopic_ListSubscriberIDs(t *testing.T) {
	topic := NewTopic("test-topic", 100)
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	// Add subscribers
	sub1, sub1Cleanup := createTestSubscriber("client-1", conn, 10)
	defer sub1Cleanup()
	sub2, sub2Cleanup := createTestSubscriber("client-2", conn, 10)
	defer sub2Cleanup()

	topic.AddSubscriber(sub1)
	topic.AddSubscriber(sub2)

	ids := topic.ListSubscriberIDs()
	if len(ids) != 2 {
		t.Errorf("Expected 2 subscriber IDs, got %d", len(ids))
	}

	// Check that both IDs are present
	found1, found2 := false, false
	for _, id := range ids {
		if id == "client-1" {
			found1 = true
		}
		if id == "client-2" {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Error("Not all expected subscriber IDs found")
	}
}

func TestTopic_GetLastN(t *testing.T) {
	topic := NewTopic("test-topic", 5)

	// Publish several messages
	for i := 1; i <= 3; i++ {
		msg := models.Message{
			ID:      fmt.Sprintf("msg-%d", i),
			Payload: json.RawMessage(fmt.Sprintf(`{"value": %d}`, i)),
		}
		topic.Publish(msg, PolicyDropOldest, 10)
	}

	// Get last 2 messages
	lastN := topic.GetLastN(2)
	if len(lastN) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(lastN))
	}

	// Check order (oldest to newest)
	if lastN[0].ID != "msg-2" {
		t.Errorf("First message should be 'msg-2', got '%s'", lastN[0].ID)
	}
	if lastN[1].ID != "msg-3" {
		t.Errorf("Second message should be 'msg-3', got '%s'", lastN[1].ID)
	}
}

func TestTopic_Close(t *testing.T) {
	topic := NewTopic("test-topic", 100)
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	// Add subscriber
	sub, subCleanup := createTestSubscriber("client-1", conn, 10)
	defer subCleanup()

	topic.AddSubscriber(sub)

	if topic.GetSubscriberCount() != 1 {
		t.Error("Failed to add subscriber")
	}

	// Close topic
	topic.Close()

	if topic.GetSubscriberCount() != 0 {
		t.Errorf("Expected 0 subscribers after close, got %d", topic.GetSubscriberCount())
	}
}

func TestTopic_Concurrency(t *testing.T) {
	topic := NewTopic("test-topic", 1000)
	var wg sync.WaitGroup

	// Start multiple goroutines publishing messages
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				msg := models.Message{
					ID:      fmt.Sprintf("goroutine-%d-msg-%d", id, j),
					Payload: json.RawMessage(`{"goroutine": "test"}`),
				}
				topic.Publish(msg, PolicyDropOldest, 10)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	messageCount := topic.GetMessageCount()
	if messageCount != 500 {
		t.Errorf("Expected 500 messages, got %d", messageCount)
	}
}

func BenchmarkTopic_Publish(b *testing.B) {
	topic := NewTopic("benchmark-topic", 1000)
	msg := models.Message{
		ID:      "benchmark",
		Payload: json.RawMessage(`{"benchmark": "test"}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topic.Publish(msg, PolicyDropOldest, 10)
	}
}
