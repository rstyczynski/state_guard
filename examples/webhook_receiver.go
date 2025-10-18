package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

// WebhookEvent represents the payload sent by FSM webhooks
type WebhookEvent struct {
	InstanceID string            `json:"instance_id"`
	AssetType  string            `json:"asset_type"`
	FromState  string            `json:"from_state"`
	ToState    string            `json:"to_state"`
	Timestamp  time.Time         `json:"timestamp"`
	Metadata   map[string]string `json:"metadata"`
}

func main() {
	// Webhook handler
	http.HandleFunc("/webhooks/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("📩 Received webhook: %s %s", r.Method, r.URL.Path)
		log.Printf("   Headers: %v", r.Header)

		// Read body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("   ❌ Error reading body: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Parse webhook event
		var event WebhookEvent
		if err := json.Unmarshal(body, &event); err != nil {
			log.Printf("   ❌ Error parsing JSON: %v", err)
			log.Printf("   Raw body: %s", string(body))
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Log the event
		log.Printf("   ✅ Instance: %s", event.InstanceID)
		log.Printf("   ✅ Asset Type: %s", event.AssetType)
		log.Printf("   ✅ Transition: %s → %s", event.FromState, event.ToState)
		log.Printf("   ✅ Timestamp: %s", event.Timestamp.Format(time.RFC3339))

		// Respond with success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"received"}`))
	})

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Start server
	port := "8081"
	log.Printf("🎯 Webhook Receiver starting on http://localhost:%s", port)
	log.Printf("   Listening for webhooks at http://localhost:%s/webhooks/*", port)
	log.Printf("   Health check: http://localhost:%s/health", port)
	log.Println()

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
