.PHONY: build test lint fmt tidy run clean migrate \
       frontend-install frontend-build frontend-dev frontend-test frontend-lint frontend-generate-api \
       build-all test-all

# Build variables
BINARY_NAME := documcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Build the server binary
build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/documcp

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

# Run the server in combined mode (loads .env if present)
run:
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; go run ./cmd/documcp serve --with-worker

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out

# Run database migrations
migrate:
	go run ./cmd/documcp migrate

# Run database migrations down (not yet supported — use goose directly)
migrate-down:
	@echo "Use 'goose -dir migrations postgres \"$$DB_DSN\" down' directly"

# Frontend
frontend-install:
	cd frontend && npm install

frontend-build: frontend-install
	cd frontend && npm run build

frontend-dev:
	cd frontend && npm run dev

frontend-test:
	cd frontend && npx vitest run

frontend-lint:
	cd frontend && npx vue-tsc --noEmit

frontend-generate-api:
	cd frontend && npm run generate-api

# Combined build
build-all: frontend-build build

# Combined test
test-all: test frontend-test
