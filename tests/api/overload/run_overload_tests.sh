#!/bin/bash

# FSM API Overload Test Runner
# This script runs comprehensive overload tests against the FSM API

set -e

# Default values
BASE_URL="http://localhost:8080"
TIMEOUT="30m"
VERBOSE=false
TEST_PATTERN=""
PROFILE=false
RACE=false
MEMORY_PROFILE=""
CPU_PROFILE=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -u|--url)
            BASE_URL="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -p|--pattern)
            TEST_PATTERN="$2"
            shift 2
            ;;
        --profile)
            PROFILE=true
            shift
            ;;
        --race)
            RACE=true
            shift
            ;;
        --mem-profile)
            MEMORY_PROFILE="$2"
            shift 2
            ;;
        --cpu-profile)
            CPU_PROFILE="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -u, --url URL        Base URL of the FSM API server (default: http://localhost:8080)"
            echo "  -t, --timeout TIME   Test timeout duration (default: 30m)"
            echo "  -v, --verbose        Enable verbose output"
            echo "  -p, --pattern PAT    Run only tests matching the pattern"
            echo "  --profile            Enable CPU and memory profiling"
            echo "  --race               Enable race detection"
            echo "  --mem-profile FILE   Memory profile output file"
            echo "  --cpu-profile FILE   CPU profile output file"
            echo "  -h, --help           Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "FSM API Overload Test Runner"
echo "============================"
echo "Target URL: $BASE_URL"
echo "Timeout: $TIMEOUT"
echo "Verbose: $VERBOSE"
if [ -n "$TEST_PATTERN" ]; then
    echo "Pattern: $TEST_PATTERN"
fi
if [ "$PROFILE" = true ]; then
    echo "Profiling: Enabled"
fi
if [ "$RACE" = true ]; then
    echo "Race Detection: Enabled"
fi
echo ""

# Check if the API server is running
echo "Checking if API server is running..."
if ! curl -s -f "$BASE_URL/api/v1/health" > /dev/null 2>&1; then
    echo "ERROR: API server is not running at $BASE_URL"
    echo "Please start the FSM API server first:"
    echo "  go run cmd/api/main.go"
    exit 1
fi
echo "✓ API server is running"

# Check system resources
echo ""
echo "Checking system resources..."
echo "==========================="

# Check available memory
if command -v free >/dev/null 2>&1; then
    AVAILABLE_MEM=$(free -m | awk 'NR==2{printf "%.0f", $7}')
    echo "Available memory: ${AVAILABLE_MEM}MB"
    if [ "$AVAILABLE_MEM" -lt 1000 ]; then
        echo "⚠️  WARNING: Low available memory (${AVAILABLE_MEM}MB). Overload tests may fail."
    fi
fi

# Check CPU cores
if command -v nproc >/dev/null 2>&1; then
    CPU_CORES=$(nproc)
    echo "CPU cores: $CPU_CORES"
    if [ "$CPU_CORES" -lt 4 ]; then
        echo "⚠️  WARNING: Limited CPU cores ($CPU_CORES). Overload tests may be slow."
    fi
fi

# Check disk space
if command -v df >/dev/null 2>&1; then
    DISK_SPACE=$(df -h . | awk 'NR==2{print $4}')
    echo "Available disk space: $DISK_SPACE"
fi

echo ""
echo "Cleaning test cache..."
echo "======================"
go clean -testcache

# Run the tests
echo ""
echo "Running overload tests..."
echo "========================"

# Build test command
ARGS=("test" "-timeout" "$TIMEOUT")

if [ "$VERBOSE" = true ]; then
    ARGS+=("-v")
fi

if [ "$RACE" = true ]; then
    ARGS+=("-race")
fi

if [ "$PROFILE" = true ] || [ -n "$MEMORY_PROFILE" ] || [ -n "$CPU_PROFILE" ]; then
    if [ -n "$MEMORY_PROFILE" ]; then
        ARGS+=("-memprofile" "$MEMORY_PROFILE")
    elif [ "$PROFILE" = true ]; then
        ARGS+=("-memprofile" "mem.prof")
    fi
    
    if [ -n "$CPU_PROFILE" ]; then
        ARGS+=("-cpuprofile" "$CPU_PROFILE")
    elif [ "$PROFILE" = true ]; then
        ARGS+=("-cpuprofile" "cpu.prof")
    fi
fi

if [ -n "$TEST_PATTERN" ]; then
    ARGS+=("-run" "$TEST_PATTERN")
fi

ARGS+=(".")

# Change to the overload test directory
cd "$(dirname "$0")"

# Run the tests
echo "Executing: go ${ARGS[*]}"
echo ""

# Capture start time
START_TIME=$(date +%s)

# Run the tests and capture output
if go "${ARGS[@]}"; then
    TEST_RESULT="PASSED"
else
    TEST_RESULT="FAILED"
fi

# Calculate duration
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo ""
echo "Overload tests completed!"
echo "========================"
echo "Result: $TEST_RESULT"
echo "Duration: ${DURATION}s"

# Show profile files if created
if [ "$PROFILE" = true ] || [ -n "$MEMORY_PROFILE" ] || [ -n "$CPU_PROFILE" ]; then
    echo ""
    echo "Profile files created:"
    if [ -f "mem.prof" ] || [ -n "$MEMORY_PROFILE" ]; then
        echo "  Memory profile: ${MEMORY_PROFILE:-mem.prof}"
        echo "    Analyze with: go tool pprof ${MEMORY_PROFILE:-mem.prof}"
    fi
    if [ -f "cpu.prof" ] || [ -n "$CPU_PROFILE" ]; then
        echo "  CPU profile: ${CPU_PROFILE:-cpu.prof}"
        echo "    Analyze with: go tool pprof ${CPU_PROFILE:-cpu.prof}"
    fi
fi

# Show system resource usage after tests
echo ""
echo "System resource usage after tests:"
echo "==================================="

if command -v free >/dev/null 2>&1; then
    AVAILABLE_MEM_AFTER=$(free -m | awk 'NR==2{printf "%.0f", $7}')
    echo "Available memory: ${AVAILABLE_MEM_AFTER}MB"
fi

if command -v df >/dev/null 2>&1; then
    DISK_SPACE_AFTER=$(df -h . | awk 'NR==2{print $4}')
    echo "Available disk space: $DISK_SPACE_AFTER"
fi

# Check for potential issues
echo ""
echo "Post-test analysis:"
echo "=================="

# Check for goroutine leaks (if pprof is available)
if curl -s "$BASE_URL/debug/pprof/goroutine" >/dev/null 2>&1; then
    GOROUTINE_COUNT=$(curl -s "$BASE_URL/debug/pprof/goroutine" | grep -c "goroutine" || echo "unknown")
    echo "Active goroutines: $GOROUTINE_COUNT"
    if [ "$GOROUTINE_COUNT" != "unknown" ] && [ "$GOROUTINE_COUNT" -gt 1000 ]; then
        echo "⚠️  WARNING: High goroutine count. Possible goroutine leak."
    fi
fi

# Check API health
if curl -s -f "$BASE_URL/api/v1/health" >/dev/null 2>&1; then
    echo "✓ API server is still healthy"
else
    echo "❌ API server may be overloaded or crashed"
fi

# Exit with appropriate code
if [ "$TEST_RESULT" = "PASSED" ]; then
    exit 0
else
    exit 1
fi