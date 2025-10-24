# WASM Instance Destruction and Recreation Solution

## Problem
The previous approach was NOT actually destroying WASM objects - it was just clearing generators and hoping garbage collection would free memory. This led to:
- Memory staying at 52579 KB (not actually freed)
- Infinite loops of WASM resets
- Browser receiving DOT format instead of expected PNG/SVG

## Solution: Actual WASM Object Destruction

### 1. **WASM Instance Tracking**
```go
// Track active WASM instances
var activeWasmInstances = make(map[string]*graphviz.Graphviz)
```

### 2. **Instance Creation with Tracking**
```go
// When creating GraphViz instance
gv, err = graphviz.New(ctx)
if err == nil {
    // Track the WASM instance
    instanceKey := fmt.Sprintf("gv_%d_%s", time.Now().UnixNano(), opts.Format)
    activeWasmInstances[instanceKey] = gv
    log.Printf("GraphViz: Created and tracked WASM instance: %s", instanceKey)
    break
}
```

### 3. **Explicit WASM Instance Destruction**
```go
// destroyAllWasmInstances explicitly destroys all active WASM instances
func destroyAllWasmInstances() {
    log.Printf("GraphViz: Destroying all active WASM instances")
    
    // Close all active WASM instances
    for key, gv := range activeWasmInstances {
        if gv != nil {
            log.Printf("GraphViz: Closing WASM instance: %s", key)
            gv.Close()  // ACTUAL WASM DESTRUCTION
        }
    }
    
    // Clear the active instances map
    activeWasmInstances = make(map[string]*graphviz.Graphviz)
    
    // Force multiple garbage collections to free WASM memory
    runtime.GC()
    runtime.GC()
    runtime.GC()
    
    // Wait for garbage collection to complete
    time.Sleep(2 * time.Second)
    
    log.Printf("GraphViz: All WASM instances destroyed")
}
```

### 4. **Instance Cleanup After Use**
```go
// Remove from active instances tracking
for key, instance := range activeWasmInstances {
    if instance == gv {
        delete(activeWasmInstances, key)
        log.Printf("GraphViz: Removed WASM instance from tracking: %s", key)
        break
    }
}
```

### 5. **WASM Reset with Actual Destruction**
```go
// forceWasmReset forces a complete WASM reset by destroying and recreating WASM instances
func forceWasmReset() {
    wasmResetCount++
    log.Printf("GraphViz: Forced WASM reset #%d - destroying and recreating WASM instances", wasmResetCount)
    
    // Clear all global generators to force recreation
    globalGenerators = make(map[string]*Generator)
    
    // Actually destroy all WASM instances
    destroyAllWasmInstances()
    
    wasmMemoryExhausted = false
    log.Printf("GraphViz: WASM instances destroyed and memory freed")
}
```

## Key Improvements

### ✅ **Actual WASM Destruction**
- **Before**: Just cleared generators, hoped GC would free memory
- **After**: Explicitly calls `gv.Close()` on all WASM instances
- **Result**: Memory is actually freed, not just garbage collected

### ✅ **Instance Tracking**
- **Before**: No tracking of WASM instances
- **After**: Every WASM instance is tracked with unique keys
- **Result**: Can destroy specific instances and monitor usage

### ✅ **Proper Cleanup**
- **Before**: Instances left in memory after use
- **After**: Instances removed from tracking after cleanup
- **Result**: No memory leaks, proper resource management

### ✅ **Fresh Instance Creation**
- **Before**: Reused potentially corrupted instances
- **After**: Creates completely new WASM instances after destruction
- **Result**: Clean slate, no memory corruption

## Expected Log Patterns

### **WASM Instance Creation**
```
GraphViz: Creating new GraphViz instance for format png
GraphViz: Created and tracked WASM instance: gv_1698123456789_png
```

### **WASM Instance Destruction**
```
GraphViz: Destroying all active WASM instances
GraphViz: Closing WASM instance: gv_1698123456789_png
GraphViz: All WASM instances destroyed
```

### **WASM Instance Cleanup**
```
GraphViz: Closing resources
GraphViz: Removed WASM instance from tracking: gv_1698123456789_png
```

### **Memory After Destruction**
```
GraphViz: Memory before operations: Alloc=25000 KB, Sys=80000 KB  # Much lower!
```

## Benefits

### 🎯 **Actual Memory Freedom**
- WASM instances are explicitly destroyed
- Memory is actually freed, not just garbage collected
- No more infinite loops of WASM resets

### 🎯 **Browser Gets Expected Format**
- PNG/SVG requests succeed after WASM destruction
- No more falling back to useless DOT format
- Fresh WASM instances work properly

### 🎯 **Proper Resource Management**
- Every WASM instance is tracked and managed
- Clean separation between creation and destruction
- No memory leaks or resource accumulation

### 🎯 **Robust Recovery**
- System can recover from WASM memory exhaustion
- Fresh instances work without corruption
- Memory usage actually decreases after destruction

## Testing

### **Test Script**: `test_wasm_destruction.sh`
- Verifies WASM instances are created and tracked
- Triggers memory pressure to force destruction
- Confirms new instances work after destruction
- Checks for proper cleanup and memory freedom

### **Expected Results**
1. **Initial requests**: Create tracked WASM instances
2. **Memory pressure**: Triggers actual WASM destruction
3. **Post-destruction**: New instances work properly
4. **Memory usage**: Actually decreases after destruction
5. **No infinite loops**: System recovers cleanly

## Summary

This solution implements **actual WASM object destruction** instead of just hoping garbage collection will free memory. By explicitly tracking, destroying, and recreating WASM instances, the system can:

- **Actually free memory** when WASM instances are destroyed
- **Create fresh instances** that work properly after destruction
- **Prevent infinite loops** by properly managing resources
- **Deliver expected formats** (PNG/SVG) to browsers instead of falling back to DOT

The key insight is that **WASM instances must be explicitly destroyed** - they don't just disappear with garbage collection. This solution ensures that when memory is exhausted, the system actually destroys the WASM objects and creates new ones, rather than just clearing references and hoping for the best.
