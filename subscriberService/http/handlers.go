// Package http provides HTTP handlers for the subscriber service.
package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tanmay-xvx/inmem-pubsub/internals/models"
	"github.com/tanmay-xvx/inmem-pubsub/internals/subscriber"
	"github.com/tanmay-xvx/inmem-pubsub/topicManagerService"
)

const (
	// WebSocket message types
	MsgTypeSubscribe   = "subscribe"
	MsgTypeUnsubscribe = "unsubscribe"
	MsgTypePublish     = "publish"
	MsgTypePing        = "ping"
	MsgTypePong        = "pong"
	MsgTypeAck         = "ack"
	MsgTypeError       = "error"
)

// WebSocketHandler manages WebSocket connections and handles client messages.
type WebSocketHandler struct {
	topicManager topicManagerService.TopicManager
	upgrader     websocket.Upgrader

	// Connection management
	connsMu sync.RWMutex
	conns   map[*websocket.Conn]*connectionInfo
}

// connectionInfo tracks information about a WebSocket connection
type connectionInfo struct {
	clientID    string
	subscribers map[string]*subscriber.Subscriber // topic -> subscriber
	writeChan   chan models.ServerMsg             // unified write channel for all messages
	mu          sync.RWMutex
}

// NewWebSocketHandler creates a new WebSocket handler with the specified topic manager.
func NewWebSocketHandler(topicManager topicManagerService.TopicManager) *WebSocketHandler {
	return &WebSocketHandler{
		topicManager: topicManager,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
		conns: make(map[*websocket.Conn]*connectionInfo),
	}
}

// HandleWebSocket upgrades the HTTP request to WebSocket and handles the connection.
func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer h.cleanupConnection(conn)

	// Create connection info
	connInfo := &connectionInfo{
		clientID:    generateClientID(),
		subscribers: make(map[string]*subscriber.Subscriber),
		writeChan:   make(chan models.ServerMsg, 100),
	}

	// Start single writer goroutine for this connection
	go h.unifiedWriter(conn, connInfo.writeChan)

	// Register connection
	h.connsMu.Lock()
	h.conns[conn] = connInfo
	h.connsMu.Unlock()

	log.Printf("WebSocket connection established for client %s", connInfo.clientID)

	// Send welcome message through unified channel
	welcomeMsg := models.ServerMsg{
		Type: "connected",
		Message: &models.Message{
			ID:      "welcome",
			Payload: json.RawMessage(fmt.Sprintf(`{"client_id": "%s"}`, connInfo.clientID)),
		},
		Ts: time.Now(),
	}
	connInfo.writeChan <- welcomeMsg

	// Start message reader
	h.handleMessages(conn, connInfo)
}

// unifiedWriter is the single goroutine responsible for writing all messages to a WebSocket connection.
// This prevents concurrent write race conditions by ensuring only one writer.
func (h *WebSocketHandler) unifiedWriter(conn *websocket.Conn, writeChan <-chan models.ServerMsg) {
	for msg := range writeChan {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("Failed to write message to WebSocket: %v", err)
			break
		}
	}
}

// handleMessages reads and processes incoming WebSocket messages.
func (h *WebSocketHandler) handleMessages(conn *websocket.Conn, connInfo *connectionInfo) {
	for {
		// Read message
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error for client %s: %v", connInfo.clientID, err)
			}
			break
		}

		// Parse client message
		var clientMsg models.WSClientMsg
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			h.sendError(conn, "INVALID_JSON", "Invalid JSON message")
			continue
		}

		// Handle message based on type
		switch clientMsg.Type {
		case MsgTypeSubscribe:
			h.handleSubscribe(conn, connInfo, &clientMsg)
		case MsgTypeUnsubscribe:
			h.handleUnsubscribe(conn, connInfo, &clientMsg)
		case MsgTypePublish:
			h.handlePublish(conn, connInfo, &clientMsg)
		case MsgTypePing:
			h.handlePing(conn, &clientMsg)
		default:
			h.sendError(conn, "UNKNOWN_TYPE", fmt.Sprintf("Unknown message type: %s", clientMsg.Type))
		}
	}
}

// handleSubscribe handles subscription requests.
func (h *WebSocketHandler) handleSubscribe(conn *websocket.Conn, connInfo *connectionInfo, msg *models.WSClientMsg) {
	if msg.Topic == "" {
		h.sendError(conn, "MISSING_TOPIC", "Topic is required for subscription")
		return
	}

	// Check if topic exists
	topic, exists := h.topicManager.GetTopic(msg.Topic)
	if !exists {
		h.sendError(conn, "TOPIC_NOT_FOUND", fmt.Sprintf("Topic '%s' not found", msg.Topic))
		return
	}

	// Check if already subscribed
	connInfo.mu.RLock()
	if _, alreadySubscribed := connInfo.subscribers[msg.Topic]; alreadySubscribed {
		connInfo.mu.RUnlock()
		h.sendAck(conn, msg.RequestID, "Already subscribed to topic")
		return
	}
	connInfo.mu.RUnlock()

	// Create subscriber and forward its messages to unified write channel
	sub := subscriber.NewSubscriber(connInfo.clientID, nil, 100) // No direct WebSocket connection

	// Start a goroutine that forwards messages from subscriber to unified write channel
	// and properly closes the Done channel when finished
	go func() {
		defer close(sub.Done) // Ensure Done channel is closed when this goroutine exits
		for msg := range sub.Send {
			select {
			case connInfo.writeChan <- msg:
				// Message forwarded successfully
			default:
				log.Printf("Warning: write channel full for client %s", connInfo.clientID)
			}
		}
	}()

	// Add subscriber to topic
	topic.AddSubscriber(sub)

	// Track subscriber
	connInfo.mu.Lock()
	connInfo.subscribers[msg.Topic] = sub
	connInfo.mu.Unlock()

	// Send acknowledgment
	h.sendAck(conn, msg.RequestID, fmt.Sprintf("Subscribed to topic '%s'", msg.Topic))

	log.Printf("Client %s subscribed to topic '%s'", connInfo.clientID, msg.Topic)
}

// handleUnsubscribe handles unsubscription requests.
func (h *WebSocketHandler) handleUnsubscribe(conn *websocket.Conn, connInfo *connectionInfo, msg *models.WSClientMsg) {
	if msg.Topic == "" {
		h.sendError(conn, "MISSING_TOPIC", "Topic is required for unsubscription")
		return
	}

	connInfo.mu.Lock()
	sub, exists := connInfo.subscribers[msg.Topic]
	if !exists {
		connInfo.mu.Unlock()
		h.sendError(conn, "NOT_SUBSCRIBED", fmt.Sprintf("Not subscribed to topic '%s'", msg.Topic))
		return
	}

	// Remove from tracking
	delete(connInfo.subscribers, msg.Topic)
	connInfo.mu.Unlock()

	// Remove from topic
	topic, topicExists := h.topicManager.GetTopic(msg.Topic)
	if topicExists {
		topic.RemoveSubscriber(connInfo.clientID)
	}

	// Close subscriber
	sub.Close()

	// Send acknowledgment
	h.sendAck(conn, msg.RequestID, fmt.Sprintf("Unsubscribed from topic '%s'", msg.Topic))

	log.Printf("Client %s unsubscribed from topic '%s'", connInfo.clientID, msg.Topic)
}

// handlePublish handles publish requests.
func (h *WebSocketHandler) handlePublish(conn *websocket.Conn, connInfo *connectionInfo, msg *models.WSClientMsg) {
	if msg.Topic == "" {
		h.sendError(conn, "MISSING_TOPIC", "Topic is required for publishing")
		return
	}

	if msg.Message == nil {
		h.sendError(conn, "MISSING_MESSAGE", "Message is required for publishing")
		return
	}

	if msg.Message.ID == "" {
		h.sendError(conn, "MISSING_MESSAGE_ID", "Message ID is required")
		return
	}

	// Check if topic exists
	topic, exists := h.topicManager.GetTopic(msg.Topic)
	if !exists {
		h.sendError(conn, "TOPIC_NOT_FOUND", fmt.Sprintf("Topic '%s' not found", msg.Topic))
		return
	}

	// Publish message
	delivered, dropped := topic.Publish(*msg.Message, "DROP_OLDEST", 100)

	// Send acknowledgment
	h.sendAck(conn, msg.RequestID, fmt.Sprintf("Message published: %d delivered, %d dropped", delivered, dropped))

	log.Printf("Client %s published message to topic '%s': %d delivered, %d dropped",
		connInfo.clientID, msg.Topic, delivered, dropped)
}

// handlePing responds to ping messages with pong.
func (h *WebSocketHandler) handlePing(conn *websocket.Conn, msg *models.WSClientMsg) {
	h.connsMu.RLock()
	connInfo, exists := h.conns[conn]
	h.connsMu.RUnlock()

	if !exists {
		return
	}

	pongMsg := models.ServerMsg{
		Type:      MsgTypePong,
		RequestID: msg.RequestID,
		Ts:        time.Now(),
	}

	select {
	case connInfo.writeChan <- pongMsg:
		// Message sent successfully
	default:
		log.Printf("Warning: write channel full for client %s", connInfo.clientID)
	}
}

// sendAck sends an acknowledgment message.
func (h *WebSocketHandler) sendAck(conn *websocket.Conn, requestID, message string) {
	h.connsMu.RLock()
	connInfo, exists := h.conns[conn]
	h.connsMu.RUnlock()

	if !exists {
		return
	}

	ackMsg := models.ServerMsg{
		Type:      MsgTypeAck,
		RequestID: requestID,
		Message: &models.Message{
			ID:      "ack",
			Payload: json.RawMessage(fmt.Sprintf(`{"message": "%s"}`, message)),
		},
		Ts: time.Now(),
	}

	select {
	case connInfo.writeChan <- ackMsg:
		// Message sent successfully
	default:
		log.Printf("Warning: write channel full for client %s", connInfo.clientID)
	}
}

// sendError sends an error message.
func (h *WebSocketHandler) sendError(conn *websocket.Conn, code, message string) {
	h.connsMu.RLock()
	connInfo, exists := h.conns[conn]
	h.connsMu.RUnlock()

	if !exists {
		return
	}

	errorMsg := models.ServerMsg{
		Type: MsgTypeError,
		Error: &models.ErrorObj{
			Code:    code,
			Message: message,
		},
		Ts: time.Now(),
	}

	select {
	case connInfo.writeChan <- errorMsg:
		// Message sent successfully
	default:
		log.Printf("Warning: write channel full for client %s", connInfo.clientID)
	}
}

// cleanupConnection removes the connection and cleans up all subscriptions.
func (h *WebSocketHandler) cleanupConnection(conn *websocket.Conn) {
	h.connsMu.Lock()
	connInfo, exists := h.conns[conn]
	delete(h.conns, conn)
	h.connsMu.Unlock()

	if !exists {
		return
	}

	// Clean up all subscriptions
	connInfo.mu.Lock()
	for topicName, sub := range connInfo.subscribers {
		// Remove from topic
		if topic, topicExists := h.topicManager.GetTopic(topicName); topicExists {
			topic.RemoveSubscriber(connInfo.clientID)
		}

		// Close subscriber
		sub.Close()
	}
	connInfo.mu.Unlock()

	// Close unified write channel to stop the writer goroutine
	close(connInfo.writeChan)

	// Close connection
	conn.Close()

	log.Printf("Cleaned up connection for client %s", connInfo.clientID)
}

// generateClientID generates a unique client ID for the connection.
func generateClientID() string {
	return fmt.Sprintf("client-%d", time.Now().UnixNano())
}
