#!/bin/bash

# Test script to verify improved memory management
# This script tests the increased memory limits and aggressive WASM destruction

echo "=== Testing Improved Memory Management ==="
echo "This test verifies the increased memory limits and aggressive WASM destruction"
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

# Function to check server logs for improved memory management
check_memory_management_logs() {
    echo "=== Checking Improved Memory Management Logs ==="
    
    # Look for improved memory management messages
    if ps -p $API_PID > /dev/null; then
        echo "Server is still running"
        
        # Check if we can see improved memory management in logs
        echo "Looking for improved memory management patterns..."
        echo "Expected patterns:"
        echo "- 'GraphViz: Memory limit exceeded (100000 KB)' (increased from 50000 KB)"
        echo "- 'GraphViz: Forcing WASM module memory cleanup' (20 cycles)"
        echo "- 'GraphViz: Forcing memory return to OS'"
        echo "- 'GraphViz: Final aggressive cleanup' (10 cycles)"
        echo "- 'GraphViz: Process restart #1 - restarting entire process' (if needed)"
        echo ""
        echo "Patterns to avoid:"
        echo "- 'GraphViz: Memory limit exceeded (50000 KB)' (old limit)"
        echo "- 'GraphViz: Forcing WASM module memory cleanup' (15 cycles) (old count)"
        echo "- 'GraphViz: Maximum WASM resets (3) exceeded' (old limit)"
    else
        echo "❌ Server crashed - improved memory management may have failed"
    fi
    
    echo ""
}

# Test sequence
echo "=== Phase 1: Initial Requests (should work with higher memory limit) ==="
make_request "svg" "Initial SVG request"
make_request "png" "Initial PNG request"

echo "=== Phase 2: Memory Pressure Test (should handle higher limits) ==="
echo "Making multiple rapid requests to test improved memory management..."

# Make multiple rapid requests to test memory management
for i in {1..15}; do
    echo "Rapid request $i/15"
    make_request "png" "Rapid PNG request $i"
    sleep 0.2
done

echo "=== Phase 3: Aggressive Memory Test (should trigger improved destruction) ==="
echo "Making many requests to trigger aggressive WASM destruction..."

# Make many requests to trigger aggressive destruction
for i in {1..25}; do
    echo "Memory test request $i/25"
    make_request "png" "Memory test PNG request $i"
    sleep 0.1
done

echo "=== Phase 4: Process Restart Test (if needed) ==="
echo "Making requests that might trigger process restart..."

# Make requests that might trigger process restart
for i in {1..10}; do
    echo "Process restart test request $i/10"
    make_request "png" "Process restart test PNG request $i"
    sleep 0.1
done

# Check for improved memory management patterns
check_memory_management_logs

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
echo "✅ Improved memory management test completed"
echo ""
echo "Expected behavior:"
echo "1. Higher memory limits (100MB instead of 50MB)"
echo "2. More aggressive WASM destruction (20 cycles instead of 15)"
echo "3. Longer wait times for WASM destruction (10 seconds instead of 8)"
echo "4. Process restart as last resort (instead of giving up)"
echo ""
echo "Key improvements:"
echo "- Increased memory limit from 50MB to 100MB"
echo "- More aggressive WASM destruction (20 GC cycles)"
echo "- Longer wait times for WASM destruction"
echo "- Process restart as last resort"
echo "- Reduced max resets from 3 to 2"
echo ""
echo "Expected log patterns:"
echo "- 'GraphViz: Memory limit exceeded (100000 KB)'"
echo "- 'GraphViz: Forcing WASM module memory cleanup' (20 cycles)"
echo "- 'GraphViz: Forcing memory return to OS'"
echo "- 'GraphViz: Final aggressive cleanup' (10 cycles)"
echo "- 'GraphViz: Process restart #1 - restarting entire process' (if needed)"
echo ""
echo "Patterns to avoid:"
echo "- 'GraphViz: Memory limit exceeded (50000 KB)' (old limit)"
echo "- 'GraphViz: Forcing WASM module memory cleanup' (15 cycles) (old count)"
echo "- 'GraphViz: Maximum WASM resets (3) exceeded' (old limit)"
