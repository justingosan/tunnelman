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

# Run unit tests (excluding E2E tests)
test:
	@echo "🧪 Running unit tests..."
	go test -v -race -coverprofile=coverage.out -skip TestE2E ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Unit tests completed"

# Run E2E tests
e2e-test:
	@echo "⚠️  WARNING: E2E tests will:"
	@echo "   • Create and delete real Cloudflare tunnels"
	@echo "   • Create and delete DNS records in your Cloudflare zone"
	@echo "   • Use your Cloudflare API credentials"
	@echo "   • Perform real operations that may incur charges"
	@echo ""
	@echo "   Ensure you have:"
	@echo "   • Valid Cloudflare API credentials configured"
	@echo "   • cloudflared CLI tool installed"
	@echo "   • Appropriate API permissions"
	@echo ""
	@read -p "Continue with E2E tests? (y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	@echo "🧪 Running E2E tests..."
	./scripts/run_e2e_tests.sh
	@echo "✅ E2E tests completed"

# Run specific E2E test
e2e-test-specific:
	@echo "⚠️  WARNING: This will run real E2E tests against your Cloudflare account!"
	@read -p "Continue with specific E2E test '$(TEST)'? (y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	@echo "🎯 Running specific E2E test: $(TEST)"
	E2E_VERBOSE=2 ./scripts/run_e2e_tests.sh $(TEST)

# Run E2E tests with verbose output
e2e-test-verbose:
	@echo "⚠️  WARNING: E2E tests will create/delete real tunnels and DNS records!"
	@read -p "Continue with verbose E2E tests? (y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	@echo "🧪 Running E2E tests (verbose)..."
	E2E_VERBOSE=2 ./scripts/run_e2e_tests.sh
	@echo "✅ E2E tests completed"

# Run E2E tests without prompts (for CI)
e2e-test-ci:
	@echo "🤖 Running E2E tests in CI mode (no prompts)..."
	./scripts/run_e2e_tests.sh
	@echo "✅ E2E tests completed"

# Run all tests (unit tests only - safe default)
test-all: test
	@echo "ℹ️  Note: Use 'make test-all-with-e2e' to include E2E tests"

# Run all tests including E2E (with prompts)
test-all-with-e2e: test e2e-test

# Run all tests including E2E (for CI - no prompts)
test-all-ci: test e2e-test-ci

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

# Install the binary to user's local bin (~/.local/bin) - recommended
install: build
	@echo "📦 Installing $(BINARY_NAME) to ~/.local/bin..."
	@mkdir -p ~/.local/bin
	@cp $(BINARY_NAME) ~/.local/bin/
	@chmod +x ~/.local/bin/$(BINARY_NAME)
	@echo "✅ $(BINARY_NAME) installed to ~/.local/bin"
	@echo "ℹ️  Make sure ~/.local/bin is in your PATH"

# Install the binary to GOPATH/bin
install-go: build
	@echo "📦 Installing $(BINARY_NAME) to GOPATH/bin..."
	go install $(LDFLAGS) $(MAIN_FILE)
	@echo "✅ $(BINARY_NAME) installed to $(shell go env GOPATH)/bin"

# Install the binary to /usr/local/bin (requires sudo)
install-system: build
	@echo "📦 Installing $(BINARY_NAME) to /usr/local/bin (requires sudo)..."
	sudo cp $(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "✅ $(BINARY_NAME) installed to /usr/local/bin"

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  build-all      - Build for multiple platforms"
	@echo "  deps           - Install dependencies"
	@echo "  test           - Run unit tests"
	@echo "  e2e-test       - Run E2E tests (with safety prompts)"
	@echo "  e2e-test-specific TEST=<name> - Run specific E2E test"
	@echo "  e2e-test-verbose - Run E2E tests with verbose output"
	@echo "  e2e-test-ci     - Run E2E tests without prompts (for CI)"
	@echo "  test-all       - Run all tests (unit tests only)"
	@echo "  test-all-with-e2e - Run all tests including E2E (with prompts)"
	@echo "  test-all-ci    - Run all tests including E2E (no prompts)"
	@echo "  dev            - Start development mode"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  clean          - Clean build artifacts"
	@echo "  install        - Install binary to ~/.local/bin (recommended)"
	@echo "  install-go     - Install binary to GOPATH/bin"
	@echo "  install-system - Install binary to /usr/local/bin (requires sudo)"
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