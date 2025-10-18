package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rstyczynski/fsm_v2/internal/fsm"
	"github.com/rstyczynski/fsm_v2/internal/storage"
	"github.com/rstyczynski/fsm_v2/internal/webhook"
)

func main() {
	assetTypeFile := flag.String("asset-type", "", "Path to Asset Type YAML file")
	configFile := flag.String("config", "", "Path to FSM YAML configuration file (DEPRECATED: use --asset-type)")
	dbPath := flag.String("db", "", "Path to SQLite database for persistence (optional)")
	instanceID := flag.String("id", "", "Asset instance ID (asset name) - REQUIRED")
	flag.Parse()

	var f *fsm.FSM
	var store storage.Storage
	var dispatcher *webhook.Dispatcher
	var err error

	// Initialize storage if db path provided
	if *dbPath != "" {
		store, err = storage.NewSQLiteStorage(*dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating storage: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := store.Initialize(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing storage: %v\n", err)
			cancel()
			os.Exit(1)
		}
		cancel()

		fmt.Printf("Persistence enabled: %s\n", *dbPath)
		defer store.Close()
	}

	// Load FSM from asset type or config
	if *assetTypeFile != "" {
		// Require ID when using --asset-type
		if *instanceID == "" {
			fmt.Fprintf(os.Stderr, "Error: --id is required when using --asset-type\n")
			fmt.Fprintf(os.Stderr, "Example: ./fsm --asset-type examples/web_server_asset_type.yaml --id server1\n")
			os.Exit(1)
		}

		f, dispatcher, err = loadAssetType(*assetTypeFile, *instanceID, store)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Asset Type loaded successfully\n")
		fmt.Printf("Asset ID: %s\n", f.ID())
		fmt.Printf("FSM: %s\n", f.Name())
		fmt.Printf("Initial state: %s\n", f.InitialState())
		fmt.Printf("Current state: %s\n", f.CurrentState())
		if dispatcher != nil {
			fmt.Printf("Webhooks: enabled (%d workers)\n", dispatcher.WorkerCount())
		}
		fmt.Println()
	} else if *configFile != "" {
		// DEPRECATED: Direct FSM loading
		fmt.Println("Warning: --config is deprecated, use --asset-type instead")

		if *instanceID == "" {
			fmt.Fprintf(os.Stderr, "Error: --id is required\n")
			fmt.Fprintf(os.Stderr, "Example: ./fsm --config examples/lifecycle.yaml --id server1\n")
			os.Exit(1)
		}

		def, err := fsm.LoadDefinition(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading definition: %v\n", err)
			os.Exit(1)
		}

		f, err = fsm.New(def, *instanceID, store)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating FSM: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("FSM '%s' loaded successfully\n", f.Name())
		fmt.Printf("FSM ID: %s\n", f.ID())
		fmt.Printf("Initial state: %s\n", f.InitialState())
		fmt.Printf("Current state: %s\n\n", f.CurrentState())
	} else {
		fmt.Println("FSM CLI - No configuration loaded")
		fmt.Println("Use 'load-asset <asset-type-path> <id>' to load an asset")
		fmt.Println("Or 'load <fsm-path> <id>' for direct FSM loading (deprecated)")
	}

	// Ensure webhook dispatcher is cleaned up on exit
	if dispatcher != nil {
		defer dispatcher.Close()
	}

	// Simple command loop
	printHelp()
	fmt.Println()

	// Main loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]

		switch cmd {
		case "load-asset":
			if len(parts) < 3 {
				fmt.Println("Error: load-asset requires an asset type path and an instance ID")
				fmt.Println("Usage: load-asset <asset-type-path> <id>")
				fmt.Println("Example: load-asset examples/web_server_asset_type.yaml server1")
				continue
			}
			assetTypePath := parts[1]
			assetID := parts[2]

			// Close existing dispatcher if any
			if dispatcher != nil {
				dispatcher.Close()
				dispatcher = nil
			}

			var newDisp *webhook.Dispatcher
			newFSM, newDisp, err := loadAssetType(assetTypePath, assetID, store)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				f = newFSM
				dispatcher = newDisp
				fmt.Printf("Asset loaded successfully\n")
				fmt.Printf("Asset ID: %s\n", f.ID())
				fmt.Printf("FSM: %s\n", f.Name())
				fmt.Printf("Initial state: %s\n", f.InitialState())
				fmt.Printf("Current state: %s\n", f.CurrentState())
				if dispatcher != nil {
					fmt.Printf("Webhooks: enabled\n")
				}
			}

		case "load":
			fmt.Println("Warning: 'load' is deprecated, use 'load-asset' instead")
			if len(parts) < 3 {
				fmt.Println("Error: load requires a file path and an instance ID")
				fmt.Println("Usage: load <path> <id>")
				fmt.Println("Example: load examples/lifecycle.yaml server1")
				continue
			}
			filepath := parts[1]
			instanceID := parts[2]

			def, err := fsm.LoadDefinition(filepath)
			if err != nil {
				fmt.Printf("Error loading definition: %v\n", err)
				continue
			}

			newFSM, err := fsm.New(def, instanceID, store)
			if err != nil {
				fmt.Printf("Error creating FSM: %v\n", err)
			} else {
				f = newFSM
				fmt.Printf("FSM '%s' loaded successfully\n", f.Name())
				fmt.Printf("FSM ID: %s\n", f.ID())
				fmt.Printf("Initial state: %s\n", f.InitialState())
				fmt.Printf("Current state: %s\n", f.CurrentState())
			}

		case "current-state":
			if f == nil {
				fmt.Println("Error: No FSM loaded. Use 'load <path>' first")
				continue
			}
			fmt.Printf("Current state: %s\n", f.CurrentState())

		case "current": // Deprecated alias for backward compatibility
			if f == nil {
				fmt.Println("Error: No FSM loaded. Use 'load <path>' first")
				continue
			}
			fmt.Printf("Current state: %s\n", f.CurrentState())
			fmt.Println("Note: 'current' is deprecated, use 'current-state' instead")

		case "validate":
			if len(parts) < 2 {
				fmt.Println("Error: validate requires a file path")
				continue
			}
			filepath := parts[1]

			def, err := fsm.LoadDefinition(filepath)
			if err != nil {
				fmt.Printf("Validation failed: %v\n", err)
			} else {
				fmt.Printf("✓ FSM definition '%s' is valid\n", def.Name)
				fmt.Printf("  - Version: %d\n", def.Version)
				fmt.Printf("  - States: %d\n", len(def.States))
				fmt.Printf("  - Transitions: %d\n", len(def.Transitions))
				fmt.Printf("  - Initial: %s\n", def.Initial)
				if len(def.Final) > 0 {
					fmt.Printf("  - Final states: %v\n", def.Final)
				}
			}

		case "states":
			if f == nil {
				fmt.Println("Error: No FSM loaded. Use 'load <path>' first")
				continue
			}
			fmt.Println("All states:")
			for _, state := range f.States() {
				marker := " "
				if state == f.CurrentState() {
					marker = "*"
				}
				fmt.Printf("  %s %s\n", marker, state)
			}

		case "available":
			if f == nil {
				fmt.Println("Error: No FSM loaded. Use 'load <path>' first")
				continue
			}
			available := f.AvailableTransitions()
			if len(available) == 0 {
				fmt.Println("No available transitions from current state")
			} else {
				fmt.Println("Available transitions:")
				for _, state := range available {
					fmt.Printf("  -> %s\n", state)
				}
			}

		case "transition":
			if f == nil {
				fmt.Println("Error: No FSM loaded. Use 'load <path>' first")
				continue
			}
			if len(parts) < 2 {
				fmt.Println("Error: transition requires a target state")
				continue
			}
			targetState := parts[1]

			oldState := f.CurrentState()
			err := f.Transition(targetState)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Transitioned: %s -> %s\n", oldState, f.CurrentState())
			}

		case "reset":
			if f == nil {
				fmt.Println("Error: No FSM loaded. Use 'load <path>' first")
				continue
			}
			err := f.Reset()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Reset to initial state: %s\n", f.CurrentState())
			}

		case "final":
			if f == nil {
				fmt.Println("Error: No FSM loaded. Use 'load <path>' first")
				continue
			}
			if f.IsFinalState() {
				fmt.Printf("Current state '%s' is a final state\n", f.CurrentState())
			} else {
				fmt.Printf("Current state '%s' is not a final state\n", f.CurrentState())
			}

		case "list-instances":
			if store == nil {
				fmt.Println("Error: Persistence not enabled. Use --db flag to enable storage")
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			instances, err := store.ListInstances(ctx)
			cancel()

			if err != nil {
				fmt.Printf("Error listing instances: %v\n", err)
				continue
			}

			if len(instances) == 0 {
				fmt.Println("No FSM instances found in database")
				continue
			}

			fmt.Printf("Found %d FSM instance(s):\n", len(instances))
			for _, instance := range instances {
				marker := " "
				if f != nil && f.ID() == instance.ID {
					marker = "*"
				}
				fmt.Printf("%s ID: %s\n", marker, instance.ID)
				fmt.Printf("  FSM: %s\n", instance.DefinitionName)
				if instance.AssetTypeName != "" {
					fmt.Printf("  Asset Type: %s\n", instance.AssetTypeName)
				}
				fmt.Printf("  State: %s\n", instance.CurrentState)
				fmt.Printf("  Created: %s\n", instance.CreatedAt.Format("2006-01-02 15:04:05"))
				fmt.Printf("  Updated: %s\n", instance.UpdatedAt.Format("2006-01-02 15:04:05"))
				fmt.Println()
			}

		case "load-instance":
			if store == nil {
				fmt.Println("Error: Persistence not enabled. Use --db flag to enable storage")
				continue
			}

			if len(parts) < 2 {
				fmt.Println("Error: load-instance requires an instance ID")
				fmt.Println("Use 'list-instances' to see available instances")
				continue
			}

			instanceID := parts[1]

			// Get the instance to find its asset type
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			instance, err := store.GetInstance(ctx, instanceID)
			cancel()

			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			// Determine asset type path
			var assetTypePath string
			if len(parts) >= 3 {
				// User provided asset type path
				assetTypePath = parts[2]
			} else if instance.AssetTypeName != "" {
				// Use stored asset type name as path
				assetTypePath = instance.AssetTypeName
				fmt.Printf("Using stored asset type: %s\n", assetTypePath)
			} else {
				// No asset type stored (legacy instance)
				fmt.Printf("Instance '%s' has no asset type stored (created before v1.3)\n", instanceID)
				fmt.Println("Usage: load-instance <id> <asset-type-path>")
				fmt.Println("Example: load-instance", instanceID, "examples/web_server_asset_type.yaml")
				continue
			}

			// Load asset type
			assetType, err := fsm.LoadAssetType(assetTypePath)
			if err != nil {
				fmt.Printf("Error loading asset type: %v\n", err)
				if len(parts) < 3 {
					fmt.Println("Try: load-instance", instanceID, "<asset-type-path>")
				}
				continue
			}

			// Load the FSM definition from asset type
			if err := assetType.LoadStateMachine(); err != nil {
				fmt.Printf("Error loading state machine: %v\n", err)
				continue
			}

			// Verify FSM definition matches
			if assetType.StateMachineRef.Name != instance.DefinitionName {
				fmt.Printf("Warning: Asset type FSM '%s' doesn't match stored FSM '%s'\n",
					assetType.StateMachineRef.Name, instance.DefinitionName)
			}

			// Create webhook dispatcher if webhooks are configured
			// Close existing dispatcher if any
			if dispatcher != nil {
				dispatcher.Close()
				dispatcher = nil
			}

			var newDisp *webhook.Dispatcher
			if len(assetType.Webhooks) > 0 {
				newDisp, err = webhook.NewDispatcher(5, 100, assetType)
				if err != nil {
					fmt.Printf("Error creating webhook dispatcher: %v\n", err)
					continue
				}
			}

			// Load FSM from storage with webhook support
			// Note: We pass the assetTypePath (file path) not assetType.Name
			// so it can be stored and reloaded later
			ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
			loadedFSM, err := fsm.LoadFromStorageWithWebhook(ctx2, instanceID, assetType.StateMachineRef, assetTypePath, store, newDisp)
			cancel2()

			if err != nil {
				fmt.Printf("Error loading instance: %v\n", err)
				if newDisp != nil {
					newDisp.Close()
				}
			} else {
				f = loadedFSM
				dispatcher = newDisp
				fmt.Printf("Asset instance loaded successfully\n")
				fmt.Printf("Asset ID: %s\n", f.ID())
				fmt.Printf("FSM: %s\n", f.Name())
				fmt.Printf("Current state: %s\n", f.CurrentState())
				if dispatcher != nil {
					fmt.Printf("Webhooks: enabled\n")
				}
			}

		case "delete-instance":
			if store == nil {
				fmt.Println("Error: Persistence not enabled. Use --db flag to enable storage")
				continue
			}

			if len(parts) < 2 {
				fmt.Println("Error: delete-instance requires an instance ID")
				fmt.Println("Use 'list-instances' to see available instances")
				continue
			}

			instanceID := parts[1]

			// Check if trying to delete currently loaded instance
			if f != nil && f.ID() == instanceID {
				fmt.Println("Warning: You are deleting the currently loaded instance")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := store.DeleteInstance(ctx, instanceID)
			cancel()

			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Instance '%s' deleted successfully\n", instanceID)

				// Clear current FSM if it was deleted
				if f != nil && f.ID() == instanceID {
					f = nil
					fmt.Println("Current FSM instance cleared")
				}
			}

		case "help":
			printHelp()

		case "exit", "quit":
			fmt.Println("Goodbye!")
			os.Exit(0)

		default:
			fmt.Printf("Unknown command: %s (type 'help' for available commands)\n", cmd)
		}
	}
}

// loadAssetType loads an asset type and creates FSM with webhook support
// assetTypePath is the file path to the asset type YAML file (e.g., "examples/web_server_asset_type.yaml")
func loadAssetType(assetTypePath, instanceID string, store storage.Storage) (*fsm.FSM, *webhook.Dispatcher, error) {
	// Load asset type
	assetType, err := fsm.LoadAssetType(assetTypePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load asset type: %w", err)
	}

	// Load the FSM definition from asset type
	if err := assetType.LoadStateMachine(); err != nil {
		return nil, nil, fmt.Errorf("failed to load state machine: %w", err)
	}

	// Create webhook dispatcher if webhooks are configured
	var dispatcher *webhook.Dispatcher
	if len(assetType.Webhooks) > 0 {
		dispatcher, err = webhook.NewDispatcher(5, 100, assetType)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create webhook dispatcher: %w", err)
		}
	}

	// Create FSM with webhook support
	// Store the asset type FILE PATH (not just the name) so it can be reloaded
	f, err := fsm.NewWithWebhook(assetType.StateMachineRef, instanceID, assetTypePath, store, dispatcher)
	if err != nil {
		if dispatcher != nil {
			dispatcher.Close()
		}
		return nil, nil, fmt.Errorf("failed to create FSM: %w", err)
	}

	return f, dispatcher, nil
}

func printHelp() {
	fmt.Println("Commands:")
	fmt.Println("  load-asset <asset-type-path> <id>  - Load asset type with instance ID")
	fmt.Println("                                       Example: load-asset examples/web_server_asset_type.yaml server1")
	fmt.Println("  load <fsm-path> <id>  - Load FSM definition directly (DEPRECATED)")
	fmt.Println("                          Example: load examples/lifecycle.yaml server1")
	fmt.Println("  transition <to>       - Transition to a new state")
	fmt.Println("  current-state         - Show current state")
	fmt.Println("  validate <path>       - Validate FSM YAML definition")
	fmt.Println()
	fmt.Println("Instance management (requires --db):")
	fmt.Println("  list-instances        - Show all persisted FSM instances")
	fmt.Println("  load-instance <id> [asset-type-path]  - Load existing instance by ID")
	fmt.Println("                                          (asset type is auto-loaded if stored)")
	fmt.Println("  delete-instance <id>  - Delete a persisted instance")
	fmt.Println()
	fmt.Println("Additional commands:")
	fmt.Println("  states                - List all states")
	fmt.Println("  available             - Show available transitions")
	fmt.Println("  reset                 - Reset to initial state")
	fmt.Println("  final                 - Check if in final state")
	fmt.Println("  help                  - Show this help")
	fmt.Println("  exit                  - Exit program")
}
