# LiteClaw Makefile

.PHONY: all build clean test run dev

VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w
LDFLAGS += -X github.com/liteclaw/liteclaw/internal/version.Version=$(VERSION)
LDFLAGS += -X github.com/liteclaw/liteclaw/internal/version.Commit=$(COMMIT)
LDFLAGS += -X github.com/liteclaw/liteclaw/internal/version.BuildDate=$(BUILD_DATE)

all: build

# Build the binary
build:
	CGO_ENABLED=1 go build -ldflags="$(LDFLAGS)" -o liteclaw ./cmd/liteclaw

# Build for production (smaller binary)
build-prod:
	CGO_ENABLED=1 go build -ldflags="$(LDFLAGS)" -trimpath -o liteclaw ./cmd/liteclaw

# Clean build artifacts
clean:
	rm -f liteclaw
	go clean

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run in development mode
dev:
	go run ./cmd/liteclaw gateway

# Run the gateway
run:
	go run ./cmd/liteclaw gateway

# Install dependencies
deps:
	go mod download
	go mod tidy

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Show version
version:
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

# Cross-compile for common platforms
release:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -ldflags="$(LDFLAGS)" -o dist/liteclaw-linux-amd64 ./cmd/liteclaw
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -ldflags="$(LDFLAGS)" -o dist/liteclaw-linux-arm64 ./cmd/liteclaw
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -ldflags="$(LDFLAGS)" -o dist/liteclaw-darwin-amd64 ./cmd/liteclaw
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -ldflags="$(LDFLAGS)" -o dist/liteclaw-darwin-arm64 ./cmd/liteclaw
