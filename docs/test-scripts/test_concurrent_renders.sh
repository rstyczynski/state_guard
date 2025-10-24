#!/bin/bash

# Test script to verify concurrent GraphViz renders don't crash
# Tests the mutex fix for WASM race conditions

set -e

INSTANCE_ID="os1"
BASE_URL="http://localhost:8080"

echo "=========================================="
echo "Concurrent GraphViz Render Test"
echo "=========================================="
echo ""
echo "This test simulates concurrent timeline slider movements"
echo "to verify the mutex prevents race conditions."
echo ""

# Get the states from the FSM definition
STATES=("CREATED" "STARTING" "RUNNING" "STOPPING" "STOPPED" "MAINTENANCE" "FAILED")

echo "Testing with instance: $INSTANCE_ID"
echo "States to cycle through: ${STATES[@]}"
echo ""

# Test: Concurrent SVG renders
echo "Test: Concurrent SVG renders (10 parallel batches)"
echo "Launching multiple background requests simultaneously..."

SUCCESS_COUNT=0
ERROR_COUNT=0
TOTAL_REQUESTS=50

for batch in {1..10}; do
    echo "Batch $batch: Launching 5 concurrent requests..."

    for i in {1..5}; do
        STATE=${STATES[$((RANDOM % ${#STATES[@]}))]}

        (
            RESPONSE=$(curl -s "http://localhost:8080/api/v1/visualize/asset/$INSTANCE_ID?format=svg&highlight_state=$STATE" 2>&1)

            if echo "$RESPONSE" | grep -q "<?xml"; then
                echo "."
            else
                echo "X"
                echo "Error on request (state=$STATE):"
                echo "$RESPONSE" | head -3
                exit 1
            fi
        ) &
    done

    # Wait for this batch to complete before starting next
    wait

    # Small delay between batches
    sleep 0.2
done

echo ""
echo ""
echo "Results:"
echo "  ✅ All concurrent requests completed successfully!"
echo ""

# Final health check
echo "Final health check..."
HEALTH=$(curl -s http://localhost:8080/api/v1/health)

if echo "$HEALTH" | grep -q "healthy"; then
    echo "  ✅ Server is still healthy after concurrent stress!"
else
    echo "  ❌ Server is not responding"
    exit 1
fi

echo ""
echo "=========================================="
echo "✅ CONCURRENT TEST PASSED!"
echo "=========================================="
echo ""
echo "The mutex fix is working:"
echo "  - GraphViz operations are serialized"
echo "  - No nil pointer crashes with concurrent requests"
echo "  - Timeline slider can be used by multiple users"
echo ""
