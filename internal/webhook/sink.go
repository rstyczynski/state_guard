package webhook

import (
	"context"
	"time"
)

// Event represents a state transition event
type Event struct {
	InstanceID string            // Asset instance ID
	AssetType  string            // Asset type name
	FromState  string            // Previous state
	ToState    string            // New state
	Timestamp  time.Time         // When the transition occurred
	Metadata   map[string]string // Additional metadata
}

// Sink defines the interface for webhook notification sinks
// This interface allows for extensibility - different implementations
// can send notifications via HTTP, message queues, files, etc.
type Sink interface {
	// Send sends a webhook notification for the given event
	// Returns error if the notification fails
	Send(ctx context.Context, event *Event) error

	// Name returns the name/type of this sink (e.g., "http", "kafka")
	Name() string

	// Close cleans up any resources used by the sink
	Close() error
}
