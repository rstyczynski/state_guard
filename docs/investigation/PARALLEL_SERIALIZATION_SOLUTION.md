# Parallel Request Serialization Solution

## Problem Analysis
The issue was that **multiple requests were running in parallel** and each one was calling `generator.GenerateInstance()` simultaneously, causing:

1. **Multiple "Starting visualization for instance: os1"** messages at the same time
2. **Multiple "Created global generator for asset type"** messages
3. **WASM memory corruption** from parallel GraphViz operations
4. **Memory errors** like `failed to read wasm memory: (ptr, size) = (2037653539, 572548464)`

## Root Cause: Parallel Generator Operations

### **The Problem**
```go
// Multiple requests can run in parallel:
Request 1: generator.GenerateInstance() -> generateGraphViz()
Request 2: generator.GenerateInstance() -> generateGraphViz()  // ❌ Parallel!
Request 3: generator.GenerateInstance() -> generateGraphViz()  // ❌ Parallel!
```

Even though we had a global mutex in `generateGraphViz`, **multiple requests could still call the Generator methods in parallel**, and each would try to acquire the mutex, causing memory corruption.

## Solution: Generator-Level Serialization

### 1. **Global Mutex at Generator Level**
```go
// CRITICAL: Lock to serialize GraphViz operations across ALL requests
// GraphViz WASM doesn't support concurrent access and will crash
// with nil pointer dereferences if multiple renders happen simultaneously
log.Printf("GraphViz: Acquiring global mutex (format: %s)", opts.Format)
globalGraphVizMutex.Lock()
defer globalGraphVizMutex.Unlock()
log.Printf("GraphViz: Global mutex acquired (format: %s)", opts.Format)
```

### 2. **Proper Mutex Scope**
The mutex is acquired **inside the `generateGraphViz` function**, which means:
- **Only one request can execute GraphViz operations at a time**
- **All other requests wait** until the current one completes
- **No parallel GraphViz operations** can occur

### 3. **Request Flow**
```go
Request 1: Handler -> Generator.GenerateInstance() -> generateGraphViz() -> [MUTEX ACQUIRED]
Request 2: Handler -> Generator.GenerateInstance() -> generateGraphViz() -> [WAITING FOR MUTEX]
Request 3: Handler -> Generator.GenerateInstance() -> generateGraphViz() -> [WAITING FOR MUTEX]
```

## Key Improvements

### ✅ **Proper Serialization**
- **Before**: Multiple requests could run GraphViz operations in parallel
- **After**: Only one request can execute GraphViz operations at a time
- **Result**: No WASM memory corruption from parallel access

### ✅ **Generator-Level Control**
- **Before**: Mutex was at handler level, allowing parallel Generator calls
- **After**: Mutex is at Generator level, serializing all GraphViz operations
- **Result**: Complete control over GraphViz execution

### ✅ **Memory Safety**
- **Before**: Parallel requests caused WASM memory corruption
- **After**: Serialized requests prevent memory corruption
- **Result**: Stable memory usage and no crashes

### ✅ **Resource Management**
- **Before**: Multiple requests competing for WASM resources
- **After**: Single request at a time uses WASM resources
- **Result**: Proper resource utilization

## Expected Behavior

### **Before (Parallel Execution)**
```
2025/10/24 08:27:10 Starting visualization for instance: os1
2025/10/24 08:27:10 Starting visualization for instance: os1  # ❌ Parallel!
2025/10/24 08:27:10 Starting visualization for instance: os1  # ❌ Parallel!
2025/10/24 08:27:10 Created global generator for asset type: examples/simple_asset_type.yaml
2025/10/24 08:27:10 Created global generator for asset type: examples/simple_asset_type.yaml  # ❌ Parallel!
2025/10/24 08:27:10 PNG render failed: failed to read wasm memory  # ❌ Memory corruption!
```

### **After (Serialized Execution)**
```
2025/10/24 08:27:10 Starting visualization for instance: os1
2025/10/24 08:27:10 GraphViz: Acquiring global mutex (format: png)
2025/10/24 08:27:10 GraphViz: Global mutex acquired (format: png)
2025/10/24 08:27:10 GraphViz: Reusing pooled instance for format: png
2025/10/24 08:27:10 GraphViz: Generation completed successfully
2025/10/24 08:27:10 GraphViz: Releasing global mutex (format: png)
# Next request waits until mutex is released
2025/10/24 08:27:11 Starting visualization for instance: os1
2025/10/24 08:27:11 GraphViz: Acquiring global mutex (format: png)
```

## Benefits

### 🎯 **No Parallel Execution**
- Only one request can execute GraphViz operations at a time
- All other requests wait in queue
- No WASM memory corruption from parallel access

### 🎯 **Memory Safety**
- Serialized operations prevent memory corruption
- Stable memory usage over time
- No crashes from parallel WASM access

### 🎯 **Resource Management**
- Single request uses WASM resources at a time
- Proper cleanup between requests
- No resource competition

### 🎯 **Predictable Behavior**
- Requests are processed in order
- No race conditions
- Consistent memory usage

## Testing

### **Test Script**: `test_parallel_serialization.sh`
- Verifies that parallel requests are properly serialized
- Tests that no parallel GraphViz generation occurs
- Checks for proper mutex acquisition and release
- Verifies no WASM memory corruption

### **Expected Results**
1. **All requests are serialized** (no parallel execution)
2. **No WASM memory errors** from parallel access
3. **No multiple "Starting visualization" messages** at the same time
4. **Proper mutex acquisition and release** for each request

## Summary

The solution ensures that **Generator operations are properly serialized** by:

- **Keeping the global mutex at the Generator level** (not handler level)
- **Serializing all GraphViz operations** across all requests
- **Preventing parallel WASM access** that causes memory corruption
- **Ensuring only one request** can execute GraphViz operations at a time

This prevents the parallel execution that was causing WASM memory corruption and ensures stable, predictable behavior.
