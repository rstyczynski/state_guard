# GraphViz Debug Logging Implementation

## Problem

GraphViz logging was verbose and inconsistent with the HTTP logging format used by the API:

```
2025/10/24 09:17:52 GraphViz: Creating 9 state nodes
2025/10/24 09:17:52 GraphViz: Creating node 1/9: CREATED
2025/10/24 09:17:52 GraphViz: Creating node 2/9: STARTING
2025/10/24 09:17:52 GraphViz: Creating node 3/9: RUNNING
...
2025/10/24 09:17:52 GraphViz: Creating edge 1/13: CREATED -> STARTING
2025/10/24 09:17:52 GraphViz: Creating edge 2/13: STARTING -> RUNNING
...
```

While HTTP logging uses structured format:
```
2025/10/24 09:22:33 [Ryszards-MacBook-Pro.local/ryIfryn6DU-000526] "GET http://localhost:8080/api/v1/visualize/asset/os1?format=svg&layout=horizontal&history=false&available=true&highlight_current=true&highlight_state=STOPPING HTTP/1.1" from [::1]:64191 - 200 7611B in 1.672980083s
```

## Solution

Implemented **debug-level logging** for GraphViz operations that:

1. **Uses environment variable** to control logging
2. **Consistent with HTTP logging** format
3. **Debug mode only** - not verbose by default
4. **Structured logging** with `[GraphViz-DEBUG]` prefix

## Implementation

### 1. **Debug Logging Control**
```go
// Debug logging for GraphViz operations
var (
    debugLogging = os.Getenv("GRAPHVIZ_DEBUG") == "true"
    debugMutex   sync.Mutex
)
```

### 2. **Debug Log Function**
```go
// debugLog logs GraphViz operations in debug mode only
func debugLog(format string, args ...interface{}) {
    if debugLogging {
        debugMutex.Lock()
        defer debugMutex.Unlock()
        log.Printf("[GraphViz-DEBUG] "+format, args...)
    }
}
```

### 3. **Replaced Verbose Logging**
```go
// Before (verbose):
log.Printf("GraphViz: Creating node %d/%d: %s", i+1, len(g.definition.States), state)
log.Printf("GraphViz: Creating edge %d/%d: %s -> %s", i+1, len(g.definition.Transitions), trans.From, trans.To)
log.Printf("GraphViz: SVG render successful, %d bytes", buf.Len())

// After (debug only):
debugLog("Creating node %d/%d: %s", i+1, len(g.definition.States), state)
debugLog("Creating edge %d/%d: %s -> %s", i+1, len(g.definition.Transitions), trans.From, trans.To)
debugLog("SVG render successful, %d bytes", buf.Len())
```

## Usage

### **Default Behavior (Clean Logs)**
```bash
# No GraphViz debug logging
go run cmd/api/main.go
```

**Output:**
```
2025/10/24 09:22:33 [Ryszards-MacBook-Pro.local/ryIfryn6DU-000526] "GET http://localhost:8080/api/v1/visualize/asset/os1?format=svg&layout=horizontal&history=false&available=true&highlight_current=true&highlight_state=STOPPING HTTP/1.1" from [::1]:64191 - 200 7611B in 1.672980083s
```

### **Debug Mode (Verbose Logs)**
```bash
# Enable GraphViz debug logging
GRAPHVIZ_DEBUG=true go run cmd/api/main.go
```

**Output:**
```
2025/10/24 09:22:33 [Ryszards-MacBook-Pro.local/ryIfryn6DU-000526] "GET http://localhost:8080/api/v1/visualize/asset/os1?format=svg&layout=horizontal&history=false&available=true&highlight_current=true&highlight_state=STOPPING HTTP/1.1" from [::1]:64191 - 200 7611B in 1.672980083s
2025/10/24 09:22:33 [GraphViz-DEBUG] Acquiring global mutex (format: svg)
2025/10/24 09:22:33 [GraphViz-DEBUG] Global mutex acquired (format: svg)
2025/10/24 09:22:33 [GraphViz-DEBUG] Creating graph with 9 states, 13 transitions
2025/10/24 09:22:33 [GraphViz-DEBUG] Creating 9 state nodes
2025/10/24 09:22:33 [GraphViz-DEBUG] Creating node 1/9: CREATED
2025/10/24 09:22:33 [GraphViz-DEBUG] Creating node 2/9: STARTING
...
2025/10/24 09:22:33 [GraphViz-DEBUG] Creating edges for 13 transitions
2025/10/24 09:22:33 [GraphViz-DEBUG] Creating edge 1/13: CREATED -> STARTING
2025/10/24 09:22:33 [GraphViz-DEBUG] Creating edge 2/13: STARTING -> RUNNING
...
2025/10/24 09:22:33 [GraphViz-DEBUG] Rendering svg format
2025/10/24 09:22:33 [GraphViz-DEBUG] SVG render successful, 7617 bytes
2025/10/24 09:22:33 [GraphViz-DEBUG] Closing resources
2025/10/24 09:22:33 [GraphViz-DEBUG] Graph closed, GraphViz instance returned to pool
2025/10/24 09:22:33 [GraphViz-DEBUG] Releasing global mutex (format: svg)
```

## Benefits

### 🎯 **Clean Default Logs**
- No verbose GraphViz logging by default
- Only HTTP request logs visible
- Consistent with API logging format

### 🎯 **Debug Mode Available**
- Enable with `GRAPHVIZ_DEBUG=true`
- Detailed GraphViz operation logging
- Structured `[GraphViz-DEBUG]` prefix

### 🎯 **Consistent Logging**
- Uses same `log` package as API
- Structured format with prefixes
- Thread-safe with mutex protection

### 🎯 **Performance**
- No logging overhead in production
- Debug logging only when needed
- Mutex protection for thread safety

## Logging Levels

### **Production (Default)**
- HTTP request logs only
- Clean, minimal output
- No GraphViz verbosity

### **Debug Mode**
- HTTP request logs
- Detailed GraphViz operations
- Node/edge creation details
- Rendering progress
- Memory management details

## Summary

The implementation provides:

1. **Clean production logs** - No verbose GraphViz logging by default
2. **Debug mode available** - Enable with `GRAPHVIZ_DEBUG=true`
3. **Consistent format** - Uses same logging approach as HTTP middleware
4. **Thread-safe** - Mutex protection for concurrent access
5. **Performance** - No overhead in production mode

This makes the logs coherent with the HTTP logging format while providing debug capabilities when needed.
