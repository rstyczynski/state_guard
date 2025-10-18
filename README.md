# FSM - Go Finite State Machine with REST API

A comprehensive Go-based Finite State Machine implementation that processes state changes following YAML definitions, with REST API, persistent storage, webhooks, and multi-instance management.

## Features

### ✅ Phase 1.2 - Persistence (Implemented)
- SQLite database backend with transaction support
- Atomic state changes with persistence-first guarantee
- Multi-instance asset tracking
- State transition history with timestamps
- Database migrations

### ✅ Phase 1.3 - Asset Types & Webhooks (Implemented)
- Asset Type definitions linking FSM + Webhooks
- Webhook notifications on state changes
- Conditions: `on_enter`, `on_exit`, `on_transition`
- HTTP/HTTPS webhook support with timeout protection
- Background worker pool for async notifications
- Extensible sink interface for future integrations

### ✅ Phase 2.2 - REST API (Implemented)
- Full REST API for multi-instance FSM management
- Persistent storage with SQLite
- OpenAPI 3.0 documentation
- CORS support
- Comprehensive error handling
- Graceful shutdown

## Installation

### Prerequisites

- Go 1.21 or higher
- SQLite3 (included via go-sqlite3)

### Clone and Build

```bash
# Clone the repository
git clone <repository-url>
cd fsm_v2

# Install dependencies
go mod download

# Build the CLI
go build -o bin/fsm cmd/fsm/main.go

# Build the API server
go build -o bin/api cmd/api/main.go
```

## Quick Start

### Option 1: REST API Server

```bash
# Start the API server
./bin/api --port 8080 --db fsm.db --asset-dir examples

# Create an asset
curl -X POST http://localhost:8080/api/v1/assets \
  -H 'Content-Type: application/json' \
  -d '{"asset_type":"simple_asset_type.yaml","instance_id":"web-server-1"}'

# Execute state transition
curl -X POST http://localhost:8080/api/v1/assets/web-server-1/transition \
  -H 'Content-Type: application/json' \
  -d '{"to_state":"RUNNING"}'

# Get asset details
curl http://localhost:8080/api/v1/assets/web-server-1

# Get state history
curl http://localhost:8080/api/v1/assets/web-server-1/history

# List all assets
curl http://localhost:8080/api/v1/assets

# Delete asset
curl -X DELETE http://localhost:8080/api/v1/assets/web-server-1
```

### Option 2: Interactive CLI

```bash
# Start with asset type and instance ID
./bin/fsm --asset-type examples/simple_asset_type.yaml --id my-server --db fsm.db

# Or load existing instance
./bin/fsm --load my-server --db fsm.db
```

### Interactive CLI Commands

```
> current              Show current state
> states               List all states (* marks current)
> available            Show available transitions
> transition <state>   Transition to a new state
> reset                Reset to initial state
> final                Check if in final state
> history [limit]      Show state transition history
> list-instances       List all instances in database
> load-asset <path> <id>     Load asset type and create instance
> load-instance <id> [path]  Load existing instance from DB
> help                 Show available commands
> exit                 Quit the program
```

### Example CLI Session

```bash
$ ./bin/fsm --asset-type examples/simple_asset_type.yaml --id web-1 --db fsm.db
Asset instance created successfully
Asset ID: web-1
FSM: generic_lifecycle
Current state: CREATED
Webhooks: enabled (1 configured)

> current
Current state: CREATED

> available
Available transitions:
  -> STARTING
  -> FAILED

> transition STARTING
Transitioned: CREATED -> STARTING

> transition RUNNING
Transitioned: STARTING -> RUNNING
[Webhook] Notifying: http://localhost:8080/webhooks/server-running

> history
State Transition History (most recent first):
  2025-10-17 22:00:15  STARTING  ->  RUNNING
  2025-10-17 22:00:10  CREATED   ->  STARTING

> list-instances
Stored FSM Instances:
  ID: web-1
    FSM: generic_lifecycle
    Asset Type: examples/simple_asset_type.yaml
    Current State: RUNNING
    Created: 2025-10-17 22:00:05
    Updated: 2025-10-17 22:00:15

> exit
Goodbye!
```

## Asset Type System

Asset Types link FSM definitions with webhook configurations.

### Asset Type Definition Format

```yaml
version: 1
name: web-server
state_machine: lifecycle.yaml  # Relative to asset type file

webhooks:
  # Trigger when entering RUNNING state
  - condition: on_enter
    state: RUNNING
    url: https://example.com/webhooks/server-running
    method: POST
    timeout: 5s
    headers:
      X-Event-Type: server-running
      Authorization: Bearer token

  # Trigger when exiting RUNNING state
  - condition: on_exit
    state: RUNNING
    url: https://example.com/webhooks/server-stopped
    method: POST
    timeout: 5s

  # Trigger on specific transition
  - condition: on_transition
    from: CREATED
    to: STARTING
    url: https://example.com/webhooks/server-starting
    method: POST
    timeout: 10s
```

### Webhook Conditions

1. **on_enter**: Triggered when entering a specific state
2. **on_exit**: Triggered when exiting a specific state
3. **on_transition**: Triggered on specific state transitions

### Webhook Payload

Webhooks receive JSON payloads:

```json
{
  "instance_id": "web-server-1",
  "asset_type": "simple_asset_type.yaml",
  "from_state": "STARTING",
  "to_state": "RUNNING",
  "timestamp": "2025-10-17T22:00:15Z",
  "metadata": {}
}
```

## FSM Definition Format

FSM configurations are defined in YAML:

```yaml
version: 1
name: my_fsm
initial: CREATED      # Starting state
final:                # Final states (optional)
  - TERMINATED

states:
  - CREATED
  - RUNNING
  - STOPPED
  - TERMINATED
  - FAILED

transitions:
  # Explicit transitions
  - from: CREATED
    to: RUNNING

  - from: RUNNING
    to: STOPPED

  - from: STOPPED
    to: TERMINATED

  # Wildcard: any state can transition to FAILED
  - from: "*"
    to: FAILED
```

## REST API Endpoints

Full API documentation available at `docs/openapi.yaml`

### Health Check
- `GET /api/v1/health` - Server health status

### Asset Management
- `POST /api/v1/assets` - Create new asset instance
- `GET /api/v1/assets` - List all assets
- `GET /api/v1/assets/{id}` - Get asset details
- `DELETE /api/v1/assets/{id}` - Delete asset

### State Operations
- `POST /api/v1/assets/{id}/transition` - Execute state transition
- `GET /api/v1/assets/{id}/history?limit=100` - Get state history

### Request/Response Examples

**Create Asset:**
```json
POST /api/v1/assets
{
  "asset_type": "examples/simple_asset_type.yaml",
  "instance_id": "web-server-1"
}

Response: 201 Created
{
  "id": "web-server-1",
  "asset_type": "simple_asset_type.yaml",
  "definition_name": "generic_lifecycle",
  "current_state": "CREATED",
  "available_transitions": ["STARTING", "FAILED"],
  "is_final_state": false,
  "created_at": "2025-10-17T22:00:00Z",
  "updated_at": "2025-10-17T22:00:00Z"
}
```

```bash
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{
    "asset_type": "simple_asset_type.yaml",
    "instance_id": "web-server-1"
  }'
```

**Execute Transition:**
```json
POST /api/v1/assets/web-server-1/transition
{
  "to_state": "CREATING"
}

Response: 200 OK
{
  "from_state": "CREATED",
  "to_state": "STARTING",
  "timestamp": "2025-10-17T22:36:32.367002+02:00"
}
```

```bash
curl -X POST http://localhost:8080/api/v1/assets/web-server-1/transition \
  -H "Content-Type: application/json" \
  -d '{
    "to_state": "STARTING"
  }' | jq
```

**Get History:**
```json
GET /api/v1/assets/web-server-1/history?limit=10

Response: 200 OK
{
  "instance_id": "web-server-1",
  "transitions": [
    {
      "from_state": "STARTING",
      "to_state": "RUNNING",
      "transitioned_at": "2025-10-17T22:00:15Z"
    },
    {
      "from_state": "CREATED",
      "to_state": "STARTING",
      "transitioned_at": "2025-10-17T22:00:10Z"
    }
  ]
}
```

```bash
curl -X GET "http://localhost:8080/api/v1/assets/web-server-1/history?limit=10" | jq
```

## Examples

Example configurations are included in `examples/`:

### 1. Simple Asset Type (`examples/simple_asset_type.yaml`)
- Basic web server with lifecycle FSM
- Single webhook on RUNNING state

### 2. Web Server Asset Type (`examples/web_server_asset_type.yaml`)
- Complete web server lifecycle
- Multiple webhooks for different events
- Custom headers for authentication

### 3. Database Asset Type (`examples/database_asset_type.yaml`)
- Database lifecycle management
- Critical alerts for failures
- PagerDuty integration example

### 4. Generic Lifecycle FSM (`examples/lifecycle.yaml`)
A comprehensive lifecycle FSM with 9 states:
- Linear: CREATED → STARTING → RUNNING → STOPPING → STOPPED → TERMINATING → TERMINATED
- Maintenance: RUNNING ↔ MAINTENANCE
- Recovery: STOPPED → STARTING, FAILED → STARTING
- Global failure: * → FAILED

## Architecture

### Project Structure

```
fsm_v2/
├── cmd/
│   ├── api/                  # REST API server
│   │   └── main.go
│   └── fsm/                  # CLI application
│       └── main.go
├── internal/
│   ├── api/                  # API handlers and server
│   │   ├── server.go         # HTTP server with middleware
│   │   ├── handlers.go       # Endpoint handlers
│   │   ├── models.go         # Request/response models
│   │   └── handlers_test.go  # API tests
│   ├── fsm/                  # Core FSM engine
│   │   ├── definition.go     # YAML parsing
│   │   ├── fsm.go           # State machine logic
│   │   ├── asset_type.go    # Asset type definitions
│   │   └── errors.go        # Custom errors
│   ├── storage/             # Persistence layer
│   │   ├── storage.go       # Storage interface
│   │   └── sqlite.go        # SQLite implementation
│   └── webhook/             # Webhook system
│       ├── dispatcher.go    # Worker pool
│       ├── sink.go          # Sink interface
│       └── http_sink.go     # HTTP implementation
├── examples/                # Example configurations
│   ├── lifecycle.yaml
│   ├── simple_asset_type.yaml
│   ├── web_server_asset_type.yaml
│   └── database_asset_type.yaml
├── docs/
│   └── openapi.yaml         # OpenAPI 3.0 spec
├── bin/                     # Compiled binaries
├── go.mod
├── go.sum
├── CLAUDE.md
├── IMPLEMENTATION_PLAN.md
└── README.md
```

### Key Components

**FSM Engine** (`internal/fsm`)
- YAML-based configuration
- Thread-safe state transitions
- Wildcard support
- Validation and error handling

**Storage Layer** (`internal/storage`)
- Interface-based design for extensibility
- SQLite implementation with WAL mode
- Atomic state updates with transactions
- Automatic schema migrations

**Webhook System** (`internal/webhook`)
- Sink interface for extensibility
- HTTP/HTTPS implementation
- Worker pool (5 concurrent workers)
- Timeout protection
- Non-blocking notifications

**REST API** (`internal/api`)
- Clean separation of concerns
- Middleware: logging, CORS
- Comprehensive error handling
- OpenAPI documented

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Check test coverage
go test ./... -cover

# Run API tests specifically
go test ./internal/api/... -v

# Generate coverage report
go test ./internal/fsm/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Current Test Coverage

```
internal/api:     59.0% of statements
internal/fsm:     95.3% of statements
internal/storage: 81.3% of statements
internal/webhook: 88.2% of statements
```

### Code Quality

```bash
# Format code
go fmt ./...

# Run linter
golint ./...

# Static analysis
go vet ./...

# Run staticcheck (requires installation)
staticcheck ./...
```

### Building

```bash
# Build CLI
go build -o bin/fsm cmd/fsm/main.go

# Build API server
go build -o bin/api cmd/api/main.go

# Build both
go build -o bin/ ./cmd/...
```

## Roadmap

Development follows a phased approach:

- **Phase 1.1** ✅ Core FSM Engine (COMPLETE)
  - YAML configuration
  - State validation
  - Basic CLI
  - Test coverage: 95%+

- **Phase 1.2** ✅ Persistence (COMPLETE)
  - SQLite backend
  - Transaction support
  - State history tracking
  - Database migrations

- **Phase 1.3** ✅ Webhook Dispatcher (COMPLETE)
  - Asset Type system
  - Webhook notifications
  - HTTP sink implementation
  - Worker pool architecture

- **Phase 2.1** 📋 Enhanced History (PLANNED)
  - Query by time range
  - JSON/CSV export
  - History rotation policies

- **Phase 2.2** ✅ REST API (COMPLETE)
  - HTTP API for FSM operations
  - Multi-instance management
  - OpenAPI documentation
  - CORS support

- **Phase 2.3** 📋 Enhanced Console (PLANNED)
  - Tab completion
  - Readline support
  - Command history (arrow keys)
  - Enhanced `/command` interface

- **Phase 3** 📋 Security (PLANNED)
  - JWT authentication
  - Role-based access control (RBAC)
  - Rate limiting
  - Input validation hardening

- **Phase 4** 📋 Observability (PLANNED)
  - Prometheus metrics
  - Structured logging (JSON)
  - Enhanced health checks
  - Distributed tracing

- **Phase 5** 📋 Advanced Features (PLANNED)
  - Bulk operations
  - Hot-reload configurations
  - Event hooks (before/after transition)
  - Client SDKs (Python, JavaScript)
  - Kafka sink support

See [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) for detailed phase descriptions.

## API Usage Examples

### Go API

```go
import (
    "github.com/rstyczynski/fsm_v2/internal/fsm"
    "github.com/rstyczynski/fsm_v2/internal/storage"
    "github.com/rstyczynski/fsm_v2/internal/webhook"
)

// Load asset type
assetType, err := fsm.LoadAssetType("examples/simple_asset_type.yaml")
if err != nil {
    log.Fatal(err)
}

// Load FSM definition
err = assetType.LoadStateMachine()
if err != nil {
    log.Fatal(err)
}

// Create storage
store, _ := storage.NewSQLiteStorage("fsm.db")
store.Initialize(context.Background())
defer store.Close()

// Create webhook dispatcher
dispatcher, _ := webhook.NewDispatcher(5, 100, assetType)
defer dispatcher.Close()

// Create FSM instance with webhooks
f, err := fsm.NewWithWebhook(
    assetType.StateMachineRef,
    "my-instance",
    "examples/simple_asset_type.yaml",
    store,
    dispatcher,
)

// Query state
currentState := f.CurrentState()
isAtEnd := f.IsFinalState()
available := f.AvailableTransitions()

// Transition (triggers webhooks automatically)
err = f.Transition("RUNNING")
if err != nil {
    // Handle InvalidTransitionError or UnknownStateError
}

// Get history
ctx := context.Background()
history, _ := store.GetTransitionHistory(ctx, "my-instance", 10)
```

### REST API

```bash
# Health check
curl http://localhost:8080/api/v1/health

# Create asset
curl -X POST http://localhost:8080/api/v1/assets \
  -H 'Content-Type: application/json' \
  -d '{
    "asset_type": "web_server_asset_type.yaml",
    "instance_id": "prod-web-1"
  }'

# Transition
curl -X POST http://localhost:8080/api/v1/assets/prod-web-1/transition \
  -H 'Content-Type: application/json' \
  -d '{"to_state": "RUNNING"}'

# Get details
curl http://localhost:8080/api/v1/assets/prod-web-1

# Get history
curl http://localhost:8080/api/v1/assets/prod-web-1/history?limit=20

# List all
curl http://localhost:8080/api/v1/assets

# Delete
curl -X DELETE http://localhost:8080/api/v1/assets/prod-web-1
```

## Error Handling

The FSM returns specific error types:

```go
// Invalid transition attempt
err := f.Transition("INVALID_TARGET")
if invErr, ok := err.(*fsm.InvalidTransitionError); ok {
    fmt.Printf("Cannot go from %s to %s\n", invErr.From, invErr.To)
}

// Unknown state
err := f.Transition("NONEXISTENT")
if unkErr, ok := err.(*fsm.UnknownStateError); ok {
    fmt.Printf("State %s not defined\n", unkErr.State)
}
```

### API Error Responses

```json
{
  "error": "Bad Request",
  "message": "Transition failed: invalid transition from CREATED to TERMINATED",
  "code": 400
}
```

## Configuration

### CLI Flags

**fsm command:**
```
--asset-type string  Path to Asset Type YAML file
--id string         Asset instance ID (required with --asset-type)
--db string         SQLite database path (default: fsm.db)
--load string       Load existing instance ID from database
--config string     FSM YAML file (deprecated, use --asset-type)
```

**api command:**
```
--port string       HTTP server port (default: 8080)
--db string         SQLite database path (default: fsm.db)
--asset-dir string  Asset type directory (default: examples)
--version          Show version and exit
```

## Performance

- **Worker Pool**: 5 concurrent webhook workers (configurable)
- **HTTP Timeouts**: 15s read/write, 60s idle
- **Webhook Timeout**: Configurable per webhook (default 5s)
- **Database**: SQLite with WAL mode for better concurrency
- **Transaction**: Atomic state updates with rollback support

## Contributing

1. Development follows strict phase ordering (see IMPLEMENTATION_PLAN.md)
2. All phases require >80% test coverage
3. Follow Go standard formatting (`go fmt`, `go vet`)
4. Use table-driven tests for new functionality
5. Update OpenAPI spec for API changes

## License

[Specify License]

## Authors

[Specify Authors]
