# DevOrbit Backend V1 — Implementation Phase 05
## Process Lifecycle Management

### Objective

Extend the existing Process Supervisor to support reliable service stopping, restarting, process-tree termination, and concurrency-safe lifecycle management.

Follow the DevOrbit Master Project Context.

Preserve all working functionality from Phases 01–04.

**Implement Phase 05 only. Do not implement later phases.**

---

## 1. Inspect Existing Implementation

Before modifying code:

1. Review the Process Supervisor from Phase 04.
2. Understand how processes are started and registered.
3. Review runtime state transitions and PID tracking.
4. Review how `exec.Cmd`, `Wait()`, and process exit detection are handled.
5. Review backend shutdown behavior.
6. Review existing HTTP endpoints and authentication.
7. Read `docs/implementation-progress.md`.

Identify lifecycle limitations and make minimal changes to the existing implementation.

Do not rewrite the Process Supervisor unless necessary for correctness.

If Phase 04 is incomplete, report the blockers rather than silently implementing missing earlier phases.

## 2. Process Stop Operation

Implement:

`Supervisor.Stop(ctx, serviceID)`

Responsibilities:

- Locate the currently managed process.
- Transition the process into `stopping`.
- Request graceful termination.
- Wait for process termination.
- Escalate to forced termination after a configurable timeout.
- Ensure the managed process and owned children are cleaned up.
- Record the final state and termination information.
- Release process-specific resources.

### Rules

- Only terminate processes owned by DevOrbit.
- Do not rely on a PID alone to establish process ownership.
- Never terminate unrelated operating-system processes.
- Do not call `Wait()` from multiple goroutines.
- Coordinate stopping with the existing exit observer.
- Do not hold the global supervisor mutex while waiting for process termination.
- Prevent new starts while a service is stopping.
- Make repeated Stop calls safe.
- Treat an already terminated service as an idempotent no-op.
- Return meaningful errors when termination fails.

Stopping a service must not affect other independently managed services.

## 3. Graceful and Forced Termination

Use a configurable shutdown timeout.

Suggested default:

`10 seconds`

Termination sequence:

1. Mark the service as stopping.
2. Send an appropriate graceful termination signal.
3. Wait for the process to exit.
4. If the deadline expires, force termination of the managed process tree.
5. Reap the process and release resources.
6. Update runtime state.

### Unix (macOS/Linux)

Implement appropriate process-group management.

- Create isolated process groups when launching managed processes.
- Use process-group signaling where appropriate.
- Prefer a graceful interrupt suitable for local development applications.
- Escalate to forced termination after timeout.
- Avoid signaling unrelated process groups.
- Handle processes that exit before signaling.
- Account for child processes created by `go run`, `npm`, and similar commands.

### Windows

Implement platform-specific termination handling.

- Use Windows-appropriate process management.
- Prefer Job Objects for tracking and terminating managed process trees.
- Where supported, use a graceful console-control mechanism before forced termination.
- Do not assume Unix signals behave identically on Windows.
- Avoid blindly relying on `Process.Kill()` when a child process tree exists.
- Document limitations where graceful shutdown is unavailable.

Use build-tagged, OS-specific files where necessary.

If the existing launcher must be adjusted to establish process-group or Job Object ownership at process creation time, make that focused change.

Do not implement unrelated platform features.

## 4. Process Restart Operation

Implement:

`Supervisor.Restart(ctx, serviceID)`

Expected behavior:

- If running, stop the existing process and its managed descendants before launching a replacement.
- If exited, failed, or stopped, launch a new process.
- Prevent overlapping old and new executions.
- Assign and record the new PID.
- Update runtime timestamps and state.
- Propagate meaningful errors when either stopping or starting fails.

Restart must use the latest persisted service configuration when starting the replacement process.

Do not introduce automatic restart policies or background restart loops.

The existing `auto_restart` configuration field remains inactive in V1 Phase 05.

## 5. Lifecycle State Management

Extend the runtime state machine.

Required states:

- `stopped`
- `starting`
- `running`
- `stopping`
- `exited`
- `failed`

### State Rules

- `stopped` → `starting` → `running`
- `running` → `stopping` → `stopped`
- `running` → `exited` on successful natural exit
- `running` → `failed` on unexpected unsuccessful exit
- `exited` / `failed` / `stopped` → `starting` when launched again
- `starting` → `failed` when process creation fails

Restart must safely transition through termination and a new launch.

Record exit codes when available.

Intentional termination must not be misclassified as an unexpected process crash.

Distinguish process launch failure from process exit failure.

Prevent stale exit callbacks from overwriting the status of a newer process instance.

## 6. Concurrency and Race Condition Handling

Handle simultaneous lifecycle requests safely.

Required scenarios:

- Start + Start
- Start + Stop
- Start + Restart
- Stop + Stop
- Stop + Restart
- Restart + Restart
- Natural exit + Stop
- Natural exit + Restart
- Backend shutdown + Start
- Backend shutdown + Restart

### Requirements

1. Enforce a single active lifecycle operation per service.
2. Prevent duplicate processes.
3. Avoid holding global locks during blocking OS operations.
4. Permit unrelated services to operate concurrently.
5. Prevent deadlocks and goroutine leaks.
6. Ensure lifecycle state updates are atomic where required.
7. Use generation IDs or equivalent mechanisms to prevent stale process events from changing newer runtime state.
8. Return predictable results for conflicting operations.

Prefer simple synchronization mechanisms over unnecessary concurrency frameworks.

Concurrent lifecycle calls must never lead to uncontrolled duplicate executions.

## 7. REST API

Extend the existing authenticated API.

### Stop Service

`POST /api/v1/services/{id}/stop`

Behavior:

- Validate service UUID.
- Confirm the registered service exists.
- Request service termination.
- Return updated runtime state.
- Return HTTP 200 after termination completes.
- Return HTTP 404 for unknown services.
- Handle conflicting lifecycle operations appropriately.
- Return an appropriate error if termination cannot be completed safely.

### Restart Service

`POST /api/v1/services/{id}/restart`

Behavior:

- Validate service UUID.
- Confirm registered service exists.
- Stop the existing process if necessary.
- Start the service using its current configuration.
- Return the new runtime status.
- Return HTTP 200 when the new OS process launches successfully.
- Return HTTP 404 for unknown services.
- Return HTTP 409 for conflicting operations.

### Status Endpoint

Preserve:

`GET /api/v1/services/{id}/status`

Extend the response to support the `stopping` state and updated termination metadata.

Follow existing authentication, request validation, response envelopes, and error handling conventions.

Do not accept arbitrary executable commands through these lifecycle endpoints.

## 8. Backend Shutdown Hardening

Improve the shutdown implementation from Phase 04.

Requirements:

- Reject new Start and Restart operations once shutdown begins.
- Stop every active managed service.
- Allow independent services to terminate concurrently.
- Apply bounded graceful and forced termination.
- Wait for process reaping and lifecycle cleanup.
- Avoid abandoning child processes.
- Release platform-specific handles and resources.
- Close the supervisor only after cleanup is complete or a clearly reported failure occurs.

Ensure backend shutdown cannot deadlock with concurrent process operations.

Avoid leaving services incorrectly marked as running after termination.

## 9. Testing

Write comprehensive unit and integration tests.

### Stop Tests

- Stop running process
- Stop already stopped process
- Stop naturally exited process
- Stop nonexistent service
- Graceful termination
- Forced termination after timeout
- Verify no managed descendant remains where supported

### Restart Tests

- Restart running service
- Restart stopped service
- Restart failed service
- Restart naturally exited service
- Ensure new PID belongs to the replacement execution
- Ensure previous process has terminated before replacement
- Handle failure during restart

### Concurrency Tests

- Concurrent Stop requests
- Concurrent Restart requests
- Start and Stop race
- Start and Restart race
- Stop during natural exit
- Shutdown during active lifecycle operations
- No duplicate managed processes
- No deadlocks or goroutine leaks

### Cross-Platform Tests

Test process-tree termination using controlled helper processes that create child processes.

Avoid fragile assumptions about installed shells or external commands.

Separate OS-specific test cases using Go build tags where necessary.

Only claim runtime verification on operating systems actually tested.

### API Tests

- Successful stop
- Successful restart
- Invalid UUID
- Nonexistent service
- Unauthorized requests
- Lifecycle conflicts
- Correct runtime status after termination

### Validation

Run:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/devorbit
```

Verify applicable cross-platform builds.

Do not claim tests passed unless executed successfully.

## 10. Documentation

Update:

- `README.md`
- `docs/implementation-progress.md`
- Existing architecture/API documentation where applicable

Document:

- Stop and Restart endpoints
- Lifecycle state transitions
- Graceful and forced termination behavior
- Process-tree ownership
- Concurrency rules
- Backend shutdown behavior
- macOS, Linux, and Windows differences
- Known platform limitations

## 11. Out of Scope

Do not implement:

- Automatic restart or