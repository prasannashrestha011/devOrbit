# DevOrbit backend

DevOrbit has a local Go API with workspace and service configuration management and an in-memory process supervisor. The Go module lives at the repository root.

## Requirements and setup

- Go 1.24
- A running PostgreSQL 17 server

Create a PostgreSQL role and database for DevOrbit, then set `DEVORBIT_DATABASE_URL` to its connection string. For a local PostgreSQL instance, a typical setup is:

```sql
CREATE ROLE devorbit LOGIN PASSWORD 'choose-a-private-password';
CREATE DATABASE devorbit OWNER devorbit;
```

Run these as a PostgreSQL administrator. The `.env.example` file lists all settings; the application reads process environment variables and does not load `.env` files itself.

```sh
export DEVORBIT_DATABASE_URL='postgres://devorbit:your-password@127.0.0.1:5432/devorbit?sslmode=disable'
go run ./cmd/devorbit
```

The backend binds to `127.0.0.1:7456` by default. `DEVORBIT_HOST` accepts loopback IP addresses only.

| Variable | Default | Purpose |
| --- | --- | --- |
| `DEVORBIT_HOST` | `127.0.0.1` | Local listen address |
| `DEVORBIT_PORT` | `7456` | HTTP port |
| `DEVORBIT_DATABASE_URL` | Required | PostgreSQL connection string |
| `DEVORBIT_DATA_DIR` | User config directory + `devorbit` | Local application data |
| `DEVORBIT_LOG_DIR` | `<data dir>/logs` | Managed process log files |
| `DEVORBIT_SHUTDOWN_TIMEOUT` | `10s` | HTTP and process shutdown deadlines |
| `DEVORBIT_AUTH_TOKEN_PATH` | `<data dir>/auth-token` | Local bearer token file |

Path settings must be absolute. The process creates missing data and log directories with private permissions. On Unix, the token file is created with mode `0600`; an existing token or its parent directory with broader permissions is rejected. On Windows, the token file receives a current-user/System ACL, and startup rejects a token directory or existing file that does not pass the private ACL check. Keep custom paths within the current user's private profile.

## API

- `GET /health` is unauthenticated and checks PostgreSQL connectivity. It returns `503` when the database is unavailable.
- `GET /api/v1/system/info` requires `Authorization: Bearer <token>`.
- `POST /api/v1/workspaces` creates a workspace from `name`, optional `description`, and an existing `root_path`.
- `GET /api/v1/workspaces` lists workspaces in creation order, with optional `limit` (1–100, default 50) and `offset` (nonnegative, default 0).
- `GET /api/v1/workspaces/{id}` retrieves one workspace.
- `POST /api/v1/workspaces/{id}/start` attempts to start every enabled service in creation order and returns a per-service result.
- `POST /api/v1/workspaces/{id}/stop` stops active managed services in reverse creation order, including disabled services that are still running.
- `GET /api/v1/workspaces/{id}/status` returns all registered services' runtime states and aggregate counts.
- `PATCH /api/v1/workspaces/{id}` updates supplied `name`, `description`, or `root_path` fields; an empty description clears it.
- `DELETE /api/v1/workspaces/{id}` permanently deletes an empty workspace; returns `409` while services remain.
- `POST /api/v1/workspaces/{id}/services` creates a service configuration.
- `GET /api/v1/workspaces/{id}/services` lists that workspace's services with the same `limit` and `offset` convention as workspaces.
- `GET /api/v1/services/{id}` retrieves a service configuration.
- `PATCH /api/v1/services/{id}` updates supplied configuration fields.
- `DELETE /api/v1/services/{id}` permanently deletes a service configuration.
- `POST /api/v1/services/{id}/start` starts a registered enabled service and returns `202` with its runtime status. A concurrent or repeated start returns `409`.
- `POST /api/v1/services/{id}/stop` waits for termination and returns `200` with runtime status. Stopping an already terminated or never started registered service is safe.
- `POST /api/v1/services/{id}/restart` stops the old process, reloads the saved configuration, and returns `200` after the replacement launches. A conflicting lifecycle operation returns `409`.
- `GET /api/v1/services/{id}/status` returns the current session's runtime status; a service never started in this session is `stopped`.
- `GET /api/v1/services/{id}/logs` returns the newest persisted process logs for a service.
- `GET /api/v1/workspaces/{id}/logs` returns the newest persisted logs across that workspace's services.
- `GET /api/v1/events` streams live process and log events with Server-Sent Events. Optional `workspace_id` and `service_id` query parameters filter the stream.

All service routes require the same bearer token as workspace routes. An invalid request returns `400`, a missing workspace or service returns `404`, and a duplicate service name in one workspace returns `409`. Responses are JSON objects for a single service and JSON arrays for lists; successful deletes return `204` without a body. Errors use `{"error":{"code":"...","message":"..."}}`.

Create a service under an existing workspace:

```json
{"name":"Go Backend","command":"go","args":["run","./cmd/api"],"working_directory":"backend","env":{"APP_ENV":"development"},"enabled":true,"auto_restart":false}
```

The response has the same fields plus `id`, `workspace_id`, `created_at`, and `updated_at`. `enabled` defaults to `true`, `auto_restart` to `false`, and omitted `args` and `env` to `[]` and `{}`. `PATCH` accepts any subset of `name`, `command`, `args`, `working_directory`, `env`, `enabled`, and `auto_restart`. Supplied `args` and `env` replace the complete array or map; for example `{"args":[],"env":{},"enabled":false}` clears both collections and disables the service. `null` and unknown fields are rejected.

`command` is one executable name or path; arguments remain separate strings. Relative working directories are resolved under the workspace root, checked on the local filesystem, and stored as normalized relative paths when they remain inside that root. An empty value means the workspace root. Explicit absolute directories outside the root are stored as normalized absolute paths. No directory is created and no command runs during configuration. Environment overrides preserve key case and are stored as plaintext JSONB; do not put credentials there until secure secret management exists. Anyone who can edit service configurations can execute commands as the DevOrbit OS user. DevOrbit is not a process sandbox. A workspace must have all its services removed before it can be deleted.

The token is generated once using a cryptographic random source and stored at `DEVORBIT_AUTH_TOKEN_PATH`. The token directory and file must be private; startup fails if their permissions or ACL cannot be verified. A future Electron main process should read the token directly, send it in the `Authorization: Bearer` header for REST and SSE requests, and keep it out of renderer code and URLs. Browser requests with an `Origin` header are accepted only from the API's own origin. No wildcard CORS policy is enabled. The server also requires the configured IP and port in the HTTP Host header. JSON write requests require `Content-Type: application/json` and have a 1 MiB body limit. Requests are logged by method, route pattern, status, and duration without query strings or authorization headers.

The process supervisor runs executables directly with their argument arrays, resolves working directories at start time, inherits the backend environment with service overrides, and tracks `stopped`, `starting`, `running`, `stopping`, `exited`, and `failed` states in memory. A `running` status means the OS started the process; it does not indicate application readiness. Exit code and timestamps are reported after the process ends. A failed or exited service may be started again. Runtime status resets when DevOrbit restarts.

Each successful process start has a new `execution_id` in its runtime status. Standard output and standard error are captured separately as UTC-stamped JSONL entries under `DEVORBIT_LOG_DIR/services/<service-id>/<execution-id>/`. Entries contain workspace, service, execution, stream, timestamp, sequence, message, and an ID. Newlines, carriage returns, and ANSI controls are preserved as JSON-escaped text. Output without newlines is captured while the process runs; long output is divided into 16 KiB chunks without truncation. Read boundaries may split one line across entries. The per-service memory cache holds at most 500 entries and 2 MiB of message data. The bounded ingestion queue drops entries if persistence cannot keep up; the service log response includes `diagnostics.dropped_entries` and `diagnostics.storage_errors` for the current backend session.

Log files rotate at 10 MiB, keep five files per execution, and retain at most 20 executions per service for seven days. Expired executions are removed at startup and when that service starts again. Log queries read persisted files, including after a backend restart. They accept `limit` (1–500, default 100) and `before` (the previous page's `next_cursor`); entries are returned newest first. Logs may contain secrets printed by child processes, so log retrieval requires the normal API token. Log retrieval scans the retained files and may be slower for large histories.

The events endpoint uses the same bearer token and origin check as other API routes. Each SSE frame has `id`, `event`, and JSON `data` fields. The JSON envelope contains `id`, `type`, `timestamp`, `workspace_id`, `service_id`, optional `execution_id`, and event-specific `data`. Event types are `process.started`, `process.stopping`, `process.stopped`, `process.exited`, `process.failed`, `process.restarted`, and `log.created`. Process payloads contain state and available PID or exit code; log payloads contain the captured stream, sequence, and message. The connection sends a comment heartbeat every 15 seconds. Events are live only: reconnecting does not replay missed events. Use the persisted log endpoints for history. A client that cannot keep up is disconnected and may reconnect. Like persisted logs, streamed process output can contain secrets printed by child processes.

Stop sends a graceful interrupt and waits up to `DEVORBIT_SHUTDOWN_TIMEOUT` (default `10s`), then forces termination. On Linux and macOS, each service starts in its own process group, and signals target that group, including descendants of launchers such as `go run` or `npm`. Intentionally detached descendants that leave the process group may survive. On Windows, DevOrbit attaches the child to a Job Object and terminates that job if graceful console control is unavailable or times out. Job assignment occurs just after process creation, so a child spawned in that brief interval may escape the job. Backend shutdown rejects new starts and restarts, then stops active services concurrently and waits for their direct processes to be reaped. Only one lifecycle operation can act on a service at a time; concurrent stop requests share the same result, while conflicting start or restart requests return `409`. `auto_restart` remains a stored setting and is not active.

Workspace start and stop return `workspace_id`, `operation`, `total`, `succeeded`, `failed`, `skipped`, and ordered `results`. Start outcomes are `started`, `already_running`, `disabled`, `failed`, and `conflict`; stop outcomes are `stopped`, `already_stopped`, `failed`, and `conflict`. `succeeded` counts newly started or stopped services, `skipped` counts already running, already stopped, and disabled services, and `failed` counts both failures and lifecycle conflicts. One failure does not roll back or prevent attempts on other services. Per-service error text is generic; command and environment values are never returned. A concurrent workspace operation on the same workspace returns `409`; different workspaces may operate concurrently. Individual service requests may overlap and are resolved by the supervisor's per-service lifecycle rules.

Aggregate status is `transitioning` when any service is starting or stopping; otherwise `running` when at least one service is enabled and all enabled services are running; otherwise `partial` when any service is running; otherwise `idle`. An empty workspace or one with no enabled services is never `running`. Counts include disabled services in their current runtime state. The response is a best-effort snapshot; a process may exit immediately afterward. Workspace status uses generic `last_error` text to avoid exposing internal execution details. A successful launch reports that the OS created the process, not that the application is ready.

## Schema and generated queries

Startup calls `database.Migrate` with Workspace followed by Service. The `services` table uses PostgreSQL UUID keys, JSONB `args` and `env`, a workspace foreign key with restricted deletion, an index on `workspace_id`, and a unique index on `(workspace_id, name)`. GORM AutoMigrate does not provide a safe general rollback or every schema evolution; review schema changes before upgrading a populated database.

After changing either model, run `go generate ./...` to regenerate `internal/dal/query`; do not edit generated files. The generator uses `ApplyBasic` on the Go models and does not require a live database. The application injects its DB into repositories, which use `query.Use(db)` for generated queries. Repository methods pass each request context to GORM Gen for cancellation and deadlines. A caller needing a transaction can construct a repository with that transaction.

Each module defines its endpoints in its own `routes` package. `internal/routes/routes.go` passes the authenticated `/api/v1` router to each module for registration.

Workspace and service configuration modules keep JSON request and response types in `dto`, domain checks in `validation`, and database work in `repository`. Shared pagination parsing lives in `internal/httpinput`.

## Tests

```sh
go test ./...
go test -race ./...
go vet ./...
go build -buildvcs=false ./cmd/devorbit
```

Set `DEVORBIT_TEST_DATABASE_URL` to a separate PostgreSQL 17 database to run migration and repository integration tests. They create and drop only unique temporary schemas. Without that variable, these tests are skipped.

Current limitations: there is no automatic restart or desktop UI. SSE events are in memory and have no replay after a disconnect or backend restart. Workspace orchestration does not resolve service dependencies or persist runtime state. Updating or deleting a service configuration does not stop an already running process. Backend structured logs still go to standard error; managed-process logs are stored separately in `DEVORBIT_LOG_DIR`.
