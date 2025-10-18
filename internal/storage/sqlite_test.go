package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSQLiteStorage(t *testing.T) {
	tests := []struct {
		name    string
		dbPath  string
		wantErr bool
	}{
		{
			name:    "valid path",
			dbPath:  ":memory:",
			wantErr: false,
		},
		{
			name:    "empty path",
			dbPath:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewSQLiteStorage(tt.dbPath)
			if tt.wantErr {
				if err == nil {
					t.Error("NewSQLiteStorage() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NewSQLiteStorage() unexpected error = %v", err)
			}
			if store == nil {
				t.Error("NewSQLiteStorage() returned nil store")
			}
		})
	}
}

func TestSQLiteStorage_Initialize(t *testing.T) {
	t.Run("successful initialization", func(t *testing.T) {
		store, err := NewSQLiteStorage(":memory:")
		if err != nil {
			t.Fatalf("NewSQLiteStorage() error = %v", err)
		}

		ctx := context.Background()
		err = store.Initialize(ctx)
		if err != nil {
			t.Errorf("Initialize() error = %v", err)
		}

		defer store.Close()
	})

	t.Run("invalid database path", func(t *testing.T) {
		store, err := NewSQLiteStorage("/nonexistent/directory/db.sqlite")
		if err != nil {
			t.Fatalf("NewSQLiteStorage() error = %v", err)
		}

		ctx := context.Background()
		err = store.Initialize(ctx)
		if err == nil {
			store.Close()
			t.Error("Initialize() expected error for invalid path, got nil")
		}
	})
}

func setupTestStorage(t *testing.T) *SQLiteStorage {
	store, err := NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStorage() error = %v", err)
	}

	ctx := context.Background()
	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	return store
}

func TestSQLiteStorage_CreateInstance(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	tests := []struct {
		name     string
		instance *FSMInstance
		wantErr  bool
	}{
		{
			name: "valid instance",
			instance: &FSMInstance{
				ID:             "test-id-1",
				DefinitionName: "test_fsm",
				CurrentState:   "INITIAL",
			},
			wantErr: false,
		},
		{
			name:     "nil instance",
			instance: nil,
			wantErr:  true,
		},
		{
			name: "empty ID",
			instance: &FSMInstance{
				ID:             "",
				DefinitionName: "test_fsm",
				CurrentState:   "INITIAL",
			},
			wantErr: true,
		},
		{
			name: "duplicate ID",
			instance: &FSMInstance{
				ID:             "test-id-1",
				DefinitionName: "test_fsm",
				CurrentState:   "INITIAL",
			},
			wantErr: true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.CreateInstance(ctx, tt.instance)
			if tt.wantErr {
				if err == nil {
					t.Error("CreateInstance() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("CreateInstance() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestSQLiteStorage_GetInstance(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	ctx := context.Background()

	// Create test instance
	instance := &FSMInstance{
		ID:             "test-id",
		DefinitionName: "test_fsm",
		CurrentState:   "INITIAL",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := store.CreateInstance(ctx, instance)
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	t.Run("get existing instance", func(t *testing.T) {
		retrieved, err := store.GetInstance(ctx, "test-id")
		if err != nil {
			t.Errorf("GetInstance() error = %v", err)
			return
		}

		if retrieved.ID != instance.ID {
			t.Errorf("GetInstance() ID = %v, want %v", retrieved.ID, instance.ID)
		}
		if retrieved.DefinitionName != instance.DefinitionName {
			t.Errorf("GetInstance() DefinitionName = %v, want %v", retrieved.DefinitionName, instance.DefinitionName)
		}
		if retrieved.CurrentState != instance.CurrentState {
			t.Errorf("GetInstance() CurrentState = %v, want %v", retrieved.CurrentState, instance.CurrentState)
		}
	})

	t.Run("get non-existent instance", func(t *testing.T) {
		_, err := store.GetInstance(ctx, "non-existent")
		if err == nil {
			t.Error("GetInstance() expected error for non-existent ID, got nil")
		}
	})
}

func TestSQLiteStorage_UpdateState(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	ctx := context.Background()

	// Create test instance
	instance := &FSMInstance{
		ID:             "test-id",
		DefinitionName: "test_fsm",
		CurrentState:   "INITIAL",
	}

	err := store.CreateInstance(ctx, instance)
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	tests := []struct {
		name      string
		id        string
		fromState string
		toState   string
		wantErr   bool
	}{
		{
			name:      "valid transition",
			id:        "test-id",
			fromState: "INITIAL",
			toState:   "RUNNING",
			wantErr:   false,
		},
		{
			name:      "wrong from state",
			id:        "test-id",
			fromState: "WRONG",
			toState:   "STOPPED",
			wantErr:   true,
		},
		{
			name:      "non-existent instance",
			id:        "non-existent",
			fromState: "INITIAL",
			toState:   "RUNNING",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.UpdateState(ctx, tt.id, tt.fromState, tt.toState)
			if tt.wantErr {
				if err == nil {
					t.Error("UpdateState() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("UpdateState() unexpected error = %v", err)
					return
				}

				// Verify state was updated
				retrieved, err := store.GetInstance(ctx, tt.id)
				if err != nil {
					t.Errorf("GetInstance() error = %v", err)
					return
				}

				if retrieved.CurrentState != tt.toState {
					t.Errorf("UpdateState() current state = %v, want %v", retrieved.CurrentState, tt.toState)
				}
			}
		})
	}
}

func TestSQLiteStorage_DeleteInstance(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	ctx := context.Background()

	// Create test instance
	instance := &FSMInstance{
		ID:             "test-id",
		DefinitionName: "test_fsm",
		CurrentState:   "INITIAL",
	}

	err := store.CreateInstance(ctx, instance)
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	t.Run("delete existing instance", func(t *testing.T) {
		err := store.DeleteInstance(ctx, "test-id")
		if err != nil {
			t.Errorf("DeleteInstance() error = %v", err)
		}

		// Verify instance was deleted
		_, err = store.GetInstance(ctx, "test-id")
		if err == nil {
			t.Error("GetInstance() expected error after delete, got nil")
		}
	})

	t.Run("delete non-existent instance", func(t *testing.T) {
		err := store.DeleteInstance(ctx, "non-existent")
		if err == nil {
			t.Error("DeleteInstance() expected error for non-existent ID, got nil")
		}
	})
}

func TestSQLiteStorage_ListInstances(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	ctx := context.Background()

	// Create test instances
	instances := []*FSMInstance{
		{
			ID:             "test-id-1",
			DefinitionName: "test_fsm",
			CurrentState:   "INITIAL",
		},
		{
			ID:             "test-id-2",
			DefinitionName: "test_fsm",
			CurrentState:   "RUNNING",
		},
	}

	for _, instance := range instances {
		err := store.CreateInstance(ctx, instance)
		if err != nil {
			t.Fatalf("CreateInstance() error = %v", err)
		}
	}

	t.Run("list all instances", func(t *testing.T) {
		retrieved, err := store.ListInstances(ctx)
		if err != nil {
			t.Errorf("ListInstances() error = %v", err)
			return
		}

		if len(retrieved) != 2 {
			t.Errorf("ListInstances() returned %d instances, want 2", len(retrieved))
		}
	})
}

func TestSQLiteStorage_GetTransitionHistory(t *testing.T) {
	store := setupTestStorage(t)
	defer store.Close()

	ctx := context.Background()

	// Create test instance
	instance := &FSMInstance{
		ID:             "test-id",
		DefinitionName: "test_fsm",
		CurrentState:   "INITIAL",
	}

	err := store.CreateInstance(ctx, instance)
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	// Perform some transitions
	transitions := []struct {
		from string
		to   string
	}{
		{"INITIAL", "RUNNING"},
		{"RUNNING", "STOPPED"},
		{"STOPPED", "RUNNING"},
	}

	for _, tr := range transitions {
		err := store.UpdateState(ctx, "test-id", tr.from, tr.to)
		if err != nil {
			t.Fatalf("UpdateState() error = %v", err)
		}
	}

	t.Run("get transition history", func(t *testing.T) {
		history, err := store.GetTransitionHistory(ctx, "test-id", 10)
		if err != nil {
			t.Errorf("GetTransitionHistory() error = %v", err)
			return
		}

		if len(history) != 3 {
			t.Errorf("GetTransitionHistory() returned %d transitions, want 3", len(history))
		}

		// Verify transitions are in reverse chronological order
		if len(history) > 0 {
			if history[0].FromState != "STOPPED" || history[0].ToState != "RUNNING" {
				t.Errorf("GetTransitionHistory() first transition = %s -> %s, want STOPPED -> RUNNING",
					history[0].FromState, history[0].ToState)
			}
		}
	})

	t.Run("get history with limit", func(t *testing.T) {
		history, err := store.GetTransitionHistory(ctx, "test-id", 2)
		if err != nil {
			t.Errorf("GetTransitionHistory() error = %v", err)
			return
		}

		if len(history) != 2 {
			t.Errorf("GetTransitionHistory() with limit 2 returned %d transitions, want 2", len(history))
		}
	})
}

func TestSQLiteStorage_PersistenceToFile(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create and initialize storage
	store, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStorage() error = %v", err)
	}

	ctx := context.Background()
	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Create instance
	instance := &FSMInstance{
		ID:             "test-id",
		DefinitionName: "test_fsm",
		CurrentState:   "INITIAL",
	}

	err = store.CreateInstance(ctx, instance)
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	// Close storage
	store.Close()

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Reopen and verify data persisted
	store2, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStorage() error = %v", err)
	}

	if err := store2.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer store2.Close()

	retrieved, err := store2.GetInstance(ctx, "test-id")
	if err != nil {
		t.Errorf("GetInstance() error after reopening = %v", err)
	}

	if retrieved.ID != "test-id" {
		t.Errorf("Retrieved instance ID = %v, want test-id", retrieved.ID)
	}
}
