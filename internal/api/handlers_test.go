package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rstyczynski/fsm_v2/internal/storage"
)

// setupTestServer creates a test server with in-memory storage
func setupTestServer(t *testing.T) (*Server, *storage.SQLiteStorage, func()) {
	t.Helper()

	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Use examples directory for asset types
	assetDir := "../../examples"

	server := NewServer(store, assetDir)

	cleanup := func() {
		store.Close()
	}

	return server, store, cleanup
}

// setURLParam adds a URL parameter to the request context (for Chi router testing)
func setURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestHealthEndpoint(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", resp.Status)
	}

	if resp.Version != "2.2.0" {
		t.Errorf("Expected version '2.2.0', got '%s'", resp.Version)
	}
}

func TestCreateAsset(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Check if examples directory exists
	if _, err := os.Stat("../../examples/simple_asset_type.yaml"); os.IsNotExist(err) {
		t.Skip("Skipping test: examples/simple_asset_type.yaml not found")
	}

	reqBody := CreateAssetRequest{
		AssetType:  "simple_asset_type.yaml",
		InstanceID: "test-asset-1",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleCreateAsset(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp AssetResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.ID != "test-asset-1" {
		t.Errorf("Expected ID 'test-asset-1', got '%s'", resp.ID)
	}

	if resp.CurrentState == "" {
		t.Error("Expected non-empty current state")
	}
}

func TestCreateAssetInvalidRequest(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name       string
		reqBody    CreateAssetRequest
		wantStatus int
	}{
		{
			name:       "missing asset type",
			reqBody:    CreateAssetRequest{InstanceID: "test-1"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing instance ID",
			reqBody:    CreateAssetRequest{AssetType: "simple_asset_type.yaml"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid asset type path",
			reqBody:    CreateAssetRequest{AssetType: "nonexistent.yaml", InstanceID: "test-1"},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.handleCreateAsset(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestListAssets(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Initially empty
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets", nil)
	w := httptest.NewRecorder()

	server.handleListAssets(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp ListAssetsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Count != 0 {
		t.Errorf("Expected count 0, got %d", resp.Count)
	}
}

func TestGetAsset(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Check if examples directory exists
	if _, err := os.Stat("../../examples/simple_asset_type.yaml"); os.IsNotExist(err) {
		t.Skip("Skipping test: examples/simple_asset_type.yaml not found")
	}

	// Create asset first
	reqBody := CreateAssetRequest{
		AssetType:  "simple_asset_type.yaml",
		InstanceID: "test-get-asset",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.handleCreateAsset(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create asset: %s", w.Body.String())
	}

	// Get asset
	req = httptest.NewRequest(http.MethodGet, "/api/v1/assets/test-get-asset", nil)
	req = setURLParam(req, "instanceID", "test-get-asset")
	w = httptest.NewRecorder()

	server.handleGetAsset(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp AssetResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.ID != "test-get-asset" {
		t.Errorf("Expected ID 'test-get-asset', got '%s'", resp.ID)
	}
}

func TestGetAssetNotFound(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets/nonexistent", nil)
	req = setURLParam(req, "instanceID", "nonexistent")
	w := httptest.NewRecorder()

	server.handleGetAsset(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestDeleteAsset(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Check if examples directory exists
	if _, err := os.Stat("../../examples/simple_asset_type.yaml"); os.IsNotExist(err) {
		t.Skip("Skipping test: examples/simple_asset_type.yaml not found")
	}

	// Create asset first
	reqBody := CreateAssetRequest{
		AssetType:  "simple_asset_type.yaml",
		InstanceID: "test-delete-asset",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.handleCreateAsset(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create asset: %s", w.Body.String())
	}

	// Delete asset
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/assets/test-delete-asset", nil)
	req = setURLParam(req, "instanceID", "test-delete-asset")
	w = httptest.NewRecorder()

	server.handleDeleteAsset(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify deletion
	req = httptest.NewRequest(http.MethodGet, "/api/v1/assets/test-delete-asset", nil)
	req = setURLParam(req, "instanceID", "test-delete-asset")
	w = httptest.NewRecorder()
	server.handleGetAsset(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 after deletion, got %d", w.Code)
	}
}

func TestTransition(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Check if examples directory exists
	if _, err := os.Stat("../../examples/simple_asset_type.yaml"); os.IsNotExist(err) {
		t.Skip("Skipping test: examples/simple_asset_type.yaml not found")
	}

	// Create asset first
	reqBody := CreateAssetRequest{
		AssetType:  "simple_asset_type.yaml",
		InstanceID: "test-transition-asset",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.handleCreateAsset(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create asset: %s", w.Body.String())
	}

	// Execute transition
	transReq := TransitionRequest{ToState: "STARTING"}
	body, _ = json.Marshal(transReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/assets/test-transition-asset/transition", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = setURLParam(req, "instanceID", "test-transition-asset")
	w = httptest.NewRecorder()

	server.handleTransition(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp TransitionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.ToState != "STARTING" {
		t.Errorf("Expected to_state 'STARTING', got '%s'", resp.ToState)
	}
}

func TestTransitionInvalid(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Check if examples directory exists
	if _, err := os.Stat("../../examples/simple_asset_type.yaml"); os.IsNotExist(err) {
		t.Skip("Skipping test: examples/simple_asset_type.yaml not found")
	}

	// Create asset first
	reqBody := CreateAssetRequest{
		AssetType:  "simple_asset_type.yaml",
		InstanceID: "test-invalid-transition",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.handleCreateAsset(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create asset: %s", w.Body.String())
	}

	// Try invalid transition (assuming TERMINATED is not directly reachable from initial state)
	transReq := TransitionRequest{ToState: "TERMINATED"}
	body, _ = json.Marshal(transReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/assets/test-invalid-transition/transition", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = setURLParam(req, "instanceID", "test-invalid-transition")
	w = httptest.NewRecorder()

	server.handleTransition(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid transition, got %d", w.Code)
	}
}

func TestHistory(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Check if examples directory exists
	if _, err := os.Stat("../../examples/simple_asset_type.yaml"); os.IsNotExist(err) {
		t.Skip("Skipping test: examples/simple_asset_type.yaml not found")
	}

	// Create asset
	reqBody := CreateAssetRequest{
		AssetType:  "simple_asset_type.yaml",
		InstanceID: "test-history-asset",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.handleCreateAsset(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create asset: %s", w.Body.String())
	}

	// Execute some transitions
	transitions := []string{"STARTING", "RUNNING"}
	for _, state := range transitions {
		transReq := TransitionRequest{ToState: state}
		body, _ = json.Marshal(transReq)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/assets/test-history-asset/transition", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = setURLParam(req, "instanceID", "test-history-asset")
		w = httptest.NewRecorder()
		server.handleTransition(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Failed to transition to %s: %s", state, w.Body.String())
		}
	}

	// Get history
	req = httptest.NewRequest(http.MethodGet, "/api/v1/assets/test-history-asset/history", nil)
	req = setURLParam(req, "instanceID", "test-history-asset")
	w = httptest.NewRecorder()

	server.handleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp HistoryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.InstanceID != "test-history-asset" {
		t.Errorf("Expected instance_id 'test-history-asset', got '%s'", resp.InstanceID)
	}

	if len(resp.Transitions) < 2 {
		t.Errorf("Expected at least 2 transitions, got %d", len(resp.Transitions))
	}
}

func TestHistoryWithLimit(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Check if examples directory exists
	if _, err := os.Stat("../../examples/simple_asset_type.yaml"); os.IsNotExist(err) {
		t.Skip("Skipping test: examples/simple_asset_type.yaml not found")
	}

	// Create asset
	reqBody := CreateAssetRequest{
		AssetType:  "simple_asset_type.yaml",
		InstanceID: "test-history-limit",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.handleCreateAsset(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create asset: %s", w.Body.String())
	}

	// Execute multiple transitions
	transitions := []string{"STARTING", "RUNNING", "MAINTENANCE"}
	for _, state := range transitions {
		transReq := TransitionRequest{ToState: state}
		body, _ = json.Marshal(transReq)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/assets/test-history-limit/transition", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = setURLParam(req, "instanceID", "test-history-limit")
		w = httptest.NewRecorder()
		server.handleTransition(w, req)
	}

	// Get history with limit
	req = httptest.NewRequest(http.MethodGet, "/api/v1/assets/test-history-limit/history?limit=2", nil)
	req = setURLParam(req, "instanceID", "test-history-limit")
	w = httptest.NewRecorder()

	server.handleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp HistoryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Transitions) > 2 {
		t.Errorf("Expected at most 2 transitions with limit=2, got %d", len(resp.Transitions))
	}
}

func TestCORSHeaders(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/assets", nil)
	w := httptest.NewRecorder()

	handler := server.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("Expected CORS headers to be set")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS request, got %d", w.Code)
	}
}

func TestInvalidJSON(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateAsset(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", w.Code)
	}
}
