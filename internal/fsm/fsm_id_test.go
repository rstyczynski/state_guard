package fsm

import (
	"context"
	"testing"
	"time"
)

func TestNewWithID(t *testing.T) {
	def := createTestDefinition()

	tests := []struct {
		name    string
		id      string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid custom ID",
			id:      "server1",
			wantErr: false,
		},
		{
			name:    "valid ID with hyphens",
			id:      "my-asset-123",
			wantErr: false,
		},
		{
			name:    "valid ID with underscores",
			id:      "asset_prod_01",
			wantErr: false,
		},
		{
			name:    "valid ID with dots",
			id:      "server.prod.01",
			wantErr: false,
		},
		{
			name:    "empty ID is required",
			id:      "",
			wantErr: true,
			errMsg:  "required",
		},
		{
			name:    "ID with spaces",
			id:      "server 1",
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name:    "ID with special chars",
			id:      "server@1",
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name:    "ID too long",
			id:      string(make([]byte, 256)),
			wantErr: true,
			errMsg:  "too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm, err := New(def, tt.id, nil)
			if tt.wantErr {
				if err == nil {
					t.Errorf("New() expected error containing '%s', got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("New() error = '%v', want error containing '%s'", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("New() unexpected error = %v", err)
					return
				}
				if fsm == nil {
					t.Error("New() returned nil FSM")
					return
				}

				// Verify ID
				if fsm.ID() != tt.id {
					t.Errorf("New() ID = %v, want %v", fsm.ID(), tt.id)
				}
			}
		})
	}
}

func TestNewWithID_UniqueIDs(t *testing.T) {
	def := createTestDefinition()

	// Create multiple FSMs with same definition but different IDs
	fsm1, err := New(def, "asset1", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fsm2, err := New(def, "asset2", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if fsm1.ID() == fsm2.ID() {
		t.Error("New() should create unique IDs for different instances")
	}

	if fsm1.ID() != "asset1" {
		t.Errorf("FSM 1 ID = %v, want asset1", fsm1.ID())
	}

	if fsm2.ID() != "asset2" {
		t.Errorf("FSM 2 ID = %v, want asset2", fsm2.ID())
	}
}

func TestNewWithID_WithStorage(t *testing.T) {
	def := createTestDefinition()
	store := setupTestStorageForFSM(t)
	defer store.Close()

	t.Run("create with custom ID", func(t *testing.T) {
		fsm, err := New(def, "my-server-01", store)
		if err != nil {
			t.Errorf("New() error = %v", err)
			return
		}

		if fsm.ID() != "my-server-01" {
			t.Errorf("FSM ID = %v, want my-server-01", fsm.ID())
		}

		// Verify persisted with correct ID
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		instance, err := store.GetInstance(ctx, "my-server-01")
		if err != nil {
			t.Errorf("GetInstance() error = %v", err)
			return
		}

		if instance.ID != "my-server-01" {
			t.Errorf("Persisted ID = %v, want my-server-01", instance.ID)
		}
	})

	t.Run("duplicate ID fails", func(t *testing.T) {
		// First instance should succeed
		_, err := New(def, "duplicate-id", store)
		if err != nil {
			t.Errorf("First New() unexpected error = %v", err)
		}

		// Second instance with same ID should fail
		_, err = New(def, "duplicate-id", store)
		if err == nil {
			t.Error("New() with duplicate ID should fail")
		}
	})
}
