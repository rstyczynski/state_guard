package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rstyczynski/fsm_v2/internal/storage"
	"github.com/rstyczynski/fsm_v2/internal/web"
)

var version = "2.2.0"

func main() {
	// Command line flags
	port := flag.String("port", "3000", "HTTP server port for Web UI")
	apiURL := flag.String("api-url", "http://localhost:8080", "URL of the sg_api backend server")
	dbPath := flag.String("db", "", "Path to SQLite database file (optional, uses HTTP client if not specified)")
	assetDir := flag.String("asset-dir", "", "Directory containing asset type YAML files (optional, only needed for definition visualization)")
	showVersion := flag.Bool("version", false, "Show version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Printf("State Guard Web UI Server (sg_web) version %s\n", version)
		os.Exit(0)
	}

	// Initialize storage
	var store storage.Storage
	var err error

	if *dbPath != "" {
		// Use direct SQLite storage (for standalone mode)
		log.Printf("Initializing SQLite storage: %s", *dbPath)
		store, err = storage.NewSQLiteStorage(*dbPath)
		if err != nil {
			log.Fatalf("Failed to create storage: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := store.Initialize(ctx); err != nil {
			log.Fatalf("Failed to initialize storage: %v", err)
		}
		defer store.Close()
	} else {
		// Use HTTP client to proxy to sg_api (recommended for WASM isolation)
		log.Printf("Using HTTP storage client (proxy to sg_api)")
		store = web.NewHTTPClient(*apiURL)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := store.Initialize(ctx); err != nil {
			log.Fatalf("Failed to connect to API server: %v", err)
		}
	}

	// Create Web UI server
	server := web.NewServer(store, *assetDir, *apiURL)

	// Graceful shutdown handler
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down Web UI server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		_ = shutdownCtx // TODO: Use for shutdown
		if err := server.Shutdown(); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	addr := ":" + *port
	log.Printf("Starting State Guard Web UI Server (sg_web) on %s", addr)
	log.Printf("Web UI available at: http://localhost%s/", addr)
	log.Printf("API Backend: %s", *apiURL)

	if *dbPath != "" {
		log.Printf("Storage Mode: Direct SQLite (%s)", *dbPath)
		if *assetDir != "" {
			log.Printf("Asset Directory: %s", *assetDir)
		} else {
			log.Printf("WARNING: No asset-dir specified - definition visualization will not work")
		}
	} else {
		log.Printf("Storage Mode: HTTP Proxy to sg_api")
		log.Printf("NOTE: FSM definitions are fetched from sg_api (no local asset files needed)")
		if *assetDir != "" {
			log.Printf("Asset Directory: %s (only used for definition visualization)", *assetDir)
		}
	}

	log.Printf("NOTE: sg_web renders visualizations locally (WASM isolation)")
	log.Printf("      but proxies data requests to sg_api")

	if err := server.Start(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
