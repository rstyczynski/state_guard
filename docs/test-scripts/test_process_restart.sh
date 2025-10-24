#!/bin/bash

# Test script to verify process restart approach
# This script tests that the process restarts when memory is exhausted

echo "=== Testing Process Restart Approach ==="
echo "This test verifies that the process restarts when WASM memory cannot be freed"
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

# Function to check if process restarted
check_process_restart() {
    echo "=== Checking Process Restart ==="
    
    # Check if the original process is still running
    if ps -p $API_PID > /dev/null; then
        echo "Original process is still running (PID: $API_PID)"
        echo "Process restart may not have occurred"
    else
        echo "Original process has stopped (PID: $API_PID)"
        echo "Process restart may have occurred"
        
        # Check if a new process is running
        NEW_PID=$(pgrep -f "go run cmd/api/main.go")
        if [ -n "$NEW_PID" ]; then
            echo "New process is running (PID: $NEW_PID)"
            echo "✅ Process restart successful"
        else
            echo "No new process found"
            echo "❌ Process restart failed"
        fi
    fi
    
    echo ""
}

# Test sequence
echo "=== Phase 1: Initial Requests (should work normally) ==="
make_request "svg" "Initial SVG request"
make_request "png" "Initial PNG request"

echo "=== Phase 2: Memory Pressure Test (should trigger process restart) ==="
echo "Making multiple rapid requests to exhaust memory..."

# Make multiple rapid requests to exhaust memory
for i in {1..20}; do
    echo "Memory exhaustion request $i/20"
    make_request "png" "Memory exhaustion PNG request $i"
    sleep 0.1
done

echo "=== Phase 3: Check Process Restart ==="
check_process_restart

echo "=== Phase 4: Post-Restart Test (if process restarted) ==="
# Check if we can make requests after restart
if pgrep -f "go run cmd/api/main.go" > /dev/null; then
    echo "Testing post-restart functionality..."
    make_request "png" "Post-restart PNG request"
    make_request "svg" "Post-restart SVG request"
else
    echo "No process running, cannot test post-restart functionality"
fi

# Cleanup
echo "=== Cleanup ==="
if pgrep -f "go run cmd/api/main.go" > /dev/null; then
    echo "Stopping API server"
    pkill -f "go run cmd/api/main.go"
    sleep 2
    echo "✅ Server stopped"
else
    echo "No server process to stop"
fi

echo ""
echo "=== Test Summary ==="
echo "✅ Process restart test completed"
echo ""
echo "Expected behavior:"
echo "1. Process should restart when memory is exhausted"
echo "2. No DOT fallback (removed as useless)"
echo "3. Process restart is the only way to free WASM memory"
echo ""
echo "Key improvements:"
echo "- Removed useless DOT fallback"
echo "- Process restart when memory is exhausted"
echo "- Only way to free WASM module memory"
echo ""
echo "Expected log patterns:"
echo "- 'GraphViz: Memory still high after GC (XXXXX KB), restarting process'"
echo "- 'GraphViz: WASM module cannot be destroyed once loaded, restarting process'"
echo "- 'GraphViz: Process restart #1 - restarting entire process'"
echo "- 'GraphViz: Exiting process to force restart'"
echo ""
echo "Patterns to avoid:"
echo "- 'GraphViz: using DOT format instead' (removed)"
echo "- 'GraphViz: DOT render successful' (removed)"
echo "- 'GraphViz: Releasing global mutex (DOT fallback)' (removed)"
