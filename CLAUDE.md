# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based Finite State Machine (FSM) implementation designed to process state changes following YAML definitions. The project includes REST API capabilities and a persistence layer. Development follows a phased approach with clear acceptance criteria for each phase.

## Project Status

**Current Phase:** Pre-implementation (Phase 1.1 planned)
- Repository initialized with implementation plan
- No code written yet

## Development Commands

### Setup
```bash
# Initialize Go module (when starting Phase 1.1)
go mod init github.com/yourusername/fsm_v2

# Install dependencies
go mod download

# Tidy dependencies
go mod tidy
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage (required >80% per phase)
go test -cover ./...

# Run tests with verbose output
go test -v ./...

# Run tests for specific package
go test ./internal/fsm/...
```

### Build
```bash
# Build CLI application (Phase 1.1+)
go build -o bin/fsm cmd/fsm/main.go

# Build API server (Phase 2.2+)
go build -o bin/api cmd/api/main.go

# Build all binaries
go build -o bin/ ./cmd/...
```

### Running
```bash
# Run CLI (Phase 1.1+)
go run cmd/fsm/main.go --config examples/lifecycle.yaml

# Run interactive console (Phase 2.3+)
go run cmd/fsm/main.go

# Run API server (Phase 2.2+)
go run cmd/api/main.go --port 8080
```

### Code Quality
```bash
# Format code
gofmt -w .

# Run linter
golint ./...

# Run static analysis
go vet ./...

# Run staticcheck (install: go install honnef.co/go/tools/cmd/staticcheck@latest)
staticcheck ./...
```

## Architecture

### Project Structure
```
fsm_v2/
├── cmd/
│   ├── fsm/          # CLI application with interactive console
│   └── api/          # REST API server (Phase 2.2+)
├── internal/
│   ├── fsm/          # Core FSM engine (state validation, transitions)
│   ├── console/      # Interactive console with readline support (Phase 2.3)
│   ├── api/          # REST API handlers (Phase 2.2+)
│   ├── storage/      # Persistence layer - SQLite (Phase 1.2+)
│   └── auth/         # JWT authentication and RBAC (Phase 3+)
├── pkg/
│   └── client/       # Client SDK (Phase 5+)
├── examples/         # Example FSM YAML definitions
├── docs/             # Documentation and ADRs
└── tests/            # Integration tests
```

### Core FSM Engine (internal/fsm)

The FSM engine is the heart of the system:

**Key Components:**
- **FSM Definition Parser:** Loads and validates YAML FSM configurations
- **State Validator:** Ensures transitions follow configured rules
- **Transition Engine:** Executes state changes with validation
- **Wildcard Support:** Handles `from: "*"` transitions (any state → target)

**FSM YAML Structure:**
```yaml
version: 1
name: <fsm-name>
initial: <initial-state>
final:
  - <final-state1>
  - <final-state2>

states:
  - STATE1
  - STATE2
  ...

transitions:
  - from: STATE1
    to: STATE2
  - from: "*"        # Wildcard: any state can transition to this
    to: FAILED
```

**Design Principles:**
1. FSM definitions are immutable once loaded (hot-reload is Phase 5 feature)
2. All transitions must be explicitly defined (except wildcards)
3. Invalid transitions return errors, not silent failures
4. State history is append-only (Phase 2.1+)

### Data Model

1. State Machine
- name
- states
- transitions

2. Asset Type
- name
- state_machine

3. Asset 
- name
- asset_type


### Phased Development

Development proceeds in strict phases (see IMPLEMENTATION_PLAN.md for full details):

**Phase 1.1 (Foundation):**
- Core FSM engine with YAML parsing
- FSM definition available in YAML file
- State transition validation
- FSM instance identified by unique IDs
- Basic CLI for testing (commands: load, transition, current-state, validate)
- Unit tests implemented with target: >80% test coverage

**Phase 1.2 (Persistence):**
- Each state change is transactional
- When persistence fails state cannot be changed
- Persistence supports multiple assets that are tracked by state machine
- SQLite backend with option to change in the future
- Unit tests implemented with target: >80% test coverage
- Persistence is prepared for state history (Phase 2.1)

**Phase 1.3 (WebHook Dispatcher):**
- provide handling Webhooks to be notified about certain conditions 
- initial conditions: on_enter(state), on_exit(state), on_transition(from→to)
- provides generic sink interface for extensibility
- initial implementation with HTTP(s) webhook
- build-in protection from blocking connection e.g. http.Client.Timeout 
- use worker pool to handle notifications
- use yaml configuration to specify webhooks
- webhooks are defined per asset_type
- Unit tests implemented with target: >80% test coverage

**Phase 2.1 (History):**
- Timestamped state transition tracking
- Query capabilities (time range, state)
- Export to JSON/CSV
- History rotation
- Unit tests implemented with target: >80% test coverage

**Phase 2.2 (REST API):**
- REST API layer for multi-instance FSM management
- Persistent storage using Phase 1.2 implementation
- Endpoints: create, get, transition, history, delete
- OpenAPI/Swagger documentation
- Unit tests implemented with target: >80% test coverage


  Phase 2.2 (REST API):
  - REST API layer for multi-instance FSM management
  - Chi router framework (go-chi/chi/v5) with professional middleware stack
    - Request ID tracking, logging, panic recovery, timeout protection
    - Declarative route definitions with automatic URL parameter extraction
  - Persistent storage using Phase 1.2 implementation
  - Endpoints: create, get, list, delete, transition, history
  - Request validation with render.Binder interface
  - CORS support for cross-origin requests
  - OpenAPI 3.0 documentation (docs/openapi.yaml)
  - Comprehensive error handling with JSON error responses
  - Unit tests implemented with 66.7% coverage (13 tests passing)
  - Integration with Asset Types and Webhooks from Phase 1.3
  - Production-ready with graceful shutdown
  
**Phase 2.3 (Enhanced Console):**
- Interactive REPL with readline
- Tab completion for commands/states
- Command history (arrow keys)
- Commands: /load, /transition, /current, /history, /help, /exit
- Unit tests implemented with target: >80% test coverage

**Phase 3 (Security):**
- JWT authentication
- RBAC (admin, user, readonly roles)
- Rate limiting
- Input validation
- Unit tests implemented with target: >80% test coverage

**Phase 4 (Observability):**
- Health endpoints
- Prometheus metrics
- Structured logging
- Webhooks for state changes
- Unit tests implemented with target: >80% test coverage

**Phase 5 (Advanced):**
- Bulk operations
- Hot-reload configurations
- Event hooks (before/after transition)
- Client SDKs (Go, Python, JavaScript)
- Unit tests implemented with target: >80% test coverage

### Testing Requirements

Each phase requires:
- **Unit tests:** >80% code coverage
- **Table-driven tests:** Use Go's table-driven test pattern
- **Integration tests:** For API and database operations (Phase 2+)
- **Load tests:** Performance benchmarks (Phase 2.2+)

## Important Notes

1. **Strict Phase Ordering:** Do not implement features from later phases until current phase acceptance criteria are met
2. **Test Coverage:** Every phase must achieve >80% test coverage before moving forward
3. **YAML as Source of Truth:** FSM behavior is defined entirely by YAML configuration
4. **Go Standards:** Follow standard Go project layout and idioms
5. **Error Handling:** All errors must be properly wrapped and logged
6. **Concurrency:** Design for concurrent FSM instance operations from the start
7. **No Premature Optimization:** Focus on correctness first, optimize in later phases

## Example FSM Configuration

See IMPLEMENTATION_PLAN.md for the `generic_lifecycle.yaml` example showing:
- Main sequence transitions (CREATED → STARTING → RUNNING → STOPPING → STOPPED → TERMINATING → TERMINATED)
- Maintenance mode (RUNNING ↔ MAINTENANCE)
- Recovery paths (STOPPED → STARTING, FAILED → STARTING)
- Global failure transition (* → FAILED)
