package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/tanmay-xvx/inmem-pubsub/topicManagerService"
	tmhttp "github.com/tanmay-xvx/inmem-pubsub/topicManagerService/http"
)

// Message represents a pub/sub message
type Message struct {
	Topic     string      `json:"topic"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// PubSubServer manages topics and subscribers
type PubSubServer struct {
	topics      map[string][]chan Message
	subscribers map[string][]*websocket.Conn
	mu          sync.RWMutex
}

// NewPubSubServer creates a new Pub/Sub server
func NewPubSubServer() *PubSubServer {
	return &PubSubServer{
		topics:      make(map[string][]chan Message),
		subscribers: make(map[string][]*websocket.Conn),
	}
}

// Publish sends a message to all subscribers of a topic
func (ps *PubSubServer) Publish(topic string, data interface{}) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	msg := Message{
		Topic:     topic,
		Data:      data,
		Timestamp: time.Now(),
	}

	// Send to REST subscribers
	if chans, exists := ps.topics[topic]; exists {
		for _, ch := range chans {
			select {
			case ch <- msg:
			default:
				// Channel is full, skip
			}
		}
	}

	// Send to WebSocket subscribers
	if conns, exists := ps.subscribers[topic]; exists {
		for _, conn := range conns {
			if err := conn.WriteJSON(msg); err != nil {
				log.Printf("Error sending to WebSocket: %v", err)
			}
		}
	}
}

// Subscribe creates a new subscription channel for a topic
func (ps *PubSubServer) Subscribe(topic string) chan Message {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ch := make(chan Message, 100)
	ps.topics[topic] = append(ps.topics[topic], ch)
	return ch
}

// AddWebSocketSubscriber adds a WebSocket connection to a topic
func (ps *PubSubServer) AddWebSocketSubscriber(topic string, conn *websocket.Conn) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.subscribers[topic] = append(ps.subscribers[topic], conn)
}

// RemoveWebSocketSubscriber removes a WebSocket connection from a topic
func (ps *PubSubServer) RemoveWebSocketSubscriber(topic string, conn *websocket.Conn) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if conns, exists := ps.subscribers[topic]; exists {
		for i, c := range conns {
			if c == conn {
				ps.subscribers[topic] = append(conns[:i], conns[i+1:]...)
				break
			}
		}
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for demo
	},
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using defaults")
	}

	// Parse flags
	port := flag.String("port", getEnv("PORT", "8080"), "HTTP server port")
	host := flag.String("host", getEnv("HOST", "0.0.0.0"), "HTTP server host")
	wsPath := flag.String("ws-path", getEnv("WS_PATH", "/ws"), "WebSocket endpoint path")
	flag.Parse()

	// Create Pub/Sub server
	ps := NewPubSubServer()

	// Create router
	r := mux.NewRouter()

	// REST API endpoints
	r.HandleFunc("/publish/{topic}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		topic := vars["topic"]

		var data interface{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		ps.Publish(topic, data)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Message published"))
	}).Methods("POST")

	r.HandleFunc("/subscribe/{topic}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		topic := vars["topic"]

		// Set headers for Server-Sent Events
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Subscribe to topic
		ch := ps.Subscribe(topic)
		defer close(ch)

		// Keep connection alive
		notify := w.(http.CloseNotifier).CloseNotify()
		go func() {
			<-notify
			ch <- Message{}
		}()

		for {
			select {
			case msg := <-ch:
				if msg.Topic == "" {
					return // Connection closed
				}
				data, _ := json.Marshal(msg)
				fmt.Fprintf(w, "data: %s\n\n", data)
				w.(http.Flusher).Flush()
			case <-time.After(30 * time.Second):
				// Send keepalive
				fmt.Fprintf(w, ": keepalive\n\n")
				w.(http.Flusher).Flush()
			}
		}
	}).Methods("GET")

	// WebSocket endpoint
	r.HandleFunc(*wsPath, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		// Handle WebSocket messages
		for {
			var msg Message
			if err := conn.ReadJSON(&msg); err != nil {
				log.Printf("WebSocket read error: %v", err)
				break
			}

			// Subscribe to topic if not already subscribed
			ps.AddWebSocketSubscriber(msg.Topic, conn)
			defer ps.RemoveWebSocketSubscriber(msg.Topic, conn)

			// Publish message
			ps.Publish(msg.Topic, msg.Data)
		}
	})

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Start server
	addr := fmt.Sprintf("%s:%s", *host, *port)
	log.Printf("Starting server on %s", addr)
	log.Printf("WebSocket endpoint: ws://%s%s", addr, *wsPath)
	log.Fatal(http.ListenAndServe(addr, r))
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// RegisterTopicManagerRoutes registers all topic manager HTTP routes with the provided chi router.
// This function mounts the following endpoints:
//   - POST /topics - Create a new topic
//   - DELETE /topics/{name} - Delete a topic by name
//   - GET /topics - List all topics
//   - GET /health - System health check
//   - GET /stats - Topic statistics
//
// The function creates a new HTTP handler with the provided topic manager and
// registers all routes with proper middleware.
func RegisterTopicManagerRoutes(r chi.Router, mgr topicManagerService.TopicManager) {
	// Create a new HTTP handler with the provided topic manager
	handler := tmhttp.NewHandler(mgr)

	// Register all routes with the handler
	handler.RegisterRoutes(r)
}
