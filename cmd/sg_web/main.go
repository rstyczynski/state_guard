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
	apiURL := flag.String("api-url", "", "URL of the sg_api backend server (optional, for hybrid mode)")
	dbPath := flag.String("db", "fsm.db", "Path to SQLite database file")
	assetDir := flag.String("asset-dir", "examples", "Directory containing asset type YAML files")
	showVersion := flag.Bool("version", false, "Show version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Printf("State Guard Web UI Server (sg_web) version %s\n", version)
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

	// Create Web UI server with storage and asset directory
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
	log.Printf("Asset directory: %s", *assetDir)
	if *apiURL != "" {
		log.Printf("Hybrid mode - API Backend: %s", *apiURL)
	} else {
		log.Printf("Standalone mode - All features served locally")
	}
	log.Printf("Visualization features: Zoom, Timeline, History, Next States, Current State highlighting")

	if err := server.Start(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
