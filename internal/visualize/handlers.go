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
	w.Header().Set("Cache-Control", "public, max-age=300") // Cache for 5 minutes
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
	if layoutStr == "" {
		layoutStr = "hierarchical"
	}
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
                    <option value="hierarchical" selected>Hierarchical</option>
                    <option value="vertical">Vertical</option>
                    <option value="horizontal">Horizontal</option>
                    <option value="circular">Circular</option>
                </select>
            </div>

            <div class="control-group checkbox-group">
                <input type="checkbox" id="history">
                <label for="history">Show History</label>
            </div>

            <div class="control-group checkbox-group">
                <input type="checkbox" id="available" checked>
                <label for="available">Show Available States</label>
            </div>

            <button onclick="refreshDiagram()">Refresh</button>
            <button onclick="downloadDiagram()">Download</button>
        </div>

        <div class="diagram-container">
            <div id="diagram">
                <div class="loading">
                    <div class="spinner"></div>
                    <div>Loading diagram...</div>
                </div>
            </div>
        </div>
    </div>

    <script>
        const instanceID = '%s';

        function refreshDiagram() {
            const format = document.getElementById('format').value;
            const layout = document.getElementById('layout').value;
            const history = document.getElementById('history').checked;
            const available = document.getElementById('available').checked;

            const params = new URLSearchParams({
                format: format,
                layout: layout,
                history: history.toString(),
                available: available.toString()
            });

            const url = '/api/v1/visualize/asset/' + instanceID + '?' + params.toString();

            const diagramDiv = document.getElementById('diagram');
            diagramDiv.innerHTML = '<div class="loading"><div class="spinner"></div><div>Loading diagram...</div></div>';

            fetch(url)
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
                        });
                    } else if (format === 'png') {
                        const img = document.createElement('img');
                        img.src = URL.createObjectURL(blob);
                        img.style.maxWidth = '100%%';
                        diagramDiv.innerHTML = '';
                        diagramDiv.appendChild(img);
                    } else {
                        // For text formats (DOT, Mermaid, JSON)
                        return blob.text().then(text => {
                            diagramDiv.innerHTML = '<pre style="text-align: left; padding: 20px; background: #f5f5f5; border-radius: 4px; overflow-x: auto;">' +
                                text.replace(/</g, '&lt;').replace(/>/g, '&gt;') + '</pre>';
                        });
                    }
                })
                .catch(error => {
                    diagramDiv.innerHTML = '<div class="error">Failed to load diagram: ' + error.message + '</div>';
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
                available: available.toString()
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
        document.getElementById('history').addEventListener('change', refreshDiagram);
        document.getElementById('available').addEventListener('change', refreshDiagram);

        // Initial load
        refreshDiagram();
    </script>
</body>
</html>`, instanceID, instanceID, instanceID)
}
