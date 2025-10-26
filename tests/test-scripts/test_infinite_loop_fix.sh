#!/bin/bash

# Test script to verify infinite loop fix
# This should show that after 3 WASM resets, the system falls back to DOT format
# instead of continuing the infinite loop

echo "Testing infinite loop fix..."

# Start the API server in background
echo "Starting API server..."
go run cmd/api/main.go &
API_PID=$!

# Wait for server to start
sleep 3

echo "Making PNG requests to trigger WASM memory exhaustion..."
echo "Expected behavior:"
echo "1. First few requests: WASM reset attempts"
echo "2. After 3 resets: Fall back to DOT format"
echo "3. No more infinite loops"

# Test PNG requests to trigger WASM memory exhaustion
for i in {1..2}; do
    echo "Starting request $i"
    curl -s "http://localhost:8080/api/v1/visualize/asset/os1?format=png" > /dev/null &
done

# Wait for all requests to complete
wait

echo ""
echo "Check the logs above for:"
echo "1. 'Forced WASM reset #1', '#2', '#3' messages"
echo "2. 'Maximum WASM resets (3) exceeded, giving up and falling back to DOT format' message"
echo "3. No more infinite reset loops after 3 attempts"

# Clean up
echo "Stopping API server..."
kill $API_PID

echo "Test completed."
