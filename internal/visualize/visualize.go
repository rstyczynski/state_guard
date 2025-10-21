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
	LayoutHierarchical Layout = "hierarchical"
	LayoutCircular     Layout = "circular"
	LayoutForce        Layout = "force"
)

// Options for diagram generation
type Options struct {
	Format            Format
	Layout            Layout
	HighlightCurrent  bool
	ShowHistory       bool
	ShowAvailable     bool
	CurrentState      string
	AvailableStates   []string
	HistoryPath       []HistoryEntry
	ColorScheme       string // "light" or "dark"
}

// HistoryEntry represents a state transition in history
type HistoryEntry struct {
	FromState      string
	ToState        string
	TransitionedAt time.Time
}

// GraphData represents structured graph data for JSON export
type GraphData struct {
	Name        string       `json:"name"`
	InitialState string      `json:"initial_state"`
	FinalStates []string     `json:"final_states"`
	CurrentState string      `json:"current_state,omitempty"`
	Nodes       []NodeData   `json:"nodes"`
	Edges       []EdgeData   `json:"edges"`
}

// NodeData represents a state node
type NodeData struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	IsInitial    bool   `json:"is_initial"`
	IsFinal      bool   `json:"is_final"`
	IsCurrent    bool   `json:"is_current"`
	IsAvailable  bool   `json:"is_available"`
}

// EdgeData represents a transition edge
type EdgeData struct {
	From        string `json:"from"`
	To          string `json:"to"`
	IsWildcard  bool   `json:"is_wildcard"`
	IsBidirect  bool   `json:"is_bidirectional"`
	IsHistory   bool   `json:"is_history"`
	Style       string `json:"style"` // "solid", "dashed"
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

	// Set current state
	opts.HighlightCurrent = true
	opts.CurrentState = instance.CurrentState

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
	switch opts.Layout {
	case LayoutHierarchical:
		layoutEngine = "dot"
	case LayoutCircular:
		layoutEngine = "circo"
	case LayoutForce:
		layoutEngine = "neato"
	default:
		layoutEngine = "dot"
	}

	graph, err := gv.Graph()
	if err != nil {
		return nil, err
	}

	// Set layout engine (graphs are directed by default in GraphViz DOT)
	graph.SetLayout(layoutEngine)

	// Set graph attributes
	graph.SetRankDir(cgraph.TBRank) // Top to bottom
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

		// Styling based on state type
		isInitial := state == g.definition.Initial
		isFinal := g.isFinalState(state)
		isCurrent := opts.HighlightCurrent && state == opts.CurrentState
		isAvailable := opts.ShowAvailable && availableMap[state]

		// Set shape
		if isFinal {
			node.SetShape(cgraph.DoubleCircleShape)
		} else {
			node.SetShape(cgraph.CircleShape)
		}

		// Set colors
		if isCurrent {
			node.SetFillColor("lightgreen")
			node.SetStyle(cgraph.FilledNodeStyle)
			node.SetPenWidth(3.0)
		} else if isAvailable {
			node.SetFillColor("lightyellow")
			node.SetStyle(cgraph.FilledNodeStyle)
		} else if isInitial {
			node.SetPenWidth(2.0)
		}

		// Special styling for error states
		if state == "FAILED" || state == "ERROR" {
			node.SetFillColor("lightcoral")
			node.SetStyle(cgraph.FilledNodeStyle)
		}

		nodes[state] = node
	}

	// Build transition map for bidirectional detection
	transitionMap := g.buildTransitionMap()

	// Create edges for transitions
	createdEdges := make(map[string]bool)

	for _, trans := range g.definition.Transitions {
		if trans.From == "*" {
			// Wildcard transition - create from all states
			for _, fromState := range g.definition.States {
				if fromState == trans.To {
					continue // Skip self-loops from wildcards
				}
				edgeKey := fromState + "->" + trans.To
				if createdEdges[edgeKey] {
					continue
				}

				edge, err := graph.CreateEdgeByName("", nodes[fromState], nodes[trans.To])
				if err != nil {
					return nil, err
				}
				edge.SetColor("red")
				edge.SetPenWidth(1.5)
				edge.SetStyle(cgraph.DashedEdgeStyle)
				createdEdges[edgeKey] = true
			}
		} else {
			// Regular transition
			edgeKey := trans.From + "->" + trans.To
			if createdEdges[edgeKey] {
				continue
			}

			// Check if bidirectional
			reverseKey := trans.To + "->" + trans.From
			isBidirectional := transitionMap[reverseKey]

			edge, err := graph.CreateEdgeByName("", nodes[trans.From], nodes[trans.To])
			if err != nil {
				return nil, err
			}

			// Style based on transition type
			if isBidirectional {
				edge.SetDir(cgraph.BothDir)
				createdEdges[reverseKey] = true // Mark reverse as created
			}

			// Dashed for recovery transitions
			if g.isRecoveryTransition(trans.From, trans.To) {
				edge.SetStyle(cgraph.DashedEdgeStyle)
			}

			// Highlight history path
			if opts.ShowHistory && g.isInHistory(trans.From, trans.To, opts.HistoryPath) {
				edge.SetColor("blue")
				edge.SetPenWidth(2.0)
			}

			createdEdges[edgeKey] = true
		}
	}

	return graph, nil
}

// buildTransitionMap creates a map of all transitions for lookup
func (g *Generator) buildTransitionMap() map[string]bool {
	m := make(map[string]bool)
	for _, trans := range g.definition.Transitions {
		if trans.From != "*" {
			key := trans.From + "->" + trans.To
			m[key] = true
		}
	}
	return m
}

// isRecoveryTransition checks if a transition is a recovery/backward transition
func (g *Generator) isRecoveryTransition(from, to string) bool {
	// Heuristic: transitions to STARTING from non-CREATED states
	if to == "STARTING" && from != "CREATED" {
		return true
	}
	// Transitions from STOPPED/FAILED backwards
	if (from == "STOPPED" || from == "FAILED") && to == "STARTING" {
		return true
	}
	return false
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

	// Add nodes
	for _, state := range g.definition.States {
		node := NodeData{
			ID:          state,
			Label:       state,
			IsInitial:   state == g.definition.Initial,
			IsFinal:     g.isFinalState(state),
			IsCurrent:   opts.HighlightCurrent && state == opts.CurrentState,
			IsAvailable: opts.ShowAvailable && availableMap[state],
		}
		data.Nodes = append(data.Nodes, node)
	}

	// Build transition map for bidirectional detection
	transitionMap := g.buildTransitionMap()

	// Add edges
	for _, trans := range g.definition.Transitions {
		if trans.From == "*" {
			// Wildcard transitions
			for _, fromState := range g.definition.States {
				if fromState == trans.To {
					continue
				}
				edge := EdgeData{
					From:       fromState,
					To:         trans.To,
					IsWildcard: true,
					Style:      "dashed",
				}
				data.Edges = append(data.Edges, edge)
			}
		} else {
			// Check if bidirectional
			reverseKey := trans.To + "->" + trans.From
			isBidirectional := transitionMap[reverseKey]

			style := "solid"
			if g.isRecoveryTransition(trans.From, trans.To) {
				style = "dashed"
			}

			edge := EdgeData{
				From:       trans.From,
				To:         trans.To,
				IsWildcard: false,
				IsBidirect: isBidirectional,
				IsHistory:  opts.ShowHistory && g.isInHistory(trans.From, trans.To, opts.HistoryPath),
				Style:      style,
			}
			data.Edges = append(data.Edges, edge)
		}
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
	case "hierarchical", "dot", "":
		return LayoutHierarchical, nil
	case "circular", "circo":
		return LayoutCircular, nil
	case "force", "neato":
		return LayoutForce, nil
	default:
		return "", fmt.Errorf("unsupported layout: %s (supported: hierarchical, circular, force)", s)
	}
}
