package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"

	"github.com/tanmay-xvx/inmem-pubsub/internals/config"
	"github.com/tanmay-xvx/inmem-pubsub/internals/metrics"
	"github.com/tanmay-xvx/inmem-pubsub/internals/registry"
	"github.com/tanmay-xvx/inmem-pubsub/subscriberService"
	subscriberHTTP "github.com/tanmay-xvx/inmem-pubsub/subscriberService/http"
	"github.com/tanmay-xvx/inmem-pubsub/topicManagerService"
	topicManagerHTTP "github.com/tanmay-xvx/inmem-pubsub/topicManagerService/http"
)

var (
	configFile = flag.String("config", ".env", "Path to configuration file")
	startTime  = time.Now()
)

func main() {
	flag.Parse()

	// Load environment variables
	if err := godotenv.Load(*configFile); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Load configuration
	cfg := config.NewConfig()
	cfg.ParseFlags()

	log.Printf("Starting in-memory Pub/Sub server on %s:%s", cfg.Host, cfg.Port)

	// Create metrics
	metrics := metrics.NewMetrics()

	// Create registry
	registry := registry.NewRegistry(cfg, metrics)

	// Create topic manager service
	topicMgrSvc := topicManagerService.NewTopicManagerService(registry, cfg, metrics)

	// Create subscriber service
	subscriberSvc := subscriberService.NewSubscriberService(registry, cfg, topicMgrSvc)

	// Start services
	if err := subscriberSvc.Start(); err != nil {
		log.Fatalf("Failed to start subscriber service: %v", err)
	}

	// Create chi router
	router := chi.NewRouter()

	// Register routes
	topicManagerHTTP.RegisterTopicManagerRoutes(router, topicMgrSvc)
	subscriberHTTP.RegisterSubscriberRoutes(router, subscriberSvc)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("HTTP server starting on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Shutdown subscriber service
	if err := subscriberSvc.Shutdown(ctx); err != nil {
		log.Printf("Subscriber service shutdown error: %v", err)
	}

	// Close all topics in registry
	topics := topicMgrSvc.ListTopics()
	for _, topicInfo := range topics {
		if err := topicMgrSvc.DeleteTopic(topicInfo.Name); err != nil {
			log.Printf("Failed to delete topic %s: %v", topicInfo.Name, err)
		}
	}

	log.Println("Server shutdown complete")
}
