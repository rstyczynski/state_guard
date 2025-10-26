#!/bin/bash

# Test script to verify global mutex is working correctly
# This should show that requests are properly serialized

echo "Testing global mutex with concurrent PNG requests..."

# Start the API server in background
echo "Starting API server..."
go run cmd/api/main.go &
API_PID=$!

# Wait for server to start
sleep 3

echo "Making 5 concurrent PNG requests to same asset type..."
echo "Expected behavior: Requests should be serialized, not concurrent"

# Test concurrent PNG requests to the same asset type
for i in {1..5}; do
    echo "Starting request $i"
    curl -s "http://localhost:8080/api/v1/visualize/asset/os1?format=png" > /dev/null &
done

# Wait for all requests to complete
wait

echo ""
echo "Check the logs above for:"
echo "1. 'GraphViz: Acquiring global mutex for format png' messages"
echo "2. 'GraphViz: Global mutex acquired for format png' messages" 
echo "3. 'GraphViz: Releasing global mutex for format png' messages"
echo ""
echo "These should appear in sequence, not simultaneously!"

# Clean up
echo "Stopping API server..."
kill $API_PID

echo "Test completed."
