package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/rstyczynski/fsm_v2/internal/fsm"
	"github.com/rstyczynski/fsm_v2/internal/storage"
	"github.com/rstyczynski/fsm_v2/internal/webhook"
)

// Server represents the API server
type Server struct {
	storage     storage.Storage
	httpServer  *http.Server
	assetDir    string // Directory containing asset type YAML files
	webURL      string // URL of the sg_web server (for home page navigation)
	dispatchers map[string]*webhook.Dispatcher // Asset type path → dispatcher
	dispMu      sync.RWMutex
}

// NewServer creates a new API server
func NewServer(store storage.Storage, assetDir string) *Server {
	return &Server{
		storage:     store,
		assetDir:    assetDir,
		webURL:      "http://localhost:3000", // Default web URL
		dispatchers: make(map[string]*webhook.Dispatcher),
	}
}

// SetWebURL sets the web UI server URL
func (s *Server) SetWebURL(url string) {
	s.webURL = url
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
		r.Get("/v1/openapi.yaml", s.handleOpenAPISpec)
		r.Get("/docs/openapi.yaml", s.handleOpenAPISpec) // Backward compatibility

		// Home page - unified navigation
		r.Get("/", s.handleHome)

		// API documentation page (Swagger UI)
		r.Get("/v1/docs", s.handleSwaggerUI)
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

// getOrCreateDispatcher gets or creates a webhook dispatcher for an asset type
func (s *Server) getOrCreateDispatcher(assetTypePath string, assetType *fsm.AssetType) (*webhook.Dispatcher, error) {
	// Fast path: read lock
	s.dispMu.RLock()
	if disp, exists := s.dispatchers[assetTypePath]; exists {
		s.dispMu.RUnlock()
		return disp, nil
	}
	s.dispMu.RUnlock()

	// Slow path: write lock
	s.dispMu.Lock()
	defer s.dispMu.Unlock()

	// Double-check after acquiring write lock
	if disp, exists := s.dispatchers[assetTypePath]; exists {
		return disp, nil
	}

	// Create new dispatcher
	disp, err := webhook.NewDispatcher(5, 100, assetType)
	if err != nil {
		return nil, err
	}

	s.dispatchers[assetTypePath] = disp
	log.Printf("Created webhook dispatcher for asset type: %s", assetTypePath)
	return disp, nil
}


// ShutdownWithContext gracefully shuts down the server with provided context
func (s *Server) ShutdownWithContext(ctx context.Context) error {
	// Close all webhook dispatchers
	s.dispMu.Lock()
	for assetType, disp := range s.dispatchers {
		log.Printf("Shutting down webhook dispatcher for: %s", assetType)
		if err := disp.Close(); err != nil {
			log.Printf("Error closing dispatcher for %s: %v", assetType, err)
		}
	}
	s.dispatchers = make(map[string]*webhook.Dispatcher)
	s.dispMu.Unlock()

	// Shutdown HTTP server
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// Shutdown gracefully shuts down the server (convenience wrapper)
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.ShutdownWithContext(ctx)
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

// handleHome serves the unified navigation home page
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>State Guard Services - Navigation</title>
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
        input[type="text"] {
            padding: 10px;
            border-radius: 5px;
            border: none;
            width: 200px;
            margin-right: 10px;
        }
        button {
            padding: 10px 20px;
            border-radius: 5px;
            border: none;
            background: white;
            color: #667eea;
            font-weight: 600;
            cursor: pointer;
        }
        button:hover {
            background: #f0f0f0;
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
                <a href="/v1/docs">API Docs</a>
                <a href="/api/v1/health">Health</a>
            </div>

            <!-- sg_web API Card -->
            <div class="card">
                <h2>sg_web API <span class="badge">Port 3000</span></h2>
                <p>Visualization API with WASM isolation for rendering state diagrams.</p>
                <a href="%s/v1/docs">API Docs</a>
                <a href="%s/health">Health</a>
            </div>

            <!-- Interactive UI Card -->
            <div class="card" style="grid-column: span 2;">
                <h2>Interactive Diagram Viewer</h2>
                <p>View live FSM state diagrams with current state highlighting and transition history. Enter your asset instance ID:</p>
                <form onsubmit="window.location='%s/console/' + document.getElementById('instanceId').value; return false;" style="margin-bottom: 10px;">
                    <input type="text" id="instanceId" placeholder="e.g., web-server-1">
                    <button type="submit">View Diagram</button>
                </form>
            </div>
        </div>

        <div class="footer">
            <p>sg_web renders visualizations locally with WASM isolation • Data from sg_api</p>
            <p><strong>Architecture:</strong> sg_api (data) + sg_web (visualization) = isolated, scalable FSM platform</p>
        </div>
    </div>
</body>
</html>`, s.webURL, s.webURL, s.webURL)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// handleSwaggerUI serves the API documentation page with Swagger UI
func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>State Guard API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; }
        .info-banner {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 15px 30px;
            text-align: center;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
        }
        .info-banner h2 { margin: 0 0 10px 0; }
        .info-banner p { margin: 5px 0; font-size: 14px; }
        .info-banner a { color: white; text-decoration: underline; }
    </style>
</head>
<body>
    <div class="info-banner">
        <h2>State Guard API Server</h2>
        <p>This is the DATA API for asset management (CRUD operations)</p>
        <p><strong>For Web UI and Visualization features (Zoom, Timeline, History, etc.), use sg_web</strong></p>
        <p><a href="/">← Back to Home</a></p>
    </div>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
                url: "/v1/openapi.yaml",
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

