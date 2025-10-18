package crash

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestBasicCrashTests runs basic crash tests against the FSM API
func TestBasicCrashTests(t *testing.T) {
	baseURL := "http://localhost:8080"
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	tests := []struct {
		name     string
		endpoint string
		method   string
		payload  string
	}{
		{
			name:     "Health check",
			endpoint: "/api/v1/health",
			method:   "GET",
		},
		{
			name:     "List assets",
			endpoint: "/api/v1/assets",
			method:   "GET",
		},
		{
			name:     "Create asset with malformed JSON",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": "examples/simple_asset_type.yaml", "instance_id": "crash-test-1"`, // Missing closing brace
		},
		{
			name:     "Create asset with invalid JSON",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": "examples/simple_asset_type.yaml", "instance_id": "crash-test-2",}`, // Trailing comma
		},
		{
			name:     "Create asset with empty payload",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  "",
		},
		{
			name:     "Create asset with null payload",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  "null",
		},
		{
			name:     "Create asset with missing fields",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": "examples/simple_asset_type.yaml"}`, // Missing instance_id
		},
		{
			name:     "Create asset with wrong data types",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": 123, "instance_id": true}`,
		},
		{
			name:     "Get non-existent asset",
			endpoint: "/api/v1/assets/non-existent-id",
			method:   "GET",
		},
		{
			name:     "Get asset with path traversal",
			endpoint: "/api/v1/assets/../../../etc/passwd",
			method:   "GET",
		},
		{
			name:     "Get asset with SQL injection",
			endpoint: "/api/v1/assets/1' OR '1'='1",
			method:   "GET",
		},
		{
			name:     "Get asset with XSS",
			endpoint: "/api/v1/assets/<script>alert('xss')</script>",
			method:   "GET",
		},
		{
			name:     "Transition with malformed JSON",
			endpoint: "/api/v1/assets/web-server-1/transition",
			method:   "POST",
			payload:  `{"to_state": "RUNNING"`, // Missing closing brace
		},
		{
			name:     "Transition with empty payload",
			endpoint: "/api/v1/assets/web-server-1/transition",
			method:   "POST",
			payload:  "",
		},
		{
			name:     "Transition with missing to_state",
			endpoint: "/api/v1/assets/web-server-1/transition",
			method:   "POST",
			payload:  `{}`,
		},
		{
			name:     "Transition with wrong data type",
			endpoint: "/api/v1/assets/web-server-1/transition",
			method:   "POST",
			payload:  `{"to_state": 123}`,
		},
		{
			name:     "History with negative limit",
			endpoint: "/api/v1/assets/web-server-1/history?limit=-1",
			method:   "GET",
		},
		{
			name:     "History with non-numeric limit",
			endpoint: "/api/v1/assets/web-server-1/history?limit=not_a_number",
			method:   "GET",
		},
		{
			name:     "History with very large limit",
			endpoint: "/api/v1/assets/web-server-1/history?limit=999999999",
			method:   "GET",
		},
		{
			name:     "Invalid HTTP method on health",
			endpoint: "/api/v1/health",
			method:   "PUT",
		},
		{
			name:     "Invalid HTTP method on assets",
			endpoint: "/api/v1/assets",
			method:   "PATCH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.payload != "" {
				body = strings.NewReader(tt.payload)
			}

			req, err := http.NewRequest(tt.method, baseURL+tt.endpoint, body)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			if tt.method == "POST" && tt.payload != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Logf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			responseBody, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(responseBody))

			// API should handle these cases gracefully
			if resp.StatusCode >= 500 {
				t.Logf("⚠️  SECURITY ISSUE: Server returned 5xx error for: %s", tt.name)
			}

			// Check for specific expected behaviors
			switch tt.name {
			case "Health check":
				if resp.StatusCode != 200 {
					t.Errorf("Health check should return 200, got %d", resp.StatusCode)
				}
			case "Get non-existent asset":
				if resp.StatusCode != 404 {
					t.Errorf("Non-existent asset should return 404, got %d", resp.StatusCode)
				}
			case "Invalid HTTP method on health", "Invalid HTTP method on assets":
				if resp.StatusCode != 405 {
					t.Errorf("Invalid HTTP method should return 405, got %d", resp.StatusCode)
				}
			}
		})
	}
}

// executeTransition performs a single state transition
func executeTransition(client *http.Client, baseURL, assetID, toState string) error {
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

// TestConcurrentRequests tests concurrent request handling with realistic FSM lifecycle paths
func TestConcurrentRequests(t *testing.T) {
	baseURL := "http://localhost:8080"
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	concurrency := 50
	done := make(chan bool, concurrency)
	createdAssets := make(chan string, concurrency)

	// Create assets concurrently
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()

			instanceID := fmt.Sprintf("crash-concurrent-%d-%d", time.Now().Unix(), id)
			payload := fmt.Sprintf(`{"asset_type": "simple_asset_type.yaml", "instance_id": "%s"}`, instanceID)

			req, err := http.NewRequest("POST", baseURL+"/api/v1/assets", strings.NewReader(payload))
			if err != nil {
				t.Errorf("Failed to create request: %v", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == 201 || resp.StatusCode == 200 {
				createdAssets <- instanceID
			} else {
				body, _ := io.ReadAll(resp.Body)
				t.Logf("Asset creation failed with status %d: %s", resp.StatusCode, string(body))
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < concurrency; i++ {
		<-done
	}
	close(createdAssets)

	// Collect created assets
	assetList := make([]string, 0)
	for asset := range createdAssets {
		assetList = append(assetList, asset)
	}

	t.Logf("Successfully created %d assets", len(assetList))

	// Execute realistic FSM lifecycle paths concurrently
	done2 := make(chan bool, len(assetList))

	for i, assetID := range assetList {
		go func(id string, index int) {
			defer func() { done2 <- true }()

			// Different lifecycle paths based on index
			switch index % 3 {
			case 0:
				// Path 1: Normal lifecycle with maintenance
				// CREATED → STARTING → RUNNING → MAINTENANCE → RUNNING → STOPPING → STOPPED → TERMINATING → TERMINATED
				if err := executeTransition(client, baseURL, id, "STARTING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "RUNNING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "MAINTENANCE"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "STOPPING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "STOPPED"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "TERMINATING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "TERMINATED"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}

			case 1:
				// Path 2: Failure recovery path
				// CREATED → STARTING → RUNNING → FAILED → STARTING → RUNNING → STOPPING → STOPPED → TERMINATING → TERMINATED
				if err := executeTransition(client, baseURL, id, "STARTING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "RUNNING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				// Simulate failure
				if err := executeTransition(client, baseURL, id, "FAILED"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				// Recover from failure
				if err := executeTransition(client, baseURL, id, "STARTING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "RUNNING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "STOPPING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "STOPPED"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "TERMINATING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "TERMINATED"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}

			case 2:
				// Path 3: Simple path to termination
				// CREATED → STARTING → RUNNING → STOPPING → STOPPED → TERMINATING → TERMINATED
				if err := executeTransition(client, baseURL, id, "STARTING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "RUNNING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "STOPPING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "STOPPED"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "TERMINATING"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)

				if err := executeTransition(client, baseURL, id, "TERMINATED"); err != nil {
					t.Logf("Asset %s: %v", id, err)
					return
				}
			}
		}(assetID, i)
	}

	// Wait for all lifecycle paths to complete
	for i := 0; i < len(assetList); i++ {
		<-done2
	}

	t.Logf("Completed lifecycle paths for %d assets", len(assetList))

	// Cleanup: Delete all created assets
	done3 := make(chan bool, len(assetList))
	for _, assetID := range assetList {
		go func(id string) {
			defer func() { done3 <- true }()

			req, err := http.NewRequest("DELETE", baseURL+"/api/v1/assets/"+id, nil)
			if err != nil {
				t.Logf("Failed to create delete request: %v", err)
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Logf("Delete failed: %v", err)
				return
			}
			defer resp.Body.Close()
		}(assetID)
	}

	// Wait for all deletions
	for i := 0; i < len(assetList); i++ {
		<-done3
	}

	t.Logf("Cleaned up %d assets", len(assetList))
}

// TestLargePayloads tests handling of large payloads
func TestLargePayloads(t *testing.T) {
	baseURL := "http://localhost:8080"
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"asset_type": "examples/simple_asset_type.yaml", "instance_id": "%s"}`, tt.payload)

			req, err := http.NewRequest("POST", baseURL+"/api/v1/assets", strings.NewReader(payload))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			startTime := time.Now()
			resp, err := client.Do(req)
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
				t.Logf("⚠️  SECURITY ISSUE: Server returned 5xx error for large payload")
			}
		})
	}
}

// TestSecurityAttacks tests various security attack vectors
func TestSecurityAttacks(t *testing.T) {
	baseURL := "http://localhost:8080"
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Test path traversal attacks
	maliciousIDs := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\drivers\\etc\\hosts",
		"../../../../../../etc/shadow",
		"..%2F..%2F..%2Fetc%2Fpasswd",
		"....//....//....//etc//passwd",
	}

	for _, maliciousID := range maliciousIDs {
		t.Run(fmt.Sprintf("PathTraversal_%s", maliciousID), func(t *testing.T) {
			req, err := http.NewRequest("GET", baseURL+"/api/v1/assets/"+maliciousID, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Logf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

			// Should return 404 or 400, not expose file contents
			if resp.StatusCode >= 500 {
				t.Logf("⚠️  SECURITY ISSUE: Server returned 5xx error for path traversal attempt")
			}

			// Check if file contents are exposed
			if strings.Contains(string(body), "root:") || strings.Contains(string(body), "localhost") {
				t.Logf("⚠️  SECURITY ISSUE: File contents may be exposed for path: %s", maliciousID)
			}
		})
	}

	// Test SQL injection attacks
	sqlPayloads := []string{
		"1' OR '1'='1",
		"1' UNION SELECT * FROM instances --",
		"1' OR 1=1 --",
		"1' OR 'x'='x",
	}

	for _, payload := range sqlPayloads {
		t.Run(fmt.Sprintf("SQLInjection_%s", payload), func(t *testing.T) {
			req, err := http.NewRequest("GET", baseURL+"/api/v1/assets/"+payload, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Logf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

			// Should return 404 or 400, not execute SQL
			if resp.StatusCode >= 500 {
				t.Logf("⚠️  SECURITY ISSUE: Server returned 5xx error for SQL injection attempt")
			}
		})
	}

	// Test XSS attacks
	xssPayloads := []string{
		"<script>alert('XSS')</script>",
		"javascript:alert('XSS')",
		"<img src=x onerror=alert('XSS')>",
		"<svg onload=alert('XSS')>",
	}

	for _, payload := range xssPayloads {
		t.Run(fmt.Sprintf("XSS_%s", payload), func(t *testing.T) {
			// Test in URL parameter
			req, err := http.NewRequest("GET", baseURL+"/api/v1/assets/"+payload, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Logf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

			// Check if XSS payload is reflected in response
			if strings.Contains(string(body), payload) {
				t.Logf("⚠️  SECURITY ISSUE: XSS payload reflected in response: %s", payload)
			}
		})
	}
}
