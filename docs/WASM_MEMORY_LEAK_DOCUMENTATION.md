# GraphViz WASM Memory Leak - Technical Documentation

## Executive Summary

The visualization system uses `go-graphviz` (v0.2.9) which has a **known memory leak** where WASM linear memory is not freed after rendering operations. This document explains the root cause, why standard cleanup methods don't work, and how we handle it in production.

## The Problem

### Symptom
- Memory grows continuously with each visualization render
- Memory does NOT decrease after `runtime.GC()` calls
- Eventually triggers "out of bounds memory access" errors
- Affects both SVG and PNG rendering equally

### Impact
- Server process memory grows from ~10MB to 50MB+ over hundreds of renders
- Without intervention, eventually crashes with WASM out-of-bounds error
- Particularly problematic for long-running server processes

### Reproduction
```bash
# Run 100 rapid visualization renders
for i in {1..100}; do
  curl http://localhost:8080/api/v1/visualize/asset/test123?format=svg
done

# Observe memory growth in logs:
# GraphViz: Memory before operations - Alloc=7342 KB   (first request)
# GraphViz: Memory before operations - Alloc=15234 KB  (after 50 requests)
# GraphViz: Memory before operations - Alloc=52000 KB  (memory limit hit)
```

## Root Cause Analysis

### WASM Linear Memory Architecture

1. **GraphViz uses WebAssembly**
   - `go-graphviz` embeds `graphviz.wasm` (compiled from C++ Graphviz)
   - WASM has its own linear memory space, separate from Go's heap
   - WASM memory is managed by the WASM runtime, not Go's GC

2. **What Happens During a Render**
   ```go
   gv, _ := graphviz.New(ctx)        // WASM module loaded (first time only)
   graph, _ := gv.Graph()            // Allocates WASM memory for graph
   gv.Render(ctx, graph, SVG, &buf)  // Allocates WASM memory for layout
   graph.Close()                      // ❌ Should free WASM memory, but doesn't
   gv.Close()                         // ❌ Should free WASM memory, but doesn't
   ```

3. **The Bug** (confirmed in go-graphviz issue #111)
   - `graph.Close()` only closes the Go wrapper
   - `gv.Close()` only closes the Go context
   - **WASM linear memory is NEVER freed**
   - Memory accumulates until process restart

### Why Standard Solutions Don't Work

| Approach | Why It Fails |
|----------|--------------|
| `graph.Close()` | Only closes Go wrapper, WASM memory remains allocated |
| `gv.Close()` | Only closes GVC context, WASM module persists |
| `runtime.GC()` | Go's GC doesn't manage WASM linear memory |
| Clear caches | Only affects Go-side structures (Generator, pools) |
| Create new instances | WASM module is loaded once, reused for all instances |
| WASM finalizers | Not implemented in go-graphviz |

### Confirmed by Community

From [go-graphviz issue #111](https://github.com/goccy/go-graphviz/issues/111):
> "Each time I rendered the svg file, my go process memory shoot up significantly
> and it doesn't come down to expected level after garbage collected...
> the pprof profiling data indicates the WASM component is retaining substantial
> amounts of memory."

## Our Workaround: Memory-Based Process Restart

Since WASM memory cannot be freed programmatically, we monitor memory usage and restart the process when it exceeds a threshold.

### Configuration

```bash
# Environment variables (all optional with sensible defaults)
export GRAPHVIZ_MEMORY_LIMIT_BEFORE=50  # MB - restart before this limit
export GRAPHVIZ_MEMORY_LIMIT_AFTER=60   # MB - restart after render if exceeded
export GRAPHVIZ_MEMORY_WARNING=40       # MB - log warnings
export GRAPHVIZ_MAX_WASM_RESETS=2       # Attempts before process restart
export GRAPHVIZ_MAX_PROCESS_RESTARTS=1  # Max process restarts
export GRAPHVIZ_DEBUG=true              # Enable verbose logging
```

### How It Works

```go
// Before each render
var m runtime.MemStats
runtime.ReadMemStats(&m)

if m.Alloc > 50*1024*1024 {  // 50MB threshold
    log.Printf("WASM memory exhausted, restarting process")
    os.Exit(1)  // ONLY way to free WASM memory
}
```

### Process Lifecycle

```
┌──────────────────────────────────────────────────────────┐
│ Process Start (Fresh WASM, ~10MB memory)                 │
└────────────────────┬─────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────────┐
│ Handle Visualization Requests                            │
│ - Check memory before each render                        │
│ - Render with GraphViz WASM                              │
│ - Close graph (doesn't free WASM memory)                 │
│ - Memory accumulates: 10MB → 20MB → 30MB → ...          │
└────────────────────┬─────────────────────────────────────┘
                     │
                     ▼
         ┌───────────────────────┐
         │ Memory < 50MB?        │
         └───┬───────────────┬───┘
             │ Yes           │ No
             │               │
             ▼               ▼
    ┌─────────────┐   ┌──────────────────────┐
    │ Continue    │   │ os.Exit(1)           │
    │ Serving     │   │ Process Manager      │
    └──────┬──────┘   │ Restarts Process     │
           │          └──────────┬───────────┘
           │                     │
           └─────────────────────┘
                     │
                     ▼
          ┌──────────────────────┐
          │ Fresh Process Start  │
          │ WASM memory cleared  │
          └──────────────────────┘
```

## Production Deployment Recommendations

### 1. Process Manager (Required)

**Systemd Example:**
```ini
[Unit]
Description=FSM Visualization API
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/api
Restart=always
RestartSec=1
Environment="GRAPHVIZ_MEMORY_LIMIT_BEFORE=50"

[Install]
WantedBy=multi-user.target
```

**Kubernetes Example:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fsm-api
spec:
  replicas: 3  # Multiple replicas for zero-downtime
  template:
    spec:
      containers:
      - name: api
        image: fsm-api:latest
        env:
        - name: GRAPHVIZ_MEMORY_LIMIT_BEFORE
          value: "50"
        resources:
          limits:
            memory: "128Mi"  # Pod limit higher than WASM limit
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
```

### 2. Memory Threshold Tuning

Choose threshold based on your usage pattern:

| Usage Pattern | Recommended Limit | Restart Frequency |
|---------------|-------------------|-------------------|
| Low traffic (<10 req/min) | 50MB | Every few hours |
| Medium traffic (10-50 req/min) | 40MB | Every hour |
| High traffic (>50 req/min) | 30MB | Every 30 minutes |

### 3. Monitoring

Monitor these metrics:
- Process restarts per hour (should be low, 1-2 per hour max)
- Memory usage trend (should sawtooth: grow → restart → reset)
- Failed requests during restart (should be zero with load balancer)

### 4. Load Balancing (Recommended)

Use multiple replicas behind a load balancer for zero-downtime restarts:
```
┌───────────┐
│  Client   │
└─────┬─────┘
      │
      ▼
┌────────────────┐
│ Load Balancer  │
└────┬─────┬─────┘
     │     │
     ▼     ▼
┌─────┐ ┌─────┐
│API 1│ │API 2│  ← When API 1 restarts, API 2 handles traffic
└─────┘ └─────┘
```

## Alternative Solutions (Future)

### 1. External GraphViz Process
```go
// Shell out to command-line graphviz
cmd := exec.Command("dot", "-Tsvg")
cmd.Stdin = bytes.NewReader(dotCode)
output, _ := cmd.Output()
// Process memory freed when command exits
```
**Pros:** No WASM memory leak
**Cons:** Slower, requires graphviz binary installed

### 2. CGO-based GraphViz
```bash
CGO_ENABLED=1 go build
```
**Pros:** Proper memory cleanup with C finalizers
**Cons:** Platform-specific binaries, more complex build

### 3. Fix Upstream (Contribute to go-graphviz)
Required changes in `go-graphviz`:
- Expose WASM memory management functions
- Implement proper finalizers for WASM cleanup
- Add explicit memory free functions

### 4. Pure Go SVG Generation
Replace GraphViz with pure Go implementation:
- No WASM dependency
- Full control over memory
- May lack GraphViz layout algorithms

## Testing

### Memory Leak Test
```bash
# Start server
go run cmd/api/main.go

# Generate 100 renders and watch memory
for i in {1..100}; do
  curl -s http://localhost:8080/api/v1/visualize/asset/test123?format=svg > /dev/null
  echo "Request $i completed"
done

# Check logs for memory growth
grep "Memory before operations" server.log
```

### Expected Output
```
GraphViz: Memory before operations - Alloc=7342 KB    # Request 1
GraphViz: Memory before operations - Alloc=8123 KB    # Request 10
GraphViz: Memory before operations - Alloc=15234 KB   # Request 50
GraphViz: Memory before operations - Alloc=23456 KB   # Request 75
GraphViz: Memory before operations - Alloc=35678 KB   # Request 90
GraphViz: Memory before operations - Alloc=48000 KB   # Request 99
GraphViz: Memory limit exceeded (52000 KB > 50 MB)    # Request 100
GraphViz: WASM memory leak requires process restart
# Process restarts, memory resets to ~10MB
```

## References

- **Upstream Bug Report:** https://github.com/goccy/go-graphviz/issues/111
- **go-graphviz Library:** https://github.com/goccy/go-graphviz
- **WebAssembly Memory Model:** https://webassembly.github.io/spec/core/syntax/modules.html#memories
- **Go WASM Documentation:** https://go.dev/wiki/WebAssembly

## Revision History

| Date | Version | Changes |
|------|---------|---------|
| 2025-10-24 | 1.0 | Initial documentation of WASM memory leak issue and workaround |

---

**Status:** ⚠️ **KNOWN ISSUE** - Workaround implemented, waiting for upstream fix

**Severity:** Medium - Workaround is production-ready but requires process manager

**Owner:** Upstream (go-graphviz maintainers)
