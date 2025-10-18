# FSM API Crash Tests

This directory contains comprehensive crash tests designed to try to break the FSM REST API by sending malformed, malicious, and boundary-condition requests.

## Test Categories

### 1. Malformed JSON Tests
- Invalid JSON syntax (missing braces, trailing commas, etc.)
- Control characters and null bytes
- Deep nesting and large payloads
- Wrong data types and missing required fields

### 2. Security Tests
- **Path Traversal**: Attempts to access files outside the intended directory
- **SQL Injection**: Tries to inject SQL commands through parameters
- **XSS (Cross-Site Scripting)**: Attempts to inject malicious scripts
- **Invalid HTTP Methods**: Tests unsupported HTTP methods

### 3. Boundary Condition Tests
- Very long instance IDs and asset types
- Special characters and unicode in identifiers
- Invalid query parameters (negative limits, non-numeric values)
- Empty and malformed requests

### 4. Resource Exhaustion Tests
- **Concurrent Requests**: Tests FSM operations (create assets, transitions, history) with 100 simultaneous requests
- **Memory Exhaustion**: Large payloads and deeply nested JSON
- **Timeout Handling**: Tests with very short timeouts
- **Resource Limits**: Tests system behavior under high load with realistic FSM operations
- **Multi-phase Testing**: Creates assets → Performs transitions → Reads history → Cleanup

### 5. Protocol Tests
- Invalid content types
- Malformed headers
- Invalid HTTP headers with special characters

## Running the Tests

### Prerequisites
1. Start the FSM API server:
   ```bash
   go run cmd/api/main.go
   ```

2. Ensure the server is running on `http://localhost:8080` (default)

### Running All Tests
```bash
# Using the shell script (recommended)
./run_crash_tests.sh

# Using go test directly
go test -v ./tests/api/crash/
```

### Running with Custom Options
```bash
# Using the shell script with options
./run_crash_tests.sh -u http://localhost:8080 -t 60s -v

# Using go test with verbose output
go test -v -timeout 60s ./tests/api/crash/
```

### Running Individual Test Categories
```bash
# Run only basic crash tests
go test -run TestBasicCrashTests ./tests/api/crash/

# Run only security tests
go test -run TestSecurityAttacks ./tests/api/crash/

# Run only concurrent request tests
go test -run TestConcurrentRequests ./tests/api/crash/

# Run only large payload tests
go test -run TestLargePayloads ./tests/api/crash/
```

## Test Results Interpretation

### Expected Behaviors
- **400 Bad Request**: For malformed JSON, invalid parameters, or boundary conditions
- **404 Not Found**: For non-existent resources or path traversal attempts
- **405 Method Not Allowed**: For unsupported HTTP methods
- **Graceful handling**: No 5xx errors for malformed input

### Red Flags
- **500 Internal Server Error**: Indicates the server crashed or couldn't handle the request
- **XSS payloads reflected**: Scripts or HTML in responses
- **File contents exposed**: Path traversal attempts returning actual file contents
- **SQL errors**: Database errors in responses
- **Memory leaks**: Server becoming unresponsive after tests

## Test Coverage

The crash tests cover:

1. **Input Validation**: All API endpoints with various malformed inputs
2. **Security**: Common attack vectors (injection, XSS, path traversal)
3. **Performance**: Concurrent requests, large payloads, resource exhaustion
4. **Protocol Compliance**: HTTP method handling, content types, headers
5. **Error Handling**: Graceful degradation under stress

## Customization

### Adding New Tests
1. Add new test methods to `crash_test.go`
2. Follow the naming convention: `Test[CategoryName]`
3. Use the existing test structure with proper error handling
4. Add the new test to the test list in `run_crash_tests.go`

### Example Test Structure
```go
func (c *CrashTestSuite) TestCustomAttack(t *testing.T) {
    // Test implementation
    req, err := http.NewRequest("METHOD", c.baseURL+"/endpoint", body)
    if err != nil {
        t.Fatalf("Failed to create request: %v", err)
    }
    
    resp, err := c.client.Do(req)
    if err != nil {
        t.Logf("Request failed: %v", err)
        return
    }
    defer resp.Body.Close()
    
    // Validate response
    if resp.StatusCode >= 500 {
        t.Errorf("Server returned 5xx error")
    }
}
```

## Continuous Integration

These tests can be integrated into CI/CD pipelines to ensure the API remains robust against various attack vectors and edge cases.

### GitHub Actions Example
```yaml
- name: Run API Crash Tests
  run: |
    ./run_crash_tests.sh -u http://localhost:8080
```

## Security Considerations

These tests are designed to be run against a test environment only. They include:
- Malicious payloads
- Resource exhaustion attempts
- Security vulnerability probes

**Never run these tests against production systems.**
