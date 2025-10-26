package web

import (
	"fmt"
	"net/http"

	"github.com/rstyczynski/fsm_v2/internal/visualize"
)

// handleOpenAPISpec serves the sg_web OpenAPI specification
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "docs/sg_web_openapi.yaml")
}

// handleAPIDocs serves a landing page with links to all services
func (s *Server) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>State Guard Web UI - Navigation</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            display: flex;
            align-items: center;
            justify-content: center;
            min-height: 100vh;
            margin: 0;
            padding: 20px;
        }
        .container {
            max-width: 900px;
            width: 100%%;
        }
        h1 {
            font-size: 48px;
            margin-bottom: 10px;
            text-align: center;
        }
        .subtitle {
            font-size: 18px;
            opacity: 0.9;
            text-align: center;
            margin-bottom: 40px;
        }
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .card {
            background: rgba(255, 255, 255, 0.1);
            border-radius: 10px;
            padding: 30px;
            backdrop-filter: blur(10px);
            border: 1px solid rgba(255, 255, 255, 0.2);
        }
        .card h2 {
            margin-top: 0;
            margin-bottom: 15px;
            font-size: 24px;
        }
        .card p {
            opacity: 0.9;
            margin-bottom: 20px;
            line-height: 1.5;
        }
        .card a {
            display: inline-block;
            background: white;
            color: #667eea;
            padding: 12px 24px;
            border-radius: 5px;
            text-decoration: none;
            font-weight: 600;
            margin-right: 10px;
            margin-bottom: 10px;
        }
        .card a:hover {
            background: #f0f0f0;
            transform: translateY(-2px);
            transition: all 0.2s;
        }
        .footer {
            text-align: center;
            opacity: 0.8;
            font-size: 14px;
            margin-top: 30px;
        }
        .badge {
            display: inline-block;
            background: rgba(255, 255, 255, 0.2);
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 12px;
            margin-left: 10px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>State Guard Services</h1>
        <p class="subtitle">Finite State Machine Management Platform</p>

        <div class="grid">
            <!-- sg_api Card -->
            <div class="card">
                <h2>sg_api <span class="badge">Port 8080</span></h2>
                <p>Main API server for asset management, state transitions, and data operations.</p>
                <a href="%s/docs">API Docs</a>
                <a href="%s/api/v1/health">Health</a>
            </div>

            <!-- sg_web API Card -->
            <div class="card">
                <h2>sg_web API <span class="badge">Port 3000</span></h2>
                <p>Visualization API with WASM isolation for rendering state diagrams.</p>
                <a href="/swagger">API Docs</a>
                <a href="/health">Health</a>
            </div>

            <!-- Interactive UI Card -->
            <div class="card" style="grid-column: span 2;">
                <h2>Interactive Diagram Viewer</h2>
                <p>View live FSM state diagrams with current state highlighting and transition history. Enter your asset instance ID:</p>
                <form onsubmit="window.location='/docs/diagram/' + document.getElementById('instanceId').value; return false;" style="margin-bottom: 10px;">
                    <input type="text" id="instanceId" placeholder="e.g., web-server-1"
                           style="padding: 10px; border-radius: 5px; border: none; width: 200px; margin-right: 10px;">
                    <button type="submit" style="padding: 10px 20px; border-radius: 5px; border: none; background: white; color: #667eea; font-weight: 600; cursor: pointer;">
                        View Diagram
                    </button>
                </form>
            </div>
        </div>

        <div class="footer">
            <p>sg_web renders visualizations locally with WASM isolation • Data from sg_api</p>
            <p><strong>Architecture:</strong> sg_api (data) + sg_web (visualization) = isolated, scalable FSM platform</p>
        </div>
    </div>
</body>
</html>`, s.apiURL, s.apiURL)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// handleDiagramPage delegates to visualization handler
func (s *Server) handleDiagramPage(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.apiURL)
	handler.HandleDiagramPage(w, r)
}

// handleVisualizeAsset delegates to visualization handler
func (s *Server) handleVisualizeAsset(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.apiURL)
	handler.HandleVisualizeAsset(w, r)
}

// handleVisualizeDefinition delegates to visualization handler
func (s *Server) handleVisualizeDefinition(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.apiURL)
	handler.HandleVisualizeDefinition(w, r)
}

// handleVisualizeHistory delegates to visualization handler
func (s *Server) handleVisualizeHistory(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.apiURL)
	handler.HandleVisualizeHistory(w, r)
}

// handleSwaggerUI serves the Swagger UI for sg_web visualization API
func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>sg_web Visualization API - Swagger UI</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui.css">
    <style>
        html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin: 0; padding: 0; }
        .topbar { display: none; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: "/openapi.yaml",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                defaultModelsExpandDepth: 1,
                defaultModelExpandDepth: 1,
                docExpansion: "list",
                filter: true,
                showRequestHeaders: true,
                supportedSubmitMethods: ['get', 'post', 'put', 'delete', 'patch']
            });
            window.ui = ui;
        };
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
