// Package config provides configuration management for the Pub/Sub system.
package config

import (
	"flag"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration options for the Pub/Sub system.
type Config struct {
	// Server configuration
	Port   string
	Host   string
	WSPath string

	// Topic configuration
	DefaultRingBufferSize int
	DefaultWSBufferSize   int
	DefaultPublishPolicy  string

	// Timeout configuration
	WriteTimeout time.Duration
	ReadTimeout  time.Duration

	// Logging configuration
	LogLevel string
}

// NewConfig creates a new configuration with default values.
func NewConfig() *Config {
	return &Config{
		Port:                  getEnv("PORT", "8080"),
		Host:                  getEnv("HOST", "0.0.0.0"),
		WSPath:                getEnv("WS_PATH", "/ws"),
		DefaultRingBufferSize: getEnvAsInt("DEFAULT_RING_BUFFER_SIZE", 1000),
		DefaultWSBufferSize:   getEnvAsInt("DEFAULT_WS_BUFFER_SIZE", 100),
		DefaultPublishPolicy:  getEnv("DEFAULT_PUBLISH_POLICY", "DROP_OLDEST"),
		WriteTimeout:          getEnvAsDuration("WRITE_TIMEOUT", 30*time.Second),
		ReadTimeout:           getEnvAsDuration("READ_TIMEOUT", 60*time.Second),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
	}
}

// ParseFlags parses command-line flags and updates the configuration.
func (c *Config) ParseFlags() {
	flag.StringVar(&c.Port, "port", c.Port, "HTTP server port")
	flag.StringVar(&c.Host, "host", c.Host, "HTTP server host")
	flag.StringVar(&c.WSPath, "ws-path", c.WSPath, "WebSocket endpoint path")
	flag.IntVar(&c.DefaultRingBufferSize, "ring-buffer-size", c.DefaultRingBufferSize, "Default ring buffer size for topics")
	flag.IntVar(&c.DefaultWSBufferSize, "ws-buffer-size", c.DefaultWSBufferSize, "Default WebSocket buffer size")
	flag.StringVar(&c.DefaultPublishPolicy, "publish-policy", c.DefaultPublishPolicy, "Default publish policy (DROP_OLDEST, DISCONNECT)")
	flag.DurationVar(&c.WriteTimeout, "write-timeout", c.WriteTimeout, "WebSocket write timeout")
	flag.DurationVar(&c.ReadTimeout, "read-timeout", c.ReadTimeout, "WebSocket read timeout")
	flag.StringVar(&c.LogLevel, "log-level", c.LogLevel, "Log level (debug, info, warn, error)")

	flag.Parse()
}

// getEnv gets an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable as an integer or returns a default value.
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsDuration gets an environment variable as a duration or returns a default value.
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
