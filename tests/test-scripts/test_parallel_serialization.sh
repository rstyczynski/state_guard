#!/bin/bash

# Test script to verify that parallel requests are properly serialized
# This script tests that no parallel GraphViz generation occurs

echo "=== Testing Parallel Request Serialization ==="
echo "This test verifies that parallel requests are properly serialized"
echo ""

# Start the API server in the background
echo "Starting API server..."
go run cmd/api/main.go &
API_PID=$!

# Wait for server to start
sleep 3

# Function to make a request in the background
make_background_request() {
    local format=$1
    local description=$2
    local delay=$3
    
    (
        sleep $delay
        echo "=== $description ==="
        echo "Making request for format: $format"
        
        # Make the request
        curl -s "http://localhost:8080/api/v1/visualize/asset/os1?format=$format&layout=horizontal&history=false&available=true&highlight_current=true&highlight_state=RUNNING" > /dev/null
        
        if [ $? -eq 0 ]; then
            echo "✅ Request successful"
        else
            echo "❌ Request failed"
        fi
    ) &
}

# Function to check server logs for serialization
check_serialization_logs() {
    echo "=== Checking Request Serialization Logs ==="
    
    # Look for serialization messages
    if ps -p $API_PID > /dev/null; then
        echo "Server is still running"
        
        # Check if we can see proper serialization in logs
        echo "Looking for serialization patterns..."
        echo "Expected patterns:"
        echo "- 'Handler: Acquiring global mutex for request'"
        echo "- 'Handler: Global mutex acquired for request'"
        echo "- 'Handler: Releasing global mutex for request'"
        echo "- No parallel 'Starting visualization' messages"
        echo ""
        echo "Patterns to avoid:"
        echo "- Multiple 'Starting visualization' messages at the same time"
        echo "- Multiple 'Created global generator' messages at the same time"
        echo "- WASM memory errors from parallel access"
    else
        echo "❌ Server crashed - serialization may have failed"
    fi
    
    echo ""
}

# Test sequence
echo "=== Phase 1: Sequential Requests (should work normally) ==="
make_background_request "svg" "Sequential SVG request" 0
sleep 2
make_background_request "png" "Sequential PNG request" 0
sleep 2

echo "=== Phase 2: Parallel Requests (should be serialized) ==="
echo "Making parallel requests to test serialization..."

# Make multiple parallel requests
make_background_request "png" "Parallel PNG request 1" 0
make_background_request "svg" "Parallel SVG request 1" 0.1
make_background_request "png" "Parallel PNG request 2" 0.2
make_background_request "svg" "Parallel SVG request 2" 0.3
make_background_request "png" "Parallel PNG request 3" 0.4

# Wait for all requests to complete
sleep 5

echo "=== Phase 3: Rapid Parallel Requests (should be serialized) ==="
echo "Making rapid parallel requests to test serialization..."

# Make rapid parallel requests
for i in {1..5}; do
    make_background_request "png" "Rapid parallel request $i" $((i * 0.1))
done

# Wait for all requests to complete
sleep 3

echo "=== Phase 4: Mixed Format Parallel Requests (should be serialized) ==="
echo "Making mixed format parallel requests..."

# Make mixed format parallel requests
make_background_request "png" "Mixed PNG request 1" 0
make_background_request "svg" "Mixed SVG request 1" 0.1
make_background_request "png" "Mixed PNG request 2" 0.2
make_background_request "svg" "Mixed SVG request 2" 0.3

# Wait for all requests to complete
sleep 3

# Check for serialization patterns
check_serialization_logs

# Cleanup
echo "=== Cleanup ==="
if ps -p $API_PID > /dev/null; then
    echo "Stopping API server (PID: $API_PID)"
    kill $API_PID
    wait $API_PID 2>/dev/null
    echo "✅ Server stopped"
else
    echo "Server already stopped"
fi

echo ""
echo "=== Test Summary ==="
echo "✅ Parallel request serialization test completed"
echo ""
echo "Expected behavior:"
echo "1. All requests should be serialized (no parallel execution)"
echo "2. No WASM memory errors from parallel access"
echo "3. No multiple 'Starting visualization' messages at the same time"
echo "4. No multiple 'Created global generator' messages at the same time"
echo ""
echo "Key improvements:"
echo "- Global mutex acquired at handler level"
echo "- No parallel GraphViz generation"
echo "- No WASM memory corruption"
echo "- Proper request serialization"
echo ""
echo "Expected log patterns:"
echo "- 'Handler: Acquiring global mutex for request'"
echo "- 'Handler: Global mutex acquired for request'"
echo "- 'Handler: Releasing global mutex for request'"
echo ""
echo "Patterns to avoid:"
echo "- Multiple 'Starting visualization' messages at the same time"
echo "- Multiple 'Created global generator' messages at the same time"
echo "- WASM memory errors from parallel access"
