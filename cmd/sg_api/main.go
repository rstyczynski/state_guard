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

	// Create API server with REAL handlers from internal/api
	// This uses the same battle-tested implementation as the original cmd/api
	server := api.NewServer(store, *assetDir)

	// Graceful shutdown handler
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down API server...")
		if err := server.Shutdown(); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	addr := ":" + *port
	log.Printf("Starting State Guard API Server (sg_api) on %s", addr)
	log.Printf("API endpoints available at: http://localhost%s/api/v1/", addr)
	log.Printf("API documentation: http://localhost%s/docs", addr)
	log.Printf("")
	log.Printf("NOTE: This is the DATA API server for asset management")
	log.Printf("      For Web UI and visualization features, use sg_web")
	log.Printf("      Example: ./bin/sg_web --port 3000 --db %s --asset-dir %s", *dbPath, *assetDir)

	if err := server.Start(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
