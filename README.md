# FSM - Go Finite State Machine with REST API

A production-ready Go-based Finite State Machine implementation with REST API, persistent storage, webhooks, and multi-instance management.

## Features

✅ **REST API** - Production-ready HTTP API with Chi router
✅ **Interactive CLI** - REPL for testing and development
✅ **Asset Types** - State machine + webhook configuration
✅ **Webhooks** - HTTP notifications on state changes (async, non-blocking)
✅ **Persistence** - SQLite with transactional guarantees
✅ **Multi-Asset** - Manage multiple assets in one database
✅ **State History** - Full audit trail of transitions
✅ **OpenAPI Documentation** - Interactive Swagger UI

## Quick Start

### 1. Build

```bash
# Build both CLI and API
go build -o bin/fsm cmd/fsm/main.go
go build -o bin/api cmd/api/main.go

# Create data directory for databases
mkdir -p data
```

### 2. Start REST API Server

```bash
# Start API server
./bin/api --port 8080 --db data/demo.db

# Server starts at http://localhost:8080
# Swagger UI: http://localhost:8080/docs
# OpenAPI spec: http://localhost:8080/openapi.yaml
```

### 3. Use the REST API

#### Create an Asset

```bash
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{
    "id": "web1",
    "asset_type": "web-server"
  }'
```

**Response:**
```json
{
  "id": "web1",
  "asset_type": "web-server",
  "current_state": "CREATED",
  "created_at": "2025-10-20T10:30:00Z"
}
```

#### Transition Through States

```bash
# CREATED → STARTING
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "STARTING"}'

# STARTING → RUNNING (triggers webhook)
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "RUNNING"}'
```

#### Get Asset Details

```bash
curl http://localhost:8080/api/v1/assets/web1
```

#### Get State History

```bash
curl http://localhost:8080/api/v1/assets/web1/history?limit=10
```

#### List All Assets

```bash
curl http://localhost:8080/api/v1/assets
```

#### Delete Asset

```bash
curl -X DELETE http://localhost:8080/api/v1/assets/web1
```

### 4. Use the Interactive CLI

#### Start CLI

```bash
# Start interactive CLI
./bin/fsm --db data/demo.db
```

#### Load an Asset

```
> load-asset web_server_asset_type.yaml web1
Asset loaded successfully
Asset ID: web1
FSM: generic_lifecycle
Initial state: CREATED
Current state: CREATED
Webhooks: enabled
```

#### Execute Transitions

```
> current-state
Current state: CREATED

> available
Available transitions:
  -> STARTING
  -> FAILED

> transition STARTING
Transitioned: CREATED -> STARTING

> transition RUNNING
Transitioned: STARTING -> RUNNING
```

#### Manage Multiple Assets

```
> load-asset database_asset_type.yaml db1
> transition STARTING
> transition RUNNING

> list-instances
Found 2 FSM instance(s):
  ID: web1
    FSM: generic_lifecycle
    Asset Type: web-server
    State: RUNNING

  ID: db1
    FSM: generic_lifecycle
    Asset Type: database
    State: RUNNING
```

#### Exit

```
> exit
Goodbye!
```

---

## Available Asset Types

The project includes three pre-configured asset types:

### 1. Web Server (`web-server`)

**File:** `examples/web_server_asset_type.yaml`

**Webhooks:** 5 configured
- Notify when server starts (on_enter RUNNING)
- Notify when server stops (on_exit RUNNING)
- Notify on startup initiation (on_transition CREATED→STARTING)
- Critical alert on failure (on_enter FAILED)
- Maintenance mode notification (on_enter MAINTENANCE)

**REST API:**
```bash
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{"id": "web1", "asset_type": "web-server"}'
```

**CLI:**
```bash
> load-asset web_server_asset_type.yaml web1
```

### 2. Database (`database`)

**File:** `examples/database_asset_type.yaml`

**Webhooks:** 4 configured
- Database available (on_enter RUNNING)
- Shutdown warning (on_transition RUNNING→STOPPING)
- Critical PagerDuty alert (on_enter FAILED)
- Termination notification (on_enter TERMINATED)

**REST API:**
```bash
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{"id": "db1", "asset_type": "database"}'
```

**CLI:**
```bash
> load-asset database_asset_type.yaml db1
```

### 3. Simple Service (`simple-service`)

**File:** `examples/simple_asset_type.yaml`

**Webhooks:** 1 configured
- Service started (on_enter RUNNING)

**REST API:**
```bash
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{"id": "service1", "asset_type": "simple-service"}'
```

**CLI:**
```bash
> load-asset simple_asset_type.yaml service1
```

---

## State Machine

All asset types use the `generic_lifecycle` state machine defined in `examples/lifecycle.yaml`.

### States

- `CREATED` - Initial state
- `STARTING` - Starting up
- `RUNNING` - Operational
- `STOPPING` - Shutting down
- `STOPPED` - Shut down
- `TERMINATING` - Being terminated
- `TERMINATED` - Final state
- `MAINTENANCE` - Under maintenance
- `FAILED` - Error state

### Transition Paths

**Main Sequence:**
```
CREATED → STARTING → RUNNING → STOPPING → STOPPED → TERMINATING → TERMINATED
```

**Maintenance Mode:**
```
RUNNING ↔ MAINTENANCE
STOPPED ↔ MAINTENANCE
```

**Recovery Paths:**
```
STOPPED → STARTING (restart)
FAILED → STARTING (recovery)
```

**Global Failure:**
```
* → FAILED (any state can transition to FAILED)
```

---

## Complete Workflow Examples

### REST API Complete Workflow

```bash
# 1. Start the API server
./bin/api --port 8080 --db data/demo.db

# 2. Create a web server asset
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{"id": "web1", "asset_type": "web-server"}'

# 3. Start the server
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "STARTING"}'

curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "RUNNING"}'

# 4. Enter maintenance mode
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "MAINTENANCE"}'

# 5. Return to running
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "RUNNING"}'

# 6. Shutdown sequence
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "STOPPING"}'

curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "STOPPED"}'

curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "TERMINATING"}'

curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "TERMINATED"}'

# 7. View history
curl http://localhost:8080/api/v1/assets/web1/history

# 8. Clean up
curl -X DELETE http://localhost:8080/api/v1/assets/web1
```

### CLI Complete Workflow

```bash
# Start CLI
./bin/fsm --db data/demo.db

# Load web server asset
> load-asset web_server_asset_type.yaml web1

# Check current state
> current-state
Current state: CREATED

# View available transitions
> available
Available transitions:
  → STARTING
  → FAILED

# Transition through lifecycle
> transition STARTING
Transitioned: CREATED -> STARTING

> transition RUNNING
Transitioned: STARTING -> RUNNING

> transition MAINTENANCE
Transitioned: RUNNING -> MAINTENANCE

> transition RUNNING
Transitioned: MAINTENANCE -> RUNNING

> transition STOPPING
Transitioned: RUNNING -> STOPPING

> transition STOPPED
> transition TERMINATING
> transition TERMINATED

# Load a database asset
> load-asset database_asset_type.yaml db1

# List all instances
> list-instances

# Exit
> exit
```

---

## Command-Line Reference

### REST API Server (`./bin/api`)

```bash
# Default settings
./bin/api

# Custom settings
./bin/api --port 8080 --db data/demo.db --asset-dir examples

# Show version
./bin/api --version
```

**Flags:**
- `--port` - HTTP server port (default: 8080)
- `--db` - SQLite database path (default: fsm.db)
- `--asset-dir` - Directory containing asset type YAML files (default: examples)
- `--version` - Show version and exit

### Interactive CLI (`./bin/fsm`)

```bash
# Start interactive mode
./bin/fsm

# With database persistence
./bin/fsm --db data/demo.db

# Load asset on startup
./bin/fsm --asset-type web_server_asset_type.yaml --id web1 --db data/demo.db

# Custom asset directory
./bin/fsm --asset-dir /path/to/assets --db data/demo.db
```

**Flags:**
- `--asset-type` - Asset type file name to load on startup
- `--asset-dir` - Directory containing asset type YAML files (default: examples)
- `--db` - SQLite database path (optional, uses in-memory if not specified)
- `--id` - Asset instance ID (required when using --asset-type)

**Interactive Commands:**
```
load-asset <name> <id>    Load asset type with instance ID
transition <state>        Transition to a new state
current-state             Show current state
states                    List all states
available                 Show available transitions
reset                     Reset to initial state
final                     Check if in final state
list-instances            Show all persisted instances
load-instance <id>        Load existing instance by ID
delete-instance <id>      Delete a persisted instance
validate <path>           Validate FSM YAML definition
help                      Show available commands
exit                      Exit program
```

---

## REST API Endpoints

### Health Check
- `GET /api/v1/health` - Server health status

### Asset Management
- `POST /api/v1/assets` - Create new asset instance
- `GET /api/v1/assets` - List all assets
- `GET /api/v1/assets/{id}` - Get asset details
- `DELETE /api/v1/assets/{id}` - Delete asset

### State Operations
- `POST /api/v1/assets/{id}/transition` - Execute state transition
- `GET /api/v1/assets/{id}/history?limit=N` - Get state history

### Documentation
- `GET /docs` - Interactive Swagger UI
- `GET /openapi.yaml` - OpenAPI 3.0 specification

---

## Webhooks

Webhooks are dispatched asynchronously in background workers. State transitions succeed even if webhooks fail.

### Webhook Conditions

**on_enter** - Triggers when entering a specific state
```yaml
- condition: on_enter
  state: RUNNING
  url: https://example.com/running
  method: POST
  timeout: 5s
```

**on_exit** - Triggers when exiting a specific state
```yaml
- condition: on_exit
  state: RUNNING
  url: https://example.com/stopped
  method: POST
  timeout: 5s
```

**on_transition** - Triggers on a specific state transition
```yaml
- condition: on_transition
  from: CREATED
  to: STARTING
  url: https://example.com/starting
  method: POST
  timeout: 10s
```

### Webhook Payload

```json
{
  "asset_id": "web1",
  "asset_type": "web-server",
  "from_state": "STARTING",
  "to_state": "RUNNING",
  "timestamp": "2025-10-20T10:31:00Z"
}
```

### Testing Webhooks

```bash
# Terminal 1: Start webhook receiver
python3 bin/webhook.py

# Terminal 2: Start API server
./bin/api --port 8080 --db data/demo.db

# Terminal 3: Trigger webhooks
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{"id": "test1", "asset_type": "simple-service"}'

curl -X POST http://localhost:8080/api/v1/assets/test1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "STARTING"}'

curl -X POST http://localhost:8080/api/v1/assets/test1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "RUNNING"}'
# Webhook fires → received by webhook.py
```

---

## Asset Type Definition Format

Asset Types link FSM definitions with webhook configurations.

### Example: Web Server Asset Type

**File:** `examples/web_server_asset_type.yaml`

```yaml
version: 1
name: web-server
state_machine: lifecycle.yaml

webhooks:
  # Notify when server enters RUNNING state
  - condition: on_enter
    state: RUNNING
    url: https://example.com/webhooks/server-running
    method: POST
    timeout: 5s
    headers:
      X-Event-Type: server-running
      Authorization: Bearer your-webhook-token-here

  # Notify when server exits RUNNING state
  - condition: on_exit
    state: RUNNING
    url: https://example.com/webhooks/server-stopped
    method: POST
    timeout: 5s
    headers:
      X-Event-Type: server-stopped

  # Notify on specific CREATED → STARTING transition
  - condition: on_transition
    from: CREATED
    to: STARTING
    url: https://example.com/webhooks/server-starting
    method: POST
    timeout: 10s

  # Notify when server enters FAILED state
  - condition: on_enter
    state: FAILED
    url: https://example.com/webhooks/server-failed
    method: POST
    timeout: 5s
    headers:
      X-Event-Type: server-failed
      X-Alert-Level: critical
```

### FSM Definition Format

**File:** `examples/lifecycle.yaml`

```yaml
version: 1
name: generic_lifecycle
initial: CREATED
final:
  - TERMINATED

states:
  - CREATED
  - STARTING
  - RUNNING
  - STOPPING
  - STOPPED
  - TERMINATING
  - TERMINATED
  - MAINTENANCE
  - FAILED

transitions:
  # Main sequence
  - from: CREATED
    to: STARTING
  - from: STARTING
    to: RUNNING
  - from: RUNNING
    to: STOPPING
  - from: STOPPING
    to: STOPPED
  - from: STOPPED
    to: TERMINATING
  - from: TERMINATING
    to: TERMINATED

  # Maintenance bidirectional transitions
  - from: RUNNING
    to: MAINTENANCE
  - from: MAINTENANCE
    to: RUNNING

  # Recovery paths
  - from: STOPPED
    to: STARTING
  - from: FAILED
    to: STARTING

  # Global failure (any state → FAILED)
  - from: "*"
    to: FAILED
```

---

## Architecture

### Project Structure

```
state_guard/
├── cmd/
│   ├── api/main.go           # REST API server
│   └── fsm/main.go           # Interactive CLI
├── internal/
│   ├── api/                  # API handlers and server
│   │   ├── server.go         # Chi router with middleware
│   │   ├── handlers.go       # Endpoint handlers
│   │   └── models.go         # Request/response models
│   ├── fsm/                  # Core FSM engine
│   │   ├── definition.go     # YAML parsing
│   │   ├── fsm.go           # State machine logic
│   │   └── asset_type.go    # Asset type definitions
│   ├── storage/             # Persistence layer
│   │   ├── storage.go       # Storage interface
│   │   └── sqlite.go        # SQLite implementation
│   └── webhook/             # Webhook system
│       ├── dispatcher.go    # Worker pool
│       ├── sink.go          # Sink interface
│       └── http_sink.go     # HTTP implementation
├── examples/                # Example configurations
│   ├── lifecycle.yaml
│   ├── web_server_asset_type.yaml
│   ├── database_asset_type.yaml
│   └── simple_asset_type.yaml
├── data/                    # Database files (gitignored)
├── bin/                     # Compiled binaries
└── docs/
    └── openapi.yaml         # OpenAPI 3.0 spec
```

### Key Components

**FSM Engine** (`internal/fsm`)
- YAML-based configuration
- Thread-safe state transitions
- Wildcard support (`*` → FAILED)
- Validation and error handling

**Storage Layer** (`internal/storage`)
- Interface-based design
- SQLite with WAL mode
- Atomic state updates
- Transaction support

**Webhook System** (`internal/webhook`)
- Asynchronous dispatch
- Worker pool (5 concurrent workers)
- Timeout protection
- Non-blocking notifications

**REST API** (`internal/api`)
- Chi router framework
- Middleware stack (logging, recovery, timeout)
- OpenAPI 3.0 documentation
- CORS support

---

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Check test coverage
go test ./... -cover

# Generate coverage report
go test ./internal/fsm/... -coverprofile=coverage.out
go tool cover -html=coverage.out
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

### Code Quality

```bash
# Format code
go fmt ./...

# Run linter
golint ./...

# Static analysis
go vet ./...
```

---

## Performance

- **Worker Pool**: 5 concurrent webhook workers
- **HTTP Timeouts**: Configurable per webhook (default: 5s)
- **Database**: SQLite with WAL mode
- **Transactions**: Atomic state updates with rollback

---

## Documentation

- **Usage Guide**: [USAGE.md](USAGE.md) - Comprehensive usage documentation
- **Implementation Plan**: [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) - Development phases
- **Project Guide**: [CLAUDE.md](CLAUDE.md) - Development commands
- **OpenAPI Spec**: [docs/openapi.yaml](docs/openapi.yaml) - REST API specification

---

## License

[Specify License]

## Authors

[Specify Authors]
