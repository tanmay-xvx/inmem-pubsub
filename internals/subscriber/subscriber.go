// Package subscriber provides WebSocket subscriber management for the Pub/Sub system.
package subscriber

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tanmay-xvx/inmem-pubsub/internals/models"
)

// Subscriber represents a WebSocket client connection with message handling capabilities.
// It ensures thread-safe message delivery through a dedicated writer goroutine.
type Subscriber struct {
	ClientID  string
	Conn      *websocket.Conn
	Send      chan models.ServerMsg
	Done      chan struct{}
	closeOnce sync.Once
}

// NewSubscriber creates a new subscriber with the specified client ID and WebSocket connection.
// The buf parameter sets the buffer size for the Send channel.
func NewSubscriber(clientID string, conn *websocket.Conn, buf int) *Subscriber {
	if buf <= 0 {
		buf = 100 // Default buffer size
	}

	return &Subscriber{
		ClientID: clientID,
		Conn:     conn,
		Send:     make(chan models.ServerMsg, buf),
		Done:     make(chan struct{}),
	}
}

// StartWriter launches a goroutine that continuously reads from the Send channel
// and writes messages as JSON to the WebSocket connection.
//
// CONCURRENCY NOTE: This is the ONLY goroutine that should write to the Conn.
// All other code should send messages through the Send channel, never directly
// to the WebSocket connection.
//
// The writer will automatically close the Done channel on any write error,
// signaling that the subscriber should be cleaned up.
func (s *Subscriber) StartWriter(ctx context.Context, writeTimeout time.Duration) {
	go func() {
		defer func() {
			// Signal completion by closing Done channel
			close(s.Done)
		}()

		for {
			select {
			case <-ctx.Done():
				log.Printf("Subscriber %s: context cancelled", s.ClientID)
				return

			case msg, ok := <-s.Send:
				if !ok {
					// Send channel closed, exit
					log.Printf("Subscriber %s: send channel closed", s.ClientID)
					return
				}

				// Set write deadline
				if writeTimeout > 0 {
					if err := s.Conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
						log.Printf("Subscriber %s: failed to set write deadline: %v", s.ClientID, err)
						return
					}
				}

				// Write message as JSON to WebSocket connection
				if err := s.Conn.WriteJSON(msg); err != nil {
					log.Printf("Subscriber %s: failed to write message: %v", s.ClientID, err)
					return
				}
			}
		}
	}()
}

// Close gracefully shuts down the subscriber by closing the Send channel
// and waiting for the writer goroutine to complete.
// This method is safe to call multiple times due to sync.Once.
func (s *Subscriber) Close() {
	s.closeOnce.Do(func() {
		// Close Send channel to signal writer to stop
		close(s.Send)

		// Wait for writer to complete
		<-s.Done

		// Close WebSocket connection
		if s.Conn != nil {
			s.Conn.Close()
		}

		log.Printf("Subscriber %s: closed", s.ClientID)
	})
}

// SendMessage sends a message to the subscriber through the Send channel.
// This is the thread-safe way to send messages to the subscriber.
// Returns false if the Send channel is full or closed.
func (s *Subscriber) SendMessage(msg models.ServerMsg) bool {
	select {
	case s.Send <- msg:
		return true
	default:
		// Channel is full or closed
		return false
	}
}

// IsActive returns true if the subscriber is still active and can receive messages.
func (s *Subscriber) IsActive() bool {
	select {
	case <-s.Done:
		return false
	default:
		return true
	}
}

// GetClientID returns the subscriber's client identifier.
func (s *Subscriber) GetClientID() string {
	return s.ClientID
}
