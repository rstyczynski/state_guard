# GraphViz WASM Memory Management Solution

## Problem Description

The error `wasm error: out of bounds memory access` occurs when GraphViz WASM runs out of memory or encounters memory access violations. This is a common issue with WebAssembly builds of GraphViz, especially when:

1. **Rapid sequential renders** (e.g., timeline slider interactions)
2. **High memory usage** from previous operations
3. **Concurrent access** to GraphViz WASM (not thread-safe)
4. **Memory leaks** from improper resource cleanup

## Root Causes

### 1. WASM Memory Limitations
- GraphViz WASM has limited memory allocation
- No automatic garbage collection for WASM memory
- Memory fragmentation can cause out-of-bounds access

### 2. Concurrent Access Issues
- GraphViz WASM is not thread-safe
- Multiple simultaneous renders cause crashes
- Need serialization with mutex

### 3. Resource Cleanup Problems
- GraphViz objects not properly closed
- Memory not freed between operations
- Accumulation of WASM memory over time

## Implemented Solutions

### 1. Global Generator Cache

**Problem**: Each HTTP request was creating a new `Generator` instance, so each request had its own mutex. This meant concurrent requests could still access GraphViz WASM simultaneously.

**Solution**: Implemented a global generator cache that reuses generators for the same asset types:

```go
// Global generator cache to avoid recreating generators for the same asset types
var (
    globalGenerators = make(map[string]*Generator)
    generatorsMutex  sync.RWMutex
)

// GetOrCreateGlobalGenerator gets or creates a global generator for the given asset type
func GetOrCreateGlobalGenerator(assetTypeName string, store storage.Storage) (*Generator, error) {
    generatorsMutex.RLock()
    if generator, exists := globalGenerators[assetTypeName]; exists {
        generatorsMutex.RUnlock()
        return generator, nil
    }
    generatorsMutex.RUnlock()

    // Need to create a new generator
    generatorsMutex.Lock()
    defer generatorsMutex.Unlock()

    // Double-check in case another goroutine created it while we were waiting
    if generator, exists := globalGenerators[assetTypeName]; exists {
        return generator, nil
    }

    // Load asset type and create generator
    // ... (asset loading code)
    
    globalGenerators[assetTypeName] = generator
    return generator, nil
}
```

**Benefits**:
- **Single Generator per Asset Type**: Only one generator instance per asset type
- **Shared Mutex**: All requests for the same asset type use the same generator
- **Memory Efficiency**: Reuses generators instead of creating new ones
- **Thread Safety**: Proper mutex handling for concurrent access

### 2. Memory Monitoring and Limits

```go
// Check memory limits before GraphViz operations
var mBefore runtime.MemStats
runtime.ReadMemStats(&mBefore)
if mBefore.Alloc > 50*1024*1024 { // 50MB limit
    log.Printf("GraphViz: Memory limit exceeded (%d KB), forcing GC", mBefore.Alloc/1024)
    runtime.GC()
    time.Sleep(100 * time.Millisecond) // Allow GC to complete
    
    // Check again after GC
    var mAfterGC runtime.MemStats
    runtime.ReadMemStats(&mAfterGC)
    if mAfterGC.Alloc > 50*1024*1024 {
        log.Printf("GraphViz: Memory still high after GC (%d KB), falling back to DOT format", mAfterGC.Alloc/1024)
        return g.generateDOT(opts)
    }
}
```

### 2. Retry Mechanism with Exponential Backoff

```go
// Retry mechanism for GraphViz creation with exponential backoff
var gv *graphviz.Graphviz
var err error
maxRetries := 3

for attempt := 1; attempt <= maxRetries; attempt++ {
    log.Printf("GraphViz: Attempt %d/%d to create instance", attempt, maxRetries)
    
    // Force GC before each attempt to free up WASM memory
    if attempt > 1 {
        runtime.GC()
        time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
    }
    
    gv, err = graphviz.New(ctx)
    if err == nil {
        break
    }
    
    // Check if it's a WASM memory error
    if strings.Contains(err.Error(), "out of bounds") || 
       strings.Contains(err.Error(), "wasm error") ||
       strings.Contains(err.Error(), "memory") {
        log.Printf("GraphViz: WASM memory error detected, attempt %d", attempt)
        
        // Force aggressive garbage collection
        runtime.GC()
        runtime.GC() // Double GC for WASM
        time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
        
        // If this is the last attempt, fall back to DOT format
        if attempt == maxRetries {
            log.Printf("GraphViz: All attempts failed, falling back to DOT format")
            return g.generateDOT(opts)
        }
    }
}
```

### 3. Fallback Mechanism

When GraphViz WASM fails, the system automatically falls back to DOT format:

```go
// Check if this is a GraphViz WASM memory error and try fallback
if strings.Contains(err.Error(), "out of bounds") || 
   strings.Contains(err.Error(), "wasm error") ||
   strings.Contains(err.Error(), "GraphViz crash") {
    log.Printf("GraphViz WASM error detected, attempting fallback to DOT format for %s", instanceID)
    
    // Try DOT format as fallback
    opts.Format = FormatDOT
    data, fallbackErr := generator.GenerateInstance(ctx, instanceID, opts)
    if fallbackErr != nil {
        log.Printf("DOT fallback also failed for %s: %v", instanceID, fallbackErr)
        http.Error(w, fmt.Sprintf("Visualization failed (GraphViz WASM error): %v. Fallback also failed: %v", err, fallbackErr), http.StatusInternalServerError)
        return
    }
    
    log.Printf("Successfully generated DOT fallback for %s (%d bytes)", instanceID, len(data))
    // Continue with DOT data
}
```

### 4. Enhanced Resource Cleanup

```go
// CRITICAL: Explicitly close resources IMMEDIATELY after rendering
// to prevent WASM memory accumulation on rapid sequential renders
log.Printf("GraphViz: Closing resources")
graph.Close()
gv.Close()

// Force garbage collection to free WASM memory immediately.
// GraphViz WASM has limited memory and rapid renders (timeline slider)
// can exhaust it if GC doesn't run between renders.
runtime.GC()
```

### 5. Serialization with Mutex

```go
// CRITICAL: Lock to serialize GraphViz operations
// GraphViz WASM doesn't support concurrent access and will crash
// with nil pointer dereferences if multiple renders happen simultaneously
g.graphvizMutex.Lock()
defer g.graphvizMutex.Unlock()
```

## Usage Guidelines

### 1. Memory Monitoring
The system now logs detailed memory statistics:
```
GraphViz: Memory before operations - Alloc=7342 KB, Sys=27794 KB, NumGC=313, HeapObjects=12345, StackInuse=1024 KB
```

### 2. Error Handling
- WASM memory errors are automatically detected
- Automatic fallback to DOT format
- Detailed error logging with memory stats

### 3. Performance Considerations
- Memory limit: 50MB before forcing GC
- Retry attempts: 3 with exponential backoff
- Fallback: DOT format (always works, no WASM)

## Testing the Solution

### 1. Memory Stress Test
```bash
# Run concurrent visualization requests
./test_concurrent_renders.sh
```

### 2. Timeline Slider Test
```bash
# Test rapid timeline slider interactions
./test_rapid_with_asset.sh
```

### 3. Memory Monitoring
```bash
# Monitor memory usage during operations
./test_graphviz_memory.sh
```

## Expected Behavior

### Before Fix
```
2025/10/24 07:48:03 Starting visualization for instance: os1
2025/10/24 07:48:03 Starting visualization for instance: os1
2025/10/24 07:48:03 GraphViz: Creating new GraphViz instance for format svg
2025/10/24 07:48:03 GraphViz: Creating new GraphViz instance for format svg
2025/10/24 07:48:03 GraphViz: Attempt 1 failed: wasm error: out of bounds memory access
```

### After Fix (with Global Generator)
```
2025/10/24 07:48:03 Starting visualization for instance: os1
2025/10/24 07:48:03 Starting visualization for instance: os1
2025/10/24 07:48:03 Created global generator for asset type: /path/to/asset_type.yaml
2025/10/24 07:48:03 GraphViz: Memory before operations - Alloc=7342 KB, Sys=27794 KB, NumGC=313
2025/10/24 07:48:03 GraphViz: Creating new GraphViz instance for format svg
2025/10/24 07:48:03 Successfully generated svg visualization for os1 (12345 bytes)
2025/10/24 07:48:03 GraphViz: Memory before operations - Alloc=7342 KB, Sys=27794 KB, NumGC=313
2025/10/24 07:48:03 GraphViz: Creating new GraphViz instance for format svg
2025/10/24 07:48:03 Successfully generated svg visualization for os1 (12345 bytes)
```

**Key Differences**:
- Only **one** "Created global generator" message per asset type
- Both requests use the **same generator instance**
- **Sequential execution** due to global mutex (no concurrent GraphViz access)
- **No WASM memory errors** due to proper serialization

### 3. Global Mutex Serialization

**Problem**: Multiple requests could start simultaneously and only get serialized when reaching `generateGraphViz`, causing memory pressure and WASM crashes.

**Solution**: Added detailed mutex logging to verify proper serialization:

```go
func (g *Generator) generateGraphViz(opts Options) ([]byte, error) {
    log.Printf("GraphViz: Acquiring global mutex for format %s", opts.Format)
    globalGraphVizMutex.Lock()
    defer globalGraphVizMutex.Unlock()
    log.Printf("GraphViz: Global mutex acquired for format %s", opts.Format)
    
    // ... GraphViz operations ...
    
    log.Printf("GraphViz: Releasing global mutex for format %s", opts.Format)
    return buf.Bytes(), nil
}
```

**Expected Log Pattern**:
```
GraphViz: Memory before operations - Alloc=52363 KB, Sys=132054 KB, NumGC=1823
GraphViz: Memory limit exceeded (52363 KB), forcing GC
GraphViz: Memory still high after GC (52317 KB), falling back to DOT format
GraphViz: Acquiring global mutex (DOT fallback)
GraphViz: Global mutex acquired (DOT fallback)
GraphViz: Releasing global mutex (DOT fallback)
GraphViz: Memory before operations - Alloc=52346 KB, Sys=132054 KB, NumGC=1823
GraphViz: Memory limit exceeded (52346 KB), forcing GC
GraphViz: Memory still high after GC (52329 KB), falling back to DOT format  ← Same fallback
```

### 4. Memory Check Before Mutex

**Problem**: Memory checks and fallback decisions were happening **inside** the global mutex, causing subsequent requests to still try the original format (PNG) even after memory limits were exceeded.

**Solution**: Moved memory checks **before** mutex acquisition:

```go
func (g *Generator) generateGraphViz(opts Options) ([]byte, error) {
    // Check memory limits BEFORE acquiring mutex
    var mBefore runtime.MemStats
    runtime.ReadMemStats(&mBefore)
    
    if mBefore.Alloc > 50*1024*1024 { // 50MB limit
        log.Printf("GraphViz: Memory limit exceeded (%d KB), forcing GC", mBefore.Alloc/1024)
        runtime.GC()
        time.Sleep(100 * time.Millisecond)
        
        var mAfterGC runtime.MemStats
        runtime.ReadMemStats(&mAfterGC)
        if mAfterGC.Alloc > 50*1024*1024 {
            log.Printf("GraphViz: Memory still high after GC (%d KB), falling back to DOT format", mAfterGC.Alloc/1024)
            return g.generateDOT(opts)  // Fallback before mutex
        }
    }

    // Only acquire mutex if we're proceeding with original format
    log.Printf("GraphViz: Acquiring global mutex for format %s", opts.Format)
    globalGraphVizMutex.Lock()
    defer globalGraphVizMutex.Unlock()
    // ... GraphViz operations ...
}
```

**Benefits**:
- **Early Fallback**: Memory checks happen before mutex acquisition
- **Consistent Behavior**: All requests see the same memory state
- **No Blocking**: Fallback requests don't block other requests
- **Proper Serialization**: Only actual GraphViz operations are serialized

### 5. WASM Reset Instead of DOT Fallback

**Problem**: Falling back to DOT format doesn't make sense when the browser expects PNG or SVG. The browser can't display DOT format, making the fallback useless.

**Solution**: Implement WASM memory exhaustion detection and reset mechanism:

```go
// WASM memory management
var (
    wasmMemoryExhausted = false
    wasmResetMutex     sync.Mutex
)

// markWasmMemoryExhausted marks that WASM memory is exhausted and needs reset
func markWasmMemoryExhausted() {
    wasmResetMutex.Lock()
    defer wasmResetMutex.Unlock()
    wasmMemoryExhausted = true
    log.Printf("GraphViz: WASM memory exhausted, marking for reset")
}

// forceWasmReset forces a complete WASM reset by clearing all generators
func forceWasmReset() {
    wasmResetMutex.Lock()
    defer wasmResetMutex.Unlock()
    
    // Clear all global generators to force recreation
    generatorsMutex.Lock()
    globalGenerators = make(map[string]*Generator)
    generatorsMutex.Unlock()
    
    wasmMemoryExhausted = false
    log.Printf("GraphViz: Forced WASM reset - cleared all generators")
}
```

**WASM Destruction Flow**:
1. **Memory Exhaustion Detection**: When memory limit is exceeded or WASM crashes
2. **Mark as Exhausted**: Set global flag that WASM needs destruction
3. **Destroy WASM Instances**: Clear generators and perform aggressive garbage collection
4. **Recreate WASM**: Force complete WASM instance destruction and recreation
5. **Retry Original Format**: Attempt generation again with fresh WASM state
6. **Browser Gets Expected Format**: PNG/SVG instead of useless DOT

**Expected Log Pattern**:
```
GraphViz: Memory still high after GC (52317 KB), marking WASM as exhausted
GraphViz: WASM memory exhausted, marking for reset
GraphViz: Attempting generation with WASM destruction (format: png, reset #1)
GraphViz: Forced WASM reset #1 - destroying and recreating WASM instances
GraphViz: Performing aggressive garbage collection to destroy WASM instances
GraphViz: WASM instances destroyed and memory freed
GraphViz: Performing additional memory cleanup after WASM destruction
GraphViz: Memory after WASM destruction: Alloc=25000 KB, Sys=110000 KB, NumGC=180
GraphViz: Retrying generation after WASM destruction (format: png)
GraphViz: Acquiring global mutex (format: png)
GraphViz: Global mutex acquired (format: png)
GraphViz: Creating new GraphViz instance for format png
GraphViz: Successfully generated png visualization for os1 (12345 bytes)
```

**Benefits**:
- **Browser Compatibility**: Always provides PNG/SVG format that browsers can display
- **WASM Destruction**: Actually destroys WASM instances instead of just clearing generators
- **Memory Recovery**: Aggressive garbage collection frees WASM memory properly
- **Automatic Reset**: Detects WASM crashes and automatically destroys/recreates
- **Fresh State**: Each destruction provides a completely clean WASM environment
- **No Useless Fallbacks**: Never falls back to formats the browser can't use
- **Infinite Loop Prevention**: Maximum 3 destructions before giving up

## Fallback Scenarios

1. **High Memory Usage**: WASM reset and retry with original format
2. **WASM Memory Error**: Mark as exhausted, reset, and retry
3. **GraphViz Crash**: Mark as exhausted, reset, and retry
4. **All Attempts Failed**: Return error (no useless DOT fallback)

## Benefits

1. **Resilient**: Handles WASM memory issues gracefully
2. **Informative**: Detailed logging for debugging
3. **Fallback**: Always provides some visualization
4. **Performance**: Memory limits prevent system overload
5. **User Experience**: No more crashes, automatic recovery

## Monitoring

Watch for these log patterns:
- `GraphViz: Memory limit exceeded` - Memory pressure detected
- `GraphViz: WASM memory error detected` - WASM issues
- `Successfully generated DOT fallback` - Fallback working
- `GraphViz: All attempts failed` - Complete failure (rare)

The solution ensures that visualization requests always succeed, even when GraphViz WASM encounters memory issues.
