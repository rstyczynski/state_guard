package web

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rstyczynski/fsm_v2/internal/storage"
)

// Server represents the Web UI server
type Server struct {
	httpServer *http.Server
	apiURL     string // URL of the sg_api backend (optional, for hybrid mode)
	storage    storage.Storage
	assetDir   string
}

// NewServer creates a new Web UI server with storage and asset directory
func NewServer(store storage.Storage, assetDir string, apiURL string) *Server {
	return &Server{
		storage:  store,
		assetDir: assetDir,
		apiURL:   apiURL,
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
	r.Use(s.corsMiddleware)

	// Web UI routes (HTML pages)
	r.Get("/", s.handleAPIDocs)
	r.Get("/docs", s.handleAPIDocs)
	r.Get("/docs/diagram/{instanceID}", s.handleDiagramPage)
	r.Get("/openapi.yaml", s.handleOpenAPISpec)
	r.Get("/docs/openapi.yaml", s.handleOpenAPISpec)

	// Health check
	r.Get("/health", s.handleHealth)

	// Visualization routes (moved from sg_api)
	r.Route("/api/v1/visualize", func(r chi.Router) {
		// Definition visualization
		r.Get("/definition/{name}", s.handleVisualizeDefinition)

		// Instance visualization
		r.Get("/asset/{instanceID}", s.handleVisualizeAsset)
		r.Get("/asset/{instanceID}/history", s.handleVisualizeHistory)
	})

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting Web UI server on %s (API backend: %s)", addr, s.apiURL)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(nil)
	}
	return nil
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"web-ui","api_backend":"%s"}`, s.apiURL)
}
