#!/bin/bash

# Test script to verify memory fallback works correctly
# This should show that when memory is high, requests fall back to DOT format
# and subsequent requests also use DOT format instead of trying PNG

echo "Testing memory fallback with high memory usage..."

# Start the API server in background
echo "Starting API server..."
go run cmd/api/main.go &
API_PID=$!

# Wait for server to start
sleep 3

echo "Making concurrent PNG requests to trigger memory fallback..."
echo "Expected behavior:"
echo "1. First request hits memory limit, falls back to DOT"
echo "2. Subsequent requests should also use DOT format"
echo "3. No more PNG attempts after memory limit is reached"

# Test concurrent PNG requests to trigger memory fallback
for i in {1..3}; do
    echo "Starting request $i"
    curl -s "http://localhost:8080/api/v1/visualize/asset/os1?format=png" > /dev/null &
done

# Wait for all requests to complete
wait

echo ""
echo "Check the logs above for:"
echo "1. 'Memory limit exceeded' messages"
echo "2. 'falling back to DOT format' messages"
echo "3. 'Acquiring global mutex for DOT format (fallback)' messages"
echo "4. No more 'Acquiring global mutex for format png' after fallback"

# Clean up
echo "Stopping API server..."
kill $API_PID

echo "Test completed."
