# DevOrbit backend

DevOrbit has a local Go API and an in-memory process supervisor. Each workspace stores its service configuration in `.devorbit/workspace.yaml`; SQLite stores workspace metadata and stable service identities. The Go module lives in `server/`.

See the [backend architecture diagram](../docs/architecture.md) for the module and data flows.

## Requirements and setup

- Go 1.24
- A C compiler for the SQLite driver: GCC on Linux, Xcode command-line tools on macOS, or MinGW-w64 GCC on Windows. Build with `CGO_ENABLED=1` (the native-build default when a compiler is available).

From `server/`:

```sh
go run ./cmd/devorbit
```

The backend creates `devorbit.db` inside `DEVORBIT_DATA_DIR` on first startup. SQLite is embedded in the executable; no database server, password, or SQLite CLI is required. Optional settings are listed in `.env.example`. The backend reads `server/.env` from the repository root, or `.env` from `server/`; exported environment variables take precedence.

SQLite starts with an empty workspace list; PostgreSQL workspaces are not imported.

The backend binds to `127.0.0.1:7456` by default. `DEVORBIT_HOST` accepts loopback IP addresses only.

The backend runs on the host to manage local project processes. The desktop installer bundles it and starts it automatically; see [installing a release](../docs/install-release.md).

| Variable | Default | Purpose |
| --- | --- | --- |
| `DEVORBIT_HOST` | `127.0.0.1` | Local listen address |
| `DEVORBIT_PORT` | `7456` | HTTP port |
| `DEVORBIT_DATA_DIR` | User config directory + `devorbit` | SQLite database and local application data |
| `DEVORBIT_LOG_DIR` | `<data dir>/logs` | Managed process log files |
| `DEVORBIT_SHUTDOWN_TIMEOUT` | `10s` | HTTP and process shutdown deadlines |
| `DEVORBIT_AUTH_TOKEN_PATH` | `<data dir>/auth-token` | Local bearer token file |

Path settings must be absolute. The process creates missing data and log directories with private permissions. On Unix, the token file is created with mode `0600`; an existing token or its parent directory with broader permissions is rejected. On Windows, the token file receives a current-user/System ACL, and startup rejects a token directory or existing file that does not pass the private ACL check. Keep custom paths within the current user's private profile.

Back up the private data directory with the backend stopped, including `devorbit.db`, workspace YAML, logs, and traffic captures. Keep the database on a local disk.

## API

- `GET /health` is unauthenticated and checks database connectivity. It returns `503` when the database is unavailable.
- `GET /api/v1/system/info` requires `Authorization: Bearer <token>`.
- `POST /api/v1/workspaces` creates a workspace from `name` and optional `description`, with `.devorbit/workspace.yaml` under `DEVORBIT_DATA_DIR/workspaces/<workspace UUID>`.
- `GET /api/v1/workspaces` lists workspaces in creation order, with optional `limit` (1–100, default 50) and `offset` (nonnegative, default 0).
- `GET /api/v1/workspaces/{id}` retrieves one workspace.
- `POST /api/v1/workspaces/{id}/start` loads validated YAML and attempts to start its services in YAML key order.
- `POST /api/v1/workspaces/{id}/stop` stops active managed services, including ones removed from YAML outside DevOrbit.
- `GET /api/v1/workspaces/{id}/status` returns all registered services' runtime states and aggregate counts.
- `PATCH /api/v1/workspaces/{id}` updates supplied `name` or `description` fields; an empty description clears it.
- `DELETE /api/v1/workspaces/{id}` deletes workspace metadata, service identities, and `.devorbit/workspace.yaml`; returns `409` while services or shell terminals are running. Empty managed workspace directories are removed.
- `GET /api/v1/workspaces/{id}/terminals` lists that workspace's shell sessions, including exited sessions that have not been closed.
- `POST /api/v1/workspaces/{id}/terminals` starts the default shell in the user's home directory and returns its session. At most 16 sessions may exist per workspace.
- `DELETE /api/v1/workspaces/{id}/terminals/{terminal_id}` stops and removes a shell session.
- `GET /api/v1/workspaces/{id}/terminals/{terminal_id}/stream` upgrades to an authenticated WebSocket. It uses the same input, resize, close, sequence, replay, and gap frames as service terminals. Closing the WebSocket only detaches the viewer.
- `GET /api/v1/workspaces/{id}/configuration` returns the YAML content and its revision.
- `POST /api/v1/workspaces/{id}/configuration/validate` validates a `content` string without saving it.
- `PUT /api/v1/workspaces/{id}/configuration` atomically saves validated `content` with an expected `revision`. A stale revision returns `409`.
- `POST /api/v1/workspaces/{id}/services` adds a service to YAML.
- `GET /api/v1/workspaces/{id}/services` lists that workspace's services with the same `limit` and `offset` convention as workspaces.
- `GET /api/v1/services/{id}` retrieves a service configuration.
- `PATCH /api/v1/services/{id}` updates supplied configuration fields.
- `DELETE /api/v1/services/{id}?revision=<revision>` removes a stopped service from YAML.
- `POST /api/v1/services/{id}/start` starts a registered enabled service and returns `202` with its runtime status. A concurrent or repeated start returns `409`.
- `POST /api/v1/services/{id}/stop` waits for termination and returns `200` with runtime status. Stopping an already terminated or never started registered service is safe.
- `POST /api/v1/services/{id}/restart` stops the old process, reloads the saved configuration, and returns `200` after the replacement launches. A conflicting lifecycle operation returns `409`.
- `GET /api/v1/services/{id}/status` returns the current session's runtime status; a service never started in this session is `stopped`.
- `GET /api/v1/services/{id}/terminal/{execution_id}` upgrades to an authenticated WebSocket for that service execution. Send JSON `{"type":"input","data":"text"}`, `{"type":"resize","cols":80,"rows":24}`, or `{"type":"close"}`. Output frames contain a sequence number and base64 `data`; `?after=<sequence>` replays up to 1 MiB of recent output, with a `gap` frame if older bytes are gone.
- `GET /api/v1/services/{id}/logs` returns the newest persisted process logs for a service.
- `GET /api/v1/workspaces/{id}/logs` returns the newest persisted logs across that workspace's services.
- `GET /api/v1/events` streams live process and log events with Server-Sent Events. Optional `workspace_id` and `service_id` query parameters filter the stream.

All service routes require the same bearer token as workspace routes. Service create and patch requests include the current YAML `revision`; stale revisions, duplicate keys, and attempts to remove running services return `409`. Invalid YAML returns `400`. Responses are JSON objects for a single service and JSON arrays for lists; successful deletes return `204` without a body. Errors use `{"error":{"code":"...","message":"..."}}`.

Create a service under an existing workspace:

```json
{"name":"api","command":"go","args":["run","./cmd/api"],"working_directory":"backend","revision":"<current YAML revision>"}
```

The YAML format is limited to `services`, with `command` and optional `cwd` per service. For example:

```yaml
services:
  api:
    command: go run ./cmd/api
    cwd: backend
```

`PATCH` accepts `command`, `args`, and `working_directory` with a current `revision`. The former `ports` field is unsupported in new YAML and API requests; startup deletes it from existing configuration files. A YAML key determines the service identity: editing that key keeps its UUID and log association; renaming it creates a new identity. External file edits are read on the next request and desktop refresh. A running process keeps the configuration it started with until explicitly restarted; YAML changes never trigger a restart.

Local HTTP traffic inspection uses detected listening TCP ports from the running service process and its children. Select the HTTP port when several are detected; inspection verifies that the selected port responds to HTTP before opening a gateway.

Send HTTP requests to the temporary gateway URL to record the original request path, query, headers, cookies, body, and the forwarded response status, headers, trailers, and body. Direct service requests bypass the gateway. Inspection stores captures under `DEVORBIT_DATA_DIR/traffic` with private file permissions. History survives disabling inspection and restarting DevOrbit; clear it in the inspector or let it expire after seven days. Each session retains its newest 500 exchanges, and all sessions share a 512 MiB storage quota. When the quota is reached, forwarding continues and the affected exchange shows that only part of its body was retained. The detail API returns 256 KiB previews; the body download endpoints stream all retained bytes. Captures can contain credentials and other secrets, so keep the data directory private. HTTPS interception and WebSocket frame capture are not supported.

YAML commands are split into an executable and arguments and run directly, without shell expansion. Relative `cwd` paths must stay inside the workspace; absolute paths may point to another existing directory, but make the YAML machine-specific. No command runs during configuration. Anyone who can edit service configurations can execute commands as the DevOrbit OS user. DevOrbit is not a process sandbox.

The token is generated once using a cryptographic random source and stored at `DEVORBIT_AUTH_TOKEN_PATH`. The token directory and file must be private; startup fails if their permissions or ACL cannot be verified. The Tauri desktop host reads the token directly, sends it in the `Authorization: Bearer` header for REST and SSE requests, and keeps it out of renderer code and URLs. Browser requests with an `Origin` header are accepted only from the API's own origin. No wildcard CORS policy is enabled. The server also requires the configured IP and port in the HTTP Host header. JSON write requests require `Content-Type: application/json` and have a 1 MiB body limit. Requests are logged by method, route pattern, status, and duration without query strings or authorization headers.

The process supervisor runs executables directly with their argument arrays, resolves working directories at start time, inherits the backend environment, and tracks `stopped`, `starting`, `running`, `stopping`, `exited`, and `failed` states in memory. A `running` status means the OS started the process; it does not indicate application readiness. The workspace status response reports TCP listening ports found on each running service process and its current children; the desktop overview refreshes these ports every four seconds. Port detection is best effort when OS process details are unavailable, and intentionally detached children cannot be attributed after their parent exits. Exit code and timestamps are reported after the process ends. A failed or exited service may be started again. Runtime status resets when DevOrbit restarts.

Each successful process start has a new `execution_id` in its runtime status and owns a PTY. On Linux and macOS, the backend creates a private tmux session named after the service and attaches that PTY to it. The desktop shows one inline terminal viewport for the selected workspace with a compact service-session switcher; switching reattaches to an existing stream. The reported PID is the original tmux pane's shell process. Developers can split tmux panes with its normal prefix keys or the viewport controls and click a pane to focus it; Stop still interrupts the original service pane. When the service command exits, status and logs close for that execution while the tmux pane becomes an interactive shell. Service Start or Restart replaces the previous tmux session. `tmux` must be installed; a missing executable causes service start to fail clearly. Windows continues to use a direct PTY and does not have the shell-after-exit behavior. New executions merge stdout and stderr into a readable `terminal` log stream under `DEVORBIT_LOG_DIR/services/<service-id>/<execution-id>/`; older `stdout` and `stderr` entries remain available. ANSI control sequences are removed from new terminal log entries, while the WebSocket sends raw terminal bytes. The PTY echoes normal typed input, so commands and other echoed text can also appear in service logs; programs can disable echo for sensitive prompts. Entries contain workspace, service, execution, stream, UTC timestamp, sequence, message, and an ID. Output without newlines is captured while the process runs; long output is divided into 16 KiB chunks without truncation. Read boundaries may split one line across entries. The per-service memory cache holds at most 500 entries and 2 MiB of message data. The bounded ingestion queue drops entries if persistence cannot keep up; the service log response includes `diagnostics.dropped_entries` and `diagnostics.storage_errors` for the current backend session.

Shell terminals are independent of configured services. They use the user's default shell in the user's home directory, stay running while the desktop view is closed, and retain up to 1 MiB of recent output in memory for reattachment. Exited sessions remain listed until deleted. Shell input and output are not written to service logs, and all shell sessions end when the backend shuts down. Creating a shell requires the same API token that permits service command execution; DevOrbit does not sandbox either kind of process.

Log files rotate at 10 MiB, keep five files per execution, and retain at most 20 executions per service for seven days. Expired executions are removed at startup and when that service starts again. Log queries read persisted files, including after a backend restart. They accept `limit` (1–500, default 100) and `before` (the previous page's `next_cursor`); entries are returned newest first. Logs may contain secrets printed by child processes, so log retrieval requires the normal API token. Log retrieval scans the retained files and may be slower for large histories.

The events endpoint uses the same bearer token and origin check as other API routes. Each SSE frame has `id`, `event`, and JSON `data` fields. The JSON envelope contains `id`, `type`, `timestamp`, `workspace_id`, `service_id`, optional `execution_id`, and event-specific `data`. Event types are `process.started`, `process.stopping`, `process.stopped`, `process.exited`, `process.failed`, `process.restarted`, and `log.created`. Process payloads contain state and available PID or exit code; log payloads contain the captured stream, sequence, and message. The connection sends a comment heartbeat every 15 seconds. Events are live only: reconnecting does not replay missed events. Use the persisted log endpoints for history. A client that cannot keep up is disconnected and may reconnect. Like persisted logs, streamed process output can contain secrets printed by child processes.

Stop sends a graceful interrupt and waits up to `DEVORBIT_SHUTDOWN_TIMEOUT` (default `10s`), then forces termination. On Linux and macOS, service Stop sends Ctrl+C to the tmux pane and force closes its tmux server if the service does not exit in time. Independent shell terminals still use their own process groups. Intentionally detached descendants may survive. On Windows, the ConPTY launcher assigns the child to a Job Object and cancels its context to terminate the tree when needed. Job assignment is best effort; if it fails, cancellation terminates only the direct child. Backend shutdown rejects new starts and restarts, then stops active services concurrently and waits for their direct processes to be reaped. Only one lifecycle operation can act on a service at a time; concurrent stop requests share the same result, while conflicting start or restart requests return `409`.

Workspace start and stop return `workspace_id`, `operation`, `total`, `succeeded`, `failed`, `skipped`, and ordered `results`. Start outcomes are `started`, `already_running`, `failed`, and `conflict`; stop outcomes are `stopped`, `already_stopped`, `failed`, and `conflict`. `succeeded` counts newly started or stopped services, `skipped` counts already running and already stopped services, and `failed` counts both failures and lifecycle conflicts. One failure does not roll back or prevent attempts on other services. Per-service error text is generic; command values are never returned. A concurrent workspace operation on the same workspace returns `409`; different workspaces may operate concurrently. Individual service requests may overlap and are resolved by the supervisor's per-service lifecycle rules.

Aggregate status is `transitioning` when any service is starting or stopping; otherwise `running` when at least one service is enabled and all enabled services are running; otherwise `partial` when any service is running; otherwise `idle`. An empty workspace is never `running`. The response is a best-effort snapshot; a process may exit immediately afterward. Workspace status uses generic `last_error` text to avoid exposing internal execution details. A successful launch reports that the OS created the process, not that the application is ready.

## Schema and generated queries

Startup migrates workspace metadata and the `service_identities` table, which maps each YAML service key to a stable UUID. It removes the unused legacy `services` table and reports a startup error if that cleanup fails, then creates an empty `.devorbit/workspace.yaml` for an existing workspace if the file is missing. Legacy database service definitions are discarded; add any services you still need to YAML. Existing YAML files have obsolete service `ports` entries removed atomically; other service settings and comments are retained. Files without those entries are left unchanged. An inaccessible workspace is reported in the backend log without blocking startup.

After changing the workspace model, run `go generate ./...` to regenerate `internal/dal/query`; do not edit generated files. The generator uses `ApplyBasic` on the workspace model and does not require a live database. The application injects its DB into repositories, which use `query.Use(db)` for generated workspace queries. Repository methods pass each request context to GORM Gen for cancellation and deadlines. A caller needing a transaction can construct a repository with that transaction.

Each module defines its endpoints in its own `routes` package. `internal/routes/routes.go` passes the authenticated `/api/v1` router to each module for registration.

Workspace and service configuration modules keep JSON request and response types in `dto`; the service repository stores identities only. Shared pagination parsing lives in `internal/httpinput`.

## Tests

```sh
go test ./...
go test -race ./...
go vet ./...
go build -buildvcs=false ./cmd/devorbit
```

Migration and repository integration tests always run against isolated SQLite files in temporary directories. They cover persistence after reopening, connection settings, UUIDs, uniqueness, foreign keys, cascading deletion, concurrent identity resolution, and transaction rollback. Release jobs run the database tests on each target OS before packaging.

Current limitations: there is no automatic restart. SSE events are in memory and have no replay after a disconnect or backend restart. Workspace orchestration does not resolve service dependencies or persist runtime state. Editing a running service changes only future starts; removing it through DevOrbit is rejected until it stops. Backend structured logs still go to standard error; managed-process logs are stored separately in `DEVORBIT_LOG_DIR`.
