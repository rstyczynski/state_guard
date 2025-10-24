#!/bin/bash

# Test script to verify highlight_current parameter works at all levels
# Usage: ./test_highlight_current.sh

set -e

echo "=== Verification Test: highlight_current Parameter ==="
echo ""

# Check if binaries exist
if [ ! -f bin/api ]; then
    echo "Error: bin/api not found. Please build first: go build -o bin/api cmd/api/main.go"
    exit 1
fi

if [ ! -f bin/fsm ]; then
    echo "Error: bin/fsm not found. Please build first: go build -o bin/fsm cmd/fsm/main.go"
    exit 1
fi

echo "✓ Binaries found"
echo ""

# Test 1: Verify parameter is documented in OpenAPI spec
echo "Test 1: Check OpenAPI spec includes highlight_current parameter"
if grep -q "highlight_current" docs/openapi.yaml; then
    echo "✓ highlight_current parameter found in OpenAPI spec"
else
    echo "✗ highlight_current parameter NOT found in OpenAPI spec"
    exit 1
fi
echo ""

# Test 2: Run unit tests
echo "Test 2: Run unit tests for parseOptions"
if go test ./internal/visualize/... -run TestParseOptions -v > /dev/null 2>&1; then
    echo "✓ Unit tests passed"
else
    echo "✗ Unit tests failed"
    exit 1
fi
echo ""

# Test 3: Check handlers.go includes the parameter parsing
echo "Test 3: Verify handlers.go parses highlight_current parameter"
if grep -q 'highlight_current' internal/visualize/handlers.go; then
    echo "✓ handlers.go includes highlight_current parsing logic"
else
    echo "✗ handlers.go does NOT include highlight_current parsing"
    exit 1
fi
echo ""

# Test 4: Check web UI includes the checkbox
echo "Test 4: Verify web UI includes Highlight Current State checkbox"
if grep -q 'id="highlight_current"' internal/visualize/handlers.go; then
    echo "✓ Web UI includes highlight_current checkbox"
else
    echo "✗ Web UI does NOT include highlight_current checkbox"
    exit 1
fi
echo ""

# Test 5: Check web UI JavaScript sends the parameter
echo "Test 5: Verify JavaScript sends highlight_current parameter"
if grep -q 'highlight_current: highlightCurrent.toString()' internal/visualize/handlers.go; then
    echo "✓ JavaScript sends highlight_current parameter to API"
else
    echo "✗ JavaScript does NOT send highlight_current parameter"
    exit 1
fi
echo ""

echo "=== All Verification Tests Passed ==="
echo ""
echo "To test manually:"
echo "1. Start the API server: ./bin/api --port 8080 --asset-dir examples/"
echo "2. Create an asset and navigate to: http://localhost:8080/docs/diagram/{instanceID}"
echo "3. Toggle the 'Highlight Current State' checkbox"
echo "4. Verify current state highlighting changes accordingly"
echo ""
echo "To test via API directly:"
echo "curl 'http://localhost:8080/api/v1/visualize/asset/{instanceID}?highlight_current=false&format=svg'"
echo "curl 'http://localhost:8080/api/v1/visualize/asset/{instanceID}?highlight_current=true&format=svg'"
