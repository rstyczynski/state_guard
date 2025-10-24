# Documentation Directory

This directory contains documentation, investigation notes, and test scripts from the GraphViz WASM memory leak debugging effort (October 2025).

## Structure

```
docs/
├── README.md                           # This file
├── investigation/                      # Investigation notes and solution attempts
│   ├── GRAPHVIZ_MEMORY_SOLUTION.md    # Main memory management solution
│   ├── PARALLEL_SERIALIZATION_SOLUTION.md
│   ├── PERFORMANCE_FIX.md             # 20x speedup (4s → 200ms)
│   ├── WASM_DESTRUCTION_SOLUTION.md
│   ├── WASM_MODULE_DESTRUCTION_SOLUTION.md
│   ├── GRAPHVIZ_INSTANCE_POOLING_SOLUTION.md
│   ├── REALISTIC_WASM_SOLUTION.md
│   ├── NO_DOT_FALLBACK_SOLUTION.md
│   ├── DOT_REMOVAL_SUMMARY.md
│   ├── MEMORY_LEAK_FIXES.md
│   ├── DEBUG_LOGGING_IMPLEMENTATION.md
│   └── ADDITIONAL_DEBUG_LOGGING_FIXES.md
└── test-scripts/                       # Test scripts for validation
    ├── test_concurrent_renders.sh      # Concurrent request testing
    ├── test_graphviz_memory.sh         # Memory monitoring
    ├── test_rapid_with_asset.sh        # Rapid slider simulation
    ├── test_graphviz_pooling.sh
    ├── test_parallel_serialization.sh
    ├── test_memory_leak_fix.sh
    ├── test_improved_memory_management.sh
    ├── test_wasm_destruction.sh
    ├── test_wasm_module_destruction.sh
    ├── test_wasm_reset.sh
    ├── test_process_restart.sh
    ├── test_memory_fallback.sh
    ├── test_global_generator.sh
    ├── test_global_mutex.sh
    ├── test_infinite_loop_fix.sh
    └── test_highlight_current.sh

```

## Investigation Summary

### The Problem
GraphViz WASM memory leak causing "out of bounds memory access" errors during rapid visualization rendering (timeline slider usage).

### Root Cause (Confirmed)
**go-graphviz issue #111**: `gv.Close()` does NOT free WASM linear memory. WASM memory is separate from Go's heap and accumulates over time.

### Solution Attempts (Chronological)

1. **GRAPHVIZ_MEMORY_SOLUTION.md** - Initial memory monitoring and limits
2. **PARALLEL_SERIALIZATION_SOLUTION.md** - Global mutex to prevent concurrent WASM access
3. **GRAPHVIZ_INSTANCE_POOLING_SOLUTION.md** - Reuse WASM instances (helps but doesn't solve leak)
4. **WASM_DESTRUCTION_SOLUTION.md** - Attempted WASM cleanup (doesn't work)
5. **WASM_MODULE_DESTRUCTION_SOLUTION.md** - Attempted module unload (doesn't work)
6. **PERFORMANCE_FIX.md** - Reduced GC delays from 4s to 200ms (20x speedup)
7. **NO_DOT_FALLBACK_SOLUTION.md** - Removed useless DOT format fallback
8. **REALISTIC_WASM_SOLUTION.md** - Accepted reality: os.Exit(1) is the only solution

### Final Solution
**Process restart via os.Exit(1)** when memory exceeds threshold (default: 50MB). This is the ONLY way to free WASM memory due to go-graphviz limitation.

See: `../WASM_MEMORY_LEAK_DOCUMENTATION.md` for comprehensive technical guide.

## Test Scripts

The test scripts validate various aspects of the solution:

- **Memory Management**: `test_graphviz_memory.sh`, `test_memory_leak_fix.sh`
- **Concurrency**: `test_concurrent_renders.sh`, `test_parallel_serialization.sh`
- **Performance**: `test_rapid_with_asset.sh` (simulates timeline slider)
- **Pooling/Caching**: `test_graphviz_pooling.sh`, `test_global_generator.sh`
- **Cleanup**: `test_wasm_destruction.sh`, `test_wasm_reset.sh`

### Running Tests

```bash
# Memory stress test
cd docs/test-scripts
./test_graphviz_memory.sh

# Concurrent rendering test
./test_concurrent_renders.sh

# Rapid slider simulation
./test_rapid_with_asset.sh
```

## Key Learnings

1. ❌ **What Doesn't Work:**
   - `gv.Close()` - only closes Go wrapper, not WASM memory
   - `runtime.GC()` - Go GC doesn't manage WASM memory
   - Instance pooling - prevents re-initialization but doesn't free render data
   - Cache clearing - only affects Go-side structures
   - WASM finalizers - not implemented in go-graphviz

2. ✅ **What Works:**
   - `os.Exit(1)` - only way to free WASM memory
   - Process manager (systemd/k8s) for automatic restart
   - Memory threshold monitoring (configurable via env vars)
   - Global mutex for thread safety

3. 🎯 **Production Impact:**
   - Process restarts every 1-2 hours under normal load
   - Zero-downtime with multiple replicas + load balancer
   - Memory threshold tunable based on traffic patterns
   - Comprehensive monitoring and logging

## References

- **Upstream Bug**: https://github.com/goccy/go-graphviz/issues/111
- **Main Documentation**: `../WASM_MEMORY_LEAK_DOCUMENTATION.md`
- **Implementation**: `../internal/visualize/visualize.go`
- **Commit History**:
  - `64ee2a8` - Initial fix: immediate resource cleanup
  - `5b2bb1a` - Add WASM management infrastructure
  - `ad5e8de` - Document reality: os.Exit(1) is the only solution

## Historical Context

These documents represent ~1 day of intensive debugging and testing (October 24, 2025). Multiple solution attempts were made before accepting that the go-graphviz library limitation requires process restart as the only viable workaround.

The investigation progression shows:
1. Optimistic attempts at "proper" memory management
2. Increasingly complex workarounds
3. Acceptance of library limitation
4. Implementation of production-ready workaround

This is a testament to: "Sometimes the right solution is the simplest one, even if it feels inelegant."
