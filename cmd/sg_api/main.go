package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/rstyczynski/fsm_v2/internal/fsm"
	"github.com/rstyczynski/fsm_v2/internal/storage"
	"github.com/rstyczynski/fsm_v2/internal/visualize"
	"github.com/rstyczynski/fsm_v2/internal/webhook"
)

var version = "2.2.0"

func main() {
	// Command line flags
	port := flag.String("port", "8080", "HTTP server port")
	dbPath := flag.String("db", "fsm.db", "Path to SQLite database file")
	assetDir := flag.String("asset-dir", "examples", "Directory containing asset type YAML files")
	showVersion := flag.Bool("version", false, "Show version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Printf("State Guard API Server (sg_api) version %s\n", version)
		os.Exit(0)
	}

	// Initialize storage
	log.Printf("Initializing storage: %s", *dbPath)
	store, err := storage.NewSQLiteStorage(*dbPath)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := store.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Create router with ONLY API endpoints (no HTML/Web UI)
	r := createAPIRouter(store, *assetDir)

	// Create HTTP server
	addr := ":" + *port
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown handler
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down API server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	log.Printf("Starting State Guard API Server (sg_api) on %s", addr)
	log.Printf("API endpoints available at: http://localhost%s/api/v1/", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

// createAPIRouter creates a router with ONLY API endpoints (no HTML/Web UI)
func createAPIRouter(store storage.Storage, assetDir string) chi.Router {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(corsMiddleware)

	// API routes ONLY - no HTML/Web UI
	r.Route("/api/v1", func(r chi.Router) {
		// Health check
		r.Get("/health", handleHealth)

		// Asset routes
		r.Route("/assets", func(r chi.Router) {
			r.Get("/", handleListAssets(store, assetDir))
			r.Post("/", handleCreateAsset(store, assetDir))

			r.Route("/{instanceID}", func(r chi.Router) {
				r.Get("/", handleGetAsset(store))
				r.Delete("/", handleDeleteAsset(store))
				r.Post("/transition", handleTransition(store, assetDir))
				r.Get("/history", handleHistory(store))
			})
		})

		// Visualization routes (returns SVG/PNG/JSON/Mermaid - NOT HTML)
		r.Route("/visualize", func(r chi.Router) {
			handler := visualize.NewHandler(store, assetDir)

			// Definition visualization
			r.Get("/definition/{name}", handler.HandleVisualizeDefinition)

			// Instance visualization
			r.Get("/asset/{instanceID}", handler.HandleVisualizeAsset)
			r.Get("/asset/{instanceID}/history", handler.HandleVisualizeHistory)
		})
	})

	return r
}

// corsMiddleware adds CORS headers for API access
func corsMiddleware(next http.Handler) http.Handler {
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
func handleHealth(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]string{
		"status":  "healthy",
		"service": "sg_api",
		"version": version,
	})
}

// Handler functions (simplified versions from internal/api/handlers.go)
// These are placeholders - in production, import from internal/api

func handleListAssets(store storage.Storage, assetDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement or import from internal/api
		render.JSON(w, r, map[string]string{"message": "List assets not yet implemented in sg_api"})
	}
}

func handleCreateAsset(store storage.Storage, assetDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement or import from internal/api
		render.JSON(w, r, map[string]string{"message": "Create asset not yet implemented in sg_api"})
	}
}

func handleGetAsset(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement or import from internal/api
		render.JSON(w, r, map[string]string{"message": "Get asset not yet implemented in sg_api"})
	}
}

func handleDeleteAsset(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement or import from internal/api
		render.JSON(w, r, map[string]string{"message": "Delete asset not yet implemented in sg_api"})
	}
}

func handleTransition(store storage.Storage, assetDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement or import from internal/api
		render.JSON(w, r, map[string]string{"message": "Transition not yet implemented in sg_api"})
	}
}

func handleHistory(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement or import from internal/api
		render.JSON(w, r, map[string]string{"message": "History not yet implemented in sg_api"})
	}
}

// Unused imports to satisfy compiler - remove when implementing handlers
var _ = fsm.Definition{}
var _ = webhook.Dispatcher{}
