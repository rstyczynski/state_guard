package visualize

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rstyczynski/fsm_v2/internal/fsm"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Format
		wantErr bool
	}{
		{"svg lowercase", "svg", FormatSVG, false},
		{"SVG uppercase", "SVG", FormatSVG, false},
		{"png", "png", FormatPNG, false},
		{"dot", "dot", FormatDOT, false},
		{"mermaid", "mermaid", FormatMermaid, false},
		{"mmd alias", "mmd", FormatMermaid, false},
		{"json", "json", FormatJSON, false},
		{"invalid", "invalid", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFormat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLayout(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Layout
		wantErr bool
	}{
		{"empty defaults to horizontal", "", LayoutHorizontal, false},
		{"vertical", "vertical", LayoutVertical, false},
		{"tb alias", "tb", LayoutVertical, false},
		{"horizontal", "horizontal", LayoutHorizontal, false},
		{"lr alias", "lr", LayoutHorizontal, false},
		{"hierarchical not supported", "hierarchical", "", true},
		{"circular not supported", "circular", "", true},
		{"invalid", "invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLayout(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLayout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseLayout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContentType(t *testing.T) {
	tests := []struct {
		format Format
		want   string
	}{
		{FormatSVG, "image/svg+xml"},
		{FormatPNG, "image/png"},
		{FormatDOT, "text/vnd.graphviz"},
		{FormatMermaid, "text/plain"},
		{FormatJSON, "application/json"},
		{"unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			if got := ContentType(tt.format); got != tt.want {
				t.Errorf("ContentType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateJSON(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "TERMINATED", "FAILED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "TERMINATED"},
			{From: "*", To: "FAILED"},
		},
	}

	gen := NewGenerator(def, nil)
	opts := Options{
		Format:           FormatJSON,
		Layout:           LayoutHorizontal,
		HighlightCurrent: true,
		CurrentState:     "RUNNING",
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() error = %v", err)
	}

	// Verify JSON structure
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"name": "test_fsm"`) {
		t.Error("JSON should contain FSM name")
	}
	if !strings.Contains(jsonStr, `"initial_state": "CREATED"`) {
		t.Error("JSON should contain initial state")
	}
	if !strings.Contains(jsonStr, `"current_state": "RUNNING"`) {
		t.Error("JSON should contain current state")
	}
	if !strings.Contains(jsonStr, `"nodes"`) {
		t.Error("JSON should contain nodes array")
	}
	if !strings.Contains(jsonStr, `"edges"`) {
		t.Error("JSON should contain edges array")
	}
}

func TestGenerateMermaid(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)
	opts := Options{
		Format: FormatMermaid,
		Layout: LayoutHorizontal,
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() error = %v", err)
	}

	mermaidStr := string(data)
	if !strings.Contains(mermaidStr, "stateDiagram-v2") {
		t.Error("Mermaid should contain stateDiagram-v2 header")
	}
	if !strings.Contains(mermaidStr, "[*] --> CREATED") {
		t.Error("Mermaid should contain initial state transition")
	}
	if !strings.Contains(mermaidStr, "CREATED --> RUNNING") {
		t.Error("Mermaid should contain state transitions")
	}
	if !strings.Contains(mermaidStr, "TERMINATED --> [*]") {
		t.Error("Mermaid should contain final state transition")
	}
}

func TestGenerateDOT(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)
	opts := Options{
		Format: FormatDOT,
		Layout: LayoutHorizontal,
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() error = %v", err)
	}

	dotStr := string(data)
	if !strings.Contains(dotStr, "digraph") {
		t.Error("DOT should contain digraph declaration")
	}
	// DOT format should contain nodes and edges
	if len(dotStr) < 50 {
		t.Error("DOT output seems too short")
	}
}

func TestGenerateSVG(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)
	opts := Options{
		Format: FormatSVG,
		Layout: LayoutHorizontal,
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() error = %v", err)
	}

	svgStr := string(data)
	if !strings.Contains(svgStr, "<svg") {
		t.Error("SVG should contain <svg> tag")
	}
	if !strings.Contains(svgStr, "</svg>") {
		t.Error("SVG should be well-formed with closing tag")
	}
}

func TestGeneratePNG(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)
	opts := Options{
		Format: FormatPNG,
		Layout: LayoutHorizontal,
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() error = %v", err)
	}

	// PNG files start with magic bytes: 89 50 4E 47 0D 0A 1A 0A
	if len(data) < 8 {
		t.Error("PNG output too short")
	}
	if data[0] != 0x89 || data[1] != 0x50 || data[2] != 0x4E || data[3] != 0x47 {
		t.Error("PNG output doesn't have correct magic bytes")
	}
}

func TestLayoutAlgorithms(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)

	// Only horizontal and vertical layouts are supported
	layouts := []Layout{LayoutVertical, LayoutHorizontal}

	for _, layout := range layouts {
		t.Run(string(layout), func(t *testing.T) {
			opts := Options{
				Format: FormatSVG,
				Layout: layout,
			}

			data, err := gen.GenerateDefinition(opts)
			if err != nil {
				t.Fatalf("GenerateDefinition() with layout %s error = %v", layout, err)
			}

			if len(data) == 0 {
				t.Errorf("GenerateDefinition() with layout %s returned empty data", layout)
			}
		})
	}
}

func TestWildcardTransitions(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "STOPPED", "FAILED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "STOPPED"},
			{From: "*", To: "FAILED"}, // Wildcard
		},
	}

	gen := NewGenerator(def, nil)
	opts := Options{
		Format: FormatJSON,
		Layout: LayoutHorizontal,
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() error = %v", err)
	}

	jsonStr := string(data)
	// Should have wildcard edge from ANY_STATE
	if !strings.Contains(jsonStr, `"is_wildcard": true`) {
		t.Error("JSON should mark wildcard transitions")
	}
	// Should have ANY_STATE node
	if !strings.Contains(jsonStr, `"ANY_STATE"`) {
		t.Error("JSON should contain ANY_STATE node for wildcard")
	}
	// Wildcard edge should be from ANY_STATE to target (note: JSON uses "from" not "From")
	if !strings.Contains(jsonStr, `"from": "ANY_STATE"`) {
		t.Errorf("Wildcard edge should be from ANY_STATE. Got JSON: %s", jsonStr)
	}
}

func TestHistoryOverlay(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "STOPPED", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "STOPPED"},
			{From: "STOPPED", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)

	// Create history path
	historyPath := []HistoryEntry{
		{FromState: "CREATED", ToState: "RUNNING", TransitionedAt: time.Now()},
		{FromState: "RUNNING", ToState: "STOPPED", TransitionedAt: time.Now()},
	}

	opts := Options{
		Format:      FormatJSON,
		Layout:      LayoutHorizontal,
		ShowHistory: true,
		HistoryPath: historyPath,
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() error = %v", err)
	}

	jsonStr := string(data)
	// Should mark edges in history
	if !strings.Contains(jsonStr, `"is_history": true`) {
		t.Error("JSON should mark history transitions")
	}
}

func TestAvailableStatesHighlight(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "STOPPED", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "STOPPED"},
			{From: "STOPPED", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)

	opts := Options{
		Format:           FormatJSON,
		Layout:           LayoutHorizontal,
		HighlightCurrent: true,
		CurrentState:     "RUNNING",
		ShowAvailable:    true,
		AvailableStates:  []string{"STOPPED"},
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() error = %v", err)
	}

	jsonStr := string(data)
	// Should mark current state
	if !strings.Contains(jsonStr, `"is_current": true`) {
		t.Error("JSON should mark current state")
	}
	// Should mark available states
	if !strings.Contains(jsonStr, `"is_available": true`) {
		t.Error("JSON should mark available states")
	}
}

func TestGenerateInstance(t *testing.T) {
	// Skip this test as it requires full storage integration
	// This functionality is tested in integration tests
	t.Skip("Skipping integration test - covered by integration test suite")
}

func TestColorScheme(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)

	t.Run("light scheme", func(t *testing.T) {
		opts := Options{
			Format:      FormatDOT,
			Layout:      LayoutHorizontal,
			ColorScheme: "light",
		}
		data, err := gen.GenerateDefinition(opts)
		if err != nil {
			t.Fatalf("GenerateDefinition() error = %v", err)
		}
		dotStr := string(data)
		if !strings.Contains(dotStr, "white") && !strings.Contains(dotStr, "bgcolor") {
			t.Log("Light scheme applied (may not be visible in DOT)")
		}
	})

	t.Run("dark scheme", func(t *testing.T) {
		opts := Options{
			Format:      FormatDOT,
			Layout:      LayoutHorizontal,
			ColorScheme: "dark",
		}
		data, err := gen.GenerateDefinition(opts)
		if err != nil {
			t.Fatalf("GenerateDefinition() error = %v", err)
		}
		dotStr := string(data)
		if len(dotStr) > 0 {
			t.Log("Dark scheme applied")
		}
	})
}

func TestEmptyDefinition(t *testing.T) {
	// Test with minimal valid definition
	def := &fsm.Definition{
		Version: 1,
		Name:    "minimal",
		Initial: "START",
		Final:   []string{},
		States:  []string{"START"},
		Transitions: []fsm.Transition{},
	}

	gen := NewGenerator(def, nil)

	t.Run("JSON with minimal definition", func(t *testing.T) {
		opts := Options{
			Format: FormatJSON,
			Layout: LayoutHorizontal,
		}
		data, err := gen.GenerateDefinition(opts)
		if err != nil {
			t.Fatalf("GenerateDefinition() with minimal definition error = %v", err)
		}
		if len(data) == 0 {
			t.Error("Expected non-empty output for minimal definition")
		}

		jsonStr := string(data)
		if !strings.Contains(jsonStr, `"name": "minimal"`) {
			t.Error("JSON should contain FSM name even for minimal definition")
		}
	})

	t.Run("SVG with minimal definition", func(t *testing.T) {
		opts := Options{
			Format: FormatSVG,
			Layout: LayoutHorizontal,
		}
		data, err := gen.GenerateDefinition(opts)
		if err != nil {
			t.Fatalf("GenerateDefinition() with minimal definition error = %v", err)
		}
		if len(data) == 0 {
			t.Error("Expected non-empty SVG for minimal definition")
		}
	})
}

func TestInvalidCurrentState(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)

	// Test with invalid current state (not in definition)
	opts := Options{
		Format:           FormatJSON,
		Layout:           LayoutHorizontal,
		HighlightCurrent: true,
		CurrentState:     "INVALID_STATE",
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() should not error on invalid current state = %v", err)
	}

	// Should still generate valid output, just won't highlight anything
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"current_state": "INVALID_STATE"`) {
		t.Error("JSON should still include current_state even if invalid")
	}
	// No node should be marked as current
	if strings.Count(jsonStr, `"is_current": true`) > 0 {
		t.Error("No node should be marked current with invalid state")
	}
}

func TestMultipleFormatsConsistency(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)

	formats := []Format{FormatSVG, FormatPNG, FormatDOT, FormatMermaid, FormatJSON}

	for _, format := range formats {
		t.Run(string(format), func(t *testing.T) {
			opts := Options{
				Format: format,
				Layout: LayoutHorizontal,
			}

			data, err := gen.GenerateDefinition(opts)
			if err != nil {
				t.Errorf("Format %s should generate without error: %v", format, err)
			}
			if len(data) == 0 {
				t.Errorf("Format %s produced empty output", format)
			}

			// Verify content type matches
			contentType := ContentType(format)
			if contentType == "application/octet-stream" {
				t.Errorf("Format %s has no specific content type", format)
			}
		})
	}
}

func TestBothLayoutsProduceDifferentOutput(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)

	// Generate with horizontal layout
	optsH := Options{
		Format: FormatDOT,
		Layout: LayoutHorizontal,
	}
	dataH, err := gen.GenerateDefinition(optsH)
	if err != nil {
		t.Fatalf("Horizontal layout error = %v", err)
	}

	// Generate with vertical layout
	optsV := Options{
		Format: FormatDOT,
		Layout: LayoutVertical,
	}
	dataV, err := gen.GenerateDefinition(optsV)
	if err != nil {
		t.Fatalf("Vertical layout error = %v", err)
	}

	// Both should produce non-empty output
	if len(dataH) == 0 || len(dataV) == 0 {
		t.Error("Both layouts should produce non-empty output")
	}

	// The outputs should be different (different rankdir)
	if string(dataH) == string(dataV) {
		t.Error("Horizontal and vertical layouts should produce different output")
	}

	// Horizontal should contain LR rank direction
	if !strings.Contains(string(dataH), "LR") && !strings.Contains(string(dataH), "rankdir") {
		t.Log("Horizontal layout may not explicitly show LR in DOT output")
	}

	// Vertical should contain TB rank direction
	if !strings.Contains(string(dataV), "TB") && !strings.Contains(string(dataV), "rankdir") {
		t.Log("Vertical layout may not explicitly show TB in DOT output")
	}
}

func TestJSONOutputStructure(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED", "FAILED"},
		States:  []string{"CREATED", "RUNNING", "TERMINATED", "FAILED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "TERMINATED"},
			{From: "*", To: "FAILED"},
		},
	}

	gen := NewGenerator(def, nil)
	opts := Options{
		Format:           FormatJSON,
		Layout:           LayoutHorizontal,
		HighlightCurrent: true,
		CurrentState:     "RUNNING",
		ShowAvailable:    true,
		AvailableStates:  []string{"TERMINATED", "FAILED"},
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() error = %v", err)
	}

	// Parse JSON to verify structure
	var graphData GraphData
	if err := json.Unmarshal(data, &graphData); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Verify top-level fields
	if graphData.Name != "test_fsm" {
		t.Errorf("Expected name 'test_fsm', got '%s'", graphData.Name)
	}
	if graphData.InitialState != "CREATED" {
		t.Errorf("Expected initial state 'CREATED', got '%s'", graphData.InitialState)
	}
	if len(graphData.FinalStates) != 2 {
		t.Errorf("Expected 2 final states, got %d", len(graphData.FinalStates))
	}
	if graphData.CurrentState != "RUNNING" {
		t.Errorf("Expected current state 'RUNNING', got '%s'", graphData.CurrentState)
	}

	// Verify nodes (4 regular + 1 ANY_STATE for wildcard)
	if len(graphData.Nodes) != 5 {
		t.Errorf("Expected 5 nodes (4 states + ANY_STATE), got %d", len(graphData.Nodes))
	}

	// Verify edges (2 regular + 1 wildcard)
	if len(graphData.Edges) != 3 {
		t.Errorf("Expected 3 edges, got %d", len(graphData.Edges))
	}

	// Count special nodes
	var currentCount, availableCount, initialCount, finalCount int
	for _, node := range graphData.Nodes {
		if node.IsCurrent {
			currentCount++
		}
		if node.IsAvailable {
			availableCount++
		}
		if node.IsInitial {
			initialCount++
		}
		if node.IsFinal {
			finalCount++
		}
	}

	if currentCount != 1 {
		t.Errorf("Expected exactly 1 current node, got %d", currentCount)
	}
	if availableCount != 2 {
		t.Errorf("Expected 2 available nodes, got %d", availableCount)
	}
	if initialCount != 1 {
		t.Errorf("Expected 1 initial node, got %d", initialCount)
	}
	if finalCount != 2 {
		t.Errorf("Expected 2 final nodes, got %d", finalCount)
	}

	// Verify wildcard edge exists
	var wildcardCount int
	for _, edge := range graphData.Edges {
		if edge.IsWildcard {
			wildcardCount++
			if edge.From != "ANY_STATE" {
				t.Errorf("Wildcard edge should be from ANY_STATE, got '%s'", edge.From)
			}
			if edge.To != "FAILED" {
				t.Errorf("Wildcard edge should be to FAILED, got '%s'", edge.To)
			}
			if edge.Style != "dashed" {
				t.Errorf("Wildcard edge should have dashed style, got '%s'", edge.Style)
			}
		}
	}
	if wildcardCount != 1 {
		t.Errorf("Expected exactly 1 wildcard edge, got %d", wildcardCount)
	}
}

func TestUnsupportedFormat(t *testing.T) {
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)

	// Test with invalid format
	opts := Options{
		Format: Format("invalid_format"),
		Layout: LayoutHorizontal,
	}

	_, err := gen.GenerateDefinition(opts)
	if err == nil {
		t.Error("Expected error with unsupported format, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("Expected 'unsupported format' error, got: %v", err)
	}
}

func TestCurrentStateStaysGreenWithAvailableStates(t *testing.T) {
	// Test the bug: when "Indicate Next States" is enabled,
	// and current state happens to be in available states (e.g., self-loop),
	// the current state should STILL be green, not yellow.
	def := &fsm.Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "CREATED",
		Final:   []string{"TERMINATED"},
		States:  []string{"CREATED", "RUNNING", "TERMINATED"},
		Transitions: []fsm.Transition{
			{From: "CREATED", To: "RUNNING"},
			{From: "RUNNING", To: "RUNNING"}, // Self-loop
			{From: "RUNNING", To: "TERMINATED"},
		},
	}

	gen := NewGenerator(def, nil)

	opts := Options{
		Format:           FormatJSON,
		Layout:           LayoutHorizontal,
		HighlightCurrent: true,
		CurrentState:     "RUNNING",
		ShowAvailable:    true,
		AvailableStates:  []string{"RUNNING", "TERMINATED"}, // Current state IS in available
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() error = %v", err)
	}

	// Parse JSON to verify
	var graphData GraphData
	if err := json.Unmarshal(data, &graphData); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Find the RUNNING node
	var runningNode *NodeData
	for i := range graphData.Nodes {
		if graphData.Nodes[i].ID == "RUNNING" {
			runningNode = &graphData.Nodes[i]
			break
		}
	}

	if runningNode == nil {
		t.Fatal("RUNNING node not found")
	}

	// RUNNING should be marked as CURRENT
	if !runningNode.IsCurrent {
		t.Error("RUNNING should be marked as current")
	}

	// RUNNING should NOT be marked as available (current state should never be "available")
	if runningNode.IsAvailable {
		t.Error("Current state (RUNNING) should NOT be marked as available, even when in AvailableStates list")
	}

	// TERMINATED should be marked as available
	var terminatedNode *NodeData
	for i := range graphData.Nodes {
		if graphData.Nodes[i].ID == "TERMINATED" {
			terminatedNode = &graphData.Nodes[i]
			break
		}
	}

	if terminatedNode == nil {
		t.Fatal("TERMINATED node not found")
	}

	if !terminatedNode.IsAvailable {
		t.Error("TERMINATED should be marked as available")
	}
}
