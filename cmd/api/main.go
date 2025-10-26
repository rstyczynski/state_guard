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

	"github.com/rstyczynski/fsm_v2/internal/api"
	"github.com/rstyczynski/fsm_v2/internal/storage"
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
		fmt.Printf("FSM API Server version %s\n", version)
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

	// Create API server
	server := api.NewServer(store, *assetDir)

	// Start server in goroutine
	addr := fmt.Sprintf(":%s", *port)
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Starting FSM API Server v%s", version)
		log.Printf("Asset directory: %s", *assetDir)
		log.Printf("Database: %s", *dbPath)
		log.Printf("Listening on http://localhost:%s", *port)
		log.Println()
		log.Println("Documentation:")
		log.Printf("  Interactive Docs: http://localhost:%s/v1/docs", *port)
		log.Printf("  OpenAPI Spec:     http://localhost:%s/v1/openapi.yaml", *port)
		log.Println()
		log.Println("API Endpoints:")
		log.Printf("  Health:   GET    http://localhost:%s/api/v1/health", *port)
		log.Println("  POST   /api/v1/assets              - Create new asset")
		log.Println("  GET    /api/v1/assets              - List all assets")
		log.Println("  GET    /api/v1/assets/{id}         - Get asset details")
		log.Println("  DELETE /api/v1/assets/{id}         - Delete asset")
		log.Println("  POST   /api/v1/assets/{id}/transition - Execute state transition")
		log.Println("  GET    /api/v1/assets/{id}/history - Get state history")
		log.Println()

		if err := server.Start(addr); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("\nShutdown signal received, gracefully shutting down...")
	case err := <-serverErr:
		log.Printf("Server error: %v", err)
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server stopped")
}
