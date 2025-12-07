.PHONY: build test install clean fmt lint help

# Binary name
BINARY_NAME=go-semver-audit
BUILD_DIR=bin

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOINSTALL=$(GOCMD) install
GOCLEAN=$(GOCMD) clean
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build the project
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/go-semver-audit

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -cover -coverprofile=coverage.txt ./...
	$(GOCMD) tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Install the binary
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOINSTALL) ./cmd/go-semver-audit

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.txt coverage.html

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Run linter
lint:
	@echo "Running go vet..."
	$(GOVET) ./...

# Run all checks (format, lint, test)
check: fmt lint test

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  install        - Install the binary"
	@echo "  clean          - Clean build artifacts"
	@echo "  fmt            - Format code"
	@echo "  lint           - Run linter"
	@echo "  check          - Run format, lint, and tests"
	@echo "  help           - Show this help message"

# Default target
.DEFAULT_GOAL := build

