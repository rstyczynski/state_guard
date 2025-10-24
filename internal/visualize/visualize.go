package visualize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"github.com/rstyczynski/fsm_v2/internal/fsm"
	"github.com/rstyczynski/fsm_v2/internal/storage"
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
	LayoutVertical     Layout = "vertical"
	LayoutHorizontal   Layout = "horizontal"
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

// GenerateDefinition generates a diagram for the FSM definition only
func (g *Generator) GenerateDefinition(opts Options) ([]byte, error) {
	switch opts.Format {
	case FormatJSON:
		return g.generateJSON(opts)
	case FormatMermaid:
		return g.generateMermaid(opts)
	case FormatDOT:
		return g.generateDOT(opts)
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

	// Set current state (only if not already set by query parameter)
	opts.HighlightCurrent = true
	if opts.CurrentState == "" {
		// Trim whitespace to ensure exact matching
		opts.CurrentState = strings.TrimSpace(instance.CurrentState)
	} else {
		// Also trim if set by query parameter
		opts.CurrentState = strings.TrimSpace(opts.CurrentState)
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
	ctx := context.Background()
	gv, err := graphviz.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create graphviz: %w", err)
	}
	defer gv.Close()

	graph, err := g.createGraph(gv, opts)
	if err != nil {
		return nil, err
	}
	defer graph.Close()

	var buf bytes.Buffer

	switch opts.Format {
	case FormatSVG:
		if err := gv.Render(ctx, graph, graphviz.SVG, &buf); err != nil {
			return nil, fmt.Errorf("SVG render failed: %w", err)
		}
	case FormatPNG:
		if err := gv.Render(ctx, graph, graphviz.PNG, &buf); err != nil {
			return nil, fmt.Errorf("PNG render failed: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// createGraph builds the GraphViz graph
func (g *Generator) createGraph(gv *graphviz.Graphviz, opts Options) (*cgraph.Graph, error) {
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

	graph, err := gv.Graph()
	if err != nil {
		return nil, err
	}

	// Set layout engine (graphs are directed by default in GraphViz DOT)
	graph.SetLayout(layoutEngine)

	// Set graph attributes
	graph.SetRankDir(rankDir)
	graph.SetBackgroundColor(g.getBackgroundColor(opts))

	// Create nodes for each state
	nodes := make(map[string]*cgraph.Node)
	availableMap := make(map[string]bool)
	for _, state := range opts.AvailableStates {
		availableMap[state] = true
	}

	for _, state := range g.definition.States {
		node, err := graph.CreateNodeByName(state)
		if err != nil {
			return nil, err
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
		node.SetWidth(1.5)
		node.SetHeight(0.6)
		node.SetFixedSize(true)

		// Set colors (order matters: check isCurrent first!)
		if isCurrent {
			// Current state always gets green, even if it's FAILED/ERROR
			node.SetFillColor("lightgreen")
			node.SetStyle(cgraph.FilledNodeStyle)
			//node.SetPenWidth(3.0)
		} else if state == "FAILED" || state == "ERROR" {
			// Error states get coral (but not when current)
			node.SetFillColor("lightcoral")
			node.SetStyle(cgraph.FilledNodeStyle)
		} else if isAvailable {
			node.SetFillColor("lightyellow")
			node.SetStyle(cgraph.FilledNodeStyle)
		} else if isInitial {
			// Bold outline for initial state
			node.SetPenWidth(2.0)
			node.SetStyle(cgraph.FilledNodeStyle)
			node.SetFillColor("lightblue")
		} else {
			node.SetFillColor("white")
			node.SetStyle(cgraph.FilledNodeStyle)
		}

		// Double outline for final states
		if isFinal && !isCurrent {
			node.SetPenWidth(4.0)
			node.SetStyle(cgraph.FilledNodeStyle)
		}

		nodes[state] = node
	}

	// Create edges for transitions
	createdEdges := make(map[string]bool)
	hasWildcard := false
	wildcardTarget := ""

	// First pass: check for wildcard transitions
	for _, trans := range g.definition.Transitions {
		if trans.From == "*" {
			hasWildcard = true
			wildcardTarget = trans.To
			break
		}
	}

	// If we have wildcard, create a special "ANY STATE" node
	if hasWildcard {
		anyNode, err := graph.CreateNodeByName("ANY_STATE")
		if err != nil {
			return nil, err
		}
		anyNode.SetLabel("ANY STATE")
		anyNode.SetShape(cgraph.BoxShape)
		anyNode.SetFillColor("white")
		anyNode.SetStyle(cgraph.FilledNodeStyle)

		// Create arrow from ANY STATE to wildcard target
		edge, err := graph.CreateEdgeByName("", anyNode, nodes[wildcardTarget])
		if err != nil {
			return nil, err
		}
		edge.SetColor("red")
		edge.SetPenWidth(2.0)
		edge.SetStyle(cgraph.DashedEdgeStyle)
	}

	// Second pass: create regular transitions (no bidirectional handling)
	for _, trans := range g.definition.Transitions {
		if trans.From == "*" {
			continue // Already handled above
		}

		// Regular transition
		edgeKey := trans.From + "->" + trans.To
		if createdEdges[edgeKey] {
			continue
		}

		edge, err := graph.CreateEdgeByName("", nodes[trans.From], nodes[trans.To])
		if err != nil {
			return nil, err
		}

		// Highlight history path
		if opts.ShowHistory && g.isInHistory(trans.From, trans.To, opts.HistoryPath) {
			edge.SetColor("blue")
			edge.SetPenWidth(2.0)
		}

		createdEdges[edgeKey] = true
	}

	// Add legend
	//g.addLegend(graph, opts)

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

// generateDOT generates DOT format output
func (g *Generator) generateDOT(opts Options) ([]byte, error) {
	ctx := context.Background()
	gv, err := graphviz.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create graphviz: %w", err)
	}
	defer gv.Close()

	graph, err := g.createGraph(gv, opts)
	if err != nil {
		return nil, err
	}
	defer graph.Close()

	// Render to DOT format
	var buf bytes.Buffer
	if err := gv.Render(ctx, graph, graphviz.XDOT, &buf); err != nil {
		return nil, fmt.Errorf("DOT render failed: %w", err)
	}

	return buf.Bytes(), nil
}

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
