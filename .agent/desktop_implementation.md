# DevOrbit desktop implementation prompt

Implement the first working DevOrbit desktop client in `desktop/` using React and TypeScript for the UI and Tauri for the desktop shell. The existing Go backend is the source of truth. Older project notes mention Electron; the current product decision is Tauri.

## Visual target

- Use `docs/design/workspace-overview-concept.png` for the workspace overview and quick controls.
- Use `docs/design/service-logs-concept.png` for the service detail and log view.
- Combine them into one consistent design system: dark graphite surfaces, blue primary actions, clear green/amber/red lifecycle states, readable log typography, and restrained borders.
- Build a normal main window plus a **separate compact Tauri overlay window** for quick service controls. The overlay is not a modal inside the main window. Make it keyboard accessible and dismissible with Escape.
- Match the useful layout and hierarchy of the concepts, not their invented text or unsupported features. Do not add per-service health checks, discovered ports, templates, cloud features, or terminal execution.

## Product scope

1. Workspace list, create/edit/delete, overview, aggregate status, Start All, and Stop All. Show each service's outcome when a workspace action only partly succeeds.
2. Service configuration list and create/edit/delete for a workspace; service start, stop, restart, and status. A background worker is an ordinary service configuration. Do not imply that editing a running service restarts it. Stop a running service before deleting its configuration; deletion alone does not stop its process.
3. Service detail with persisted log history, pagination, stdout/stderr distinction, and live updates. Use the events stream for lifecycle and new-log notifications; refetch state and logs after reconnect because events have no replay.
4. Quick overlay showing the selected workspace, service states, search, and direct start/stop/restart controls. Actions and status must stay synchronized with the main window. Keep the overlay useful at roughly 480×320 without reproducing the whole app.
5. Loading, empty, transitioning, failed, backend-unavailable, and authentication-error states. Confirm destructive deletes. A `running` process has launched, but is not necessarily healthy. Do not present `auto_restart` as active; the backend stores it but does not implement it.

## Integration and security

- Read `README.md`, `internal/api/api.go`, `internal/config/config.go`, and the module routes before coding. Use the existing `/api/v1` JSON endpoints and their actual DTOs; do not invent backend operations. Preserve pagination and error conventions.
- The Go API requires a bearer token, restricts browser Origins, and validates the HTTP Host. Keep the token in the Tauri host side, not React/WebView state, local storage, URLs, or logs. Use a narrow Tauri bridge for authenticated REST calls and SSE delivery. Do not weaken Go authentication, Origin, or Host checks to make direct renderer requests work.
- Resolve the backend address and token path consistently with the Go configuration. Permit only the configured loopback backend; do not turn the bridge into a general HTTP proxy or shell executor.
- SSE is live only. On connection loss, show a reconnecting state and refresh authoritative status and log history after reconnect. Close streams and listeners when windows close.
- Treat process logs as potentially sensitive output: do not put their contents in OS notifications, telemetry, or routine desktop logs.
- In this phase the Go backend runs as a separate local process. Document development startup for both processes. Bundling and launching the Go binary as a Tauri sidecar is a later packaging task.

## Delivery

- Implement the UI and overlay against the real backend, with typed TypeScript request/response shapes and one shared client layer. Use mock data only in tests and isolated previews.
- Add focused tests for request/error handling and the main lifecycle interactions. Verify the TypeScript build, Tauri compilation on the available host, and the main and overlay layouts at desktop sizes. Report any platform checks that could not run.
- Update the README with desktop setup and run instructions. Preserve existing Go behavior and unrelated worktree changes.
- Finish with a short report: what works, how it was verified, and remaining packaging or native-platform limits.
