package fsm

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Definition represents the YAML FSM configuration
type Definition struct {
	Version     int          `yaml:"version"`
	Name        string       `yaml:"name"`
	Initial     string       `yaml:"initial"`
	Final       []string     `yaml:"final"`
	States      []string     `yaml:"states"`
	Transitions []Transition `yaml:"transitions"`
}

// Transition represents a state transition rule
type Transition struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// LoadDefinition loads and parses an FSM definition from a YAML file
func LoadDefinition(filepath string) (*Definition, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read definition file: %w", err)
	}

	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := def.Validate(); err != nil {
		return nil, fmt.Errorf("invalid definition: %w", err)
	}

	return &def, nil
}

// Validate checks if the FSM definition is valid
func (d *Definition) Validate() error {
	if d.Version != 1 {
		return fmt.Errorf("unsupported version: %d (expected 1)", d.Version)
	}

	if d.Name == "" {
		return fmt.Errorf("name is required")
	}

	if d.Initial == "" {
		return fmt.Errorf("initial state is required")
	}

	if len(d.States) == 0 {
		return fmt.Errorf("at least one state is required")
	}

	if len(d.Transitions) == 0 {
		return fmt.Errorf("at least one transition is required")
	}

	// Build state map for validation
	stateMap := make(map[string]bool)
	for _, state := range d.States {
		if state == "" {
			return fmt.Errorf("empty state name not allowed")
		}
		if stateMap[state] {
			return fmt.Errorf("duplicate state: %s", state)
		}
		stateMap[state] = true
	}

	// Validate initial state exists
	if !stateMap[d.Initial] {
		return fmt.Errorf("initial state '%s' not in states list", d.Initial)
	}

	// Validate final states exist
	for _, finalState := range d.Final {
		if !stateMap[finalState] {
			return fmt.Errorf("final state '%s' not in states list", finalState)
		}
	}

	// Validate transitions
	for i, t := range d.Transitions {
		if t.From == "" || t.To == "" {
			return fmt.Errorf("transition %d: from and to are required", i)
		}

		// Wildcard "*" is allowed for from
		if t.From != "*" && !stateMap[t.From] {
			return fmt.Errorf("transition %d: from state '%s' not in states list", i, t.From)
		}

		if !stateMap[t.To] {
			return fmt.Errorf("transition %d: to state '%s' not in states list", i, t.To)
		}
	}

	return nil
}
