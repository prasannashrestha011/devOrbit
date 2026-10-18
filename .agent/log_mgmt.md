# DevOrbit Backend V1 — Implementation Phase 07
## Centralized Log Management

### Objective

Implement a centralized logging system that captures, stores, manages, and retrieves stdout and stderr from processes managed by DevOrbit.

Follow the existing DevOrbit Master Project Context.

**Technology Stack:**
- Go
- Chi
- GORM + GORM Gen
- PostgreSQL 17
- GORM AutoMigrate
- Electron + TypeScript (future frontend)

Preserve functionality from Phases 01–06.

**Implement Phase 07 only. Do not implement SSE streaming or future phases.**

---

## 1. Pre-Implementation Inspection

Before making changes:

1. Inspect the existing Process Supervisor.
2. Review how commands are executed using `os/exec`.
3. Inspect the process registry and execution lifecycle.
4. Review Start, Stop, Restart, and workspace orchestration.
5. Identify how stdout and stderr are currently handled.
6. Review application initialization and shutdown.
7. Review existing REST API and authentication conventions.
8. Read `docs/implementation-progress.md`.

Identify the smallest integration changes necessary.

Do not rewrite working process management code.

The Log Manager must integrate with the existing Process Supervisor rather than creating a separate execution engine.

## 2. Log Manager Architecture

Introduce a dedicated component.

Suggested package:

`internal/logs`

Responsibilities:

- Capture managed-process stdout.
- Capture managed-process stderr.
- Attach log metadata.
- Maintain bounded recent logs in memory.
- Persist logs to local disk.
- Rotate log files.
- Enforce log retention.
- Retrieve historical logs.
- Filter logs by service and workspace.
- Safely handle concurrent writes.
- Release resources during shutdown.

The Log Manager must be independent of HTTP handlers and GORM.

### Persistence Decision

Store process logs in the local filesystem.

PostgreSQL remains responsible for Workspace and Service configuration.

Do not introduce a database table for every application log entry.

Do not introduce Elasticsearch, Loki, Kafka, Redis, or another external logging service.

Use the Go standard library wherever practical.

## 3. Log Entry Structure

Create a structured log entry containing:

| Field | Description |
|---|---|
| ID | Stable identifier or cursor-compatible position |
| WorkspaceID | Parent workspace UUID |
| ServiceID | Registered service UUID |
| ExecutionID | Unique identifier for the process execution |
| Stream | `stdout` or `stderr` |
| Timestamp | Time the output was captured |
| Message | Captured output |
| Sequence | Sequence number within the execution |

Example:

```json
{
  "id": "log-identifier",
  "workspace_id": "workspace-uuid",
  "service_id": "service-uuid",
  "execution_id": "execution-uuid",
  "stream": "stdout",
  "timestamp": "2026-10-18T14:00:00Z",
  "sequence": 42,
  "message": "HTTP server listening on :8080"
}
```

### Requirements

- Generate a new ExecutionID every time a service starts.
- Preserve the distinction between different executions of the same service.
- Support logs from concurrently running services.
- Use UTC timestamps.
- Preserve message content safely.
- Do not assume stdout and stderr have a guaranteed global ordering.
- Preserve ordering according to the Log Manager's ingestion sequence where possible.

Do not mix the identity of an old execution with a restarted process.

## 4. Process Output Capture

Integrate with the existing Process Supervisor.

When a managed process starts:

1. Create an execution identifier.
2. Initialize log capture.
3. Attach stdout and stderr destinations before starting the process.
4. Launch the process using the existing supervisor.
5. Capture its output while it runs.
6. Continue capturing until output streams close.
7. Flush remaining buffered output after process termination.
8. Release execution-specific logging resources.

### Requirements

- Capture stdout and stderr separately.
- Capture output continuously.
- Support multiple concurrent processes.
- Support short-lived commands.
- Handle processes producing no output.
- Handle large output volumes.
- Handle output without newline characters.
- Handle partial lines.
- Handle carriage returns and ANSI control sequences safely.
- Do not silently truncate long messages without recording that truncation occurred.
- Avoid goroutine leaks and deadlocks.

### Important Process Lifecycle Requirement

Preserve the existing rule that every successfully started process is reaped exactly once.

If using `StdoutPipe` and `StderrPipe`, coordinate stream readers carefully with `Wait()`.

Alternatively, use concurrency-safe writers when appropriate.

Do not introduce a race where process termination closes output streams before buffered data has been collected.

## 5. Bounded In-Memory Log Buffer

Implement recent-log buffering for each service.

Suggested defaults:

- Maximum 500 recent log entries per service.
- Maximum 2 MiB buffered log data per service.
- Maximum 16 KiB per stored log message chunk.

Make these limits configurable where reasonable.

### Requirements

- Use a ring buffer or similarly bounded data structure.
- Enforce both entry-count and byte-size limits.
- Remove oldest entries when capacity is exceeded.
- Preserve ordering of retained entries.
- Support concurrent readers and writers.
- Return immutable snapshots.
- Never let process output consume unlimited memory.

The memory buffer is a recent-history cache, not the authoritative persistent log store.

If a process produces more output than the system can handle, use an explicit bounded overflow policy.

Avoid blocking child-process output indefinitely because disk writes or API consumers are slow.

Record dropped-entry counts or equivalent diagnostics when overload occurs.

## 6. Local Log Persistence

Persist process logs to local disk.

Suggested structure:

```text
DevOrbit/
  logs/
    services/
      {service-id}/
        {execution-id}/
          output-0001.jsonl
          output-0002.jsonl
```

Use platform-appropriate application data directories.

The exact file layout may be adjusted based on existing project conventions.

### Requirements

- Use JSON Lines (JSONL) or another documented append-friendly structured format.
- Ensure each persisted entry includes sufficient metadata.
- Support concurrent processes.
- Do not overwrite logs from previous executions.
- Create directories with restrictive permissions where supported.
- Close