package api

import (
	"fmt"
	"net/http"
	"time"
)

// CreateAssetRequest represents a request to create a new asset instance
type CreateAssetRequest struct {
	AssetType  string `json:"asset_type"`  // Path to asset type YAML file
	InstanceID string `json:"instance_id"` // Unique identifier for the asset
}

// Bind implements render.Binder interface for request validation
func (c *CreateAssetRequest) Bind(r *http.Request) error {
	if c.AssetType == "" {
		return fmt.Errorf("asset_type is required")
	}
	if c.InstanceID == "" {
		return fmt.Errorf("instance_id is required")
	}
	return nil
}

// AssetResponse represents an asset instance in API responses
type AssetResponse struct {
	ID                   string                 `json:"id"`
	AssetType            string                 `json:"asset_type"`
	DefinitionName       string                 `json:"definition_name"`
	CurrentState         string                 `json:"current_state"`
	AvailableTransitions []string               `json:"available_transitions"`
	IsFinalState         bool                   `json:"is_final_state"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	FSMDefinition        *FSMDefinitionResponse `json:"fsm_definition,omitempty"` // Include FSM definition for visualization
}

// FSMDefinitionResponse represents an FSM definition in API responses
type FSMDefinitionResponse struct {
	Name    string   `json:"name"`
	Initial string   `json:"initial"`
	Final   []string `json:"final"`
	States  []string `json:"states"`
	Transitions []FSMTransitionResponse `json:"transitions"`
}

// FSMTransitionResponse represents a transition in FSM definition
type FSMTransitionResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// TransitionRequest represents a state transition request
type TransitionRequest struct {
	ToState string `json:"to_state"`
}

// Bind implements render.Binder interface for request validation
func (t *TransitionRequest) Bind(r *http.Request) error {
	if t.ToState == "" {
		return fmt.Errorf("to_state is required")
	}
	return nil
}

// TransitionResponse represents the result of a state transition
type TransitionResponse struct {
	FromState string    `json:"from_state"`
	ToState   string    `json:"to_state"`
	Timestamp time.Time `json:"timestamp"`
}

// HistoryResponse represents state transition history
type HistoryResponse struct {
	InstanceID  string              `json:"instance_id"`
	Transitions []TransitionHistory `json:"transitions"`
}

// TransitionHistory represents a single state transition in history
type TransitionHistory struct {
	FromState      string    `json:"from_state"`
	ToState        string    `json:"to_state"`
	TransitionedAt time.Time `json:"transitioned_at"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// ListAssetsResponse represents a list of assets
type ListAssetsResponse struct {
	Assets []AssetResponse `json:"assets"`
	Count  int             `json:"count"`
}
