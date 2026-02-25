.PHONY: build test lint fmt tidy run clean migrate

# Build variables
BINARY_NAME := documcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Build the server binary
build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/server

# Run all tests
test:
	go test ./...

# Run tests with race detection
test-race:
	go test -race ./...

# Run tests with coverage
test-cover:
	go test -cover -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	gofmt -w .
	goimports -w .

# Tidy dependencies
tidy:
	go mod tidy

# Run the server
run:
	go run ./cmd/server

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out

# Run database migrations
migrate:
	go run ./cmd/cli migrate up

# Run database migrations down
migrate-down:
	go run ./cmd/cli migrate down
