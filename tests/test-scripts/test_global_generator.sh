#!/bin/bash

# Test script to verify global generator works correctly
# This should show that concurrent requests now use the same generator instance

echo "Testing global generator with concurrent requests..."

# Start the API server in background
echo "Starting API server..."
go run cmd/api/main.go &
API_PID=$!

# Wait for server to start
sleep 3

# Test concurrent requests to the same asset type
echo "Making concurrent requests to same asset type..."
for i in {1..5}; do
    curl -s "http://localhost:8080/api/v1/visualize/asset/os1?format=svg" > /dev/null &
done

# Wait for all requests to complete
wait

echo "Checking generator cache stats..."
curl -s "http://localhost:8080/api/v1/visualize/asset/os1?format=dot" | head -20

# Clean up
echo "Stopping API server..."
kill $API_PID

echo "Test completed. Check logs for 'Created global generator' messages."
echo "You should see only ONE 'Created global generator' message for the same asset type."
