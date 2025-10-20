# FSM Usage Guide

## Overview

This guide explains how to use the FSM (Finite State Machine) system to manage assets through state transitions. The system supports two interfaces:

1. **REST API** - Production-ready HTTP API with Swagger documentation
2. **CLI** - Interactive command-line interface for testing and development

## Core Concepts

### Data Model

1. **State Machine** - Defines states and transitions (e.g., `lifecycle.yaml`)
2. **Asset Type** - Links a state machine with webhook configurations
3. **Asset** - An instance with a unique ID and asset type

### Architecture Flow

```
Asset Type YAML → State Machine + Webhooks
                ↓
        Asset Instance (ID)
                ↓
        State Transitions
                ↓
        Webhooks Triggered (async)
```

---

## 1. REST API Usage (Recommended)

The REST API is the primary interface for production deployments.

### Starting the API Server

#### Option 1: Using Pre-built Binary

```bash
# Start with defaults (port 8080, fsm.db in current dir, examples/ directory)
./bin/api

# Start with database in data directory (recommended)
./bin/api --db data/demo.db

# Start with all custom settings
./bin/api --port 8080 --db data/demo.db --asset-dir examples
```

#### Option 2: Using Go Run

```bash
# Development mode
go run cmd/api/main.go

# With custom settings
go run cmd/api/main.go --port 8080 --db data/demo.db --asset-dir examples
```

#### Command-Line Flags

- `--port` - HTTP server port (default: 8080)
- `--db` - Path to SQLite database file (default: fsm.db)
- `--asset-dir` - Directory containing asset type YAML files (default: examples)
- `--version` - Show version and exit

### API Documentation

Once the server is running:

- **Interactive Swagger UI**: http://localhost:8080/docs
- **OpenAPI Specification**: http://localhost:8080/openapi.yaml
- **Health Check**: http://localhost:8080/api/v1/health

### API Endpoints

#### Create Asset

```bash
# Create a web server asset
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

#### List All Assets

```bash
curl http://localhost:8080/api/v1/assets
```

**Response:**
```json
{
  "assets": [
    {
      "id": "web1",
      "asset_type": "web-server",
      "current_state": "CREATED"
    }
  ]
}
```

#### Get Asset Details

```bash
curl http://localhost:8080/api/v1/assets/web1
```

#### Transition Asset State

```bash
# Transition web1 from CREATED to STARTING
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{
    "to_state": "STARTING"
  }'

# Transition to RUNNING (triggers webhook)
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{
    "to_state": "RUNNING"
  }'
```

**Response:**
```json
{
  "id": "web1",
  "asset_type": "web-server",
  "previous_state": "STARTING",
  "current_state": "RUNNING",
  "timestamp": "2025-10-20T10:31:00Z"
}
```

#### Get State History

```bash
# Get last 10 transitions
curl http://localhost:8080/api/v1/assets/web1/history?limit=10
```

**Response:**
```json
{
  "asset_id": "web1",
  "history": [
    {
      "from_state": "STARTING",
      "to_state": "RUNNING",
      "timestamp": "2025-10-20T10:31:00Z"
    },
    {
      "from_state": "CREATED",
      "to_state": "STARTING",
      "timestamp": "2025-10-20T10:30:30Z"
    }
  ]
}
```

#### Delete Asset

```bash
curl -X DELETE http://localhost:8080/api/v1/assets/web1
```

### Complete REST API Workflow

```bash
# 1. Start the API server
./bin/api --port 8080 --db data/demo.db

# 2. Create a web server asset
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{"id": "web1", "asset_type": "web-server"}'

# 3. Transition through lifecycle
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "STARTING"}'

curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "RUNNING"}'
# ↑ Webhook fires: on_enter RUNNING

# 4. Check state history
curl http://localhost:8080/api/v1/assets/web1/history

# 5. Transition to maintenance
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "MAINTENANCE"}'

# 6. Return to running
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "RUNNING"}'
# ↑ Webhook fires: on_enter RUNNING

# 7. Shutdown sequence
curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "STOPPING"}'
# ↑ Webhook fires: on_exit RUNNING

curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "STOPPED"}'

curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "TERMINATING"}'

curl -X POST http://localhost:8080/api/v1/assets/web1/transition \
  -H "Content-Type: application/json" \
  -d '{"to_state": "TERMINATED"}'

# 8. View final history
curl http://localhost:8080/api/v1/assets/web1/history

# 9. Clean up
curl -X DELETE http://localhost:8080/api/v1/assets/web1
```

---

## 2. CLI Usage (Interactive Testing)

The CLI provides an interactive interface for testing and development.

### Starting the CLI

#### Option 1: Using Pre-built Binary

```bash
# Start with defaults (asset-dir: examples)
./bin/fsm

# Start with custom database
./bin/fsm --db data/test.db

# Start with custom asset directory
./bin/fsm --asset-dir examples --db data/test.db

# Load asset directly on startup
./bin/fsm --asset-type web_server_asset_type.yaml --id web1 --db data/test.db
```

#### Option 2: Using Go Run

```bash
# Development mode
go run cmd/fsm/main.go

# With custom settings
go run cmd/fsm/main.go --asset-dir examples --db data/test.db
```

#### Command-Line Flags

- `--asset-type` - Asset type file name to load on startup
- `--asset-dir` - Directory containing asset type YAML files (default: examples)
- `--db` - Path to SQLite database file (optional, uses in-memory if not specified)
- `--id` - Asset instance ID (required when using --asset-type)

### Interactive Commands

Once the CLI is running, you'll see a prompt where you can enter commands.

#### Asset Management

```bash
# Load an asset type (creates or loads existing asset)
# Asset type files are loaded from --asset-dir (default: examples)
load-asset web_server_asset_type.yaml web1

# Load a different asset
load-asset database_asset_type.yaml db1

# List all persisted instances
list-instances

# Switch to a different instance (asset type auto-loaded if stored)
load-instance web1

# Or specify asset type explicitly
load-instance web1 web_server_asset_type.yaml

# Delete an instance
delete-instance web1
```

#### State Operations

```bash
# Show current state
current-state

# List all valid states
states

# Show available transitions from current state
available

# Perform a state transition
transition STARTING
# ↑ Webhook fires: on_enter STARTING with error as no one listens on target port

transition RUNNING
# ↑ Webhook fires: on_enter RUNNING with error as no one listens on target port

# Reset to initial state
reset

# Check if in final state
final
```

#### Validation

```bash
# Validate an FSM definition
validate examples/lifecycle.yaml
```

#### Help and Exit

```bash
# Show help
help

# Exit CLI
exit
# or
quit
```

### Complete CLI Workflow

```bash
# Start the CLI with persistence
./bin/fsm --db data/demo.db

# Load a web server asset (from examples/ directory)
> load-asset web_server_asset_type.yaml web1
Asset type loaded: web-server
FSM loaded: generic_lifecycle
Instance ID: web1
Webhooks: enabled (5 configured)

# Check current state
> current-state
Current state: CREATED

# View available transitions
> available
Available transitions from CREATED:
  → STARTING
  → FAILED

# Transition through lifecycle
> transition STARTING
Transitioned: CREATED -> STARTING

> transition RUNNING
Transitioned: STARTING -> RUNNING
# Webhook fires: on_enter RUNNING

# Enter maintenance mode
> transition MAINTENANCE
Transitioned: RUNNING -> MAINTENANCE

# Return to running
> transition RUNNING
Transitioned: MAINTENANCE -> RUNNING
# Webhook fires: on_enter RUNNING

# Shutdown sequence
> transition STOPPING
Transitioned: RUNNING -> STOPPING
# Webhook fires: on_exit RUNNING

> transition STOPPED
> transition TERMINATING
> transition TERMINATED

# Check final state
> final
true

# Load a database asset (from examples/ directory)
> load-asset database_asset_type.yaml db1
Asset type loaded: database
FSM loaded: generic_lifecycle
Instance ID: db1
Webhooks: enabled (4 configured)

# List all instances
> list-instances
Instances in database:
  - web1 (asset_type: web-server, state: TERMINATED)
  - db1 (asset_type: database, state: CREATED)

# Exit
> exit
```

---

## Asset Type Examples

The project includes three pre-configured asset types in the `examples/` directory:

### 1. Web Server (`web_server_asset_type.yaml`)

**State Machine**: `lifecycle.yaml`

**Webhooks** (5 configured):
- `on_enter RUNNING` - Notify when server starts
- `on_exit RUNNING` - Notify when server stops
- `on_transition CREATED→STARTING` - Notify on startup initiation
- `on_enter FAILED` - Critical alert on failure
- `on_enter MAINTENANCE` - Notify maintenance mode

**Usage**:
```bash
# REST API
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{"id": "web1", "asset_type": "web-server"}'

# CLI (asset files loaded from examples/ directory by default)
load-asset web_server_asset_type.yaml web1
```

### 2. Database (`database_asset_type.yaml`)

**State Machine**: `lifecycle.yaml`

**Webhooks** (4 configured):
- `on_enter RUNNING` - Database becomes available
- `on_transition RUNNING→STOPPING` - Database shutting down warning
- `on_enter FAILED` - Critical PagerDuty alert
- `on_enter TERMINATED` - Database terminated notification

**Usage**:
```bash
# REST API
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{"id": "db1", "asset_type": "database"}'

# CLI
load-asset database_asset_type.yaml db1
```

### 3. Simple Service (`simple_asset_type.yaml`)

**State Machine**: `lifecycle.yaml`

**Webhooks** (1 configured):
- `on_enter RUNNING` - Notify when service starts

**Usage**:
```bash
# REST API
curl -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{"id": "service1", "asset_type": "simple-service"}'

# CLI
load-asset simple_asset_type.yaml service1
```

---

## State Machine Reference

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

### Transitions

**Main Sequence**:
```
CREATED → STARTING → RUNNING → STOPPING → STOPPED → TERMINATING → TERMINATED
```

**Maintenance Mode**:
```
RUNNING ↔ MAINTENANCE
STOPPED ↔ MAINTENANCE
```

**Recovery Paths**:
```
STOPPED → STARTING (restart)
FAILED → STARTING (recovery)
```

**Global Failure**:
```
* → FAILED (any state can transition to FAILED)
```

---

## Webhook Behavior

### Webhook Conditions

1. **on_enter** - Triggers when entering a specific state
   ```yaml
   - condition: on_enter
     state: RUNNING
     url: https://example.com/running
   ```

2. **on_exit** - Triggers when exiting a specific state
   ```yaml
   - condition: on_exit
     state: RUNNING
     url: https://example.com/stopped
   ```

3. **on_transition** - Triggers on a specific state transition
   ```yaml
   - condition: on_transition
     from: CREATED
     to: STARTING
     url: https://example.com/starting
   ```

### Webhook Architecture

- **Asynchronous**: Webhooks are dispatched in background workers
- **Non-blocking**: State transitions succeed even if webhooks fail
- **Worker Pool**: 5 concurrent webhook workers
- **Timeout Protection**: Configurable timeout (default: 5s)
- **Retry**: No automatic retries (webhooks fire once per transition)

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

Use the included webhook receiver for testing:

```bash
# Terminal 1: Start webhook receiver
python3 bin/webhook.py

# Terminal 2: Start API server
./bin/api --port 8080

# Terminal 3: Create asset and trigger webhooks
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

## Persistence

Both the REST API and CLI use SQLite for persistence.

### Features

- **Transactional**: State changes are atomic (either fully saved or rolled back)
- **Multi-Asset**: One database can manage multiple asset instances
- **History Tracking**: Every state transition is recorded with timestamp
- **Crash Recovery**: State is persisted before transition completes

### Database Schema

The SQLite database contains tables for:
- **assets** - Asset instances (id, asset_type, current_state)
- **state_history** - Transition history (asset_id, from_state, to_state, timestamp)

### Database Location

- REST API: Specified by `--db` flag (default: `fsm.db` in current directory)
- CLI: Specified by `--db` flag (default: in-memory if not specified)
- **Recommended**: Store databases in `data/` directory (e.g., `data/demo.db`, `data/test.db`)

---

## Troubleshooting

### REST API

**Error: Address already in use**
```bash
# Change the port
./bin/api --port 8081
```

**Error: Asset type not found**
```bash
# Ensure asset type YAML exists in asset-dir
ls examples/*.yaml

# Or specify custom asset directory
./bin/api --asset-dir /path/to/asset/types
```

**Webhook not firing**
1. Check webhook configuration in asset type YAML
2. Verify webhook URL is accessible
3. Check timeout settings (increase if needed)
4. Test with local webhook receiver: `python3 bin/webhook.py`

### CLI

**Error: --id is required**
```bash
# Always provide an ID when loading an asset
load-asset web_server_asset_type.yaml web1
```

**Error: Invalid transition**
```bash
# Check available transitions
available

# View all valid states
states
```

**Cannot load instance**
```bash
# Make sure database file exists
ls -la data/*.db

# Verify instance exists
list-instances
```

---

## Features Summary

✅ **REST API** - Production-ready HTTP API with Chi router
✅ **Interactive CLI** - REPL for testing and development
✅ **Asset Types** - State machine + webhook configuration
✅ **User-specified IDs** - Meaningful asset names (web1, db-prod, etc.)
✅ **Webhooks** - HTTP notifications on state changes
✅ **Asynchronous Dispatch** - Non-blocking webhook delivery
✅ **Worker Pool** - 5 concurrent webhook workers
✅ **Timeout Protection** - Prevents blocking connections
✅ **Persistence** - SQLite with transactional guarantees
✅ **Multi-Asset** - Manage multiple assets in one database
✅ **State History** - Full audit trail of transitions
✅ **OpenAPI Documentation** - Interactive Swagger UI
✅ **Graceful Shutdown** - Clean server shutdown on SIGINT/SIGTERM

---

## Next Steps

1. **Create data directory**: `mkdir -p data`
2. **Start the REST API**: `./bin/api --db data/demo.db`
3. **Explore the Swagger UI**: http://localhost:8080/docs
4. **Create your first asset**: Use the examples above
5. **Test webhooks**: Run `python3 bin/webhook.py` to see webhook notifications
6. **Try the CLI**: `./bin/fsm --db data/demo.db` for interactive testing

For more details, see:
- `IMPLEMENTATION_PLAN.md` - Development phases and technical details
- `CLAUDE.md` - Development commands and project structure
- `examples/*.yaml` - Asset type and FSM definitions
