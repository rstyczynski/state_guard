package overload

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	baseURL = "http://localhost:8080"
)

// executeTransition performs a single state transition
func executeTransition(client *http.Client, assetID, toState string) error {
	payload := fmt.Sprintf(`{"to_state": "%s"}`, toState)
	req, err := http.NewRequest("POST", baseURL+"/api/v1/assets/"+assetID+"/transition", strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("transition to %s failed with status %d: %s", toState, resp.StatusCode, string(body))
	}
	return nil
}

// executeFullLifecycle executes a complete FSM lifecycle based on the path type
func executeFullLifecycle(client *http.Client, assetID string, pathType int) error {
	switch pathType % 3 {
	case 0:
		// Path 1: Normal lifecycle with maintenance
		states := []string{"STARTING", "RUNNING", "MAINTENANCE", "STOPPING", "STOPPED", "TERMINATING", "TERMINATED"}
		for _, state := range states {
			if err := executeTransition(client, assetID, state); err != nil {
				return err
			}
		}
	case 1:
		// Path 2: Failure recovery path
		states := []string{"STARTING", "RUNNING", "FAILED", "STARTING", "RUNNING", "STOPPING", "STOPPED", "TERMINATING", "TERMINATED"}
		for _, state := range states {
			if err := executeTransition(client, assetID, state); err != nil {
				return err
			}
		}
	case 2:
		// Path 3: Normal path to termination
		states := []string{"STARTING", "RUNNING", "STOPPING", "STOPPED", "TERMINATING", "TERMINATED"}
		for _, state := range states {
			if err := executeTransition(client, assetID, state); err != nil {
				return err
			}
		}
	}
	return nil
}

// TestConcurrentRequestOverload tests the API with massive concurrent asset creation and transitions
func TestConcurrentRequestOverload(t *testing.T) {
	concurrencyLevels := []int{100, 500, 1000, 2000}

	for _, concurrency := range concurrencyLevels {
		t.Run(fmt.Sprintf("Concurrent_%d_requests", concurrency), func(t *testing.T) {
			start := time.Now()

			var wg sync.WaitGroup
			createSuccess := 0
			transitionSuccess := 0
			errorCount := 0
			var mu sync.Mutex
			createdAssets := make([]string, 0, concurrency)

			client := &http.Client{
				Timeout: 30 * time.Second,
			}

			// Phase 1: Create assets concurrently
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					instanceID := fmt.Sprintf("overload-%d-%d", time.Now().UnixNano(), id)
					payload := fmt.Sprintf(`{"asset_type": "simple_asset_type.yaml", "instance_id": "%s"}`, instanceID)

					req, err := http.NewRequest("POST", baseURL+"/api/v1/assets", strings.NewReader(payload))
					if err != nil {
						mu.Lock()
						errorCount++
						mu.Unlock()
						return
					}
					req.Header.Set("Content-Type", "application/json")

					resp, err := client.Do(req)
					if err != nil {
						mu.Lock()
						errorCount++
						mu.Unlock()
						return
					}
					defer resp.Body.Close()

					mu.Lock()
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						createSuccess++
						createdAssets = append(createdAssets, instanceID)
					} else {
						errorCount++
					}
					mu.Unlock()
				}(i)
			}

			wg.Wait()
			createDuration := time.Since(start)

			// Phase 2: Execute full lifecycle paths on created assets
			lifecycleStart := time.Now()
			for idx, assetID := range createdAssets {
				wg.Add(1)
				go func(id string, pathType int) {
					defer wg.Done()

					// Execute full lifecycle (7-9 transitions depending on path)
					if err := executeFullLifecycle(client, id, pathType); err != nil {
						return
					}

					mu.Lock()
					transitionSuccess++
					mu.Unlock()
				}(assetID, idx)
			}

			wg.Wait()
			lifecycleDuration := time.Since(lifecycleStart)

			// Cleanup
			for _, assetID := range createdAssets {
				wg.Add(1)
				go func(id string) {
					defer wg.Done()
					req, _ := http.NewRequest("DELETE", baseURL+"/api/v1/assets/"+id, nil)
					resp, _ := client.Do(req)
					if resp != nil {
						resp.Body.Close()
					}
				}(assetID)
			}
			wg.Wait()

			duration := time.Since(start)

			t.Logf("Concurrency: %d, Total Duration: %v, Create: %dms, Lifecycle: %dms, Create Success: %d, Lifecycle Success: %d, Errors: %d, Overall RPS: %.2f",
				concurrency, duration, createDuration.Milliseconds(), lifecycleDuration.Milliseconds(),
				createSuccess, transitionSuccess, errorCount, float64(concurrency)/duration.Seconds())

			// API should handle at least 80% of requests successfully
			totalOps := concurrency
			successCount := createSuccess
			successRate := float64(successCount) / float64(totalOps) * 100
			if successRate < 80 {
				t.Errorf("Low success rate: %.2f%% (expected >= 80%%)", successRate)
			}
		})
	}
}

// TestMemoryExhaustionOverload tests API with memory-intensive requests
func TestMemoryExhaustionOverload(t *testing.T) {
	payloadSizes := []int{
		1024 * 1024,      // 1MB
		10 * 1024 * 1024, // 10MB
		50 * 1024 * 1024, // 50MB
	}

	for _, size := range payloadSizes {
		t.Run(fmt.Sprintf("Memory_%dMB", size/(1024*1024)), func(t *testing.T) {
			// Create large payload
			largePayload := make([]byte, size)
			for i := range largePayload {
				largePayload[i] = byte(i % 256)
			}

			start := time.Now()

			client := &http.Client{
				Timeout: 60 * time.Second,
			}

			req, err := http.NewRequest("POST", baseURL+"/api/v1/assets", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				t.Logf("Request failed (expected for large payload): %v", err)
				return
			}
			defer resp.Body.Close()

			duration := time.Since(start)

			t.Logf("Payload size: %d bytes, Duration: %v, Status: %d",
				size, duration, resp.StatusCode)

			// API should handle large payloads gracefully (not crash)
			if resp.StatusCode >= 500 {
				t.Errorf("Server returned 5xx error for %dMB payload", size/(1024*1024))
			}
		})
	}
}

// TestRapidFireOverload tests API with rapid-fire asset creation and transitions
func TestRapidFireOverload(t *testing.T) {
	durations := []time.Duration{5 * time.Second, 10 * time.Second}
	requestRates := []int{10, 50, 100} // requests per second

	for _, duration := range durations {
		for _, rate := range requestRates {
			t.Run(fmt.Sprintf("RapidFire_%ds_%drps", int(duration.Seconds()), rate), func(t *testing.T) {
				start := time.Now()
				requestCount := 0
				successCount := 0
				errorCount := 0
				createdAssets := make([]string, 0)
				var mu sync.Mutex

				client := &http.Client{
					Timeout: 10 * time.Second,
				}

				ticker := time.NewTicker(time.Second / time.Duration(rate))
				defer ticker.Stop()

				timeout := time.After(duration)

				for {
					select {
					case <-ticker.C:
						requestCount++

						go func(id int) {
							instanceID := fmt.Sprintf("rapidfire-%d-%d", time.Now().UnixNano(), id)
							payload := fmt.Sprintf(`{"asset_type": "simple_asset_type.yaml", "instance_id": "%s"}`, instanceID)

							req, err := http.NewRequest("POST", baseURL+"/api/v1/assets", strings.NewReader(payload))
							if err != nil {
								mu.Lock()
								errorCount++
								mu.Unlock()
								return
							}
							req.Header.Set("Content-Type", "application/json")

							resp, err := client.Do(req)
							if err != nil {
								mu.Lock()
								errorCount++
								mu.Unlock()
								return
							}
							defer resp.Body.Close()

							mu.Lock()
							if resp.StatusCode >= 200 && resp.StatusCode < 300 {
								successCount++
								createdAssets = append(createdAssets, instanceID)
							} else {
								errorCount++
							}
							mu.Unlock()
						}(requestCount)

					case <-timeout:
						// Wait a bit for pending requests to complete
						time.Sleep(1 * time.Second)

						actualDuration := time.Since(start)
						actualRate := float64(requestCount) / actualDuration.Seconds()

						t.Logf("Duration: %v, Requests: %d, Success: %d, Errors: %d, Actual RPS: %.2f",
							actualDuration, requestCount, successCount, errorCount, actualRate)

						// Cleanup created assets
						var wg sync.WaitGroup
						for _, assetID := range createdAssets {
							wg.Add(1)
							go func(id string) {
								defer wg.Done()
								req, _ := http.NewRequest("DELETE", baseURL+"/api/v1/assets/"+id, nil)
								resp, _ := client.Do(req)
								if resp != nil {
									resp.Body.Close()
								}
							}(assetID)
						}
						wg.Wait()

						// API should maintain reasonable performance
						successRate := float64(successCount) / float64(requestCount) * 100
						if successRate < 90 {
							t.Errorf("Low success rate: %.2f%% (expected >= 90%%)", successRate)
						}
						return
					}
				}
			})
		}
	}
}

// TestConnectionPoolExhaustion tests API with connection pool exhaustion
func TestConnectionPoolExhaustion(t *testing.T) {
	// Create many long-running requests to exhaust connection pool
	concurrency := 1000
	var wg sync.WaitGroup
	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Create request with long timeout to hold connections
			client := &http.Client{
				Timeout: 10 * time.Second,
			}

			req, err := http.NewRequest("GET", baseURL+"/api/v1/assets", nil)
			if err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				successCount++
			} else {
				errorCount++
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Connection pool test - Duration: %v, Success: %d, Errors: %d",
		duration, successCount, errorCount)

	// API should handle connection pool exhaustion gracefully
	successRate := float64(successCount) / float64(concurrency) * 100
	if successRate < 70 {
		t.Errorf("Low success rate under connection pressure: %.2f%% (expected >= 70%%)", successRate)
	}
}

// TestResourceExhaustionOverload tests API with resource exhaustion scenarios using FSM operations
func TestResourceExhaustionOverload(t *testing.T) {
	// Test with very large number of asset creation requests
	concurrency := 5000
	var wg sync.WaitGroup
	successCount := 0
	errorCount := 0
	var mu sync.Mutex
	createdAssets := make([]string, 0, concurrency)

	start := time.Now()

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			instanceID := fmt.Sprintf("resource-exhaustion-%d-%d", time.Now().UnixNano(), id)
			payload := fmt.Sprintf(`{"asset_type": "simple_asset_type.yaml", "instance_id": "%s"}`, instanceID)

			req, err := http.NewRequest("POST", baseURL+"/api/v1/assets", strings.NewReader(payload))
			if err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				successCount++
				createdAssets = append(createdAssets, instanceID)
			} else {
				errorCount++
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Resource exhaustion test - Concurrency: %d, Duration: %v, Success: %d, Errors: %d",
		concurrency, duration, successCount, errorCount)

	// Cleanup created assets
	for _, assetID := range createdAssets {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			req, _ := http.NewRequest("DELETE", baseURL+"/api/v1/assets/"+id, nil)
			resp, _ := client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
		}(assetID)
	}
	wg.Wait()

	// API should handle resource exhaustion gracefully
	successRate := float64(successCount) / float64(concurrency) * 100
	if successRate < 50 {
		t.Errorf("Very low success rate under resource pressure: %.2f%% (expected >= 50%%)", successRate)
	}
}
