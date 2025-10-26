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

	"github.com/rstyczynski/fsm_v2/internal/web"
)

var version = "2.2.0"

func main() {
	// Command line flags
	port := flag.String("port", "3000", "HTTP server port for Web UI")
	apiURL := flag.String("api-url", "http://localhost:8080", "URL of the sg_api backend server")
	showVersion := flag.Bool("version", false, "Show version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Printf("State Guard Web UI Server (sg_web) version %s\n", version)
		os.Exit(0)
	}

	// Use HTTP client to proxy ALL data to sg_api (WASM isolation)
	log.Printf("Using HTTP storage client (proxy to sg_api)")
	store := web.NewHTTPClient(*apiURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := store.Initialize(ctx); err != nil {
		log.Fatalf("Failed to connect to API server: %v", err)
	}

	// Create Web UI server
	server := web.NewServer(store, *apiURL)

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
	log.Printf("Storage Mode: HTTP Proxy to sg_api (ALL data via API)")
	log.Printf("NOTE: sg_web renders visualizations locally (WASM isolation)")
	log.Printf("      ALL data (including FSM definitions) fetched from sg_api")

	if err := server.Start(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
