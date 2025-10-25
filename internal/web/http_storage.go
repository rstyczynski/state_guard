package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rstyczynski/fsm_v2/internal/storage"
)

// HTTPClient implements Storage interface by proxying to sg_api HTTP endpoints
type HTTPClient struct {
	baseURL string
	client  *http.Client
}

// NewHTTPClient creates a new HTTP storage client
func NewHTTPClient(apiURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: apiURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Initialize is a no-op for HTTP client
func (h *HTTPClient) Initialize(ctx context.Context) error {
	// Test connection
	resp, err := h.client.Get(h.baseURL + "/api/v1/health")
	if err != nil {
		return fmt.Errorf("failed to connect to API server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API server unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// Close is a no-op for HTTP client
func (h *HTTPClient) Close() error {
	return nil
}

// GetInstance fetches an asset instance via HTTP
func (h *HTTPClient) GetInstance(ctx context.Context, instanceID string) (*storage.FSMInstance, error) {
	url := fmt.Sprintf("%s/api/v1/assets/%s", h.baseURL, instanceID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("instance not found")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var instance storage.FSMInstance
	if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &instance, nil
}

// UpdateState is not supported in HTTP client (read-only for visualization)
func (h *HTTPClient) UpdateState(ctx context.Context, id string, fromState, toState string) error {
	return fmt.Errorf("UpdateState not supported in HTTP client")
}

// GetTransitionHistory fetches asset history via HTTP
func (h *HTTPClient) GetTransitionHistory(ctx context.Context, instanceID string, limit int) ([]*storage.StateTransition, error) {
	url := fmt.Sprintf("%s/api/v1/assets/%s/history", h.baseURL, instanceID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Transitions []*storage.StateTransition `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Transitions, nil
}

// ListInstances is not used by visualization, return empty
func (h *HTTPClient) ListInstances(ctx context.Context) ([]*storage.FSMInstance, error) {
	return []*storage.FSMInstance{}, nil
}

// CreateInstance is not supported in HTTP client (read-only for visualization)
func (h *HTTPClient) CreateInstance(ctx context.Context, instance *storage.FSMInstance) error {
	return fmt.Errorf("CreateInstance not supported in HTTP client")
}

// DeleteInstance is not supported in HTTP client (read-only for visualization)
func (h *HTTPClient) DeleteInstance(ctx context.Context, instanceID string) error {
	return fmt.Errorf("DeleteInstance not supported in HTTP client")
}
