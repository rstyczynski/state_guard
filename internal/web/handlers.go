package web

import (
	"fmt"
	"net/http"

	"github.com/rstyczynski/fsm_v2/internal/visualize"
)

// handleOpenAPISpec serves the OpenAPI specification
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "docs/openapi.yaml")
}

// handleAPIDocs serves a simple page that redirects to sg_api for full docs
func (s *Server) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>State Guard Web UI</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            display: flex;
            align-items: center;
            justify-content: center;
            height: 100vh;
            margin: 0;
            text-align: center;
        }
        .container {
            max-width: 600px;
            padding: 40px;
        }
        h1 { font-size: 48px; margin-bottom: 20px; }
        p { font-size: 18px; opacity: 0.9; margin-bottom: 30px; }
        a {
            display: inline-block;
            background: white;
            color: #667eea;
            padding: 15px 30px;
            border-radius: 5px;
            text-decoration: none;
            font-weight: 600;
            margin: 10px;
        }
        a:hover { background: #f0f0f0; }
        .note { font-size: 14px; opacity: 0.8; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>State Guard Web UI</h1>
        <p>Visualization server with WASM isolation for diagram rendering.</p>
        <a href="%s/docs">API Documentation</a>
        <a href="%s/api/v1/health">API Health</a>
        <p class="note">Visualizations rendered here • Data from sg_api</p>
    </div>
</body>
</html>`, s.apiURL, s.apiURL)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// handleDiagramPage delegates to visualization handler
func (s *Server) handleDiagramPage(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.assetDir, s.apiURL)
	handler.HandleDiagramPage(w, r)
}

// handleVisualizeAsset delegates to visualization handler
func (s *Server) handleVisualizeAsset(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.assetDir, s.apiURL)
	handler.HandleVisualizeAsset(w, r)
}

// handleVisualizeDefinition delegates to visualization handler
func (s *Server) handleVisualizeDefinition(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.assetDir, s.apiURL)
	handler.HandleVisualizeDefinition(w, r)
}

// handleVisualizeHistory delegates to visualization handler
func (s *Server) handleVisualizeHistory(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.assetDir, s.apiURL)
	handler.HandleVisualizeHistory(w, r)
}
