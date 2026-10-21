# DevOrbit — HTTP Traffic Inspector Implementation

## Objective

Implement a temporary, opt-in HTTP Traffic Inspector for DevOrbit.

Developers should be able to inspect HTTP requests and responses for a selected local or Docker-managed service without modifying the application's source code or interfering with existing process management.

The Traffic Inspector must use a temporary reverse proxy that exists only while inspection is enabled.

**Implement this feature incrementally, following the existing DevOrbit architecture.**

Before changing code, inspect existing workspace management, YAML service configuration, Docker integration, process supervision, terminal management, event broker, SSE handling, API security, and desktop UI.

Reuse existing components wherever appropriate.

---

## 1. Core Architecture

The intended request flow is:

```text
HTTP Client
    |
    v
Temporary DevOrbit Gateway
    |
    | Capture request metadata
    |
    v
Actual Backend Service
    |
    | HTTP response
    v
Temporary DevOrbit Gateway
    |
    | Capture response metadata
    |
    +----> Traffic Recorder
               |
               +----> In-Memory Traffic Buffer
               |
               +----> Existing Event Broker
                            |
                            | SSE
                            v
                     Traffic Inspector UI
```

The reverse proxy handles real HTTP communication.

SSE is used only for transmitting captured traffic records to the desktop, not for forwarding HTTP requests.

### Core Components

- **Inspection Manager:** Controls temporary inspection sessions.
- **Target Resolver:** Resolves the selected service and its HTTP endpoint.
- **Gateway:** Receives and forwards HTTP traffic.
- **Traffic Recorder:** Observes request and response metadata.
- **Traffic Buffer:** Retains recent records in bounded memory.
- **Event Publisher:** Integrates with DevOrbit's existing event broker.
- **Inspector API:** Provides session lifecycle, history, and live event access.
- **Desktop UI:** Displays live traffic and controls inspection sessions.

Do not create separate logging, event streaming, process supervision, or persistence systems.

---

## 2. Inspection Target Models

Support inspection of:

1. Local DevOrbit services defined in `workspace.yaml`.
2. Docker Compose services exposing an explicitly selected, reachable HTTP host port.

Suggested models:

```go
type TargetKind string

const (
    TargetLocal  TargetKind = "local"
    TargetDocker TargetKind = "docker"
)

type TargetRef struct {
    WorkspaceID uuid.UUID
    Kind        TargetKind
    ServiceKey  string
    PortKey     string
}

type ResolvedTarget struct {
    Ref      TargetRef
    Upstream *url.URL
}
```

These are conceptual model definitions. Adapt them to existing project conventions.

### Requirements

- Resolve targets using authoritative workspace YAML or Docker runtime configuration.
- Do not accept arbitrary upstream URLs from API clients.
- Verify the selected port represents an intended HTTP endpoint.
- Resolve service identity consistently with DevOrbit's existing identity model.
- Support services declaring multiple ports.
- Require explicit port selection when ambiguous.
- Reject nonexistent services, invalid ports, and inaccessible targets.
- Restrict the proxy to local development endpoints.
- Prevent an inspection session from being redirected to unintended upstreams.

The Traffic Inspector must not modify service configuration files.

---

## 3. Inspection Session Manager

Create a manager responsible for starting, stopping, and tracking inspection sessions.

Suggested runtime model:

```go
type SessionState string

const (
    SessionStarting SessionState = "starting"
    SessionActive   SessionState = "active"
    SessionStopping SessionState = "stopping"
    SessionStopped  SessionState = "stopped"
)

type SessionInfo struct {
    ID          uuid.UUID
    Target      TargetRef
    ProxyURL    string
    State       SessionState
    StartedAt   time.Time
}

type Session struct {
    Info     SessionInfo
    Target   ResolvedTarget
    Listener net.Listener
    Server   *http.Server
    Proxy    *httputil.ReverseProxy
    Cancel   context.CancelFunc
}

type Manager struct {
    mu       sync.RWMutex
    sessions map[uuid.UUID]*Session
    byTarget map[TargetRef]uuid.UUID
}
```

Extend this design only where needed for correct concurrency and lifecycle management.

### Requirements

- Support one active inspection session per workspace/service/port combination.
- Support multiple different inspection sessions concurrently.
- Allocate available loopback ports safely using `net.Listen("tcp", "127.0.0.1:0")`.
- Do not select a port by checking availability and then binding it later.
- Track session lifecycle accurately.
- Prevent duplicate sessions during concurrent Start requests.
- Release listeners and associated resources during Stop and Shutdown.
- Stop inspection without stopping the underlying application.
- Clean up appropriately when a workspace or service is removed.
- Avoid holding manager mutexes during blocking network operations.
- Handle repeated Start and Stop requests safely.

Session information should remain in memory.

Do not create PostgreSQL tables for inspection sessions.

---

## 4. Temporary HTTP Reverse Proxy

Use Go's `net/http` and `httputil.ReverseProxy`.

### Example

Actual backend:

`http://127.0.0.1:8080`

Temporary inspection gateway:

`http://127.0.0.1:18080`

When the developer enables inspection, DevOrbit creates a temporary local HTTP listener.

Requests sent to the gateway are forwarded to the selected backend.

### Requirements

- Bind to loopback only.
- Forward HTTP methods, paths, headers, and bodies correctly.
- Preserve response codes, headers, and response bodies.
- Preserve ordinary application behavior.
- Handle upstream connection errors.
- Support concurrent HTTP requests.
- Preserve streaming responses and HTTP flushing.
- Support WebSocket upgrades where feasible without interpreting their payloads.
- Avoid buffering complete request or response bodies.
- Prevent proxy loops.
- Apply appropriate connection and timeout handling.
- Shut down gracefully when inspection is disabled.

The proxy must not become an unrestricted forward proxy.

Do not install certificates, intercept system-wide HTTPS traffic, or modify operating-system proxy settings.

### Important UX Constraint

The client must explicitly send requests to the temporary gateway URL.

DevOrbit cannot capture requests that continue going directly to the original backend.

The UI must communicate this clearly.

---

## 5. Traffic Recorder

Capture metadata for both incoming requests and outgoing responses.

Suggested model:

```go
type TrafficOutcome string

const (
    OutcomeCompleted     TrafficOutcome = "completed"
    OutcomeUpstreamError TrafficOutcome = "upstream_error"
    OutcomeCanceled      TrafficOutcome = "canceled"
)

type TrafficRecord struct {
    ID          uuid.UUID
    SessionID   uuid.UUID
    Sequence    uint64

    StartedAt   time.Time
    Method      string
    Path        string

    StatusCode  int
    DurationMS  int64

    RequestBytes  *int64
    ResponseBytes *int64

    Outcome TrafficOutcome
    Error   string
}
```

### Capture Requirements

Capture:

- Unique request ID
- Inspection session ID
- Request method
- Sanitized request path
- Request timestamp
- HTTP response status
- Total request duration
- Request/response sizes when reliably measurable
- Upstream connection failures
- Request cancellation or termination status

### Privacy Requirements

Metadata-only capture by default.

Do not capture or persist:

- Authorization headers
- Cookies
- Authentication tokens
- Raw query parameters
- Request bodies
- Response bodies
- Other sensitive header values

Bound stored strings and sanitize potentially sensitive paths and error messages.

Never include authentication credentials in SSE payloads or logs.

### Performance Requirements

- Recording must not block HTTP forwarding unnecessarily.
- Use bounded memory.
- Avoid expensive synchronous processing in the request path.
- Never wait for a slow SSE subscriber before completing a client response.
- Handle concurrent requests without mixing their records.

Record duration consistently, including time spent streaming the response.

---

## 6. In-Memory Traffic Buffer

Implement a thread-safe, bounded traffic history store.

Suggested interface:

```go
type TrafficStore interface {
    Append(record TrafficRecord) TrafficRecord

    Recent(
        sessionID uuid.UUID,
        afterSequence uint64,
        limit int,
    ) []TrafficRecord

    Clear(sessionID uuid.UUID)
}
```

### Requirements

- Use a bounded ring buffer or equivalent structure.
- Default to approximately 500 recent records per session.
- Assign monotonically increasing sequence numbers within each session.
- Maintain separate histories for separate sessions.
- Support retrieving records after a sequence number.
- Support clearing displayed session history.
- Evict old entries when capacity is reached.
- Ensure safe concurrent access.
- Prevent unbounded memory growth.
- Clean up expired or ended session histories according to a defined policy.

Do not store traffic in PostgreSQL or on disk.

---

## 7. SSE and Event Integration

Reuse DevOrbit's existing event broker and SSE infrastructure.

For each completed traffic record:

```text
Traffic Recorder
       |
       +----> Append to Traffic Buffer
       |
       +----> Publish traffic.completed
                       |
                       v
                 Existing SSE Broker
                       |
                       v
                Traffic Inspector UI
```

### Requirements

- Define a structured traffic event.
- Include session ID and record sequence.
- Scope subscriptions to the appropriate workspace/session.
- Publish through the existing event infrastructure.
- Avoid leaking traffic events between workspaces.
- Handle disconnected subscribers.
- Never block proxy requests because of event delivery.
- Support loading recent history followed by live events.
- Prevent missing or duplicating records during UI initialization using sequence-based reconciliation.
- Handle event-buffer gaps and reconnects explicitly.

Do not create a second SSE server if the existing infrastructure can support the feature safely.

---

## 8. Backend API

Follow DevOrbit's existing authenticated route conventions.

Expose operations equivalent to:

| Operation | Purpose |
|---|---|
| Start inspection | Create a temporary gateway session |
| Get inspection | Return active session information |
| Stop inspection | Shut down the gateway |
| Get traffic | Retrieve buffered traffic records |
| Stream events | Receive live traffic events |
| Clear traffic | Clear retained session records |

The exact endpoints should follow existing routing and API naming conventions.

### API Requirements

- All operations must be authenticated.
- Validate workspace and service identities.
- Enforce session ownership/scoping.
- Reject unauthorized cross-workspace access.
- Never accept arbitrary shell commands or upstream URLs.
- Return clear session states and actionable errors.
- Handle duplicate Start, repeated Stop, missing sessions, and timeouts.
- Integrate with the desktop API client and route allowlist if applicable.

---

## 9. Desktop Traffic Inspector

Integrate the feature into the existing DevOrbit workspace/service experience.

### Enable Inspection

For an HTTP-capable service, provide an **Inspect HTTP Traffic** action.

When enabled, display:

- Session status
- Selected service
- Actual backend target
- Temporary proxy URL
- Copy URL action
- Disable Inspection action

Clearly indicate that the client must send requests through the temporary URL.

### Live Traffic Viewer

Display:

- Method
- Request path
- HTTP status
- Duration
- Timestamp
- Error indicator

Provide:

- Live updates
- Request selection
- Request detail view
- Filtering by method/status
- Search
- Clear captured history
- Pause/resume live display

Pausing the UI should not stop the proxy itself.

### UI Design

- Preserve the existing DevOrbit theme.
- Reuse existing icons, buttons, badges, and layout patterns.
- Keep compatibility with future theme settings.
- Avoid a full workspace redesign.
- Use the existing desktop navigation approach.
- Make captured traffic readable and easy to scan.
- Display empty, loading, disconnected, and error states clearly.
- Use a proper expandable workspace panel or dedicated inspector view rather than a cramped modal.

---

## 10. Session Shutdown Behavior

When the developer disables inspection:

1. Mark the session as stopping.
2. Stop accepting new requests.
3. Allow appropriate graceful shutdown.
4. Cancel remaining session work after a bounded timeout.
5. Close the HTTP listener.
6. Release resources.
7. Publish the final session state.
8. Preserve or clear buffered traffic according to the defined retention policy.

The actual backend service must remain unaffected.

### Important

Clients still using the temporary proxy URL will lose access after inspection stops.

Warn the user before stopping the proxy.

Never automatically modify the frontend's environment variables or API endpoint configuration.

---

## 11. Error Handling

Handle at least:

- Service does not exist.
- No HTTP port is available.
- Multiple HTTP ports require selection.
- Backend service is stopped.
- Docker container is unavailable.
- Upstream connection is refused.
- Proxy port allocation fails.
- Session already exists.
- Session is stopped externally.
- Client disconnects.
- WebSocket or streaming connection terminates.
- Event subscriber disconnects.
- Traffic buffer reaches capacity.
- Workspace is deleted.
- DevOrbit shuts down with active sessions.

Failures must not affect unrelated local services, Docker containers, terminals, or workspaces.

---

## 12. Testing Requirements

Add focused tests for:

### Manager

- Start and Stop lifecycle
- Duplicate sessions
- Concurrent operations
- Port allocation
- Cleanup
- Graceful shutdown

### Reverse Proxy

- GET and POST forwarding
- Request bodies
- Response statuses
- Response headers
- Upstream errors
- Concurrent requests
- Streaming behavior
- WebSocket forwarding where supported

### Recorder

- Request metadata
- Response metadata
- Duration
- Status codes
- Error outcomes
- Sensitive information redaction

### Buffer

- Append
- Sequence ordering
- Capacity eviction
- Concurrent access
- History retrieval
- Clear
- Session isolation

### SSE

- Event publication
- Subscription scoping
- Reconnection
- History/live synchronization
- Slow subscribers
- Event gaps

### Regression

- Existing process supervisor
- Docker lifecycle operations
- Existing terminals
- Existing log management
- Workspace YAML configuration
- Desktop route security

Use `httptest` and local in-process test servers for proxy tests.

Ordinary tests must not require Docker Engine or external network access.

Run relevant Go tests, including race detection where practical, and desktop build/type checks.

---

## 13. Explicit Non-Goals

Do not implement:

- Persistent Development Gateway
- Custom `.localhost` hostname generation
- System-wide HTTP interception
- HTTPS man-in-the-middle inspection
- Certificate installation
- Browser proxy configuration automation
- Automatic frontend URL rewriting
- Persistent traffic database
- Full request/response body capture
- Request modification or replay
- Distributed tracing
- OpenTelemetry integration
- Production traffic interception
- Remote-host proxying
- Changes to the workspace YAML schema

These are outside this feature's initial scope.

---

## 14. Implementation Sequence

Implement incrementally:

**Step 1 — Session Manager**

Implement inspection session identity, lifecycle, concurrency, and listener management.

**Step 2 — HTTP Gateway**

Implement a temporary reverse proxy forwarding requests to a validated target.

**Step 3 — Traffic Recorder**

Capture sanitized request/response metadata, status codes, durations, and errors.

**Step 4 — Traffic Buffer**

Implement bounded in-memory history and sequence-based retrieval.

**Step 5 — SSE Integration**

Connect captured events to the existing DevOrbit event broker.

**Step 6 — Backend API**

Expose authenticated session and traffic operations.

**Step 7 — Desktop Integration**

Add inspector enable/disable controls, temporary URL display, and live traffic viewer.

**Step 8 — Validation and Regression**

Test concurrency, cancellation, streaming, security, and existing functionality.

Follow the smallest maintainable implementation that fits the repository.

Do not introduce abstractions without demonstrated need.

---

## 15. Acceptance Criteria

Implementation is complete when:

1. Developers can enable temporary HTTP inspection for a selected service.
2. DevOrbit creates a loopback-only reverse proxy on an available port.
3. Requests sent through that proxy reach the real backend.
4. Responses reach the original client correctly.
5. The recorder captures safe HTTP request and response metadata.
6. Captured records are retained in bounded memory.
7. New traffic records appear live through the existing SSE infrastructure.
8. Recent records remain available when the UI is reopened during an active session.
9. Developers can disable inspection and release the proxy listener.
10. The underlying application continues running independently.
11. Multiple workspaces and inspection sessions remain isolated.
12. Existing Docker, Supervisor, logging, and terminal functionality remain intact.
13. All relevant tests and builds pass.

---

## 16. Final Instructions

Inspect the current repository first.

Implement only the Temporary HTTP Traffic Inspector described here.

Preserve existing architecture and code conventions.

After implementation, provide:

- Architecture summary
- Models and interfaces introduced
- Main files created or modified
- Backend API endpoints
- Desktop functionality
- Tests executed and results
- Known limitations
- Any recommended future improvements

**Do not implement persistent gateway routing, custom domains, or advanced traffic interception.**

The goal is a lightweight, safe, opt-in HTTP debugging tool that integrates naturally with DevOrbit.

file structure
        ├── model.go        # Session, Target, TrafficRecord
        ├── manager.go      # Start/Stop inspection sessions
        ├── resolver.go     # Resolve local/Docker services
        ├── gateway.go      # HTTP reverse proxy
        ├── recorder.go     # Capture requests and responses
        ├── buffer.go       # In-memory traffic history
        ├── events.go       # SSE event publishing
        ├── service.go      # Application business logic
        ├── routes.go       # HTTP API endpoints