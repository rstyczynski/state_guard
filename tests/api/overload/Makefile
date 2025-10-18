# FSM API Overload Tests Makefile

.PHONY: help test test-concurrent test-memory test-network test-system test-all clean profile race

# Default target
help:
	@echo "FSM API Overload Tests"
	@echo "====================="
	@echo ""
	@echo "Available targets:"
	@echo "  test              Run all overload tests"
	@echo "  test-concurrent   Run concurrent request overload tests"
	@echo "  test-memory       Run memory exhaustion overload tests"
	@echo "  test-network      Run network overload tests"
	@echo "  test-system       Run system resource overload tests"
	@echo "  test-all          Run all tests with profiling"
	@echo "  profile           Run tests with CPU and memory profiling"
	@echo "  race              Run tests with race detection"
	@echo "  clean             Clean test cache and profile files"
	@echo "  help              Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make test-concurrent"
	@echo "  make profile"
	@echo "  make test-all"

# Run all overload tests
test:
	@echo "Running all overload tests..."
	@./run_overload_tests.sh

# Run concurrent request overload tests
test-concurrent:
	@echo "Running concurrent request overload tests..."
	@./run_overload_tests.sh -p "TestConcurrentRequestOverload|TestRapidFireOverload|TestConnectionPoolExhaustion"

# Run memory exhaustion overload tests
test-memory:
	@echo "Running memory exhaustion overload tests..."
	@./run_overload_tests.sh -p "TestMemoryExhaustionOverload|TestBandwidthSaturation|TestMemoryLeakDetection"

# Run network overload tests
test-network:
	@echo "Running network overload tests..."
	@./run_overload_tests.sh -p "TestNetworkLatencyOverload|TestConnectionTimeoutOverload|TestSlowLorisAttack|TestPacketLossSimulation|TestBandwidthThrottling"

# Run system resource overload tests
test-system:
	@echo "Running system resource overload tests..."
	@./run_overload_tests.sh -p "TestDatabaseOverload|TestFileSystemOverload|TestGoroutineLeakDetection|TestResourceExhaustionRecovery"

# Run all tests with profiling
test-all:
	@echo "Running all overload tests with profiling..."
	@./run_overload_tests.sh --profile

# Run tests with CPU and memory profiling
profile:
	@echo "Running overload tests with profiling..."
	@./run_overload_tests.sh --profile --mem-profile mem.prof --cpu-profile cpu.prof

# Run tests with race detection
race:
	@echo "Running overload tests with race detection..."
	@./run_overload_tests.sh --race

# Clean test cache and profile files
clean:
	@echo "Cleaning test cache and profile files..."
	@go clean -testcache
	@rm -f *.prof
	@rm -f mem.prof cpu.prof
	@echo "Cleanup completed"

# Run specific test patterns
test-pattern:
	@echo "Usage: make test-pattern PATTERN='TestName'"
	@if [ -z "$(PATTERN)" ]; then \
		echo "Error: PATTERN is required"; \
		echo "Example: make test-pattern PATTERN='TestConcurrentRequestOverload'"; \
		exit 1; \
	fi
	@echo "Running tests matching pattern: $(PATTERN)"
	@./run_overload_tests.sh -p "$(PATTERN)"

# Run tests with custom timeout
test-timeout:
	@echo "Usage: make test-timeout TIMEOUT='10m'"
	@if [ -z "$(TIMEOUT)" ]; then \
		echo "Error: TIMEOUT is required"; \
		echo "Example: make test-timeout TIMEOUT='10m'"; \
		exit 1; \
	fi
	@echo "Running tests with timeout: $(TIMEOUT)"
	@./run_overload_tests.sh -t "$(TIMEOUT)"

# Run tests with custom URL
test-url:
	@echo "Usage: make test-url URL='http://localhost:8080'"
	@if [ -z "$(URL)" ]; then \
		echo "Error: URL is required"; \
		echo "Example: make test-url URL='http://localhost:8080'"; \
		exit 1; \
	fi
	@echo "Running tests against URL: $(URL)"
	@./run_overload_tests.sh -u "$(URL)"

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	@./run_overload_tests.sh -v

# Run tests with all options
test-full:
	@echo "Running full overload test suite..."
	@./run_overload_tests.sh -v --profile --race -t 30m

# Quick test (reduced concurrency)
test-quick:
	@echo "Running quick overload tests..."
	@go test -v -timeout 5m -run "TestConcurrentRequestOverload/Concurrent_100_requests|TestMemoryExhaustionOverload/Memory_1MB" .

# Stress test (high concurrency)
test-stress:
	@echo "Running stress overload tests..."
	@./run_overload_tests.sh -p "TestConcurrentRequestOverload/Concurrent_5000_requests|TestResourceExhaustionOverload"

# Memory test (large payloads)
test-memory-large:
	@echo "Running large memory overload tests..."
	@./run_overload_tests.sh -p "TestMemoryExhaustionOverload/Memory_100MB|TestBandwidthSaturation"

# Network test (latency and timeouts)
test-network-stress:
	@echo "Running network stress tests..."
	@./run_overload_tests.sh -p "TestNetworkLatencyOverload|TestConnectionTimeoutOverload|TestSlowLorisAttack"

# System test (resource exhaustion)
test-system-stress:
	@echo "Running system stress tests..."
	@./run_overload_tests.sh -p "TestDatabaseOverload|TestFileSystemOverload|TestResourceExhaustionRecovery"

# Show test results summary
summary:
	@echo "Overload Test Summary"
	@echo "===================="
	@echo "Test files:"
	@ls -la *.go | grep -E "\.(go)$" | wc -l | xargs echo "  Go files:"
	@echo "Test categories:"
	@echo "  - Concurrent Request Overload"
	@echo "  - Memory Exhaustion Overload"
	@echo "  - Network Overload"
	@echo "  - System Resource Overload"
	@echo "  - CPU Intensive Overload"
	@echo "  - Resource Exhaustion Recovery"
	@echo ""
	@echo "Available test patterns:"
	@grep -r "func Test" *.go | sed 's/.*func //' | sed 's/(.*//' | sort | sed 's/^/  /'
