# Additional Debug Logging Fixes

## Problem

Even after implementing debug logging, there were still verbose GraphViz logs appearing in production:

```
2025/10/24 09:28:44 GraphViz: Creating graph with 9 states, 13 transitions
2025/10/24 09:28:44 GraphViz: Creating graph with layout engine: dot, rank direction: LR
2025/10/24 09:28:44 GraphViz: Found wildcard transition to FAILED
2025/10/24 09:28:44 GraphViz: Creating wildcard ANY_STATE node
2025/10/24 09:28:44 GraphViz: Creating wildcard edge to FAILED
2025/10/24 09:28:44 GraphViz: Creating regular transitions
2025/10/24 09:28:44 GraphViz: Graph creation completed successfully with 9 nodes and 12 edges
2025/10/24 09:28:44 GraphViz: Performing WASM instance cleanup
2025/10/24 09:28:44 GraphViz: Releasing global mutex (format: png)
```

## Solution

Converted additional verbose GraphViz logs to debug mode:

### **Graph Creation Logs**
```go
// Before (verbose):
log.Printf("GraphViz: Creating graph with %d states, %d transitions", len(g.definition.States), len(g.definition.Transitions))
log.Printf("GraphViz: Creating graph with layout engine: %s, rank direction: %v", layoutEngine, rankDir)

// After (debug only):
debugLog("Creating graph with %d states, %d transitions", len(g.definition.States), len(g.definition.Transitions))
debugLog("Creating graph with layout engine: %s, rank direction: %v", layoutEngine, rankDir)
```

### **Wildcard Transition Logs**
```go
// Before (verbose):
log.Printf("GraphViz: Found wildcard transition to %s", wildcardTarget)
log.Printf("GraphViz: Creating wildcard ANY_STATE node")
log.Printf("GraphViz: Creating wildcard edge to %s", wildcardTarget)

// After (debug only):
debugLog("Found wildcard transition to %s", wildcardTarget)
debugLog("Creating wildcard ANY_STATE node")
debugLog("Creating wildcard edge to %s", wildcardTarget)
```

### **Regular Transitions Logs**
```go
// Before (verbose):
log.Printf("GraphViz: Creating regular transitions")

// After (debug only):
debugLog("Creating regular transitions")
```

### **Graph Completion Logs**
```go
// Before (verbose):
log.Printf("GraphViz: Graph creation completed successfully with %d nodes and %d edges", len(nodes), len(createdEdges))

// After (debug only):
debugLog("Graph creation completed successfully with %d nodes and %d edges", len(nodes), len(createdEdges))
```

### **Cleanup and Mutex Logs**
```go
// Before (verbose):
log.Printf("GraphViz: Performing WASM instance cleanup")
log.Printf("GraphViz: Releasing global mutex (format: %s)", opts.Format)

// After (debug only):
debugLog("Performing WASM instance cleanup")
debugLog("Releasing global mutex (format: %s)", opts.Format)
```

## Expected Behavior

### **Production (Default) - Clean Logs**
```bash
go run cmd/api/main.go
```

**Output:**
```
2025/10/24 09:28:44 [Ryszards-MacBook-Pro.local/MymUK5rlEQ-000002] "GET http://localhost:8080/api/v1/visualize/asset/os1?format=png&layout=horizontal&history=false&available=true&highlight_current=true HTTP/1.1" from [::1]:50385 - 200 36465B in 255.816167ms
```

**No verbose GraphViz logs!**

### **Debug Mode - Verbose Logs**
```bash
GRAPHVIZ_DEBUG=true go run cmd/api/main.go
```

**Output:**
```
2025/10/24 09:28:44 [Ryszards-MacBook-Pro.local/MymUK5rlEQ-000002] "GET http://localhost:8080/api/v1/visualize/asset/os1?format=png&layout=horizontal&history=false&available=true&highlight_current=true HTTP/1.1" from [::1]:50385 - 200 36465B in 255.816167ms
2025/10/24 09:28:44 [GraphViz-DEBUG] Creating graph with 9 states, 13 transitions
2025/10/24 09:28:44 [GraphViz-DEBUG] Creating graph with layout engine: dot, rank direction: LR
2025/10/24 09:28:44 [GraphViz-DEBUG] Found wildcard transition to FAILED
2025/10/24 09:28:44 [GraphViz-DEBUG] Creating wildcard ANY_STATE node
2025/10/24 09:28:44 [GraphViz-DEBUG] Creating wildcard edge to FAILED
2025/10/24 09:28:44 [GraphViz-DEBUG] Creating regular transitions
2025/10/24 09:28:44 [GraphViz-DEBUG] Graph creation completed successfully with 9 nodes and 12 edges
2025/10/24 09:28:44 [GraphViz-DEBUG] Performing WASM instance cleanup
2025/10/24 09:28:44 [GraphViz-DEBUG] Releasing global mutex (format: png)
```

## Benefits

### 🎯 **Clean Production Logs**
- No verbose GraphViz logging by default
- Only HTTP request logs visible
- Consistent with API logging format

### 🎯 **Debug Mode Available**
- Enable with `GRAPHVIZ_DEBUG=true`
- Detailed GraphViz operation logging
- Structured `[GraphViz-DEBUG]` prefix

### 🎯 **Complete Coverage**
- All verbose GraphViz logs converted to debug mode
- Graph creation, wildcard transitions, cleanup logs
- Mutex acquisition and release logs

### 🎯 **Performance**
- No logging overhead in production
- Debug logging only when needed
- Thread-safe with mutex protection

## Summary

The additional fixes ensure that **all verbose GraphViz logging** is now in debug mode:

1. **Graph creation logs** - Debug only
2. **Wildcard transition logs** - Debug only  
3. **Regular transition logs** - Debug only
4. **Graph completion logs** - Debug only
5. **Cleanup and mutex logs** - Debug only

This provides **clean production logs** with **debug mode available** when needed, making the logging coherent with the HTTP logging format used by the API.
