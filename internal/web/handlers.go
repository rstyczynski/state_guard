package web

import (
	"net/http"

	"github.com/rstyczynski/fsm_v2/internal/visualize"
)

// handleOpenAPISpec serves the OpenAPI specification
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "docs/openapi.yaml")
}

// handleAPIDocs serves the API documentation page with Swagger UI
func (s *Server) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>State Guard API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
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
                layout: "StandaloneLayout"
            });
        };
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// handleDiagramPage delegates to visualization handler for full interactive diagram
func (s *Server) handleDiagramPage(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.assetDir)
	handler.HandleDiagramPage(w, r)
}

// handleVisualizeAsset delegates to visualization handler
func (s *Server) handleVisualizeAsset(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.assetDir)
	handler.HandleVisualizeAsset(w, r)
}

// handleVisualizeDefinition delegates to visualization handler
func (s *Server) handleVisualizeDefinition(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.assetDir)
	handler.HandleVisualizeDefinition(w, r)
}

// handleVisualizeHistory delegates to visualization handler
func (s *Server) handleVisualizeHistory(w http.ResponseWriter, r *http.Request) {
	handler := visualize.NewHandler(s.storage, s.assetDir)
	handler.HandleVisualizeHistory(w, r)
}
