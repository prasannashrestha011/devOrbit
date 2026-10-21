# DevOrbit — Docker Integration
## Phase 02: Container Logs, Interactive Terminal & Diagnostics

### Objective

Extend the existing Docker Compose integration with developer-focused debugging capabilities.

Developers should be able to inspect container logs, open interactive terminals, examine port mappings, and monitor container health without leaving DevOrbit.

**Implement Phase 02 only. Preserve all existing Phase 01 functionality.**

Before implementation, inspect the current Docker integration, terminal infrastructure, log handling, event streaming, API routes, and desktop components. Reuse established patterns wherever possible.

---

## 1. Container Log Viewer

Allow developers to view logs from individual Docker Compose services.

### Requirements

- Retrieve existing container logs.
- Support configurable tail limits, with a sensible default of 200 lines.
- Stream new logs in real time.
- Support stdout and stderr.
- Display timestamps when available.
- Support services with multiple container replicas.
- Clearly identify the source service/container.
- Allow pausing and resuming live log display.
- Allow clearing the displayed logs without deleting Docker logs.
- Handle container restarts, exits, and removals gracefully.
- Stop streaming when the viewer disconnects.

Use Docker Compose logging capabilities, such as:

```bash
docker compose logs --timestamps --no-color --tail 200 --follow <service>
```

Reuse DevOrbit's existing log viewer and streaming infrastructure where appropriate.

Do not duplicate Docker logs into PostgreSQL or introduce a second persistent log storage system.

Ensure bounded memory consumption, cancellation, and proper cleanup of streaming processes.

---

## 2. Interactive Container Terminal

Allow developers to open a real interactive terminal inside a running Compose container.

### Requirements

- Open a terminal for a selected running container.
- Reuse DevOrbit's existing terminal UI and PTY/session infrastructure where suitable.
- Support interactive stdin/stdout.
- Support terminal resize.
- Support command-line applications requiring TTY behavior.
- Support multiple terminal sessions.
- Distinguish sessions connected to local processes from Docker container sessions.
- Handle container exits and unexpected disconnections.
- Close sessions and release resources properly.
- Prevent terminal sessions from being attached to the wrong workspace or container.

Use Docker Compose exec or an equivalent existing-container execution mechanism.

For example:

```bash
docker compose exec <service> sh
```

Handle containers without a supported shell gracefully. Do not assume Bash exists.

For replicated services, allow selecting the specific existing container when necessary.

Opening a terminal must not create a new container, restart a service, or modify Compose configuration.

Use the existing authenticated terminal transport where possible. Do not expose an unrestricted command-execution endpoint.

---

## 3. Container Health Monitoring

Improve the current Docker service status presentation.

### Display

- Running
- Stopped
- Exited
- Starting
- Healthy
- Unhealthy
- No health check configured
- Restarting, when reported

Distinguish **container lifecycle state** from **Docker health-check state**.

A running container must not automatically be considered healthy.

### Requirements

- Read actual container state from Docker Engine.
- Read Docker-reported health status when available.
- Display container restart counts where available.
- Show recent health-check failures or diagnostics when provided by Docker.
- Refresh health data without blocking the UI.
- Update health information after lifecycle operations.
- Support manual refresh.

Do not invent health-check results for containers without a Docker health check.

Do not automatically restart unhealthy containers.

---

## 4. Container Port Inspector

Provide a clear view of published and internal container ports.

### Requirements

Display:

- Compose service name
- Container name
- Host address
- Host port
- Container port
- Protocol
- Whether the port is actually published

Example:

```text
postgres
Host:      127.0.0.1:5444
Container: 5432/tcp

backend
Host:      0.0.0.0:9999
Container: 9999/tcp
```

Requirements:

- Read actual published bindings from Docker.
- Distinguish declared/exposed ports from published host ports.
- Support multiple port mappings per container.
- Handle dynamically assigned host ports.
- Show useful diagnostics for unavailable mappings.
- Provide an Open URL action only when a usable HTTP/HTTPS URL can be determined reliably.
- Do not assume every TCP port serves HTTP.

Do not change container port mappings automatically.

---

## 5. Docker Service Detail Experience

Improve the existing Docker services UI without redesigning DevOrbit.

Each Docker service should provide access to:

- Service information
- Container status and health
- Image information
- Published ports
- Logs
- Interactive terminal
- Existing lifecycle controls

Prefer a compact service overview with focused details, contextual actions, and reusable components.

Avoid displaying every operation as a large button in every service row.

### UI Requirements

- Preserve the current DevOrbit visual theme.
- Reuse existing theme tokens, buttons, icons, badges, and typography.
- Keep compatibility with future theme settings.
- Maintain consistent action icons and sizing.
- Use clear running/stopped/healthy/unhealthy indicators.
- Present errors and loading states cleanly.
- Avoid unnecessary dashboard elements.
- Do not introduce the previously rejected workspace tab navigation.
- Preserve accessibility, keyboard operation, and responsive layout.

Logs and terminals should have enough space to be genuinely usable, rather than being squeezed into small dialog windows.

---

## 6. Backend Requirements

Extend the existing Docker integration rather than creating another Docker manager.

Reuse the selected workspace's Compose project context for every operation.

Requirements:

- Workspace-scoped API operations
- Existing authentication
- Validated service/container identifiers
- Structured Docker CLI arguments
- No shell interpolation of API input
- Context cancellation
- Controlled timeouts
- Proper subprocess/session cleanup
- Bounded log and event buffers
- Clear errors when Docker Engine is unavailable

Docker Engine remains responsible for container lifecycle.

DevOrbit's Process Supervisor remains responsible for local processes.

Both may share UI, logging presentation, terminal transport, and event utilities where appropriate.

Do not persist Docker runtime state as an alternative source of truth.

---

## 7. Error Handling

Handle the following situations:

- Docker Engine becomes unavailable.
- Container exits while logs are streaming.
- Container exits while a terminal is attached.
- Container is removed externally.
- Selected service has no container.
- Service has multiple replicas.
- Container has no supported shell.
- Terminal connection fails.
- Log stream disconnects.
- Docker health check is not configured.
- Container has no published ports.
- Workspace is deleted while a session is active.

These failures must not break local DevOrbit services or unrelated Docker projects.

---

## 8. Testing Requirements

Add focused tests for:

1. Container log retrieval, streaming, cancellation, and cleanup.
2. Interactive terminal input, output, resize, and session termination.
3. Workspace/container authorization boundaries.
4. Health-state interpretation.
5. Published versus internal port mapping.
6. Multiple Compose service replicas.
7. Docker Engine or container disappearance.
8. Existing Phase 01 lifecycle functionality.
9. Regression of local process terminals and logs.

Use mocked or injectable Docker command execution for unit tests where practical.

Perform real Docker integration tests only against isolated test resources.

---

## 9. Explicit Non-Goals

Do not implement in Phase 02:

- CPU/memory/network resource monitoring
- Advanced image management
- Image registry authentication
- Volume management
- Dockerfile editing
- Compose file generation
- Kubernetes support
- Remote Docker host management
- Automatic container recovery
- Automatic modification of Compose health checks
- Local service and Docker dependency orchestration
- Changes to `.devorbit/workspace.yaml`

These belong to later phases.

---

## 10. Acceptance Criteria

Phase 02 is complete when:

- Developers can open and stream logs from a Compose service.
- Developers can open an interactive terminal inside a running container.
- Terminal resize, input, output, and cleanup work correctly.
- DevOrbit displays actual Docker container health separately from process state.
- Developers can inspect real host/container port mappings.
- The Docker UI follows DevOrbit's existing design system.
- Existing Docker lifecycle actions remain functional.
- Unavailable or removed containers are handled gracefully.
- No unrelated workspaces or Docker projects are affected.
- Relevant tests and builds pass.

---

## 11. Implementation Instructions

1. Inspect and understand the existing Phase 01 implementation.
2. Identify reusable log, PTY, event, API, and UI components.
3. Plan the smallest changes needed for Phase 02.
4. Implement the features incrementally.
5. Add focused tests for each feature.
6. Verify that Phase 01 functionality still works.
7. Report completed work, changed files, tests, and known limitations.

**Do not unnecessarily refactor existing modules.**

**Do not proceed to Phase 03 automatically.**

The final result should make DevOrbit a practical Docker debugging environment while keeping Docker Compose as the authority for container definitions and Docker Engine as the authority for runtime state.