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

### Local Development

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

### Docker

#### Prerequisites

- Docker installed and running on your system
- Port 8080 available on your host machine

#### Quick Start with Docker

```bash
# 1. Clone the repository
git clone https://github.com/tanmay-xvx/inmem-pubsub.git
cd inmem-pubsub

# 2. Build Docker image
docker build -t inmem-pubsub .

# 3. Run container in detached mode
docker run -d -p 8080:8080 --name inmem-pubsub-container inmem-pubsub

# 4. Verify service is running
curl http://localhost:8080/health
```

#### Docker Management Commands

```bash
# View container logs (real-time)
docker logs -f inmem-pubsub-container

# View last 50 log lines
docker logs --tail 50 inmem-pubsub-container

# Check container status
docker ps

# Stop the container
docker stop inmem-pubsub-container

# Remove the container
docker rm inmem-pubsub-container

# Remove the image
docker rmi inmem-pubsub
```

#### Docker with Custom Configuration

```bash
# Run with environment variables
docker run -d -p 8080:8080 \
  -e HOST=0.0.0.0 \
  -e PORT=8080 \
  -e READ_TIMEOUT=30s \
  -e WRITE_TIMEOUT=30s \
  --name inmem-pubsub-container \
  inmem-pubsub

# Run with custom port mapping
docker run -d -p 9090:8080 --name inmem-pubsub-custom-port inmem-pubsub
# Service will be available at http://localhost:9090
```

#### Testing the Dockerized Service

```bash
# 1. Create a topic
curl -X POST http://localhost:8080/topics \
  -H "Content-Type: application/json" \
  -d '{"name": "orders"}'

# 2. Check health
curl http://localhost:8080/health | jq .

# 3. List topics
curl http://localhost:8080/topics | jq .

# 4. Get statistics
curl http://localhost:8080/stats | jq .

# 5. Test WebSocket (requires Python websocket-client)
pip install websocket-client
python test_websocket_proper.py
```

#### Docker Compose (Optional)

Create a `docker-compose.yml` file:

```yaml
version: "3.8"
services:
  inmem-pubsub:
    build: .
    ports:
      - "8080:8080"
    environment:
      - HOST=0.0.0.0
      - PORT=8080
      - READ_TIMEOUT=30s
      - WRITE_TIMEOUT=30s
    restart: unless-stopped
    container_name: inmem-pubsub
```

Run with Docker Compose:

```bash
# Start service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop service
docker-compose down
```

#### Troubleshooting Docker

```bash
# Check if Docker is running
docker --version

# Check if port 8080 is available
lsof -i :8080  # On macOS/Linux
netstat -an | grep 8080  # On Windows

# Rebuild image (force)
docker build --no-cache -t inmem-pubsub .

# Check container resource usage
docker stats inmem-pubsub-container

# Access container shell for debugging
docker exec -it inmem-pubsub-container /bin/sh
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
  "topic": "orders",
  "client_id": "test-client",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "last_n": 5
}
```

#### Unsubscribe from Topic

```json
{
  "type": "unsubscribe",
  "topic": "orders",
  "client_id": "test-client",
  "request_id": "340e8400-e29b-41d4-a716-4466554480098"
}
```

#### Publish Message

```json
{
  "type": "publish",
  "topic": "orders",
  "message": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "payload": {
      "order_id": "ORD-123",
      "amount": "99.5",
      "currency": "USD"
    }
  },
  "request_id": "340e8400-e29b-41d4-a716-4466554480098"
}
```

#### Ping (Keep-alive)

```json
{
  "type": "ping",
  "request_id": "570t8400-e29b-41d4-a716-4466554412345"
}
```

### Response Messages

#### Acknowledgment

```json
{
  "type": "ack",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "topic": "orders",
  "status": "ok",
  "ts": "2025-08-25T10:00:00Z"
}
```

#### Published Message Event

```json
{
  "type": "event",
  "topic": "orders",
  "message": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "payload": {
      "order_id": "ORD-123",
      "amount": 99.5,
      "currency": "USD"
    }
  },
  "ts": "2025-08-25T10:01:00Z"
}
```

#### Error

```json
{
  "type": "error",
  "request_id": "req-67890",
  "error": {
    "code": "TOPIC_NOT_FOUND",
    "message": "Topic 'nonexistent' not found"
  },
  "ts": "2025-08-25T10:02:00Z"
}
```

#### Pong Response

```json
{
  "type": "pong",
  "request_id": "ping-abc",
  "ts": "2025-08-25T10:03:00Z"
}
```

## 🧪 Testing

### Unit Tests

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

### Integration Testing

Test the complete WebSocket functionality:

```bash
# Create a topic first
curl -X POST http://localhost:8080/topics -H "Content-Type: application/json" -d '{"name": "orders"}'

# Test WebSocket functionality (requires websocket-client Python package)
pip install websocket-client
python test_websocket_proper.py
```

### Manual API Testing

```bash
# Health check
curl http://localhost:8080/health

# List topics
curl http://localhost:8080/topics

# Get detailed stats
curl http://localhost:8080/stats

# Create a topic
curl -X POST http://localhost:8080/topics -H "Content-Type: application/json" -d '{"name": "test"}'

# Delete a topic
curl -X DELETE http://localhost:8080/topics/test
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
- **Race Condition Free**: Unified write channels eliminate WebSocket write conflicts
- **Memory Efficient**: Ring buffer implementation for message storage
- **Fast Operations**: O(1) topic lookup, O(n) subscriber notification
- **Scalable**: Designed for high-throughput message delivery
- **Backpressure Handling**: DROP_OLDEST policy for buffer overflow management

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 🙏 Acknowledgments

- Built with [Go](https://golang.org/)
- HTTP routing with [Chi](https://github.com/go-chi/chi)
- WebSocket support with [Gorilla WebSocket](https://github.com/gorilla/websocket)
- Environment configuration with [Godotenv](https://github.com/joho/godotenv)

## 📞 Support

For questions, issues, or contributions, please open an issue on GitHub or contact the maintainers.
