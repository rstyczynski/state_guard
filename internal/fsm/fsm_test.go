package fsm

import (
	"testing"
)

func createTestDefinition() *Definition {
	return &Definition{
		Version: 1,
		Name:    "test_fsm",
		Initial: "A",
		Final:   []string{"D"},
		States:  []string{"A", "B", "C", "D", "FAILED"},
		Transitions: []Transition{
			{From: "A", To: "B"},
			{From: "B", To: "C"},
			{From: "C", To: "D"},
			{From: "A", To: "C"}, // Skip transition
			{From: "*", To: "FAILED"},
		},
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		def     *Definition
		id      string
		wantErr bool
	}{
		{
			name:    "valid definition with ID",
			def:     createTestDefinition(),
			id:      "test-asset-1",
			wantErr: false,
		},
		{
			name:    "nil definition",
			def:     nil,
			id:      "test-asset-1",
			wantErr: true,
		},
		{
			name:    "empty ID",
			def:     createTestDefinition(),
			id:      "",
			wantErr: true,
		},
		{
			name:    "invalid ID with spaces",
			def:     createTestDefinition(),
			id:      "test asset",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm, err := New(tt.def, tt.id, nil)
			if tt.wantErr {
				if err == nil {
					t.Error("New() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("New() unexpected error = %v", err)
				return
			}
			if fsm == nil {
				t.Error("New() returned nil FSM")
				return
			}
			if fsm.CurrentState() != tt.def.Initial {
				t.Errorf("New() current state = %v, want %v", fsm.CurrentState(), tt.def.Initial)
			}
			if fsm.ID() != tt.id {
				t.Errorf("New() ID = %v, want %v", fsm.ID(), tt.id)
			}
		})
	}
}

func TestNewFromFile(t *testing.T) {
	t.Run("deprecated function", func(t *testing.T) {
		_, err := NewFromFile("any-file.yaml")
		if err == nil {
			t.Error("NewFromFile() expected deprecation error, got nil")
		}
	})
}

func TestFSM_CurrentState(t *testing.T) {
	def := createTestDefinition()
	f, err := New(def, "test-instance", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := f.CurrentState(); got != "A" {
		t.Errorf("CurrentState() = %v, want A", got)
	}

	// After transition
	f.Transition("B")
	if got := f.CurrentState(); got != "B" {
		t.Errorf("CurrentState() after transition = %v, want B", got)
	}
}

func TestFSM_CanTransition(t *testing.T) {
	def := createTestDefinition()
	f, err := New(def, "test-instance", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name      string
		fromState string
		toState   string
		want      bool
	}{
		{"valid explicit transition", "A", "B", true},
		{"valid skip transition", "A", "C", true},
		{"invalid transition", "A", "D", false},
		{"wildcard transition from A", "A", "FAILED", true},
		{"wildcard transition from B", "B", "FAILED", true},
		{"non-existent target state", "A", "NONEXISTENT", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the current state
			f.Reset()
			if tt.fromState != "A" {
				// Transition to the desired from state
				switch tt.fromState {
				case "B":
					f.Transition("B")
				case "C":
					f.Transition("B")
					f.Transition("C")
				}
			}

			if got := f.CanTransition(tt.toState); got != tt.want {
				t.Errorf("CanTransition(%v) = %v, want %v (from state: %v)",
					tt.toState, got, tt.want, f.CurrentState())
			}
		})
	}
}

func TestFSM_Transition(t *testing.T) {
	tests := []struct {
		name      string
		fromState string
		toState   string
		wantErr   bool
		errType   string
	}{
		{"valid transition", "A", "B", false, ""},
		{"invalid transition", "A", "D", true, "InvalidTransitionError"},
		{"wildcard transition", "A", "FAILED", false, ""},
		{"non-existent state", "A", "NONEXISTENT", true, "UnknownStateError"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := createTestDefinition()
			f, err := New(def, "test-instance", nil)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			// Ensure we're in the correct from state
			f.Reset()
			if tt.fromState != "A" {
				switch tt.fromState {
				case "B":
					f.Transition("B")
				case "C":
					f.Transition("B")
					f.Transition("C")
				}
			}

			err = f.Transition(tt.toState)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Transition(%v) expected error, got nil", tt.toState)
					return
				}

				switch tt.errType {
				case "InvalidTransitionError":
					if _, ok := err.(*InvalidTransitionError); !ok {
						t.Errorf("Transition(%v) error type = %T, want *InvalidTransitionError", tt.toState, err)
					}
				case "UnknownStateError":
					if _, ok := err.(*UnknownStateError); !ok {
						t.Errorf("Transition(%v) error type = %T, want *UnknownStateError", tt.toState, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Transition(%v) unexpected error = %v", tt.toState, err)
					return
				}
				if f.CurrentState() != tt.toState {
					t.Errorf("Transition(%v) current state = %v, want %v", tt.toState, f.CurrentState(), tt.toState)
				}
			}
		})
	}
}

func TestFSM_States(t *testing.T) {
	def := createTestDefinition()
	f, err := New(def, "test-instance", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	states := f.States()
	if len(states) != len(def.States) {
		t.Errorf("States() length = %v, want %v", len(states), len(def.States))
	}

	// Check if all states are present
	stateMap := make(map[string]bool)
	for _, state := range states {
		stateMap[state] = true
	}

	for _, expectedState := range def.States {
		if !stateMap[expectedState] {
			t.Errorf("States() missing state %v", expectedState)
		}
	}
}

func TestFSM_InitialState(t *testing.T) {
	def := createTestDefinition()
	f, err := New(def, "test-instance", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := f.InitialState(); got != "A" {
		t.Errorf("InitialState() = %v, want A", got)
	}
}

func TestFSM_FinalStates(t *testing.T) {
	def := createTestDefinition()
	f, err := New(def, "test-instance", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	finalStates := f.FinalStates()
	if len(finalStates) != len(def.Final) {
		t.Errorf("FinalStates() length = %v, want %v", len(finalStates), len(def.Final))
	}
}

func TestFSM_IsFinalState(t *testing.T) {
	def := createTestDefinition()
	f, err := New(def, "test-instance", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		state string
		want  bool
	}{
		{"A", false},
		{"B", false},
		{"C", false},
		{"D", true},
		{"FAILED", false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			// Transition to the target state
			f.Reset()
			switch tt.state {
			case "B":
				f.Transition("B")
			case "C":
				f.Transition("B")
				f.Transition("C")
			case "D":
				f.Transition("B")
				f.Transition("C")
				f.Transition("D")
			case "FAILED":
				f.Transition("FAILED")
			}

			if got := f.IsFinalState(); got != tt.want {
				t.Errorf("IsFinalState() at state %v = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestFSM_Name(t *testing.T) {
	def := createTestDefinition()
	f, err := New(def, "test-instance", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := f.Name(); got != "test_fsm" {
		t.Errorf("Name() = %v, want test_fsm", got)
	}
}

func TestFSM_ID(t *testing.T) {
	def := createTestDefinition()

	// Test that ID is set correctly
	f1, err := New(def, "server-1", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	id1 := f1.ID()
	if id1 != "server-1" {
		t.Errorf("ID() = %v, want server-1", id1)
	}

	// Test that different instances can have different IDs
	f2, err := New(def, "database-prod", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	id2 := f2.ID()
	if id2 != "database-prod" {
		t.Errorf("ID() = %v, want database-prod", id2)
	}

	if id1 == id2 {
		t.Errorf("ID() returned same ID for different instances: %v", id1)
	}

	// Test that ID is stable across method calls
	if f1.ID() != id1 {
		t.Error("ID() returned different value on subsequent call")
	}
}

func TestFSM_Reset(t *testing.T) {
	def := createTestDefinition()
	f, err := New(def, "test-instance", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Transition to a different state
	f.Transition("B")
	if f.CurrentState() != "B" {
		t.Errorf("CurrentState() after transition = %v, want B", f.CurrentState())
	}

	// Reset
	err = f.Reset()
	if err != nil {
		t.Errorf("Reset() unexpected error = %v", err)
	}
	if f.CurrentState() != "A" {
		t.Errorf("CurrentState() after Reset() = %v, want A", f.CurrentState())
	}
}

func TestFSM_AvailableTransitions(t *testing.T) {
	def := createTestDefinition()
	f, err := New(def, "test-instance", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		state    string
		wantMin  int // Minimum number of transitions (wildcard is always available)
		contains []string
	}{
		{"A", 3, []string{"B", "C", "FAILED"}},
		{"B", 2, []string{"C", "FAILED"}},
		{"C", 2, []string{"D", "FAILED"}},
		{"D", 1, []string{"FAILED"}},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			// Transition to the target state
			f.Reset()
			switch tt.state {
			case "B":
				f.Transition("B")
			case "C":
				f.Transition("B")
				f.Transition("C")
			case "D":
				f.Transition("B")
				f.Transition("C")
				f.Transition("D")
			}

			available := f.AvailableTransitions()
			if len(available) < tt.wantMin {
				t.Errorf("AvailableTransitions() at state %v returned %d transitions, want at least %d",
					tt.state, len(available), tt.wantMin)
			}

			// Check that expected states are present
			availableMap := make(map[string]bool)
			for _, state := range available {
				availableMap[state] = true
			}

			for _, expectedState := range tt.contains {
				if !availableMap[expectedState] {
					t.Errorf("AvailableTransitions() at state %v missing expected state %v",
						tt.state, expectedState)
				}
			}
		})
	}
}

func TestFSM_ConcurrentAccess(t *testing.T) {
	def := createTestDefinition()
	f, err := New(def, "test-instance", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Test concurrent reads and writes
	done := make(chan bool, 10)

	// Concurrent readers
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = f.CurrentState()
				_ = f.States()
				_ = f.IsFinalState()
				_ = f.AvailableTransitions()
			}
			done <- true
		}()
	}

	// Concurrent writers
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = f.Reset()
				_ = f.Transition("B")
				_ = f.Transition("FAILED")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// FSM should still be in a valid state
	currentState := f.CurrentState()
	validStates := map[string]bool{"A": true, "B": true, "FAILED": true}
	if !validStates[currentState] {
		t.Errorf("After concurrent access, CurrentState() = %v, want one of A, B, or FAILED", currentState)
	}
}
