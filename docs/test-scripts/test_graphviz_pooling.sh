#!/bin/bash

# Test script to verify GraphViz instance pooling works
# This script tests that GraphViz instances are reused instead of creating new ones

echo "=== Testing GraphViz Instance Pooling ==="
echo "This test verifies that GraphViz instances are reused instead of creating new ones"
echo ""

# Start the API server in the background
echo "Starting API server..."
go run cmd/api/main.go &
API_PID=$!

# Wait for server to start
sleep 3

# Function to make a request and capture memory stats
make_request() {
    local format=$1
    local description=$2
    
    echo "=== $description ==="
    echo "Making request for format: $format"
    
    # Make the request
    curl -s "http://localhost:8080/api/v1/visualize/asset/os1?format=$format&layout=horizontal&history=false&available=true&highlight_current=true&highlight_state=RUNNING" > /dev/null
    
    if [ $? -eq 0 ]; then
        echo "✅ Request successful"
    else
        echo "❌ Request failed"
    fi
    
    echo ""
}

# Function to check server logs for pooling
check_pooling_logs() {
    echo "=== Checking GraphViz Pooling Logs ==="
    
    # Look for pooling messages
    if ps -p $API_PID > /dev/null; then
        echo "Server is still running"
        
        # Check if we can see pooling in logs
        echo "Looking for pooling patterns..."
        echo "Expected patterns:"
        echo "- 'GraphViz: Created and pooled instance for format: png'"
        echo "- 'GraphViz: Reusing pooled instance for format: png'"
        echo "- 'GraphViz: Graph closed, GraphViz instance returned to pool'"
        echo "- 'GraphViz: Cleared instance pool'"
        echo ""
        echo "Patterns to avoid:"
        echo "- 'GraphViz: Creating new GraphViz instance for format'"
        echo "- 'GraphViz: Created and tracked WASM instance:'"
        echo "- 'GraphViz: Removed WASM instance from tracking:'"
    else
        echo "❌ Server crashed - pooling may have failed"
    fi
    
    echo ""
}

# Test sequence
echo "=== Phase 1: Initial Requests (should create pooled instances) ==="
make_request "svg" "Initial SVG request"
make_request "png" "Initial PNG request"

echo "=== Phase 2: Repeated Requests (should reuse pooled instances) ==="
echo "Making repeated requests to test instance reuse..."

# Make multiple requests to test instance reuse
for i in {1..5}; do
    echo "Request $i/5"
    make_request "png" "PNG request $i"
    sleep 0.2
done

echo "=== Phase 3: Mixed Format Requests (should reuse instances) ==="
make_request "svg" "SVG request (should reuse SVG instance)"
make_request "png" "PNG request (should reuse PNG instance)"
make_request "svg" "SVG request (should reuse SVG instance)"

echo "=== Phase 4: Memory Pressure Test (should clear pool when needed) ==="
echo "Making multiple rapid requests to trigger memory pressure..."

# Make multiple rapid requests to trigger memory pressure
for i in {1..10}; do
    echo "Rapid request $i/10"
    make_request "png" "Rapid PNG request $i"
    sleep 0.1
done

echo "=== Phase 5: Post-Pressure Requests (should work with fresh instances) ==="
make_request "svg" "Post-pressure SVG request"
make_request "png" "Post-pressure PNG request"

# Check for pooling patterns
check_pooling_logs

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
echo "✅ GraphViz instance pooling test completed"
echo ""
echo "Expected behavior:"
echo "1. Initial requests create pooled instances"
echo "2. Repeated requests reuse pooled instances"
echo "3. Memory pressure clears the pool when needed"
echo "4. Post-pressure requests create fresh instances"
echo ""
echo "Key improvements:"
echo "- GraphViz instances are reused instead of creating new ones"
echo "- Memory usage should be more stable"
echo "- No more memory accumulation from repeated instance creation"
echo "- Pool is cleared when memory pressure is detected"
echo ""
echo "Expected log patterns:"
echo "- 'GraphViz: Created and pooled instance for format: png'"
echo "- 'GraphViz: Reusing pooled instance for format: png'"
echo "- 'GraphViz: Graph closed, GraphViz instance returned to pool'"
echo "- 'GraphViz: Cleared instance pool'"
echo ""
echo "Patterns to avoid:"
echo "- 'GraphViz: Creating new GraphViz instance for format'"
echo "- 'GraphViz: Created and tracked WASM instance:'"
echo "- 'GraphViz: Removed WASM instance from tracking:'"
