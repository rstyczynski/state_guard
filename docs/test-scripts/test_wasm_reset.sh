#!/bin/bash

# Test script to verify WASM reset functionality
# This should show that when WASM memory is exhausted, the system resets and retries
# instead of falling back to DOT format

echo "Testing WASM reset functionality..."

# Start the API server in background
echo "Starting API server..."
go run cmd/api/main.go &
API_PID=$!

# Wait for server to start
sleep 3

echo "Making concurrent PNG requests to trigger WASM memory exhaustion..."
echo "Expected behavior:"
echo "1. First request hits memory limit, marks WASM as exhausted"
echo "2. Subsequent requests detect exhausted WASM and force reset"
echo "3. Requests retry with fresh WASM state"
echo "4. Browser receives PNG/SVG format, not DOT format"

# Test concurrent PNG requests to trigger WASM memory exhaustion
for i in {1..3}; do
    echo "Starting request $i"
    curl -s "http://localhost:8080/api/v1/visualize/asset/os1?format=png" > /dev/null &
done

# Wait for all requests to complete
wait

echo ""
echo "Check the logs above for:"
echo "1. 'WASM memory exhausted, marking for reset' messages"
echo "2. 'WASM memory exhausted, forcing complete reset' messages"
echo "3. 'Forced WASM reset - cleared all generators' messages"
echo "4. 'Retrying generation after WASM reset' messages"
echo "5. No 'falling back to DOT format' messages"

# Clean up
echo "Stopping API server..."
kill $API_PID

echo "Test completed."
