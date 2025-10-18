package webhook

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rstyczynski/fsm_v2/internal/fsm"
)

func TestNewDispatcher(t *testing.T) {
	t.Run("create with valid asset type", func(t *testing.T) {
		assetType := &fsm.AssetType{
			Version:      1,
			Name:         "test-asset",
			StateMachine: "test.yaml",
			Webhooks: []fsm.WebhookConfig{
				{
					Condition: fsm.OnEnter,
					State:     "RUNNING",
					URL:       "https://example.com/hook",
					Method:    "POST",
					Timeout:   "5s",
				},
			},
		}

		// Validate webhooks first
		for i := range assetType.Webhooks {
			if err := assetType.Webhooks[i].Validate(); err != nil {
				t.Fatalf("Webhook validation failed: %v", err)
			}
		}

		dispatcher, err := NewDispatcher(5, 100, assetType)
		if err != nil {
			t.Errorf("NewDispatcher() unexpected error = %v", err)
		}
		defer dispatcher.Close()

		if dispatcher.WorkerCount() != 5 {
			t.Errorf("WorkerCount() = %d, want 5", dispatcher.WorkerCount())
		}
	})

	t.Run("create with nil asset type", func(t *testing.T) {
		dispatcher, err := NewDispatcher(5, 100, nil)
		if err != nil {
			t.Errorf("NewDispatcher() unexpected error = %v", err)
		}
		defer dispatcher.Close()

		// Should work fine with no webhooks
		dispatcher.Notify("test", "test-type", "A", "B")
	})

	t.Run("default workers and queue size", func(t *testing.T) {
		dispatcher, err := NewDispatcher(0, 0, nil)
		if err != nil {
			t.Errorf("NewDispatcher() unexpected error = %v", err)
		}
		defer dispatcher.Close()

		if dispatcher.WorkerCount() != 5 {
			t.Errorf("WorkerCount() = %d, want 5 (default)", dispatcher.WorkerCount())
		}
	})
}

func TestDispatcher_Notify(t *testing.T) {
	t.Run("on_enter webhook triggers", func(t *testing.T) {
		// Create test server
		var receivedEvents []*Event
		var mu sync.Mutex
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var event Event
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &event)

			mu.Lock()
			receivedEvents = append(receivedEvents, &event)
			mu.Unlock()

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Create asset type with on_enter webhook
		assetType := &fsm.AssetType{
			Version:      1,
			Name:         "test-asset",
			StateMachine: "test.yaml",
			Webhooks: []fsm.WebhookConfig{
				{
					Condition: fsm.OnEnter,
					State:     "RUNNING",
					URL:       server.URL,
					Method:    "POST",
					Timeout:   "5s",
				},
			},
		}

		// Validate webhooks
		for i := range assetType.Webhooks {
			assetType.Webhooks[i].Validate()
		}

		dispatcher, err := NewDispatcher(2, 10, assetType)
		if err != nil {
			t.Fatalf("NewDispatcher() error = %v", err)
		}
		defer dispatcher.Close()

		// Notify transition to RUNNING (should trigger)
		dispatcher.Notify("test-instance", "test-asset", "STARTING", "RUNNING")

		// Wait for webhook to be processed
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		if len(receivedEvents) != 1 {
			t.Errorf("Expected 1 webhook, got %d", len(receivedEvents))
		}
		if len(receivedEvents) > 0 {
			if receivedEvents[0].ToState != "RUNNING" {
				t.Errorf("Event ToState = %v, want RUNNING", receivedEvents[0].ToState)
			}
		}
		mu.Unlock()
	})

	t.Run("on_exit webhook triggers", func(t *testing.T) {
		var receivedEvents []*Event
		var mu sync.Mutex
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var event Event
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &event)

			mu.Lock()
			receivedEvents = append(receivedEvents, &event)
			mu.Unlock()

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		assetType := &fsm.AssetType{
			Version:      1,
			Name:         "test-asset",
			StateMachine: "test.yaml",
			Webhooks: []fsm.WebhookConfig{
				{
					Condition: fsm.OnExit,
					State:     "RUNNING",
					URL:       server.URL,
					Method:    "POST",
					Timeout:   "5s",
				},
			},
		}

		for i := range assetType.Webhooks {
			assetType.Webhooks[i].Validate()
		}

		dispatcher, err := NewDispatcher(2, 10, assetType)
		if err != nil {
			t.Fatalf("NewDispatcher() error = %v", err)
		}
		defer dispatcher.Close()

		// Notify transition from RUNNING (should trigger)
		dispatcher.Notify("test-instance", "test-asset", "RUNNING", "STOPPING")

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		if len(receivedEvents) != 1 {
			t.Errorf("Expected 1 webhook, got %d", len(receivedEvents))
		}
		if len(receivedEvents) > 0 {
			if receivedEvents[0].FromState != "RUNNING" {
				t.Errorf("Event FromState = %v, want RUNNING", receivedEvents[0].FromState)
			}
		}
		mu.Unlock()
	})

	t.Run("on_transition webhook triggers", func(t *testing.T) {
		var receivedEvents []*Event
		var mu sync.Mutex
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var event Event
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &event)

			mu.Lock()
			receivedEvents = append(receivedEvents, &event)
			mu.Unlock()

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		assetType := &fsm.AssetType{
			Version:      1,
			Name:         "test-asset",
			StateMachine: "test.yaml",
			Webhooks: []fsm.WebhookConfig{
				{
					Condition: fsm.OnTransition,
					From:      "CREATED",
					To:        "STARTING",
					URL:       server.URL,
					Method:    "POST",
					Timeout:   "5s",
				},
			},
		}

		for i := range assetType.Webhooks {
			assetType.Webhooks[i].Validate()
		}

		dispatcher, err := NewDispatcher(2, 10, assetType)
		if err != nil {
			t.Fatalf("NewDispatcher() error = %v", err)
		}
		defer dispatcher.Close()

		// Notify CREATED -> STARTING transition (should trigger)
		dispatcher.Notify("test-instance", "test-asset", "CREATED", "STARTING")

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		if len(receivedEvents) != 1 {
			t.Errorf("Expected 1 webhook, got %d", len(receivedEvents))
		}
		if len(receivedEvents) > 0 {
			if receivedEvents[0].FromState != "CREATED" || receivedEvents[0].ToState != "STARTING" {
				t.Errorf("Event transition = %v->%v, want CREATED->STARTING",
					receivedEvents[0].FromState, receivedEvents[0].ToState)
			}
		}
		mu.Unlock()
	})

	t.Run("webhook does not trigger for non-matching transition", func(t *testing.T) {
		var receivedEvents []*Event
		var mu sync.Mutex
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var event Event
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &event)

			mu.Lock()
			receivedEvents = append(receivedEvents, &event)
			mu.Unlock()

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		assetType := &fsm.AssetType{
			Version:      1,
			Name:         "test-asset",
			StateMachine: "test.yaml",
			Webhooks: []fsm.WebhookConfig{
				{
					Condition: fsm.OnEnter,
					State:     "RUNNING",
					URL:       server.URL,
					Method:    "POST",
					Timeout:   "5s",
				},
			},
		}

		for i := range assetType.Webhooks {
			assetType.Webhooks[i].Validate()
		}

		dispatcher, err := NewDispatcher(2, 10, assetType)
		if err != nil {
			t.Fatalf("NewDispatcher() error = %v", err)
		}
		defer dispatcher.Close()

		// Notify transition to STOPPED (should NOT trigger)
		dispatcher.Notify("test-instance", "test-asset", "STOPPING", "STOPPED")

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		if len(receivedEvents) != 0 {
			t.Errorf("Expected 0 webhooks, got %d", len(receivedEvents))
		}
		mu.Unlock()
	})
}

func TestDispatcher_Close(t *testing.T) {
	assetType := &fsm.AssetType{
		Version:      1,
		Name:         "test-asset",
		StateMachine: "test.yaml",
		Webhooks: []fsm.WebhookConfig{
			{
				Condition: fsm.OnEnter,
				State:     "RUNNING",
				URL:       "https://example.com/hook",
				Method:    "POST",
				Timeout:   "5s",
			},
		},
	}

	for i := range assetType.Webhooks {
		assetType.Webhooks[i].Validate()
	}

	dispatcher, err := NewDispatcher(5, 100, assetType)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	err = dispatcher.Close()
	if err != nil {
		t.Errorf("Close() unexpected error = %v", err)
	}
}
