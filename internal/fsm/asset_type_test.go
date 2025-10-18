package fsm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWebhookConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		webhook WebhookConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid on_enter",
			webhook: WebhookConfig{
				Condition: OnEnter,
				State:     "RUNNING",
				URL:       "https://example.com/hook",
				Method:    "POST",
				Timeout:   "5s",
			},
			wantErr: false,
		},
		{
			name: "valid on_exit",
			webhook: WebhookConfig{
				Condition: OnExit,
				State:     "RUNNING",
				URL:       "https://example.com/hook",
				Method:    "POST",
				Timeout:   "5s",
			},
			wantErr: false,
		},
		{
			name: "valid on_transition",
			webhook: WebhookConfig{
				Condition: OnTransition,
				From:      "CREATED",
				To:        "STARTING",
				URL:       "https://example.com/hook",
				Method:    "POST",
				Timeout:   "5s",
			},
			wantErr: false,
		},
		{
			name: "on_enter without state",
			webhook: WebhookConfig{
				Condition: OnEnter,
				URL:       "https://example.com/hook",
			},
			wantErr: true,
			errMsg:  "state is required",
		},
		{
			name: "on_transition without from",
			webhook: WebhookConfig{
				Condition: OnTransition,
				To:        "STARTING",
				URL:       "https://example.com/hook",
			},
			wantErr: true,
			errMsg:  "from and to are required",
		},
		{
			name: "invalid condition",
			webhook: WebhookConfig{
				Condition: "invalid",
				URL:       "https://example.com/hook",
			},
			wantErr: true,
			errMsg:  "unknown condition",
		},
		{
			name: "missing URL",
			webhook: WebhookConfig{
				Condition: OnEnter,
				State:     "RUNNING",
			},
			wantErr: true,
			errMsg:  "url is required",
		},
		{
			name: "invalid timeout",
			webhook: WebhookConfig{
				Condition: OnEnter,
				State:     "RUNNING",
				URL:       "https://example.com/hook",
				Timeout:   "invalid",
			},
			wantErr: true,
			errMsg:  "invalid timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.webhook.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing '%s', got nil", tt.errMsg)
					return
				}
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = '%v', want error containing '%s'", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestWebhookConfig_Matches(t *testing.T) {
	tests := []struct {
		name      string
		webhook   WebhookConfig
		condition WebhookCondition
		fromState string
		toState   string
		want      bool
	}{
		{
			name: "on_enter matches",
			webhook: WebhookConfig{
				Condition: OnEnter,
				State:     "RUNNING",
			},
			condition: OnEnter,
			fromState: "STARTING",
			toState:   "RUNNING",
			want:      true,
		},
		{
			name: "on_enter does not match wrong state",
			webhook: WebhookConfig{
				Condition: OnEnter,
				State:     "RUNNING",
			},
			condition: OnEnter,
			fromState: "STARTING",
			toState:   "STOPPED",
			want:      false,
		},
		{
			name: "on_exit matches",
			webhook: WebhookConfig{
				Condition: OnExit,
				State:     "RUNNING",
			},
			condition: OnExit,
			fromState: "RUNNING",
			toState:   "STOPPING",
			want:      true,
		},
		{
			name: "on_transition matches",
			webhook: WebhookConfig{
				Condition: OnTransition,
				From:      "CREATED",
				To:        "STARTING",
			},
			condition: OnTransition,
			fromState: "CREATED",
			toState:   "STARTING",
			want:      true,
		},
		{
			name: "on_transition does not match wrong states",
			webhook: WebhookConfig{
				Condition: OnTransition,
				From:      "CREATED",
				To:        "STARTING",
			},
			condition: OnTransition,
			fromState: "STARTING",
			toState:   "RUNNING",
			want:      false,
		},
		{
			name: "different condition does not match",
			webhook: WebhookConfig{
				Condition: OnEnter,
				State:     "RUNNING",
			},
			condition: OnExit,
			fromState: "STARTING",
			toState:   "RUNNING",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.webhook.Matches(tt.condition, tt.fromState, tt.toState)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAssetType_Validate(t *testing.T) {
	tests := []struct {
		name      string
		assetType AssetType
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid asset type",
			assetType: AssetType{
				Version:      1,
				Name:         "web-server",
				StateMachine: "lifecycle.yaml",
			},
			wantErr: false,
		},
		{
			name: "invalid version",
			assetType: AssetType{
				Version:      2,
				Name:         "web-server",
				StateMachine: "lifecycle.yaml",
			},
			wantErr: true,
			errMsg:  "unsupported version",
		},
		{
			name: "missing name",
			assetType: AssetType{
				Version:      1,
				StateMachine: "lifecycle.yaml",
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "missing state machine",
			assetType: AssetType{
				Version: 1,
				Name:    "web-server",
			},
			wantErr: true,
			errMsg:  "state_machine is required",
		},
		{
			name: "invalid webhook",
			assetType: AssetType{
				Version:      1,
				Name:         "web-server",
				StateMachine: "lifecycle.yaml",
				Webhooks: []WebhookConfig{
					{
						Condition: OnEnter,
						// Missing State and URL
					},
				},
			},
			wantErr: true,
			errMsg:  "webhook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.assetType.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing '%s', got nil", tt.errMsg)
					return
				}
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = '%v', want error containing '%s'", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestLoadAssetType(t *testing.T) {
	tempDir := t.TempDir()

	validAssetType := `version: 1
name: web-server
state_machine: examples/lifecycle.yaml
webhooks:
  - condition: on_enter
    state: RUNNING
    url: https://example.com/running
    method: POST
    timeout: 5s
  - condition: on_transition
    from: CREATED
    to: STARTING
    url: https://example.com/starting
    method: POST
    timeout: 10s
`

	invalidYAML := `invalid: yaml: syntax: [
`

	invalidAssetType := `version: 2
name: invalid
`

	tests := []struct {
		name    string
		content string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid asset type",
			content: validAssetType,
			wantErr: false,
		},
		{
			name:    "invalid YAML syntax",
			content: invalidYAML,
			wantErr: true,
			errMsg:  "failed to parse YAML",
		},
		{
			name:    "invalid asset type",
			content: invalidAssetType,
			wantErr: true,
			errMsg:  "invalid asset type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := filepath.Join(tempDir, "asset_type.yaml")
			err := os.WriteFile(filename, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			assetType, err := LoadAssetType(filename)
			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadAssetType() expected error containing '%s', got nil", tt.errMsg)
					return
				}
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("LoadAssetType() error = '%v', want error containing '%s'", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("LoadAssetType() unexpected error = %v", err)
					return
				}
				if assetType == nil {
					t.Error("LoadAssetType() returned nil")
				}
			}
		})
	}

	t.Run("non-existent file", func(t *testing.T) {
		_, err := LoadAssetType("/nonexistent/file.yaml")
		if err == nil {
			t.Error("LoadAssetType() expected error for non-existent file")
		}
	})
}

func TestAssetType_LoadStateMachine(t *testing.T) {
	// Create a temporary state machine file
	tempDir := t.TempDir()
	fsmFile := filepath.Join(tempDir, "test_fsm.yaml")
	fsmContent := `version: 1
name: test_fsm
initial: A
states:
  - A
  - B
transitions:
  - from: A
    to: B
`
	err := os.WriteFile(fsmFile, []byte(fsmContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write FSM file: %v", err)
	}

	t.Run("load existing state machine", func(t *testing.T) {
		assetType := &AssetType{
			Version:      1,
			Name:         "test-asset",
			StateMachine: fsmFile,
		}

		err := assetType.LoadStateMachine()
		if err != nil {
			t.Errorf("LoadStateMachine() unexpected error = %v", err)
			return
		}

		if assetType.StateMachineRef == nil {
			t.Error("LoadStateMachine() did not set StateMachineRef")
			return
		}

		if assetType.StateMachineRef.Name != "test_fsm" {
			t.Errorf("Loaded FSM name = %v, want test_fsm", assetType.StateMachineRef.Name)
		}
	})

	t.Run("load non-existent state machine", func(t *testing.T) {
		assetType := &AssetType{
			Version:      1,
			Name:         "test-asset",
			StateMachine: "/nonexistent/fsm.yaml",
		}

		err := assetType.LoadStateMachine()
		if err == nil {
			t.Error("LoadStateMachine() expected error for non-existent file")
		}
	})
}

func TestWebhookConfig_GetTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		want    string
	}{
		{
			name:    "5 seconds",
			timeout: "5s",
			want:    "5s",
		},
		{
			name:    "10 seconds",
			timeout: "10s",
			want:    "10s",
		},
		{
			name:    "1 minute",
			timeout: "1m",
			want:    "1m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wc := &WebhookConfig{Timeout: tt.timeout}
			got := wc.GetTimeout()
			if got.String() != tt.want {
				t.Errorf("GetTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}
