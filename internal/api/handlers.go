package api

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/rstyczynski/fsm_v2/internal/fsm"
	"github.com/rstyczynski/fsm_v2/internal/storage"
	"github.com/rstyczynski/fsm_v2/internal/webhook"
)

// handleListAssets lists all asset instances
func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	instances, err := s.storage.ListInstances(ctx)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to list instances: %v", err))
		return
	}

	assets := make([]AssetResponse, 0, len(instances))
	for _, inst := range instances {
		// Load FSM to get available transitions
		assetResp, err := s.buildAssetResponse(ctx, inst)
		if err != nil {
			// Log error but continue with partial data
			assetResp = AssetResponse{
				ID:             inst.ID,
				AssetType:      inst.AssetTypeName,
				DefinitionName: inst.DefinitionName,
				CurrentState:   inst.CurrentState,
				CreatedAt:      inst.CreatedAt,
				UpdatedAt:      inst.UpdatedAt,
			}
		}
		assets = append(assets, assetResp)
	}

	respondJSON(w, r, http.StatusOK, ListAssetsResponse{
		Assets: assets,
		Count:  len(assets),
	})
}

// handleCreateAsset creates a new asset instance
func (s *Server) handleCreateAsset(w http.ResponseWriter, r *http.Request) {
	var req CreateAssetRequest
	if err := render.Bind(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// Resolve asset type path
	assetTypePath := req.AssetType
	if !filepath.IsAbs(assetTypePath) && s.assetDir != "" {
		assetTypePath = filepath.Join(s.assetDir, assetTypePath)
	}

	// Load asset type
	assetType, err := fsm.LoadAssetType(assetTypePath)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, fmt.Sprintf("Failed to load asset type: %v", err))
		return
	}

	// Load FSM definition from asset type
	if err := assetType.LoadStateMachine(); err != nil {
		respondError(w, r, http.StatusBadRequest, fmt.Sprintf("Failed to load state machine: %v", err))
		return
	}

	// Create webhook dispatcher if configured
	var dispatcher *webhook.Dispatcher
	if len(assetType.Webhooks) > 0 {
		dispatcher, err = webhook.NewDispatcher(5, 100, assetType)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to create webhook dispatcher: %v", err))
			return
		}
		defer dispatcher.Close()
	}

	// Create FSM instance with webhook support
	f, err := fsm.NewWithWebhook(assetType.StateMachineRef, req.InstanceID, req.AssetType, s.storage, dispatcher)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, fmt.Sprintf("Failed to create FSM instance: %v", err))
		return
	}

	// Get instance from storage to return complete data
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	instance, err := s.storage.GetInstance(ctx, req.InstanceID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve created instance: %v", err))
		return
	}

	assetResp := AssetResponse{
		ID:                   instance.ID,
		AssetType:            instance.AssetTypeName,
		DefinitionName:       instance.DefinitionName,
		CurrentState:         instance.CurrentState,
		AvailableTransitions: f.AvailableTransitions(),
		IsFinalState:         f.IsFinalState(),
		CreatedAt:            instance.CreatedAt,
		UpdatedAt:            instance.UpdatedAt,
	}

	respondJSON(w, r, http.StatusCreated, assetResp)
}

// handleGetAsset retrieves a specific asset instance
func (s *Server) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instanceID")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	instance, err := s.storage.GetInstance(ctx, instanceID)
	if err != nil {
		respondError(w, r, http.StatusNotFound, fmt.Sprintf("Instance not found: %v", err))
		return
	}

	assetResp, err := s.buildAssetResponse(ctx, instance)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to build response: %v", err))
		return
	}

	respondJSON(w, r, http.StatusOK, assetResp)
}

// handleDeleteAsset deletes an asset instance
func (s *Server) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instanceID")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.storage.DeleteInstance(ctx, instanceID); err != nil {
		respondError(w, r, http.StatusNotFound, fmt.Sprintf("Failed to delete instance: %v", err))
		return
	}

	respondJSON(w, r, http.StatusOK, map[string]string{
		"message": "Instance deleted successfully",
		"id":      instanceID,
	})
}

// handleTransition executes a state transition
func (s *Server) handleTransition(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instanceID")

	var req TransitionRequest
	if err := render.Bind(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Load instance from storage
	instance, err := s.storage.GetInstance(ctx, instanceID)
	if err != nil {
		respondError(w, r, http.StatusNotFound, fmt.Sprintf("Instance not found: %v", err))
		return
	}

	// Determine asset type path
	assetTypePath := instance.AssetTypeName
	if assetTypePath == "" {
		respondError(w, r, http.StatusBadRequest, "Instance has no asset type (legacy instance)")
		return
	}

	// Resolve asset type path
	if !filepath.IsAbs(assetTypePath) && s.assetDir != "" {
		assetTypePath = filepath.Join(s.assetDir, assetTypePath)
	}

	// Load asset type
	assetType, err := fsm.LoadAssetType(assetTypePath)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to load asset type: %v", err))
		return
	}

	// Load FSM definition
	if err := assetType.LoadStateMachine(); err != nil {
		respondError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to load state machine: %v", err))
		return
	}

	// Create webhook dispatcher if configured
	var dispatcher *webhook.Dispatcher
	if len(assetType.Webhooks) > 0 {
		dispatcher, err = webhook.NewDispatcher(5, 100, assetType)
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to create webhook dispatcher: %v", err))
			return
		}
		defer dispatcher.Close()
	}

	// Load FSM from storage with webhook support
	f, err := fsm.LoadFromStorageWithWebhook(ctx, instanceID, assetType.StateMachineRef, instance.AssetTypeName, s.storage, dispatcher)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to load FSM: %v", err))
		return
	}

	fromState := f.CurrentState()

	// Execute transition
	if err := f.Transition(req.ToState); err != nil {
		respondError(w, r, http.StatusBadRequest, fmt.Sprintf("Transition failed: %v", err))
		return
	}

	respondJSON(w, r, http.StatusOK, TransitionResponse{
		FromState: fromState,
		ToState:   req.ToState,
		Timestamp: time.Now(),
	})
}

// handleHistory retrieves state transition history
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instanceID")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check if instance exists
	_, err := s.storage.GetInstance(ctx, instanceID)
	if err != nil {
		respondError(w, r, http.StatusNotFound, fmt.Sprintf("Instance not found: %v", err))
		return
	}

	// Parse limit parameter
	limit := 100 // default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Get transition history
	transitions, err := s.storage.GetTransitionHistory(ctx, instanceID, limit)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to get history: %v", err))
		return
	}

	// Convert to API response format
	history := make([]TransitionHistory, len(transitions))
	for i, t := range transitions {
		history[i] = TransitionHistory{
			FromState:      t.FromState,
			ToState:        t.ToState,
			TransitionedAt: t.TransitionedAt,
		}
	}

	respondJSON(w, r, http.StatusOK, HistoryResponse{
		InstanceID:  instanceID,
		Transitions: history,
	})
}

// buildAssetResponse builds a complete asset response with FSM details
func (s *Server) buildAssetResponse(ctx context.Context, instance *storage.FSMInstance) (AssetResponse, error) {
	resp := AssetResponse{
		ID:             instance.ID,
		AssetType:      instance.AssetTypeName,
		DefinitionName: instance.DefinitionName,
		CurrentState:   instance.CurrentState,
		CreatedAt:      instance.CreatedAt,
		UpdatedAt:      instance.UpdatedAt,
	}

	// Try to load FSM to get additional details
	if instance.AssetTypeName != "" {
		assetTypePath := instance.AssetTypeName
		if !filepath.IsAbs(assetTypePath) && s.assetDir != "" {
			assetTypePath = filepath.Join(s.assetDir, assetTypePath)
		}

		assetType, err := fsm.LoadAssetType(assetTypePath)
		if err == nil {
			if err := assetType.LoadStateMachine(); err == nil {
				// Load FSM to get available transitions
				f, err := fsm.LoadFromStorage(ctx, instance.ID, assetType.StateMachineRef, s.storage)
				if err == nil {
					resp.AvailableTransitions = f.AvailableTransitions()
					resp.IsFinalState = f.IsFinalState()
				}
			}
		}
	}

	return resp, nil
}
