#!/bin/bash

# Quick test of rapid renders with existing asset

INSTANCE_ID="test-visual-1"
STATES=("CREATED" "STARTING" "RUNNING" "STOPPING" "STOPPED")

echo "Testing rapid SVG renders with $INSTANCE_ID..."

for i in {1..30}; do
    STATE_IDX=$((i % 5))
    STATE=${STATES[$STATE_IDX]}

    RESULT=$(curl -s "http://localhost:8080/api/v1/visualize/asset/$INSTANCE_ID?format=svg&highlight_state=$STATE" 2>&1)

    if echo "$RESULT" | grep -q "<?xml"; then
        echo -n "."
    else
        echo -n "X"
        echo ""
        echo "Error: $RESULT" | head -3
    fi

    sleep 0.05
done

echo ""
echo ""
echo "✅ Test completed - 30 rapid renders!"
echo ""

# Check server health
HEALTH=$(curl -s http://localhost:8080/api/v1/health)
if echo "$HEALTH" | grep -q "healthy"; then
    echo "✅ Server is still healthy!"
else
    echo "❌ Server health check failed"
    exit 1
fi
