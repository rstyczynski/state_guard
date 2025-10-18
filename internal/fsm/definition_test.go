package fsm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefinitionValidate(t *testing.T) {
	tests := []struct {
		name    string
		def     Definition
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid definition",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "A",
				Final:   []string{"C"},
				States:  []string{"A", "B", "C"},
				Transitions: []Transition{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid version",
			def: Definition{
				Version: 2,
				Name:    "test",
				Initial: "A",
				States:  []string{"A"},
				Transitions: []Transition{
					{From: "A", To: "A"},
				},
			},
			wantErr: true,
			errMsg:  "unsupported version",
		},
		{
			name: "missing name",
			def: Definition{
				Version: 1,
				Name:    "",
				Initial: "A",
				States:  []string{"A"},
				Transitions: []Transition{
					{From: "A", To: "A"},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "missing initial state",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "",
				States:  []string{"A"},
				Transitions: []Transition{
					{From: "A", To: "A"},
				},
			},
			wantErr: true,
			errMsg:  "initial state is required",
		},
		{
			name: "no states",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "A",
				States:  []string{},
				Transitions: []Transition{
					{From: "A", To: "A"},
				},
			},
			wantErr: true,
			errMsg:  "at least one state is required",
		},
		{
			name: "no transitions",
			def: Definition{
				Version:     1,
				Name:        "test",
				Initial:     "A",
				States:      []string{"A"},
				Transitions: []Transition{},
			},
			wantErr: true,
			errMsg:  "at least one transition is required",
		},
		{
			name: "empty state name",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "A",
				States:  []string{"A", ""},
				Transitions: []Transition{
					{From: "A", To: "A"},
				},
			},
			wantErr: true,
			errMsg:  "empty state name not allowed",
		},
		{
			name: "duplicate state",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "A",
				States:  []string{"A", "B", "A"},
				Transitions: []Transition{
					{From: "A", To: "B"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate state",
		},
		{
			name: "initial state not in states",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "X",
				States:  []string{"A", "B"},
				Transitions: []Transition{
					{From: "A", To: "B"},
				},
			},
			wantErr: true,
			errMsg:  "initial state 'X' not in states list",
		},
		{
			name: "final state not in states",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "A",
				Final:   []string{"X"},
				States:  []string{"A", "B"},
				Transitions: []Transition{
					{From: "A", To: "B"},
				},
			},
			wantErr: true,
			errMsg:  "final state 'X' not in states list",
		},
		{
			name: "transition with empty from",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "A",
				States:  []string{"A", "B"},
				Transitions: []Transition{
					{From: "", To: "B"},
				},
			},
			wantErr: true,
			errMsg:  "from and to are required",
		},
		{
			name: "transition with empty to",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "A",
				States:  []string{"A", "B"},
				Transitions: []Transition{
					{From: "A", To: ""},
				},
			},
			wantErr: true,
			errMsg:  "from and to are required",
		},
		{
			name: "transition from non-existent state",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "A",
				States:  []string{"A", "B"},
				Transitions: []Transition{
					{From: "X", To: "B"},
				},
			},
			wantErr: true,
			errMsg:  "from state 'X' not in states list",
		},
		{
			name: "transition to non-existent state",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "A",
				States:  []string{"A", "B"},
				Transitions: []Transition{
					{From: "A", To: "X"},
				},
			},
			wantErr: true,
			errMsg:  "to state 'X' not in states list",
		},
		{
			name: "wildcard transition",
			def: Definition{
				Version: 1,
				Name:    "test",
				Initial: "A",
				States:  []string{"A", "B", "FAILED"},
				Transitions: []Transition{
					{From: "A", To: "B"},
					{From: "*", To: "FAILED"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.def.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing '%s', got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = '%v', want error containing '%s'", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestLoadDefinition(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		filename string
		wantErr  bool
	}{
		{
			name: "valid YAML",
			content: `version: 1
name: test
initial: A
states:
  - A
  - B
transitions:
  - from: A
    to: B
`,
			filename: "valid.yaml",
			wantErr:  false,
		},
		{
			name:     "invalid YAML syntax",
			content:  "invalid: yaml: content:",
			filename: "invalid.yaml",
			wantErr:  true,
		},
		{
			name: "invalid definition",
			content: `version: 1
name: test
initial: X
states:
  - A
transitions:
  - from: A
    to: A
`,
			filename: "invalid_def.yaml",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filepath := filepath.Join(tempDir, tt.filename)
			err := os.WriteFile(filepath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			def, err := LoadDefinition(filepath)
			if tt.wantErr {
				if err == nil {
					t.Error("LoadDefinition() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("LoadDefinition() unexpected error = %v", err)
				}
				if def == nil {
					t.Error("LoadDefinition() returned nil definition")
				}
			}
		})
	}

	// Test non-existent file
	t.Run("non-existent file", func(t *testing.T) {
		_, err := LoadDefinition("/nonexistent/file.yaml")
		if err == nil {
			t.Error("LoadDefinition() expected error for non-existent file, got nil")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
