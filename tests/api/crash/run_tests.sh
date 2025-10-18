#!/bin/bash

# FSM API Crash Test Runner
# This script runs comprehensive crash tests against the FSM API

set -e

# Default values
BASE_URL="http://localhost:8080"
TIMEOUT="60s"
VERBOSE=false
TEST_PATTERN=""

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
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -u, --url URL        Base URL of the FSM API server (default: http://localhost:8080)"
            echo "  -t, --timeout TIME   Test timeout duration (default: 60s)"
            echo "  -v, --verbose        Enable verbose output"
            echo "  -p, --pattern PAT    Run only tests matching the pattern"
            echo "  -h, --help           Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "FSM API Crash Test Runner"
echo "========================="
echo "Target URL: $BASE_URL"
echo "Timeout: $TIMEOUT"
echo "Verbose: $VERBOSE"
if [ -n "$TEST_PATTERN" ]; then
    echo "Pattern: $TEST_PATTERN"
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

# Run the tests
echo ""
echo "Running crash tests..."
echo "====================="

if [ -n "$TEST_PATTERN" ]; then
    go test -v -timeout "$TIMEOUT" -run "$TEST_PATTERN" ./tests/api/crash/
else
    go test -v -timeout "$TIMEOUT" ./tests/api/crash/
fi

echo ""
echo "Crash tests completed!"
