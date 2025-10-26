package crash

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPerformanceUnderLoad tests API performance under various load conditions
func (c *CrashTestSuite) TestPerformanceUnderLoad(t *testing.T) {
	loadTests := []struct {
		name        string
		concurrency int
		duration    time.Duration
		requestRate int // requests per second
	}{
		{
			name:        "Light Load",
			concurrency: 10,
			duration:    10 * time.Second,
			requestRate: 5,
		},
		{
			name:        "Medium Load",
			concurrency: 50,
			duration:    30 * time.Second,
			requestRate: 20,
		},
		{
			name:        "Heavy Load",
			concurrency: 100,
			duration:    60 * time.Second,
			requestRate: 50,
		},
		{
			name:        "Extreme Load",
			concurrency: 500,
			duration:    120 * time.Second,
			requestRate: 100,
		},
	}

	for _, lt := range loadTests {
		t.Run(lt.name, func(t *testing.T) {
			c.runLoadTest(t, lt.name, lt.concurrency, lt.duration, lt.requestRate)
		})
	}
}

// runLoadTest executes a load test with the given parameters
func (c *CrashTestSuite) runLoadTest(t *testing.T, name string, concurrency int, duration time.Duration, requestRate int) {
	t.Logf("Starting %s test: %d concurrent clients, %v duration, %d req/s", name, concurrency, duration, requestRate)

	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency*100) // Larger buffer to prevent blocking
	var successCount, errorCount int32 // Use atomic counters
	var wg sync.WaitGroup

	// Start concurrent clients
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			defer func() { done <- true }()

			// Calculate request interval based on rate
			interval := time.Duration(1000/requestRate) * time.Millisecond
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			startTime := time.Now()
			for time.Since(startTime) < duration {
				select {
				case <-ticker.C:
					// Make a request
					req, err := http.NewRequest("GET", c.baseURL+"/api/v1/health", nil)
					if err != nil {
						select {
						case errors <- fmt.Errorf("client %d: failed to create request: %v", clientID, err):
						default: // Don't block if channel is full
						}
						continue
					}

					resp, err := c.client.Do(req)
					if err != nil {
						select {
						case errors <- fmt.Errorf("client %d: request failed: %v", clientID, err):
						default: // Don't block if channel is full
						}
						continue
					}

					body, _ := io.ReadAll(resp.Body)
					resp.Body.Close()

					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						atomic.AddInt32(&successCount, 1)
					} else {
						atomic.AddInt32(&errorCount, 1)
						select {
						case errors <- fmt.Errorf("client %d: unexpected status %d: %s", clientID, resp.StatusCode, string(body)):
						default: // Don't block if channel is full
						}
					}
				}
			}
		}(i)
	}

	// Wait for all clients to complete or timeout
	timeout := duration + 30*time.Second
	completed := 0
	for {
		select {
		case <-done:
			completed++
			if completed >= concurrency {
				goto collectErrors
			}
		case <-time.After(timeout):
			t.Logf("Timeout reached, completed %d/%d clients", completed, concurrency)
			goto collectErrors
		}
	}

collectErrors:
	// Wait for all goroutines to finish before closing channel
	wg.Wait()
	close(errors)

	// Collect any remaining errors
	var errCount int32
	for err := range errors {
		if err != nil {
			errCount++
			if errCount <= 10 { // Only log first 10 errors to avoid spam
				t.Logf("Error: %v", err)
			}
		}
	}

	finalSuccessCount := atomic.LoadInt32(&successCount)
	finalErrorCount := atomic.LoadInt32(&errorCount)
	t.Logf("Load test completed: %d successful, %d errors", finalSuccessCount, finalErrorCount)

	// Check if error rate is too high
	totalRequests := finalSuccessCount + finalErrorCount
	if totalRequests > 0 {
		errorRate := float64(finalErrorCount) / float64(totalRequests) * 100
		if errorRate > 10 { // More than 10% error rate
			t.Errorf("High error rate: %.2f%% (%d errors out of %d total requests)", errorRate, finalErrorCount, totalRequests)
		}
	}
}

// TestMemoryLeaks tests for memory leaks under sustained load
func (c *CrashTestSuite) TestMemoryLeaks(t *testing.T) {
	t.Log("Testing for memory leaks under sustained load...")

	// Run sustained load for 5 minutes
	duration := 5 * time.Minute
	concurrency := 20
	done := make(chan bool, concurrency)

	startTime := time.Now()
	requestCount := 0

	for i := 0; i < concurrency; i++ {
		go func(clientID int) {
			defer func() { done <- true }()

			for time.Since(startTime) < duration {
				// Alternate between different endpoints
				endpoints := []string{
					"/api/v1/health",
					"/api/v1/assets",
				}

				for _, endpoint := range endpoints {
					req, err := http.NewRequest("GET", c.baseURL+endpoint, nil)
					if err != nil {
						continue
					}

					resp, err := c.client.Do(req)
					if err != nil {
						continue
					}

					// Read response body to ensure it's fully consumed
					io.ReadAll(resp.Body)
					resp.Body.Close()
					requestCount++
				}

				// Small delay to prevent overwhelming the server
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}

	// Wait for completion
	timeout := duration + 30*time.Second
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
	t.Logf("Memory leak test completed: %d requests processed", requestCount)
}

// TestConnectionPoolExhaustion tests connection pool exhaustion
func (c *CrashTestSuite) TestConnectionPoolExhaustion(t *testing.T) {
	t.Log("Testing connection pool exhaustion...")

	// Create many concurrent connections without properly closing them
	concurrency := 1000
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(clientID int) {
			defer func() { done <- true }()

			// Create a new client for each request to exhaust connections
			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			req, err := http.NewRequest("GET", c.baseURL+"/api/v1/health", nil)
			if err != nil {
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				return
			}

			// Don't close the response body to exhaust connections
			// This is intentionally bad practice to test server resilience
			_ = resp
		}(i)
	}

	// Wait for completion
	timeout := 30 * time.Second
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
	t.Logf("Connection pool exhaustion test completed: %d requests processed", completed)
}

// TestLargePayloadHandling tests handling of large payloads
func (c *CrashTestSuite) TestLargePayloadHandling(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		payload string
	}{
		{
			name:    "1MB Payload",
			size:    1024 * 1024,
			payload: strings.Repeat("x", 1024*1024),
		},
		{
			name:    "10MB Payload",
			size:    10 * 1024 * 1024,
			payload: strings.Repeat("x", 10*1024*1024),
		},
		{
			name:    "100MB Payload",
			size:    100 * 1024 * 1024,
			payload: strings.Repeat("x", 100*1024*1024),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test large asset creation
			payload := map[string]string{
				"asset_type":  "simple_asset_type.yaml",
				"instance_id": tt.payload,
			}

			jsonPayload, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("Failed to marshal payload: %v", err)
			}

			req, err := http.NewRequest("POST", c.baseURL+"/api/v1/assets", bytes.NewReader(jsonPayload))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			startTime := time.Now()
			resp, err := c.client.Do(req)
			duration := time.Since(startTime)

			if err != nil {
				t.Logf("Request failed (expected for large payload): %v", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Duration: %v, Body length: %d", resp.StatusCode, duration, len(body))

			// API should handle large payloads gracefully
			if resp.StatusCode >= 500 {
				t.Errorf("Server returned 5xx error for large payload")
			}

			// Check if response time is reasonable
			if duration > 30*time.Second {
				t.Errorf("Response time too long: %v", duration)
			}
		})
	}
}

// TestSlowLoris tests SlowLoris attack (slow HTTP requests)
func (c *CrashTestSuite) TestSlowLoris(t *testing.T) {
	t.Log("Testing SlowLoris attack...")

	concurrency := 100
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(clientID int) {
			defer func() { done <- true }()

			// Create a slow request that sends data very slowly
			req, err := http.NewRequest("POST", c.baseURL+"/api/v1/assets", &slowReader{
				data:  `{"asset_type": "simple_asset_type.yaml", "instance_id": "slow-loris-` + fmt.Sprintf("%d", clientID) + `"}`,
				delay: 1 * time.Second,
			})
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")

			// Set a long timeout
			client := &http.Client{
				Timeout: 60 * time.Second,
			}

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			io.ReadAll(resp.Body)
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
	t.Logf("SlowLoris test completed: %d requests processed", completed)
}

// slowReader implements io.Reader that reads data very slowly
type slowReader struct {
	data  string
	pos   int
	delay time.Duration
}

func (sr *slowReader) Read(p []byte) (n int, err error) {
	if sr.pos >= len(sr.data) {
		return 0, io.EOF
	}

	// Read one byte at a time
	n = 1
	p[0] = sr.data[sr.pos]
	sr.pos++

	// Add delay between reads
	time.Sleep(sr.delay)

	return n, nil
}

// TestResourceExhaustion tests various resource exhaustion scenarios
func (c *CrashTestSuite) TestResourceExhaustion(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		duration    time.Duration
		endpoint    string
		method      string
		payload     string
	}{
		{
			name:        "Health Endpoint Spam",
			concurrency: 1000,
			duration:    30 * time.Second,
			endpoint:    "/api/v1/health",
			method:      "GET",
		},
		{
			name:        "Assets List Spam",
			concurrency: 500,
			duration:    30 * time.Second,
			endpoint:    "/api/v1/assets",
			method:      "GET",
		},
		{
			name:        "Asset Creation Spam",
			concurrency: 100,
			duration:    30 * time.Second,
			endpoint:    "/api/v1/assets",
			method:      "POST",
			payload:     `{"asset_type": "simple_asset_type.yaml", "instance_id": "spam-test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.runResourceExhaustionTest(t, tt.name, tt.concurrency, tt.duration, tt.endpoint, tt.method, tt.payload)
		})
	}
}

// runResourceExhaustionTest executes a resource exhaustion test
func (c *CrashTestSuite) runResourceExhaustionTest(t *testing.T, name string, concurrency int, duration time.Duration, endpoint, method, payload string) {
	t.Logf("Starting %s: %d concurrent clients, %v duration", name, concurrency, duration)

	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency*100) // Larger buffer to prevent blocking
	var successCount, errorCount int32 // Use atomic counters
	var wg sync.WaitGroup

	// Start concurrent clients
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			defer func() { done <- true }()

			startTime := time.Now()
			for time.Since(startTime) < duration {
				var body io.Reader
				if payload != "" {
					body = strings.NewReader(payload)
				}

				req, err := http.NewRequest(method, c.baseURL+endpoint, body)
				if err != nil {
					select {
					case errors <- fmt.Errorf("client %d: failed to create request: %v", clientID, err):
					default: // Don't block if channel is full
					}
					continue
				}

				if payload != "" {
					req.Header.Set("Content-Type", "application/json")
				}

				resp, err := c.client.Do(req)
				if err != nil {
					select {
					case errors <- fmt.Errorf("client %d: request failed: %v", clientID, err):
					default: // Don't block if channel is full
					}
					continue
				}

				responseBody, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode >= 200 && resp.StatusCode < 500 {
					atomic.AddInt32(&successCount, 1)
				} else {
					atomic.AddInt32(&errorCount, 1)
					select {
					case errors <- fmt.Errorf("client %d: unexpected status %d: %s", clientID, resp.StatusCode, string(responseBody)):
					default: // Don't block if channel is full
					}
				}

				// Small delay to prevent overwhelming the server
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	// Wait for all clients to complete or timeout
	timeout := duration + 30*time.Second
	completed := 0
	for {
		select {
		case <-done:
			completed++
			if completed >= concurrency {
				goto collectErrors
			}
		case <-time.After(timeout):
			t.Logf("Timeout reached, completed %d/%d clients", completed, concurrency)
			goto collectErrors
		}
	}

collectErrors:
	// Wait for all goroutines to finish before closing channel
	wg.Wait()
	close(errors)

	// Collect any remaining errors
	var errCount int32
	for err := range errors {
		if err != nil {
			errCount++
			if errCount <= 10 { // Only log first 10 errors to avoid spam
				t.Logf("Error: %v", err)
			}
		}
	}

	finalSuccessCount := atomic.LoadInt32(&successCount)
	finalErrorCount := atomic.LoadInt32(&errorCount)
	t.Logf("Resource exhaustion test completed: %d successful, %d errors", finalSuccessCount, finalErrorCount)

	// Check if error rate is too high
	totalRequests := finalSuccessCount + finalErrorCount
	if totalRequests > 0 {
		errorRate := float64(finalErrorCount) / float64(totalRequests) * 100
		if errorRate > 50 { // More than 50% error rate
			t.Errorf("High error rate: %.2f%% (%d errors out of %d total requests)", errorRate, finalErrorCount, totalRequests)
		}
	}
}

// TestRapidVisualizationRenders tests GraphViz WASM memory handling under rapid sequential renders
// This simulates timeline slider movements which previously caused WASM memory exhaustion
func (c *CrashTestSuite) TestRapidVisualizationRenders(t *testing.T) {
	// First, create a test asset with history
	instanceID := fmt.Sprintf("rapid-viz-test-%d", time.Now().Unix())

	// Create asset
	createPayload := map[string]string{
		"asset_type":  "simple_asset_type.yaml",
		"instance_id": instanceID,
	}
	jsonPayload, _ := json.Marshal(createPayload)

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/assets", bytes.NewReader(jsonPayload))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create asset: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create asset: status %d, body: %s", resp.StatusCode, string(body))
	}

	t.Logf("Created test asset: %s", instanceID)

	// Transition through several states to build history
	states := []string{"STARTING", "RUNNING", "STOPPING", "STOPPED", "STARTING", "RUNNING"}
	for _, state := range states {
		transitionPayload := map[string]string{"to_state": state}
		jsonPayload, _ := json.Marshal(transitionPayload)

		req, _ := http.NewRequest("POST", c.baseURL+"/api/v1/assets/"+instanceID+"/transition", bytes.NewReader(jsonPayload))
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			t.Logf("Warning: transition to %s failed: %v", state, err)
			continue
		}
		resp.Body.Close()
		time.Sleep(50 * time.Millisecond) // Small delay between transitions
	}

	t.Logf("Asset %s has history, starting visualization stress tests", instanceID)

	// Test scenarios
	tests := []struct {
		name        string
		format      string
		count       int
		delay       time.Duration
		expectError bool
	}{
		{
			name:        "Rapid SVG renders (timeline slider simulation)",
			format:      "svg",
			count:       100,
			delay:       50 * time.Millisecond,
			expectError: false,
		},
		{
			name:        "Rapid PNG renders (heavier load)",
			format:      "png",
			count:       50,
			delay:       100 * time.Millisecond,
			expectError: false,
		},
		{
			name:        "Burst SVG renders (no delay)",
			format:      "svg",
			count:       50,
			delay:       0,
			expectError: false,
		},
		{
			name:        "Mixed format renders",
			format:      "mixed", // Special flag to alternate formats
			count:       60,
			delay:       50 * time.Millisecond,
			expectError: false,
		},
	}

	// States to cycle through for timeline slider simulation
	highlightStates := []string{"CREATED", "STARTING", "RUNNING", "STOPPING", "STOPPED", "MAINTENANCE", "FAILED"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			successCount := 0
			errorCount := 0
			wasmErrors := 0

			t.Logf("Starting %s: %d requests with %v delay", tt.name, tt.count, tt.delay)

			for i := 0; i < tt.count; i++ {
				// Cycle through states to simulate timeline slider
				highlightState := highlightStates[i%len(highlightStates)]

				// Determine format
				format := tt.format
				if format == "mixed" {
					if i%2 == 0 {
						format = "svg"
					} else {
						format = "png"
					}
				}

				// Build URL with highlight_state parameter (timeline slider)
				url := fmt.Sprintf("%s/api/v1/visualize/asset/%s?format=%s&highlight_state=%s&highlight_current=true",
					c.baseURL, instanceID, format, highlightState)

				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					errorCount++
					continue
				}

				resp, err := c.client.Do(req)
				if err != nil {
					errorCount++
					t.Logf("Request %d failed: %v", i+1, err)
					continue
				}

				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					// Check for WASM errors in response
					if strings.Contains(string(body), "wasm error") ||
						strings.Contains(string(body), "out of bounds memory access") {
						wasmErrors++
						errorCount++
						t.Logf("WASM error detected in request %d", i+1)
					} else {
						successCount++
					}
				} else {
					errorCount++
					if strings.Contains(string(body), "wasm") {
						wasmErrors++
					}
				}

				// Delay between requests
				if tt.delay > 0 {
					time.Sleep(tt.delay)
				}
			}

			t.Logf("Results for %s: %d successful, %d errors, %d WASM errors",
				tt.name, successCount, errorCount, wasmErrors)

			// CRITICAL: WASM errors indicate the memory fix isn't working
			if wasmErrors > 0 {
				t.Errorf("WASM memory errors detected: %d/%d requests failed with WASM errors", wasmErrors, tt.count)
				t.Error("This indicates GraphViz memory cleanup is not working properly!")
			}

			// Calculate error rate
			if successCount+errorCount > 0 {
				errorRate := float64(errorCount) / float64(successCount+errorCount) * 100

				// For visualization, we expect near-zero errors
				if errorRate > 5 && !tt.expectError {
					t.Errorf("High error rate: %.2f%% (%d errors out of %d requests)",
						errorRate, errorCount, tt.count)
				}
			}
		})
	}

	// Cleanup: delete the test asset
	req, _ = http.NewRequest("DELETE", c.baseURL+"/api/v1/assets/"+instanceID, nil)
	resp, err = c.client.Do(req)
	if err == nil {
		resp.Body.Close()
	}

	t.Logf("Cleaned up test asset: %s", instanceID)
}

// TestConcurrentVisualizationRequests tests GraphViz mutex protection under truly concurrent requests
// This verifies that the mutex prevents WASM race conditions when multiple users view visualizations simultaneously
func (c *CrashTestSuite) TestConcurrentVisualizationRequests(t *testing.T) {
	// Create test asset with state transitions
	instanceID := fmt.Sprintf("concurrent-viz-test-%d", time.Now().Unix())
	createPayload := map[string]string{
		"asset_type":  "simple_asset_type.yaml",
		"instance_id": instanceID,
	}

	jsonPayload, err := json.Marshal(createPayload)
	if err != nil {
		t.Fatalf("Failed to marshal create payload: %v", err)
	}

	resp, err := c.client.Post(c.baseURL+"/api/v1/assets", "application/json", bytes.NewReader(jsonPayload))
	if err != nil {
		t.Fatalf("Failed to create asset: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", resp.StatusCode)
	}

	t.Logf("Created test asset: %s", instanceID)

	// Create some state history
	states := []string{"STARTING", "RUNNING", "STOPPING", "STOPPED", "STARTING", "RUNNING"}
	for _, state := range states {
		transitionPayload := map[string]string{"new_state": state}
		jsonPayload, _ := json.Marshal(transitionPayload)

		resp, err := c.client.Post(
			c.baseURL+"/api/v1/assets/"+instanceID+"/transition",
			"application/json",
			bytes.NewReader(jsonPayload),
		)
		if err == nil {
			resp.Body.Close()
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Logf("Asset %s has history, starting concurrent visualization tests", instanceID)

	// Define test scenarios
	scenarios := []struct {
		name        string
		concurrency int
		format      string
	}{
		{"Concurrent SVG renders", 50, "svg"},
		{"Concurrent PNG renders", 30, "png"},
		{"Mixed format concurrent", 40, "mixed"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("Starting %s: %d concurrent requests", scenario.name, scenario.concurrency)

			var wg sync.WaitGroup
			successChan := make(chan bool, scenario.concurrency)
			errorChan := make(chan error, scenario.concurrency)
			wasmErrorChan := make(chan bool, scenario.concurrency)

			// Launch concurrent requests
			for i := 0; i < scenario.concurrency; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()

					// Vary the highlight_state
					highlightStates := []string{"CREATED", "STARTING", "RUNNING", "STOPPING", "STOPPED", "MAINTENANCE", "FAILED"}
					highlightState := highlightStates[idx%len(highlightStates)]

					// Determine format
					format := scenario.format
					if format == "mixed" {
						if idx%2 == 0 {
							format = "svg"
						} else {
							format = "png"
						}
					}

					url := fmt.Sprintf("%s/api/v1/visualize/asset/%s?format=%s&highlight_state=%s&highlight_current=true",
						c.baseURL, instanceID, format, highlightState)

					resp, err := c.client.Get(url)
					if err != nil {
						errorChan <- fmt.Errorf("request %d failed: %w", idx, err)
						return
					}
					defer resp.Body.Close()

					body, err := io.ReadAll(resp.Body)
					if err != nil {
						errorChan <- fmt.Errorf("read body failed: %w", err)
						return
					}

					// Check for WASM errors
					bodyStr := string(body)
					if strings.Contains(bodyStr, "wasm") || strings.Contains(bodyStr, "nil pointer") || strings.Contains(bodyStr, "out of bounds") {
						wasmErrorChan <- true
						errorChan <- fmt.Errorf("WASM error in response %d: %s", idx, bodyStr[:min(200, len(bodyStr))])
						return
					}

					// Check for valid response based on format
					if format == "svg" {
						if !strings.Contains(bodyStr, "<?xml") {
							errorChan <- fmt.Errorf("invalid SVG response %d", idx)
							return
						}
					} else if format == "png" {
						if len(body) < 100 || !bytes.HasPrefix(body, []byte{0x89, 0x50, 0x4E, 0x47}) {
							errorChan <- fmt.Errorf("invalid PNG response %d", idx)
							return
						}
					}

					if resp.StatusCode != http.StatusOK {
						errorChan <- fmt.Errorf("request %d: status %d", idx, resp.StatusCode)
						return
					}

					successChan <- true
				}(i)
			}

			// Wait for all requests to complete
			wg.Wait()
			close(successChan)
			close(errorChan)
			close(wasmErrorChan)

			// Count results
			successCount := len(successChan)
			errorCount := len(errorChan)
			wasmErrorCount := len(wasmErrorChan)

			t.Logf("Results for %s: %d successful, %d errors, %d WASM errors",
				scenario.name, successCount, errorCount, wasmErrorCount)

			// Report all errors
			for err := range errorChan {
				t.Logf("  Error: %v", err)
			}

			// Fail if WASM errors occurred (mutex should prevent these)
			if wasmErrorCount > 0 {
				t.Errorf("CRITICAL: %d WASM errors detected - mutex is not working!", wasmErrorCount)
			}

			// Most requests should succeed
			if successCount < scenario.concurrency*8/10 {
				t.Errorf("Too many failures: %d/%d succeeded", successCount, scenario.concurrency)
			}
		})
	}

	// Clean up
	req, _ := http.NewRequest("DELETE", c.baseURL+"/api/v1/assets/"+instanceID, nil)
	resp, err = c.client.Do(req)
	if err == nil {
		resp.Body.Close()
	}

	t.Logf("Cleaned up test asset: %s", instanceID)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Wrapper test functions to invoke the suite methods

// TestPerformanceUnderLoadWrapper runs the performance under load tests
func TestPerformanceUnderLoadWrapper(t *testing.T) {
	suite := NewCrashTestSuite("http://localhost:8080")
	suite.TestPerformanceUnderLoad(t)
}

// TestMemoryLeaksWrapper runs the memory leak tests
func TestMemoryLeaksWrapper(t *testing.T) {
	suite := NewCrashTestSuite("http://localhost:8080")
	suite.TestMemoryLeaks(t)
}

// TestConnectionPoolExhaustionWrapper runs the connection pool exhaustion tests
func TestConnectionPoolExhaustionWrapper(t *testing.T) {
	suite := NewCrashTestSuite("http://localhost:8080")
	suite.TestConnectionPoolExhaustion(t)
}

// TestLargePayloadHandlingWrapper runs the large payload handling tests
func TestLargePayloadHandlingWrapper(t *testing.T) {
	suite := NewCrashTestSuite("http://localhost:8080")
	suite.TestLargePayloadHandling(t)
}

// TestSlowLorisWrapper runs the SlowLoris attack tests
func TestSlowLorisWrapper(t *testing.T) {
	suite := NewCrashTestSuite("http://localhost:8080")
	suite.TestSlowLoris(t)
}

// TestResourceExhaustionWrapper runs the resource exhaustion tests
func TestResourceExhaustionWrapper(t *testing.T) {
	suite := NewCrashTestSuite("http://localhost:8080")
	suite.TestResourceExhaustion(t)
}

// TestRapidVisualizationRendersWrapper runs the rapid visualization render tests
func TestRapidVisualizationRendersWrapper(t *testing.T) {
	suite := NewCrashTestSuite("http://localhost:8080")
	suite.TestRapidVisualizationRenders(t)
}
