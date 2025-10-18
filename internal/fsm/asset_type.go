package fsm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// WebhookCondition represents the type of event that triggers a webhook
type WebhookCondition string

const (
	// OnEnter triggers when entering a specific state
	OnEnter WebhookCondition = "on_enter"
	// OnExit triggers when exiting a specific state
	OnExit WebhookCondition = "on_exit"
	// OnTransition triggers on a specific state transition
	OnTransition WebhookCondition = "on_transition"
)

// WebhookConfig represents a webhook configuration
type WebhookConfig struct {
	Condition WebhookCondition `yaml:"condition"`
	State     string           `yaml:"state,omitempty"`     // For on_enter/on_exit
	From      string           `yaml:"from,omitempty"`      // For on_transition
	To        string           `yaml:"to,omitempty"`        // For on_transition
	URL       string           `yaml:"url"`
	Method    string           `yaml:"method"`              // HTTP method (GET, POST, PUT, etc.)
	Timeout   string           `yaml:"timeout"`             // e.g., "5s", "10s"
	Headers   map[string]string `yaml:"headers,omitempty"` // Optional HTTP headers
}

// AssetType represents a type of asset with its state machine and webhooks
type AssetType struct {
	Version          int              `yaml:"version"`
	Name             string           `yaml:"name"`
	StateMachine     string           `yaml:"state_machine"`     // Path to FSM definition file
	StateMachineRef  *Definition      `yaml:"-"`                 // Loaded FSM definition
	Webhooks         []WebhookConfig  `yaml:"webhooks,omitempty"`
	baseDir          string           `yaml:"-"`                 // Base directory for resolving relative paths
}

// LoadAssetType loads and parses an asset type definition from a YAML file
func LoadAssetType(assetFilePath string) (*AssetType, error) {
	data, err := os.ReadFile(assetFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read asset type file: %w", err)
	}

	var assetType AssetType
	if err := yaml.Unmarshal(data, &assetType); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := assetType.Validate(); err != nil {
		return nil, fmt.Errorf("invalid asset type: %w", err)
	}

	// Store the directory of the asset type file for resolving relative paths
	absPath, err := filepath.Abs(assetFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve asset type path: %w", err)
	}
	assetType.baseDir = filepath.Dir(absPath)

	return &assetType, nil
}

// Validate checks if the asset type definition is valid
func (at *AssetType) Validate() error {
	if at.Version != 1 {
		return fmt.Errorf("unsupported version: %d (expected 1)", at.Version)
	}

	if at.Name == "" {
		return fmt.Errorf("name is required")
	}

	if at.StateMachine == "" {
		return fmt.Errorf("state_machine is required")
	}

	// Validate webhooks
	for i, webhook := range at.Webhooks {
		if err := webhook.Validate(); err != nil {
			return fmt.Errorf("webhook %d: %w", i, err)
		}
	}

	return nil
}

// LoadStateMachine loads the FSM definition referenced by this asset type
func (at *AssetType) LoadStateMachine() error {
	// Resolve the state machine path relative to the asset type file's directory
	fsmPath := at.StateMachine
	if !filepath.IsAbs(fsmPath) && at.baseDir != "" {
		fsmPath = filepath.Join(at.baseDir, fsmPath)
	}

	def, err := LoadDefinition(fsmPath)
	if err != nil {
		return fmt.Errorf("failed to load state machine '%s': %w", at.StateMachine, err)
	}

	at.StateMachineRef = def
	return nil
}

// Validate checks if a webhook configuration is valid
func (wc *WebhookConfig) Validate() error {
	// Validate condition
	switch wc.Condition {
	case OnEnter, OnExit:
		if wc.State == "" {
			return fmt.Errorf("state is required for %s condition", wc.Condition)
		}
		if wc.From != "" || wc.To != "" {
			return fmt.Errorf("from/to should not be specified for %s condition", wc.Condition)
		}
	case OnTransition:
		if wc.From == "" || wc.To == "" {
			return fmt.Errorf("from and to are required for %s condition", wc.Condition)
		}
		if wc.State != "" {
			return fmt.Errorf("state should not be specified for %s condition", wc.Condition)
		}
	default:
		return fmt.Errorf("unknown condition: %s (expected on_enter, on_exit, or on_transition)", wc.Condition)
	}

	// Validate URL
	if wc.URL == "" {
		return fmt.Errorf("url is required")
	}

	// Validate and set default method
	if wc.Method == "" {
		wc.Method = "POST" // Default to POST
	}

	// Validate timeout
	if wc.Timeout == "" {
		wc.Timeout = "5s" // Default to 5 seconds
	}
	if _, err := time.ParseDuration(wc.Timeout); err != nil {
		return fmt.Errorf("invalid timeout '%s': %w", wc.Timeout, err)
	}

	return nil
}

// GetTimeout returns the parsed timeout duration
func (wc *WebhookConfig) GetTimeout() time.Duration {
	duration, _ := time.ParseDuration(wc.Timeout)
	return duration
}

// Matches checks if this webhook should be triggered for the given event
func (wc *WebhookConfig) Matches(condition WebhookCondition, fromState, toState string) bool {
	if wc.Condition != condition {
		return false
	}

	switch condition {
	case OnEnter:
		return wc.State == toState
	case OnExit:
		return wc.State == fromState
	case OnTransition:
		return wc.From == fromState && wc.To == toState
	}

	return false
}
