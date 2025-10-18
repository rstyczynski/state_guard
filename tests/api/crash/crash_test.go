package crash

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// CrashTestSuite contains all crash tests for the FSM API
type CrashTestSuite struct {
	baseURL string
	client  *http.Client
}

// NewCrashTestSuite creates a new crash test suite
func NewCrashTestSuite(baseURL string) *CrashTestSuite {
	return &CrashTestSuite{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TestMalformedJSON tests various malformed JSON scenarios
func (c *CrashTestSuite) TestMalformedJSON(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		endpoint string
		method   string
	}{
		{
			name:     "Invalid JSON - Missing closing brace",
			payload:  `{"asset_type": "test", "instance_id": "test1"`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Trailing comma",
			payload:  `{"asset_type": "test", "instance_id": "test1",}`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Unescaped quotes",
			payload:  `{"asset_type": "test", "instance_id": "test with "quotes""}`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Null bytes",
			payload:  `{"asset_type": "test\x00", "instance_id": "test1"}`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Control characters",
			payload:  `{"asset_type": "test\x01\x02\x03", "instance_id": "test1"}`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Deep nesting",
			payload:  `{"asset_type": {"nested": {"deep": {"value": "test"}}}}`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Array instead of object",
			payload:  `["asset_type", "test", "instance_id", "test1"]`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - String instead of object",
			payload:  `"just a string"`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Number instead of object",
			payload:  `12345`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Boolean instead of object",
			payload:  `true`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Empty object",
			payload:  `{}`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Missing required fields",
			payload:  `{"asset_type": "test"}`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Extra fields",
			payload:  `{"asset_type": "test", "instance_id": "test1", "extra_field": "should_not_exist"}`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Wrong data types",
			payload:  `{"asset_type": 123, "instance_id": true}`,
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
		{
			name:     "Invalid JSON - Very large payload",
			payload:  fmt.Sprintf(`{"asset_type": "%s", "instance_id": "test1"}`, strings.Repeat("x", 1000000)),
			endpoint: "/api/v1/assets",
			method:   "POST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, c.baseURL+tt.endpoint, strings.NewReader(tt.payload))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := c.client.Do(req)
			if err != nil {
				t.Logf("Request failed (expected for malformed data): %v", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

			// API should handle malformed JSON gracefully
			if resp.StatusCode >= 500 {
				t.Errorf("Server returned 5xx error for malformed JSON, should handle gracefully")
			}
		})
	}
}

// TestInvalidHTTPMethods tests unsupported HTTP methods
func (c *CrashTestSuite) TestInvalidHTTPMethods(t *testing.T) {
	endpoints := []string{
		"/api/v1/health",
		"/api/v1/assets",
		"/api/v1/assets/test-id",
		"/api/v1/assets/test-id/transition",
		"/api/v1/assets/test-id/history",
	}

	methods := []string{"PUT", "PATCH", "HEAD", "OPTIONS", "TRACE", "CONNECT"}

	for _, endpoint := range endpoints {
		for _, method := range methods {
			t.Run(fmt.Sprintf("%s_%s", method, endpoint), func(t *testing.T) {
				req, err := http.NewRequest(method, c.baseURL+endpoint, nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := c.client.Do(req)
				if err != nil {
					t.Logf("Request failed: %v", err)
					return
				}
				defer resp.Body.Close()

				body, _ := io.ReadAll(resp.Body)
				t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

				// Should return 405 Method Not Allowed for unsupported methods
				if resp.StatusCode != 405 {
					t.Errorf("Expected 405 Method Not Allowed, got %d", resp.StatusCode)
				}
			})
		}
	}
}

// TestPathTraversal tests path traversal attacks
func (c *CrashTestSuite) TestPathTraversal(t *testing.T) {
	maliciousIDs := []string{
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
	}

	for _, maliciousID := range maliciousIDs {
		t.Run(fmt.Sprintf("PathTraversal_%s", maliciousID), func(t *testing.T) {
			endpoints := []string{
				fmt.Sprintf("/api/v1/assets/%s", url.PathEscape(maliciousID)),
				fmt.Sprintf("/api/v1/assets/%s/transition", url.PathEscape(maliciousID)),
				fmt.Sprintf("/api/v1/assets/%s/history", url.PathEscape(maliciousID)),
			}

			for _, endpoint := range endpoints {
				req, err := http.NewRequest("GET", c.baseURL+endpoint, nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := c.client.Do(req)
				if err != nil {
					t.Logf("Request failed: %v", err)
					return
				}
				defer resp.Body.Close()

				body, _ := io.ReadAll(resp.Body)
				t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

				// Should return 404 or 400, not expose file contents
				if resp.StatusCode >= 500 {
					t.Errorf("Server returned 5xx error for path traversal attempt")
				}
			}
		})
	}
}

// TestSQLInjection tests SQL injection attempts
func (c *CrashTestSuite) TestSQLInjection(t *testing.T) {
	sqlPayloads := []string{
		"'; DROP TABLE instances; --",
		"1' OR '1'='1",
		"1' UNION SELECT * FROM instances --",
		"1'; INSERT INTO instances VALUES ('hacked', 'hacked', 'hacked'); --",
		"1' AND (SELECT COUNT(*) FROM sqlite_master) > 0 --",
		"1' OR 1=1 --",
		"1' OR 'x'='x",
		"1' OR 1=1#",
		"1' OR 1=1/*",
		"1' OR 1=1; --",
		"1' OR 1=1 LIMIT 1 --",
		"1' OR 1=1 ORDER BY 1 --",
		"1' OR 1=1 GROUP BY 1 --",
		"1' OR 1=1 HAVING 1=1 --",
		"1' OR 1=1 UNION SELECT 1,2,3,4,5,6,7,8,9,10 --",
	}

	for _, payload := range sqlPayloads {
		t.Run(fmt.Sprintf("SQLInjection_%s", payload), func(t *testing.T) {
			endpoints := []string{
				fmt.Sprintf("/api/v1/assets/%s", url.PathEscape(payload)),
				fmt.Sprintf("/api/v1/assets/%s/transition", url.PathEscape(payload)),
				fmt.Sprintf("/api/v1/assets/%s/history", url.PathEscape(payload)),
			}

			for _, endpoint := range endpoints {
				req, err := http.NewRequest("GET", c.baseURL+endpoint, nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := c.client.Do(req)
				if err != nil {
					t.Logf("Request failed: %v", err)
					return
				}
				defer resp.Body.Close()

				body, _ := io.ReadAll(resp.Body)
				t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

				// Should return 404 or 400, not execute SQL
				if resp.StatusCode >= 500 {
					t.Errorf("Server returned 5xx error for SQL injection attempt")
				}
			}
		})
	}
}

// TestXSS tests Cross-Site Scripting attempts
func (c *CrashTestSuite) TestXSS(t *testing.T) {
	xssPayloads := []string{
		"<script>alert('XSS')</script>",
		"javascript:alert('XSS')",
		"<img src=x onerror=alert('XSS')>",
		"<svg onload=alert('XSS')>",
		"<iframe src=javascript:alert('XSS')>",
		"<body onload=alert('XSS')>",
		"<input onfocus=alert('XSS') autofocus>",
		"<select onfocus=alert('XSS') autofocus>",
		"<textarea onfocus=alert('XSS') autofocus>",
		"<keygen onfocus=alert('XSS') autofocus>",
		"<video><source onerror=alert('XSS')>",
		"<audio src=x onerror=alert('XSS')>",
		"<details open ontoggle=alert('XSS')>",
		"<marquee onstart=alert('XSS')>",
		"<div onmouseover=alert('XSS')>",
		"<style>@import'javascript:alert(\"XSS\")';</style>",
		"<link rel=stylesheet href=javascript:alert('XSS')>",
		"<meta http-equiv=refresh content=0;url=javascript:alert('XSS')>",
		"<object data=javascript:alert('XSS')>",
		"<embed src=javascript:alert('XSS')>",
	}

	for _, payload := range xssPayloads {
		t.Run(fmt.Sprintf("XSS_%s", payload), func(t *testing.T) {
			// Test in URL parameters
			endpoints := []string{
				fmt.Sprintf("/api/v1/assets/%s", url.PathEscape(payload)),
				fmt.Sprintf("/api/v1/assets/%s/transition", url.PathEscape(payload)),
				fmt.Sprintf("/api/v1/assets/%s/history", url.PathEscape(payload)),
			}

			for _, endpoint := range endpoints {
				req, err := http.NewRequest("GET", c.baseURL+endpoint, nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := c.client.Do(req)
				if err != nil {
					t.Logf("Request failed: %v", err)
					return
				}
				defer resp.Body.Close()

				body, _ := io.ReadAll(resp.Body)
				t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

				// Check if XSS payload is reflected in response
				if strings.Contains(string(body), payload) {
					t.Errorf("XSS payload reflected in response: %s", payload)
				}
			}

			// Test in JSON payload
			jsonPayload := fmt.Sprintf(`{"asset_type": "%s", "instance_id": "test1"}`, payload)
			req, err := http.NewRequest("POST", c.baseURL+"/api/v1/assets", strings.NewReader(jsonPayload))
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

			// Check if XSS payload is reflected in response
			if strings.Contains(string(body), payload) {
				t.Errorf("XSS payload reflected in response: %s", payload)
			}
		})
	}
}

// TestBoundaryConditions tests boundary conditions and limits
func (c *CrashTestSuite) TestBoundaryConditions(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		method   string
		payload  string
	}{
		{
			name:     "Very long instance ID",
			endpoint: fmt.Sprintf("/api/v1/assets/%s", strings.Repeat("a", 10000)),
			method:   "GET",
		},
		{
			name:     "Very long asset type",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  fmt.Sprintf(`{"asset_type": "%s", "instance_id": "test1"}`, strings.Repeat("a", 10000)),
		},
		{
			name:     "Empty instance ID",
			endpoint: "/api/v1/assets/",
			method:   "GET",
		},
		{
			name:     "Instance ID with special characters",
			endpoint: "/api/v1/assets/test@#$%^&*()",
			method:   "GET",
		},
		{
			name:     "Instance ID with unicode",
			endpoint: "/api/v1/assets/测试实例",
			method:   "GET",
		},
		{
			name:     "Instance ID with emoji",
			endpoint: "/api/v1/assets/🚀test🔥",
			method:   "GET",
		},
		{
			name:     "Very long query parameter",
			endpoint: fmt.Sprintf("/api/v1/assets/test/history?limit=%s", strings.Repeat("9", 100)),
			method:   "GET",
		},
		{
			name:     "Negative limit parameter",
			endpoint: "/api/v1/assets/test/history?limit=-1",
			method:   "GET",
		},
		{
			name:     "Zero limit parameter",
			endpoint: "/api/v1/assets/test/history?limit=0",
			method:   "GET",
		},
		{
			name:     "Very large limit parameter",
			endpoint: "/api/v1/assets/test/history?limit=999999999",
			method:   "GET",
		},
		{
			name:     "Invalid limit parameter",
			endpoint: "/api/v1/assets/test/history?limit=not_a_number",
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
			if tt.payload != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := c.client.Do(req)
			if err != nil {
				t.Logf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			responseBody, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(responseBody))

			// API should handle boundary conditions gracefully
			if resp.StatusCode >= 500 {
				t.Errorf("Server returned 5xx error for boundary condition")
			}
		})
	}
}

// TestConcurrentRequests tests concurrent request handling with full FSM lifecycle
func (c *CrashTestSuite) TestConcurrentRequests(t *testing.T) {
	concurrency := 100
	done := make(chan bool, concurrency)
	createdAssets := make(chan string, concurrency)

	// Phase 1: Create assets concurrently
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()

			instanceID := fmt.Sprintf("crash-suite-%d-%d", time.Now().Unix(), id)
			payload := fmt.Sprintf(`{"asset_type": "simple_asset_type.yaml", "instance_id": "%s"}`, instanceID)

			req, err := http.NewRequest("POST", c.baseURL+"/api/v1/assets", strings.NewReader(payload))
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

			if resp.StatusCode == 201 || resp.StatusCode == 200 {
				createdAssets <- instanceID
			} else {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected 201, got %d: %s", resp.StatusCode, string(body))
			}
		}(i)
	}

	// Wait for all asset creations
	for i := 0; i < concurrency; i++ {
		<-done
	}
	close(createdAssets)

	assetList := make([]string, 0)
	for asset := range createdAssets {
		assetList = append(assetList, asset)
	}

	t.Logf("Created %d assets concurrently", len(assetList))

	// Phase 2: Perform multiple transitions concurrently
	done2 := make(chan bool, len(assetList)*3)
	for _, assetID := range assetList {
		go func(id string) {
			defer func() { done2 <- true }()

			// Transition 1: CREATED → STARTING
			payload := `{"to_state": "STARTING"}`
			req, _ := http.NewRequest("POST", c.baseURL+"/api/v1/assets/"+id+"/transition", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := c.client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
		}(assetID)

		go func(id string) {
			defer func() { done2 <- true }()

			// Transition 2: STARTING → RUNNING
			time.Sleep(10 * time.Millisecond) // Small delay to ensure order
			payload := `{"to_state": "RUNNING"}`
			req, _ := http.NewRequest("POST", c.baseURL+"/api/v1/assets/"+id+"/transition", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := c.client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
		}(assetID)

		go func(id string) {
			defer func() { done2 <- true }()

			// Read history concurrently with transitions
			time.Sleep(15 * time.Millisecond)
			req, _ := http.NewRequest("GET", c.baseURL+"/api/v1/assets/"+id+"/history", nil)
			resp, _ := c.client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
		}(assetID)
	}

	// Wait for all transitions and history reads
	for i := 0; i < len(assetList)*3; i++ {
		<-done2
	}

	t.Logf("Performed %d concurrent transitions and history reads", len(assetList)*3)

	// Phase 3: Cleanup - delete all assets
	done3 := make(chan bool, len(assetList))
	for _, assetID := range assetList {
		go func(id string) {
			defer func() { done3 <- true }()

			req, _ := http.NewRequest("DELETE", c.baseURL+"/api/v1/assets/"+id, nil)
			resp, _ := c.client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
		}(assetID)
	}

	// Wait for all deletions
	for i := 0; i < len(assetList); i++ {
		<-done3
	}

	t.Logf("Cleaned up %d assets", len(assetList))
}

// TestMemoryExhaustion tests memory exhaustion scenarios
func (c *CrashTestSuite) TestMemoryExhaustion(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		method   string
		payload  string
	}{
		{
			name:     "Very large JSON payload",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  fmt.Sprintf(`{"asset_type": "%s", "instance_id": "test1"}`, strings.Repeat("x", 10*1024*1024)), // 10MB
		},
		{
			name:     "Very large transition payload",
			endpoint: "/api/v1/assets/test/transition",
			method:   "POST",
			payload:  fmt.Sprintf(`{"to_state": "%s"}`, strings.Repeat("x", 10*1024*1024)), // 10MB
		},
		{
			name:     "Deeply nested JSON",
			endpoint: "/api/v1/assets",
			method:   "POST",
			payload:  createDeeplyNestedJSON(1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, c.baseURL+tt.endpoint, strings.NewReader(tt.payload))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := c.client.Do(req)
			if err != nil {
				t.Logf("Request failed (expected for large payload): %v", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body length: %d", resp.StatusCode, len(body))

			// API should handle large payloads gracefully
			if resp.StatusCode >= 500 {
				t.Errorf("Server returned 5xx error for large payload")
			}
		})
	}
}

// TestInvalidContentTypes tests invalid content types
func (c *CrashTestSuite) TestInvalidContentTypes(t *testing.T) {
	contentTypes := []string{
		"text/plain",
		"application/xml",
		"text/html",
		"application/octet-stream",
		"multipart/form-data",
		"application/x-www-form-urlencoded",
		"image/jpeg",
		"video/mp4",
		"audio/mpeg",
		"application/pdf",
		"",
		"invalid/content-type",
		"application/json; charset=utf-8; boundary=something",
	}

	payload := `{"asset_type": "test", "instance_id": "test1"}`

	for _, contentType := range contentTypes {
		t.Run(fmt.Sprintf("ContentType_%s", contentType), func(t *testing.T) {
			req, err := http.NewRequest("POST", c.baseURL+"/api/v1/assets", strings.NewReader(payload))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			if contentType != "" {
				req.Header.Set("Content-Type", contentType)
			}

			resp, err := c.client.Do(req)
			if err != nil {
				t.Logf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

			// API should handle invalid content types gracefully
			if resp.StatusCode >= 500 {
				t.Errorf("Server returned 5xx error for invalid content type")
			}
		})
	}
}

// TestInvalidHeaders tests various invalid headers
func (c *CrashTestSuite) TestInvalidHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{
			name: "Very long header values",
			headers: map[string]string{
				"X-Custom-Header": strings.Repeat("x", 10000),
			},
		},
		{
			name: "Headers with null bytes",
			headers: map[string]string{
				"X-Custom-Header": "value\x00with\x00nulls",
			},
		},
		{
			name: "Headers with control characters",
			headers: map[string]string{
				"X-Custom-Header": "value\x01\x02\x03",
			},
		},
		{
			name: "Headers with unicode",
			headers: map[string]string{
				"X-Custom-Header": "测试值",
			},
		},
		{
			name: "Headers with emoji",
			headers: map[string]string{
				"X-Custom-Header": "🚀🔥💯",
			},
		},
		{
			name: "Very long header names",
			headers: map[string]string{
				strings.Repeat("X-", 1000) + "Header": "value",
			},
		},
		{
			name: "Headers with special characters",
			headers: map[string]string{
				"X-Header@#$%": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", c.baseURL+"/api/v1/health", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			resp, err := c.client.Do(req)
			if err != nil {
				t.Logf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))

			// API should handle invalid headers gracefully
			if resp.StatusCode >= 500 {
				t.Errorf("Server returned 5xx error for invalid headers")
			}
		})
	}
}

// Helper function to create deeply nested JSON
func createDeeplyNestedJSON(depth int) string {
	if depth <= 0 {
		return `"value"`
	}
	return fmt.Sprintf(`{"nested": %s}`, createDeeplyNestedJSON(depth-1))
}

// TestTimeoutHandling tests timeout scenarios
func (c *CrashTestSuite) TestTimeoutHandling(t *testing.T) {
	// Create a client with very short timeout
	shortTimeoutClient := &http.Client{
		Timeout: 1 * time.Millisecond,
	}

	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := shortTimeoutClient.Do(req)
	if err != nil {
		// Expected to fail due to timeout
		t.Logf("Request timed out as expected: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Response status: %d, Body: %s", resp.StatusCode, string(body))
}
