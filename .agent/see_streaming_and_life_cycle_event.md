# DevOrbit Backend V1 — Implementation Phase 08
## Real-Time SSE Streaming & Lifecycle Events

### Objective

Implement real-time event streaming for DevOrbit using Server-Sent Events (SSE).

The backend must broadcast process lifecycle events and stdout/stderr logs to authenticated clients without polling.

Follow the existing DevOrbit Master Project Context.

**Tech Stack**
- Go + Chi
- GORM + GORM Gen
- PostgreSQL 17
- GORM AutoMigrate
- Electron + TypeScript (future desktop frontend)

Preserve all working functionality from Phases 01–07.

**Implement Phase 08 only. Do not implement Phase 09 or the Electron frontend.**

---

## 1. Inspect Existing Implementation

Before writing code:

1. Inspect the Process Supervisor.
2. Review Start, Stop, Restart, and process exit detection.
3. Review the Log Manager implemented in Phase 07.
4. Review log ingestion, persistence, and retrieval.
5. Review workspace orchestration.
6. Review Chi routing and authentication middleware.
7. Review HTTP server timeout configuration.
8. Review application initialization and shutdown.
9. Read `docs/implementation-progress.md`.

Identify existing abstractions suitable for event publishing.

Do not recreate the Process Supervisor or Log Manager.

Avoid unnecessary refactoring.

## 2. Event System Architecture

Introduce a lightweight in-memory event broker.

Suggested package:

`internal/events`

Responsibilities:

- Publish process lifecycle events.
- Publish log-created events.
- Register and unregister subscribers.
- Filter events by workspace and service.
- Broadcast events to multiple subscribers.
- Handle disconnected or slow consumers.
- Shut down cleanly.

### Architecture

Process Supervisor → Event Broker

Log Manager → Event Broker

Event Broker → SSE HTTP Handler → Electron Client

The Event Broker must not:

- Execute processes.
- Query PostgreSQL for every event.
- Write application logs to disk.
- Manage process lifecycle.
- Block process execution when clients are slow.

Use Go channels and synchronization primitives.

Do not introduce Redis, Kafka, NATS, RabbitMQ, or a distributed messaging system.

## 3. Event Structure

Define a consistent event envelope.

Required fields:

| Field | Description |
|---|---|
| ID | Unique event identifier |
| Type | Event type |
| Timestamp | UTC event timestamp |
| WorkspaceID | Associated workspace |
| ServiceID | Associated service, if applicable |
| ExecutionID | Associated execution, if applicable |
| Data | Event-specific payload |

### Event Types

Implement:

- `process.started`
- `process.stopping`
- `process.stopped`
- `process.exited`
- `process.failed`
- `process.restarted`
- `log.created`

Use constants or a typed event representation rather than scattering event-name strings throughout the codebase.

Optional internal lifecycle events may be added only if necessary.

### Example Process Event

```json
{
  "id": "event-identifier",
  "type": "process.started",
  "timestamp": "2026-10-18T14:00:00Z",
  "workspace_id": "workspace-uuid",
  "service_id": "service-uuid",
  "execution_id": "execution-uuid",
  "data": {
    "pid": 12345,
    "state": "running"
  }
}
```

### Example Log Event

```json
{
  "id": "event-identifier",
  "type": "log.created",
  "timestamp": "2026-10-18T14:00:01Z",
  "workspace_id": "workspace-uuid",
  "service_id": "service-uuid",
  "execution_id": "execution-uuid",
  "data": {
    "stream": "stdout",
    "sequence": 42,
    "message": "HTTP server started"
  }
}
```

Reuse Phase 07 log entry metadata.

Do not create a second log schema with incompatible field meanings.

Ensure events never automatically expose authentication tokens, process environment values, or internal credentials.

## 4. Event Broker

Implement an in-memory publish/subscribe broker.

Suggested responsibilities:

- `Publish(event)`
- `Subscribe(filter)`
- `Unsubscribe(subscription)`
- `Close()`

These are conceptual operations. Adapt interfaces to existing project conventions.

### Requirements

1