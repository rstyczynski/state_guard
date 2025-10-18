package fsm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rstyczynski/fsm_v2/internal/storage"
)

// WebhookNotifier defines the interface for webhook notifications
// This allows the FSM to notify webhooks without depending on the webhook package
type WebhookNotifier interface {
	Notify(instanceID, assetType, fromState, toState string)
}

// FSM represents a finite state machine instance
type FSM struct {
	id             string
	definition     *Definition
	currentState   string
	transitionMap  map[string]map[string]bool // from -> to -> allowed
	wildcardStates map[string]bool            // states reachable via wildcard
	storage        storage.Storage            // Optional persistence backend
	webhook        WebhookNotifier            // Optional webhook notifier
	assetTypeName  string                     // Asset type name for webhook context
	mu             sync.RWMutex
}

// New creates a new FSM instance from a definition
// ID is required and must be provided by the caller
// Storage is optional - pass nil for in-memory only
func New(def *Definition, id string, store storage.Storage) (*FSM, error) {
	return NewWithWebhook(def, id, "", store, nil)
}

// NewWithWebhook creates a new FSM instance with webhook support
// assetTypeName is used for webhook context
func NewWithWebhook(def *Definition, id string, assetTypeName string, store storage.Storage, webhook WebhookNotifier) (*FSM, error) {
	if def == nil {
		return nil, fmt.Errorf("definition cannot be nil")
	}

	// ID is mandatory
	if id == "" {
		return nil, fmt.Errorf("instance ID is required")
	}

	// Validate user-provided ID
	if err := validateID(id); err != nil {
		return nil, err
	}

	fsm := &FSM{
		id:             id,
		definition:     def,
		currentState:   def.Initial,
		transitionMap:  make(map[string]map[string]bool),
		wildcardStates: make(map[string]bool),
		storage:        store,
		webhook:        webhook,
		assetTypeName:  assetTypeName,
	}

	// Build transition map for O(1) lookups
	for _, t := range def.Transitions {
		if t.From == "*" {
			fsm.wildcardStates[t.To] = true
		} else {
			if fsm.transitionMap[t.From] == nil {
				fsm.transitionMap[t.From] = make(map[string]bool)
			}
			fsm.transitionMap[t.From][t.To] = true
		}
	}

	// Persist initial instance if storage is enabled
	if fsm.storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		instance := &storage.FSMInstance{
			ID:             fsm.id,
			DefinitionName: def.Name,
			AssetTypeName:  assetTypeName,
			CurrentState:   fsm.currentState,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if err := fsm.storage.CreateInstance(ctx, instance); err != nil {
			return nil, fmt.Errorf("failed to persist FSM instance: %w", err)
		}
	}

	return fsm, nil
}

// validateID checks if the provided ID is valid
func validateID(id string) error {
	if len(id) == 0 {
		return fmt.Errorf("ID cannot be empty")
	}
	if len(id) > 255 {
		return fmt.Errorf("ID too long (max 255 characters)")
	}
	// Allow alphanumeric, hyphens, underscores, and periods
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return fmt.Errorf("ID contains invalid character '%c' (allowed: a-z, A-Z, 0-9, -, _, .)", r)
		}
	}
	return nil
}

// NewFromFile creates a new FSM instance from a YAML file
// Deprecated: Use New() with explicit ID instead
func NewFromFile(filepath string) (*FSM, error) {
	return nil, fmt.Errorf("NewFromFile is deprecated: use New(def, id, store) with explicit ID")
}

// LoadFromStorage loads an existing FSM instance from storage
func LoadFromStorage(ctx context.Context, instanceID string, def *Definition, store storage.Storage) (*FSM, error) {
	return LoadFromStorageWithWebhook(ctx, instanceID, def, "", store, nil)
}

// LoadFromStorageWithWebhook loads an existing FSM instance from storage with webhook support
func LoadFromStorageWithWebhook(ctx context.Context, instanceID string, def *Definition, assetTypeName string, store storage.Storage, webhook WebhookNotifier) (*FSM, error) {
	if store == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}

	if def == nil {
		return nil, fmt.Errorf("definition cannot be nil")
	}

	// Load instance from storage
	instance, err := store.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load instance: %w", err)
	}

	// Verify definition name matches
	if instance.DefinitionName != def.Name {
		return nil, fmt.Errorf("definition mismatch: expected %s, got %s", instance.DefinitionName, def.Name)
	}

	// Create FSM with loaded state
	fsm := &FSM{
		id:             instance.ID,
		definition:     def,
		currentState:   instance.CurrentState,
		transitionMap:  make(map[string]map[string]bool),
		wildcardStates: make(map[string]bool),
		storage:        store,
		webhook:        webhook,
		assetTypeName:  assetTypeName,
	}

	// Build transition map for O(1) lookups
	for _, t := range def.Transitions {
		if t.From == "*" {
			fsm.wildcardStates[t.To] = true
		} else {
			if fsm.transitionMap[t.From] == nil {
				fsm.transitionMap[t.From] = make(map[string]bool)
			}
			fsm.transitionMap[t.From][t.To] = true
		}
	}

	return fsm, nil
}

// CurrentState returns the current state of the FSM
func (f *FSM) CurrentState() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.currentState
}

// CanTransition checks if a transition to the target state is allowed
func (f *FSM) CanTransition(to string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check if target state exists
	stateExists := false
	for _, state := range f.definition.States {
		if state == to {
			stateExists = true
			break
		}
	}
	if !stateExists {
		return false
	}

	// Check explicit transition
	if f.transitionMap[f.currentState] != nil && f.transitionMap[f.currentState][to] {
		return true
	}

	// Check wildcard transition
	if f.wildcardStates[to] {
		return true
	}

	return false
}

// Transition attempts to transition to a new state
// If persistence is enabled, state is persisted first - if persistence fails,
// in-memory state is NOT changed (transactional guarantee)
func (f *FSM) Transition(to string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if target state exists
	stateExists := false
	for _, state := range f.definition.States {
		if state == to {
			stateExists = true
			break
		}
	}
	if !stateExists {
		return &UnknownStateError{State: to}
	}

	// Check if transition is allowed
	allowed := false

	// Check explicit transition
	if f.transitionMap[f.currentState] != nil && f.transitionMap[f.currentState][to] {
		allowed = true
	}

	// Check wildcard transition
	if f.wildcardStates[to] {
		allowed = true
	}

	if !allowed {
		return &InvalidTransitionError{From: f.currentState, To: to}
	}

	// Persist transition BEFORE changing in-memory state
	// If persistence fails, in-memory state remains unchanged
	if f.storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fromState := f.currentState
		if err := f.storage.UpdateState(ctx, f.id, fromState, to); err != nil {
			return fmt.Errorf("persistence failed: %w", err)
		}
	}

	// Save fromState for webhook notification
	fromState := f.currentState

	// Only update in-memory state after successful persistence (or if no storage)
	f.currentState = to

	// Notify webhooks (non-blocking, happens in background)
	if f.webhook != nil {
		f.webhook.Notify(f.id, f.assetTypeName, fromState, to)
	}

	return nil
}

// States returns all available states
func (f *FSM) States() []string {
	return f.definition.States
}

// InitialState returns the initial state of the FSM
func (f *FSM) InitialState() string {
	return f.definition.Initial
}

// FinalStates returns the final states of the FSM
func (f *FSM) FinalStates() []string {
	return f.definition.Final
}

// IsFinalState checks if the current state is a final state
func (f *FSM) IsFinalState() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, finalState := range f.definition.Final {
		if f.currentState == finalState {
			return true
		}
	}
	return false
}

// Name returns the FSM definition name
func (f *FSM) Name() string {
	return f.definition.Name
}

// ID returns the unique identifier of the FSM instance
func (f *FSM) ID() string {
	return f.id
}

// Reset resets the FSM to its initial state
// If persistence is enabled, state is persisted first
func (f *FSM) Reset() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	initialState := f.definition.Initial
	if f.currentState == initialState {
		return nil // Already at initial state
	}

	// Persist reset if storage is enabled
	if f.storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fromState := f.currentState
		if err := f.storage.UpdateState(ctx, f.id, fromState, initialState); err != nil {
			return fmt.Errorf("persistence failed during reset: %w", err)
		}
	}

	// Save fromState for webhook notification
	fromState := f.currentState

	f.currentState = initialState

	// Notify webhooks (non-blocking, happens in background)
	if f.webhook != nil {
		f.webhook.Notify(f.id, f.assetTypeName, fromState, initialState)
	}

	return nil
}

// AvailableTransitions returns all states that can be transitioned to from the current state
func (f *FSM) AvailableTransitions() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	available := make([]string, 0)
	seen := make(map[string]bool)

	// Add explicit transitions
	if f.transitionMap[f.currentState] != nil {
		for to := range f.transitionMap[f.currentState] {
			if !seen[to] {
				available = append(available, to)
				seen[to] = true
			}
		}
	}

	// Add wildcard transitions
	for to := range f.wildcardStates {
		if !seen[to] {
			available = append(available, to)
			seen[to] = true
		}
	}

	return available
}
