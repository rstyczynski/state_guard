# State Guard Build System
# Builds three binaries: sg (CLI), sg_api (API server), sg_web (Web UI)

.PHONY: all build clean test help install

# Default target
all: build

# Build all three binaries
build: sg sg_api sg_web

# Build CLI binary
sg:
	@echo "Building sg (CLI)..."
	@go build -o bin/sg ./cmd/sg

# Build API server binary
sg_api:
	@echo "Building sg_api (API Server)..."
	@go build -o bin/sg_api ./cmd/sg_api

# Build Web UI server binary
sg_web:
	@echo "Building sg_web (Web UI Server)..."
	@go build -o bin/sg_web ./cmd/sg_web

# Build legacy api binary (for compatibility)
api:
	@echo "Building legacy api binary (deprecated - use sg_api instead)..."
	@go build -o bin/api ./cmd/api

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/sg bin/sg_api bin/sg_web bin/api bin/fsm
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -cover ./...

# Install binaries to system
install: build
	@echo "Installing binaries to $(GOPATH)/bin..."
	@cp bin/sg $(GOPATH)/bin/
	@cp bin/sg_api $(GOPATH)/bin/
	@cp bin/sg_web $(GOPATH)/bin/
	@echo "Installation complete"

# Show help
help:
	@echo "State Guard Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  make all         - Build all binaries (default)"
	@echo "  make build       - Build all binaries"
	@echo "  make sg          - Build CLI tool"
	@echo "  make sg_api      - Build API server"
	@echo "  make sg_web      - Build Web UI server"
	@echo "  make api         - Build legacy API binary (deprecated)"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make test        - Run tests"
	@echo "  make test-coverage - Run tests with coverage"
	@echo "  make install     - Install binaries to \$$GOPATH/bin"
	@echo "  make help        - Show this help"
	@echo ""
	@echo "Binaries:"
	@echo "  bin/sg       - State Guard CLI tool"
	@echo "  bin/sg_api   - State Guard API Server (pure API, no HTML)"
	@echo "  bin/sg_web   - State Guard Web UI (HTML that calls sg_api)"
	@echo ""
	@echo "Usage:"
	@echo "  # Start API server on port 8080"
	@echo "  ./bin/sg_api --port 8080 --db state_guard.db"
	@echo ""
	@echo "  # Start Web UI on port 3000 (calls API on 8080)"
	@echo "  ./bin/sg_web --port 3000 --api-url http://localhost:8080"
	@echo ""
	@echo "  # Use CLI tool"
	@echo "  ./bin/sg --asset-type examples/simple_asset_type.yaml --id myasset"
