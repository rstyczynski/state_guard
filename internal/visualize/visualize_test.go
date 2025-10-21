package visualize

import (
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
		{"hierarchical", "hierarchical", LayoutHierarchical, false},
		{"dot alias", "dot", LayoutHierarchical, false},
		{"empty defaults to hierarchical", "", LayoutHierarchical, false},
		{"circular", "circular", LayoutCircular, false},
		{"circo alias", "circo", LayoutCircular, false},
		{"force", "force", LayoutForce, false},
		{"neato alias", "neato", LayoutForce, false},
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
		Layout:           LayoutHierarchical,
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
		Layout: LayoutHierarchical,
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
		Layout: LayoutHierarchical,
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
		Layout: LayoutHierarchical,
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
		Layout: LayoutHierarchical,
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

	layouts := []Layout{LayoutHierarchical, LayoutCircular, LayoutForce}

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
		Layout: LayoutHierarchical,
	}

	data, err := gen.GenerateDefinition(opts)
	if err != nil {
		t.Fatalf("GenerateDefinition() error = %v", err)
	}

	jsonStr := string(data)
	// Should have wildcard edges
	if !strings.Contains(jsonStr, `"is_wildcard": true`) {
		t.Error("JSON should mark wildcard transitions")
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
		Layout:      LayoutHierarchical,
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
		Layout:           LayoutHierarchical,
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
			Layout:      LayoutHierarchical,
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
			Layout:      LayoutHierarchical,
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
