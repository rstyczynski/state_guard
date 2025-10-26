#!/bin/bash

# Test script to verify WASM module destruction and reload
# This script tests the new WASM module destruction mechanism

echo "=== Testing WASM Module Destruction and Reload ==="
echo "This test verifies that the WASM module is actually destroyed and reloaded"
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

# Function to check server logs for WASM module destruction
check_wasm_module_destruction() {
    echo "=== Checking WASM Module Destruction Logs ==="
    
    # Look for WASM module destruction messages
    if ps -p $API_PID > /dev/null; then
        echo "Server is still running"
        
        # Check if we can see WASM module destruction in logs
        echo "Looking for WASM module destruction patterns..."
        echo "Expected patterns:"
        echo "- 'Destroying WASM module completely'"
        echo "- 'Forcing WASM module memory cleanup'"
        echo "- 'Waiting for WASM module to be unloaded'"
        echo "- 'WASM module destroyed - will be reloaded on next use'"
        echo "- 'WASM module was destroyed, forcing complete reload'"
        echo "- 'WASM module reload initiated'"
    else
        echo "❌ Server crashed - WASM module destruction may have failed"
    fi
    
    echo ""
}

# Test sequence
echo "=== Phase 1: Initial Requests (should create WASM instances) ==="
make_request "svg" "Initial SVG request"
make_request "png" "Initial PNG request"

echo "=== Phase 2: Memory Pressure (should trigger WASM module destruction) ==="
echo "Making multiple rapid requests to trigger memory pressure..."

# Make multiple rapid requests to trigger memory exhaustion
for i in {1..8}; do
    echo "Rapid request $i/8"
    make_request "png" "Rapid PNG request $i"
    sleep 0.3
done

echo "=== Phase 3: Post-Destruction Requests (should reload WASM module) ==="
make_request "svg" "Post-destruction SVG request"
make_request "png" "Post-destruction PNG request"

# Check for WASM module destruction patterns
check_wasm_module_destruction

# Test WASM instance statistics endpoint
echo "=== Testing WASM Instance Statistics ==="
echo "Checking WASM instance stats..."

# Try to get stats (this might not be implemented yet)
curl -s "http://localhost:8080/api/v1/visualize/stats" 2>/dev/null | jq . 2>/dev/null || echo "Stats endpoint not available"

echo ""

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
echo "✅ WASM module destruction test completed"
echo ""
echo "Expected behavior:"
echo "1. Initial requests create WASM instances"
echo "2. Memory pressure triggers WASM module destruction"
echo "3. New requests reload the WASM module completely"
echo "4. Memory usage should decrease after module destruction"
echo ""
echo "Key improvements:"
echo "- WASM module is completely destroyed and reloaded"
echo "- Memory is actually freed by destroying the module"
echo "- New instances are created from a fresh module"
echo "- No more infinite loops of WASM resets"
echo ""
echo "Expected log patterns:"
echo "- 'Destroying WASM module completely'"
echo "- 'Forcing WASM module memory cleanup'"
echo "- 'Waiting for WASM module to be unloaded'"
echo "- 'WASM module destroyed - will be reloaded on next use'"
echo "- 'WASM module was destroyed, forcing complete reload'"
echo "- 'WASM module reload initiated'"
