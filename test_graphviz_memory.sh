#!/bin/bash

# Test script to simulate rapid timeline slider movements
# This should NOT crash the server with the GC fix in place

set -e

INSTANCE_ID="os1"
BASE_URL="http://localhost:8080"

echo "=========================================="
echo "GraphViz Memory Stress Test"
echo "=========================================="
echo ""
echo "This test simulates rapid timeline slider movements"
echo "by making many sequential visualization requests."
echo ""

# Get the states from the FSM definition
STATES=("CREATED" "STARTING" "RUNNING" "STOPPING" "STOPPED" "MAINTENANCE" "FAILED")

echo "Testing with instance: $INSTANCE_ID"
echo "States to cycle through: ${STATES[@]}"
echo ""

# Test 1: Rapid SVG renders (simulating slider)
echo "Test 1: Rapid SVG renders (50 requests)"
echo "Cycling through different highlighted states..."

SUCCESS_COUNT=0
ERROR_COUNT=0

for i in {1..50}; do
    # Cycle through states
    STATE=${STATES[$((i % ${#STATES[@]}))]}

    # Make request
    RESPONSE=$(curl -s "http://localhost:8080/api/v1/visualize/asset/$INSTANCE_ID?format=svg&highlight_state=$STATE" 2>&1)

    if echo "$RESPONSE" | grep -q "<?xml"; then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        echo -n "."
    else
        ERROR_COUNT=$((ERROR_COUNT + 1))
        echo -n "X"
        echo ""
        echo "Error on request $i (state=$STATE):"
        echo "$RESPONSE" | head -3
    fi

    # Small delay to simulate realistic usage
    sleep 0.1
done

echo ""
echo ""
echo "Results:"
echo "  ✅ Successful: $SUCCESS_COUNT"
echo "  ❌ Failed:     $ERROR_COUNT"
echo ""

if [ $ERROR_COUNT -gt 0 ]; then
    echo "❌ Test FAILED - Server crashed or returned errors"
    exit 1
fi

# Test 2: Check server is still responsive
echo "Test 2: Verify server is still healthy after stress test"
HEALTH=$(curl -s http://localhost:8080/api/v1/health)

if echo "$HEALTH" | grep -q "healthy"; then
    echo "  ✅ Server is still healthy!"
else
    echo "  ❌ Server is not responding"
    exit 1
fi

echo ""

# Test 3: Rapid PNG renders (heavier load)
echo "Test 3: Rapid PNG renders (20 requests)"
echo "Testing heavier memory load with PNG format..."

SUCCESS_COUNT=0
ERROR_COUNT=0

for i in {1..20}; do
    STATE=${STATES[$((i % ${#STATES[@]}))]}

    RESPONSE=$(curl -s "http://localhost:8080/api/v1/visualize/asset/$INSTANCE_ID?format=png&highlight_state=$STATE" 2>&1)

    if file <(echo "$RESPONSE") 2>/dev/null | grep -q "PNG"; then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        echo -n "."
    else
        ERROR_COUNT=$((ERROR_COUNT + 1))
        echo -n "X"
        echo ""
        echo "Error on PNG request $i (state=$STATE):"
        echo "$RESPONSE" | head -3
    fi

    sleep 0.2
done

echo ""
echo ""
echo "Results:"
echo "  ✅ Successful: $SUCCESS_COUNT"
echo "  ❌ Failed:     $ERROR_COUNT"
echo ""

if [ $ERROR_COUNT -gt 0 ]; then
    echo "❌ Test FAILED - PNG rendering crashed"
    exit 1
fi

# Final health check
echo "Final health check..."
HEALTH=$(curl -s http://localhost:8080/api/v1/health)

if echo "$HEALTH" | grep -q "healthy"; then
    echo "  ✅ Server survived all stress tests!"
else
    echo "  ❌ Server crashed"
    exit 1
fi

echo ""
echo "=========================================="
echo "✅ ALL TESTS PASSED!"
echo "=========================================="
echo ""
echo "The GraphViz memory fix is working:"
echo "  - Explicit Close() calls free resources immediately"
echo "  - runtime.GC() forces WASM memory cleanup"
echo "  - Rapid slider movements no longer crash the server"
echo ""
echo "You can now use the timeline slider extensively without crashes!"
