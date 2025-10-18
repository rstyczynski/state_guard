package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPSink implements the Sink interface for HTTP/HTTPS webhooks
type HTTPSink struct {
	url     string
	method  string
	timeout time.Duration
	headers map[string]string
	client  *http.Client
}

// NewHTTPSink creates a new HTTP webhook sink
func NewHTTPSink(url, method string, timeout time.Duration, headers map[string]string) *HTTPSink {
	return &HTTPSink{
		url:     url,
		method:  method,
		timeout: timeout,
		headers: headers,
		client: &http.Client{
			Timeout: timeout, // Built-in protection from blocking connections
		},
	}
}

// Send sends the webhook notification via HTTP/HTTPS
func (h *HTTPSink) Send(ctx context.Context, event *Event) error {
	// Prepare payload
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, h.method, h.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FSM-Webhook/1.0")
	for key, value := range h.headers {
		req.Header.Set(key, value)
	}

	// Send request with timeout protection
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	// Read response body (limited to avoid memory issues)
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // Max 1MB response

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Name returns the sink type name
func (h *HTTPSink) Name() string {
	return "http"
}

// Close cleans up resources (HTTP client has no resources to clean up)
func (h *HTTPSink) Close() error {
	return nil
}

// URL returns the configured webhook URL
func (h *HTTPSink) URL() string {
	return h.url
}

// Method returns the configured HTTP method
func (h *HTTPSink) Method() string {
	return h.method
}

// Timeout returns the configured timeout
func (h *HTTPSink) Timeout() time.Duration {
	return h.timeout
}
