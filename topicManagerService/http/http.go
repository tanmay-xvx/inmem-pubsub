// Package http provides HTTP handlers for the topic manager service.
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tanmay-xvx/inmem-pubsub/topicManagerService"
)

// Handler provides HTTP handlers for topic management operations.
type Handler struct {
	topicManager topicManagerService.TopicManager
	startTime    time.Time
}

// NewHandler creates a new HTTP handler with the specified topic manager.
func NewHandler(topicManager topicManagerService.TopicManager) *Handler {
	return &Handler{
		topicManager: topicManager,
		startTime:    time.Now(),
	}
}

// RegisterRoutes registers all HTTP routes with the chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Add middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// API routes
	r.Route("/topics", func(r chi.Router) {
		r.Post("/", h.CreateTopic)
		r.Get("/", h.ListTopics)
		r.Delete("/{name}", h.DeleteTopic)
	})

	// Health and stats endpoints
	r.Get("/health", h.Health)
	r.Get("/stats", h.Stats)
}

// CreateTopicRequest represents the request body for creating a topic.
type CreateTopicRequest struct {
	Name string `json:"name"`
}

// CreateTopicResponse represents the response for topic creation.
type CreateTopicResponse struct {
	Message string `json:"message"`
	Topic   string `json:"topic"`
}

// CreateTopic handles POST /topics requests.
// Expects JSON body: {"name": "topic-name"}
// Returns 201 Created on success, 409 Conflict if topic exists.
func (h *Handler) CreateTopic(w http.ResponseWriter, r *http.Request) {
	var req CreateTopicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Topic name is required", http.StatusBadRequest)
		return
	}

	// Attempt to create the topic
	err := h.topicManager.CreateTopic(req.Name)
	if err != nil {
		switch err.Error() {
		case "topic already exists":
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Topic already exists",
				"topic": req.Name,
			})
			return
		case "invalid topic name":
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid topic name",
				"topic": req.Name,
			})
			return
		default:
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Topic created successfully
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := CreateTopicResponse{
		Message: "Topic created successfully",
		Topic:   req.Name,
	}
	json.NewEncoder(w).Encode(response)
}

// DeleteTopic handles DELETE /topics/{name} requests.
// Returns 200 OK on success, 404 Not Found if topic doesn't exist.
func (h *Handler) DeleteTopic(w http.ResponseWriter, r *http.Request) {
	topicName := chi.URLParam(r, "name")
	if topicName == "" {
		http.Error(w, "Topic name is required", http.StatusBadRequest)
		return
	}

	// Attempt to delete the topic
	err := h.topicManager.DeleteTopic(topicName)
	if err != nil {
		switch err.Error() {
		case "topic not found":
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Topic not found",
				"topic": topicName,
			})
			return
		case "invalid topic name":
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid topic name",
				"topic": topicName,
			})
			return
		default:
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Topic deleted successfully
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Topic deleted successfully",
		"topic":   topicName,
	})
}

// ListTopicsResponse represents the response for listing topics.
type ListTopicsResponse struct {
	Topics []topicManagerService.TopicInfo `json:"topics"`
}

// ListTopics handles GET /topics requests.
// Returns JSON response with list of all topics.
func (h *Handler) ListTopics(w http.ResponseWriter, r *http.Request) {
	topics := h.topicManager.ListTopics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := ListTopicsResponse{
		Topics: topics,
	}
	json.NewEncoder(w).Encode(response)
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status           string  `json:"status"`
	UptimeSeconds    float64 `json:"uptime_seconds"`
	TopicsCount      int     `json:"topics_count"`
	TotalSubscribers int     `json:"total_subscribers"`
	Timestamp        string  `json:"timestamp"`
}

// Health handles GET /health requests.
// Returns system health information including uptime and counts.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime).Seconds()

	// Get topics count and total subscribers
	topics := h.topicManager.ListTopics()
	topicsCount := len(topics)

	totalSubscribers := 0
	for _, topic := range topics {
		totalSubscribers += topic.Subscribers
	}

	response := HealthResponse{
		Status:           "healthy",
		UptimeSeconds:    uptime,
		TopicsCount:      topicsCount,
		TotalSubscribers: totalSubscribers,
		Timestamp:        time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Stats handles GET /stats requests.
// Returns detailed statistics for all topics.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats := h.topicManager.Stats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"topics":    stats,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// ErrorResponse represents a standardized error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

// writeError writes a standardized error response.
func (h *Handler) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    statusCode,
	}

	json.NewEncoder(w).Encode(response)
}
