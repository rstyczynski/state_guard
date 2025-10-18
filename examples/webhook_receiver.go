package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// WebhookEvent represents the webhook payload
type WebhookEvent struct {
	InstanceID string            `json:"instance_id"`
	AssetType  string            `json:"asset_type"`
	FromState  string            `json:"from_state"`
	ToState    string            `json:"to_state"`
	Timestamp  time.Time         `json:"timestamp"`
	Metadata   map[string]string `json:"metadata"`
}

func main() {
	http.HandleFunc("/webhooks/server-running", handleServerRunning)
	http.HandleFunc("/webhooks/", handleGeneric)

	fmt.Println("🎣 Webhook Receiver Starting")
	fmt.Println("================================")
	fmt.Println("Listening on: http://localhost:8081")
	fmt.Println("")
	fmt.Println("Endpoints:")
	fmt.Println("  - /webhooks/server-running")
	fmt.Println("  - /webhooks/* (catch-all)")
	fmt.Println("")

	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}

func handleServerRunning(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("\n[%s] %s %s\n", time.Now().Format("15:04:05"), r.Method, r.URL.Path)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("❌ Error reading body: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		fmt.Printf("❌ Error parsing JSON: %v\n", err)
		fmt.Printf("Raw body: %s\n", string(body))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fmt.Printf("✅ SERVER RUNNING WEBHOOK RECEIVED\n")
	fmt.Printf("   Instance:    %s\n", event.InstanceID)
	fmt.Printf("   Asset Type:  %s\n", event.AssetType)
	fmt.Printf("   Transition:  %s → %s\n", event.FromState, event.ToState)
	fmt.Printf("   Timestamp:   %s\n", event.Timestamp.Format("15:04:05"))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"received"}`))
}

func handleGeneric(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("\n[%s] %s %s\n", time.Now().Format("15:04:05"), r.Method, r.URL.Path)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err == nil {
		fmt.Printf("   %s → %s (instance: %s)\n", event.FromState, event.ToState, event.InstanceID)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"received"}`))
}
