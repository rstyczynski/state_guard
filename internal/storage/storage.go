package storage

import (
	"context"
	"time"
)

// FSMInstance represents a stored FSM instance
type FSMInstance struct {
	ID             string
	DefinitionName string
	AssetTypeName  string    // Asset type file path (e.g., "examples/web_server_asset_type.yaml") for reloading
	CurrentState   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// StateTransition represents a state change event (prepared for Phase 2.1 History)
type StateTransition struct {
	ID           int64
	InstanceID   string
	FromState    string
	ToState      string
	TransitionedAt time.Time
}

// Storage defines the interface for FSM persistence
type Storage interface {
	// Initialize sets up the storage backend
	Initialize(ctx context.Context) error

	// Close closes the storage backend
	Close() error

	// CreateInstance creates a new FSM instance in storage
	CreateInstance(ctx context.Context, instance *FSMInstance) error

	// GetInstance retrieves an FSM instance by ID
	GetInstance(ctx context.Context, id string) (*FSMInstance, error)

	// UpdateState atomically updates the state of an FSM instance
	// Returns error if persistence fails - caller should not change in-memory state
	UpdateState(ctx context.Context, id string, fromState, toState string) error

	// DeleteInstance removes an FSM instance from storage
	DeleteInstance(ctx context.Context, id string) error

	// ListInstances returns all FSM instances
	ListInstances(ctx context.Context) ([]*FSMInstance, error)

	// GetTransitionHistory returns state transitions for an instance (Phase 2.1)
	GetTransitionHistory(ctx context.Context, instanceID string, limit int) ([]*StateTransition, error)
}
