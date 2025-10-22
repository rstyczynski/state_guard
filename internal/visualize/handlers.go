package visualize

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rstyczynski/fsm_v2/internal/fsm"
	"github.com/rstyczynski/fsm_v2/internal/storage"
)

// Handler provides HTTP handlers for visualization endpoints
type Handler struct {
	storage  storage.Storage
	assetDir string
}

// NewHandler creates a new visualization handler
func NewHandler(store storage.Storage, assetDir string) *Handler {
	return &Handler{
		storage:  store,
		assetDir: assetDir,
	}
}

// HandleVisualizeAsset generates visualization for a specific asset instance
// GET /api/v1/visualize/asset/:instanceID
func (h *Handler) HandleVisualizeAsset(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instanceID")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	format, layout, opts, err := h.parseOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Load instance
	instance, err := h.storage.GetInstance(ctx, instanceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Instance not found: %v", err), http.StatusNotFound)
		return
	}

	// Load asset type and FSM definition
	generator, err := h.loadGenerator(instance.AssetTypeName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load FSM definition: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate visualization
	opts.Format = format
	opts.Layout = layout
	data, err := generator.GenerateInstance(ctx, instanceID, opts)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate visualization: %v", err), http.StatusInternalServerError)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", ContentType(format))
	// No caching for instance visualizations since they change with transitions
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// HandleVisualizeDefinition generates visualization for FSM definition only
// GET /api/v1/visualize/definition/:name
func (h *Handler) HandleVisualizeDefinition(w http.ResponseWriter, r *http.Request) {
	definitionName := chi.URLParam(r, "name")

	// Parse query parameters
	format, layout, opts, err := h.parseOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Load asset type by name
	assetTypePath := definitionName
	if !filepath.IsAbs(assetTypePath) && h.assetDir != "" {
		assetTypePath = filepath.Join(h.assetDir, assetTypePath)
	}

	generator, err := h.loadGenerator(assetTypePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load FSM definition: %v", err), http.StatusNotFound)
		return
	}

	// Generate visualization
	opts.Format = format
	opts.Layout = layout
	opts.HighlightCurrent = false // Definition only, no instance highlighting
	data, err := generator.GenerateDefinition(opts)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate visualization: %v", err), http.StatusInternalServerError)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", ContentType(format))
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour (definitions don't change)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// HandleVisualizeHistory generates animated visualization of state progression
// GET /api/v1/visualize/asset/:instanceID/history
func (h *Handler) HandleVisualizeHistory(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instanceID")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	format, layout, opts, err := h.parseOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Force history display
	opts.ShowHistory = true

	// Load instance
	instance, err := h.storage.GetInstance(ctx, instanceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Instance not found: %v", err), http.StatusNotFound)
		return
	}

	// Load generator
	generator, err := h.loadGenerator(instance.AssetTypeName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load FSM definition: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate visualization with history
	opts.Format = format
	opts.Layout = layout
	data, err := generator.GenerateInstance(ctx, instanceID, opts)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate visualization: %v", err), http.StatusInternalServerError)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", ContentType(format))
	w.Header().Set("Cache-Control", "public, max-age=60") // Shorter cache for history
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// HandleDiagramPage serves interactive HTML wrapper
// GET /docs/diagram/:instanceID
func (h *Handler) HandleDiagramPage(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instanceID")

	// Generate HTML with embedded SVG viewer
	html := h.generateHTML(instanceID)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// parseOptions parses query parameters into visualization options
func (h *Handler) parseOptions(r *http.Request) (Format, Layout, Options, error) {
	opts := Options{
		ColorScheme:  "light",
		ShowHistory:  false,
		ShowAvailable: false,
	}

	// Parse format
	formatStr := r.URL.Query().Get("format")
	if formatStr == "" {
		formatStr = "svg"
	}
	format, err := ParseFormat(formatStr)
	if err != nil {
		return "", "", opts, err
	}

	// Parse layout
	layoutStr := r.URL.Query().Get("layout")
	// If not specified, ParseLayout will default to horizontal
	layout, err := ParseLayout(layoutStr)
	if err != nil {
		return "", "", opts, err
	}

	// Parse boolean options
	if r.URL.Query().Get("history") == "true" {
		opts.ShowHistory = true
	}
	if r.URL.Query().Get("available") == "true" {
		opts.ShowAvailable = true
	}

	// Parse color scheme
	if scheme := r.URL.Query().Get("scheme"); scheme != "" {
		opts.ColorScheme = scheme
	}

	// Parse highlight_state for timeline slider (overrides actual current state)
	if highlightState := r.URL.Query().Get("highlight_state"); highlightState != "" {
		opts.HighlightCurrent = true
		opts.CurrentState = highlightState
	}

	return format, layout, opts, nil
}

// loadGenerator loads a visualization generator for an asset type
func (h *Handler) loadGenerator(assetTypeName string) (*Generator, error) {
	// Resolve asset type path
	assetTypePath := assetTypeName
	if !filepath.IsAbs(assetTypePath) && h.assetDir != "" {
		assetTypePath = filepath.Join(h.assetDir, assetTypePath)
	}

	// Load asset type
	assetType, err := fsm.LoadAssetType(assetTypePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load asset type: %w", err)
	}

	// Load FSM definition
	if err := assetType.LoadStateMachine(); err != nil {
		return nil, fmt.Errorf("failed to load state machine: %w", err)
	}

	return NewGenerator(assetType.StateMachineRef, h.storage), nil
}

// generateHTML generates the interactive HTML wrapper
func (h *Handler) generateHTML(instanceID string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>FSM Diagram - %s</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 20px 30px;
        }
        .header h1 {
            font-size: 24px;
            margin-bottom: 5px;
        }
        .header p {
            opacity: 0.9;
            font-size: 14px;
        }
        .controls {
            padding: 20px 30px;
            background: #fafafa;
            border-bottom: 1px solid #e0e0e0;
            display: flex;
            gap: 15px;
            flex-wrap: wrap;
            align-items: center;
        }
        .control-group {
            display: flex;
            gap: 8px;
            align-items: center;
        }
        label {
            font-size: 14px;
            font-weight: 500;
            color: #555;
        }
        select, button {
            padding: 8px 12px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 14px;
            cursor: pointer;
            background: white;
        }
        button {
            background: #667eea;
            color: white;
            border: none;
            font-weight: 500;
            transition: background 0.2s;
        }
        button:hover {
            background: #5568d3;
        }
        .checkbox-group {
            display: flex;
            gap: 5px;
            align-items: center;
        }
        .checkbox-group input {
            cursor: pointer;
        }
        .diagram-container {
            padding: 30px;
            overflow: auto;
            min-height: 500px;
            display: flex;
            justify-content: center;
            align-items: center;
        }
        #diagram {
            max-width: 100%%;
            height: auto;
        }
        .loading {
            text-align: center;
            padding: 40px;
            color: #999;
        }
        .spinner {
            border: 3px solid #f3f3f3;
            border-top: 3px solid #667eea;
            border-radius: 50%%;
            width: 40px;
            height: 40px;
            animation: spin 1s linear infinite;
            margin: 0 auto 15px;
        }
        @keyframes spin {
            0%% { transform: rotate(0deg); }
            100%% { transform: rotate(360deg); }
        }
        .error {
            background: #fee;
            color: #c33;
            padding: 15px;
            margin: 20px;
            border-radius: 4px;
            border-left: 4px solid #c33;
        }
        /* State metadata modal */
        .metadata-panel {
            position: fixed;
            right: -400px;
            top: 0;
            bottom: 0;
            width: 400px;
            background: white;
            box-shadow: -2px 0 10px rgba(0,0,0,0.1);
            transition: right 0.3s ease;
            z-index: 1000;
            overflow-y: auto;
        }
        .metadata-panel.open {
            right: 0;
        }
        .metadata-header {
            background: #667eea;
            color: white;
            padding: 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .metadata-content {
            padding: 20px;
        }
        .metadata-item {
            margin-bottom: 15px;
        }
        .metadata-label {
            font-weight: 600;
            color: #666;
            font-size: 12px;
            text-transform: uppercase;
            margin-bottom: 5px;
        }
        .metadata-value {
            font-size: 14px;
            color: #333;
        }
        .close-btn {
            background: none;
            border: none;
            color: white;
            font-size: 24px;
            cursor: pointer;
            padding: 0;
        }
        /* Zoom controls in command bar */
        .zoom-controls {
            display: inline-flex;
            align-items: center;
            gap: 5px;
            margin-left: 15px;
        }
        .zoom-btn {
            padding: 6px 10px;
            font-size: 14px;
            border: 1px solid #ddd;
            background: white;
            color: #333;
            cursor: pointer;
            border-radius: 3px;
            transition: all 0.2s;
        }
        .zoom-btn:hover {
            background: #f0f0f0;
            border-color: #999;
        }
        .zoom-level {
            font-size: 13px;
            font-weight: 600;
            color: #333;
            margin-left: 5px;
        }
        /* Timeline slider */
        .timeline-container {
            padding: 15px 30px;
            background: #fafafa;
            border-top: 1px solid #e0e0e0;
            display: none;
        }
        .timeline-container.active {
            display: block;
        }
        .timeline-slider {
            width: 100%%;
            margin: 10px 0;
        }
        .timeline-info {
            display: flex;
            justify-content: space-between;
            font-size: 12px;
            color: #666;
        }
        .state-clickable {
            cursor: pointer;
        }
        .state-clickable:hover {
            opacity: 0.8;
        }
        .diagram-wrapper {
            position: relative;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>FSM State Diagram</h1>
            <p>Instance: <strong>%s</strong></p>
        </div>

        <div class="controls">
            <div class="control-group">
                <label for="format">Format:</label>
                <select id="format">
                    <option value="svg" selected>SVG</option>
                    <option value="png">PNG</option>
                    <option value="dot">DOT</option>
                    <option value="mermaid">Mermaid</option>
                    <option value="json">JSON</option>
                </select>
            </div>

            <div class="control-group">
                <label for="layout">Layout:</label>
                <select id="layout">
                    <option value="horizontal" selected>Horizontal (default)</option>
                    <option value="vertical">Vertical</option>
                </select>
            </div>

            <div class="control-group checkbox-group">
                <input type="checkbox" id="history">
                <label for="history">Show History</label>
            </div>

            <div class="control-group checkbox-group">
                <input type="checkbox" id="available" checked>
                <label for="available">Indicate Next States</label>
            </div>

            <div class="zoom-controls">
                <button class="zoom-btn" onclick="zoomIn()" title="Zoom in">+</button>
                <button class="zoom-btn" onclick="zoomOut()" title="Zoom out">−</button>
                <button class="zoom-btn" onclick="resetZoom()" title="Reset zoom">⊙</button>
                <span class="zoom-level" id="zoom-level">100%%</span>
            </div>

            <button onclick="refreshDiagram()">Refresh</button>
            <button onclick="downloadDiagram()">Download</button>
        </div>

        <div class="timeline-container" id="timeline-container">
            <div class="timeline-info">
                <span id="timeline-state">State: <strong>-</strong></span>
                <span id="timeline-time">Time: <strong>-</strong></span>
            </div>
            <input type="range" id="timeline-slider" class="timeline-slider" min="0" max="100" value="0">
        </div>

        <div class="diagram-container">
            <div class="diagram-wrapper" id="diagram-wrapper">
                <div id="diagram">
                    <div class="loading">
                        <div class="spinner"></div>
                        <div>Loading diagram...</div>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <!-- Metadata panel -->
    <div class="metadata-panel" id="metadata-panel">
        <div class="metadata-header">
            <h2 id="metadata-title">State Information</h2>
            <button class="close-btn" onclick="closeMetadata()">&times;</button>
        </div>
        <div class="metadata-content" id="metadata-content">
            <p>Click a state to view information</p>
        </div>
    </div>

    <script>
        const instanceID = '%s';
        let currentAsset = null;
        let historyData = [];
        let zoomLevel = 1;
        let svgElement = null;

        // Pan and zoom state
        let isPanning = false;
        let startPoint = { x: 0, y: 0 };
        let panOffset = { x: 0, y: 0 };

        async function loadAssetData() {
            try {
                const response = await fetch('/api/v1/assets/' + instanceID);
                if (response.ok) {
                    currentAsset = await response.json();
                }
            } catch (error) {
                console.error('Failed to load asset data:', error);
            }
        }

        async function loadHistoryData() {
            try {
                const response = await fetch('/api/v1/assets/' + instanceID + '/history');
                if (response.ok) {
                    const data = await response.json();
                    const transitions = (data.transitions || []).reverse();

                    // Convert transitions to states array (including initial state)
                    historyData = [];
                    if (transitions.length > 0) {
                        // Add initial state (from_state of first transition)
                        historyData.push({
                            to_state: transitions[0].from_state,
                            transitioned_at: null  // Initial state has no transition time
                        });
                        // Add all subsequent states (to_state of each transition)
                        transitions.forEach(t => {
                            historyData.push({
                                to_state: t.to_state,
                                transitioned_at: t.transitioned_at
                            });
                        });
                    }
                    updateTimeline();
                }
            } catch (error) {
                console.error('Failed to load history:', error);
            }
        }

        function refreshDiagram(highlightState) {
            const format = document.getElementById('format').value;
            const layout = document.getElementById('layout').value;
            const history = document.getElementById('history').checked;
            const available = document.getElementById('available').checked;

            const params = new URLSearchParams({
                format: format,
                layout: layout,
                history: history.toString(),
                available: available.toString(),
                _t: new Date().getTime() // Cache-busting timestamp
            });

            // Add highlight_state parameter for timeline slider
            if (highlightState) {
                params.set('highlight_state', highlightState);
            }

            const url = '/api/v1/visualize/asset/' + instanceID + '?' + params.toString();
            const diagramDiv = document.getElementById('diagram');
            diagramDiv.innerHTML = '<div class="loading"><div class="spinner"></div><div>Loading diagram...</div></div>';

            return fetch(url, {
                cache: 'no-store' // Force no caching on fetch
            })
                .then(response => {
                    if (!response.ok) {
                        throw new Error('HTTP ' + response.status + ': ' + response.statusText);
                    }
                    return response.blob();
                })
                .then(blob => {
                    if (format === 'svg') {
                        return blob.text().then(svg => {
                            diagramDiv.innerHTML = svg;
                            enableInteractivity();
                            applyZoom(); // Restore zoom level after SVG reload
                            if (!highlightState) {
                                resetTimelinePosition();
                            }
                        });
                    } else if (format === 'png') {
                        // Clear old SVG reference
                        svgElement = null;

                        const img = document.createElement('img');
                        img.src = URL.createObjectURL(blob);
                        img.style.maxWidth = '100%%';
                        img.style.transformOrigin = 'center';
                        img.style.cursor = 'grab';
                        img.style.userSelect = 'none';
                        diagramDiv.innerHTML = '';
                        diagramDiv.appendChild(img);

                        // Store img reference for zoom/pan
                        svgElement = img;

                        // Enable pan for PNG
                        img.addEventListener('mousedown', startPan);
                        img.addEventListener('mousemove', pan);
                        img.addEventListener('mouseup', endPan);
                        img.addEventListener('mouseleave', endPan);

                        // Apply zoom to PNG
                        applyZoom();

                        if (!highlightState) {
                            resetTimelinePosition();
                        }
                    } else {
                        return blob.text().then(text => {
                            diagramDiv.innerHTML = '<pre style="text-align: left; padding: 20px; background: #f5f5f5; border-radius: 4px; overflow-x: auto;">' +
                                text.replace(/</g, '&lt;').replace(/>/g, '&gt;') + '</pre>';
                            if (!highlightState) {
                                resetTimelinePosition();
                            }
                        });
                    }
                })
                .catch(error => {
                    diagramDiv.innerHTML = '<div class="error">Failed to load diagram: ' + error.message + '</div>';
                });
        }

        function resetTimelinePosition() {
            const slider = document.getElementById('timeline-slider');
            if (slider && historyData && historyData.length > 0) {
                slider.value = historyData.length - 1;
                const entry = historyData[historyData.length - 1];
                document.getElementById('timeline-state').innerHTML =
                    'State: <strong>' + entry.to_state + '</strong>';

                // Handle initial state (no transition timestamp)
                if (entry.transitioned_at) {
                    document.getElementById('timeline-time').innerHTML =
                        'Time: <strong>' + new Date(entry.transitioned_at).toLocaleString() + '</strong>';
                } else {
                    document.getElementById('timeline-time').innerHTML =
                        'Time: <strong>Initial state</strong>';
                }
            }
        }

        function enableInteractivity() {
            svgElement = document.querySelector('#diagram svg');
            if (!svgElement) return;

            // Set cursor for panning
            svgElement.style.cursor = 'grab';

            // Enable pan with mouse drag
            svgElement.addEventListener('mousedown', startPan);
            svgElement.addEventListener('mousemove', pan);
            svgElement.addEventListener('mouseup', endPan);
            svgElement.addEventListener('mouseleave', endPan);

            // Add click handlers to state nodes
            const stateNodes = svgElement.querySelectorAll('g.node');
            stateNodes.forEach(node => {
                const title = node.querySelector('title');
                if (!title) return;

                const stateName = title.textContent.trim();
                if (stateName.startsWith('leg_') || stateName === 'ANY_STATE') return;

                // Make state clickable
                const polygon = node.querySelector('polygon, ellipse, circle');
                if (polygon) {
                    polygon.classList.add('state-clickable');
                    polygon.style.cursor = 'pointer';
                }

                node.addEventListener('click', (e) => {
                    e.stopPropagation();
                    handleStateClick(stateName);
                });
            });
        }

        async function handleStateClick(stateName) {
            // Show metadata panel
            showMetadata(stateName);

            // If state is available for transition, offer to transition to it
            if (currentAsset && currentAsset.current_state !== stateName) {
                // Check if this is a valid transition
                const isAvailable = await checkTransitionAvailability(stateName);
                if (isAvailable && confirm('Transition to ' + stateName + '?')) {
                    await transitionToState(stateName);
                }
            }
        }

        async function checkTransitionAvailability(toState) {
            if (!currentAsset) return false;
            // Simple check - in real implementation, query the FSM for valid transitions
            return true;
        }

        async function transitionToState(toState) {
            try {
                const response = await fetch('/api/v1/assets/' + instanceID + '/transition', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ to_state: toState })
                });

                if (response.ok) {
                    await loadAssetData();
                    await loadHistoryData();
                    refreshDiagram();
                } else {
                    alert('Transition failed: ' + response.statusText);
                }
            } catch (error) {
                alert('Transition error: ' + error.message);
            }
        }

        function showMetadata(stateName) {
            const panel = document.getElementById('metadata-panel');
            const title = document.getElementById('metadata-title');
            const content = document.getElementById('metadata-content');

            title.textContent = 'State: ' + stateName;

            let html = '<div class="metadata-item">';
            html += '<div class="metadata-label">State Name</div>';
            html += '<div class="metadata-value">' + stateName + '</div>';
            html += '</div>';

            if (currentAsset) {
                html += '<div class="metadata-item">';
                html += '<div class="metadata-label">Current State</div>';
                html += '<div class="metadata-value">' + (currentAsset.current_state === stateName ? 'Yes' : 'No') + '</div>';
                html += '</div>';

                html += '<div class="metadata-item">';
                html += '<div class="metadata-label">Asset ID</div>';
                html += '<div class="metadata-value">' + currentAsset.id + '</div>';
                html += '</div>';

                html += '<div class="metadata-item">';
                html += '<div class="metadata-label">Asset Type</div>';
                html += '<div class="metadata-value">' + currentAsset.asset_type + '</div>';
                html += '</div>';
            }

            content.innerHTML = html;
            panel.classList.add('open');
        }

        function closeMetadata() {
            document.getElementById('metadata-panel').classList.remove('open');
        }

        // Pan and zoom functions
        function zoomIn() {
            zoomLevel *= 1.2;
            applyZoom();
            updateZoomDisplay();
        }

        function zoomOut() {
            zoomLevel /= 1.2;
            applyZoom();
            updateZoomDisplay();
        }

        function resetZoom() {
            zoomLevel = 1;
            panOffset = { x: 0, y: 0 };
            applyZoom();
            updateZoomDisplay();
        }

        function applyZoom() {
            // Apply zoom to current diagram element (SVG or IMG)
            if (!svgElement) return;
            svgElement.style.transform = 'scale(' + zoomLevel + ') translate(' + panOffset.x + 'px, ' + panOffset.y + 'px)';
        }

        function updateZoomDisplay() {
            const zoomPercent = Math.round(zoomLevel * 100);
            const zoomLevelEl = document.getElementById('zoom-level');
            if (zoomLevelEl) {
                zoomLevelEl.textContent = zoomPercent + '%%';
            }
        }

        function startPan(e) {
            // For SVG, skip if clicking on a clickable state
            if (svgElement && svgElement.tagName === 'svg' && e.target.classList.contains('state-clickable')) {
                return;
            }
            isPanning = true;
            startPoint = { x: e.clientX - panOffset.x, y: e.clientY - panOffset.y };
            // Change cursor to grabbing
            if (svgElement) {
                svgElement.style.cursor = 'grabbing';
            }
        }

        function pan(e) {
            if (!isPanning) return;
            e.preventDefault();
            panOffset = {
                x: e.clientX - startPoint.x,
                y: e.clientY - startPoint.y
            };
            applyZoom();
        }

        function endPan() {
            isPanning = false;
            // Restore cursor to grab
            if (svgElement) {
                svgElement.style.cursor = 'grab';
            }
        }

        // Timeline functions
        function updateTimeline() {
            if (!historyData || historyData.length === 0) {
                document.getElementById('timeline-container').classList.remove('active');
                return;
            }

            document.getElementById('timeline-container').classList.add('active');
            const slider = document.getElementById('timeline-slider');
            slider.max = historyData.length - 1;
            slider.value = historyData.length - 1;

            // Remove old listener by cloning
            const newSlider = slider.cloneNode(true);
            slider.parentNode.replaceChild(newSlider, slider);

            newSlider.addEventListener('input', (e) => {
                const index = parseInt(e.target.value);
                if (historyData[index]) {
                    const entry = historyData[index];
                    document.getElementById('timeline-state').innerHTML =
                        'State: <strong>' + entry.to_state + '</strong>';

                    // Handle initial state (no transition timestamp)
                    if (entry.transitioned_at) {
                        document.getElementById('timeline-time').innerHTML =
                            'Time: <strong>' + new Date(entry.transitioned_at).toLocaleString() + '</strong>';
                    } else {
                        document.getElementById('timeline-time').innerHTML =
                            'Time: <strong>Initial state</strong>';
                    }

                    // Re-render diagram with historical state highlighted by server
                    refreshDiagram(entry.to_state);
                }
            });
        }

        function downloadDiagram() {
            const format = document.getElementById('format').value;
            const layout = document.getElementById('layout').value;
            const history = document.getElementById('history').checked;
            const available = document.getElementById('available').checked;

            const params = new URLSearchParams({
                format: format,
                layout: layout,
                history: history.toString(),
                available: available.toString(),
                _t: new Date().getTime() // Cache-busting
            });

            const url = '/api/v1/visualize/asset/' + instanceID + '?' + params.toString();
            const a = document.createElement('a');
            a.href = url;
            a.download = 'fsm-diagram-' + instanceID + '.' + format;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
        }

        // Auto-refresh on control change
        document.getElementById('format').addEventListener('change', refreshDiagram);
        document.getElementById('layout').addEventListener('change', refreshDiagram);
        document.getElementById('history').addEventListener('change', () => {
            refreshDiagram();
            loadHistoryData();
        });
        document.getElementById('available').addEventListener('change', refreshDiagram);

        // Initial load
        loadAssetData().then(() => {
            loadHistoryData();
            refreshDiagram();
        });
    </script>
</body>
</html>`, instanceID, instanceID, instanceID)
}
