.PHONY: build run test clean docker-build docker-run

# Build the application
build:
	go build -o bin/inmem-pubsub .

# Run the application
run:
	go run main.go

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Build Docker image
docker-build:
	docker build -t inmem-pubsub .

# Run Docker container
docker-run:
	docker run -p 8080:8080 inmem-pubsub

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt .

# Lint code
lint:
	golangci-lint run

# Help
help:
	@echo "Available commands:"
	@echo "  build       - Build the application"
	@echo "  run         - Run the application locally"
	@echo "  test        - Run tests"
	@echo "  clean       - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run  - Run Docker container"
	@echo "  deps        - Install dependencies"
	@echo "  fmt         - Format code"
	@echo "  lint        - Lint code"
	@echo "  help        - Show this help"
