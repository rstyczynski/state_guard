package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements Storage interface using SQLite
type SQLiteStorage struct {
	db   *sql.DB
	path string
}

// NewSQLiteStorage creates a new SQLite storage backend
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path cannot be empty")
	}

	return &SQLiteStorage{
		path: dbPath,
	}, nil
}

// Initialize sets up the SQLite database and creates tables
func (s *SQLiteStorage) Initialize(ctx context.Context) error {
	db, err := sql.Open("sqlite3", s.path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys and WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	s.db = db

	// Create tables
	schema := `
		CREATE TABLE IF NOT EXISTS fsm_instances (
			id TEXT PRIMARY KEY,
			definition_name TEXT NOT NULL,
			asset_type_name TEXT NOT NULL DEFAULT '',
			current_state TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_instances_definition ON fsm_instances(definition_name);
		CREATE INDEX IF NOT EXISTS idx_instances_updated ON fsm_instances(updated_at);
		CREATE INDEX IF NOT EXISTS idx_instances_asset_type ON fsm_instances(asset_type_name);

		CREATE TABLE IF NOT EXISTS state_transitions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			instance_id TEXT NOT NULL,
			from_state TEXT NOT NULL,
			to_state TEXT NOT NULL,
			transitioned_at TIMESTAMP NOT NULL,
			FOREIGN KEY (instance_id) REFERENCES fsm_instances(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_transitions_instance ON state_transitions(instance_id);
		CREATE INDEX IF NOT EXISTS idx_transitions_time ON state_transitions(transitioned_at);
	`

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Migration: Add asset_type_name column to existing tables
	migrationQuery := `
		ALTER TABLE fsm_instances ADD COLUMN asset_type_name TEXT NOT NULL DEFAULT '';
	`
	// Ignore error if column already exists
	s.db.ExecContext(ctx, migrationQuery)

	return nil
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// CreateInstance creates a new FSM instance in storage
func (s *SQLiteStorage) CreateInstance(ctx context.Context, instance *FSMInstance) error {
	if instance == nil {
		return fmt.Errorf("instance cannot be nil")
	}

	if instance.ID == "" {
		return fmt.Errorf("instance ID cannot be empty")
	}

	now := time.Now()
	if instance.CreatedAt.IsZero() {
		instance.CreatedAt = now
	}
	if instance.UpdatedAt.IsZero() {
		instance.UpdatedAt = now
	}

	query := `
		INSERT INTO fsm_instances (id, definition_name, asset_type_name, current_state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		instance.ID,
		instance.DefinitionName,
		instance.AssetTypeName,
		instance.CurrentState,
		instance.CreatedAt,
		instance.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	return nil
}

// GetInstance retrieves an FSM instance by ID
func (s *SQLiteStorage) GetInstance(ctx context.Context, id string) (*FSMInstance, error) {
	query := `
		SELECT id, definition_name, asset_type_name, current_state, created_at, updated_at
		FROM fsm_instances
		WHERE id = ?
	`

	instance := &FSMInstance{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&instance.ID,
		&instance.DefinitionName,
		&instance.AssetTypeName,
		&instance.CurrentState,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("instance not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	return instance, nil
}

// UpdateState atomically updates the state of an FSM instance
func (s *SQLiteStorage) UpdateState(ctx context.Context, id string, fromState, toState string) error {
	// Start transaction for atomicity
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update current state
	now := time.Now()
	updateQuery := `
		UPDATE fsm_instances
		SET current_state = ?, updated_at = ?
		WHERE id = ? AND current_state = ?
	`

	result, err := tx.ExecContext(ctx, updateQuery, toState, now, id, fromState)
	if err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("state update failed: instance not found or state mismatch (expected: %s)", fromState)
	}

	// Record transition in history
	historyQuery := `
		INSERT INTO state_transitions (instance_id, from_state, to_state, transitioned_at)
		VALUES (?, ?, ?, ?)
	`

	_, err = tx.ExecContext(ctx, historyQuery, id, fromState, toState, now)
	if err != nil {
		return fmt.Errorf("failed to record transition: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteInstance removes an FSM instance from storage
func (s *SQLiteStorage) DeleteInstance(ctx context.Context, id string) error {
	query := `DELETE FROM fsm_instances WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("instance not found: %s", id)
	}

	return nil
}

// ListInstances returns all FSM instances
func (s *SQLiteStorage) ListInstances(ctx context.Context) ([]*FSMInstance, error) {
	query := `
		SELECT id, definition_name, asset_type_name, current_state, created_at, updated_at
		FROM fsm_instances
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}
	defer rows.Close()

	var instances []*FSMInstance
	for rows.Next() {
		instance := &FSMInstance{}
		err := rows.Scan(
			&instance.ID,
			&instance.DefinitionName,
			&instance.AssetTypeName,
			&instance.CurrentState,
			&instance.CreatedAt,
			&instance.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan instance: %w", err)
		}
		instances = append(instances, instance)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating instances: %w", err)
	}

	return instances, nil
}

// GetTransitionHistory returns state transitions for an instance
func (s *SQLiteStorage) GetTransitionHistory(ctx context.Context, instanceID string, limit int) ([]*StateTransition, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	query := `
		SELECT id, instance_id, from_state, to_state, transitioned_at
		FROM state_transitions
		WHERE instance_id = ?
		ORDER BY transitioned_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, instanceID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get transition history: %w", err)
	}
	defer rows.Close()

	var transitions []*StateTransition
	for rows.Next() {
		transition := &StateTransition{}
		err := rows.Scan(
			&transition.ID,
			&transition.InstanceID,
			&transition.FromState,
			&transition.ToState,
			&transition.TransitionedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transition: %w", err)
		}
		transitions = append(transitions, transition)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transitions: %w", err)
	}

	return transitions, nil
}
