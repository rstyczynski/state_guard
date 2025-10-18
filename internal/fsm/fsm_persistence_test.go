package fsm

import (
	"context"
	"testing"

	"github.com/rstyczynski/fsm_v2/internal/storage"
)

func setupTestStorageForFSM(t *testing.T) storage.Storage {
	store, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStorage() error = %v", err)
	}

	ctx := context.Background()
	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	return store
}

func TestNewWithStorage(t *testing.T) {
	def := createTestDefinition()
	store := setupTestStorageForFSM(t)
	defer store.Close()

	t.Run("create with storage", func(t *testing.T) {
		f, err := New(def, "test-instance", store)
		if err != nil {
			t.Errorf("New() with storage error = %v", err)
			return
		}

		if f == nil {
			t.Error("New() with storage returned nil FSM")
			return
		}

		// Verify instance was persisted
		ctx := context.Background()
		instance, err := store.GetInstance(ctx, f.ID())
		if err != nil {
			t.Errorf("GetInstance() error = %v", err)
			return
		}

		if instance.CurrentState != def.Initial {
			t.Errorf("Persisted state = %v, want %v", instance.CurrentState, def.Initial)
		}
	})

	t.Run("create without storage (nil)", func(t *testing.T) {
		f, err := New(def, "test-instance-2", nil)
		if err != nil {
			t.Errorf("New() with nil storage error = %v", err)
			return
		}

		if f == nil {
			t.Error("New() with nil storage returned nil FSM")
		}
	})
}

func TestFSM_TransitionWithPersistence(t *testing.T) {
	def := createTestDefinition()
	store := setupTestStorageForFSM(t)
	defer store.Close()

	f, err := New(def, "test-instance", store)
	if err != nil {
		t.Fatalf("NewWithStorage() error = %v", err)
	}

	t.Run("successful transition persists state", func(t *testing.T) {
		err := f.Transition("B")
		if err != nil {
			t.Errorf("Transition() error = %v", err)
			return
		}

		// Verify state was persisted
		ctx := context.Background()
		instance, err := store.GetInstance(ctx, f.ID())
		if err != nil {
			t.Errorf("GetInstance() error = %v", err)
			return
		}

		if instance.CurrentState != "B" {
			t.Errorf("Persisted state = %v, want B", instance.CurrentState)
		}

		if f.CurrentState() != "B" {
			t.Errorf("In-memory state = %v, want B", f.CurrentState())
		}
	})

	t.Run("invalid transition does not change state", func(t *testing.T) {
		currentState := f.CurrentState()

		err := f.Transition("D") // Invalid transition from B to D
		if err == nil {
			t.Error("Transition() expected error for invalid transition, got nil")
			return
		}

		// Verify in-memory state unchanged
		if f.CurrentState() != currentState {
			t.Errorf("In-memory state changed to %v, should remain %v", f.CurrentState(), currentState)
		}

		// Verify persisted state unchanged
		ctx := context.Background()
		instance, err := store.GetInstance(ctx, f.ID())
		if err != nil {
			t.Errorf("GetInstance() error = %v", err)
			return
		}

		if instance.CurrentState != currentState {
			t.Errorf("Persisted state changed to %v, should remain %v", instance.CurrentState, currentState)
		}
	})

	t.Run("transition history is recorded", func(t *testing.T) {
		// Perform a valid transition
		err := f.Transition("C")
		if err != nil {
			t.Errorf("Transition() error = %v", err)
			return
		}

		// Check transition history
		ctx := context.Background()
		history, err := store.GetTransitionHistory(ctx, f.ID(), 10)
		if err != nil {
			t.Errorf("GetTransitionHistory() error = %v", err)
			return
		}

		// Should have at least 2 transitions (B->C and A->B)
		if len(history) < 2 {
			t.Errorf("GetTransitionHistory() returned %d transitions, want at least 2", len(history))
		}
	})
}

func TestFSM_ResetWithPersistence(t *testing.T) {
	def := createTestDefinition()
	store := setupTestStorageForFSM(t)
	defer store.Close()

	f, err := New(def, "test-instance", store)
	if err != nil {
		t.Fatalf("NewWithStorage() error = %v", err)
	}

	// Transition to a different state
	err = f.Transition("B")
	if err != nil {
		t.Fatalf("Transition() error = %v", err)
	}

	t.Run("reset persists state change", func(t *testing.T) {
		err := f.Reset()
		if err != nil {
			t.Errorf("Reset() error = %v", err)
			return
		}

		if f.CurrentState() != "A" {
			t.Errorf("In-memory state after Reset() = %v, want A", f.CurrentState())
		}

		// Verify state was persisted
		ctx := context.Background()
		instance, err := store.GetInstance(ctx, f.ID())
		if err != nil {
			t.Errorf("GetInstance() error = %v", err)
			return
		}

		if instance.CurrentState != "A" {
			t.Errorf("Persisted state after Reset() = %v, want A", instance.CurrentState)
		}
	})
}

func TestLoadFromStorage(t *testing.T) {
	def := createTestDefinition()
	store := setupTestStorageForFSM(t)
	defer store.Close()

	// Create initial FSM and perform some transitions
	f1, err := New(def, "test-instance", store)
	if err != nil {
		t.Fatalf("NewWithStorage() error = %v", err)
	}

	instanceID := f1.ID()

	err = f1.Transition("B")
	if err != nil {
		t.Fatalf("Transition() error = %v", err)
	}

	err = f1.Transition("C")
	if err != nil {
		t.Fatalf("Transition() error = %v", err)
	}

	t.Run("load existing instance", func(t *testing.T) {
		ctx := context.Background()
		f2, err := LoadFromStorage(ctx, instanceID, def, store)
		if err != nil {
			t.Errorf("LoadFromStorage() error = %v", err)
			return
		}

		// Verify loaded FSM has correct state
		if f2.ID() != instanceID {
			t.Errorf("Loaded FSM ID = %v, want %v", f2.ID(), instanceID)
		}

		if f2.CurrentState() != "C" {
			t.Errorf("Loaded FSM state = %v, want C", f2.CurrentState())
		}

		// Verify can continue transitions
		err = f2.Transition("D")
		if err != nil {
			t.Errorf("Transition() on loaded FSM error = %v", err)
			return
		}

		if f2.CurrentState() != "D" {
			t.Errorf("State after transition = %v, want D", f2.CurrentState())
		}
	})

	t.Run("load non-existent instance", func(t *testing.T) {
		ctx := context.Background()
		_, err := LoadFromStorage(ctx, "non-existent", def, store)
		if err == nil {
			t.Error("LoadFromStorage() expected error for non-existent ID, got nil")
		}
	})

	t.Run("load with nil storage", func(t *testing.T) {
		ctx := context.Background()
		_, err := LoadFromStorage(ctx, instanceID, def, nil)
		if err == nil {
			t.Error("LoadFromStorage() expected error for nil storage, got nil")
		}
	})

	t.Run("load with nil definition", func(t *testing.T) {
		ctx := context.Background()
		_, err := LoadFromStorage(ctx, instanceID, nil, store)
		if err == nil {
			t.Error("LoadFromStorage() expected error for nil definition, got nil")
		}
	})
}

func TestFSM_PersistenceTransactional(t *testing.T) {
	def := createTestDefinition()
	store := setupTestStorageForFSM(t)
	defer store.Close()

	f, err := New(def, "test-instance", store)
	if err != nil {
		t.Fatalf("NewWithStorage() error = %v", err)
	}

	initialState := f.CurrentState()

	t.Run("failed persistence does not change in-memory state", func(t *testing.T) {
		// Close the storage to simulate persistence failure
		store.Close()

		// Try to perform transition
		err := f.Transition("B")
		if err == nil {
			t.Error("Transition() expected error when storage is closed, got nil")
			return
		}

		// Verify in-memory state unchanged
		if f.CurrentState() != initialState {
			t.Errorf("State changed to %v despite persistence failure, should remain %v",
				f.CurrentState(), initialState)
		}
	})
}

func TestFSM_WithoutStorage(t *testing.T) {
	def := createTestDefinition()

	f, err := New(def, "no-storage-instance", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Run("transitions work without storage", func(t *testing.T) {
		err := f.Transition("B")
		if err != nil {
			t.Errorf("Transition() error = %v", err)
		}

		if f.CurrentState() != "B" {
			t.Errorf("CurrentState() = %v, want B", f.CurrentState())
		}
	})

	t.Run("reset works without storage", func(t *testing.T) {
		err := f.Reset()
		if err != nil {
			t.Errorf("Reset() error = %v", err)
		}

		if f.CurrentState() != "A" {
			t.Errorf("CurrentState() after Reset() = %v, want A", f.CurrentState())
		}
	})
}
