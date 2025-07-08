# Tunnelman Makefile

.PHONY: build test e2e-test clean install dev lint fmt vet deps

# Variables
BINARY_NAME=tunnelman
MAIN_FILE=main.go
BUILD_DIR=dist
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Default target
all: clean deps fmt vet test build

# Build the application
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_FILE)
	@echo "✅ Build completed: $(BINARY_NAME)"

# Build for multiple platforms
build-all:
	@echo "🔨 Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_FILE)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_FILE)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_FILE)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_FILE)
	@echo "✅ Multi-platform builds completed in $(BUILD_DIR)/"

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	go mod tidy
	go mod download
	@echo "✅ Dependencies installed"

# Run unit tests
test:
	@echo "🧪 Running unit tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Unit tests completed"

# Run E2E tests
e2e-test:
	@echo "🧪 Running E2E tests..."
	./scripts/run_e2e_tests.sh
	@echo "✅ E2E tests completed"

# Run specific E2E test
e2e-test-specific:
	@echo "🎯 Running specific E2E test: $(TEST)"
	E2E_VERBOSE=2 ./scripts/run_e2e_tests.sh $(TEST)

# Run E2E tests with verbose output
e2e-test-verbose:
	@echo "🧪 Running E2E tests (verbose)..."
	E2E_VERBOSE=2 ./scripts/run_e2e_tests.sh
	@echo "✅ E2E tests completed"

# Run all tests
test-all: test e2e-test

# Development mode with live reload
dev:
	@echo "🚀 Starting development mode..."
	go run $(MAIN_FILE)

# Lint the code
lint:
	@echo "🔍 Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not installed, using go vet instead"; \
		go vet ./...; \
	fi
	@echo "✅ Linting completed"

# Format the code
fmt:
	@echo "📝 Formatting code..."
	go fmt ./...
	@echo "✅ Code formatted"

# Run go vet
vet:
	@echo "🔍 Running go vet..."
	go vet ./...
	@echo "✅ Vet completed"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning up..."
	go clean
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "✅ Cleanup completed"

# Install the binary to GOPATH/bin
install: build
	@echo "📦 Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) $(MAIN_FILE)
	@echo "✅ $(BINARY_NAME) installed to GOPATH/bin"

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  build-all      - Build for multiple platforms"
	@echo "  deps           - Install dependencies"
	@echo "  test           - Run unit tests"
	@echo "  e2e-test       - Run E2E tests"
	@echo "  e2e-test-specific TEST=<name> - Run specific E2E test"
	@echo "  e2e-test-verbose - Run E2E tests with verbose output"
	@echo "  test-all       - Run all tests (unit + E2E)"
	@echo "  dev            - Start development mode"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  clean          - Clean build artifacts"
	@echo "  install        - Install binary to GOPATH/bin"
	@echo "  help           - Show this help"

# Test targets for CI/CD
ci-test:
	@echo "🤖 Running CI tests..."
	make deps
	make fmt
	make vet
	make test
	@echo "✅ CI tests completed"

# Full CI pipeline
ci: ci-test build

# Pre-commit hook
pre-commit: fmt vet test

# Quick development check
check: fmt vet
	@echo "✅ Quick check completed"