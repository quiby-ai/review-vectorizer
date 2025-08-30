.PHONY: build test clean

# Build the main application
build:
	go build -o bin/review-vectorizer cmd/main.go

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Install dependencies
deps:
	go mod tidy
	go mod download

# Run linting
lint:
	golangci-lint run

# Build all binaries
all: clean deps build

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the main application (Kafka consumer)"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Install dependencies"
	@echo "  lint          - Run linting"
	@echo "  all           - Build all binaries"
	@echo "  help          - Show this help message"
