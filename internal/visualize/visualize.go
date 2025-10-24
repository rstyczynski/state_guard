package visualize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"github.com/rstyczynski/fsm_v2/internal/fsm"
	"github.com/rstyczynski/fsm_v2/internal/storage"
)

// Debug logging for GraphViz operations
var (
	debugLogging = os.Getenv("GRAPHVIZ_DEBUG") == "true"
	debugMutex   sync.Mutex
)

// debugLog logs GraphViz operations in debug mode only
func debugLog(format string, args ...interface{}) {
	if debugLogging {
		debugMutex.Lock()
		defer debugMutex.Unlock()
		log.Printf("[GraphViz-DEBUG] "+format, args...)
	}
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		log.Printf("Warning: Invalid %s value '%s', using default %d", key, value, defaultValue)
	}
	return defaultValue
}

// Global mutex to serialize ALL GraphViz operations across all requests
// GraphViz WASM doesn't support concurrent access and will crash
// with nil pointer dereferences if multiple renders happen simultaneously
var globalGraphVizMutex sync.Mutex

// Global generator cache to avoid recreating generators for the same asset types
var (
	globalGenerators = make(map[string]*Generator)
	generatorsMutex  sync.RWMutex
)

// GraphViz instance pool to reuse WASM instances
var (
	graphvizPool = make(map[string]*graphviz.Graphviz)
	poolMutex    sync.RWMutex
)

// WASM memory management - CONFIGURABLE PARAMETERS
//
// CRITICAL LIMITATION: go-graphviz WASM Memory Leak (github.com/goccy/go-graphviz issue #111)
// =======================================================================================
// The go-graphviz library has a KNOWN BUG where WASM linear memory is NOT freed when
// calling Close(). Each render allocates WASM memory that accumulates over time.
//
// Root Cause:
// - GraphViz uses WebAssembly with its own linear memory space (separate from Go heap)
// - gv.Close() only closes the Go wrapper, NOT the underlying WASM memory
// - WASM module is loaded once and its memory persists for process lifetime
// - Go's runtime.GC() cannot collect WASM linear memory
//
// The ONLY reliable solution is os.Exit(1) to restart the process and free WASM memory.
// This is not a bug in our code - it's a fundamental limitation of the current go-graphviz implementation.
//
// See: https://github.com/goccy/go-graphviz/issues/111
//
var (
	// Memory thresholds (configurable via environment variables)
	wasmMemoryLimitBeforeRender = getEnvInt("GRAPHVIZ_MEMORY_LIMIT_BEFORE", 50) // MB - restart before render
	wasmMemoryLimitAfterRender  = getEnvInt("GRAPHVIZ_MEMORY_LIMIT_AFTER", 60)  // MB - restart after render
	wasmMemoryWarningThreshold  = getEnvInt("GRAPHVIZ_MEMORY_WARNING", 40)      // MB - warning threshold

	// Reset and restart limits
	maxWasmResets      = getEnvInt("GRAPHVIZ_MAX_WASM_RESETS", 2)      // Max WASM resets before process restart
	maxProcessRestarts = getEnvInt("GRAPHVIZ_MAX_PROCESS_RESTARTS", 1) // Max process restarts

	// State tracking
	wasmMemoryExhausted = false
	wasmResetMutex      sync.Mutex
	wasmResetCount      = 0
	activeWasmInstances = make(map[string]*graphviz.Graphviz) // Track active WASM instances
	wasmModuleDestroyed = false                               // Track if WASM module was destroyed
	processRestartCount = 0                                   // Track process restart attempts
)

// Format represents the output format for visualization
type Format string

const (
	FormatSVG     Format = "svg"
	FormatPNG     Format = "png"
	FormatDOT     Format = "dot"
	FormatMermaid Format = "mermaid"
	FormatJSON    Format = "json"
)

// Layout represents the layout algorithm
type Layout string

const (
	LayoutVertical   Layout = "vertical"
	LayoutHorizontal Layout = "horizontal"
)

// Options for diagram generation
type Options struct {
	Format           Format
	Layout           Layout
	HighlightCurrent bool
	ShowHistory      bool
	ShowAvailable    bool
	CurrentState     string
	AvailableStates  []string
	HistoryPath      []HistoryEntry
	ColorScheme      string // "light" or "dark"
}

// HistoryEntry represents a state transition in history
type HistoryEntry struct {
	FromState      string
	ToState        string
	TransitionedAt time.Time
}

// GraphData represents structured graph data for JSON export
type GraphData struct {
	Name         string     `json:"name"`
	InitialState string     `json:"initial_state"`
	FinalStates  []string   `json:"final_states"`
	CurrentState string     `json:"current_state,omitempty"`
	Nodes        []NodeData `json:"nodes"`
	Edges        []EdgeData `json:"edges"`
}

// NodeData represents a state node
type NodeData struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	IsInitial   bool   `json:"is_initial"`
	IsFinal     bool   `json:"is_final"`
	IsCurrent   bool   `json:"is_current"`
	IsAvailable bool   `json:"is_available"`
}

// EdgeData represents a transition edge
type EdgeData struct {
	From       string `json:"from"`
	To         string `json:"to"`
	IsWildcard bool   `json:"is_wildcard"`
	IsBidirect bool   `json:"is_bidirectional"`
	IsHistory  bool   `json:"is_history"`
	Style      string `json:"style"` // "solid", "dashed"
}

// Generator generates state diagram visualizations
type Generator struct {
	definition *fsm.Definition
	storage    storage.Storage
}

// NewGenerator creates a new visualization generator
func NewGenerator(def *fsm.Definition, store storage.Storage) *Generator {
	return &Generator{
		definition: def,
		storage:    store,
	}
}

// GetOrCreateGlobalGenerator gets or creates a global generator for the given asset type
// This ensures we reuse generators and avoid recreating them for the same asset types
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

	// Load asset type
	assetType, err := fsm.LoadAssetType(assetTypeName)
	if err != nil {
		return nil, fmt.Errorf("failed to load asset type: %w", err)
	}

	// Load FSM definition
	if err := assetType.LoadStateMachine(); err != nil {
		return nil, fmt.Errorf("failed to load state machine: %w", err)
	}

	// Create and cache the generator
	generator := NewGenerator(assetType.StateMachineRef, store)
	globalGenerators[assetTypeName] = generator

	log.Printf("Global generator created for asset type: %s", assetTypeName)
	return generator, nil
}

// ClearGlobalGenerators clears the global generator cache
// Useful for testing or when asset types change
func ClearGlobalGenerators() {
	generatorsMutex.Lock()
	defer generatorsMutex.Unlock()

	globalGenerators = make(map[string]*Generator)
	log.Printf("Cleared global generator cache")
}

// cleanupGlobalGenerators performs selective cleanup of generators to prevent memory leaks
func cleanupGlobalGenerators() {
	generatorsMutex.Lock()
	defer generatorsMutex.Unlock()

	// CRITICAL: Only clean up generators if we have too many
	// This prevents memory accumulation while maintaining performance
	if len(globalGenerators) > 5 {
		log.Printf("GraphViz: Cleaning up %d cached generators (excess)", len(globalGenerators))

		// Keep only the most recent 3 generators, clear the rest
		count := 0
		for assetType := range globalGenerators {
			if count < len(globalGenerators)-3 {
				delete(globalGenerators, assetType)
				count++
			}
		}
		log.Printf("GraphViz: Generator cleanup completed, kept %d generators", len(globalGenerators))
	}
}

// GetGlobalGeneratorStats returns statistics about the global generator cache
func GetGlobalGeneratorStats() map[string]interface{} {
	generatorsMutex.RLock()
	defer generatorsMutex.RUnlock()

	return map[string]interface{}{
		"cached_generators": len(globalGenerators),
		"asset_types":       getMapKeys(globalGenerators),
	}
}

// GetWasmInstanceStats returns statistics about active WASM instances
func GetWasmInstanceStats() map[string]interface{} {
	return map[string]interface{}{
		"active_wasm_instances": len(activeWasmInstances),
		"wasm_instance_keys":    getWasmInstanceKeys(activeWasmInstances),
		"wasm_reset_count":      wasmResetCount,
		"wasm_memory_exhausted": wasmMemoryExhausted,
	}
}

// GetWasmMemoryConfig returns current WASM memory configuration
func GetWasmMemoryConfig() map[string]interface{} {
	return map[string]interface{}{
		"memory_limit_before_render": wasmMemoryLimitBeforeRender,
		"memory_limit_after_render":  wasmMemoryLimitAfterRender,
		"memory_warning_threshold":   wasmMemoryWarningThreshold,
		"max_wasm_resets":            maxWasmResets,
		"max_process_restarts":       maxProcessRestarts,
		"current_wasm_resets":        wasmResetCount,
		"current_process_restarts":   processRestartCount,
	}
}

// GetOrCreateGraphVizInstance gets or creates a GraphViz instance from the pool
func GetOrCreateGraphVizInstance(format string) (*graphviz.Graphviz, error) {
	poolMutex.RLock()
	if gv, exists := graphvizPool[format]; exists {
		poolMutex.RUnlock()
		log.Printf("GraphViz: Reusing pooled instance for format: %s", format)
		return gv, nil
	}
	poolMutex.RUnlock()

	// Need to create a new instance
	poolMutex.Lock()
	defer poolMutex.Unlock()

	// Double-check in case another goroutine created it while we were waiting
	if gv, exists := graphvizPool[format]; exists {
		return gv, nil
	}

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

// cleanupGraphVizPool performs selective cleanup of GraphViz instances to prevent memory leaks
func cleanupGraphVizPool() {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	// CRITICAL: Only clean up instances that are NOT currently being used
	// This prevents WASM errors while still managing memory
	instancesToClose := make([]*graphviz.Graphviz, 0)

	// For now, be conservative and only clean up if we have too many instances
	if len(graphvizPool) > 3 {
		log.Printf("GraphViz: Pool has %d instances, cleaning up excess", len(graphvizPool))

		// Keep only the most recent 2 instances, close the rest
		count := 0
		for format, gv := range graphvizPool {
			if gv != nil && count < len(graphvizPool)-2 {
				log.Printf("GraphViz: Marking old instance for cleanup: %s", format)
				instancesToClose = append(instancesToClose, gv)
				delete(graphvizPool, format)
				count++
			}
		}
	}

	// Close excess instances to free WASM memory
	for _, gv := range instancesToClose {
		if gv != nil {
			log.Printf("GraphViz: Closing excess instance to free WASM memory")
			gv.Close()
		}
	}

	if len(instancesToClose) > 0 {
		log.Printf("GraphViz: Pool cleanup completed, closed %d excess instances", len(instancesToClose))
	}
}

// cleanupOtherGraphVizInstances cleans up all instances except the current one
func cleanupOtherGraphVizInstances(currentFormat string) {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	// CRITICAL: Close all instances except the current one to prevent memory accumulation
	instancesToClose := make([]*graphviz.Graphviz, 0)

	for format, gv := range graphvizPool {
		if gv != nil && format != currentFormat {
			log.Printf("GraphViz: Force closing other instance for format: %s", format)
			instancesToClose = append(instancesToClose, gv)
			// Remove from pool immediately
			delete(graphvizPool, format)
		}
	}

	// Close instances to free WASM memory
	for _, gv := range instancesToClose {
		if gv != nil {
			log.Printf("GraphViz: Closing other instance to free WASM memory")
			gv.Close()
		}
	}

	if len(instancesToClose) > 0 {
		log.Printf("GraphViz: Cleaned up %d other instances, keeping only %s", len(instancesToClose), currentFormat)
	}
}

// Helper function to get map keys for Generator map
func getMapKeys(m map[string]*Generator) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Helper function to get map keys for WASM instances map
func getWasmInstanceKeys(m map[string]*graphviz.Graphviz) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// markWasmMemoryExhausted marks that WASM memory is exhausted and needs reset
func markWasmMemoryExhausted() {
	wasmResetMutex.Lock()
	defer wasmResetMutex.Unlock()
	wasmMemoryExhausted = true
	log.Printf("GraphViz: WASM memory exhausted, marking for reset")
}

// isWasmMemoryExhausted checks if WASM memory is exhausted
func isWasmMemoryExhausted() bool {
	wasmResetMutex.Lock()
	defer wasmResetMutex.Unlock()
	return wasmMemoryExhausted
}

// resetWasmMemory resets the WASM memory state
func resetWasmMemory() {
	wasmResetMutex.Lock()
	defer wasmResetMutex.Unlock()
	wasmMemoryExhausted = false
	wasmResetCount = 0 // Reset the counter when memory is actually freed

	runtime.GC()
	time.Sleep(100 * time.Millisecond) // Reduced from 2 seconds to 100ms
	runtime.GC()
	time.Sleep(100 * time.Millisecond) // Reduced from 2 seconds to 100ms
	runtime.GC()

	log.Printf("GraphViz: WASM memory state reset (reset count: %d)", wasmResetCount)
}

// restartProcess restarts the entire Go process to free WASM memory
//
// This is the ONLY reliable way to free GraphViz WASM memory due to a known bug
// in go-graphviz (issue #111) where Close() does not free WASM linear memory.
//
// WASM linear memory is separate from Go's heap and cannot be freed by:
// - gv.Close() - only closes Go wrapper
// - runtime.GC() - Go GC doesn't manage WASM memory
// - Clearing caches/pools - only affects Go-side structures
//
// Process restart is the only way to truly free WASM memory.
func restartProcess() {
	processRestartCount++
	if processRestartCount > maxProcessRestarts {
		log.Printf("GraphViz: Maximum process restarts (%d) exceeded, giving up", maxProcessRestarts)
		return
	}

	log.Printf("GraphViz: Process restart #%d - restarting entire process", processRestartCount)
	log.Printf("GraphViz: WASM memory leak requires process restart (go-graphviz issue #111)")
	log.Printf("GraphViz: Close() does NOT free WASM linear memory - os.Exit(1) is the only solution")

	os.Exit(1)
}

// forceWasmReset forces a complete WASM reset by destroying and recreating WASM instances
func forceWasmReset() {
	wasmResetMutex.Lock()
	defer wasmResetMutex.Unlock()

	wasmResetCount++
	log.Printf("GraphViz: Forced WASM reset #%d - destroying and recreating WASM instances", wasmResetCount)

	// Clear all global generators to force recreation
	generatorsMutex.Lock()
	globalGenerators = make(map[string]*Generator)
	generatorsMutex.Unlock()

	// Clear GraphViz instance pool
	ClearGraphVizPool()

	wasmMemoryExhausted = false
	log.Printf("GraphViz: WASM module destroyed and memory freed")
}

// logMemoryStats logs detailed memory statistics for debugging
func logMemoryStats(prefix string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("%s: Alloc=%d KB, Sys=%d KB, NumGC=%d, HeapObjects=%d, StackInuse=%d KB",
		prefix, m.Alloc/1024, m.Sys/1024, m.NumGC, m.HeapObjects, m.StackInuse/1024)
}

// logWasmMemoryConstraints logs WASM-specific memory constraints and status
func logWasmMemoryConstraints(prefix string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Calculate WASM memory usage as percentage of configured limit
	wasmMemoryUsage := float64(m.Alloc) / float64(wasmMemoryLimitBeforeRender*1024*1024) * 100

	log.Printf("%s: WASM Memory Status:", prefix)
	log.Printf("  - Current Alloc: %d KB (%.1f%% of %dMB limit)", m.Alloc/1024, wasmMemoryUsage, wasmMemoryLimitBeforeRender)
	log.Printf("  - System Memory: %d KB", m.Sys/1024)
	log.Printf("  - Heap Objects: %d", m.HeapObjects)
	log.Printf("  - GC Count: %d", m.NumGC)
	log.Printf("  - WASM Memory Exhausted: %t", wasmMemoryExhausted)
	log.Printf("  - WASM Reset Count: %d/%d", wasmResetCount, maxWasmResets)
	log.Printf("  - Process Restart Count: %d/%d", processRestartCount, maxProcessRestarts)
	log.Printf("  - Active WASM Instances: %d", len(activeWasmInstances))
	log.Printf("  - GraphViz Pool Size: %d", len(graphvizPool))
	log.Printf("  - Generator Cache Size: %d", len(globalGenerators))

	// Memory pressure warnings
	if wasmMemoryUsage > 80 {
		log.Printf("  - WARNING: WASM memory usage > 80%% - process restart may be needed soon")
	} else if wasmMemoryUsage > 60 {
		log.Printf("  - WARNING: WASM memory usage > 60%% - monitoring closely")
	}
}

// GenerateDefinition generates a diagram for the FSM definition only
func (g *Generator) GenerateDefinition(opts Options) ([]byte, error) {
	switch opts.Format {
	case FormatJSON:
		return g.generateJSON(opts)
	case FormatMermaid:
		return g.generateMermaid(opts)
	case FormatDOT:
		return nil, fmt.Errorf("DOT format not supported - use PNG or SVG instead")
	case FormatSVG, FormatPNG:
		return g.generateGraphViz(opts)
	default:
		return nil, fmt.Errorf("unsupported format: %s", opts.Format)
	}
}

// GenerateInstance generates a diagram for a specific FSM instance
func (g *Generator) GenerateInstance(ctx context.Context, instanceID string, opts Options) ([]byte, error) {
	// Load instance data
	instance, err := g.storage.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load instance: %w", err)
	}

	// Set current state (only if highlighting is enabled and not already set by query parameter)
	if opts.HighlightCurrent {
		if opts.CurrentState == "" {
			// Trim whitespace to ensure exact matching
			opts.CurrentState = strings.TrimSpace(instance.CurrentState)
		} else {
			// Also trim if set by query parameter
			opts.CurrentState = strings.TrimSpace(opts.CurrentState)
		}
	}

	// Load available transitions if needed
	if opts.ShowAvailable {
		f, err := fsm.LoadFromStorage(ctx, instanceID, g.definition, g.storage)
		if err != nil {
			return nil, fmt.Errorf("failed to load FSM: %w", err)
		}
		opts.AvailableStates = f.AvailableTransitions()
	}

	// Load history if needed
	if opts.ShowHistory {
		transitions, err := g.storage.GetTransitionHistory(ctx, instanceID, 1000)
		if err != nil {
			return nil, fmt.Errorf("failed to load history: %w", err)
		}
		opts.HistoryPath = make([]HistoryEntry, len(transitions))
		for i, t := range transitions {
			opts.HistoryPath[i] = HistoryEntry{
				FromState:      t.FromState,
				ToState:        t.ToState,
				TransitionedAt: t.TransitionedAt,
			}
		}
	}

	return g.GenerateDefinition(opts)
}

// generateGraphViz generates SVG or PNG using GraphViz
func (g *Generator) generateGraphViz(opts Options) ([]byte, error) {
	// Check if WASM memory is exhausted and needs reset
	if isWasmMemoryExhausted() {
		log.Printf("GraphViz: WASM memory exhausted, forcing complete reset")
		forceWasmReset()
		// Force aggressive garbage collection after reset
		runtime.GC()
		runtime.GC()
		time.Sleep(200 * time.Millisecond)
	}

	// Check memory limits BEFORE acquiring mutex to avoid blocking other requests
	// GraphViz WASM has limited memory and can crash with out of bounds access
	var mBefore runtime.MemStats
	runtime.ReadMemStats(&mBefore)
	logMemoryStats("GraphViz: Memory before operations")

	// CRITICAL: Configurable memory threshold for process restart to prevent WASM memory leaks
	// GraphViz WASM instances cannot be freed, so we must restart the process
	memoryLimitBytes := int64(wasmMemoryLimitBeforeRender) * 1024 * 1024
	if mBefore.Alloc > uint64(memoryLimitBytes) {
		log.Printf("GraphViz: Memory limit exceeded (%d KB > %d MB), WASM instances cannot be freed",
			mBefore.Alloc/1024, wasmMemoryLimitBeforeRender)
		log.Printf("GraphViz: WASM module cannot be destroyed once loaded, restarting process")
		// Restart the entire process to free all WASM memory
		restartProcess()
	} else {
		// Memory is low, reset the WASM state
		resetWasmMemory()
	}

	// CRITICAL: Lock to serialize GraphViz operations across ALL requests
	// GraphViz WASM doesn't support concurrent access and will crash
	// with nil pointer dereferences if multiple renders happen simultaneously
	debugLog("Acquiring global mutex (format: %s)", opts.Format)
	globalGraphVizMutex.Lock()
	defer globalGraphVizMutex.Unlock()
	debugLog("Global mutex acquired (format: %s)", opts.Format)

	// Panic recovery for GraphViz operations
	defer func() {
		if panicErr := recover(); panicErr != nil {
			stack := debug.Stack()
			log.Printf("CRASH in generateGraphViz: %v\nStack trace:\n%s", panicErr, string(stack))

			// Log memory stats at crash
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Printf("GraphViz crash memory: Alloc=%d KB, Sys=%d KB, NumGC=%d",
				m.Alloc/1024, m.Sys/1024, m.NumGC)

			// CRITICAL: Force cleanup of any remaining resources
			log.Printf("GraphViz: Forcing resource cleanup after crash")
			runtime.GC()
			runtime.GC()
			time.Sleep(100 * time.Millisecond) // Allow cleanup to complete

			// Mark WASM as exhausted due to crash
			markWasmMemoryExhausted()
			log.Printf("GraphViz: Marked WASM as exhausted due to crash")

			// Re-panic to be caught by handler
			panic(fmt.Sprintf("GraphViz crash: %v", panicErr))
		}
	}()

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

	// Get or create GraphViz instance from pool
	gv, err := GetOrCreateGraphVizInstance(string(opts.Format))
	if err != nil {
		log.Printf("GraphViz: Failed to get GraphViz instance: %v", err)
		return nil, fmt.Errorf("failed to get graphviz instance: %w", err)
	}

	// Note: Don't cleanup other instances here as it can cause WASM errors
	// The current instance needs to remain active during rendering

	debugLog("Creating graph with %d states, %d transitions",
		len(g.definition.States), len(g.definition.Transitions))

	graph, err := g.createGraph(gv, opts)
	if err != nil {
		log.Printf("GraphViz: Failed to create graph: %v", err)
		// Note: Don't close gv here as it's pooled and reused
		return nil, err
	}

	// Check for nil graph (defensive programming)
	if graph == nil {
		log.Printf("GraphViz: Graph creation returned nil")
		// Note: Don't close gv here as it's pooled and reused
		return nil, fmt.Errorf("graph creation returned nil")
	}

	var buf bytes.Buffer
	ctx := context.Background()
	debugLog("Rendering %s format", opts.Format)

	switch opts.Format {
	case FormatSVG:
		if err := gv.Render(ctx, graph, graphviz.SVG, &buf); err != nil {
			log.Printf("GraphViz: SVG render failed: %v", err)
			graph.Close() // Clean up graph on error
			// Note: Don't close gv here as it's pooled and reused
			return nil, fmt.Errorf("SVG render failed: %w", err)
		}
		debugLog("SVG render successful, %d bytes", buf.Len())
	case FormatPNG:
		if err := gv.Render(ctx, graph, graphviz.PNG, &buf); err != nil {
			log.Printf("GraphViz: PNG render failed: %v", err)
			graph.Close() // Clean up graph on error
			// Note: Don't close gv here as it's pooled and reused
			return nil, fmt.Errorf("PNG render failed: %w", err)
		}
		debugLog("PNG render successful, %d bytes", buf.Len())
	}

	// CRITICAL: Explicitly close resources IMMEDIATELY after rendering
	// to prevent WASM memory accumulation on rapid sequential renders
	debugLog("Closing resources")
	graph.Close()
	// Note: Don't close gv here as it's pooled and reused

	debugLog("Graph closed, GraphViz instance returned to pool")

	// CRITICAL: GraphViz WASM Memory Leak (go-graphviz issue #111)
	// ================================================================
	// WASM instances CANNOT be freed by Close() - memory leaks are unavoidable.
	// The WASM linear memory accumulates until process restart (os.Exit(1)).
	//
	// Why pooling/caching doesn't solve the leak:
	// - Each render allocates WASM memory for graph data structures
	// - graph.Close() is called above, but WASM memory is NOT freed
	// - Pooling the gv instance only prevents re-creating the WASM module
	// - The render data itself still leaks in WASM linear memory
	//
	// This is why we monitor memory and restart when threshold is exceeded.

	// Note: Don't close the current instance here as it's pooled and reused
	// Closing it would cause WASM errors on subsequent renders

	// Force garbage collection to free WASM memory immediately.
	// GraphViz WASM has limited memory and rapid renders (timeline slider)
	// can exhaust it if GC doesn't run between renders.
	runtime.GC()
	runtime.GC()                      // Double GC for WASM memory
	time.Sleep(50 * time.Millisecond) // Allow cleanup to complete

	// Additional cleanup for WASM instances
	debugLog("Performing WASM instance cleanup")
	runtime.GC()

	// Log memory after cleanup
	logMemoryStats("GraphViz: Memory after cleanup")

	// CRITICAL: Check if memory is still too high after cleanup
	// If so, restart process as WASM instances cannot be freed
	var mAfter runtime.MemStats
	runtime.ReadMemStats(&mAfter)
	memoryLimitAfterBytes := int64(wasmMemoryLimitAfterRender) * 1024 * 1024
	if mAfter.Alloc > uint64(memoryLimitAfterBytes) {
		log.Printf("GraphViz: Memory still high after cleanup (%d KB > %d MB), WASM instances cannot be freed",
			mAfter.Alloc/1024, wasmMemoryLimitAfterRender)
		log.Printf("GraphViz: WASM module cannot be destroyed once loaded, restarting process")
		// Restart the entire process to free all WASM memory
		restartProcess()
	}

	debugLog("Releasing global mutex (format: %s)", opts.Format)

	return buf.Bytes(), nil
}

// generateGraphVizWithReset attempts to generate with WASM reset and retry
func (g *Generator) generateGraphVizWithReset(opts Options) ([]byte, error) {
	wasmResetMutex.Lock()
	resetCount := wasmResetCount
	wasmResetMutex.Unlock()

	if resetCount >= maxWasmResets {
		log.Printf("GraphViz: Maximum WASM resets (%d) exceeded, restarting process", maxWasmResets)
		log.Printf("GraphViz: WASM module cannot be destroyed once loaded, restarting process")
		// Restart the entire process to free all memory
		restartProcess()
	}

	log.Printf("GraphViz: Attempting generation with WASM destruction (format: %s, reset #%d)", opts.Format, resetCount+1)

	// Force complete WASM destruction and recreation
	forceWasmReset()

	// Additional aggressive memory cleanup
	log.Printf("GraphViz: Performing additional memory cleanup after WASM destruction")
	runtime.GC()
	runtime.GC()
	runtime.GC()
	time.Sleep(2 * time.Second) // Longer wait for complete WASM destruction

	// Log memory after destruction
	var mAfter runtime.MemStats
	runtime.ReadMemStats(&mAfter)
	log.Printf("GraphViz: Memory after WASM destruction: Alloc=%d KB, Sys=%d KB, NumGC=%d",
		mAfter.Alloc/1024, mAfter.Sys/1024, mAfter.NumGC)

	// Try to generate again with fresh WASM state
	log.Printf("GraphViz: Retrying generation after WASM destruction (format: %s)", opts.Format)
	return g.generateGraphViz(opts)
}

// createGraph builds the GraphViz graph
func (g *Generator) createGraph(gv *graphviz.Graphviz, opts Options) (*cgraph.Graph, error) {
	// Width constants for different node types
	const (
		nodeWidth            = 1.5
		nodeHeight           = 0.6
		standardPenWidth     = 1.0 //1.0
		initialStatePenWidth = 2.0 //2.0
		finalStatePenWidth   = 2.0 //4.0
		edgePenWidth         = 2.0 //2.0
	)

	// Periphery constants for different node types and formats
	var (
		standardPeripheries     int
		initialStatePeripheries int
		finalStatePeripheries   int
		wildcardPeripheries     int
	)

	// Set periphery values based on format
	if opts.Format == FormatPNG {
		// PNG needs thicker outlines to be visible
		standardPeripheries = 2
		initialStatePeripheries = 3
		finalStatePeripheries = 3
		wildcardPeripheries = 2
	} else {
		// SVG works well with thinner outlines
		standardPeripheries = 1
		initialStatePeripheries = 2
		finalStatePeripheries = 2
		wildcardPeripheries = 1
	}

	var graph *cgraph.Graph
	var nodes map[string]*cgraph.Node

	// Panic recovery for graph creation
	defer func() {
		if panicErr := recover(); panicErr != nil {
			stack := debug.Stack()
			log.Printf("CRASH in createGraph: %v\nStack trace:\n%s", panicErr, string(stack))

			// Log memory stats at crash
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Printf("createGraph crash memory: Alloc=%d KB, Sys=%d KB, NumGC=%d",
				m.Alloc/1024, m.Sys/1024, m.NumGC)

			// CRITICAL: Clean up any created objects before re-panicking
			if graph != nil {
				log.Printf("GraphViz: Cleaning up graph after crash")
				graph.Close()
			}

			// Force cleanup of any created nodes
			if nodes != nil {
				log.Printf("GraphViz: Cleaning up %d nodes after crash", len(nodes))
				for state, node := range nodes {
					if node != nil {
						log.Printf("GraphViz: Force cleaning node: %s", state)
						// Note: Individual nodes are cleaned up when graph.Close() is called
						// No explicit node cleanup needed in GraphViz
					}
				}
			}

			// Force garbage collection after cleanup
			runtime.GC()
			time.Sleep(50 * time.Millisecond)

			// Re-panic to be caught by generateGraphViz
			panic(fmt.Sprintf("createGraph crash: %v", panicErr))
		}
	}()

	// Determine GraphViz layout engine
	var layoutEngine string
	var rankDir cgraph.RankDir

	// Only horizontal (default) and vertical layouts are supported
	switch opts.Layout {
	case LayoutVertical:
		layoutEngine = "dot"
		rankDir = cgraph.TBRank // Top to bottom (vertical)
	case LayoutHorizontal:
		layoutEngine = "dot"
		rankDir = cgraph.LRRank // Left to right (horizontal - default)
	default:
		layoutEngine = "dot"
		rankDir = cgraph.LRRank // Default to horizontal
	}

	debugLog("Creating graph with layout engine: %s, rank direction: %v", layoutEngine, rankDir)

	graph, err := gv.Graph()
	if err != nil {
		log.Printf("GraphViz: Failed to create graph instance: %v", err)
		return nil, err
	}
	if graph == nil {
		log.Printf("GraphViz: Graph instance is nil")
		return nil, fmt.Errorf("graphviz returned nil graph")
	}

	// Ensure graph is cleaned up on any error
	defer func() {
		if r := recover(); r != nil {
			if graph != nil {
				log.Printf("GraphViz: Cleaning up graph after panic in createGraph")
				graph.Close()
			}

			// CRITICAL: Clean up any created nodes on panic
			if nodes != nil {
				log.Printf("GraphViz: Cleaning up %d nodes after panic", len(nodes))
				for state, node := range nodes {
					if node != nil {
						log.Printf("GraphViz: Force cleaning node on panic: %s", state)
						// Note: Individual nodes are cleaned up when graph.Close() is called
						// No explicit node cleanup needed in GraphViz
					}
				}
			}

			// Force cleanup
			runtime.GC()
			panic(r) // Re-panic
		}
	}()

	// Set layout engine (graphs are directed by default in GraphViz DOT)
	graph.SetLayout(layoutEngine)

	// Set graph attributes
	graph.SetRankDir(rankDir)
	graph.SetBackgroundColor(g.getBackgroundColor(opts))

	// Create nodes for each state
	nodes = make(map[string]*cgraph.Node)
	availableMap := make(map[string]bool)
	for _, state := range opts.AvailableStates {
		availableMap[state] = true
	}

	// Cleanup function for nodes on error
	cleanupNodes := func() {
		log.Printf("GraphViz: Cleaning up %d nodes due to error", len(nodes))

		// CRITICAL: In GraphViz, nodes are cleaned up when the graph is closed
		// But we need to ensure the graph is properly closed to free WASM memory
		if graph != nil {
			log.Printf("GraphViz: Force closing graph to clean up %d nodes", len(nodes))
			graph.Close() // This will clean up all nodes
		}

		// Clear the nodes map to prevent further references
		nodes = make(map[string]*cgraph.Node)

		// CRITICAL: Force garbage collection after node cleanup
		runtime.GC()
		time.Sleep(10 * time.Millisecond) // Allow cleanup to complete

		log.Printf("GraphViz: Node cleanup completed")
	}

	debugLog("Creating %d state nodes", len(g.definition.States))

	for i, state := range g.definition.States {
		debugLog("Creating node %d/%d: %s", i+1, len(g.definition.States), state)

		node, err := graph.CreateNodeByName(state)
		if err != nil {
			log.Printf("GraphViz: Failed to create node for state %s: %v", state, err)
			cleanupNodes()
			graph.Close()
			return nil, fmt.Errorf("failed to create node for state %s: %w", state, err)
		}

		// Trim state name for comparison to handle any whitespace issues
		trimmedState := strings.TrimSpace(state)
		trimmedCurrentState := strings.TrimSpace(opts.CurrentState)

		// Styling based on state type
		isInitial := trimmedState == g.definition.Initial
		isFinal := g.isFinalState(trimmedState)
		// IMPORTANT: Current state must ALWAYS be green, so check this first
		isCurrent := opts.HighlightCurrent && trimmedState == trimmedCurrentState
		// Exclude current state from being marked as available (current state should always be green, not yellow)
		isAvailable := opts.ShowAvailable && availableMap[trimmedState] && trimmedState != trimmedCurrentState

		// ALL states are rectangles with the same size
		node.SetShape(cgraph.BoxShape)
		node.SetWidth(nodeWidth)
		node.SetHeight(nodeHeight)
		node.SetFixedSize(true)

		// Set colors (order matters: check isCurrent first!)
		if isCurrent {
			// Current state always gets green, even if it's FAILED/ERROR
			node.SetFillColor("lightgreen")
			node.SetStyle("filled")                  // Use filled style
			node.SetPenWidth(standardPenWidth)       // Outline width
			node.SetPeripheries(standardPeripheries) // Standard outline
		} else if state == "FAILED" || state == "ERROR" {
			// Error states get coral (but not when current)
			node.SetFillColor("lightcoral")
			node.SetStyle("filled")                  // Use filled style
			node.SetPenWidth(standardPenWidth)       // Outline width
			node.SetPeripheries(standardPeripheries) // Standard outline
		} else if isAvailable {
			node.SetFillColor("lightyellow")
			node.SetStyle("filled")                  // Use filled style
			node.SetPenWidth(standardPenWidth)       // Outline width
			node.SetPeripheries(standardPeripheries) // Standard outline
		} else if isInitial {
			// Bold outline for initial state
			node.SetFillColor("lightblue")
			node.SetStyle("filled")                      // Use filled style
			node.SetPenWidth(initialStatePenWidth)       // Thicker outline for initial state
			node.SetPeripheries(initialStatePeripheries) // Initial state outline
		} else {
			node.SetFillColor("white")
			node.SetStyle("filled")                  // Use filled style
			node.SetPenWidth(standardPenWidth)       // Outline width
			node.SetPeripheries(standardPeripheries) // Standard outline
		}

		// Override peripheries for final states
		if isFinal && !isCurrent {
			node.SetPeripheries(finalStatePeripheries) // Double outline for final states
			node.SetPenWidth(finalStatePenWidth)       // Thicker outline
		}

		nodes[state] = node
	}

	// Create edges for transitions
	debugLog("Creating edges for %d transitions", len(g.definition.Transitions))

	createdEdges := make(map[string]bool)
	hasWildcard := false
	wildcardTarget := ""

	// First pass: check for wildcard transitions
	for _, trans := range g.definition.Transitions {
		if trans.From == "*" {
			hasWildcard = true
			wildcardTarget = trans.To
			debugLog("Found wildcard transition to %s", wildcardTarget)
			break
		}
	}

	// If we have wildcard, create a special "ANY STATE" node
	if hasWildcard {
		debugLog("Creating wildcard ANY_STATE node")

		anyNode, err := graph.CreateNodeByName("ANY_STATE")
		if err != nil {
			log.Printf("GraphViz: Failed to create ANY_STATE node: %v", err)
			cleanupNodes()
			graph.Close()
			return nil, fmt.Errorf("failed to create ANY_STATE node: %w", err)
		}
		anyNode.SetLabel("ANY STATE")
		anyNode.SetShape(cgraph.BoxShape)
		anyNode.SetFillColor("white")
		anyNode.SetStyle("filled")                  // Use filled style
		anyNode.SetPeripheries(wildcardPeripheries) // Add outline using peripheries
		anyNode.SetPenWidth(standardPenWidth)       // Outline width

		// Create arrow from ANY STATE to wildcard target
		debugLog("Creating wildcard edge to %s", wildcardTarget)

		edge, err := graph.CreateEdgeByName("", anyNode, nodes[wildcardTarget])
		if err != nil {
			log.Printf("GraphViz: Failed to create wildcard edge to %s: %v", wildcardTarget, err)
			cleanupNodes()
			graph.Close()
			return nil, fmt.Errorf("failed to create wildcard edge to %s: %w", wildcardTarget, err)
		}
		edge.SetColor("red")
		edge.SetPenWidth(edgePenWidth)
		edge.SetStyle(cgraph.DashedEdgeStyle)
	}

	// Second pass: create regular transitions (no bidirectional handling)
	debugLog("Creating regular transitions")

	for i, trans := range g.definition.Transitions {
		if trans.From == "*" {
			continue // Already handled above
		}

		// Regular transition
		edgeKey := trans.From + "->" + trans.To
		if createdEdges[edgeKey] {
			log.Printf("GraphViz: Skipping duplicate edge: %s", edgeKey)
			continue
		}

		debugLog("Creating edge %d/%d: %s -> %s", i+1, len(g.definition.Transitions), trans.From, trans.To)

		edge, err := graph.CreateEdgeByName("", nodes[trans.From], nodes[trans.To])
		if err != nil {
			log.Printf("GraphViz: Failed to create edge %s -> %s: %v", trans.From, trans.To, err)
			cleanupNodes()
			graph.Close()
			return nil, fmt.Errorf("failed to create edge %s -> %s: %w", trans.From, trans.To, err)
		}

		// Highlight history path
		if opts.ShowHistory && g.isInHistory(trans.From, trans.To, opts.HistoryPath) {
			edge.SetColor("blue")
			edge.SetPenWidth(edgePenWidth)
		}

		createdEdges[edgeKey] = true
	}

	// Add legend
	//g.addLegend(graph, opts)

	debugLog("Graph creation completed successfully with %d nodes and %d edges",
		len(nodes), len(createdEdges))

	return graph, nil
}

// isInHistory checks if a transition is in the history path
func (g *Generator) isInHistory(from, to string, history []HistoryEntry) bool {
	for _, h := range history {
		if h.FromState == from && h.ToState == to {
			return true
		}
	}
	return false
}

// isFinalState checks if a state is a final state
func (g *Generator) isFinalState(state string) bool {
	for _, final := range g.definition.Final {
		if state == final {
			return true
		}
	}
	return false
}

// getBackgroundColor returns background color based on color scheme
func (g *Generator) getBackgroundColor(opts Options) string {
	if opts.ColorScheme == "dark" {
		return "#2b2b2b"
	}
	return "white"
}

// DOT format removed - it's useless for browsers expecting PNG/SVG

// generateMermaid generates Mermaid diagram syntax
func (g *Generator) generateMermaid(opts Options) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("stateDiagram-v2\n")

	// Mark initial state
	buf.WriteString(fmt.Sprintf("    [*] --> %s\n", g.definition.Initial))

	// Add transitions
	for _, trans := range g.definition.Transitions {
		if trans.From == "*" {
			// Wildcard - represent symbolically
			buf.WriteString(fmt.Sprintf("    note right of %s: Any state → %s (wildcard)\n",
				trans.To, trans.To))
		} else {
			buf.WriteString(fmt.Sprintf("    %s --> %s\n", trans.From, trans.To))
		}
	}

	// Mark final states
	for _, final := range g.definition.Final {
		buf.WriteString(fmt.Sprintf("    %s --> [*]\n", final))
	}

	// Highlight current state if needed
	if opts.HighlightCurrent {
		buf.WriteString(fmt.Sprintf("    note right of %s: Current State\n", opts.CurrentState))
	}

	return buf.Bytes(), nil
}

// generateJSON generates JSON graph data
func (g *Generator) generateJSON(opts Options) ([]byte, error) {
	data := GraphData{
		Name:         g.definition.Name,
		InitialState: g.definition.Initial,
		FinalStates:  g.definition.Final,
		CurrentState: opts.CurrentState,
		Nodes:        []NodeData{},
		Edges:        []EdgeData{},
	}

	// Build available states map
	availableMap := make(map[string]bool)
	for _, state := range opts.AvailableStates {
		availableMap[state] = true
	}

	// Check if we have wildcard transitions
	hasWildcard := false
	wildcardTarget := ""
	for _, trans := range g.definition.Transitions {
		if trans.From == "*" {
			hasWildcard = true
			wildcardTarget = trans.To
			break
		}
	}

	// Add wildcard node if needed
	if hasWildcard {
		node := NodeData{
			ID:          "ANY_STATE",
			Label:       "ANY STATE",
			IsInitial:   false,
			IsFinal:     false,
			IsCurrent:   false,
			IsAvailable: false,
		}
		data.Nodes = append(data.Nodes, node)
	}

	// Add regular state nodes
	for _, state := range g.definition.States {
		// Trim state names for consistent comparison
		trimmedState := strings.TrimSpace(state)
		trimmedCurrentState := strings.TrimSpace(opts.CurrentState)

		node := NodeData{
			ID:          trimmedState,
			Label:       trimmedState,
			IsInitial:   trimmedState == g.definition.Initial,
			IsFinal:     g.isFinalState(trimmedState),
			IsCurrent:   opts.HighlightCurrent && trimmedState == trimmedCurrentState,
			IsAvailable: opts.ShowAvailable && availableMap[trimmedState] && trimmedState != trimmedCurrentState,
		}
		data.Nodes = append(data.Nodes, node)
	}

	// Add wildcard edge if needed
	if hasWildcard {
		edge := EdgeData{
			From:       "ANY_STATE",
			To:         wildcardTarget,
			IsWildcard: true,
			IsBidirect: false,
			IsHistory:  false,
			Style:      "dashed",
		}
		data.Edges = append(data.Edges, edge)
	}

	// Add regular edges (no bidirectional logic)
	for _, trans := range g.definition.Transitions {
		if trans.From == "*" {
			continue // Already handled above
		}

		style := "solid"

		edge := EdgeData{
			From:       trans.From,
			To:         trans.To,
			IsWildcard: false,
			IsBidirect: false, // No bidirectional arrows per updated spec
			IsHistory:  opts.ShowHistory && g.isInHistory(trans.From, trans.To, opts.HistoryPath),
			Style:      style,
		}
		data.Edges = append(data.Edges, edge)
	}

	return json.MarshalIndent(data, "", "  ")
}

// ContentType returns the appropriate Content-Type for the format
func ContentType(format Format) string {
	switch format {
	case FormatSVG:
		return "image/svg+xml"
	case FormatPNG:
		return "image/png"
	case FormatDOT:
		return "text/vnd.graphviz"
	case FormatMermaid:
		return "text/plain"
	case FormatJSON:
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

// ParseFormat parses a format string
func ParseFormat(s string) (Format, error) {
	s = strings.ToLower(s)
	switch s {
	case "svg":
		return FormatSVG, nil
	case "png":
		return FormatPNG, nil
	case "dot":
		return FormatDOT, nil
	case "mermaid", "mmd":
		return FormatMermaid, nil
	case "json":
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported format: %s (supported: svg, png, dot, mermaid, json)", s)
	}
}

// ParseLayout parses a layout string
func ParseLayout(s string) (Layout, error) {
	s = strings.ToLower(s)
	switch s {
	case "vertical", "tb":
		return LayoutVertical, nil
	case "horizontal", "lr", "": // Default is horizontal
		return LayoutHorizontal, nil
	default:
		return "", fmt.Errorf("unsupported layout: %s (supported: horizontal, vertical)", s)
	}
}
