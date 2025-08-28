// Package models provides data structures for the in-memory Pub/Sub system.
package models

import (
	"encoding/json"
	"time"
)

// Message represents a pub/sub message with unique identifier and raw payload.
type Message struct {
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
}

// WSClientMsg represents a WebSocket client message with various operation types.
type WSClientMsg struct {
	Type      string   `json:"type"`
	Topic     string   `json:"topic,omitempty"`
	ClientID  string   `json:"client_id,omitempty"`
	LastN     int      `json:"last_n,omitempty"`
	Message   *Message `json:"message,omitempty"`
	RequestID string   `json:"request_id,omitempty"`
}

// ServerMsg represents a server response message with optional error handling.
type ServerMsg struct {
	Type      string    `json:"type"`
	RequestID string    `json:"request_id,omitempty"`
	Topic     string    `json:"topic,omitempty"`
	Message   *Message  `json:"message,omitempty"`
	Error     *ErrorObj `json:"error,omitempty"`
	Ts        time.Time `json:"ts,omitempty"`
}

// ErrorObj represents an error with code and message.
type ErrorObj struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewErrorObj creates a new ErrorObj with the specified code and message.
func NewErrorObj(code, message string) *ErrorObj {
	return &ErrorObj{
		Code:    code,
		Message: message,
	}
}

// NewServerError creates a new ServerMsg with error information.
func NewServerError(requestID, code, message string) *ServerMsg {
	return &ServerMsg{
		Type:      "error",
		RequestID: requestID,
		Error:     NewErrorObj(code, message),
		Ts:        time.Now(),
	}
}

// NewServerMsg creates a new ServerMsg with the specified type and request ID.
func NewServerMsg(msgType, requestID string) *ServerMsg {
	return &ServerMsg{
		Type:      msgType,
		RequestID: requestID,
		Ts:        time.Now(),
	}
}

// NewMessage creates a new Message with the specified ID and payload.
func NewMessage(id string, payload json.RawMessage) *Message {
	return &Message{
		ID:      id,
		Payload: payload,
	}
}

// NewWSClientMsg creates a new WSClientMsg with the specified type.
func NewWSClientMsg(msgType string) *WSClientMsg {
	return &WSClientMsg{
		Type: msgType,
	}
}
