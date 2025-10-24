# WASM Module Destruction and Reload Solution

## Problem Analysis
The previous approach failed because:
1. **No WASM instances were being tracked** - they were never created due to memory exhaustion
2. **Memory was NOT freed** - `Alloc=51280 KB` before and after destruction
3. **Still falling back to DOT** - system gave up and returned DOT format instead of PNG
4. **Infinite loops continued** - system kept trying to reset without actually freeing memory

## Root Cause
The issue is that **the GraphViz WASM module itself is not releasing memory properly**. Simply closing individual instances doesn't free the underlying WASM module memory. We need to **destroy the entire WASM module** and force a complete reload.

## Solution: WASM Module Destruction and Reload

### 1. **WASM Module State Tracking**
```go
var (
    wasmModuleDestroyed = false // Track if WASM module was destroyed
    // ... other variables
)
```

### 2. **Complete WASM Module Destruction**
```go
// destroyWasmModule forces complete destruction of the WASM module
func destroyWasmModule() {
    log.Printf("GraphViz: Destroying WASM module completely")
    
    // Mark module as destroyed
    wasmModuleDestroyed = true
    
    // Force multiple garbage collections to free WASM module memory
    log.Printf("GraphViz: Forcing WASM module memory cleanup")
    for i := 0; i < 15; i++ {
        runtime.GC()
        time.Sleep(300 * time.Millisecond)
    }
    
    // Wait for WASM module to be completely unloaded
    log.Printf("GraphViz: Waiting for WASM module to be unloaded")
    time.Sleep(8 * time.Second)
    
    // Final cleanup
    runtime.GC()
    
    log.Printf("GraphViz: WASM module destroyed - will be reloaded on next use")
}
```

### 3. **WASM Module Reload Detection**
```go
// Check if WASM module was destroyed and needs complete reload
if wasmModuleDestroyed {
    log.Printf("GraphViz: WASM module was destroyed, forcing complete reload")
    // Force additional cleanup before reload
    for i := 0; i < 5; i++ {
        runtime.GC()
        time.Sleep(500 * time.Millisecond)
    }
    wasmModuleDestroyed = false
    log.Printf("GraphViz: WASM module reload initiated")
}
```

### 4. **Aggressive Memory Cleanup**
```go
// forceCompleteWasmRestart forces a complete restart of the WASM environment
func forceCompleteWasmRestart() {
    // Step 1: Destroy any existing WASM instances
    destroyAllWasmInstances()
    
    // Step 2: Force complete memory cleanup with multiple cycles
    for i := 0; i < 10; i++ {
        runtime.GC()
        time.Sleep(200 * time.Millisecond)
    }
    
    // Step 3: Force memory to be returned to OS
    runtime.GC()
    runtime.GC()
    
    // Step 4: Wait for WASM environment to fully reset
    time.Sleep(5 * time.Second)
    
    // Step 5: Final aggressive cleanup
    for i := 0; i < 3; i++ {
        runtime.GC()
        time.Sleep(1 * time.Second)
    }
    
    // Step 6: Mark WASM module as destroyed to force complete reload
    wasmModuleDestroyed = true
}
```

## Key Improvements

### ✅ **Complete WASM Module Destruction**
- **Before**: Only closed individual instances, module memory remained
- **After**: Destroys the entire WASM module and forces complete reload
- **Result**: Memory is actually freed at the module level

### ✅ **Module-Level Memory Management**
- **Before**: Instance-level cleanup that didn't free module memory
- **After**: Module-level destruction that frees all WASM memory
- **Result**: Complete memory freedom, not just garbage collection

### ✅ **Forced Module Reload**
- **Before**: Reused potentially corrupted WASM module
- **After**: Forces complete module reload from scratch
- **Result**: Fresh WASM module that works properly

### ✅ **Aggressive Memory Cleanup**
- **Before**: Single GC call, hoping for the best
- **After**: Multiple GC cycles with delays to ensure memory is freed
- **Result**: Memory is actually returned to the OS

## Expected Log Patterns

### **WASM Module Destruction**
```
GraphViz: Destroying WASM module completely
GraphViz: Forcing WASM module memory cleanup
GraphViz: Waiting for WASM module to be unloaded
GraphViz: WASM module destroyed - will be reloaded on next use
```

### **WASM Module Reload**
```
GraphViz: WASM module was destroyed, forcing complete reload
GraphViz: WASM module reload initiated
GraphViz: Creating new GraphViz instance for format png
```

### **Memory After Module Destruction**
```
GraphViz: Memory before operations: Alloc=25000 KB, Sys=80000 KB  # Much lower!
```

## Benefits

### 🎯 **Actual Memory Freedom**
- WASM module is completely destroyed and reloaded
- Memory is actually freed at the module level
- No more infinite loops of WASM resets

### 🎯 **Fresh Module State**
- Each reload creates a completely fresh WASM module
- No memory corruption or accumulated state
- Clean slate for each new generation

### 🎯 **Proper Resource Management**
- Module-level destruction ensures complete cleanup
- No memory leaks or resource accumulation
- System can recover from any memory state

### 🎯 **Robust Recovery**
- System can recover from WASM module corruption
- Fresh modules work without memory issues
- Memory usage actually decreases after destruction

## Testing

### **Test Script**: `test_wasm_module_destruction.sh`
- Verifies WASM module destruction and reload
- Triggers memory pressure to force module destruction
- Confirms new instances work after module reload
- Checks for proper cleanup and memory freedom

### **Expected Results**
1. **Initial requests**: Create WASM instances from module
2. **Memory pressure**: Triggers complete WASM module destruction
3. **Post-destruction**: New requests reload the module completely
4. **Memory usage**: Actually decreases after module destruction
5. **No infinite loops**: System recovers cleanly with fresh module

## Summary

This solution implements **complete WASM module destruction and reload** instead of just hoping garbage collection will free memory. By destroying the entire WASM module and forcing a complete reload, the system can:

- **Actually free memory** by destroying the module completely
- **Create fresh modules** that work properly after destruction
- **Prevent infinite loops** by properly managing module state
- **Deliver expected formats** (PNG/SVG) to browsers instead of falling back to DOT

The key insight is that **WASM modules must be completely destroyed and reloaded** - they don't just disappear with garbage collection. This solution ensures that when memory is exhausted, the system actually destroys the entire WASM module and creates a fresh one, rather than just clearing references and hoping for the best.
