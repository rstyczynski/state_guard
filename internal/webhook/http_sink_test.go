package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPSink_Send(t *testing.T) {
	t.Run("successful POST request", func(t *testing.T) {
		// Create test server
		var receivedEvent *Event
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != "POST" {
				t.Errorf("Expected POST, got %s", r.Method)
			}

			// Read and decode body
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &receivedEvent)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		// Create sink
		sink := NewHTTPSink(server.URL, "POST", 5*time.Second, nil)

		// Send event
		event := &Event{
			InstanceID: "test-instance",
			AssetType:  "test-type",
			FromState:  "A",
			ToState:    "B",
			Timestamp:  time.Now(),
			Metadata:   make(map[string]string),
		}

		ctx := context.Background()
		err := sink.Send(ctx, event)
		if err != nil {
			t.Errorf("Send() unexpected error = %v", err)
		}

		// Verify received event
		if receivedEvent == nil {
			t.Fatal("Server did not receive event")
		}
		if receivedEvent.InstanceID != "test-instance" {
			t.Errorf("Received InstanceID = %v, want test-instance", receivedEvent.InstanceID)
		}
	})

	t.Run("server returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		sink := NewHTTPSink(server.URL, "POST", 5*time.Second, nil)

		event := &Event{
			InstanceID: "test-instance",
			AssetType:  "test-type",
			FromState:  "A",
			ToState:    "B",
			Timestamp:  time.Now(),
		}

		ctx := context.Background()
		err := sink.Send(ctx, event)
		if err == nil {
			t.Error("Send() expected error for 500 status, got nil")
		}
	})

	t.Run("timeout protection", func(t *testing.T) {
		// Create server that delays response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Create sink with short timeout
		sink := NewHTTPSink(server.URL, "POST", 100*time.Millisecond, nil)

		event := &Event{
			InstanceID: "test-instance",
			AssetType:  "test-type",
			FromState:  "A",
			ToState:    "B",
			Timestamp:  time.Now(),
		}

		ctx := context.Background()
		err := sink.Send(ctx, event)
		if err == nil {
			t.Error("Send() expected timeout error, got nil")
		}
	})

	t.Run("custom headers", func(t *testing.T) {
		var receivedHeaders http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = r.Header
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		headers := map[string]string{
			"X-Custom-Header": "test-value",
			"Authorization":   "Bearer token123",
		}
		sink := NewHTTPSink(server.URL, "POST", 5*time.Second, headers)

		event := &Event{
			InstanceID: "test-instance",
			AssetType:  "test-type",
			FromState:  "A",
			ToState:    "B",
			Timestamp:  time.Now(),
		}

		ctx := context.Background()
		err := sink.Send(ctx, event)
		if err != nil {
			t.Errorf("Send() unexpected error = %v", err)
		}

		if receivedHeaders.Get("X-Custom-Header") != "test-value" {
			t.Errorf("Custom header not received correctly")
		}
		if receivedHeaders.Get("Authorization") != "Bearer token123" {
			t.Errorf("Authorization header not received correctly")
		}
	})

	t.Run("GET method", func(t *testing.T) {
		var receivedMethod string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sink := NewHTTPSink(server.URL, "GET", 5*time.Second, nil)

		event := &Event{
			InstanceID: "test-instance",
			AssetType:  "test-type",
			FromState:  "A",
			ToState:    "B",
			Timestamp:  time.Now(),
		}

		ctx := context.Background()
		err := sink.Send(ctx, event)
		if err != nil {
			t.Errorf("Send() unexpected error = %v", err)
		}

		if receivedMethod != "GET" {
			t.Errorf("Expected GET method, got %s", receivedMethod)
		}
	})
}

func TestHTTPSink_Properties(t *testing.T) {
	url := "https://example.com/webhook"
	method := "POST"
	timeout := 10 * time.Second
	headers := map[string]string{"X-Test": "value"}

	sink := NewHTTPSink(url, method, timeout, headers)

	if sink.Name() != "http" {
		t.Errorf("Name() = %v, want http", sink.Name())
	}

	if sink.URL() != url {
		t.Errorf("URL() = %v, want %v", sink.URL(), url)
	}

	if sink.Method() != method {
		t.Errorf("Method() = %v, want %v", sink.Method(), method)
	}

	if sink.Timeout() != timeout {
		t.Errorf("Timeout() = %v, want %v", sink.Timeout(), timeout)
	}

	err := sink.Close()
	if err != nil {
		t.Errorf("Close() unexpected error = %v", err)
	}
}
