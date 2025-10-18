package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/rstyczynski/fsm_v2/internal/storage"
)

// Server represents the API server
type Server struct {
	storage    storage.Storage
	httpServer *http.Server
	assetDir   string // Directory containing asset type YAML files
}

// NewServer creates a new API server
func NewServer(store storage.Storage, assetDir string) *Server {
	return &Server{
		storage:  store,
		assetDir: assetDir,
	}
}

// Start starts the HTTP server with Chi router
func (s *Server) Start(addr string) error {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(s.corsMiddleware)

	// Documentation routes (served without JSON content-type)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequestID)
		r.Use(middleware.RealIP)
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		r.Use(s.corsMiddleware)

		// Serve OpenAPI spec
		r.Get("/openapi.yaml", s.handleOpenAPISpec)
		r.Get("/docs/openapi.yaml", s.handleOpenAPISpec)

		// API documentation page
		r.Get("/docs", s.handleAPIDocs)
		r.Get("/", s.handleAPIDocs) // Root redirects to docs
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Health check
		r.Get("/health", s.handleHealth)

		// Asset routes
		r.Route("/assets", func(r chi.Router) {
			r.Get("/", s.handleListAssets)
			r.Post("/", s.handleCreateAsset)

			r.Route("/{instanceID}", func(r chi.Router) {
				r.Get("/", s.handleGetAsset)
				r.Delete("/", s.handleDeleteAsset)
				r.Post("/transition", s.handleTransition)
				r.Get("/history", s.handleHistory)
			})
		})
	})

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting API server on %s", addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// respondJSON sends a JSON response using chi/render
func respondJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	render.Status(r, status)
	render.JSON(w, r, data)
}

// respondError sends an error response
func respondError(w http.ResponseWriter, r *http.Request, status int, message string) {
	render.Status(r, status)
	render.JSON(w, r, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
	})
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Check storage health
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	_, err := s.storage.ListInstances(ctx)
	status := "healthy"
	if err != nil {
		status = fmt.Sprintf("unhealthy: %v", err)
	}

	respondJSON(w, r, http.StatusOK, HealthResponse{
		Status:    status,
		Version:   "2.2.0",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// handleOpenAPISpec serves the OpenAPI specification file
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
    <title>FSM API Documentation</title>
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
