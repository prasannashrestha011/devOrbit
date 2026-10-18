# DevOrbit Backend V1 — Implementation Phase 04

## Objective

Implement the **Process Supervisor Core** for DevOrbit using Go.

This phase introduces real operating-system process execution and in-memory runtime management.

Follow the existing DevOrbit Master Project Context and preserve the architecture established in Phases 01–03.

**Implement Phase 04 only. Do not implement Phase 05 or later functionality.**

---

## 1. Pre-Implementation Inspection

Before making changes:

1. Inspect the existing repository structure.
2. Review the Service and Workspace models.
3. Review the existing service repository and application layer.
4. Review the REST API routing and authentication.
5. Review backend startup and graceful shutdown.
6. Review relevant tests and `docs/implementation-progress.md`.
7. Identify existing interfaces that can be reused.

Do not rewrite working components unnecessarily.

Ensure registered service configurations are available through the existing service application/repository layer.

Do not bypass existing application boundaries.

## 2. Process Supervisor Architecture

Implement a dedicated process supervisor responsible for managing executable processes.

Suggested package:

`internal/process`

The process supervisor must be independent of HTTP handlers and GORM.

It must manage runtime state in memory.

### Responsibilities

- Launch registered services.
- Track active processes.
- Maintain process lifecycle states.
- Prevent duplicate process execution.
- Observe process termination.
- Track process exit information.
- Support multiple concurrently running services.
- Release runtime resources correctly.
- Clean up owned processes during backend shutdown.

Use Go's `os/exec` package.

Do not introduce a distributed scheduler, message queue, or external process manager.

## 3. Runtime Process Model

Create a runtime model separate from the persisted Service configuration.

Suggested fields:

| Field | Type | Description |
|---|---|---|
| ServiceID | UUID | Registered service identifier |
| PID | nullable int | OS process identifier |
| State | enum/string | Current lifecycle state |
| StartedAt | nullable time.Time | Start time |
| ExitedAt | nullable time.Time | Termination time |
| ExitCode | nullable int | Process exit code |
| LastError | string | Relevant execution failure |
| UpdatedAt | time.Time | Last state change |

### Lifecycle States

Define these states:

- `stopped` — service has never started during this backend session.
- `starting` — process launch is in progress.
- `running` — OS process was started successfully.
- `exited` — process terminated with exit code 0.
- `failed` — process launch failed or process exited unsuccessfully.

Reserve `stopping` for Phase 05.

### Important Rules

- Runtime state must not be stored in PostgreSQL.
- Never use persisted PIDs as proof that a process is running.
- Runtime state belongs to the current supervisor instance.
- A service can have at most one active managed process.
- A service that has exited can be started again.
- A failed service can be retried.
- A process can exit immediately after starting; this must not corrupt lifecycle state.
- PID availability does not imply application readiness.

## 4. Process Registry

Implement a concurrency-safe process registry.

The registry must associate each ServiceID with its current runtime process information.

Requirements:

1. Use appropriate synchronization such as `sync.Mutex` or `sync.RWMutex`.
2. Prevent concurrent duplicate launches.
3. Support concurrent execution of different services.
4. Avoid holding global locks during slow operations.
5. Coordinate state transitions safely.
6. Return consistent snapshots of runtime status.
7. Avoid exposing mutable internal state.
8. Do not overwrite a newer execution's state with an older execution's exit callback.

### Concurrency Scenario

Two API requests attempt to start the same service simultaneously.

Expected behavior:

- Only one process may launch.
- The competing request receives a conflict response.
- No duplicate child processes are created.

Use an atomic state reservation or equivalent synchronization mechanism.

Do not rely solely on checking whether a PID exists.

## 5. Process Execution

Implement a Start operation using `exec.Command`.

Example registered service:

```json
{
  "name": "Go Backend",
  "command": "go",
  "args": ["run", "./cmd/api"],
  "working_directory": "./backend",
  "env": {
    "APP_ENV": "development",
    "PORT": "8080"
  }
}
```

### Execution Requirements

1. Load the registered service by ID.
2. Load its parent workspace when required.
3. Validate the current execution configuration.
4. Resolve the working directory.
5. Confirm the directory still exists.
6. Construct `exec.Cmd` with separate executable and argument values.
7. Set the working directory.
8. Inherit the backend's environment and apply service-specific overrides.
9. Launch the process.
10. Store the PID and runtime metadata.
11. Transition the state to `running`.
12. Observe termination asynchronously.

Do not execute the command through a shell automatically.

Do not combine command and arguments into a shell command string.

### Environment Handling

Build the child environment without mutating the backend's environment.

Service environment overrides must replace inherited variables predictably.

Account for platform-specific environment key behavior, especially Windows case-insensitive environment keys.

Do not expose environment values in errors or logs.

### Executable Validation

Handle:

- Missing executable
- Invalid executable path
- Permission denied
- Missing working directory
- Invalid environment configuration
- Process creation failure

Executable resolution must be performed consistently with the command's configured working directory and execution