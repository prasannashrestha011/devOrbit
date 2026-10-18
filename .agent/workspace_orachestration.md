# DevOrbit Backend V1 — Implementation Phase 06
## Workspace Orchestration

### Objective

Implement workspace-level process orchestration, allowing developers to start, stop, and monitor all services belonging to a development workspace.

Follow the existing DevOrbit Master Project Context.

Use the established stack:

- Go
- Chi
- GORM
- GORM Gen
- PostgreSQL 17
- GORM AutoMigrate

Preserve all working functionality from Phases 01–05.

**Implement Phase 06 only. Do not implement future phases.**

---

## 1. Pre-Implementation Inspection

Before modifying code:

1. Inspect the Workspace and Service modules.
2. Review the existing Process Supervisor.
3. Review individual Start, Stop, Restart, and Status operations.
4. Review process lifecycle states and concurrency protection.
5. Review the existing database repository patterns.
6. Review application startup and shutdown behavior.
7. Review API authentication and error handling.
8. Read `docs/implementation-progress.md`.

Reuse the existing process supervisor.

Do not create another process execution engine.

If Phase 05 has unresolved lifecycle correctness issues, identify them before building orchestration on top of it.

## 2. Workspace Orchestrator

Introduce a workspace orchestration component.

Suggested package:

`internal/orchestration`

Responsibilities:

- Start all enabled services in a workspace.
- Stop all running services in a workspace.
- Retrieve aggregate workspace runtime status.
- Coordinate multiple process operations.
- Handle individual service failures.
- Prevent conflicting workspace operations.
- Return structured execution results.

The orchestrator must delegate execution to the existing Process Supervisor.

It must not directly use `os/exec`.

Keep workspace orchestration independent of HTTP handlers and persistence implementation details.

## 3. Start All Services

Implement a workspace-level operation equivalent to:

`StartWorkspace(ctx, workspaceID)`

### Required Workflow

1. Validate the workspace identifier.
2. Confirm the workspace exists.
3. Retrieve all registered services belonging to the workspace.
4. Identify enabled services.
5. Sort services deterministically by creation time, then ID.
6. Check the supervisor's current state for each service.
7. Skip services already running.
8. Attempt to start eligible services.
9. Continue processing remaining services if an individual service fails.
10. Return a structured result for each service.

### Rules

- Start only services with `enabled = true`.
- Disabled services must not start automatically.
- Already-running services must not be duplicated.
- Services in conflicting lifecycle states must be reported appropriately.
- Different services may run concurrently after launching.
- Do not wait for a service process to exit before starting the next one.
- Do not interpret successful OS process launch as application readiness.
- Do not implement dependency-aware startup.
- Do not automatically roll back successfully started services when another service fails.

### Ordering

Use deterministic launch order.

Example:

1. Go Backend
2. React Frontend
3. Flutter Desktop
4. Background Worker

The order comes from persisted ordering, not hardcoded service names.

Sequential launch attempts are acceptable for V1. Once started, the processes run independently.

### Failure Handling

If four services are enabled and one fails to launch:

- The other three must still be attempted.
- Successfully launched services remain running.
- The failure must appear in the result.
- Do not automatically stop successful services.

Partial success is a valid outcome.

## 4. Stop All Services

Implement:

`StopWorkspace(ctx, workspaceID)`

### Required Workflow

1. Confirm the workspace exists.
2. Retrieve the workspace's registered services.
3. Determine which services are currently running.
4. Attempt to stop all active managed services.
5. Continue stopping other services even when one fails.
6. Wait for individual stop operations to complete according to their configured timeouts.
7. Return a structured result for each service.

### Rules

- Stop running services regardless of their `enabled` value.
- Skip already stopped or naturally exited services.
- Reuse the existing supervisor's graceful and forced termination implementation.
- Do not directly signal process IDs from the orchestrator.
- Do not terminate services belonging to other workspaces.
- Do not fail the entire operation immediately because one process cannot stop.
- Report failures individually.
- Never claim successful termination when an owned process is still running.

Use reverse deterministic service order when stopping.

This does not imply a dependency graph.

## 5. Workspace Runtime Status

Implement:

`GetWorkspaceStatus(ctx, workspaceID)`

Return a snapshot of runtime status for all registered services.

### Required Information

Workspace information:

- workspace_id
- workspace_name
- aggregate_state
- total_services
- enabled_services
- running_services
- stopped_services
- failed_services
- starting_services
- stopping_services
- exited_services

Service-level information:

- service_id
- name
- enabled
- pid
- state
- started_at
- exited_at
- exit_code
- last_error

Reuse the existing supervisor's runtime status representation.

Do not persist process state to PostgreSQL.

### Aggregate State

Derive workspace status from current service states.

Suggested aggregate states:

- `idle`: no service is starting, running, or stopping.
- `running`: all enabled services are running.
- `partial`: some services are running, but the enabled workspace environment is not fully running.
- `transitioning`: at least one service is starting or stopping.

Handle the case where no services are enabled explicitly. Do not report an empty workspace as running.

Failure and exit counts must remain visible independently from aggregate state.

Document the exact aggregation rules.

### Consistency

Produce a coherent best-effort snapshot of process states.

Concurrent process exits may happen immediately after the snapshot is created.

Do not hold supervisor locks while querying PostgreSQL.

## 6. Orchestration Operation Results

Design a consistent result object for Start All and Stop All.

Suggested structure:

```json
{
  "workspace_id": "workspace-uuid",
  "operation": "start",
  "total": 4,
  "succeeded": 3,
  "failed": 1,
  "skipped": 0,
  "results": [
    {
      "service_id": "service-uuid",
      "service_name": "Go Backend",
      "outcome": "started",
      "pid": 12345,
      "error": null
    }
  ]
}
```

The example is illustrative; reuse existing API conventions.

### Required Outcomes

For Start All:

- started
- already_running
- disabled
- failed
- conflict

For Stop All:

- stopped
- already_stopped
- failed
- conflict

Keep summary counters consistent with individual results.

Never expose sensitive environment variables or raw internal errors in API responses.

## 7. Concurrency Management

Workspace-level operations must be concurrency-safe.

### Scenarios to Handle

- Start All + Start All
- Stop All + Stop All
- Start All + Stop All
- Start All + individual Start
- Stop All + individual Stop
- Start All + individual Restart
- Workspace operation + backend shutdown
- Service naturally exits during Start All
- Service naturally exits during Stop All

### Requirements

1. Prevent conflicting workspace-level orchestration operations.
2. Allow independent workspaces to operate concurrently.
3. Reuse the existing supervisor's per-service concurrency guarantees.
4. Do not introduce a global execution lock for all workspaces.
5. Do not hold locks while performing slow database operations or waiting for process termination.
6. Ensure locks and operation reservations are released on errors and cancellation.
7. Do not allow duplicate process execution.
8. Handle individual lifecycle conflicts as explicit per-service outcomes.
9. Use predictable behavior when a workspace operation overlaps with individual service operations.
10. Prevent new workspace starts during backend shutdown.

Use a per