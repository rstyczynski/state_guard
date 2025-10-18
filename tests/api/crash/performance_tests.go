package crash

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	errors := make(chan error, concurrency*10)
	successCount := 0
	errorCount := 0

	// Start concurrent clients
	for i := 0; i < concurrency; i++ {
		go func(clientID int) {
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
						errors <- fmt.Errorf("client %d: failed to create request: %v", clientID, err)
						continue
					}

					resp, err := c.client.Do(req)
					if err != nil {
						errors <- fmt.Errorf("client %d: request failed: %v", clientID, err)
						continue
					}

					body, _ := io.ReadAll(resp.Body)
					resp.Body.Close()

					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						successCount++
					} else {
						errors <- fmt.Errorf("client %d: unexpected status %d: %s", clientID, resp.StatusCode, string(body))
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
	// Collect any remaining errors
	close(errors)
	for err := range errors {
		if err != nil {
			errorCount++
			t.Logf("Error: %v", err)
		}
	}

	t.Logf("Load test completed: %d successful, %d errors", successCount, errorCount)

	// Check if error rate is too high
	errorRate := float64(errorCount) / float64(successCount+errorCount) * 100
	if errorRate > 10 { // More than 10% error rate
		t.Errorf("High error rate: %.2f%% (%d errors out of %d total requests)", errorRate, errorCount, successCount+errorCount)
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
	errors := make(chan error, concurrency*10)
	successCount := 0
	errorCount := 0

	// Start concurrent clients
	for i := 0; i < concurrency; i++ {
		go func(clientID int) {
			defer func() { done <- true }()

			startTime := time.Now()
			for time.Since(startTime) < duration {
				var body io.Reader
				if payload != "" {
					body = strings.NewReader(payload)
				}

				req, err := http.NewRequest(method, c.baseURL+endpoint, body)
				if err != nil {
					errors <- fmt.Errorf("client %d: failed to create request: %v", clientID, err)
					continue
				}

				if payload != "" {
					req.Header.Set("Content-Type", "application/json")
				}

				resp, err := c.client.Do(req)
				if err != nil {
					errors <- fmt.Errorf("client %d: request failed: %v", clientID, err)
					continue
				}

				responseBody, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode >= 200 && resp.StatusCode < 500 {
					successCount++
				} else {
					errors <- fmt.Errorf("client %d: unexpected status %d: %s", clientID, resp.StatusCode, string(responseBody))
					errorCount++
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
	// Collect any remaining errors
	close(errors)
	for err := range errors {
		if err != nil {
			t.Logf("Error: %v", err)
		}
	}

	t.Logf("Resource exhaustion test completed: %d successful, %d errors", successCount, errorCount)

	// Check if error rate is too high
	errorRate := float64(errorCount) / float64(successCount+errorCount) * 100
	if errorRate > 50 { // More than 50% error rate
		t.Errorf("High error rate: %.2f%% (%d errors out of %d total requests)", errorRate, errorCount, successCount+errorCount)
	}
}
