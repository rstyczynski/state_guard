# FSM API Overload Tests

This directory contains comprehensive overload tests designed to stress test the FSM API under various extreme conditions. These tests help identify performance bottlenecks, resource limits, and system behavior under load.

## Test Categories

### 1. **Concurrent Request Overload**
- **TestConcurrentRequestOverload**: Tests API with massive concurrent FSM operations (100-2000)
  - Phase 1: Concurrent asset creation
  - Phase 2: Concurrent state transitions (CREATED → STARTING)
  - Phase 3: Concurrent cleanup
  - Measures: Create success rate, transition success rate, overall RPS

- **TestRapidFireOverload**: Tests API with rapid-fire asset creation at various rates
  - Creates assets continuously at 10, 50, or 100 requests/second
  - Tests sustained load over 5-10 seconds
  - Automatic cleanup of created assets

- **TestConnectionPoolExhaustion**: Tests API behavior when connection pool is exhausted
  - Uses realistic FSM operations to exhaust connections

### 2. **Memory Exhaustion Overload**
- **TestMemoryExhaustionOverload**: Tests API with large payloads (1MB-50MB)

### 3. **Resource Exhaustion Overload**
- **TestResourceExhaustionOverload**: Tests API with 5000 concurrent asset creation requests
  - Stress tests the FSM engine with extreme concurrency
  - Tests database transaction handling under pressure
  - Automatic cleanup of all created assets

## Running the Tests

### Prerequisites
- FSM API server must be running on `http://localhost:8080`
- Ensure sufficient system resources for overload testing
- Consider running on a dedicated test environment

### Basic Test Execution

```bash
# Run all overload tests
go test -v ./tests/api/overload/

# Run specific test categories
go test -v -run TestConcurrentRequestOverload ./tests/api/overload/
go test -v -run TestMemoryExhaustionOverload ./tests/api/overload/
go test -v -run TestRapidFireOverload ./tests/api/overload/
go test -v -run TestConnectionPoolExhaustion ./tests/api/overload/
go test -v -run TestResourceExhaustionOverload ./tests/api/overload/
```

### Advanced Test Execution

```bash
# Run with custom timeout
go test -v -timeout 10m ./tests/api/overload/

# Run specific concurrency levels
go test -v -run "TestConcurrentRequestOverload/Concurrent_1000_requests" ./tests/api/overload/

# Run with race detection
go test -v -race ./tests/api/overload/

# Run with memory profiling
go test -v -memprofile=mem.prof ./tests/api/overload/
```

### Performance Monitoring

```bash
# Run with CPU profiling
go test -v -cpuprofile=cpu.prof ./tests/api/overload/

# Run with detailed timing
go test -v -timeout 30m ./tests/api/overload/ | grep "Duration:"

# Run with memory monitoring
go test -v -memprofile=mem.prof -cpuprofile=cpu.prof ./tests/api/overload/
```

## Test Parameters

### Concurrency Levels
- **Low**: 100-500 concurrent requests
- **Medium**: 500-1000 concurrent requests  
- **High**: 1000-2000 concurrent requests
- **Extreme**: 2000+ concurrent requests

### Payload Sizes
- **Small**: 1KB-10KB
- **Medium**: 10KB-100KB
- **Large**: 100KB-1MB
- **Huge**: 1MB-50MB

### Request Rates
- **Low**: 10 requests per second
- **Medium**: 50 requests per second
- **High**: 100 requests per second

## Expected Results

### Success Criteria
- **Concurrent Requests**: ≥80% success rate under normal load
- **Memory Tests**: API should handle large payloads gracefully (not crash)
- **Rapid Fire**: ≥90% success rate under rapid-fire conditions
- **Connection Pool**: ≥70% success rate under connection pressure
- **Resource Exhaustion**: ≥50% success rate under extreme resource pressure

### Warning Signs
- **High Error Rates**: >20% failure rate indicates performance issues
- **Memory Leaks**: Performance degradation across multiple cycles
- **Resource Exhaustion**: API becomes unresponsive under load
- **Poor Recovery**: API doesn't recover well after stress

## Test Results Analysis

### Example Output
```
=== RUN   TestConcurrentRequestOverload
=== RUN   TestConcurrentRequestOverload/Concurrent_100_requests
    overload_test.go:123: Concurrency: 100, Total Duration: 479ms, Create: 50ms, Transition: 369ms,
                          Create Success: 100, Transition Success: 100, Errors: 0, Overall RPS: 208.49
=== RUN   TestConcurrentRequestOverload/Concurrent_500_requests
    overload_test.go:123: Concurrency: 500, Total Duration: 1.2s, Create: 250ms, Transition: 850ms,
                          Create Success: 500, Transition Success: 500, Errors: 0, Overall RPS: 416.67
=== RUN   TestConcurrentRequestOverload/Concurrent_1000_requests
    overload_test.go:123: Concurrency: 1000, Total Duration: 2.5s, Create: 339ms, Transition: 1746ms,
                          Create Success: 1000, Transition Success: 1000, Errors: 0, Overall RPS: 400.00
--- PASS: TestConcurrentRequestOverload (4.2s)
```

### Key Metrics
- **Overall RPS**: Total operations (creates + transitions) per second
- **Create Success**: Number of successful asset creations
- **Transition Success**: Number of successful state transitions
- **Create Duration**: Time taken for concurrent asset creation phase
- **Transition Duration**: Time taken for concurrent transition phase
- **Total Duration**: End-to-end test duration including cleanup
- **Success Rate**: Percentage of successful operations
- **Error Count**: Number of failed operations

## Monitoring and Debugging

### System Monitoring
```bash
# Monitor system resources during tests
htop
iostat -x 1
netstat -an | grep :8080 | wc -l
```

### API Monitoring
```bash
# Monitor API server logs
tail -f /path/to/api/logs

# Monitor API metrics
curl -s http://localhost:8080/api/v1/health | jq
```

### Performance Analysis
```bash
# Analyze CPU profile
go tool pprof cpu.prof

# Analyze memory profile
go tool pprof mem.prof

# Analyze goroutine profile
go tool pprof http://localhost:8080/debug/pprof/goroutine
```

## Safety Considerations

### Resource Limits
- **Memory**: Ensure sufficient RAM for large payload tests
- **CPU**: Monitor CPU usage during intensive tests
- **Network**: Consider network bandwidth limitations
- **Storage**: Ensure adequate disk space for file operations

### System Protection
- Run tests on dedicated test environments
- Monitor system resources during execution
- Set appropriate timeouts to prevent hanging
- Use process limits to prevent system overload

### Cleanup
- Tests automatically clean up resources
- Monitor for lingering connections
- Check for memory leaks after test completion
- Verify system recovery after stress tests

## Troubleshooting

### Common Issues
1. **Connection Refused**: Ensure API server is running
2. **Timeout Errors**: Increase test timeouts or reduce concurrency
3. **Memory Errors**: Reduce payload sizes or concurrency levels
4. **Resource Exhaustion**: Run tests on more powerful hardware

### Debug Mode
```bash
# Run with maximum verbosity
go test -v -timeout 30m ./tests/api/overload/ 2>&1 | tee overload-test.log

# Run individual tests for debugging
go test -v -run TestConcurrentRequestOverload -timeout 5m ./tests/api/overload/
```

## Integration with CI/CD

### GitHub Actions Example
```yaml
- name: Run Overload Tests
  run: |
    go test -v -timeout 30m ./tests/api/overload/
    go test -v -race ./tests/api/overload/
```

### Jenkins Pipeline Example
```groovy
stage('Overload Tests') {
    steps {
        sh 'go test -v -timeout 30m ./tests/api/overload/'
        sh 'go test -v -race ./tests/api/overload/'
    }
}
```

## Contributing

When adding new overload tests:
1. Follow the existing naming conventions
2. Include comprehensive logging
3. Set appropriate success criteria
4. Add documentation for new test categories
5. Consider resource usage and safety
6. Test on various system configurations

## Support

For issues with overload tests:
1. Check system resource availability
2. Verify API server configuration
3. Review test logs for specific error patterns
4. Consider reducing test intensity
5. Monitor system performance during execution