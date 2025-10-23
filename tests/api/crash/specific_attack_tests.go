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

// TestSpecificAPIEndpoints tests specific vulnerabilities for each API endpoint
func (c *CrashTestSuite) TestSpecificAPIEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		method   string
		payload  string
		headers  map[string]string
	}{
		// Health endpoint tests
		{
			name:     "Health endpoint with malformed query",
			endpoint: "/api/v1/health?malicious=../../etc/passwd",
			method:   "GET",
		},
		{
			name:     "Health endpoint with SQL injection",
			endpoint: "/api/v1/health?id=1' OR '1'='1",
			method:   "GET",
		},

		// Assets endpoint tests
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
			name:     "Create asset with array payload",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `[]`,
		},
		{
			name:     "Create asset with string payload",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `"just a string"`,
		},
		{
			name:     "Create asset with number payload",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `12345`,
		},
		{
			name:     "Create asset with boolean payload",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `true`,
		},
		{
			name:     "Create asset with missing asset_type",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"instance_id": "test1"}`,
		},
		{
			name:     "Create asset with missing instance_id",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": "test.yaml"}`,
		},
		{
			name:     "Create asset with empty asset_type",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": "", "instance_id": "test1"}`,
		},
		{
			name:     "Create asset with empty instance_id",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": "test.yaml", "instance_id": ""}`,
		},
		{
			name:     "Create asset with null asset_type",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": null, "instance_id": "test1"}`,
		},
		{
			name:     "Create asset with null instance_id",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": "test.yaml", "instance_id": null}`,
		},
		{
			name:     "Create asset with numeric asset_type",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": 123, "instance_id": "test1"}`,
		},
		{
			name:     "Create asset with numeric instance_id",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": "test.yaml", "instance_id": 123}`,
		},
		{
			name:     "Create asset with boolean asset_type",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": true, "instance_id": "test1"}`,
		},
		{
			name:     "Create asset with boolean instance_id",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": "test.yaml", "instance_id": true}`,
		},
		{
			name:     "Create asset with array asset_type",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": [], "instance_id": "test1"}`,
		},
		{
			name:     "Create asset with array instance_id",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": "test.yaml", "instance_id": []}`,
		},
		{
			name:     "Create asset with object asset_type",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": {}, "instance_id": "test1"}`,
		},
		{
			name:     "Create asset with object instance_id",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  `{"asset_type": "test.yaml", "instance_id": {}}`,
		},

		// Asset-specific endpoint tests
		{
			name:     "Get asset with empty ID",
			endpoint: "/api/v1/assets/",
			method:   "GET",
		},
		{
			name:     "Get asset with very long ID",
			endpoint: fmt.Sprintf("/api/v1/assets/%s", strings.Repeat("a", 100000)),
			method:   "GET",
		},
		{
			name:     "Get asset with SQL injection ID",
			endpoint: "/api/v1/assets/1' OR '1'='1",
			method:   "GET",
		},
		{
			name:     "Get asset with path traversal ID",
			endpoint: "/api/v1/assets/../../../etc/passwd",
			method:   "GET",
		},
		{
			name:     "Get asset with XSS ID",
			endpoint: "/api/v1/assets/<script>alert('xss')</script>",
			method:   "GET",
		},

		// Transition endpoint tests
		{
			name:     "Transition with empty payload",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  "",
		},
		{
			name:     "Transition with null payload",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  "null",
		},
		{
			name:     "Transition with missing to_state",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  `{}`,
		},
		{
			name:     "Transition with empty to_state",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  `{"to_state": ""}`,
		},
		{
			name:     "Transition with null to_state",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  `{"to_state": null}`,
		},
		{
			name:     "Transition with numeric to_state",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  `{"to_state": 123}`,
		},
		{
			name:     "Transition with boolean to_state",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  `{"to_state": true}`,
		},
		{
			name:     "Transition with array to_state",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  `{"to_state": []}`,
		},
		{
			name:     "Transition with object to_state",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  `{"to_state": {}}`,
		},
		{
			name:     "Transition with very long to_state",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  fmt.Sprintf(`{"to_state": "%s"}`, strings.Repeat("x", 100000)),
		},
		{
			name:     "Transition with SQL injection to_state",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  `{"to_state": "RUNNING'; DROP TABLE instances; --"}`,
		},
		{
			name:     "Transition with XSS to_state",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  `{"to_state": "<script>alert('xss')</script>"}`,
		},

		// History endpoint tests
		{
			name:     "History with negative limit",
			endpoint: "/api/v1/assets/test/history?limit=-1",
			method:   "GET",
		},
		{
			name:     "History with zero limit",
			endpoint: "/api/v1/assets/test/history?limit=0",
			method:   "GET",
		},
		{
			name:     "History with very large limit",
			endpoint: "/api/v1/assets/test/history?limit=999999999",
			method:   "GET",
		},
		{
			name:     "History with non-numeric limit",
			endpoint: "/api/v1/assets/test/history?limit=not_a_number",
			method:   "GET",
		},
		{
			name:     "History with float limit",
			endpoint: "/api/v1/assets/test/history?limit=1.5",
			method:   "GET",
		},
		{
			name:     "History with negative float limit",
			endpoint: "/api/v1/assets/test/history?limit=-1.5",
			method:   "GET",
		},
		{
			name:     "History with multiple limit parameters",
			endpoint: "/api/v1/assets/test/history?limit=10&limit=20",
			method:   "GET",
		},
		{
			name:     "History with SQL injection in limit",
			endpoint: "/api/v1/assets/test/history?limit=1' OR '1'='1",
			method:   "GET",
		},
		{
			name:     "History with XSS in limit",
			endpoint: "/api/v1/assets/test/history?limit=<script>alert('xss')</script>",
			method:   "GET",
		},
		{
			name:     "History with path traversal in limit",
			endpoint: "/api/v1/assets/test/history?limit=../../../etc/passwd",
			method:   "GET",
		},
		{
			name:     "History with very long limit parameter",
			endpoint: fmt.Sprintf("/api/v1/assets/test/history?limit=%s", strings.Repeat("9", 1000)),
			method:   "GET",
		},
		{
			name:     "History with special characters in limit",
			endpoint: "/api/v1/assets/test/history?limit=@#$%^&*()",
			method:   "GET",
		},
		{
			name:     "History with unicode in limit",
			endpoint: "/api/v1/assets/test/history?limit=测试",
			method:   "GET",
		},
		{
			name:     "History with emoji in limit",
			endpoint: "/api/v1/assets/test/history?limit=🚀🔥💯",
			method:   "GET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.payload != "" {
				body = strings.NewReader(tt.payload)
			}

			req, err := http.NewRequest(tt.method, c.baseURL+tt.endpoint, body)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Set content type for POST requests
			if tt.method == "POST" && tt.payload != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			// Set custom headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			resp, err := c.client.Do(req)
			if err != nil {
				t.Logf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			responseBody, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(responseBody))

			// API should handle these cases gracefully
			if resp.StatusCode >= 500 {
				t.Errorf("Server returned 5xx error for: %s", tt.name)
			}
		})
	}
}

// TestAssetTypePathInjection tests injection through asset type paths
func (c *CrashTestSuite) TestAssetTypePathInjection(t *testing.T) {
	maliciousPaths := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\drivers\\etc\\hosts",
		"../../../../../../etc/shadow",
		"..%2F..%2F..%2Fetc%2Fpasswd",
		"..%252F..%252F..%252Fetc%252Fpasswd",
		"....//....//....//etc//passwd",
		"..%c0%af..%c0%af..%c0%afetc%c0%afpasswd",
		"..%c1%9c..%c1%9c..%c1%9cetc%c1%9cpasswd",
		"..%c0%2f..%c0%2f..%c0%2fetc%c0%2fpasswd",
		"..%c1%af..%c1%af..%c1%afetc%c1%afpasswd",
		"/etc/passwd",
		"C:\\Windows\\System32\\drivers\\etc\\hosts",
		"file:///etc/passwd",
		"file:///C:/Windows/System32/drivers/etc/hosts",
		"http://evil.com/malicious.yaml",
		"https://evil.com/malicious.yaml",
		"ftp://evil.com/malicious.yaml",
		"smb://evil.com/malicious.yaml",
		"ldap://evil.com/malicious.yaml",
		"gopher://evil.com/malicious.yaml",
		"data:text/plain,<script>alert('xss')</script>",
		"javascript:alert('xss')",
		"vbscript:alert('xss')",
		"data:text/html,<script>alert('xss')</script>",
		"data:application/json,{\"malicious\": \"payload\"}",
		"data:text/xml,<?xml version=\"1.0\"?><root><script>alert('xss')</script></root>",
		"data:text/csv,malicious,payload",
		"data:text/plain;base64,PHNjcmlwdD5hbGVydCgneHNzJyk8L3NjcmlwdD4=",
		"data:image/svg+xml,<svg onload=alert('xss')></svg>",
		"data:image/svg+xml;base64,PHN2ZyBvbmxvYWQ9YWxlcnQoJ3hzcycpPjwvc3ZnPg==",
	}

	for _, maliciousPath := range maliciousPaths {
		t.Run(fmt.Sprintf("AssetTypePath_%s", maliciousPath), func(t *testing.T) {
			payload := map[string]string{
				"asset_type":  maliciousPath,
				"instance_id": "test1",
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

			resp, err := c.client.Do(req)
			if err != nil {
				t.Logf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

			// API should handle malicious paths gracefully
			if resp.StatusCode >= 500 {
				t.Errorf("Server returned 5xx error for malicious path: %s", maliciousPath)
			}

			// Check if file contents are exposed
			if strings.Contains(string(body), "root:") || strings.Contains(string(body), "localhost") {
				t.Errorf("File contents may be exposed for path: %s", maliciousPath)
			}
		})
	}
}

// TestConcurrentAssetCreation tests concurrent asset creation
func (c *CrashTestSuite) TestConcurrentAssetCreation(t *testing.T) {
	concurrency := 50
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()

			payload := map[string]string{
				"asset_type":  "simple_asset_type.yaml",
				"instance_id": fmt.Sprintf("concurrent-test-%d", id),
			}

			jsonPayload, err := json.Marshal(payload)
			if err != nil {
				t.Errorf("Failed to marshal payload: %v", err)
				return
			}

			req, err := http.NewRequest("POST", c.baseURL+"/api/v1/assets", bytes.NewReader(jsonPayload))
			if err != nil {
				t.Errorf("Failed to create request: %v", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := c.client.Do(req)
			if err != nil {
				t.Errorf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

			// Should handle concurrent requests gracefully
			if resp.StatusCode >= 500 {
				t.Errorf("Server returned 5xx error for concurrent request %d", id)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	timeout := time.After(30 * time.Second)
	completed := 0
	for {
		select {
		case <-done:
			completed++
			if completed >= concurrency {
				return
			}
		case <-timeout:
			t.Logf("Timeout reached, completed %d/%d requests", completed, concurrency)
			return
		}
	}
}

// TestRaceConditions tests for race conditions
func (c *CrashTestSuite) TestRaceConditions(t *testing.T) {
	// Test concurrent transitions on the same asset
	assetID := "race-test-asset"

	// First create an asset
	createPayload := map[string]string{
		"asset_type":  "simple_asset_type.yaml",
		"instance_id": assetID,
	}

	jsonPayload, err := json.Marshal(createPayload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/assets", bytes.NewReader(jsonPayload))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Logf("Failed to create asset for race condition test: %d", resp.StatusCode)
		return
	}

	// Now test concurrent transitions
	concurrency := 20
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()

			transitionPayload := map[string]string{
				"to_state": "RUNNING",
			}

			jsonPayload, err := json.Marshal(transitionPayload)
			if err != nil {
				t.Errorf("Failed to marshal payload: %v", err)
				return
			}

			req, err := http.NewRequest("POST", c.baseURL+fmt.Sprintf("/api/v1/assets/%s/transition", assetID), bytes.NewReader(jsonPayload))
			if err != nil {
				t.Errorf("Failed to create request: %v", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := c.client.Do(req)
			if err != nil {
				t.Errorf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Transition %d response status: %d, Body: %s", id, resp.StatusCode, string(body))

			// Should handle race conditions gracefully
			if resp.StatusCode >= 500 {
				t.Errorf("Server returned 5xx error for race condition test %d", id)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	timeout := time.After(30 * time.Second)
	completed := 0
	for {
		select {
		case <-done:
			completed++
			if completed >= concurrency {
				return
			}
		case <-timeout:
			t.Logf("Timeout reached, completed %d/%d requests", completed, concurrency)
			return
		}
	}
}

// Wrapper test functions to invoke the suite methods

// TestSpecificAPIEndpointsWrapper runs the specific API endpoint tests
func TestSpecificAPIEndpointsWrapper(t *testing.T) {
	suite := NewCrashTestSuite("http://localhost:8080")
	suite.TestSpecificAPIEndpoints(t)
}

// TestAssetTypePathInjectionWrapper runs the asset type path injection tests
func TestAssetTypePathInjectionWrapper(t *testing.T) {
	suite := NewCrashTestSuite("http://localhost:8080")
	suite.TestAssetTypePathInjection(t)
}

// TestConcurrentAssetCreationWrapper runs the concurrent asset creation tests
func TestConcurrentAssetCreationWrapper(t *testing.T) {
	suite := NewCrashTestSuite("http://localhost:8080")
	suite.TestConcurrentAssetCreation(t)
}

// TestRaceConditionsWrapper runs the race condition tests
func TestRaceConditionsWrapper(t *testing.T) {
	suite := NewCrashTestSuite("http://localhost:8080")
	suite.TestRaceConditions(t)
}
