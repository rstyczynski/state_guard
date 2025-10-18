# FSM CLI Usage Guide

## Overview

The FSM CLI manages assets using Asset Types, which define both the state machine and webhook notifications.

## Core Concepts

### Data Model

1. **State Machine** - Defines states and transitions
2. **Asset Type** - Links state machine + webhook configuration
3. **Asset** - An instance with a unique ID and asset type

### Architecture Flow

```
Asset Type YAML → State Machine + Webhooks
                ↓
        Asset Instance (ID)
                ↓
        State Transitions
                ↓
        Webhooks Triggered
```

## Command-Line Usage

### Load Asset Type (Recommended)

```bash
# Create new asset instance
./fsm --asset-type examples/web_server_asset_type.yaml --id server1

# With persistence
./fsm --asset-type examples/web_server_asset_type.yaml --id server1 --db /path/to/db.sqlite
```

### Direct FSM Loading (Deprecated)

```bash
# Load FSM directly (no webhooks)
./fsm --config examples/lifecycle.yaml --id server1
```

**Note:** Using `--config` directly bypasses asset types and webhooks.

## Interactive Commands

### Asset Management

```bash
# Load asset type with webhooks
load-asset <asset-type-path> <id>
# Example:
load-asset examples/web_server_asset_type.yaml server1

# Load FSM directly (deprecated)
load <fsm-path> <id>
# Example:
load examples/lifecycle.yaml server1
```

### State Operations

```bash
# Show current state
current-state

# List all states
states

# Show available transitions
available

# Perform transition
transition <state>
# Example:
transition RUNNING

# Reset to initial state
reset

# Check if in final state
final
```

### Instance Management (requires --db)

```bash
# List all persisted instances
list-instances

# Load existing instance with asset type
load-instance <id> <asset-type-path>
# Example:
load-instance server1 examples/web_server_asset_type.yaml

# Delete instance
delete-instance <id>
```

### Validation

```bash
# Validate FSM definition
validate <fsm-path>
# Example:
validate examples/lifecycle.yaml
```

## Asset Type Examples

### Web Server with Webhooks

File: `examples/web_server_asset_type.yaml`

```yaml
version: 1
name: web-server
state_machine: examples/lifecycle.yaml

webhooks:
  - condition: on_enter
    state: RUNNING
    url: https://example.com/webhooks/server-running
    method: POST
    timeout: 5s
    headers:
      X-Event-Type: server-running
```

Usage:

```bash
./fsm --asset-type examples/web_server_asset_type.yaml --id web1 --db servers.db
```

Interactive:

```bash
> load-asset examples/web_server_asset_type.yaml web1
> transition STARTING
> transition RUNNING   # Webhook fires on entering RUNNING
```

### Database Asset Type

File: `examples/database_asset_type.yaml`

```yaml
version: 1
name: database
state_machine: examples/lifecycle.yaml

webhooks:
  - condition: on_enter
    state: FAILED
    url: https://pagerduty.example.com/webhooks/critical-alert
    method: POST
    timeout: 5s
    headers:
      X-Alert-Level: critical
```

### Simple Asset Type (No Webhooks)

File: `examples/simple_asset_type.yaml`

```yaml
version: 1
name: simple-service
state_machine: examples/lifecycle.yaml
# Webhooks are optional
```

## Webhook Conditions

### on_enter

Triggers when **entering** a specific state.

```yaml
webhooks:
  - condition: on_enter
    state: RUNNING
    url: https://example.com/running
```

### on_exit

Triggers when **exiting** a specific state.

```yaml
webhooks:
  - condition: on_exit
    state: RUNNING
    url: https://example.com/stopped
```

### on_transition

Triggers on a **specific transition**.

```yaml
webhooks:
  - condition: on_transition
    from: CREATED
    to: STARTING
    url: https://example.com/starting
```

## Workflow Examples

### Managing Multiple Assets

```bash
# Start interactive mode with persistence
./fsm --db infrastructure.db

# Load different asset types
> load-asset examples/web_server_asset_type.yaml web1
> transition STARTING
> transition RUNNING

> load-asset examples/database_asset_type.yaml db1
> transition STARTING
> transition RUNNING

# View all instances
> list-instances

# Switch between instances
> load-instance web1 examples/web_server_asset_type.yaml
> current-state
```

### Complete Lifecycle

```bash
./fsm --asset-type examples/web_server_asset_type.yaml --id prod-server --db production.db
```

```
> current-state
Current state: CREATED

> transition STARTING
Transitioned: CREATED -> STARTING

> transition RUNNING
Transitioned: STARTING -> RUNNING
# Webhook fires: on_enter RUNNING

> transition MAINTENANCE
Transitioned: RUNNING -> MAINTENANCE

> transition RUNNING
Transitioned: MAINTENANCE -> RUNNING
# Webhook fires: on_enter RUNNING

> transition STOPPING
Transitioned: RUNNING -> STOPPING
# Webhook fires: on_exit RUNNING

> transition STOPPED
> transition TERMINATING
> transition TERMINATED
```

## Persistence

When using `--db`:

- Each state transition is saved to SQLite
- State changes are **transactional** (if persistence fails, state doesn't change)
- Can load instances later with `load-instance`
- Full state history is recorded

## Features

✅ **Asset Types** - State machine + webhook configuration
✅ **User-specified IDs** - Meaningful asset names (server1, db-prod, etc.)
✅ **Webhooks** - HTTP notifications on state changes
✅ **Worker Pool** - 5 concurrent webhook workers
✅ **Timeout Protection** - Prevents blocking connections
✅ **Persistence** - SQLite with transactional guarantees
✅ **Multi-Asset** - Manage multiple assets in one database
✅ **Backward Compatible** - Old `--config` flag still works

## Migration from Direct FSM Loading

**Old way (deprecated):**

```bash
./fsm --config examples/lifecycle.yaml --id server1
```

**New way (recommended):**

1. Create asset type YAML:

```yaml
version: 1
name: my-asset-type
state_machine: examples/lifecycle.yaml
# Add webhooks if needed
```

2. Use asset type:

```bash
./fsm --asset-type my-asset-type.yaml --id server1
```

## Exit Commands

```bash
exit    # Exit CLI
quit    # Exit CLI
help    # Show help
```

## Troubleshooting

### Error: --id is required

You must provide an instance ID when loading:

```bash
./fsm --asset-type examples/web_server_asset_type.yaml --id my-server
```

### Webhook Not Firing

1. Check webhook configuration in asset type YAML
2. Verify webhook URL is accessible
3. Check timeout settings (default: 5s)
4. Look for "Webhooks: enabled" message on load

### Cannot Load Instance

Make sure to provide the **asset type path**, not FSM path:

```bash
# Correct
load-instance server1 examples/web_server_asset_type.yaml

# Incorrect (old way)
load-instance server1 examples/lifecycle.yaml
```
