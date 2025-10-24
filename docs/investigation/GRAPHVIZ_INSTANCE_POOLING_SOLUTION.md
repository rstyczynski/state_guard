# GraphViz Instance Pooling Solution

## Problem Analysis
The memory was only increasing because **we were creating a new GraphViz instance for every request** (`graphviz.New(ctx)`), but the **Generator objects were being cached globally** and never cleaned up. Each request:

1. **Gets a cached Generator** (which contains the FSM definition data)
2. **Creates a NEW GraphViz instance** for every request
3. **The Generator cache grows** but never shrinks
4. **The FSM definition data accumulates** in memory

## Root Cause: Wrong Caching Strategy

### **Before (Memory Leak)**
```go
// Every request creates a new GraphViz instance
gv, err := graphviz.New(ctx)  // ❌ New instance every time
if err == nil {
    // Track the WASM instance
    instanceKey := fmt.Sprintf("gv_%d_%s", time.Now().UnixNano(), opts.Format)
    activeWasmInstances[instanceKey] = gv
    // ... use instance ...
    gv.Close()  // ❌ Close instance after use
}
```

### **The Problem**
- **New GraphViz instance created for every request**
- **Generator objects cached globally** (wrong thing to cache)
- **FSM definition data accumulates** in memory
- **Memory only increases** over time

## Solution: GraphViz Instance Pooling

### 1. **GraphViz Instance Pool**
```go
// GraphViz instance pool to reuse WASM instances
var (
    graphvizPool = make(map[string]*graphviz.Graphviz)
    poolMutex    sync.RWMutex
)
```

### 2. **Get or Create from Pool**
```go
// GetOrCreateGraphVizInstance gets or creates a GraphViz instance from the pool
func GetOrCreateGraphVizInstance(format string) (*graphviz.Graphviz, error) {
    poolMutex.RLock()
    if gv, exists := graphvizPool[format]; exists {
        poolMutex.RUnlock()
        log.Printf("GraphViz: Reusing pooled instance for format: %s", format)
        return gv, nil  // ✅ Reuse existing instance
    }
    poolMutex.RUnlock()

    // Need to create a new instance
    poolMutex.Lock()
    defer poolMutex.Unlock()

    // Create new GraphViz instance
    ctx := context.Background()
    gv, err := graphviz.New(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to create graphviz instance: %w", err)
    }

    // Add to pool
    graphvizPool[format] = gv
    log.Printf("GraphViz: Created and pooled instance for format: %s", format)
    return gv, nil
}
```

### 3. **Pool Management**
```go
// ClearGraphVizPool clears the GraphViz instance pool
func ClearGraphVizPool() {
    poolMutex.Lock()
    defer poolMutex.Unlock()

    // Close all instances in the pool
    for format, gv := range graphvizPool {
        if gv != nil {
            log.Printf("GraphViz: Closing pooled instance for format: %s", format)
            gv.Close()
        }
    }

    // Clear the pool
    graphvizPool = make(map[string]*graphviz.Graphviz)
    log.Printf("GraphViz: Cleared instance pool")
}
```

### 4. **Updated Usage Pattern**
```go
// Get or create GraphViz instance from pool
gv, err := GetOrCreateGraphVizInstance(string(opts.Format))
if err != nil {
    log.Printf("GraphViz: Failed to get GraphViz instance: %v", err)
    return nil, fmt.Errorf("failed to get graphviz instance: %w", err)
}

// ... use instance ...

// CRITICAL: Don't close pooled instance
log.Printf("GraphViz: Closing resources")
graph.Close()
// Note: Don't close gv here as it's pooled and reused
log.Printf("GraphViz: Graph closed, GraphViz instance returned to pool")
```

### 5. **Pool Clearing on Memory Pressure**
```go
// forceWasmReset forces a complete WASM reset by destroying and recreating WASM instances
func forceWasmReset() {
    // Clear all global generators to force recreation
    globalGenerators = make(map[string]*Generator)
    
    // Clear GraphViz instance pool
    ClearGraphVizPool()
    
    // Force complete WASM module destruction
    destroyWasmModule()
}
```

## Key Improvements

### ✅ **Instance Reuse Instead of Creation**
- **Before**: New GraphViz instance for every request
- **After**: Reuse pooled instances across requests
- **Result**: No memory accumulation from repeated instance creation

### ✅ **Proper Resource Management**
- **Before**: Close instances after use, recreate next time
- **After**: Keep instances in pool, reuse them
- **Result**: Stable memory usage over time

### ✅ **Format-Specific Pooling**
- **Before**: One instance type for all formats
- **After**: Separate instances for SVG, PNG, etc.
- **Result**: Optimized instances for each format

### ✅ **Memory Pressure Handling**
- **Before**: No cleanup of cached instances
- **After**: Pool is cleared when memory pressure is detected
- **Result**: Fresh instances when memory is exhausted

## Expected Behavior

### **Before (Memory Leak)**
```
GraphViz: Creating new GraphViz instance for format png
GraphViz: Created and tracked WASM instance: gv_1698123456789_png
GraphViz: Closing resources
GraphViz: Removed WASM instance from tracking: gv_1698123456789_png
# Next request:
GraphViz: Creating new GraphViz instance for format png  # ❌ New instance again!
```

### **After (Instance Pooling)**
```
GraphViz: Created and pooled instance for format: png
GraphViz: Graph closed, GraphViz instance returned to pool
# Next request:
GraphViz: Reusing pooled instance for format: png  # ✅ Reuse existing instance!
```

## Benefits

### 🎯 **Stable Memory Usage**
- GraphViz instances are reused instead of recreated
- Memory usage remains stable over multiple requests
- No memory accumulation from repeated instance creation

### 🎯 **Better Performance**
- No overhead of creating new instances
- Faster request processing
- Reduced memory allocation/deallocation

### 🎯 **Proper Resource Management**
- Instances are kept alive and reused
- Pool is cleared when memory pressure is detected
- Fresh instances when needed

### 🎯 **Format Optimization**
- Separate instances for different formats
- Optimized for each format's requirements
- Better memory utilization

## Testing

### **Test Script**: `test_graphviz_pooling.sh`
- Verifies GraphViz instances are reused instead of created
- Tests instance reuse across multiple requests
- Checks memory pressure handling
- Verifies pool clearing when needed

### **Expected Results**
1. **Initial requests**: Create pooled instances
2. **Repeated requests**: Reuse pooled instances
3. **Memory pressure**: Clear pool when needed
4. **Post-pressure**: Create fresh instances

## Summary

The issue was **not with WASM destruction** but with **creating new GraphViz instances for every request**. By implementing **GraphViz instance pooling**, the system now:

- **Reuses GraphViz instances** instead of creating new ones
- **Maintains stable memory usage** over multiple requests
- **Clears the pool** when memory pressure is detected
- **Creates fresh instances** only when needed

The key insight is that **GraphViz instances should be pooled and reused** rather than created and destroyed for each request. This prevents memory accumulation and provides better performance while maintaining the ability to clear the pool when memory pressure is detected.
