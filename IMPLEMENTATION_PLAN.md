# Go Finite State Machine (FSM)

A comprehensive Go-based Finite State Machine implementation that processes state changes following YAML definitions, with REST API and persistence capabilities.

## Requirements

Implementation is done in phases, as defined below. Each phase is concluded by comprehensive unit tests and has clear acceptance criteria.

---

## Phase 1: Core FSM Implementation

### Phase 1.1: FSM Core Engine
**Status:** Foundation
**Dependencies:** None

**Features:**
- Go library with minimalistic CLI
- YAML-based FSM configuration
- State transition validation
- Support for wildcard transitions (from: "*")

**Acceptance Criteria:**
- ✓ Load and parse YAML FSM definitions
- ✓ Validate state transitions against configuration
- ✓ Support wildcard transitions (any state → target)
- ✓ Basic CLI for testing transitions
- ✓ Unit tests with >80% coverage

**Example YAML Configuration:**
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

  # Dotted backward transitions
  - from: STOPPED
    to: STARTING

  - from: FAILED
    to: STARTING

  # ANY → FAILED
  - from: "*"
    to: FAILED
```

### Phase 1.2: Enhanced Console Interface
**Status:** In Development
**Dependencies:** Phase 1.1

**Features:**
- Interactive console interface
- Command Interface: Add `/command` prefix support for console commands
- Tab autocomplete: tab autocompletes commands and parameters
- Command history: arrow up/down navigates in command history

**Acceptance Criteria:**
- ✓ Interactive REPL with command parsing
- ✓ Tab completion for commands and state names
- ✓ Arrow key navigation for command history
- ✓ Help system for available commands
- ✓ Graceful error handling and user feedback

**Example Commands:**
```
/load <fsm-file.yaml>
/transition <state>
/current
/history
/help
/exit
```

---

## Phase 2: State History & Persistence

### Phase 2.1: State Change History
**Status:** Planned
**Dependencies:** Phase 1.1

**Features:**
- State history tracking
- Timestamped History: Add timestamps to state transition history
- History Persistence: Save console session history with timestamps
- Query historical transitions

**Acceptance Criteria:**
- ✓ Track all state transitions with timestamps
- ✓ Store transition metadata (user, reason, etc.)
- ✓ Query history by time range or state
- ✓ Export history to JSON/CSV
- ✓ History size limits and rotation

### Phase 2.2: REST API Core
**Status:** Planned
**Dependencies:** Phase 1.1, Phase 2.1

**Features:**
- REST Endpoints: HTTP API for FSM operations
- Instance Management: Multiple FSM instances with unique IDs
- In-Memory Storage: Fast state management
- JSON API: RESTful interface with JSON requests/responses

**API Endpoints:**
```
POST   /api/v1/fsm              - Create new FSM instance
GET    /api/v1/fsm/:id          - Get FSM current state
POST   /api/v1/fsm/:id/transition - Execute state transition
GET    /api/v1/fsm/:id/history  - Get transition history
DELETE /api/v1/fsm/:id          - Delete FSM instance
GET    /api/v1/definitions      - List available FSM definitions
```

**Acceptance Criteria:**
- ✓ All CRUD operations for FSM instances
- ✓ Proper HTTP status codes and error responses
- ✓ JSON request/response validation
- ✓ Concurrent request handling
- ✓ API documentation (Swagger/OpenAPI)
- ✓ Integration tests for all endpoints

### Phase 2.3: Database Persistence
**Status:** Planned
**Dependencies:** Phase 2.2

**Features:**
- State Persistence: Automatic state saving on transitions
- History Storage: Complete transition audit trail
- Configuration Management: Versioned FSM configurations
- PostgreSQL/SQLite: Flexible database backend

**Acceptance Criteria:**
- ✓ Persistent storage for FSM instances
- ✓ Automatic state synchronization
- ✓ Transaction support for atomic updates
- ✓ Database migration system
- ✓ Configurable backend (SQLite for dev, PostgreSQL for prod)
- ✓ Connection pooling and retry logic

### Phase 2.4: State Diagram Visualization
**Status:** In progress
**Dependencies:** Phase 2.2 (REST API), Phase 2.1 (History)

**Features:**
- **Multiple Visualization Formats:**
  - SVG - Scalable vector graphics (lightweight, embeddable, interactive)
  - PNG - Raster image for reports/documentation
  - DOT - GraphViz source format for advanced customization
  - Mermaid - Markdown-compatible diagrams for documentation
  - PlantUML - a highly versatile tool that facilitates the rapid and straightforward creation of a wide array of diagrams.
  - JSON - Structured graph data for custom rendering

- **Rich Visual Elements:**
  - States: rectangles, bold outline (initial), double outline (final)
  - All rectangles the same size, current state has the same outline as all other
  - Color-coding: Current state vs available states, distinct styling for error states
  - Transitions: Solid arrows (direct), dashed arrows (recovery), no bidirectional arrows
  - Wildcard transitions: represent as ANY STATE → FAILED. No arrows from real states
  - Layout algorithms: horizontal (default), vertical, no other types

- **Legend:**
**Status:** POSTPONED
  - Add legend describing diagram colors.
  - Keep legend at bottom of the diagram - centered. It's critical!
  - Legend layout is horizontal.
  - Legend uses rectangles in the legend. 
  - Legend rectangles are 50% size of diagram's ones. 
  - Add text label under each rectangle. Text is 50% of the one from diagrams. 
  - Text is wrapped to be exactly under legend rectangle.
 
- **Instance-Specific Visualizations:**
  - Highlight current state of specific asset instance
  - Show transition history path (breadcrumb trail)
  - Display available next states from current position
  - Highlight "hot paths" (most frequently used transitions)

- **Interactive Features (HTML Wrapper):**
  - Click states to see metadata
  - Click next state to change the current state graphically
  - Hover tooltips showing transition rules (POSTPONED)
  - Pan and zoom controls
  - Toggle views: definition-only vs instance-specific (POSTPONED)
  - Export buttons for different formats
  - Timeline slider for historical state visualization

**API Endpoints:**
```
GET /api/v1/visualize/definition/:name
  - Visualizes FSM definition (no instance data)
  - Query params: format=[svg|png|dot|mermaid|json], layout=[hierarchical|circular|force]

GET /api/v1/visualize/asset/:instanceID
  - Visualizes FSM with current instance state highlighted
  - Query params: format, layout, history=[true|false], available=[true|false]

GET /api/v1/visualize/asset/:instanceID/history
  - Animated visualization showing state progression over time
  - Query params: format, from=[timestamp], to=[timestamp]

GET /api/v1/visualize/asset-type/:assetType
  - Visualizes all instances of an asset type
  - Shows aggregate statistics and patterns

GET /docs/diagram/:instanceID
  - Interactive HTML page with embedded SVG
  - Real-time updates via WebSocket (optional)
```

**Acceptance Criteria:**

**Core Features:**
- ✓ API endpoint returns state visualization in multiple formats (SVG, PNG, DOT, Mermaid, JSON)
- ✓ States represented as circles with appropriate styling
- ✓ Transitions represented as arrows with appropriate styling (solid/dashed/bidirectional)
- ✓ Initial state marked with bold outline
- ✓ Final states marked with double circles
- ✓ Wildcard transitions visually distinct (bold red arrows)
- ✓ Current state highlighted for instance-specific visualizations
- ✓ Multiple layout algorithms supported (hierarchical, circular, force-directed)
- ✓ HTML wrapper page for interactive testing with URL parameters
- ✓ Available transitions highlighted when viewing specific instance

**Advanced Features:**
- ✓ Transition history overlay showing path taken by instance
- ✓ Export functionality in HTML wrapper (download SVG/PNG/DOT)
- ✓ Interactive tooltips showing state/transition metadata
- ✓ Pan and zoom controls for large diagrams
- ✓ Comparison view for multiple instances
- ✓ Aggregate visualization for asset types

**Technical Requirements:**
- ✓ Efficient rendering for FSMs with >50 states
- ✓ Caching layer for frequently requested diagrams
- ✓ Configurable color schemes and styling
- ✓ Mobile-responsive HTML viewer
- ✓ Proper HTTP headers for image formats (Content-Type, caching)
- ✓ Error handling for invalid instances/definitions

**Testing:**
- ✓ Unit tests for diagram generation logic (>80% coverage)
- ✓ Integration tests for API endpoints
- ✓ Visual regression tests for diagram rendering
- ✓ Performance tests for large FSMs (100+ states)

**Documentation:**
- ✓ OpenAPI/Swagger specs for visualization endpoints
- ✓ Example diagrams in documentation
- ✓ Embedding guide (how to use in external apps)

**Implementation Stack:**
- GraphViz for graph layout (`golang-graphviz/graphviz`)
- Native Go SVG generation
- Go `html/template` for HTML wrapper
- D3.js or Cytoscape.js for interactive features


---

## Phase 3: Security & Authentication

**Status:** Planned
**Dependencies:** Phase 2.2 (REST API must exist first)

**Features:**
- JWT Authentication: Secure API access
- Role-Based Access: Permission management
- Input Validation: Comprehensive request validation
- Rate Limiting: API protection

**Acceptance Criteria:**
- ✓ JWT token generation and validation
- ✓ Role-based permissions (admin, user, readonly)
- ✓ API key authentication support
- ✓ Request rate limiting per client
- ✓ Input sanitization and validation
- ✓ Security audit logging

**Security Roles:**
- `admin`: Full access (create, modify, delete FSMs)
- `user`: Create and manage own FSM instances
- `readonly`: View FSM states and history only

---

## Phase 4: Monitoring & Observability

**Status:** Planned
**Dependencies:** Phase 2.2, Phase 3

**Features:**
- Health Checks: System health monitoring
- Metrics: Prometheus metrics for transitions
- Structured Logging: Correlation IDs and tracing
- Webhooks: Event notifications

**Acceptance Criteria:**
- ✓ Health check endpoint (`/health`, `/readiness`)
- ✓ Prometheus metrics export (`/metrics`)
- ✓ Structured JSON logging with correlation IDs
- ✓ Distributed tracing support (OpenTelemetry)
- ✓ Webhook notifications for state changes
- ✓ Grafana dashboard templates

**Key Metrics:**
- FSM instance count
- State transition count by FSM type
- Transition latency percentiles
- Error rates and types
- API request rates and latencies

---

## Phase 5: Webhook improvements

  1. Webhook Retry Logic
    - Exponential backoff
    - Configurable retry count
    - Dead letter queue for failed webhooks
  2. Monitoring
    - Metrics for webhook success/failure rates
    - Alerts for high failure rates
    - Dashboard for webhook health
  3. Webhook Status Endpoint
    - Query pending/failed webhooks
    - Manual retry capability
    - Webhook audit log

    
---

## Phase 6: Advanced Features

**Status:** Future
**Dependencies:** Phase 2.3, Phase 3, Phase 4

**Features:**
- Bulk Operations: Batch state transitions
- Dynamic Configuration: Runtime FSM updates
- Event Hooks: Pre/post transition callbacks
- Client SDKs: Auto-generated libraries

**Acceptance Criteria:**
- ✓ Batch API for multiple transitions
- ✓ Hot-reload FSM definitions without restart
- ✓ Configurable hooks for transition events
- ✓ Generated client libraries (Go, Python, JavaScript)
- ✓ GraphQL API option
- ✓ WebSocket support for real-time updates

**Event Hook Types:**
- `beforeTransition`: Validate or prevent transition
- `afterTransition`: Trigger side effects
- `onStateEnter`: Execute when entering a state
- `onStateExit`: Execute when leaving a state

---

## Development Guidelines

### Testing Strategy
- **Unit Tests:** Each phase requires >80% code coverage
- **Integration Tests:** API endpoints and database operations
- **Load Tests:** Performance benchmarks for Phase 2.2+
- **Security Tests:** Penetration testing for Phase 3+

### Documentation Requirements
- API documentation (OpenAPI/Swagger)
- Architecture Decision Records (ADRs)
- Deployment guides
- Client library documentation

### Code Quality
- Go standard formatting (`gofmt`, `golint`)
- Static analysis (`go vet`, `staticcheck`)
- Dependency scanning
- Code review required for all phases

---

## Quick Start

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Run CLI
go run cmd/fsm/main.go --config examples/lifecycle.yaml

# Run API server (Phase 2.2+)
go run cmd/api/main.go --port 8080
```

## Project Structure

```
fsm_v2/
├── cmd/
│   ├── fsm/          # CLI application
│   └── api/          # REST API server
├── internal/
│   ├── fsm/          # Core FSM engine
│   ├── console/      # Interactive console
│   ├── api/          # REST API handlers
│   ├── storage/      # Persistence layer
│   └── auth/         # Authentication/authorization
├── pkg/
│   └── client/       # Client SDK
├── examples/         # Example FSM definitions
├── docs/             # Documentation
└── tests/            # Integration tests
```

---

## License

[Specify License]

## Contributing

[Contribution guidelines]
