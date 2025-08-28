# In-Memory Pub/Sub System

A high-performance, in-memory publish/subscribe system built in Go with WebSocket support, HTTP API, and comprehensive metrics.

## 🚀 Features

- **Real-time Messaging**: WebSocket-based real-time communication
- **Topic Management**: Dynamic topic creation and deletion
- **Subscriber Management**: Efficient subscriber lifecycle management
- **Message Policies**: Configurable message handling policies (drop oldest, disconnect)
- **HTTP API**: RESTful API for topic management and system monitoring
- **Metrics & Monitoring**: Comprehensive metrics collection and reporting
- **Thread Safety**: Fully concurrent with proper synchronization
- **Graceful Shutdown**: Clean resource cleanup on system shutdown

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   HTTP Client   │    │  WebSocket Client│    │   HTTP Client   │
└─────────┬───────┘    └──────────┬───────┘    └─────────┬───────┘
          │                        │                       │
          ▼                        ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP Server                              │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Topic Manager   │  │ Subscriber      │  │ Health & Stats  │ │
│  │     API         │  │   Service       │  │     API         │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
          │                        │                       │
          ▼                        ▼                       ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Registry      │    │   Subscriber     │    │    Metrics      │
│  (Topic Store)  │    │   Manager        │    │  Collection     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
          │                        │                       │
          ▼                        ▼                       ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│     Topics      │    │   WebSocket      │    │   Ring Buffer   │
│  (Per Topic)    │    │   Connections    │    │   (Per Topic)   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## 📦 Installation

```bash
# Clone the repository
git clone https://github.com/tanmay-xvx/inmem-pubsub.git
cd inmem-pubsub

# Install dependencies
go mod tidy

# Build the project
go build .

# Run the server
./inmem-pubsub
```

## ⚙️ Configuration

The system can be configured via environment variables or command-line flags:

```bash
# Environment variables
export HOST=localhost
export PORT=8080
export READ_TIMEOUT=30s
export WRITE_TIMEOUT=30s
export DEFAULT_PUBLISH_POLICY=DROP_OLDEST
export DEFAULT_WS_BUFFER_SIZE=100

# Or use a .env file
cp .env.example .env
# Edit .env with your configuration
```

## 🔌 API Endpoints

### Topic Management

- `POST /topics` - Create a new topic
- `DELETE /topics/{name}` - Delete a topic
- `GET /topics` - List all topics

### WebSocket

- `GET /ws` - WebSocket endpoint for real-time communication

### System Monitoring

- `GET /health` - Health check with uptime and system stats
- `GET /stats` - Detailed metrics and statistics

## 📡 WebSocket Protocol

### Message Types

#### Subscribe to Topic

```json
{
  "type": "subscribe",
  "topic": "news",
  "request_id": "req-123"
}
```

#### Unsubscribe from Topic

```json
{
  "type": "unsubscribe",
  "topic": "news",
  "request_id": "req-124"
}
```

#### Publish Message

```json
{
  "type": "publish",
  "topic": "news",
  "message": {
    "id": "msg-456",
    "payload": "Breaking news!",
    "timestamp": "2024-01-01T12:00:00Z"
  },
  "request_id": "req-125"
}
```

#### Ping (Keep-alive)

```json
{
  "type": "ping",
  "request_id": "req-126"
}
```

### Response Messages

#### Acknowledgment

```json
{
  "type": "ack",
  "request_id": "req-123",
  "message": {
    "id": "ack",
    "payload": "Subscribed to topic 'news'"
  },
  "ts": "2024-01-01T12:00:00Z"
}
```

#### Error

```json
{
  "type": "error",
  "error": {
    "code": "TOPIC_NOT_FOUND",
    "message": "Topic 'nonexistent' not found"
  },
  "ts": "2024-01-01T12:00:00Z"
}
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test ./internals/topic

# Run tests with coverage
go test -cover ./...
```

## 📊 Metrics

The system provides comprehensive metrics:

- **Global Metrics**: Total topics, subscribers, messages, dropped messages
- **Per-Topic Metrics**: Published, delivered, dropped, subscriber count
- **Real-time Updates**: Atomic counters for live monitoring
- **JSON Export**: `/stats` endpoint for integration with monitoring systems

## 🔧 Development

### Project Structure

```
├── internals/                 # Core internal packages
│   ├── config/               # Configuration management
│   ├── metrics/              # Metrics collection
│   ├── models/               # Data models and types
│   ├── registry/             # Topic registry
│   ├── ringbuffer/           # Ring buffer implementation
│   ├── subscriber/           # Subscriber management
│   └── topic/                # Topic implementation
├── subscriberService/         # Subscriber service layer
│   ├── http/                 # HTTP handlers
│   ├── interface.go          # Service interface
│   └── service.go            # Service implementation
├── topicManagerService/       # Topic management service
│   ├── http/                 # HTTP handlers
│   ├── interface.go          # Service interface
│   └── service.go            # Service implementation
├── main.go                   # Main application entry point
├── go.mod                    # Go module definition
└── README.md                 # This file
```

### Adding New Features

1. **New Message Types**: Add to `internals/models/` and update handlers
2. **New Policies**: Extend topic publishing policies in `internals/topic/`
3. **New Metrics**: Add to `internals/metrics/` and integrate with services
4. **New Endpoints**: Add HTTP handlers and register routes

## 🚀 Performance

- **Concurrent Design**: Fully concurrent with proper synchronization
- **Memory Efficient**: Ring buffer implementation for message storage
- **Fast Operations**: O(1) topic lookup, O(n) subscriber notification
- **Scalable**: Designed for high-throughput message delivery

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Go](https://golang.org/)
- HTTP routing with [Chi](https://github.com/go-chi/chi)
- WebSocket support with [Gorilla WebSocket](https://github.com/gorilla/websocket)
- Environment configuration with [Godotenv](https://github.com/joho/godotenv)

## 📞 Support

For questions, issues, or contributions, please open an issue on GitHub or contact the maintainers.
