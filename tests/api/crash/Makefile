# FSM API Crash Tests Makefile

.PHONY: help test test-basic test-security test-performance test-all clean

# Default target
help:
	@echo "FSM API Crash Tests"
	@echo "=================="
	@echo ""
	@echo "Available targets:"
	@echo "  test-basic      Run basic crash tests (malformed JSON, invalid methods, boundary conditions)"
	@echo "  test-security   Run security crash tests (path traversal, SQL injection, XSS)"
	@echo "  test-performance Run performance crash tests (concurrent requests, memory exhaustion)"
	@echo "  test-all        Run all crash tests"
	@echo "  test            Run all crash tests (alias for test-all)"
	@echo "  clean           Clean up test artifacts"
	@echo ""
	@echo "Environment variables:"
	@echo "  API_URL         Base URL of the FSM API server (default: http://localhost:8080)"
	@echo "  TIMEOUT         Test timeout duration (default: 60s)"
	@echo "  VERBOSE         Enable verbose output (default: false)"
	@echo ""
	@echo "Examples:"
	@echo "  make test-basic"
	@echo "  make test-security API_URL=http://localhost:8080"
	@echo "  make test-all VERBOSE=true"

# Set default values
API_URL ?= http://localhost:8080
TIMEOUT ?= 60s
VERBOSE ?= false

# Build the test runner
build:
	@echo "Building test runner..."
	@cd runner && go build -o run_tests main.go

# Run basic crash tests
test-basic: build
	@echo "Running basic crash tests..."
	@cd runner && ./run_tests -url $(API_URL) -timeout $(TIMEOUT) -pattern "TestBasicCrashTests"

# Run security crash tests
test-security: build
	@echo "Running security crash tests..."
	@cd runner && ./run_tests -url $(API_URL) -timeout $(TIMEOUT) -pattern "TestSecurityCrashTests"

# Run performance crash tests
test-performance: build
	@echo "Running performance crash tests..."
	@cd runner && ./run_tests -url $(API_URL) -timeout $(TIMEOUT) -pattern "TestPerformanceCrashTests"

# Run all crash tests
test-all: build
	@echo "Running all crash tests..."
	@cd runner && ./run_tests -url $(API_URL) -timeout $(TIMEOUT)

# Run all crash tests (alias)
test: test-all

# Clean up test artifacts
clean:
	@echo "Cleaning up test artifacts..."
	@rm -f runner/run_tests
	@echo "Cleanup complete"

# Check if API server is running
check-api:
	@echo "Checking if API server is running at $(API_URL)..."
	@curl -s -f $(API_URL)/api/v1/health > /dev/null || (echo "ERROR: API server is not running at $(API_URL)" && echo "Please start the FSM API server first:" && echo "  go run cmd/api/main.go" && exit 1)
	@echo "✓ API server is running"

# Run tests with API server check
test-with-check: check-api test-all

# Run specific test pattern
test-pattern: build
	@echo "Running tests matching pattern: $(PATTERN)"
	@cd runner && ./run_tests -url $(API_URL) -timeout $(TIMEOUT) -pattern "$(PATTERN)"

# Run tests with verbose output
test-verbose: build
	@echo "Running crash tests with verbose output..."
	@cd runner && ./run_tests -url $(API_URL) -timeout $(TIMEOUT) -verbose

# Run tests with custom timeout
test-timeout: build
	@echo "Running crash tests with timeout: $(TIMEOUT)"
	@cd runner && ./run_tests -url $(API_URL) -timeout $(TIMEOUT)

# Run tests with custom API URL
test-url: build
	@echo "Running crash tests against: $(API_URL)"
	@cd runner && ./run_tests -url $(API_URL) -timeout $(TIMEOUT)
