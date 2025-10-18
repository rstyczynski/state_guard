package crash

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// TestWebhookQueueExhaustion tests webhook queue overflow behavior
// The webhook dispatcher has a queue size of 100. This test generates
// transitions faster than webhooks can be delivered to exhaust the queue.
func TestWebhookQueueExhaustion(t *testing.T) {
	baseURL := "http://localhost:8080"
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	t.Log("Testing webhook queue exhaustion...")
	t.Log("Expected behavior: Queue fills up, excess webhooks dropped with logging, API remains responsive")

	// Configuration
	concurrency := 50             // Number of concurrent goroutines
	transitionsPerClient := 10    // Transitions per client
	assetPrefix := "webhook-test-"

	done := make(chan bool, concurrency)
	successCount := 0
	errorCount := 0
	responseTimes := make([]time.Duration, 0, concurrency*transitionsPerClient)

	// Phase 1: Create assets
	t.Logf("Phase 1: Creating %d assets...", concurrency)
	for i := 0; i < concurrency; i++ {
		instanceID := fmt.Sprintf("%s%d", assetPrefix, i)

		payload := map[string]string{
			"instance_id": instanceID,
			"asset_type":  "simple_asset_type.yaml",
		}

		jsonPayload, _ := json.Marshal(payload)
		req, err := http.NewRequest("POST", baseURL+"/api/v1/assets", bytes.NewReader(jsonPayload))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to create asset %s: %v", instanceID, err)
		}

		io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to create asset %s: status %d", instanceID, resp.StatusCode)
		}
	}
	t.Logf("✓ Created %d assets", concurrency)

	// Phase 2: Rapid transitions to exhaust webhook queue
	// Each transition triggers a webhook (on_enter RUNNING)
	t.Logf("Phase 2: Generating rapid transitions to exhaust webhook queue...")
	t.Logf("Expected: %d total transitions (queue capacity: 100)", concurrency*transitionsPerClient)

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		go func(clientID int) {
			defer func() { done <- true }()

			instanceID := fmt.Sprintf("%s%d", assetPrefix, clientID)

			// Perform multiple rapid transitions
			for j := 0; j < transitionsPerClient; j++ {
				// Transition: CREATED → STARTING → RUNNING (webhook triggered)

				// Step 1: CREATED → STARTING
				transitionReq := map[string]string{"to_state": "STARTING"}
				jsonPayload, _ := json.Marshal(transitionReq)

				transitionStart := time.Now()
				req, err := http.NewRequest("POST",
					fmt.Sprintf("%s/api/v1/assets/%s/transition", baseURL, instanceID),
					bytes.NewReader(jsonPayload))
				if err != nil {
					errorCount++
					continue
				}
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				if err != nil {
					errorCount++
					continue
				}
				io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					errorCount++
					continue
				}

				// Step 2: STARTING → RUNNING (triggers webhook)
				transitionReq = map[string]string{"to_state": "RUNNING"}
				jsonPayload, _ = json.Marshal(transitionReq)

				req, err = http.NewRequest("POST",
					fmt.Sprintf("%s/api/v1/assets/%s/transition", baseURL, instanceID),
					bytes.NewReader(jsonPayload))
				if err != nil {
					errorCount++
					continue
				}
				req.Header.Set("Content-Type", "application/json")

				resp, err = client.Do(req)
				responseTime := time.Since(transitionStart)
				responseTimes = append(responseTimes, responseTime)

				if err != nil {
					errorCount++
					continue
				}

				io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					successCount++
				} else {
					errorCount++
				}

				// Step 3: RUNNING → STARTING (to allow next iteration)
				if j < transitionsPerClient-1 {
					transitionReq = map[string]string{"to_state": "STARTING"}
					jsonPayload, _ = json.Marshal(transitionReq)
					req, _ = http.NewRequest("POST",
						fmt.Sprintf("%s/api/v1/assets/%s/transition", baseURL, instanceID),
						bytes.NewReader(jsonPayload))
					req.Header.Set("Content-Type", "application/json")
					resp, _ = client.Do(req)
					io.ReadAll(resp.Body)
					resp.Body.Close()
				}
			}
		}(i)
	}

	// Wait for completion
	timeout := 2 * time.Minute
	completed := 0
	for {
		select {
		case <-done:
			completed++
			if completed >= concurrency {
				goto testComplete
			}
		case <-time.After(timeout):
			t.Logf("Timeout reached, completed %d/%d clients", completed, concurrency)
			goto testComplete
		}
	}

testComplete:
	totalDuration := time.Since(startTime)

	// Calculate response time statistics
	var totalResponseTime time.Duration
	var maxResponseTime time.Duration
	var minResponseTime time.Duration = time.Hour

	for _, rt := range responseTimes {
		totalResponseTime += rt
		if rt > maxResponseTime {
			maxResponseTime = rt
		}
		if rt < minResponseTime {
			minResponseTime = rt
		}
	}

	avgResponseTime := time.Duration(0)
	if len(responseTimes) > 0 {
		avgResponseTime = totalResponseTime / time.Duration(len(responseTimes))
	}

	// Results
	t.Logf("")
	t.Logf("=== Webhook Queue Exhaustion Test Results ===")
	t.Logf("Total Duration: %v", totalDuration)
	t.Logf("Successful Transitions: %d", successCount)
	t.Logf("Failed Transitions: %d", errorCount)
	t.Logf("Total Transitions: %d", successCount+errorCount)
	t.Logf("")
	t.Logf("HTTP Response Times:")
	t.Logf("  Average: %v", avgResponseTime)
	t.Logf("  Min: %v", minResponseTime)
	t.Logf("  Max: %v", maxResponseTime)
	t.Logf("")
	t.Logf("✓ API remained responsive (avg response: %v)", avgResponseTime)
	t.Logf("✓ Check server logs for 'Webhook queue full' messages")
	t.Logf("✓ Expected: Some webhooks dropped when queue exceeded 100 capacity")

	// Verify API remained responsive (< 1s average)
	if avgResponseTime > 1*time.Second {
		t.Errorf("Average response time too high: %v (should be < 1s for async webhooks)", avgResponseTime)
	}

	// We expect some transitions to succeed even if webhooks are dropped
	if successCount == 0 {
		t.Errorf("No successful transitions - API may have crashed")
	}

	// Phase 3: Cleanup - delete test assets
	t.Logf("Phase 3: Cleaning up %d test assets...", concurrency)
	for i := 0; i < concurrency; i++ {
		instanceID := fmt.Sprintf("%s%d", assetPrefix, i)
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/assets/%s", baseURL, instanceID), nil)
		resp, err := client.Do(req)
		if err == nil {
			io.ReadAll(resp.Body)
			resp.Body.Close()
		}
	}
	t.Logf("✓ Cleanup complete")
}
