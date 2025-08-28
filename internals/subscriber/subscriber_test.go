package subscriber

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tanmay-xvx/inmem-pubsub/internals/models"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func createTestWebSocket() (*websocket.Conn, func()) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, "WebSocket upgrade failed", http.StatusInternalServerError)
			return
		}
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

func TestNewSubscriber(t *testing.T) {
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	sub := NewSubscriber("test-client", conn, 50)
	if sub == nil {
		t.Fatal("NewSubscriber returned nil")
	}

	if sub.ClientID != "test-client" {
		t.Errorf("Expected ClientID 'test-client', got '%s'", sub.ClientID)
	}

	if sub.Conn != conn {
		t.Error("WebSocket connection not set correctly")
	}

	if cap(sub.Send) != 50 {
		t.Errorf("Expected Send channel capacity 50, got %d", cap(sub.Send))
	}

	if sub.Done == nil {
		t.Error("Done channel not initialized")
	}
}

func TestNewSubscriber_DefaultBuffer(t *testing.T) {
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	// Test with zero buffer size
	sub := NewSubscriber("test-client", conn, 0)
	if cap(sub.Send) != 100 {
		t.Errorf("Expected default buffer size 100, got %d", cap(sub.Send))
	}

	// Test with negative buffer size
	sub = NewSubscriber("test-client", conn, -5)
	if cap(sub.Send) != 100 {
		t.Errorf("Expected default buffer size 100, got %d", cap(sub.Send))
	}
}

func TestSubscriber_StartWriter(t *testing.T) {
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	sub := NewSubscriber("test-client", conn, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start writer with short timeout
	sub.StartWriter(ctx, 100*time.Millisecond)

	// Give writer time to start
	time.Sleep(10 * time.Millisecond)

	// Send a message
	msg := models.NewServerMsg("test", "req-1")
	success := sub.SendMessage(*msg)
	if !success {
		t.Error("Failed to send message")
	}

	// Wait for message to be processed
	time.Sleep(50 * time.Millisecond)

	// Check if subscriber is still active
	if !sub.IsActive() {
		t.Error("Subscriber should still be active")
	}
}

func TestSubscriber_StartWriter_ContextCancellation(t *testing.T) {
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	sub := NewSubscriber("test-client", conn, 10)
	ctx, cancel := context.WithCancel(context.Background())

	sub.StartWriter(ctx, 100*time.Millisecond)

	// Give writer time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for writer to finish
	time.Sleep(50 * time.Millisecond)

	// Check if subscriber is no longer active
	if sub.IsActive() {
		t.Error("Subscriber should not be active after context cancellation")
	}
}

func TestSubscriber_Close(t *testing.T) {
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	sub := NewSubscriber("test-client", conn, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub.StartWriter(ctx, 100*time.Millisecond)

	// Give writer time to start
	time.Sleep(10 * time.Millisecond)

	// Close subscriber
	sub.Close()

	// Wait for cleanup
	time.Sleep(50 * time.Millisecond)

	// Check if subscriber is no longer active
	if sub.IsActive() {
		t.Error("Subscriber should not be active after closing")
	}

	// Test that Close is idempotent
	sub.Close()
}

func TestSubscriber_SendMessage(t *testing.T) {
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	sub := NewSubscriber("test-client", conn, 2)

	// Send messages up to buffer capacity
	msg1 := models.NewServerMsg("test1", "req-1")
	msg2 := models.NewServerMsg("test2", "req-2")

	if !sub.SendMessage(*msg1) {
		t.Error("Failed to send first message")
	}

	if !sub.SendMessage(*msg2) {
		t.Error("Failed to send second message")
	}

	// Buffer should be full now
	if sub.SendMessage(*models.NewServerMsg("test3", "req-3")) {
		t.Error("Should not be able to send message when buffer is full")
	}
}

func TestSubscriber_Concurrency(t *testing.T) {
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	sub := NewSubscriber("test-client", conn, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub.StartWriter(ctx, 100*time.Millisecond)

	// Give writer time to start
	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup

	// Start multiple goroutines sending messages
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				msg := models.NewServerMsg("concurrent", "req-concurrent")
				sub.SendMessage(*msg)
			}
		}(i)
	}

	wg.Wait()

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)

	// Check if subscriber is still active
	if !sub.IsActive() {
		t.Error("Subscriber should still be active after concurrent sends")
	}
}

func TestSubscriber_GetClientID(t *testing.T) {
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	sub := NewSubscriber("test-client-123", conn, 10)

	clientID := sub.GetClientID()
	if clientID != "test-client-123" {
		t.Errorf("Expected ClientID 'test-client-123', got '%s'", clientID)
	}
}

func BenchmarkSubscriber_SendMessage(b *testing.B) {
	conn, cleanup := createTestWebSocket()
	defer cleanup()

	sub := NewSubscriber("benchmark-client", conn, 1000)
	msg := models.NewServerMsg("benchmark", "req-bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sub.SendMessage(*msg)
	}
}
