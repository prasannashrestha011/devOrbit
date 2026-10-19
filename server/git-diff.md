# README.md

diff --git a/README.md b/README.md
index 9fc33d3..fe4043b 100644
--- a/README.md
+++ b/README.md
@@ -1,6 +1,6 @@
 # DevOrbit backend
 
-DevOrbit has a local Go API with workspace and service configuration management. The Go module lives at the repository root.
+DevOrbit has a local Go API with workspace and service configuration management and an in-memory process supervisor. The Go module lives at the repository root.
 
 ## Requirements and setup
 
@@ -29,11 +29,11 @@ The backend binds to `127.0.0.1:7456` by default. `DEVORBIT_HOST` accepts loopba
 | `DEVORBIT_PORT` | `7456` | HTTP port |
 | `DEVORBIT_DATABASE_URL` | Required | PostgreSQL connection string |
 | `DEVORBIT_DATA_DIR` | User config directory + `devorbit` | Local application data |
-| `DEVORBIT_LOG_DIR` | `<data dir>/logs` | Reserved application log directory |
-| `DEVORBIT_SHUTDOWN_TIMEOUT` | `10s` | Graceful HTTP shutdown deadline |
+| `DEVORBIT_LOG_DIR` | `<data dir>/logs` | Managed process log files |
+| `DEVORBIT_SHUTDOWN_TIMEOUT` | `10s` | HTTP and process shutdown deadlines |
 | `DEVORBIT_AUTH_TOKEN_PATH` | `<data dir>/auth-token` | Local bearer token file |
 
-Path settings must be absolute. The process creates missing data and log directories with private permissions. On Unix, the token file is created with mode `0600`; an existing token or its parent directory with broader permissions is rejected. On Windows, use a token location within the current user's private profile and ensure its ACL excludes other users.
+Path settings must be absolute. The process creates missing data and log directories with private permissions. On Unix, the token file is created with mode `0600`; an existing token or its parent directory with broader permissions is rejected. On Windows, the token file receives a current-user/System ACL, and startup rejects a token directory or existing file that does not pass the private ACL check. Keep custom paths within the current user's private profile.
 
 ## API
 
@@ -42,6 +42,9 @@ Path settings must be absolute. The process creates missing data and log directo
 - `POST /api/v1/workspaces` creates a workspace from `name`, optional `description`, and an existing `root_path`.
 - `GET /api/v1/workspaces` lists workspaces in creation order, with optional `limit` (1–100, default 50) and `offset` (nonnegative, default 0).
 - `GET /api/v1/workspaces/{id}` retrieves one workspace.
+- `POST /api/v1/workspaces/{id}/start` attempts to start every enabled service in creation order and returns a per-service result.
+- `POST /api/v1/workspaces/{id}/stop` stops active managed services in reverse creation order, including disabled services that are still running.
+- `GET /api/v1/workspaces/{id}/status` returns all registered services' runtime states and aggregate counts.
 - `PATCH /api/v1/workspaces/{id}` updates supplied `name`, `description`, or `root_path` fields; an empty description clears it.
 - `DELETE /api/v1/workspaces/{id}` permanently deletes an empty workspace; returns `409` while services remain.
 - `POST /api/v1/workspaces/{id}/services` creates a service configuration.
@@ -49,6 +52,13 @@ Path settings must be absolute. The process creates missing data and log directo
 - `GET /api/v1/services/{id}` retrieves a service configuration.
 - `PATCH /api/v1/services/{id}` updates supplied configuration fields.
 - `DELETE /api/v1/services/{id}` permanently deletes a service configuration.
+- `POST /api/v1/services/{id}/start` starts a registered enabled service and returns `202` with its runtime status. A concurrent or repeated start returns `409`.
+- `POST /api/v1/services/{id}/stop` waits for termination and returns `200` with runtime status. Stopping an already terminated or never started registered service is safe.
+- `POST /api/v1/services/{id}/restart` stops the old process, reloads the saved configuration, and returns `200` after the replacement launches. A conflicting lifecycle operation returns `409`.
+- `GET /api/v1/services/{id}/status` returns the current session's runtime status; a service never started in this session is `stopped`.
+- `GET /api/v1/services/{id}/logs` returns the newest persisted process logs for a service.
+- `GET /api/v1/workspaces/{id}/logs` returns the newest persisted logs across that workspace's services.
+- `GET /api/v1/events` streams live process and log events with Server-Sent Events. Optional `workspace_id` and `service_id` query parameters filter the stream.
 
 All service routes require the same bearer token as workspace routes. An invalid request returns `400`, a missing workspace or service returns `404`, and a duplicate service name in one workspace returns `409`. Responses are JSON objects for a single service and JSON arrays for lists; successful deletes return `204` without a body. Errors use `{"error":{"code":"...","message":"..."}}`.
 
@@ -60,9 +70,23 @@ Create a service under an existing workspace:
 
 The response has the same fields plus `id`, `workspace_id`, `created_at`, and `updated_at`. `enabled` defaults to `true`, `auto_restart` to `false`, and omitted `args` and `env` to `[]` and `{}`. `PATCH` accepts any subset of `name`, `command`, `args`, `working_directory`, `env`, `enabled`, and `auto_restart`. Supplied `args` and `env` replace the complete array or map; for example `{"args":[],"env":{},"enabled":false}` clears both collections and disables the service. `null` and unknown fields are rejected.
 
-`command` is one executable name or path; arguments remain separate strings. Relative working directories are resolved under the workspace root, checked on the local filesystem, and stored as normalized relative paths when they remain inside that root. An empty value means the workspace root. Explicit absolute directories outside the root are stored as normalized absolute paths. No directory is created and no command runs during configuration. Environment overrides preserve key case and are stored as plaintext JSONB; do not put credentials there until secure secret management exists. A workspace must have all its services removed before it can be deleted.
+`command` is one executable name or path; arguments remain separate strings. Relative working directories are resolved under the workspace root, checked on the local filesystem, and stored as normalized relative paths when they remain inside that root. An empty value means the workspace root. Explicit absolute directories outside the root are stored as normalized absolute paths. No directory is created and no command runs during configuration. Environment overrides preserve key case and are stored as plaintext JSONB; do not put credentials there until secure secret management exists. Anyone who can edit service configurations can execute commands as the DevOrbit OS user. DevOrbit is not a process sandbox. A workspace must have all its services removed before it can be deleted.
 
-The token is generated once using a cryptographic random source and stored at `DEVORBIT_AUTH_TOKEN_PATH`. A future Electron main process should read it directly and keep it out of renderer code. Browser requests with an `Origin` header are accepted only from the API's own origin. No wildcard CORS policy is enabled. Requests are logged by method, path, status, and duration without query strings or authorization headers.
+The token is generated once using a cryptographic random source and stored at `DEVORBIT_AUTH_TOKEN_PATH`. The token directory and file must be private; startup fails if their permissions or ACL cannot be verified. A future Electron main process should read the token directly, send it in the `Authorization: Bearer` header for REST and SSE requests, and keep it out of renderer code and URLs. Browser requests with an `Origin` header are accepted only from the API's own origin. No wildcard CORS policy is enabled. The server also requires the configured IP and port in the HTTP Host header. JSON write requests require `Content-Type: application/json` and have a 1 MiB body limit. Requests are logged by method, route pattern, status, and duration without query strings or authorization headers.
+
+The process supervisor runs executables directly with their argument arrays, resolves working directories at start time, inherits the backend environment with service overrides, and tracks `stopped`, `starting`, `running`, `stopping`, `exited`, and `failed` states in memory. A `running` status means the OS started the process; it does not indicate application readiness. Exit code and timestamps are reported after the process ends. A failed or exited service may be started again. Runtime status resets when DevOrbit restarts.
+
+Each successful process start has a new `execution_id` in its runtime status. Standard output and standard error are captured separately as UTC-stamped JSONL entries under `DEVORBIT_LOG_DIR/services/<service-id>/<execution-id>/`. Entries contain workspace, service, execution, stream, timestamp, sequence, message, and an ID. Newlines, carriage returns, and ANSI controls are preserved as JSON-escaped text. Output without newlines is captured while the process runs; long output is divided into 16 KiB chunks without truncation. Read boundaries may split one line across entries. The per-service memory cache holds at most 500 entries and 2 MiB of message data. The bounded ingestion queue drops entries if persistence cannot keep up; the service log response includes `diagnostics.dropped_entries` and `diagnostics.storage_errors` for the current backend session.
+
+Log files rotate at 10 MiB, keep five files per execution, and retain at most 20 executions per service for seven days. Expired executions are removed at startup and when that service starts again. Log queries read persisted files, including after a backend restart. They accept `limit` (1–500, default 100) and `before` (the previous page's `next_cursor`); entries are returned newest first. Logs may contain secrets printed by child processes, so log retrieval requires the normal API token. Log retrieval scans the retained files and may be slower for large histories.
+
+The events endpoint uses the same bearer token and origin check as other API routes. Each SSE frame has `id`, `event`, and JSON `data` fields. The JSON envelope contains `id`, `type`, `timestamp`, `workspace_id`, `service_id`, optional `execution_id`, and event-specific `data`. Event types are `process.started`, `process.stopping`, `process.stopped`, `process.exited`, `process.failed`, `process.restarted`, and `log.created`. Process payloads contain state and available PID or exit code; log payloads contain the captured stream, sequence, and message. The connection sends a comment heartbeat every 15 seconds. Events are live only: reconnecting does not replay missed events. Use the persisted log endpoints for history. A client that cannot keep up is disconnected and may reconnect. Like persisted logs, streamed process output can contain secrets printed by child processes.
+
+Stop sends a graceful interrupt and waits up to `DEVORBIT_SHUTDOWN_TIMEOUT` (default `10s`), then forces termination. On Linux and macOS, each service starts in its own process group, and signals target that group, including descendants of launchers such as `go run` or `npm`. Intentionally detached descendants that leave the process group may survive. On Windows, DevOrbit attaches the child to a Job Object and terminates that job if graceful console control is unavailable or times out. Job assignment occurs just after process creation, so a child spawned in that brief interval may escape the job. Backend shutdown rejects new starts and restarts, then stops active services concurrently and waits for their direct processes to be reaped. Only one lifecycle operation can act on a service at a time; concurrent stop requests share the same result, while conflicting start or restart requests return `409`. `auto_restart` remains a stored setting and is not active.
+
+Workspace start and stop return `workspace_id`, `operation`, `total`, `succeeded`, `failed`, `skipped`, and ordered `results`. Start outcomes are `started`, `already_running`, `disabled`, `failed`, and `conflict`; stop outcomes are `stopped`, `already_stopped`, `failed`, and `conflict`. `succeeded` counts newly started or stopped services, `skipped` counts already running, already stopped, and disabled services, and `failed` counts both failures and lifecycle conflicts. One failure does not roll back or prevent attempts on other services. Per-service error text is generic; command and environment values are never returned. A concurrent workspace operation on the same workspace returns `409`; different workspaces may operate concurrently. Individual service requests may overlap and are resolved by the supervisor's per-service lifecycle rules.
+
+Aggregate status is `transitioning` when any service is starting or stopping; otherwise `running` when at least one service is enabled and all enabled services are running; otherwise `partial` when any service is running; otherwise `idle`. An empty workspace or one with no enabled services is never `running`. Counts include disabled services in their current runtime state. The response is a best-effort snapshot; a process may exit immediately afterward. Workspace status uses generic `last_error` text to avoid exposing internal execution details. A successful launch reports that the OS created the process, not that the application is ready.
 
 ## Schema and generated queries
 
@@ -72,6 +96,8 @@ After changing either model, run `go generate ./...` to regenerate `internal/dal
 
 Each module defines its endpoints in its own `routes` package. `internal/routes/routes.go` passes the authenticated `/api/v1` router to each module for registration.
 
+Workspace and service configuration modules keep JSON request and response types in `dto`, domain checks in `validation`, and database work in `repository`. Shared pagination parsing lives in `internal/httpinput`.
+
 ## Tests
 
 ```sh
@@ -83,4 +109,4 @@ go build -buildvcs=false ./cmd/devorbit
 
 Set `DEVORBIT_TEST_DATABASE_URL` to a separate PostgreSQL 17 database to run migration and repository integration tests. They create and drop only unique temporary schemas. Without that variable, these tests are skipped.
 
-Current limitations: no process supervisor, log streaming, SSE, or desktop UI. Service configuration does not execute commands or implement automatic restart. The log directory is prepared but file logging is not implemented; structured logs go to standard error.
+Current limitations: there is no automatic restart or desktop UI. SSE events are in memory and have no replay after a disconnect or backend restart. Workspace orchestration does not resolve service dependencies or persist runtime state. Updating or deleting a service configuration does not stop an already running process. Backend structured logs still go to standard error; managed-process logs are stored separately in `DEVORBIT_LOG_DIR`.

# docs/implementation-progress.md

diff --git a/docs/implementation-progress.md b/docs/implementation-progress.md
index 79a9f16..be3ba79 100644
--- a/docs/implementation-progress.md
+++ b/docs/implementation-progress.md
@@ -20,6 +20,42 @@ Implemented the Service persistence model, Workspace foreign key and delete rest
 
 Unit and HTTP API tests cover validation, defaults, duplicate names, authentication, bad IDs and JSON types, partial updates, deletion, and workspace deletion conflict response. A PostgreSQL 17 integration test covers repeated migration, foreign key and unique constraints, JSONB round trips, repository CRUD, concurrent duplicate creation, and workspace deletion conflict. The full `go test -race -count=1 ./...` suite passed on 2026-10-18 against the temporary isolated PostgreSQL 17 container. `go vet ./...`, Linux, Windows, and macOS builds passed. Repeated `go generate ./...` produced identical hashes. Phase 03 acceptance is complete.
 
+## Phase 04 — process supervisor core
+
+Implemented an in-memory process supervisor independent of HTTP and GORM. It reserves each service ID before launching, runs registered executables directly with separate arguments and resolved working directories, applies environment overrides, tracks PID and lifecycle status, observes exits asynchronously, supports retries and concurrent distinct services, and kills/reaps active direct children at backend shutdown. New authenticated start and status routes use the existing service and workspace application layers. Automatic restart and readiness detection remain outside this phase.
+
+Unit tests execute real child processes to cover concurrent starts, immediate exits, retries, failure status, environment and working directory resolution, invalid configuration, and shutdown cleanup. HTTP tests cover authentication, invalid IDs, missing and disabled services, start conflict, and status responses. The full `go test -race -count=1 -buildvcs=false ./...` suite passed on 2026-10-18; focused supervisor race tests passed 20 consecutive runs. `go vet` and Linux, macOS, and Windows builds passed. Phase 04 acceptance is complete.
+
+## Phase 05 — process lifecycle management
+
+Implemented stop and restart in the existing in-memory supervisor, with `stopping` state, per-service operation coordination, graceful interruption, timeout escalation, process-tree cleanup, and concurrent shutdown. Restart reloads the current persisted service configuration after the previous execution terminates. New authenticated stop and restart API endpoints preserve existing response and error conventions. Linux and macOS use isolated process groups; Windows uses Job Objects and attempts console control before force termination.
+
+Unit tests cover idempotent stop, graceful and forced stop, replacement PIDs, restart after stopped, failed, and naturally exited states, configuration reload ordering, concurrent stops and restarts, lifecycle conflicts, shutdown during a stop, and Unix descendant termination. API tests cover authentication, UUID validation, missing and disabled services, stop, restart, and current configuration use. The full `go test -count=1 -buildvcs=false ./...` and `go test -race -count=1 -buildvcs=false ./...` suites passed on 2026-10-18. `go vet -buildvcs=false ./...` and Linux, macOS, and Windows/amd64 builds passed. PostgreSQL integration tests were skipped because `DEVORBIT_TEST_DATABASE_URL` was unset. Phase 05 implementation is complete. Native macOS and Windows process behavior remains unverified, and the Windows launch gap is noted below.
+
+## Phase 06 — workspace orchestration
+
+Added a workspace orchestrator that lists every registered service, sorts by creation time and ID, starts enabled services in that order, stops active services in reverse order, and returns individual outcomes and summary counts even when one service fails. It delegates execution and lifecycle conflicts to the existing process service and supervisor. Per-workspace operation reservations reject overlapping workspace starts and stops without blocking independent workspaces. A status endpoint combines persisted workspace and service details with best-effort in-memory runtime snapshots. Runtime state remains unpersisted.
+
+Authenticated `POST /api/v1/workspaces/{id}/start`, `POST /api/v1/workspaces/{id}/stop`, and `GET /api/v1/workspaces/{id}/status` are registered through the central router. Unit tests cover deterministic ordering, pagination beyond 100 services, disabled and already-running behavior, partial failure, aggregate states, conflicts, shutdown, missing IDs, and real supervisor process launch and cleanup; HTTP tests cover authentication, IDs, missing workspaces, and responses. The full `go test -race -count=1 -buildvcs=false ./...` suite, `go vet -buildvcs=false ./...`, and Linux, macOS/amd64, and Windows/amd64 builds passed on 2026-10-18. PostgreSQL integration tests were skipped because `DEVORBIT_TEST_DATABASE_URL` was unset. Phase 06's supplied prompt ends mid-sentence in the concurrency section; implementation follows all complete requirements present in that file.
+
+## Phase 07 — centralized process logs
+
+Added a filesystem log manager integrated with the existing supervisor's stdout and stderr writers. Each execution gets a unique ID; output is captured in bounded chunks, queued without blocking on file writes, persisted as JSONL, and cached in a bounded per-service memory buffer. The manager rotates files, removes old executions, exposes dropped-entry and storage-error counts, and retrieves service or workspace history with cursor pagination. The authenticated service and workspace log routes use the central router. Tests cover concurrent stream output, partial lines, UTF-8 chunk boundaries, distinct executions, process integration, pagination, persistence across restart, memory bounds, rotation, retention, and API validation. The full Go race suite, vet, and Linux, macOS/amd64, and Windows/amd64 builds passed on 2026-10-18. PostgreSQL integration tests were skipped because `DEVORBIT_TEST_DATABASE_URL` was unset.
+
+The supplied Phase 07 prompt ends mid-sentence in its persistence section. The implementation follows its complete requirements and the stated objective; live streaming remains out of scope.
+
+## Phase 08 — real-time SSE events
+
+Added an in-memory event broker with workspace and service filtering, fan-out to multiple subscribers, bounded per-client channels, slow-client disconnection, and clean closure. The existing supervisor publishes started, stopping, stopped, exited, failed, and restarted events at state transitions; the log manager publishes created entries with the original log timestamp and metadata. Lifecycle payloads exclude service configuration and environment values; log payloads reflect captured output and may contain secrets printed by child processes. The authenticated `GET /api/v1/events` route supports optional workspace and service filters, validates IDs against persisted records, streams SSE frames with heartbeat comments, and clears the request's write deadline for long-lived connections. Application shutdown closes the broker to release streams. Unit and HTTP tests cover filtering, overflow, closure, lifecycle and log publication, authentication, and live delivery. The full Go race suite, vet, and macOS/Windows builds passed on 2026-10-18. PostgreSQL integration tests were skipped because `DEVORBIT_TEST_DATABASE_URL` was unset.
+
+The supplied Phase 08 prompt ends at the first broker requirement. The implementation follows its complete sections and stated objective. SSE has no replay; persisted log endpoints provide history.
+
+## Phase 09 — security and reliability hardening
+
+Reviewed API authentication, Host and Origin checks, token storage, process launch and termination, filesystem paths, logs, events, SSE, and shutdown. Added a strict HTTP Host check at the production server, rejected multiple Origin headers, capped API bodies at 1 MiB, required JSON content type for configuration writes, and tightened SSE and log query filters. Token loading now reads from verified file handles; Unix rejects untrusted symlink paths, wrong owners, and broad permissions. Windows creates the token with an explicit current-user/System DACL and refuses an existing token or directory whose owner or allow ACEs cannot be verified as private. Log storage rejects redirected roots and service directories and skips symlinked log files. Process launch failures now keep OS error details out of status, stop handles natural completion during the transition, SSE writes have rolling deadlines, and log shutdown runs even if process shutdown fails.
+
+The complete Go race suite, vet, and Linux, macOS/amd64, and Windows/amd64 builds passed on 2026-10-18. Windows ACL and process-tree behavior still need native Windows tests; macOS process-tree behavior still needs native macOS tests. PostgreSQL integration tests were skipped because `DEVORBIT_TEST_DATABASE_URL` was unset. See [security review](security-review.md) for findings and residual risks. The supplied Phase 09 prompt ends mid-list in its reconnection section; work follows its complete requirements.
+
 ## Known limitations and next phase
 
-GORM AutoMigrate has no general rollback; review schema changes before upgrading a populated database. Windows token-file privacy relies on the configured directory ACL. Environment overrides are plaintext in PostgreSQL and should not contain secrets. Phase 04 will add the Process Supervisor Core after Phase 03 acceptance.
+GORM AutoMigrate has no general rollback; review schema changes before upgrading a populated database. Native Windows token ACL behavior remains unverified. Environment overrides are plaintext in PostgreSQL and should not contain secrets. Windows Job Object assignment happens immediately after process creation, leaving a brief window in which an early spawned descendant may escape ownership. Windows graceful console control depends on an available shared console; otherwise DevOrbit forces the owned job to terminate. Updating or deleting a running service configuration does not stop its current process. Automatic restart remains pending for later phases. Workspace orchestration has no dependency ordering or readiness detection; creation time and ID determine launch order.

# internal/api/api.go

diff --git a/internal/api/api.go b/internal/api/api.go
index 1e7e7bb..552e32b 100644
--- a/internal/api/api.go
+++ b/internal/api/api.go
@@ -36,6 +36,12 @@ func Router(log *slog.Logger, token, allowedOrigin string, ping func(context.Con
 	})
 	r.Route("/api/v1", func(r chi.Router) {
 		r.Use(authenticate(token, allowedOrigin))
+		r.Use(func(next http.Handler) http.Handler {
+			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
+				req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
+				next.ServeHTTP(w, req)
+			})
+		})
 		r.Get("/system/info", func(w http.ResponseWriter, _ *http.Request) {
 			writeJSON(w, http.StatusOK, map[string]string{"name": "DevOrbit", "api_version": "v1"})
 		})
@@ -59,11 +65,16 @@ func writeError(w http.ResponseWriter, status int, code, message string) {
 func authenticate(token, allowedOrigin string) func(http.Handler) http.Handler {
 	return func(next http.Handler) http.Handler {
 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
-			if origin := r.Header.Get("Origin"); origin != "" && origin != allowedOrigin {
+			if origins := r.Header.Values("Origin"); len(origins) > 1 || len(origins) == 1 && origins[0] != allowedOrigin {
 				writeError(w, http.StatusForbidden, "origin_forbidden", "origin is not allowed")
 				return
 			}
-			provided, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
+			credentials := r.Header.Values("Authorization")
+			if len(credentials) != 1 {
+				writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token required")
+				return
+			}
+			provided, ok := strings.CutPrefix(credentials[0], "Bearer ")
 			if !ok || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
 				writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token required")
 				return

# internal/api/api_test.go

diff --git a/internal/api/api_test.go b/internal/api/api_test.go
index 5156133..7bb90ad 100644
--- a/internal/api/api_test.go
+++ b/internal/api/api_test.go
@@ -10,6 +10,8 @@ import (
 	"net/http/httptest"
 	"strings"
 	"testing"
+
+	"github.com/go-chi/chi/v5"
 )
 
 func testRouter(ping func(context.Context) error) http.Handler {
@@ -79,3 +81,39 @@ func TestUnknownRouteUsesJSONError(t *testing.T) {
 		t.Fatalf("unexpected error response: %d %s", w.Code, w.Body.String())
 	}
 }
+
+func TestRejectsDuplicateOriginAndOversizedBody(t *testing.T) {
+	h := Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
+		r.Post("/size", func(w http.ResponseWriter, req *http.Request) {
+			if _, err := io.Copy(io.Discard, req.Body); err != nil {
+				w.WriteHeader(http.StatusRequestEntityTooLarge)
+				return
+			}
+			w.WriteHeader(http.StatusNoContent)
+		})
+	})
+	req := httptest.NewRequest(http.MethodPost, "/api/v1/size", strings.NewReader("x"))
+	req.Header.Set("Authorization", "Bearer token")
+	req.Header.Add("Origin", "http://127.0.0.1:7456")
+	req.Header.Add("Origin", "https://evil.example")
+	w := httptest.NewRecorder()
+	h.ServeHTTP(w, req)
+	if w.Code != http.StatusForbidden {
+		t.Fatalf("duplicate origins: %d", w.Code)
+	}
+	req = httptest.NewRequest(http.MethodPost, "/api/v1/size", strings.NewReader("x"))
+	req.Header.Add("Authorization", "Bearer token")
+	req.Header.Add("Authorization", "Bearer wrong")
+	w = httptest.NewRecorder()
+	h.ServeHTTP(w, req)
+	if w.Code != http.StatusUnauthorized {
+		t.Fatalf("duplicate credentials: %d", w.Code)
+	}
+	req = httptest.NewRequest(http.MethodPost, "/api/v1/size", strings.NewReader(strings.Repeat("x", 1<<20+1)))
+	req.Header.Set("Authorization", "Bearer token")
+	w = httptest.NewRecorder()
+	h.ServeHTTP(w, req)
+	if w.Code != http.StatusRequestEntityTooLarge {
+		t.Fatalf("oversized body: %d", w.Code)
+	}
+}

# internal/api/workspace_api_test.go

diff --git a/internal/api/workspace_api_test.go b/internal/api/workspace_api_test.go
index 66dfc51..8c8ad28 100644
--- a/internal/api/workspace_api_test.go
+++ b/internal/api/workspace_api_test.go
@@ -19,7 +19,7 @@ import (
 )
 
 type workspaceMemoryStore struct {
-	items map[uuid.UUID]*model.Workspace
+	items     map[uuid.UUID]*model.Workspace
 	deleteErr error
 }
 
@@ -54,7 +54,9 @@ func (s *workspaceMemoryStore) Update(ctx context.Context, id uuid.UUID, changes
 	return item, nil
 }
 func (s *workspaceMemoryStore) Delete(_ context.Context, id uuid.UUID) error {
-	if s.deleteErr != nil { return s.deleteErr }
+	if s.deleteErr != nil {
+		return s.deleteErr
+	}
 	if s.items[id] == nil {
 		return repository.ErrNotFound
 	}
@@ -76,6 +78,9 @@ func TestWorkspaceAPI(t *testing.T) {
 	request := func(method, path, body, token string) *httptest.ResponseRecorder {
 		t.Helper()
 		req := httptest.NewRequest(method, path, strings.NewReader(body))
+		if body != "" {
+			req.Header.Set("Content-Type", "application/json")
+		}
 		if token != "" {
 			req.Header.Set("Authorization", "Bearer "+token)
 		}
@@ -95,6 +100,7 @@ func TestWorkspaceAPI(t *testing.T) {
 		{http.MethodPost, "", `{"name":"x","root_path":"/tmp","extra":true}`},
 		{http.MethodGet, "?limit=0", ""},
 		{http.MethodGet, "?offset=-1", ""},
+		{http.MethodGet, "?offset=1000001", ""},
 		{http.MethodGet, "/invalid", ""},
 		{http.MethodPatch, "/" + uuid.NewString(), `{}`},
 		{http.MethodPatch, "/" + uuid.NewString(), `{"id":"bad"}`},
@@ -104,6 +110,13 @@ func TestWorkspaceAPI(t *testing.T) {
 			t.Fatalf("%s %s: %d %s", tc.method, tc.suffix, w.Code, w.Body.String())
 		}
 	}
+	missingType := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"name":"x","root_path":"/tmp"}`))
+	missingType.Header.Set("Authorization", "Bearer test-token")
+	missingTypeResponse := httptest.NewRecorder()
+	api.ServeHTTP(missingTypeResponse, missingType)
+	if missingTypeResponse.Code != http.StatusBadRequest {
+		t.Fatalf("missing JSON content type: %d", missingTypeResponse.Code)
+	}
 	root := t.TempDir()
 	createBody, _ := json.Marshal(map[string]string{"name": " project ", "description": "notes", "root_path": root})
 	created := request(http.MethodPost, path, string(createBody), "test-token")

# internal/app/app.go

diff --git a/internal/app/app.go b/internal/app/app.go
index 8f11185..3b0b6af 100644
--- a/internal/app/app.go
+++ b/internal/app/app.go
@@ -15,13 +15,19 @@ import (
 	"github.com/prasannashrestha011/devOrbit/internal/api"
 	"github.com/prasannashrestha011/devOrbit/internal/config"
 	"github.com/prasannashrestha011/devOrbit/internal/database"
+	"github.com/prasannashrestha011/devOrbit/internal/events"
+	"github.com/prasannashrestha011/devOrbit/internal/logs"
 	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
+	"github.com/prasannashrestha011/devOrbit/internal/process"
 	"github.com/prasannashrestha011/devOrbit/internal/routes"
 	"github.com/prasannashrestha011/devOrbit/internal/security"
 )
 
 func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
+	if ip := net.ParseIP(cfg.Host); ip == nil || !ip.IsLoopback() || cfg.Port < 1 || cfg.Port > 65535 {
+		return errors.New("API listen address must be a loopback IP and valid port")
+	}
 	for _, dir := range []string{cfg.DataDir, cfg.LogDir} {
 		if err := os.MkdirAll(dir, 0700); err != nil {
 			return fmt.Errorf("initialize application directory: %w", err)
@@ -46,9 +52,19 @@ func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
 		return err
 	}
 	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
+	broker := events.New()
+	defer broker.Close()
+	stopEvents := context.AfterFunc(ctx, broker.Close)
+	defer stopEvents()
+	manager, err := logs.New(cfg.LogDir, broker)
+	if err != nil {
+		return fmt.Errorf("initialize process logs: %w", err)
+	}
+	supervisor := process.New(cfg.ShutdownTimeout, manager)
+	supervisor.SetBroker(broker)
 	server := &http.Server{
 		Addr:              address,
-		Handler:           api.Router(log, token, "http://"+address, sqlDB.PingContext, func(r chi.Router) { routes.Register(r, db) }),
+		Handler:           localHost(address, api.Router(log, token, "http://"+address, sqlDB.PingContext, func(r chi.Router) { routes.Register(r, db, supervisor, manager, broker) })),
 		ReadHeaderTimeout: 5 * time.Second,
 		ReadTimeout:       10 * time.Second,
 		WriteTimeout:      15 * time.Second,
@@ -57,10 +73,28 @@ func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
 	}
 	listener, err := net.Listen("tcp", address)
 	if err != nil {
+		_ = manager.Close()
 		return fmt.Errorf("listen on %s: %w", address, err)
 	}
 	log.Info("DevOrbit API listening", "address", address)
-	return serve(ctx, server, listener, cfg.ShutdownTimeout)
+	serveErr := serve(ctx, server, listener, cfg.ShutdownTimeout)
+	shutdownCtx, stop := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
+	defer stop()
+	processErr := supervisor.Shutdown(shutdownCtx)
+	logErr := manager.Close()
+	return errors.Join(serveErr, processErr, logErr)
+}
+
+func localHost(address string, next http.Handler) http.Handler {
+	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
+		if r.Host != address {
+			w.Header().Set("Content-Type", "application/json; charset=utf-8")
+			w.WriteHeader(http.StatusForbidden)
+			_, _ = w.Write([]byte(`{"error":{"code":"host_forbidden","message":"host is not allowed"}}`))
+			return
+		}
+		next.ServeHTTP(w, r)
+	})
 }
 
 func serve(ctx context.Context, server *http.Server, listener net.Listener, timeout time.Duration) error {

# internal/app/app_test.go

diff --git a/internal/app/app_test.go b/internal/app/app_test.go
index 75b6cea..ca93f67 100644
--- a/internal/app/app_test.go
+++ b/internal/app/app_test.go
@@ -4,6 +4,7 @@ import (
 	"context"
 	"net"
 	"net/http"
+	"net/http/httptest"
 	"testing"
 	"time"
 )
@@ -37,3 +38,28 @@ func TestServeGracefulShutdown(t *testing.T) {
 		t.Fatal("server did not stop")
 	}
 }
+
+func TestLocalHostRejectsRebinding(t *testing.T) {
+	h := localHost("127.0.0.1:7456", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
+		w.WriteHeader(http.StatusNoContent)
+	}))
+	for _, tc := range []struct {
+		host   string
+		status int
+	}{
+		{"127.0.0.1:7456", http.StatusNoContent},
+		{"evil.example:7456", http.StatusForbidden},
+		{"127.0.0.1:9999", http.StatusForbidden},
+		{"[::1]:7456", http.StatusForbidden},
+	} {
+		req, err := http.NewRequest(http.MethodGet, "http://"+tc.host+"/health", nil)
+		if err != nil {
+			t.Fatal(err)
+		}
+		w := httptest.NewRecorder()
+		h.ServeHTTP(w, req)
+		if w.Code != tc.status {
+			t.Fatalf("host %q: got %d", tc.host, w.Code)
+		}
+	}
+}

# internal/modules/serviceconfiguration/controller/controller.go

diff --git a/internal/modules/serviceconfiguration/controller/controller.go b/internal/modules/serviceconfiguration/controller/controller.go
index 53b0ac1..bc24c0f 100644
--- a/internal/modules/serviceconfiguration/controller/controller.go
+++ b/internal/modules/serviceconfiguration/controller/controller.go
@@ -1,15 +1,14 @@
 package controller
 
 import (
-	"bytes"
 	"encoding/json"
 	"errors"
-	"io"
 	"net/http"
-	"strconv"
 
 	"github.com/go-chi/chi/v5"
 	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/httpinput"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/dto"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
 )
 
@@ -22,54 +21,17 @@ func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
 	if !ok {
 		return
 	}
-	fields, ok := readFields(w, r)
-	if !ok {
+	input, err := dto.DecodeCreate(r)
+	if err != nil {
+		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
 		return
 	}
-	var input service.CreateInput
-	for field, raw := range fields {
-		switch field {
-		case "name":
-			if !readValue(w, raw, &input.Name) {
-				return
-			}
-		case "command":
-			if !readValue(w, raw, &input.Command) {
-				return
-			}
-		case "args":
-			if !readValue(w, raw, &input.Args) {
-				return
-			}
-		case "working_directory":
-			if !readValue(w, raw, &input.WorkingDirectory) {
-				return
-			}
-		case "env":
-			if !readValue(w, raw, &input.Env) {
-				return
-			}
-		case "enabled":
-			input.Enabled = new(bool)
-			if !readValue(w, raw, input.Enabled) {
-				return
-			}
-		case "auto_restart":
-			input.AutoRestart = new(bool)
-			if !readValue(w, raw, input.AutoRestart) {
-				return
-			}
-		default:
-			writeError(w, http.StatusBadRequest, "invalid_request", "unsupported service field")
-			return
-		}
-	}
 	item, err := c.service.Create(r.Context(), workspaceID, input)
 	if err != nil {
 		writeOperationError(w, err)
 		return
 	}
-	writeJSON(w, http.StatusCreated, item)
+	writeJSON(w, http.StatusCreated, dto.FromModel(item))
 }
 
 func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
@@ -77,8 +39,9 @@ func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
 	if !ok {
 		return
 	}
-	limit, offset, ok := pagination(w, r)
-	if !ok {
+	limit, offset, err := httpinput.Pagination(r.URL.Query())
+	if err != nil {
+		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
 		return
 	}
 	items, err := c.service.List(r.Context(), workspaceID, limit, offset)
@@ -86,7 +49,7 @@ func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
 		writeOperationError(w, err)
 		return
 	}
-	writeJSON(w, http.StatusOK, items)
+	writeJSON(w, http.StatusOK, dto.FromModels(items))
 }
 
 func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
@@ -99,7 +62,7 @@ func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
 		writeOperationError(w, err)
 		return
 	}
-	writeJSON(w, http.StatusOK, item)
+	writeJSON(w, http.StatusOK, dto.FromModel(item))
 }
 
 func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
@@ -107,59 +70,17 @@ func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
 	if !ok {
 		return
 	}
-	fields, ok := readFields(w, r)
-	if !ok {
+	input, err := dto.DecodePatch(r)
+	if err != nil {
+		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
 		return
 	}
-	var input service.PatchInput
-	for field, raw := range fields {
-		switch field {
-		case "name":
-			input.Name = new(string)
-			if !readValue(w, raw, input.Name) {
-				return
-			}
-		case "command":
-			input.Command = new(string)
-			if !readValue(w, raw, input.Command) {
-				return
-			}
-		case "args":
-			input.Args = new([]string)
-			if !readValue(w, raw, input.Args) {
-				return
-			}
-		case "working_directory":
-			input.WorkingDirectory = new(string)
-			if !readValue(w, raw, input.WorkingDirectory) {
-				return
-			}
-		case "env":
-			input.Env = new(map[string]string)
-			if !readValue(w, raw, input.Env) {
-				return
-			}
-		case "enabled":
-			input.Enabled = new(bool)
-			if !readValue(w, raw, input.Enabled) {
-				return
-			}
-		case "auto_restart":
-			input.AutoRestart = new(bool)
-			if !readValue(w, raw, input.AutoRestart) {
-				return
-			}
-		default:
-			writeError(w, http.StatusBadRequest, "invalid_request", "unsupported service field")
-			return
-		}
-	}
 	item, err := c.service.Update(r.Context(), id, input)
 	if err != nil {
 		writeOperationError(w, err)
 		return
 	}
-	writeJSON(w, http.StatusOK, item)
+	writeJSON(w, http.StatusOK, dto.FromModel(item))
 }
 
 func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
@@ -183,51 +104,6 @@ func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
 	return id, true
 }
 
-func readFields(w http.ResponseWriter, r *http.Request) (map[string]json.RawMessage, bool) {
-	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
-	var fields map[string]json.RawMessage
-	if err := decoder.Decode(&fields); err != nil || fields == nil {
-		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
-		return nil, false
-	}
-	var trailing any
-	if err := decoder.Decode(&trailing); err != io.EOF {
-		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
-		return nil, false
-	}
-	return fields, true
-}
-
-func readValue(w http.ResponseWriter, raw json.RawMessage, target any) bool {
-	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, target) != nil {
-		writeError(w, http.StatusBadRequest, "invalid_request", "invalid service field value")
-		return false
-	}
-	return true
-}
-
-func pagination(w http.ResponseWriter, r *http.Request) (int, int, bool) {
-	limit, offset := 50, 0
-	values := r.URL.Query()
-	if values.Has("limit") {
-		parsed, err := strconv.Atoi(values.Get("limit"))
-		if err != nil || parsed < 1 || parsed > 100 {
-			writeError(w, http.StatusBadRequest, "invalid_request", "limit must be between 1 and 100")
-			return 0, 0, false
-		}
-		limit = parsed
-	}
-	if values.Has("offset") {
-		parsed, err := strconv.Atoi(values.Get("offset"))
-		if err != nil || parsed < 0 {
-			writeError(w, http.StatusBadRequest, "invalid_request", "offset must be nonnegative")
-			return 0, 0, false
-		}
-		offset = parsed
-	}
-	return limit, offset, true
-}
-
 func writeOperationError(w http.ResponseWriter, err error) {
 	var validation *service.ValidationError
 	switch {

# internal/modules/serviceconfiguration/controller/controller_test.go

diff --git a/internal/modules/serviceconfiguration/controller/controller_test.go b/internal/modules/serviceconfiguration/controller/controller_test.go
index 7f4c87e..e5a7561 100644
--- a/internal/modules/serviceconfiguration/controller/controller_test.go
+++ b/internal/modules/serviceconfiguration/controller/controller_test.go
@@ -100,6 +100,9 @@ func TestServiceAPI(t *testing.T) {
 	request := func(method, path, body, token string) *httptest.ResponseRecorder {
 		t.Helper()
 		req := httptest.NewRequest(method, path, strings.NewReader(body))
+		if body != "" {
+			req.Header.Set("Content-Type", "application/json")
+		}
 		if token != "" {
 			req.Header.Set("Authorization", "Bearer "+token)
 		}
@@ -126,6 +129,7 @@ func TestServiceAPI(t *testing.T) {
 		{"POST", path, `{"name":"x","command":"go","unknown":1}`, 400},
 		{"GET", path + "?limit=0", "", 400},
 		{"GET", path + "?offset=-1", "", 400},
+		{"GET", path + "?offset=1000001", "", 400},
 		{"GET", "/api/v1/services/bad", "", 400},
 	} {
 		w := request(tc.method, tc.path, tc.body, "token")

# internal/modules/serviceconfiguration/service/service.go

diff --git a/internal/modules/serviceconfiguration/service/service.go b/internal/modules/serviceconfiguration/service/service.go
index 693a38e..516dfb5 100644
--- a/internal/modules/serviceconfiguration/service/service.go
+++ b/internal/modules/serviceconfiguration/service/service.go
@@ -4,15 +4,12 @@ import (
 	"context"
 	"encoding/json"
 	"errors"
-	"fmt"
-	"os"
-	"path/filepath"
-	"strings"
-	"unicode/utf8"
 
 	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/dto"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/validation"
 	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
 	workspaceRepository "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
 )
@@ -39,51 +36,31 @@ func New(store Store, workspaces WorkspaceStore) *Service {
 	return &Service{store: store, workspaces: workspaces}
 }
 
-type ValidationError struct{ Field, Message string }
-
-func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
-
-type CreateInput struct {
-	Name             string
-	Command          string
-	Args             []string
-	WorkingDirectory string
-	Env              map[string]string
-	Enabled          *bool
-	AutoRestart      *bool
-}
-
-type PatchInput struct {
-	Name             *string
-	Command          *string
-	Args             *[]string
-	WorkingDirectory *string
-	Env              *map[string]string
-	Enabled          *bool
-	AutoRestart      *bool
-}
+type ValidationError = validation.Error
+type CreateInput = dto.CreateInput
+type PatchInput = dto.PatchInput
 
 func (s *Service) Create(ctx context.Context, workspaceID uuid.UUID, input CreateInput) (*model.Service, error) {
 	workspace, err := s.workspaces.Get(ctx, workspaceID)
 	if err != nil {
 		return nil, err
 	}
-	name, err := validateName(input.Name)
+	name, err := validation.Name(input.Name)
 	if err != nil {
 		return nil, err
 	}
-	command, err := validateCommand(input.Command)
+	command, err := validation.Command(input.Command)
 	if err != nil {
 		return nil, err
 	}
-	workingDir, err := normalizeDirectory(workspace.RootPath, input.WorkingDirectory)
+	workingDir, err := validation.Directory(workspace.RootPath, input.WorkingDirectory)
 	if err != nil {
 		return nil, err
 	}
-	if err := validateArgs(input.Args); err != nil {
+	if err := validation.Args(input.Args); err != nil {
 		return nil, err
 	}
-	if err := validateEnv(input.Env); err != nil {
+	if err := validation.Env(input.Env); err != nil {
 		return nil, err
 	}
 	exists, err := s.store.NameExists(ctx, workspaceID, name, uuid.Nil)
@@ -142,7 +119,7 @@ func (s *Service) Update(ctx context.Context, id uuid.UUID, input PatchInput) (*
 	}
 	changes := make(map[string]any)
 	if input.Name != nil {
-		name, err := validateName(*input.Name)
+		name, err := validation.Name(*input.Name)
 		if err != nil {
 			return nil, err
 		}
@@ -156,7 +133,7 @@ func (s *Service) Update(ctx context.Context, id uuid.UUID, input PatchInput) (*
 		changes["name"] = name
 	}
 	if input.Command != nil {
-		command, err := validateCommand(*input.Command)
+		command, err := validation.Command(*input.Command)
 		if err != nil {
 			return nil, err
 		}
@@ -167,14 +144,14 @@ func (s *Service) Update(ctx context.Context, id uuid.UUID, input PatchInput) (*
 		if err != nil {
 			return nil, err
 		}
-		dir, err := normalizeDirectory(workspace.RootPath, *input.WorkingDirectory)
+		dir, err := validation.Directory(workspace.RootPath, *input.WorkingDirectory)
 		if err != nil {
 			return nil, err
 		}
 		changes["working_directory"] = dir
 	}
 	if input.Args != nil {
-		if err := validateArgs(*input.Args); err != nil {
+		if err := validation.Args(*input.Args); err != nil {
 			return nil, err
 		}
 		args := *input.Args
@@ -185,7 +162,7 @@ func (s *Service) Update(ctx context.Context, id uuid.UUID, input PatchInput) (*
 		changes["args"] = string(encoded)
 	}
 	if input.Env != nil {
-		if err := validateEnv(*input.Env); err != nil {
+		if err := validation.Env(*input.Env); err != nil {
 			return nil, err
 		}
 		env := *input.Env
@@ -209,71 +186,6 @@ func (s *Service) Update(ctx context.Context, id uuid.UUID, input PatchInput) (*
 
 func (s *Service) Delete(ctx context.Context, id uuid.UUID) error { return s.store.Delete(ctx, id) }
 
-func validateName(name string) (string, error) {
-	name = strings.TrimSpace(name)
-	if name == "" || utf8.RuneCountInString(name) > 120 {
-		return "", &ValidationError{Field: "name", Message: "must contain 1 to 120 characters"}
-	}
-	return name, nil
-}
-
-func validateCommand(command string) (string, error) {
-	command = strings.TrimSpace(command)
-	if command == "" || strings.ContainsRune(command, 0) {
-		return "", &ValidationError{Field: "command", Message: "must be a nonempty executable without null bytes"}
-	}
-	return command, nil
-}
-
-func validateArgs(args []string) error {
-	for _, arg := range args {
-		if strings.ContainsRune(arg, 0) {
-			return &ValidationError{Field: "args", Message: "must not contain null bytes"}
-		}
-	}
-	return nil
-}
-
-func validateEnv(env map[string]string) error {
-	for key, value := range env {
-		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, 0) {
-			return &ValidationError{Field: "env", Message: "variable names must be nonempty and cannot contain '=' or null bytes; values cannot contain null bytes"}
-		}
-	}
-	return nil
-}
-
-func normalizeDirectory(root, directory string) (string, error) {
-	if strings.ContainsRune(directory, 0) {
-		return "", &ValidationError{Field: "working_directory", Message: "invalid directory path"}
-	}
-	absolute := filepath.IsAbs(directory)
-	path := directory
-	if !absolute {
-		path = filepath.Join(root, directory)
-	}
-	canonical, err := filepath.EvalSymlinks(path)
-	if err != nil {
-		return "", &ValidationError{Field: "working_directory", Message: "must be an accessible existing directory"}
-	}
-	info, err := os.Stat(canonical)
-	if err != nil || !info.IsDir() {
-		return "", &ValidationError{Field: "working_directory", Message: "must be an accessible existing directory"}
-	}
-	canonical = filepath.Clean(canonical)
-	relative, err := filepath.Rel(root, canonical)
-	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
-		if relative == "." {
-			return "", nil
-		}
-		return relative, nil
-	}
-	if !absolute {
-		return "", &ValidationError{Field: "working_directory", Message: "relative directory must remain inside the workspace"}
-	}
-	return canonical, nil
-}
-
 func IsNotFound(err error) bool {
 	return errors.Is(err, repository.ErrNotFound) || errors.Is(err, workspaceRepository.ErrNotFound) || errors.Is(err, repository.ErrWorkspaceNotFound)
 }

# internal/modules/workspace/controller/controller.go

diff --git a/internal/modules/workspace/controller/controller.go b/internal/modules/workspace/controller/controller.go
index a919821..bd671e1 100644
--- a/internal/modules/workspace/controller/controller.go
+++ b/internal/modules/workspace/controller/controller.go
@@ -1,15 +1,14 @@
 package controller
 
 import (
-	"bytes"
 	"encoding/json"
 	"errors"
-	"io"
 	"net/http"
-	"strconv"
 
 	"github.com/go-chi/chi/v5"
 	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/httpinput"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/dto"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
 )
@@ -19,48 +18,31 @@ type Controller struct{ service *service.Service }
 func New(s *service.Service) *Controller { return &Controller{service: s} }
 
 func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
-	var input struct {
-		Name        string `json:"name"`
-		Description string `json:"description"`
-		RootPath    string `json:"root_path"`
-	}
-	if err := decode(r, &input); err != nil {
+	input, err := dto.DecodeCreate(r)
+	if err != nil {
 		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
 		return
 	}
-	workspace, err := c.service.Create(r.Context(), service.CreateInput(input))
+	workspace, err := c.service.Create(r.Context(), input)
 	if err != nil {
 		writeOperationError(w, err)
 		return
 	}
-	writeJSON(w, http.StatusCreated, workspace)
+	writeJSON(w, http.StatusCreated, dto.FromModel(workspace))
 }
 
 func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
-	limit, offset := 50, 0
-	values := r.URL.Query()
-	if values.Has("limit") {
-		parsed, err := strconv.Atoi(values.Get("limit"))
-		if err != nil || parsed < 1 || parsed > 100 {
-			writeError(w, http.StatusBadRequest, "invalid_request", "limit must be between 1 and 100")
-			return
-		}
-		limit = parsed
-	}
-	if values.Has("offset") {
-		parsed, err := strconv.Atoi(values.Get("offset"))
-		if err != nil || parsed < 0 {
-			writeError(w, http.StatusBadRequest, "invalid_request", "offset must be nonnegative")
-			return
-		}
-		offset = parsed
+	limit, offset, err := httpinput.Pagination(r.URL.Query())
+	if err != nil {
+		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
+		return
 	}
 	workspaces, err := c.service.List(r.Context(), limit, offset)
 	if err != nil {
 		writeOperationError(w, err)
 		return
 	}
-	writeJSON(w, http.StatusOK, workspaces)
+	writeJSON(w, http.StatusOK, dto.FromModels(workspaces))
 }
 
 func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
@@ -73,7 +55,7 @@ func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
 		writeOperationError(w, err)
 		return
 	}
-	writeJSON(w, http.StatusOK, workspace)
+	writeJSON(w, http.StatusOK, dto.FromModel(workspace))
 }
 
 func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
@@ -81,40 +63,17 @@ func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
 	if !ok {
 		return
 	}
-	var fields map[string]json.RawMessage
-	if err := decode(r, &fields); err != nil || fields == nil {
-		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
+	input, err := dto.DecodePatch(r)
+	if err != nil {
+		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
 		return
 	}
-	var input service.PatchInput
-	for field, raw := range fields {
-		if bytes.Equal(raw, []byte("null")) {
-			writeError(w, http.StatusBadRequest, "invalid_request", "null field value is not supported")
-			return
-		}
-		var value string
-		if err := json.Unmarshal(raw, &value); err != nil {
-			writeError(w, http.StatusBadRequest, "invalid_request", "fields must be strings")
-			return
-		}
-		switch field {
-		case "name":
-			input.Name = &value
-		case "description":
-			input.Description = &value
-		case "root_path":
-			input.RootPath = &value
-		default:
-			writeError(w, http.StatusBadRequest, "invalid_request", "unsupported workspace field")
-			return
-		}
-	}
 	workspace, err := c.service.Update(r.Context(), id, input)
 	if err != nil {
 		writeOperationError(w, err)
 		return
 	}
-	writeJSON(w, http.StatusOK, workspace)
+	writeJSON(w, http.StatusOK, dto.FromModel(workspace))
 }
 
 func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
@@ -138,19 +97,6 @@ func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
 	return id, true
 }
 
-func decode(r *http.Request, target any) error {
-	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
-	decoder.DisallowUnknownFields()
-	if err := decoder.Decode(target); err != nil {
-		return err
-	}
-	var trailing any
-	if err := decoder.Decode(&trailing); err != io.EOF {
-		return errors.New("trailing JSON value")
-	}
-	return nil
-}
-
 func writeOperationError(w http.ResponseWriter, err error) {
 	var validation *service.ValidationError
 	if errors.As(err, &validation) {

# internal/modules/workspace/routes/routes.go

diff --git a/internal/modules/workspace/routes/routes.go b/internal/modules/workspace/routes/routes.go
index 5000370..5cbe797 100644
--- a/internal/modules/workspace/routes/routes.go
+++ b/internal/modules/workspace/routes/routes.go
@@ -2,17 +2,29 @@ package routes
 
 import (
 	"github.com/go-chi/chi/v5"
+	processService "github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
+	configRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
+	configService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/controller"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
+	"github.com/prasannashrestha011/devOrbit/internal/orchestration"
+	"github.com/prasannashrestha011/devOrbit/internal/process"
 	"gorm.io/gorm"
 )
 
-func RegisterRoutes(r chi.Router, db *gorm.DB) {
-	c := controller.New(service.New(repository.New(db)))
+func RegisterRoutes(r chi.Router, db *gorm.DB, supervisor *process.Supervisor) {
+	workspaces := repository.New(db)
+	services := configRepo.New(db)
+	c := controller.New(service.New(workspaces))
+	lifecycle := processService.New(configService.New(services, workspaces), service.New(workspaces), supervisor)
+	o := controller.NewOrchestration(orchestration.New(workspaces, services, lifecycle, supervisor))
 	r.Post("/workspaces", c.Create)
 	r.Get("/workspaces", c.List)
 	r.Get("/workspaces/{id}", c.Get)
 	r.Patch("/workspaces/{id}", c.Update)
 	r.Delete("/workspaces/{id}", c.Delete)
+	r.Post("/workspaces/{id}/start", o.Start)
+	r.Post("/workspaces/{id}/stop", o.Stop)
+	r.Get("/workspaces/{id}/status", o.Status)
 }

# internal/modules/workspace/service/service.go

diff --git a/internal/modules/workspace/service/service.go b/internal/modules/workspace/service/service.go
index a40bc17..7fe951a 100644
--- a/internal/modules/workspace/service/service.go
+++ b/internal/modules/workspace/service/service.go
@@ -3,15 +3,12 @@ package service
 import (
 	"context"
 	"errors"
-	"fmt"
-	"os"
-	"path/filepath"
-	"strings"
-	"unicode/utf8"
 
 	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/dto"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/validation"
 )
 
 type Store interface {
@@ -30,34 +27,19 @@ func New(store Store) *Service {
 	return &Service{store: store}
 }
 
-type ValidationError struct {
-	Field   string
-	Message string
-}
-
-func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
-
-type CreateInput struct {
-	Name        string
-	Description string
-	RootPath    string
-}
-
-type PatchInput struct {
-	Name        *string
-	Description *string
-	RootPath    *string
-}
+type ValidationError = validation.Error
+type CreateInput = dto.CreateInput
+type PatchInput = dto.PatchInput
 
 func (s *Service) Create(ctx context.Context, input CreateInput) (*model.Workspace, error) {
-	name, err := validateName(input.Name)
+	name, err := validation.Name(input.Name)
 	if err != nil {
 		return nil, err
 	}
-	if err := validateDescription(input.Description); err != nil {
+	if err := validation.Description(input.Description); err != nil {
 		return nil, err
 	}
-	root, err := normalizeRoot(input.RootPath)
+	root, err := validation.Root(input.RootPath)
 	if err != nil {
 		return nil, err
 	}
@@ -84,20 +66,20 @@ func (s *Service) List(ctx context.Context, limit, offset int) ([]*model.Workspa
 func (s *Service) Update(ctx context.Context, id uuid.UUID, input PatchInput) (*model.Workspace, error) {
 	changes := make(map[string]any)
 	if input.Name != nil {
-		name, err := validateName(*input.Name)
+		name, err := validation.Name(*input.Name)
 		if err != nil {
 			return nil, err
 		}
 		changes["name"] = name
 	}
 	if input.Description != nil {
-		if err := validateDescription(*input.Description); err != nil {
+		if err := validation.Description(*input.Description); err != nil {
 			return nil, err
 		}
 		changes["description"] = *input.Description
 	}
 	if input.RootPath != nil {
-		root, err := normalizeRoot(*input.RootPath)
+		root, err := validation.Root(*input.RootPath)
 		if err != nil {
 			return nil, err
 		}
@@ -113,38 +95,4 @@ func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
 	return s.store.Delete(ctx, id)
 }
 
-func validateName(name string) (string, error) {
-	name = strings.TrimSpace(name)
-	if name == "" || utf8.RuneCountInString(name) > 120 {
-		return "", &ValidationError{Field: "name", Message: "must contain 1 to 120 characters"}
-	}
-	return name, nil
-}
-
-func validateDescription(description string) error {
-	if utf8.RuneCountInString(description) > 1000 {
-		return &ValidationError{Field: "description", Message: "must contain at most 1000 characters"}
-	}
-	return nil
-}
-
-func normalizeRoot(root string) (string, error) {
-	if strings.TrimSpace(root) == "" {
-		return "", &ValidationError{Field: "root_path", Message: "must be an existing directory"}
-	}
-	absolute, err := filepath.Abs(root)
-	if err != nil {
-		return "", &ValidationError{Field: "root_path", Message: "invalid directory path"}
-	}
-	canonical, err := filepath.EvalSymlinks(absolute)
-	if err != nil {
-		return "", &ValidationError{Field: "root_path", Message: "must be an accessible existing directory"}
-	}
-	info, err := os.Stat(canonical)
-	if err != nil || !info.IsDir() {
-		return "", &ValidationError{Field: "root_path", Message: "must be an accessible existing directory"}
-	}
-	return filepath.Clean(canonical), nil
-}
-
 func IsNotFound(err error) bool { return errors.Is(err, repository.ErrNotFound) }

# internal/modules/workspace/service/service_test.go

diff --git a/internal/modules/workspace/service/service_test.go b/internal/modules/workspace/service/service_test.go
index 6d3a31e..173bb7e 100644
--- a/internal/modules/workspace/service/service_test.go
+++ b/internal/modules/workspace/service/service_test.go
@@ -9,6 +9,7 @@ import (
 
 	"github.com/google/uuid"
 	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/validation"
 )
 
 type captureStore struct{ workspace *model.Workspace }
@@ -93,7 +94,7 @@ func TestRelativeRootPath(t *testing.T) {
 	if err != nil {
 		t.Fatal(err)
 	}
-	actual, err := normalizeRoot(relative)
+	actual, err := validation.Root(relative)
 	if err != nil {
 		t.Fatal(err)
 	}

# internal/routes/routes.go

diff --git a/internal/routes/routes.go b/internal/routes/routes.go
index dcda5b0..f6c06e1 100644
--- a/internal/routes/routes.go
+++ b/internal/routes/routes.go
@@ -2,13 +2,22 @@ package routes
 
 import (
 	"github.com/go-chi/chi/v5"
+	"github.com/prasannashrestha011/devOrbit/internal/events"
+	"github.com/prasannashrestha011/devOrbit/internal/logs"
+	eventRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/events/routes"
+	logRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/logs/routes"
+	processRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/process/routes"
 	serviceRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/routes"
 	workspaceRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/routes"
+	"github.com/prasannashrestha011/devOrbit/internal/process"
 	"gorm.io/gorm"
 )
 
 // Register passes the authenticated API router to each module.
-func Register(r chi.Router, db *gorm.DB) {
-	workspaceRoutes.RegisterRoutes(r, db)
+func Register(r chi.Router, db *gorm.DB, supervisor *process.Supervisor, manager *logs.Manager, broker *events.Broker) {
+	workspaceRoutes.RegisterRoutes(r, db, supervisor)
 	serviceRoutes.RegisterRoutes(r, db)
+	processRoutes.RegisterRoutes(r, db, supervisor)
+	logRoutes.RegisterRoutes(r, db, manager)
+	eventRoutes.RegisterRoutes(r, db, broker)
 }

# internal/security/token.go

diff --git a/internal/security/token.go b/internal/security/token.go
index 0f635d3..40d59c7 100644
--- a/internal/security/token.go
+++ b/internal/security/token.go
@@ -7,25 +7,23 @@ import (
 	"io"
 	"os"
 	"path/filepath"
-	"runtime"
 )
 
+func PrivateDirectory(path string) error { return privateDirectory(path) }
+
 func LoadOrCreateToken(path string) (string, error) {
 	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
 		return "", errors.New("cannot create token directory")
 	}
-	if runtime.GOOS != "windows" {
-		info, err := os.Stat(filepath.Dir(path))
-		if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
-			return "", errors.New("token directory must be private")
-		}
+	if err := privateDirectory(filepath.Dir(path)); err != nil {
+		return "", errors.New("token directory must be private")
 	}
 	var bytes [32]byte
 	if _, err := rand.Read(bytes[:]); err != nil {
 		return "", errors.New("cannot generate authentication token")
 	}
 	token := hex.EncodeToString(bytes[:])
-	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
+	f, err := createToken(path)
 	if err == nil {
 		if _, err = io.WriteString(f, token); err != nil {
 			_ = f.Close()
@@ -41,12 +39,13 @@ func LoadOrCreateToken(path string) (string, error) {
 	if !errors.Is(err, os.ErrExist) {
 		return "", errors.New("cannot create authentication token")
 	}
-	info, err := os.Lstat(path)
-	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
+	f, err = openToken(path)
+	if err != nil {
 		return "", errors.New("authentication token file must be private and regular")
 	}
-	stored, err := os.ReadFile(path)
-	if err != nil || len(stored) != 64 {
+	stored, err := io.ReadAll(io.LimitReader(f, 65))
+	closeErr := f.Close()
+	if err != nil || closeErr != nil || len(stored) != 64 {
 		return "", errors.New("cannot read valid authentication token")
 	}
 	if _, err := hex.DecodeString(string(stored)); err != nil {

# internal/security/token_test.go

diff --git a/internal/security/token_test.go b/internal/security/token_test.go
index aa95895..44ac871 100644
--- a/internal/security/token_test.go
+++ b/internal/security/token_test.go
@@ -38,3 +38,52 @@ func TestRejectsLooseTokenPermissions(t *testing.T) {
 		t.Fatal("expected loose token permissions to be rejected")
 	}
 }
+
+func TestRejectsTokenSymlinks(t *testing.T) {
+	if runtime.GOOS == "windows" {
+		t.Skip("Windows link permissions vary by host")
+	}
+	dir := t.TempDir()
+	private := filepath.Join(dir, "private")
+	if err := os.Mkdir(private, 0700); err != nil {
+		t.Fatal(err)
+	}
+	elsewhere := filepath.Join(dir, "elsewhere")
+	if err := os.WriteFile(elsewhere, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), 0600); err != nil {
+		t.Fatal(err)
+	}
+	if err := os.Symlink(elsewhere, filepath.Join(private, "token")); err != nil {
+		t.Fatal(err)
+	}
+	if _, err := LoadOrCreateToken(filepath.Join(private, "token")); err == nil {
+		t.Fatal("accepted symlinked token")
+	}
+	link := filepath.Join(dir, "link")
+	if err := os.Symlink(private, link); err != nil {
+		t.Fatal(err)
+	}
+	if _, err := LoadOrCreateToken(filepath.Join(link, "other-token")); err == nil {
+		t.Fatal("accepted symlinked token directory")
+	}
+}
+
+func TestRejectsSharedTokenAncestor(t *testing.T) {
+	if runtime.GOOS == "windows" {
+		t.Skip("Unix directory mode test")
+	}
+	shared, err := os.MkdirTemp("/tmp", "devorbit-shared-")
+	if err != nil {
+		t.Fatal(err)
+	}
+	t.Cleanup(func() { _ = os.RemoveAll(shared) })
+	private := filepath.Join(shared, "private")
+	if err := os.MkdirAll(private, 0700); err != nil {
+		t.Fatal(err)
+	}
+	if err := os.Chmod(shared, 0777); err != nil {
+		t.Fatal(err)
+	}
+	if _, err := LoadOrCreateToken(filepath.Join(private, "token")); err == nil {
+		t.Fatal("accepted token under writable shared ancestor")
+	}
+}

# .agent/log_mgmt.md

diff --git a/.agent/log_mgmt.md b/.agent/log_mgmt.md
new file mode 100644
index 0000000..972107f
--- /dev/null
+++ b/.agent/log_mgmt.md
@@ -0,0 +1,217 @@
+# DevOrbit Backend V1 — Implementation Phase 07
+## Centralized Log Management
+
+### Objective
+
+Implement a centralized logging system that captures, stores, manages, and retrieves stdout and stderr from processes managed by DevOrbit.
+
+Follow the existing DevOrbit Master Project Context.
+
+**Technology Stack:**
+- Go
+- Chi
+- GORM + GORM Gen
+- PostgreSQL 17
+- GORM AutoMigrate
+- Electron + TypeScript (future frontend)
+
+Preserve functionality from Phases 01–06.
+
+**Implement Phase 07 only. Do not implement SSE streaming or future phases.**
+
+---
+
+## 1. Pre-Implementation Inspection
+
+Before making changes:
+
+1. Inspect the existing Process Supervisor.
+2. Review how commands are executed using `os/exec`.
+3. Inspect the process registry and execution lifecycle.
+4. Review Start, Stop, Restart, and workspace orchestration.
+5. Identify how stdout and stderr are currently handled.
+6. Review application initialization and shutdown.
+7. Review existing REST API and authentication conventions.
+8. Read `docs/implementation-progress.md`.
+
+Identify the smallest integration changes necessary.
+
+Do not rewrite working process management code.
+
+The Log Manager must integrate with the existing Process Supervisor rather than creating a separate execution engine.
+
+## 2. Log Manager Architecture
+
+Introduce a dedicated component.
+
+Suggested package:
+
+`internal/logs`
+
+Responsibilities:
+
+- Capture managed-process stdout.
+- Capture managed-process stderr.
+- Attach log metadata.
+- Maintain bounded recent logs in memory.
+- Persist logs to local disk.
+- Rotate log files.
+- Enforce log retention.
+- Retrieve historical logs.
+- Filter logs by service and workspace.
+- Safely handle concurrent writes.
+- Release resources during shutdown.
+
+The Log Manager must be independent of HTTP handlers and GORM.
+
+### Persistence Decision
+
+Store process logs in the local filesystem.
+
+PostgreSQL remains responsible for Workspace and Service configuration.
+
+Do not introduce a database table for every application log entry.
+
+Do not introduce Elasticsearch, Loki, Kafka, Redis, or another external logging service.
+
+Use the Go standard library wherever practical.
+
+## 3. Log Entry Structure
+
+Create a structured log entry containing:
+
+| Field | Description |
+|---|---|
+| ID | Stable identifier or cursor-compatible position |
+| WorkspaceID | Parent workspace UUID |
+| ServiceID | Registered service UUID |
+| ExecutionID | Unique identifier for the process execution |
+| Stream | `stdout` or `stderr` |
+| Timestamp | Time the output was captured |
+| Message | Captured output |
+| Sequence | Sequence number within the execution |
+
+Example:
+
+```json
+{
+  "id": "log-identifier",
+  "workspace_id": "workspace-uuid",
+  "service_id": "service-uuid",
+  "execution_id": "execution-uuid",
+  "stream": "stdout",
+  "timestamp": "2026-10-18T14:00:00Z",
+  "sequence": 42,
+  "message": "HTTP server listening on :8080"
+}
+```
+
+### Requirements
+
+- Generate a new ExecutionID every time a service starts.
+- Preserve the distinction between different executions of the same service.
+- Support logs from concurrently running services.
+- Use UTC timestamps.
+- Preserve message content safely.
+- Do not assume stdout and stderr have a guaranteed global ordering.
+- Preserve ordering according to the Log Manager's ingestion sequence where possible.
+
+Do not mix the identity of an old execution with a restarted process.
+
+## 4. Process Output Capture
+
+Integrate with the existing Process Supervisor.
+
+When a managed process starts:
+
+1. Create an execution identifier.
+2. Initialize log capture.
+3. Attach stdout and stderr destinations before starting the process.
+4. Launch the process using the existing supervisor.
+5. Capture its output while it runs.
+6. Continue capturing until output streams close.
+7. Flush remaining buffered output after process termination.
+8. Release execution-specific logging resources.
+
+### Requirements
+
+- Capture stdout and stderr separately.
+- Capture output continuously.
+- Support multiple concurrent processes.
+- Support short-lived commands.
+- Handle processes producing no output.
+- Handle large output volumes.
+- Handle output without newline characters.
+- Handle partial lines.
+- Handle carriage returns and ANSI control sequences safely.
+- Do not silently truncate long messages without recording that truncation occurred.
+- Avoid goroutine leaks and deadlocks.
+
+### Important Process Lifecycle Requirement
+
+Preserve the existing rule that every successfully started process is reaped exactly once.
+
+If using `StdoutPipe` and `StderrPipe`, coordinate stream readers carefully with `Wait()`.
+
+Alternatively, use concurrency-safe writers when appropriate.
+
+Do not introduce a race where process termination closes output streams before buffered data has been collected.
+
+## 5. Bounded In-Memory Log Buffer
+
+Implement recent-log buffering for each service.
+
+Suggested defaults:
+
+- Maximum 500 recent log entries per service.
+- Maximum 2 MiB buffered log data per service.
+- Maximum 16 KiB per stored log message chunk.
+
+Make these limits configurable where reasonable.
+
+### Requirements
+
+- Use a ring buffer or similarly bounded data structure.
+- Enforce both entry-count and byte-size limits.
+- Remove oldest entries when capacity is exceeded.
+- Preserve ordering of retained entries.
+- Support concurrent readers and writers.
+- Return immutable snapshots.
+- Never let process output consume unlimited memory.
+
+The memory buffer is a recent-history cache, not the authoritative persistent log store.
+
+If a process produces more output than the system can handle, use an explicit bounded overflow policy.
+
+Avoid blocking child-process output indefinitely because disk writes or API consumers are slow.
+
+Record dropped-entry counts or equivalent diagnostics when overload occurs.
+
+## 6. Local Log Persistence
+
+Persist process logs to local disk.
+
+Suggested structure:
+
+```text
+DevOrbit/
+  logs/
+    services/
+      {service-id}/
+        {execution-id}/
+          output-0001.jsonl
+          output-0002.jsonl
+```
+
+Use platform-appropriate application data directories.
+
+The exact file layout may be adjusted based on existing project conventions.
+
+### Requirements
+
+- Use JSON Lines (JSONL) or another documented append-friendly structured format.
+- Ensure each persisted entry includes sufficient metadata.
+- Support concurrent processes.
+- Do not overwrite logs from previous executions.
+- Create directories with restrictive permissions where supported.
+- Close
\ No newline at end of file

# .agent/process_lifecycle_mgmt.md

diff --git a/.agent/process_lifecycle_mgmt.md b/.agent/process_lifecycle_mgmt.md
new file mode 100644
index 0000000..4ff37de
--- /dev/null
+++ b/.agent/process_lifecycle_mgmt.md
@@ -0,0 +1,355 @@
+# DevOrbit Backend V1 — Implementation Phase 05
+## Process Lifecycle Management
+
+### Objective
+
+Extend the existing Process Supervisor to support reliable service stopping, restarting, process-tree termination, and concurrency-safe lifecycle management.
+
+Follow the DevOrbit Master Project Context.
+
+Preserve all working functionality from Phases 01–04.
+
+**Implement Phase 05 only. Do not implement later phases.**
+
+---
+
+## 1. Inspect Existing Implementation
+
+Before modifying code:
+
+1. Review the Process Supervisor from Phase 04.
+2. Understand how processes are started and registered.
+3. Review runtime state transitions and PID tracking.
+4. Review how `exec.Cmd`, `Wait()`, and process exit detection are handled.
+5. Review backend shutdown behavior.
+6. Review existing HTTP endpoints and authentication.
+7. Read `docs/implementation-progress.md`.
+
+Identify lifecycle limitations and make minimal changes to the existing implementation.
+
+Do not rewrite the Process Supervisor unless necessary for correctness.
+
+If Phase 04 is incomplete, report the blockers rather than silently implementing missing earlier phases.
+
+## 2. Process Stop Operation
+
+Implement:
+
+`Supervisor.Stop(ctx, serviceID)`
+
+Responsibilities:
+
+- Locate the currently managed process.
+- Transition the process into `stopping`.
+- Request graceful termination.
+- Wait for process termination.
+- Escalate to forced termination after a configurable timeout.
+- Ensure the managed process and owned children are cleaned up.
+- Record the final state and termination information.
+- Release process-specific resources.
+
+### Rules
+
+- Only terminate processes owned by DevOrbit.
+- Do not rely on a PID alone to establish process ownership.
+- Never terminate unrelated operating-system processes.
+- Do not call `Wait()` from multiple goroutines.
+- Coordinate stopping with the existing exit observer.
+- Do not hold the global supervisor mutex while waiting for process termination.
+- Prevent new starts while a service is stopping.
+- Make repeated Stop calls safe.
+- Treat an already terminated service as an idempotent no-op.
+- Return meaningful errors when termination fails.
+
+Stopping a service must not affect other independently managed services.
+
+## 3. Graceful and Forced Termination
+
+Use a configurable shutdown timeout.
+
+Suggested default:
+
+`10 seconds`
+
+Termination sequence:
+
+1. Mark the service as stopping.
+2. Send an appropriate graceful termination signal.
+3. Wait for the process to exit.
+4. If the deadline expires, force termination of the managed process tree.
+5. Reap the process and release resources.
+6. Update runtime state.
+
+### Unix (macOS/Linux)
+
+Implement appropriate process-group management.
+
+- Create isolated process groups when launching managed processes.
+- Use process-group signaling where appropriate.
+- Prefer a graceful interrupt suitable for local development applications.
+- Escalate to forced termination after timeout.
+- Avoid signaling unrelated process groups.
+- Handle processes that exit before signaling.
+- Account for child processes created by `go run`, `npm`, and similar commands.
+
+### Windows
+
+Implement platform-specific termination handling.
+
+- Use Windows-appropriate process management.
+- Prefer Job Objects for tracking and terminating managed process trees.
+- Where supported, use a graceful console-control mechanism before forced termination.
+- Do not assume Unix signals behave identically on Windows.
+- Avoid blindly relying on `Process.Kill()` when a child process tree exists.
+- Document limitations where graceful shutdown is unavailable.
+
+Use build-tagged, OS-specific files where necessary.
+
+If the existing launcher must be adjusted to establish process-group or Job Object ownership at process creation time, make that focused change.
+
+Do not implement unrelated platform features.
+
+## 4. Process Restart Operation
+
+Implement:
+
+`Supervisor.Restart(ctx, serviceID)`
+
+Expected behavior:
+
+- If running, stop the existing process and its managed descendants before launching a replacement.
+- If exited, failed, or stopped, launch a new process.
+- Prevent overlapping old and new executions.
+- Assign and record the new PID.
+- Update runtime timestamps and state.
+- Propagate meaningful errors when either stopping or starting fails.
+
+Restart must use the latest persisted service configuration when starting the replacement process.
+
+Do not introduce automatic restart policies or background restart loops.
+
+The existing `auto_restart` configuration field remains inactive in V1 Phase 05.
+
+## 5. Lifecycle State Management
+
+Extend the runtime state machine.
+
+Required states:
+
+- `stopped`
+- `starting`
+- `running`
+- `stopping`
+- `exited`
+- `failed`
+
+### State Rules
+
+- `stopped` → `starting` → `running`
+- `running` → `stopping` → `stopped`
+- `running` → `exited` on successful natural exit
+- `running` → `failed` on unexpected unsuccessful exit
+- `exited` / `failed` / `stopped` → `starting` when launched again
+- `starting` → `failed` when process creation fails
+
+Restart must safely transition through termination and a new launch.
+
+Record exit codes when available.
+
+Intentional termination must not be misclassified as an unexpected process crash.
+
+Distinguish process launch failure from process exit failure.
+
+Prevent stale exit callbacks from overwriting the status of a newer process instance.
+
+## 6. Concurrency and Race Condition Handling
+
+Handle simultaneous lifecycle requests safely.
+
+Required scenarios:
+
+- Start + Start
+- Start + Stop
+- Start + Restart
+- Stop + Stop
+- Stop + Restart
+- Restart + Restart
+- Natural exit + Stop
+- Natural exit + Restart
+- Backend shutdown + Start
+- Backend shutdown + Restart
+
+### Requirements
+
+1. Enforce a single active lifecycle operation per service.
+2. Prevent duplicate processes.
+3. Avoid holding global locks during blocking OS operations.
+4. Permit unrelated services to operate concurrently.
+5. Prevent deadlocks and goroutine leaks.
+6. Ensure lifecycle state updates are atomic where required.
+7. Use generation IDs or equivalent mechanisms to prevent stale process events from changing newer runtime state.
+8. Return predictable results for conflicting operations.
+
+Prefer simple synchronization mechanisms over unnecessary concurrency frameworks.
+
+Concurrent lifecycle calls must never lead to uncontrolled duplicate executions.
+
+## 7. REST API
+
+Extend the existing authenticated API.
+
+### Stop Service
+
+`POST /api/v1/services/{id}/stop`
+
+Behavior:
+
+- Validate service UUID.
+- Confirm the registered service exists.
+- Request service termination.
+- Return updated runtime state.
+- Return HTTP 200 after termination completes.
+- Return HTTP 404 for unknown services.
+- Handle conflicting lifecycle operations appropriately.
+- Return an appropriate error if termination cannot be completed safely.
+
+### Restart Service
+
+`POST /api/v1/services/{id}/restart`
+
+Behavior:
+
+- Validate service UUID.
+- Confirm registered service exists.
+- Stop the existing process if necessary.
+- Start the service using its current configuration.
+- Return the new runtime status.
+- Return HTTP 200 when the new OS process launches successfully.
+- Return HTTP 404 for unknown services.
+- Return HTTP 409 for conflicting operations.
+
+### Status Endpoint
+
+Preserve:
+
+`GET /api/v1/services/{id}/status`
+
+Extend the response to support the `stopping` state and updated termination metadata.
+
+Follow existing authentication, request validation, response envelopes, and error handling conventions.
+
+Do not accept arbitrary executable commands through these lifecycle endpoints.
+
+## 8. Backend Shutdown Hardening
+
+Improve the shutdown implementation from Phase 04.
+
+Requirements:
+
+- Reject new Start and Restart operations once shutdown begins.
+- Stop every active managed service.
+- Allow independent services to terminate concurrently.
+- Apply bounded graceful and forced termination.
+- Wait for process reaping and lifecycle cleanup.
+- Avoid abandoning child processes.
+- Release platform-specific handles and resources.
+- Close the supervisor only after cleanup is complete or a clearly reported failure occurs.
+
+Ensure backend shutdown cannot deadlock with concurrent process operations.
+
+Avoid leaving services incorrectly marked as running after termination.
+
+## 9. Testing
+
+Write comprehensive unit and integration tests.
+
+### Stop Tests
+
+- Stop running process
+- Stop already stopped process
+- Stop naturally exited process
+- Stop nonexistent service
+- Graceful termination
+- Forced termination after timeout
+- Verify no managed descendant remains where supported
+
+### Restart Tests
+
+- Restart running service
+- Restart stopped service
+- Restart failed service
+- Restart naturally exited service
+- Ensure new PID belongs to the replacement execution
+- Ensure previous process has terminated before replacement
+- Handle failure during restart
+
+### Concurrency Tests
+
+- Concurrent Stop requests
+- Concurrent Restart requests
+- Start and Stop race
+- Start and Restart race
+- Stop during natural exit
+- Shutdown during active lifecycle operations
+- No duplicate managed processes
+- No deadlocks or goroutine leaks
+
+### Cross-Platform Tests
+
+Test process-tree termination using controlled helper processes that create child processes.
+
+Avoid fragile assumptions about installed shells or external commands.
+
+Separate OS-specific test cases using Go build tags where necessary.
+
+Only claim runtime verification on operating systems actually tested.
+
+### API Tests
+
+- Successful stop
+- Successful restart
+- Invalid UUID
+- Nonexistent service
+- Unauthorized requests
+- Lifecycle conflicts
+- Correct runtime status after termination
+
+### Validation
+
+Run:
+
+```bash
+go test ./...
+go test -race ./...
+go vet ./...
+go build ./cmd/devorbit
+```
+
+Verify applicable cross-platform builds.
+
+Do not claim tests passed unless executed successfully.
+
+## 10. Documentation
+
+Update:
+
+- `README.md`
+- `docs/implementation-progress.md`
+- Existing architecture/API documentation where applicable
+
+Document:
+
+- Stop and Restart endpoints
+- Lifecycle state transitions
+- Graceful and forced termination behavior
+- Process-tree ownership
+- Concurrency rules
+- Backend shutdown behavior
+- macOS, Linux, and Windows differences
+- Known platform limitations
+
+## 11. Out of Scope
+
+Do not implement:
+
+- Automatic restart or
\ No newline at end of file

# .agent/process_supervisor.md

diff --git a/.agent/process_supervisor.md b/.agent/process_supervisor.md
new file mode 100644
index 0000000..fef0b90
--- /dev/null
+++ b/.agent/process_supervisor.md
@@ -0,0 +1,191 @@
+# DevOrbit Backend V1 — Implementation Phase 04
+
+## Objective
+
+Implement the **Process Supervisor Core** for DevOrbit using Go.
+
+This phase introduces real operating-system process execution and in-memory runtime management.
+
+Follow the existing DevOrbit Master Project Context and preserve the architecture established in Phases 01–03.
+
+**Implement Phase 04 only. Do not implement Phase 05 or later functionality.**
+
+---
+
+## 1. Pre-Implementation Inspection
+
+Before making changes:
+
+1. Inspect the existing repository structure.
+2. Review the Service and Workspace models.
+3. Review the existing service repository and application layer.
+4. Review the REST API routing and authentication.
+5. Review backend startup and graceful shutdown.
+6. Review relevant tests and `docs/implementation-progress.md`.
+7. Identify existing interfaces that can be reused.
+
+Do not rewrite working components unnecessarily.
+
+Ensure registered service configurations are available through the existing service application/repository layer.
+
+Do not bypass existing application boundaries.
+
+## 2. Process Supervisor Architecture
+
+Implement a dedicated process supervisor responsible for managing executable processes.
+
+Suggested package:
+
+`internal/process`
+
+The process supervisor must be independent of HTTP handlers and GORM.
+
+It must manage runtime state in memory.
+
+### Responsibilities
+
+- Launch registered services.
+- Track active processes.
+- Maintain process lifecycle states.
+- Prevent duplicate process execution.
+- Observe process termination.
+- Track process exit information.
+- Support multiple concurrently running services.
+- Release runtime resources correctly.
+- Clean up owned processes during backend shutdown.
+
+Use Go's `os/exec` package.
+
+Do not introduce a distributed scheduler, message queue, or external process manager.
+
+## 3. Runtime Process Model
+
+Create a runtime model separate from the persisted Service configuration.
+
+Suggested fields:
+
+| Field | Type | Description |
+|---|---|---|
+| ServiceID | UUID | Registered service identifier |
+| PID | nullable int | OS process identifier |
+| State | enum/string | Current lifecycle state |
+| StartedAt | nullable time.Time | Start time |
+| ExitedAt | nullable time.Time | Termination time |
+| ExitCode | nullable int | Process exit code |
+| LastError | string | Relevant execution failure |
+| UpdatedAt | time.Time | Last state change |
+
+### Lifecycle States
+
+Define these states:
+
+- `stopped` — service has never started during this backend session.
+- `starting` — process launch is in progress.
+- `running` — OS process was started successfully.
+- `exited` — process terminated with exit code 0.
+- `failed` — process launch failed or process exited unsuccessfully.
+
+Reserve `stopping` for Phase 05.
+
+### Important Rules
+
+- Runtime state must not be stored in PostgreSQL.
+- Never use persisted PIDs as proof that a process is running.
+- Runtime state belongs to the current supervisor instance.
+- A service can have at most one active managed process.
+- A service that has exited can be started again.
+- A failed service can be retried.
+- A process can exit immediately after starting; this must not corrupt lifecycle state.
+- PID availability does not imply application readiness.
+
+## 4. Process Registry
+
+Implement a concurrency-safe process registry.
+
+The registry must associate each ServiceID with its current runtime process information.
+
+Requirements:
+
+1. Use appropriate synchronization such as `sync.Mutex` or `sync.RWMutex`.
+2. Prevent concurrent duplicate launches.
+3. Support concurrent execution of different services.
+4. Avoid holding global locks during slow operations.
+5. Coordinate state transitions safely.
+6. Return consistent snapshots of runtime status.
+7. Avoid exposing mutable internal state.
+8. Do not overwrite a newer execution's state with an older execution's exit callback.
+
+### Concurrency Scenario
+
+Two API requests attempt to start the same service simultaneously.
+
+Expected behavior:
+
+- Only one process may launch.
+- The competing request receives a conflict response.
+- No duplicate child processes are created.
+
+Use an atomic state reservation or equivalent synchronization mechanism.
+
+Do not rely solely on checking whether a PID exists.
+
+## 5. Process Execution
+
+Implement a Start operation using `exec.Command`.
+
+Example registered service:
+
+```json
+{
+  "name": "Go Backend",
+  "command": "go",
+  "args": ["run", "./cmd/api"],
+  "working_directory": "./backend",
+  "env": {
+    "APP_ENV": "development",
+    "PORT": "8080"
+  }
+}
+```
+
+### Execution Requirements
+
+1. Load the registered service by ID.
+2. Load its parent workspace when required.
+3. Validate the current execution configuration.
+4. Resolve the working directory.
+5. Confirm the directory still exists.
+6. Construct `exec.Cmd` with separate executable and argument values.
+7. Set the working directory.
+8. Inherit the backend's environment and apply service-specific overrides.
+9. Launch the process.
+10. Store the PID and runtime metadata.
+11. Transition the state to `running`.
+12. Observe termination asynchronously.
+
+Do not execute the command through a shell automatically.
+
+Do not combine command and arguments into a shell command string.
+
+### Environment Handling
+
+Build the child environment without mutating the backend's environment.
+
+Service environment overrides must replace inherited variables predictably.
+
+Account for platform-specific environment key behavior, especially Windows case-insensitive environment keys.
+
+Do not expose environment values in errors or logs.
+
+### Executable Validation
+
+Handle:
+
+- Missing executable
+- Invalid executable path
+- Permission denied
+- Missing working directory
+- Invalid environment configuration
+- Process creation failure
+
+Executable resolution must be performed consistently with the command's configured working directory and execution
\ No newline at end of file

# .agent/security.md

diff --git a/.agent/security.md b/.agent/security.md
new file mode 100644
index 0000000..5c20f7d
--- /dev/null
+++ b/.agent/security.md
@@ -0,0 +1,437 @@
+# DevOrbit Backend V1 — Implementation Phase 09
+## Security, Cross-Platform Compatibility & Reliability Hardening
+
+### Objective
+
+Perform a comprehensive security, correctness, and cross-platform reliability review of the existing DevOrbit backend.
+
+Identify and fix weaknesses in process supervision, API authentication, filesystem access, concurrency, logging, SSE streaming, and application lifecycle management.
+
+**This is a hardening phase, not a feature-development phase.**
+
+Follow the DevOrbit Master Project Context.
+
+### Technology Stack
+
+- Go
+- Chi HTTP Router
+- GORM
+- GORM Gen
+- PostgreSQL 17
+- GORM AutoMigrate
+- Electron + TypeScript (future frontend)
+
+Preserve functionality from Phases 01–08.
+
+**Implement Phase 09 only. Do not implement Phase 10 or the Electron frontend.**
+
+---
+
+## 1. Initial Architecture and Security Audit
+
+Before modifying code, inspect:
+
+- Application initialization and shutdown
+- HTTP router and middleware
+- Bearer-token authentication
+- Workspace and Service CRUD
+- GORM and GORM Gen repositories
+- Process Supervisor
+- Process-tree management
+- Start/Stop/Restart operations
+- Workspace Orchestrator
+- Log Manager
+- Event Broker
+- SSE handlers
+- Configuration and local filesystem storage
+- Existing tests and documentation
+
+Read `docs/implementation-progress.md`.
+
+Identify architectural risks and correctness problems.
+
+Classify findings as:
+
+- Critical: Potential unintended command execution, unsafe process termination, or authentication bypass
+- High: Resource leaks, serious lifecycle races, or sensitive information disclosure
+- Medium: Reliability, validation, isolation, or error-handling weaknesses
+- Low: Maintainability and minor hardening improvements
+
+Prioritize concrete, reproducible problems.
+
+Do not rewrite working modules merely for stylistic consistency.
+
+Avoid adding new dependencies unless they solve an identified problem.
+
+## 2. HTTP API Security Hardening
+
+Review and harden the existing local REST API.
+
+### Authentication
+
+Verify:
+
+1. All sensitive `/api/v1` endpoints require authentication.
+2. Authentication cannot be bypassed using alternate paths or HTTP methods.
+3. Bearer tokens are compared safely.
+4. Missing, malformed, and invalid tokens are rejected.
+5. Authentication tokens are never included in logs.
+6. API error responses do not disclose internal credentials.
+7. Token generation uses a cryptographically secure random source.
+8. Token loading and regeneration are handled safely.
+
+Do not replace the existing authentication architecture unless a concrete vulnerability requires it.
+
+### Loopback Binding
+
+DevOrbit is a local-first developer tool.
+
+Requirements:
+
+- Bind to loopback by default.
+- Prevent accidental exposure through wildcard interfaces.
+- Validate configured listen addresses.
+- Review IPv4 and IPv6 loopback handling.
+- Validate the HTTP Host header to reduce DNS-rebinding risk.
+- Do not blindly trust forwarded host or proxy headers.
+- Reject unsafe network configurations rather than silently exposing the API.
+
+Do not implement remote API access.
+
+### Origin and CORS Validation
+
+Review:
+
+- Browser-initiated REST requests
+- Cross-origin requests
+- SSE connections
+- Requests without Origin headers
+
+Requirements:
+
+- Reject untrusted browser origins.
+- Do not allow wildcard CORS.
+- Do not allow arbitrary websites to access local process-management endpoints.
+- Permit authenticated nonbrowser requests without an Origin header when appropriate.
+- Ensure SSE uses the same security boundary as REST.
+
+Document how the future Electron main process should authenticate.
+
+### HTTP Input Validation
+
+Verify:
+
+- JSON request body size limits
+- Unknown-field rejection
+- UUID validation
+- Query parameter validation
+- Pagination bounds
+- Appropriate HTTP methods
+- Content-Type validation where applicable
+- Request timeout behavior
+- Consistent error responses
+
+Avoid exposing stack traces, SQL errors, filesystem internals, or secrets to API clients.
+
+## 3. Local Authentication Token Security
+
+Inspect the token-storage mechanism implemented in Phase 01.
+
+### macOS and Linux
+
+Verify:
+
+- Token file permissions are restrictive.
+- Parent-directory permissions are appropriate.
+- Token files are not unintentionally world-readable.
+- Symlink-based redirection cannot expose or overwrite unrelated files.
+- Token creation does not use predictable values.
+
+### Windows
+
+Review actual Windows filesystem ACL behavior.
+
+Requirements:
+
+- Do not assume Unix permission bits provide meaningful Windows ACL protection.
+- Protect the token from unintended access by other local users.
+- Use Windows-specific APIs where necessary.
+- Avoid insecure token files in shared directories.
+- Report any unverified ACL behavior.
+
+If secure token storage cannot be established, fail safely rather than silently falling back to known insecure storage.
+
+Do not implement an enterprise secret vault or new authentication protocol.
+
+## 4. Process Execution Security
+
+Review the existing Process Supervisor.
+
+This is a security-sensitive component because it launches operating-system executables.
+
+### Command Execution
+
+Verify:
+
+- Only registered service configurations can be executed.
+- REST lifecycle endpoints cannot inject arbitrary commands or arguments.
+- Commands are executed using structured executable and argument arrays.
+- Shell execution is not introduced implicitly.
+- Untrusted HTTP input cannot override execution settings during Start.
+- Environment variables are passed safely.
+- Executable resolution behaves correctly across operating systems.
+- Invalid executable paths fail predictably.
+
+### Trust Boundary
+
+DevOrbit intentionally allows its authenticated local user to register arbitrary development commands.
+
+Preserve that capability.
+
+Do not introduce a restrictive command allowlist that prevents normal development workflows.
+
+Document that whoever can modify service configurations effectively has code-execution authority under the DevOrbit user's OS account.
+
+Do not claim DevOrbit is a security sandbox.
+
+### Environment Handling
+
+Verify:
+
+- Parent process environment is not mutated.
+- Service environment overrides behave correctly.
+- Malformed environment variable names are rejected.
+- Sensitive environment values do not appear in routine logs.
+- Windows case-insensitive environment-key behavior is handled.
+- Process startup errors avoid exposing secret values.
+
+## 5. Cross-Platform Process Management
+
+Audit process execution and termination on:
+
+- macOS
+- Linux
+- Windows
+
+### Unix Process Handling
+
+Review:
+
+- Process group creation
+- Parent/child relationships
+- Signal delivery
+- Graceful termination
+- Forced termination
+- Process reaping
+- Exit-code handling
+
+Ensure process termination does not accidentally signal unrelated process groups.
+
+Avoid relying on PID existence alone to establish ownership.
+
+### Windows Process Handling
+
+Review:
+
+- Windows process creation
+- Process handle lifecycle
+- Job Object ownership and cleanup
+- Child-process termination
+- Graceful termination limitations
+- Forced termination
+- Process exit-code retrieval
+- Resource handle cleanup
+
+Use OS-specific files and build constraints where appropriate.
+
+Do not assume Unix signal semantics work on Windows.
+
+### Process-Tree Safety
+
+Test managed execution trees representative of:
+
+- `go run`
+- `npm run dev`
+- `flutter run`
+- Parent processes that spawn child processes
+
+Use controlled test helpers for automated tests. Manual tests may additionally validate real developer commands.
+
+Verify that stopping a managed service handles its owned descendants correctly.
+
+Document limitations involving intentionally detached processes, escaped process groups, or platform capabilities that cannot be guaranteed.
+
+Never claim universal descendant cleanup without supporting verification.
+
+## 6. Process Lifecycle and Concurrency Audit
+
+Review all supervisor lifecycle operations.
+
+Required concurrency scenarios:
+
+- Start + Start
+- Start + Stop
+- Start + Restart
+- Stop + Stop
+- Stop + Restart
+- Restart + Restart
+- Natural exit + Stop
+- Natural exit + Restart
+- Workspace Start All + individual Start
+- Workspace Stop All + individual Restart
+- Backend shutdown + Start
+- Backend shutdown + Restart
+
+### Requirements
+
+1. Prevent duplicate managed process execution.
+2. Prevent stale process state updates.
+3. Avoid deadlocks.
+4. Avoid goroutine leaks.
+5. Ensure every started process is reaped exactly once.
+6. Avoid blocking global locks during process termination.
+7. Keep unrelated service operations independent.
+8. Preserve accurate lifecycle states.
+9. Prevent operations after shutdown begins.
+10. Handle process completion during concurrent lifecycle transitions.
+
+Audit the existing:
+
+- Process registry synchronization
+- Per-service operation coordination
+- `sync.WaitGroup` usage
+- Process completion channels
+- Operation completion channels
+- Supervisor shutdown flag
+
+Ensure WaitGroup additions cannot race incorrectly with a shutdown waiter.
+
+Use deterministic synchronization where practical rather than timing-dependent assumptions.
+
+## 7. Workspace and Filesystem Security
+
+Review filesystem operations across Workspace and Service configuration modules.
+
+### Workspace Paths
+
+Verify:
+
+- Path normalization
+- Absolute path handling
+- Relative path resolution
+- Symbolic link behavior
+- Missing directories
+- Permission failures
+- Cross-platform path handling
+
+Do not unintentionally change the established feature that allows users to register legitimate absolute development paths.
+
+### Service Working Directories
+
+Ensure:
+
+- Relative paths resolve against workspace root.
+- Invalid directories are rejected.
+- Directory validation is repeated when necessary before execution.
+- Path errors are handled safely.
+- Symlink behavior is explicitly defined.
+- Concurrent filesystem changes produce safe failures rather than crashes.
+
+### File Access Security
+
+Review:
+
+- Log directories
+- Authentication token files
+- Configuration files
+- Application data directories
+
+Prevent untrusted identifiers from constructing arbitrary filesystem paths.
+
+Avoid path traversal and unsafe file overwrite behavior.
+
+Use platform-appropriate file permissions.
+
+## 8. Log Manager Security and Reliability
+
+Review the Phase 07 Log Manager.
+
+### Security
+
+Verify:
+
+- Log files are stored in intended application directories.
+- File access permissions are appropriate.
+- Service IDs and execution IDs cannot cause path traversal.
+- Log APIs require authentication.
+- Workspace log queries do not return unrelated workspace data.
+- Log responses do not expose internal storage paths.
+- Untrusted application output is safely serialized.
+- Terminal control characters and ANSI sequences cannot affect SSE framing or JSON validity.
+
+Document that applications may print their own secrets and that DevOrbit cannot automatically guarantee redaction of arbitrary child-process output.
+
+### Reliability
+
+Verify:
+
+- Bounded memory buffering
+- Maximum log-entry sizes
+- File rotation
+- Storage retention
+- Disk-full handling
+- Concurrent log writes
+- Safe file closing
+- Output draining on process termination
+- Output draining on backend shutdown
+
+Ensure failures in log persistence do not unnecessarily crash process supervision.
+
+Avoid unbounded buffering under sustained output.
+
+Do not introduce external logging services.
+
+## 9. SSE and Event Broker Hardening
+
+Review Phase 08.
+
+### Security
+
+Verify:
+
+- SSE endpoints require authentication.
+- Workspace filtering is enforced.
+- Service filters cannot escape workspace boundaries.
+- No bearer token is included in event URLs.
+- Event payloads exclude credentials by default.
+- Event data is safely JSON-serialized.
+
+### Reliability
+
+Verify:
+
+- Multiple subscriber support
+- Concurrent publishing
+- Concurrent subscription removal
+- Slow-client backpressure
+- Queue overflow handling
+- Heartbeat cleanup
+- Client disconnect detection
+- No send-on-closed-channel panics
+- Event Broker shutdown behavior
+- Bounded subscriber memory usage
+
+### SSE Timeout Handling
+
+Review existing HTTP server timeouts.
+
+Long-lived SSE connections must remain functional without disabling appropriate timeouts for ordinary REST requests.
+
+Ensure SSE connections cannot prevent graceful application shutdown indefinitely.
+
+### Reconnection
+
+Preserve the documented V1 model:
+
+- Best-effort live event delivery
+- No durable event replay guarantee
\ No newline at end of file

# .agent/see_streaming_and_life_cycle_event.md

diff --git a/.agent/see_streaming_and_life_cycle_event.md b/.agent/see_streaming_and_life_cycle_event.md
new file mode 100644
index 0000000..4ab84cd
--- /dev/null
+++ b/.agent/see_streaming_and_life_cycle_event.md
@@ -0,0 +1,171 @@
+# DevOrbit Backend V1 — Implementation Phase 08
+## Real-Time SSE Streaming & Lifecycle Events
+
+### Objective
+
+Implement real-time event streaming for DevOrbit using Server-Sent Events (SSE).
+
+The backend must broadcast process lifecycle events and stdout/stderr logs to authenticated clients without polling.
+
+Follow the existing DevOrbit Master Project Context.
+
+**Tech Stack**
+- Go + Chi
+- GORM + GORM Gen
+- PostgreSQL 17
+- GORM AutoMigrate
+- Electron + TypeScript (future desktop frontend)
+
+Preserve all working functionality from Phases 01–07.
+
+**Implement Phase 08 only. Do not implement Phase 09 or the Electron frontend.**
+
+---
+
+## 1. Inspect Existing Implementation
+
+Before writing code:
+
+1. Inspect the Process Supervisor.
+2. Review Start, Stop, Restart, and process exit detection.
+3. Review the Log Manager implemented in Phase 07.
+4. Review log ingestion, persistence, and retrieval.
+5. Review workspace orchestration.
+6. Review Chi routing and authentication middleware.
+7. Review HTTP server timeout configuration.
+8. Review application initialization and shutdown.
+9. Read `docs/implementation-progress.md`.
+
+Identify existing abstractions suitable for event publishing.
+
+Do not recreate the Process Supervisor or Log Manager.
+
+Avoid unnecessary refactoring.
+
+## 2. Event System Architecture
+
+Introduce a lightweight in-memory event broker.
+
+Suggested package:
+
+`internal/events`
+
+Responsibilities:
+
+- Publish process lifecycle events.
+- Publish log-created events.
+- Register and unregister subscribers.
+- Filter events by workspace and service.
+- Broadcast events to multiple subscribers.
+- Handle disconnected or slow consumers.
+- Shut down cleanly.
+
+### Architecture
+
+Process Supervisor → Event Broker
+
+Log Manager → Event Broker
+
+Event Broker → SSE HTTP Handler → Electron Client
+
+The Event Broker must not:
+
+- Execute processes.
+- Query PostgreSQL for every event.
+- Write application logs to disk.
+- Manage process lifecycle.
+- Block process execution when clients are slow.
+
+Use Go channels and synchronization primitives.
+
+Do not introduce Redis, Kafka, NATS, RabbitMQ, or a distributed messaging system.
+
+## 3. Event Structure
+
+Define a consistent event envelope.
+
+Required fields:
+
+| Field | Description |
+|---|---|
+| ID | Unique event identifier |
+| Type | Event type |
+| Timestamp | UTC event timestamp |
+| WorkspaceID | Associated workspace |
+| ServiceID | Associated service, if applicable |
+| ExecutionID | Associated execution, if applicable |
+| Data | Event-specific payload |
+
+### Event Types
+
+Implement:
+
+- `process.started`
+- `process.stopping`
+- `process.stopped`
+- `process.exited`
+- `process.failed`
+- `process.restarted`
+- `log.created`
+
+Use constants or a typed event representation rather than scattering event-name strings throughout the codebase.
+
+Optional internal lifecycle events may be added only if necessary.
+
+### Example Process Event
+
+```json
+{
+  "id": "event-identifier",
+  "type": "process.started",
+  "timestamp": "2026-10-18T14:00:00Z",
+  "workspace_id": "workspace-uuid",
+  "service_id": "service-uuid",
+  "execution_id": "execution-uuid",
+  "data": {
+    "pid": 12345,
+    "state": "running"
+  }
+}
+```
+
+### Example Log Event
+
+```json
+{
+  "id": "event-identifier",
+  "type": "log.created",
+  "timestamp": "2026-10-18T14:00:01Z",
+  "workspace_id": "workspace-uuid",
+  "service_id": "service-uuid",
+  "execution_id": "execution-uuid",
+  "data": {
+    "stream": "stdout",
+    "sequence": 42,
+    "message": "HTTP server started"
+  }
+}
+```
+
+Reuse Phase 07 log entry metadata.
+
+Do not create a second log schema with incompatible field meanings.
+
+Ensure events never automatically expose authentication tokens, process environment values, or internal credentials.
+
+## 4. Event Broker
+
+Implement an in-memory publish/subscribe broker.
+
+Suggested responsibilities:
+
+- `Publish(event)`
+- `Subscribe(filter)`
+- `Unsubscribe(subscription)`
+- `Close()`
+
+These are conceptual operations. Adapt interfaces to existing project conventions.
+
+### Requirements
+
+1
\ No newline at end of file

# .agent/workspace_orachestration.md

diff --git a/.agent/workspace_orachestration.md b/.agent/workspace_orachestration.md
new file mode 100644
index 0000000..68f89ce
--- /dev/null
+++ b/.agent/workspace_orachestration.md
@@ -0,0 +1,299 @@
+# DevOrbit Backend V1 — Implementation Phase 06
+## Workspace Orchestration
+
+### Objective
+
+Implement workspace-level process orchestration, allowing developers to start, stop, and monitor all services belonging to a development workspace.
+
+Follow the existing DevOrbit Master Project Context.
+
+Use the established stack:
+
+- Go
+- Chi
+- GORM
+- GORM Gen
+- PostgreSQL 17
+- GORM AutoMigrate
+
+Preserve all working functionality from Phases 01–05.
+
+**Implement Phase 06 only. Do not implement future phases.**
+
+---
+
+## 1. Pre-Implementation Inspection
+
+Before modifying code:
+
+1. Inspect the Workspace and Service modules.
+2. Review the existing Process Supervisor.
+3. Review individual Start, Stop, Restart, and Status operations.
+4. Review process lifecycle states and concurrency protection.
+5. Review the existing database repository patterns.
+6. Review application startup and shutdown behavior.
+7. Review API authentication and error handling.
+8. Read `docs/implementation-progress.md`.
+
+Reuse the existing process supervisor.
+
+Do not create another process execution engine.
+
+If Phase 05 has unresolved lifecycle correctness issues, identify them before building orchestration on top of it.
+
+## 2. Workspace Orchestrator
+
+Introduce a workspace orchestration component.
+
+Suggested package:
+
+`internal/orchestration`
+
+Responsibilities:
+
+- Start all enabled services in a workspace.
+- Stop all running services in a workspace.
+- Retrieve aggregate workspace runtime status.
+- Coordinate multiple process operations.
+- Handle individual service failures.
+- Prevent conflicting workspace operations.
+- Return structured execution results.
+
+The orchestrator must delegate execution to the existing Process Supervisor.
+
+It must not directly use `os/exec`.
+
+Keep workspace orchestration independent of HTTP handlers and persistence implementation details.
+
+## 3. Start All Services
+
+Implement a workspace-level operation equivalent to:
+
+`StartWorkspace(ctx, workspaceID)`
+
+### Required Workflow
+
+1. Validate the workspace identifier.
+2. Confirm the workspace exists.
+3. Retrieve all registered services belonging to the workspace.
+4. Identify enabled services.
+5. Sort services deterministically by creation time, then ID.
+6. Check the supervisor's current state for each service.
+7. Skip services already running.
+8. Attempt to start eligible services.
+9. Continue processing remaining services if an individual service fails.
+10. Return a structured result for each service.
+
+### Rules
+
+- Start only services with `enabled = true`.
+- Disabled services must not start automatically.
+- Already-running services must not be duplicated.
+- Services in conflicting lifecycle states must be reported appropriately.
+- Different services may run concurrently after launching.
+- Do not wait for a service process to exit before starting the next one.
+- Do not interpret successful OS process launch as application readiness.
+- Do not implement dependency-aware startup.
+- Do not automatically roll back successfully started services when another service fails.
+
+### Ordering
+
+Use deterministic launch order.
+
+Example:
+
+1. Go Backend
+2. React Frontend
+3. Flutter Desktop
+4. Background Worker
+
+The order comes from persisted ordering, not hardcoded service names.
+
+Sequential launch attempts are acceptable for V1. Once started, the processes run independently.
+
+### Failure Handling
+
+If four services are enabled and one fails to launch:
+
+- The other three must still be attempted.
+- Successfully launched services remain running.
+- The failure must appear in the result.
+- Do not automatically stop successful services.
+
+Partial success is a valid outcome.
+
+## 4. Stop All Services
+
+Implement:
+
+`StopWorkspace(ctx, workspaceID)`
+
+### Required Workflow
+
+1. Confirm the workspace exists.
+2. Retrieve the workspace's registered services.
+3. Determine which services are currently running.
+4. Attempt to stop all active managed services.
+5. Continue stopping other services even when one fails.
+6. Wait for individual stop operations to complete according to their configured timeouts.
+7. Return a structured result for each service.
+
+### Rules
+
+- Stop running services regardless of their `enabled` value.
+- Skip already stopped or naturally exited services.
+- Reuse the existing supervisor's graceful and forced termination implementation.
+- Do not directly signal process IDs from the orchestrator.
+- Do not terminate services belonging to other workspaces.
+- Do not fail the entire operation immediately because one process cannot stop.
+- Report failures individually.
+- Never claim successful termination when an owned process is still running.
+
+Use reverse deterministic service order when stopping.
+
+This does not imply a dependency graph.
+
+## 5. Workspace Runtime Status
+
+Implement:
+
+`GetWorkspaceStatus(ctx, workspaceID)`
+
+Return a snapshot of runtime status for all registered services.
+
+### Required Information
+
+Workspace information:
+
+- workspace_id
+- workspace_name
+- aggregate_state
+- total_services
+- enabled_services
+- running_services
+- stopped_services
+- failed_services
+- starting_services
+- stopping_services
+- exited_services
+
+Service-level information:
+
+- service_id
+- name
+- enabled
+- pid
+- state
+- started_at
+- exited_at
+- exit_code
+- last_error
+
+Reuse the existing supervisor's runtime status representation.
+
+Do not persist process state to PostgreSQL.
+
+### Aggregate State
+
+Derive workspace status from current service states.
+
+Suggested aggregate states:
+
+- `idle`: no service is starting, running, or stopping.
+- `running`: all enabled services are running.
+- `partial`: some services are running, but the enabled workspace environment is not fully running.
+- `transitioning`: at least one service is starting or stopping.
+
+Handle the case where no services are enabled explicitly. Do not report an empty workspace as running.
+
+Failure and exit counts must remain visible independently from aggregate state.
+
+Document the exact aggregation rules.
+
+### Consistency
+
+Produce a coherent best-effort snapshot of process states.
+
+Concurrent process exits may happen immediately after the snapshot is created.
+
+Do not hold supervisor locks while querying PostgreSQL.
+
+## 6. Orchestration Operation Results
+
+Design a consistent result object for Start All and Stop All.
+
+Suggested structure:
+
+```json
+{
+  "workspace_id": "workspace-uuid",
+  "operation": "start",
+  "total": 4,
+  "succeeded": 3,
+  "failed": 1,
+  "skipped": 0,
+  "results": [
+    {
+      "service_id": "service-uuid",
+      "service_name": "Go Backend",
+      "outcome": "started",
+      "pid": 12345,
+      "error": null
+    }
+  ]
+}
+```
+
+The example is illustrative; reuse existing API conventions.
+
+### Required Outcomes
+
+For Start All:
+
+- started
+- already_running
+- disabled
+- failed
+- conflict
+
+For Stop All:
+
+- stopped
+- already_stopped
+- failed
+- conflict
+
+Keep summary counters consistent with individual results.
+
+Never expose sensitive environment variables or raw internal errors in API responses.
+
+## 7. Concurrency Management
+
+Workspace-level operations must be concurrency-safe.
+
+### Scenarios to Handle
+
+- Start All + Start All
+- Stop All + Stop All
+- Start All + Stop All
+- Start All + individual Start
+- Stop All + individual Stop
+- Start All + individual Restart
+- Workspace operation + backend shutdown
+- Service naturally exits during Start All
+- Service naturally exits during Stop All
+
+### Requirements
+
+1. Prevent conflicting workspace-level orchestration operations.
+2. Allow independent workspaces to operate concurrently.
+3. Reuse the existing supervisor's per-service concurrency guarantees.
+4. Do not introduce a global execution lock for all workspaces.
+5. Do not hold locks while performing slow database operations or waiting for process termination.
+6. Ensure locks and operation reservations are released on errors and cancellation.
+7. Do not allow duplicate process execution.
+8. Handle individual lifecycle conflicts as explicit per-service outcomes.
+9. Use predictable behavior when a workspace operation overlaps with individual service operations.
+10. Prevent new workspace starts during backend shutdown.
+
+Use a per
\ No newline at end of file

# docs/security-review.md

diff --git a/docs/security-review.md b/docs/security-review.md
new file mode 100644
index 0000000..e7c4b97
--- /dev/null
+++ b/docs/security-review.md
@@ -0,0 +1,18 @@
+# Phase 09 security review
+
+The review covered startup and shutdown, HTTP routing and authentication, CRUD validation and persistence, process supervision, orchestration, filesystem access, logs, and SSE. Findings below are based on code paths and focused tests, with severity tied to impact.
+
+| Severity | Finding | Resolution |
+| --- | --- | --- |
+| High | The local listener accepted arbitrary HTTP Host values, leaving a DNS rebinding route to local endpoints if a token was exposed to a browser. | The production server now requires its configured loopback IP and port in Host, including on `/health`. Config supplied directly to `app.Run` is also checked. |
+| High | A failed process launch stored a raw OS error in `last_error`, which could disclose an executable path or an environment value. | Launch errors are converted to a generic status and API error. |
+| High | An SSE write could block indefinitely after the handler cleared the server write deadline. | Each frame gets a fresh 30-second write deadline; the 15-second heartbeat keeps idle streams active. |
+| High | Token and log paths could follow links or be read with permissions inherited from unsafe locations. | Token files are opened without following the final Unix link and checked by handle. Token directories and log roots must be private; Windows uses explicit file ACLs and checks existing ACLs. Log queries skip linked files. |
+| Medium | A process could exit after Stop reserved the operation but before it read the tree pointer, causing a nil dereference. | Stop accepts an already completed process. |
+| Medium | A valid JSON prefix followed by more than 1 MiB of data could be accepted because decoders used `io.LimitReader`. | The API now uses `http.MaxBytesReader`, including trailing input. |
+| Medium | Duplicate Origin and query parameters had ambiguous interpretation. | Multiple Origin headers and duplicate, empty, or unknown SSE/log filters are rejected. |
+| Low | A process shutdown error skipped log-manager close. | Both shutdown operations now run and their errors are joined. |
+
+The token, logs, and SSE are accessible to anyone holding the local API token. Child processes can print secrets to logs; output is not automatically redacted. Authenticated configuration writers can execute arbitrary commands under the DevOrbit user's OS account. DevOrbit does not isolate those commands.
+
+Native Windows verification remains necessary for token directory ACLs, token-file creation and reload, Job Object ownership, descendant cleanup, and console control. Cross compilation checks API use but does not prove Windows runtime behavior. Native macOS verification remains necessary for process-group and signal behavior. Unix process groups and Windows jobs do not guarantee cleanup of intentionally detached or escaped descendants. On Unix, signaling a process group after its leader has exited has a narrow group-ID reuse risk; a future stronger ownership mechanism would be needed to eliminate it. SSE is best effort, with no replay.

# internal/events/broker.go

diff --git a/internal/events/broker.go b/internal/events/broker.go
new file mode 100644
index 0000000..e3d4a9e
--- /dev/null
+++ b/internal/events/broker.go
@@ -0,0 +1,102 @@
+package events
+
+import (
+	"sync"
+	"time"
+
+	"github.com/google/uuid"
+)
+
+type Type string
+
+const (
+	ProcessStarted   Type = "process.started"
+	ProcessStopping  Type = "process.stopping"
+	ProcessStopped   Type = "process.stopped"
+	ProcessExited    Type = "process.exited"
+	ProcessFailed    Type = "process.failed"
+	ProcessRestarted Type = "process.restarted"
+	LogCreated       Type = "log.created"
+)
+
+type Event struct {
+	ID          string     `json:"id"`
+	Type        Type       `json:"type"`
+	Timestamp   time.Time  `json:"timestamp"`
+	WorkspaceID uuid.UUID  `json:"workspace_id"`
+	ServiceID   uuid.UUID  `json:"service_id"`
+	ExecutionID *uuid.UUID `json:"execution_id,omitempty"`
+	Data        any        `json:"data"`
+}
+
+type Filter struct {
+	WorkspaceID uuid.UUID
+	ServiceID   uuid.UUID
+}
+
+type Broker struct {
+	mu          sync.Mutex
+	subscribers map[chan Event]Filter
+	closed      bool
+}
+
+func New() *Broker { return &Broker{subscribers: make(map[chan Event]Filter)} }
+
+func (b *Broker) Publish(event Event) {
+	if b == nil {
+		return
+	}
+	event.ID = uuid.NewString()
+	if event.Timestamp.IsZero() {
+		event.Timestamp = time.Now().UTC()
+	}
+	b.mu.Lock()
+	defer b.mu.Unlock()
+	if b.closed {
+		return
+	}
+	for ch, filter := range b.subscribers {
+		if filter.WorkspaceID != uuid.Nil && filter.WorkspaceID != event.WorkspaceID || filter.ServiceID != uuid.Nil && filter.ServiceID != event.ServiceID {
+			continue
+		}
+		select {
+		case ch <- event:
+		default:
+			// A slow stream must reconnect; process output cannot wait for its reader.
+			delete(b.subscribers, ch)
+			close(ch)
+		}
+	}
+}
+
+func (b *Broker) Subscribe(filter Filter) (<-chan Event, func()) {
+	ch := make(chan Event, 64)
+	b.mu.Lock()
+	if b.closed {
+		close(ch)
+	} else {
+		b.subscribers[ch] = filter
+	}
+	b.mu.Unlock()
+	return ch, func() {
+		b.mu.Lock()
+		if _, ok := b.subscribers[ch]; ok {
+			delete(b.subscribers, ch)
+			close(ch)
+		}
+		b.mu.Unlock()
+	}
+}
+
+func (b *Broker) Close() {
+	b.mu.Lock()
+	defer b.mu.Unlock()
+	if b.closed {
+		return
+	}
+	b.closed = true
+	for ch := range b.subscribers {
+		close(ch)
+		delete(b.subscribers, ch)
+	}
+}

# internal/events/broker_test.go

diff --git a/internal/events/broker_test.go b/internal/events/broker_test.go
new file mode 100644
index 0000000..744f8c9
--- /dev/null
+++ b/internal/events/broker_test.go
@@ -0,0 +1,54 @@
+package events
+
+import (
+	"testing"
+	"time"
+
+	"github.com/google/uuid"
+)
+
+func TestBrokerFiltersOverflowAndClose(t *testing.T) {
+	b := New()
+	wid, sid := uuid.New(), uuid.New()
+	workspace, cancelWorkspace := b.Subscribe(Filter{WorkspaceID: wid})
+	defer cancelWorkspace()
+	service, cancelService := b.Subscribe(Filter{ServiceID: sid})
+	defer cancelService()
+	other, cancelOther := b.Subscribe(Filter{WorkspaceID: uuid.New()})
+	defer cancelOther()
+	b.Publish(Event{Type: ProcessStarted, WorkspaceID: wid, ServiceID: sid})
+	for _, ch := range []<-chan Event{workspace, service} {
+		select {
+		case e := <-ch:
+			if e.ID == "" || e.Type != ProcessStarted || e.Timestamp.Location() != time.UTC || e.WorkspaceID != wid || e.ServiceID != sid {
+				t.Fatalf("bad event: %+v", e)
+			}
+		default:
+			t.Fatal("matching subscriber missed event")
+		}
+	}
+	select {
+	case <-other:
+		t.Fatal("unmatched subscriber got event")
+	default:
+	}
+	for range 65 {
+		b.Publish(Event{Type: LogCreated, WorkspaceID: wid, ServiceID: sid})
+	}
+	for range 64 {
+		<-workspace
+	}
+	if _, ok := <-workspace; ok {
+		t.Fatal("slow subscriber was not disconnected")
+	}
+	b.Close()
+	b.Close()
+	if _, ok := <-other; ok {
+		t.Fatal("broker close did not disconnect subscriber")
+	}
+	closed, cancel := b.Subscribe(Filter{})
+	defer cancel()
+	if _, ok := <-closed; ok {
+		t.Fatal("closed broker accepted subscriber")
+	}
+}

# internal/httpinput/pagination.go

diff --git a/internal/httpinput/pagination.go b/internal/httpinput/pagination.go
new file mode 100644
index 0000000..f99b176
--- /dev/null
+++ b/internal/httpinput/pagination.go
@@ -0,0 +1,26 @@
+package httpinput
+
+import (
+	"errors"
+	"net/url"
+	"strconv"
+)
+
+func Pagination(values url.Values) (int, int, error) {
+	limit, offset := 50, 0
+	if values.Has("limit") {
+		parsed, err := strconv.Atoi(values.Get("limit"))
+		if err != nil || parsed < 1 || parsed > 100 {
+			return 0, 0, errors.New("limit must be between 1 and 100")
+		}
+		limit = parsed
+	}
+	if values.Has("offset") {
+		parsed, err := strconv.Atoi(values.Get("offset"))
+		if err != nil || parsed < 0 || parsed > 1000000 {
+			return 0, 0, errors.New("offset must be between 0 and 1000000")
+		}
+		offset = parsed
+	}
+	return limit, offset, nil
+}

# internal/logs/manager.go

diff --git a/internal/logs/manager.go b/internal/logs/manager.go
new file mode 100644
index 0000000..90493ca
--- /dev/null
+++ b/internal/logs/manager.go
@@ -0,0 +1,483 @@
+package logs
+
+import (
+	"bufio"
+	"bytes"
+	"container/heap"
+	"context"
+	"encoding/json"
+	"errors"
+	"fmt"
+	"io"
+	"os"
+	"path/filepath"
+	"sort"
+	"strings"
+	"sync"
+	"time"
+	"unicode/utf8"
+
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/events"
+	"github.com/prasannashrestha011/devOrbit/internal/security"
+)
+
+const (
+	maxEntries    = 500
+	maxBytes      = 2 << 20
+	maxChunk      = 16 << 10
+	maxFileBytes  = 10 << 20
+	maxFiles      = 5
+	maxExecutions = 20
+	retention     = 7 * 24 * time.Hour
+)
+
+var ErrClosed = errors.New("log manager is closed")
+
+type Entry struct {
+	ID          string    `json:"id"`
+	WorkspaceID uuid.UUID `json:"workspace_id"`
+	ServiceID   uuid.UUID `json:"service_id"`
+	ExecutionID uuid.UUID `json:"execution_id"`
+	Stream      string    `json:"stream"`
+	Timestamp   time.Time `json:"timestamp"`
+	Sequence    uint64    `json:"sequence"`
+	Message     string    `json:"message"`
+}
+
+type Diagnostics struct {
+	DroppedEntries uint64 `json:"dropped_entries"`
+	StorageErrors  uint64 `json:"storage_errors"`
+}
+
+type Filter struct {
+	WorkspaceID uuid.UUID
+	ServiceID   uuid.UUID
+	Before      string
+	Limit       int
+}
+
+type serviceBuffer struct {
+	entries []Entry
+	bytes   int
+	diag    Diagnostics
+}
+
+type Manager struct {
+	root   string
+	events *events.Broker
+	mu     sync.Mutex
+	closed bool
+	active map[uuid.UUID]uuid.UUID // execution ID -> service ID
+	buffer map[uuid.UUID]*serviceBuffer
+	wg     sync.WaitGroup
+}
+
+func New(root string, broker ...*events.Broker) (*Manager, error) {
+	if err := os.MkdirAll(root, 0700); err != nil {
+		return nil, err
+	}
+	if err := security.PrivateDirectory(root); err != nil {
+		return nil, errors.New("log directory must be private")
+	}
+	root = filepath.Join(root, "services")
+	if err := os.MkdirAll(root, 0700); err != nil {
+		return nil, err
+	}
+	if err := security.PrivateDirectory(root); err != nil {
+		return nil, errors.New("log directory must be private")
+	}
+	m := &Manager{root: root, active: make(map[uuid.UUID]uuid.UUID), buffer: make(map[uuid.UUID]*serviceBuffer)}
+	if len(broker) > 0 {
+		m.events = broker[0]
+	}
+	services, err := os.ReadDir(root)
+	if err != nil {
+		return nil, err
+	}
+	for _, service := range services {
+		if service.IsDir() {
+			if id, err := uuid.Parse(service.Name()); err == nil {
+				if err := m.prune(id); err != nil {
+					return nil, err
+				}
+			}
+		}
+	}
+	return m, nil
+}
+
+func (m *Manager) Begin(workspaceID, serviceID uuid.UUID) (*Capture, error) {
+	if workspaceID == uuid.Nil || serviceID == uuid.Nil {
+		return nil, errors.New("workspace and service IDs are required")
+	}
+	m.mu.Lock()
+	defer m.mu.Unlock()
+	if m.closed {
+		return nil, ErrClosed
+	}
+	if err := m.prune(serviceID); err != nil {
+		return nil, err
+	}
+	executionID := uuid.New()
+	dir := filepath.Join(m.root, serviceID.String(), executionID.String())
+	if err := os.MkdirAll(dir, 0700); err != nil {
+		return nil, err
+	}
+	file, err := os.OpenFile(filepath.Join(dir, "output-0001.jsonl"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
+	if err != nil {
+		_ = os.RemoveAll(dir)
+		return nil, err
+	}
+	c := &Capture{manager: m, workspaceID: workspaceID, serviceID: serviceID, executionID: executionID, dir: dir, file: file, fileNumber: 1, queue: make(chan pending, 1024), done: make(chan struct{})}
+	c.Stdout = &streamWriter{capture: c, stream: "stdout"}
+	c.Stderr = &streamWriter{capture: c, stream: "stderr"}
+	m.active[executionID] = serviceID
+	m.wg.Add(1)
+	go c.run()
+	return c, nil
+}
+
+func (m *Manager) Close() error {
+	m.mu.Lock()
+	m.closed = true
+	m.mu.Unlock()
+	m.wg.Wait()
+	return nil
+}
+
+func (m *Manager) Diagnostics(serviceID uuid.UUID) Diagnostics {
+	m.mu.Lock()
+	defer m.mu.Unlock()
+	if b := m.buffer[serviceID]; b != nil {
+		return b.diag
+	}
+	return Diagnostics{}
+}
+
+func (m *Manager) Recent(serviceID uuid.UUID) []Entry {
+	m.mu.Lock()
+	defer m.mu.Unlock()
+	if b := m.buffer[serviceID]; b != nil {
+		return append([]Entry(nil), b.entries...)
+	}
+	return nil
+}
+
+func (m *Manager) add(e Entry) {
+	m.mu.Lock()
+	b := m.buffer[e.ServiceID]
+	if b == nil {
+		b = &serviceBuffer{}
+		m.buffer[e.ServiceID] = b
+	}
+	b.entries = append(b.entries, e)
+	b.bytes += len(e.Message)
+	for len(b.entries) > maxEntries || b.bytes > maxBytes {
+		b.bytes -= len(b.entries[0].Message)
+		b.entries[0] = Entry{}
+		b.entries = b.entries[1:]
+	}
+	m.mu.Unlock()
+	m.events.Publish(events.Event{Type: events.LogCreated, Timestamp: e.Timestamp, WorkspaceID: e.WorkspaceID, ServiceID: e.ServiceID, ExecutionID: &e.ExecutionID, Data: map[string]any{
+		"stream": e.Stream, "sequence": e.Sequence, "message": e.Message,
+	}})
+}
+
+func (m *Manager) diagnostic(serviceID uuid.UUID, dropped, storage uint64) {
+	m.mu.Lock()
+	defer m.mu.Unlock()
+	b := m.buffer[serviceID]
+	if b == nil {
+		b = &serviceBuffer{}
+		m.buffer[serviceID] = b
+	}
+	b.diag.DroppedEntries += dropped
+	b.diag.StorageErrors += storage
+}
+
+type pending struct {
+	stream  string
+	message string
+}
+
+type Capture struct {
+	Stdout, Stderr io.Writer
+	manager        *Manager
+	workspaceID    uuid.UUID
+	serviceID      uuid.UUID
+	executionID    uuid.UUID
+	dir            string
+	file           *os.File
+	fileNumber     int
+	fileSize       int64
+	sequence       uint64
+	queue          chan pending
+	done           chan struct{}
+	closeOnce      sync.Once
+	writeErr       error
+}
+
+func (c *Capture) ExecutionID() uuid.UUID { return c.executionID }
+
+func (c *Capture) enqueue(p pending) {
+	select {
+	case c.queue <- p:
+	default:
+		c.manager.diagnostic(c.serviceID, 1, 0)
+	}
+}
+
+func (c *Capture) run() {
+	defer close(c.done)
+	defer c.manager.wg.Done()
+	for p := range c.queue {
+		c.sequence++
+		now := time.Now().UTC()
+		e := Entry{
+			ID:          fmt.Sprintf("%020d-%s-%012d", now.UnixNano(), c.executionID, c.sequence),
+			WorkspaceID: c.workspaceID, ServiceID: c.serviceID, ExecutionID: c.executionID,
+			Stream: p.stream, Timestamp: now, Sequence: c.sequence, Message: p.message,
+		}
+		c.manager.add(e)
+		if c.writeErr == nil {
+			if err := c.persist(e); err != nil {
+				c.writeErr = err
+				c.manager.diagnostic(c.serviceID, 0, 1)
+			}
+		}
+	}
+	if c.file != nil {
+		if err := c.file.Close(); err != nil {
+			c.writeErr = err
+			c.manager.diagnostic(c.serviceID, 0, 1)
+		}
+	}
+	_ = os.Chtimes(c.dir, time.Now(), time.Now())
+	c.manager.mu.Lock()
+	delete(c.manager.active, c.executionID)
+	c.manager.mu.Unlock()
+}
+
+func (c *Capture) persist(e Entry) error {
+	data, err := json.Marshal(e)
+	if err != nil {
+		return err
+	}
+	data = append(data, '\n')
+	if c.fileSize > 0 && c.fileSize+int64(len(data)) > maxFileBytes {
+		if err := c.file.Close(); err != nil {
+			return err
+		}
+		c.fileNumber++
+		c.file, err = os.OpenFile(filepath.Join(c.dir, fmt.Sprintf("output-%04d.jsonl", c.fileNumber)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
+		if err != nil {
+			return err
+		}
+		c.fileSize = 0
+		if c.fileNumber > maxFiles {
+			if err := os.Remove(filepath.Join(c.dir, fmt.Sprintf("output-%04d.jsonl", c.fileNumber-maxFiles))); err != nil {
+				return err
+			}
+		}
+	}
+	n, err := c.file.Write(data)
+	c.fileSize += int64(n)
+	return err
+}
+
+func (c *Capture) Close() error {
+	c.closeOnce.Do(func() {
+		c.Stdout.(*streamWriter).close()
+		c.Stderr.(*streamWriter).close()
+		close(c.queue)
+	})
+	<-c.done
+	return c.writeErr
+}
+
+type streamWriter struct {
+	mu      sync.Mutex
+	capture *Capture
+	stream  string
+	partial []byte
+	closed  bool
+}
+
+func (w *streamWriter) Write(p []byte) (int, error) {
+	w.mu.Lock()
+	defer w.mu.Unlock()
+	if w.closed {
+		return 0, io.ErrClosedPipe
+	}
+	n := len(p)
+	for len(p) > 0 {
+		end := len(p)
+		if end > maxChunk-len(w.partial) {
+			end = maxChunk - len(w.partial)
+		}
+		if i := bytes.IndexByte(p[:end], '\n'); i >= 0 {
+			end = i + 1
+		}
+		w.partial = append(w.partial, p[:end]...)
+		p = p[end:]
+		if len(w.partial) == maxChunk {
+			w.emit()
+		} else if w.partial[len(w.partial)-1] == '\n' {
+			w.capture.enqueue(pending{w.stream, string(w.partial)})
+			w.partial = w.partial[:0]
+		}
+	}
+	w.emit()
+	return n, nil
+}
+
+func (w *streamWriter) emit() {
+	cut := len(w.partial)
+	for i := cut - 1; i >= 0 && i >= cut-3; i-- {
+		if utf8.RuneStart(w.partial[i]) {
+			if !utf8.FullRune(w.partial[i:]) {
+				cut = i
+			}
+			break
+		}
+	}
+	if cut > 0 {
+		w.capture.enqueue(pending{w.stream, string(w.partial[:cut])})
+		w.partial = append(w.partial[:0], w.partial[cut:]...)
+	}
+}
+
+func (w *streamWriter) close() {
+	w.mu.Lock()
+	defer w.mu.Unlock()
+	if len(w.partial) > 0 {
+		w.capture.enqueue(pending{w.stream, string(w.partial)})
+		w.partial = nil
+	}
+	w.closed = true
+}
+
+func (m *Manager) prune(serviceID uuid.UUID) error {
+	dir := filepath.Join(m.root, serviceID.String())
+	info, statErr := os.Lstat(dir)
+	if errors.Is(statErr, os.ErrNotExist) {
+		return nil
+	}
+	if statErr != nil || !info.IsDir() {
+		return errors.New("service log directory is not a directory")
+	}
+	items, err := os.ReadDir(dir)
+	if errors.Is(err, os.ErrNotExist) {
+		return nil
+	}
+	if err != nil {
+		return err
+	}
+	type item struct {
+		name     string
+		modified time.Time
+	}
+	var kept []item
+	for _, e := range items {
+		id, err := uuid.Parse(e.Name())
+		if err != nil || !e.IsDir() {
+			continue
+		}
+		info, err := e.Info()
+		if err != nil {
+			return err
+		}
+		if _, active := m.active[id]; !active && time.Since(info.ModTime()) > retention {
+			if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
+				return err
+			}
+			continue
+		}
+		kept = append(kept, item{e.Name(), info.ModTime()})
+	}
+	sort.Slice(kept, func(i, j int) bool { return kept[i].modified.After(kept[j].modified) })
+	if len(kept) < maxExecutions {
+		return nil
+	}
+	for _, e := range kept[maxExecutions-1:] {
+		id, _ := uuid.Parse(e.name)
+		if _, active := m.active[id]; !active {
+			if err := os.RemoveAll(filepath.Join(dir, e.name)); err != nil {
+				return err
+			}
+		}
+	}
+	return nil
+}
+
+type entryHeap []Entry
+
+func (h entryHeap) Len() int           { return len(h) }
+func (h entryHeap) Less(i, j int) bool { return h[i].ID < h[j].ID }
+func (h entryHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
+func (h *entryHeap) Push(x any)        { *h = append(*h, x.(Entry)) }
+func (h *entryHeap) Pop() any {
+	old := *h
+	x := old[len(old)-1]
+	*h = old[:len(old)-1]
+	return x
+}
+
+// List returns newest entries first; Before is the ID of the oldest entry on the previous page.
+func (m *Manager) List(ctx context.Context, f Filter) ([]Entry, error) {
+	if f.Limit < 1 || f.Limit > 500 || len(f.Before) > 128 {
+		return nil, errors.New("invalid log query")
+	}
+	root := m.root
+	if f.ServiceID != uuid.Nil {
+		root = filepath.Join(root, f.ServiceID.String())
+	}
+	h := &entryHeap{}
+	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
+		if errors.Is(err, os.ErrNotExist) {
+			return nil
+		}
+		if err != nil || ctx.Err() != nil {
+			return errors.Join(err, ctx.Err())
+		}
+		if d.IsDir() || d.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(d.Name(), ".jsonl") {
+			return nil
+		}
+		file, err := os.Open(path)
+		if err != nil {
+			return err
+		}
+		scanner := bufio.NewScanner(file)
+		scanner.Buffer(make([]byte, 64<<10), 1<<20)
+		for scanner.Scan() {
+			if err := ctx.Err(); err != nil {
+				_ = file.Close()
+				return err
+			}
+			var e Entry
+			if json.Unmarshal(scanner.Bytes(), &e) != nil || e.ID == "" {
+				continue // A crash can leave the final JSONL record incomplete.
+			}
+			if f.ServiceID != uuid.Nil && e.ServiceID != f.ServiceID || f.WorkspaceID != uuid.Nil && e.WorkspaceID != f.WorkspaceID || f.Before != "" && e.ID >= f.Before {
+				continue
+			}
+			if h.Len() < f.Limit {
+				heap.Push(h, e)
+			} else if e.ID > (*h)[0].ID {
+				(*h)[0] = e
+				heap.Fix(h, 0)
+			}
+		}
+		scanErr := scanner.Err()
+		return errors.Join(scanErr, file.Close())
+	})
+	if err != nil {
+		return nil, err
+	}
+	result := make([]Entry, h.Len())
+	for i := len(result) - 1; i >= 0; i-- {
+		result[i] = heap.Pop(h).(Entry)
+	}
+	return result, nil
+}

# internal/logs/manager_test.go

diff --git a/internal/logs/manager_test.go b/internal/logs/manager_test.go
new file mode 100644
index 0000000..d214cdc
--- /dev/null
+++ b/internal/logs/manager_test.go
@@ -0,0 +1,242 @@
+package logs
+
+import (
+	"context"
+	"os"
+	"path/filepath"
+	"strings"
+	"sync"
+	"testing"
+	"time"
+
+	"github.com/google/uuid"
+)
+
+func privateTempDir(t *testing.T) string {
+	t.Helper()
+	dir := t.TempDir()
+	if err := os.Chmod(dir, 0700); err != nil {
+		t.Fatal(err)
+	}
+	return dir
+}
+
+func TestCaptureHistoryAndPaging(t *testing.T) {
+	root := privateTempDir(t)
+	m, err := New(root)
+	if err != nil {
+		t.Fatal(err)
+	}
+	workspace, service := uuid.New(), uuid.New()
+	c, err := m.Begin(workspace, service)
+	if err != nil {
+		t.Fatal(err)
+	}
+	var wg sync.WaitGroup
+	for _, stream := range []struct {
+		writer interface{ Write([]byte) (int, error) }
+		text   string
+	}{{c.Stdout, "hello\npartial"}, {c.Stderr, "\x1b[31merror\r\n"}} {
+		wg.Add(1)
+		go func() {
+			defer wg.Done()
+			if _, err := stream.writer.Write([]byte(stream.text)); err != nil {
+				t.Error(err)
+			}
+		}()
+	}
+	wg.Wait()
+	if err := c.Close(); err != nil {
+		t.Fatal(err)
+	}
+	if got := m.Recent(service); len(got) != 3 {
+		t.Fatalf("recent count = %d", len(got))
+	}
+	recent := m.Recent(service)
+	recent[0].Message = "changed"
+	if m.Recent(service)[0].Message == "changed" {
+		t.Fatal("snapshot was mutable")
+	}
+	first, err := m.List(context.Background(), Filter{ServiceID: service, Limit: 2})
+	if err != nil || len(first) != 2 {
+		t.Fatalf("first page = %+v, %v", first, err)
+	}
+	second, err := m.List(context.Background(), Filter{WorkspaceID: workspace, Limit: 2, Before: first[1].ID})
+	if err != nil || len(second) != 1 {
+		t.Fatalf("second page = %+v, %v", second, err)
+	}
+	if first[0].ID <= first[1].ID || second[0].ID >= first[1].ID {
+		t.Fatal("incorrect order")
+	}
+	for _, e := range append(first, second...) {
+		if e.ExecutionID != c.ExecutionID() || e.WorkspaceID != workspace || e.ServiceID != service || e.Timestamp.Location() != time.UTC {
+			t.Fatalf("bad metadata: %+v", e)
+		}
+	}
+	if err := m.Close(); err != nil {
+		t.Fatal(err)
+	}
+	reopened, err := New(root)
+	if err != nil {
+		t.Fatal(err)
+	}
+	defer reopened.Close()
+	all, err := reopened.List(context.Background(), Filter{ServiceID: service, Limit: 100})
+	if err != nil || len(all) != 3 {
+		t.Fatalf("persisted = %+v, %v", all, err)
+	}
+	if _, err := m.Begin(workspace, service); err != ErrClosed {
+		t.Fatalf("begin after close: %v", err)
+	}
+}
+
+func TestRejectsLogDirectorySymlinks(t *testing.T) {
+	base := filepath.Join(t.TempDir(), "redirected")
+	if err := os.Symlink(t.TempDir(), base); err != nil {
+		t.Skipf("symlinks unavailable: %v", err)
+	}
+	if _, err := New(base); err == nil {
+		t.Fatal("accepted redirected configured log directory")
+	}
+	root := privateTempDir(t)
+	if err := os.Symlink(t.TempDir(), filepath.Join(root, "services")); err != nil {
+		t.Skipf("symlinks unavailable: %v", err)
+	}
+	if _, err := New(root); err == nil {
+		t.Fatal("accepted redirected log root")
+	}
+	other := privateTempDir(t)
+	m, err := New(other)
+	if err != nil {
+		t.Fatal(err)
+	}
+	defer m.Close()
+	id := uuid.New()
+	if err := os.Symlink(t.TempDir(), filepath.Join(other, "services", id.String())); err != nil {
+		t.Skipf("symlinks unavailable: %v", err)
+	}
+	if _, err := m.Begin(uuid.New(), id); err == nil {
+		t.Fatal("accepted redirected service log directory")
+	}
+}
+
+func TestChunkBufferRotationAndRetention(t *testing.T) {
+	root := privateTempDir(t)
+	m, err := New(root)
+	if err != nil {
+		t.Fatal(err)
+	}
+	defer m.Close()
+	workspace, service := uuid.New(), uuid.New()
+	c, err := m.Begin(workspace, service)
+	if err != nil {
+		t.Fatal(err)
+	}
+	c.fileSize = maxFileBytes - 100
+	if _, err := c.Stdout.Write([]byte(strings.Repeat("x", maxChunk*550))); err != nil {
+		t.Fatal(err)
+	}
+	if err := c.Close(); err != nil {
+		t.Fatal(err)
+	}
+	if got := m.Recent(service); len(got) != maxBytes/maxChunk {
+		t.Fatalf("recent count = %d", len(got))
+	}
+	if m.Recent(service)[0].Message == "" {
+		t.Fatal("missing buffered message")
+	}
+	files, err := os.ReadDir(c.dir)
+	if err != nil || len(files) < 2 {
+		t.Fatalf("rotation files = %d, %v", len(files), err)
+	}
+	all, err := m.List(context.Background(), Filter{ServiceID: service, Limit: 500})
+	if err != nil || len(all) != 500 {
+		t.Fatalf("history = %d, %v", len(all), err)
+	}
+	stale := filepath.Join(root, "services", service.String(), uuid.NewString())
+	if err := os.MkdirAll(stale, 0700); err != nil {
+		t.Fatal(err)
+	}
+	old := time.Now().Add(-retention - time.Hour)
+	if err := os.Chtimes(stale, old, old); err != nil {
+		t.Fatal(err)
+	}
+	next, err := m.Begin(workspace, service)
+	if err != nil {
+		t.Fatal(err)
+	}
+	if err := next.Close(); err != nil {
+		t.Fatal(err)
+	}
+	if _, err := os.Stat(stale); !os.IsNotExist(err) {
+		t.Fatalf("stale execution retained: %v", err)
+	}
+}
+
+func TestExecutionSeparationAndUTF8ChunkBoundary(t *testing.T) {
+	m, err := New(privateTempDir(t))
+	if err != nil {
+		t.Fatal(err)
+	}
+	defer m.Close()
+	wid, sid := uuid.New(), uuid.New()
+	first, err := m.Begin(wid, sid)
+	if err != nil {
+		t.Fatal(err)
+	}
+	message := strings.Repeat("x", maxChunk-1) + "€"
+	if _, err := first.Stdout.Write([]byte(message)); err != nil {
+		t.Fatal(err)
+	}
+	if err := first.Close(); err != nil {
+		t.Fatal(err)
+	}
+	second, err := m.Begin(wid, sid)
+	if err != nil {
+		t.Fatal(err)
+	}
+	if _, err := second.Stderr.Write([]byte("new run\n")); err != nil {
+		t.Fatal(err)
+	}
+	if err := second.Close(); err != nil {
+		t.Fatal(err)
+	}
+	entries, err := m.List(context.Background(), Filter{ServiceID: sid, Limit: 10})
+	if err != nil || len(entries) != 3 {
+		t.Fatalf("entries = %+v, %v", entries, err)
+	}
+	if first.ExecutionID() == second.ExecutionID() || entries[0].ExecutionID != second.ExecutionID() {
+		t.Fatal("execution IDs mixed")
+	}
+	if entries[2].Message+entries[1].Message != message {
+		t.Fatal("UTF-8 chunk boundary changed content")
+	}
+}
+
+func TestPartialOutputIsVisibleWhileRunning(t *testing.T) {
+	m, err := New(privateTempDir(t))
+	if err != nil {
+		t.Fatal(err)
+	}
+	defer m.Close()
+	c, err := m.Begin(uuid.New(), uuid.New())
+	if err != nil {
+		t.Fatal(err)
+	}
+	if _, err := c.Stdout.Write([]byte("still running")); err != nil {
+		t.Fatal(err)
+	}
+	deadline := time.Now().Add(time.Second)
+	for time.Now().Before(deadline) {
+		if entries := m.Recent(c.serviceID); len(entries) == 1 && entries[0].Message == "still running" {
+			break
+		}
+		time.Sleep(time.Millisecond)
+	}
+	if entries := m.Recent(c.serviceID); len(entries) != 1 || entries[0].Message != "still running" {
+		t.Fatalf("partial output unavailable: %+v", entries)
+	}
+	if err := c.Close(); err != nil {
+		t.Fatal(err)
+	}
+}

# internal/modules/events/controller/controller.go

diff --git a/internal/modules/events/controller/controller.go b/internal/modules/events/controller/controller.go
new file mode 100644
index 0000000..33b9f50
--- /dev/null
+++ b/internal/modules/events/controller/controller.go
@@ -0,0 +1,128 @@
+package controller
+
+import (
+	"context"
+	"encoding/json"
+	"fmt"
+	"io"
+	"net/http"
+	"time"
+
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/events"
+	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
+	serviceconfig "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
+	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
+	workspace "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
+)
+
+type serviceLookup interface {
+	Get(context.Context, uuid.UUID) (*serviceModel.Service, error)
+}
+
+type workspaceLookup interface {
+	Get(context.Context, uuid.UUID) (*workspaceModel.Workspace, error)
+}
+
+type Controller struct {
+	broker     *events.Broker
+	services   serviceLookup
+	workspaces workspaceLookup
+}
+
+func New(broker *events.Broker, services serviceLookup, workspaces workspaceLookup) *Controller {
+	return &Controller{broker: broker, services: services, workspaces: workspaces}
+}
+
+func (c *Controller) Stream(w http.ResponseWriter, r *http.Request) {
+	if err := validateFilterQuery(r.URL.Query()); err != nil {
+		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
+		return
+	}
+	filter := events.Filter{}
+	if value := r.URL.Query().Get("workspace_id"); value != "" {
+		id, err := parseFilterID(value, "workspace_id")
+		if err != nil {
+			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
+			return
+		}
+		if _, err := c.workspaces.Get(r.Context(), id); err != nil {
+			writeLookupError(w, workspace.IsNotFound(err), "workspace")
+			return
+		}
+		filter.WorkspaceID = id
+	}
+	if value := r.URL.Query().Get("service_id"); value != "" {
+		id, err := parseFilterID(value, "service_id")
+		if err != nil {
+			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
+			return
+		}
+		service, err := c.services.Get(r.Context(), id)
+		if err != nil {
+			writeLookupError(w, serviceconfig.IsNotFound(err), "service")
+			return
+		}
+		if filter.WorkspaceID != uuid.Nil && service.WorkspaceID != filter.WorkspaceID {
+			writeError(w, http.StatusBadRequest, "invalid_request", "service is outside workspace")
+			return
+		}
+		filter.ServiceID = id
+	}
+	stream, unsubscribe := c.broker.Subscribe(filter)
+	defer unsubscribe()
+	response := http.NewResponseController(w)
+	// A fresh deadline for each frame keeps the stream live without pinning shutdown on a stalled reader.
+	w.Header().Set("Content-Type", "text/event-stream")
+	w.Header().Set("Cache-Control", "no-cache")
+	w.Header().Set("X-Accel-Buffering", "no")
+	if err := response.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
+		return
+	}
+	if _, err := io.WriteString(w, ": connected\n\n"); err != nil || response.Flush() != nil {
+		return
+	}
+	heartbeat := time.NewTicker(15 * time.Second)
+	defer heartbeat.Stop()
+	for {
+		select {
+		case event, ok := <-stream:
+			if !ok {
+				return
+			}
+			data, err := json.Marshal(event)
+			if err != nil {
+				return
+			}
+			if err := response.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
+				return
+			}
+			if _, err := fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", event.ID, event.Type, data); err != nil || response.Flush() != nil {
+				return
+			}
+		case <-heartbeat.C:
+			if err := response.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
+				return
+			}
+			if _, err := io.WriteString(w, ": keep-alive\n\n"); err != nil || response.Flush() != nil {
+				return
+			}
+		case <-r.Context().Done():
+			return
+		}
+	}
+}
+
+func writeLookupError(w http.ResponseWriter, missing bool, name string) {
+	if missing {
+		writeError(w, http.StatusNotFound, "not_found", name+" not found")
+	} else {
+		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
+	}
+}
+
+func writeError(w http.ResponseWriter, status int, code, message string) {
+	w.Header().Set("Content-Type", "application/json; charset=utf-8")
+	w.WriteHeader(status)
+	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
+}

# internal/modules/events/controller/controller_test.go

diff --git a/internal/modules/events/controller/controller_test.go b/internal/modules/events/controller/controller_test.go
new file mode 100644
index 0000000..e01f68d
--- /dev/null
+++ b/internal/modules/events/controller/controller_test.go
@@ -0,0 +1,115 @@
+package controller
+
+import (
+	"bufio"
+	"context"
+	"io"
+	"log/slog"
+	"net/http"
+	"net/http/httptest"
+	"strings"
+	"testing"
+	"time"
+
+	"github.com/go-chi/chi/v5"
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/api"
+	"github.com/prasannashrestha011/devOrbit/internal/events"
+	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
+	serviceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
+	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
+	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
+)
+
+type services map[uuid.UUID]*serviceModel.Service
+
+func (s services) Get(_ context.Context, id uuid.UUID) (*serviceModel.Service, error) {
+	if item := s[id]; item != nil {
+		return item, nil
+	}
+	return nil, serviceRepo.ErrNotFound
+}
+
+type workspaces map[uuid.UUID]*workspaceModel.Workspace
+
+func (s workspaces) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
+	if item := s[id]; item != nil {
+		return item, nil
+	}
+	return nil, workspaceRepo.ErrNotFound
+}
+
+func TestAuthenticatedSSEFiltersAndWriteTimeout(t *testing.T) {
+	b := events.New()
+	defer b.Close()
+	wid, sid := uuid.New(), uuid.New()
+	c := New(b, services{sid: {ID: sid, WorkspaceID: wid}}, workspaces{wid: {ID: wid}})
+	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
+		r.Get("/events", c.Stream)
+	})
+	server := httptest.NewUnstartedServer(handler)
+	server.Config.WriteTimeout = 100 * time.Millisecond
+	server.Start()
+	defer server.Close()
+	client := &http.Client{Timeout: 5 * time.Second}
+	request := func(path, token string) *http.Response {
+		t.Helper()
+		req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
+		if err != nil {
+			t.Fatal(err)
+		}
+		if token != "" {
+			req.Header.Set("Authorization", "Bearer "+token)
+		}
+		resp, err := client.Do(req)
+		if err != nil {
+			t.Fatal(err)
+		}
+		return resp
+	}
+	for _, tc := range []struct {
+		path, token string
+		status      int
+	}{
+		{"/api/v1/events", "", 401},
+		{"/api/v1/events?workspace_id=bad", "token", 400},
+		{"/api/v1/events?service_id=" + sid.String() + "&service_id=" + sid.String(), "token", 400},
+		{"/api/v1/events?unknown=1", "token", 400},
+		{"/api/v1/events?service_id=" + uuid.NewString(), "token", 404},
+		{"/api/v1/events?workspace_id=" + uuid.NewString(), "token", 404},
+		{"/api/v1/events?workspace_id=" + wid.String() + "&service_id=" + sid.String(), "token", 200},
+	} {
+		resp := request(tc.path, tc.token)
+		if resp.StatusCode != tc.status {
+			t.Fatalf("%s: %d", tc.path, resp.StatusCode)
+		}
+		resp.Body.Close()
+	}
+	resp := request("/api/v1/events?service_id="+sid.String(), "token")
+	defer resp.Body.Close()
+	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
+		t.Fatalf("stream response: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
+	}
+	reader := bufio.NewReader(resp.Body)
+	if line, err := reader.ReadString('\n'); err != nil || line != ": connected\n" {
+		t.Fatalf("initial flush: %q, %v", line, err)
+	}
+	_, _ = reader.ReadString('\n')
+	time.Sleep(150 * time.Millisecond)
+	b.Publish(events.Event{Type: events.ProcessStarted, WorkspaceID: wid, ServiceID: uuid.New()})
+	b.Publish(events.Event{Type: events.ProcessStarted, WorkspaceID: wid, ServiceID: sid, Data: map[string]any{"state": "running"}})
+	var frame string
+	for {
+		line, err := reader.ReadString('\n')
+		if err != nil {
+			t.Fatal(err)
+		}
+		frame += line
+		if line == "\n" {
+			break
+		}
+	}
+	if !strings.HasPrefix(frame, "id: ") || !strings.Contains(frame, "event: process.started\n") || !strings.Contains(frame, `"service_id":"`+sid.String()+`"`) {
+		t.Fatalf("unexpected SSE frame: %s", frame)
+	}
+}

# internal/modules/events/controller/validation.go

diff --git a/internal/modules/events/controller/validation.go b/internal/modules/events/controller/validation.go
new file mode 100644
index 0000000..d0f0de4
--- /dev/null
+++ b/internal/modules/events/controller/validation.go
@@ -0,0 +1,25 @@
+package controller
+
+import (
+	"errors"
+	"net/url"
+
+	"github.com/google/uuid"
+)
+
+func validateFilterQuery(values url.Values) error {
+	for key, items := range values {
+		if key != "workspace_id" && key != "service_id" || len(items) != 1 || items[0] == "" {
+			return errors.New("invalid event filter")
+		}
+	}
+	return nil
+}
+
+func parseFilterID(value, field string) (uuid.UUID, error) {
+	id, err := uuid.Parse(value)
+	if err != nil || id == uuid.Nil {
+		return uuid.Nil, errors.New("invalid " + field)
+	}
+	return id, nil
+}

# internal/modules/events/routes/routes.go

diff --git a/internal/modules/events/routes/routes.go b/internal/modules/events/routes/routes.go
new file mode 100644
index 0000000..664cb0f
--- /dev/null
+++ b/internal/modules/events/routes/routes.go
@@ -0,0 +1,18 @@
+package routes
+
+import (
+	"github.com/go-chi/chi/v5"
+	"github.com/prasannashrestha011/devOrbit/internal/events"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/events/controller"
+	serviceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
+	serviceService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
+	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
+	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
+	"gorm.io/gorm"
+)
+
+func RegisterRoutes(r chi.Router, db *gorm.DB, broker *events.Broker) {
+	workspaces := workspaceService.New(workspaceRepo.New(db))
+	services := serviceService.New(serviceRepo.New(db), workspaceRepo.New(db))
+	r.Get("/events", controller.New(broker, services, workspaces).Stream)
+}

# internal/modules/logs/controller/controller.go

diff --git a/internal/modules/logs/controller/controller.go b/internal/modules/logs/controller/controller.go
new file mode 100644
index 0000000..1723ae5
--- /dev/null
+++ b/internal/modules/logs/controller/controller.go
@@ -0,0 +1,77 @@
+package controller
+
+import (
+	"encoding/json"
+	"net/http"
+
+	"github.com/prasannashrestha011/devOrbit/internal/logs"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/logs/dto"
+	configService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
+	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
+)
+
+type Controller struct {
+	logs       *logs.Manager
+	configs    *configService.Service
+	workspaces *workspaceService.Service
+}
+
+func New(manager *logs.Manager, configs *configService.Service, workspaces *workspaceService.Service) *Controller {
+	return &Controller{logs: manager, configs: configs, workspaces: workspaces}
+}
+
+func (c *Controller) Service(w http.ResponseWriter, r *http.Request) {
+	id, filter, err := parseQuery(r)
+	if err != nil {
+		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
+		return
+	}
+	if _, err := c.configs.Get(r.Context(), id); err != nil {
+		if configService.IsNotFound(err) {
+			writeError(w, http.StatusNotFound, "not_found", "service not found")
+		} else {
+			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
+		}
+		return
+	}
+	filter.ServiceID = id
+	entries, err := c.logs.List(r.Context(), filter)
+	if err != nil {
+		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
+		return
+	}
+	writeJSON(w, http.StatusOK, dto.NewListResponse(entries, c.logs.Diagnostics(id)))
+}
+
+func (c *Controller) Workspace(w http.ResponseWriter, r *http.Request) {
+	id, filter, err := parseQuery(r)
+	if err != nil {
+		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
+		return
+	}
+	if _, err := c.workspaces.Get(r.Context(), id); err != nil {
+		if workspaceService.IsNotFound(err) {
+			writeError(w, http.StatusNotFound, "not_found", "workspace not found")
+		} else {
+			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
+		}
+		return
+	}
+	filter.WorkspaceID = id
+	entries, err := c.logs.List(r.Context(), filter)
+	if err != nil {
+		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
+		return
+	}
+	writeJSON(w, http.StatusOK, dto.NewListResponse(entries, logs.Diagnostics{}))
+}
+
+func writeJSON(w http.ResponseWriter, code int, body any) {
+	w.Header().Set("Content-Type", "application/json; charset=utf-8")
+	w.WriteHeader(code)
+	_ = json.NewEncoder(w).Encode(body)
+}
+
+func writeError(w http.ResponseWriter, code int, name, message string) {
+	writeJSON(w, code, map[string]any{"error": map[string]string{"code": name, "message": message}})
+}

# internal/modules/logs/controller/controller_test.go

diff --git a/internal/modules/logs/controller/controller_test.go b/internal/modules/logs/controller/controller_test.go
new file mode 100644
index 0000000..db6c60f
--- /dev/null
+++ b/internal/modules/logs/controller/controller_test.go
@@ -0,0 +1,114 @@
+package controller
+
+import (
+	"context"
+	"io"
+	"log/slog"
+	"net/http"
+	"net/http/httptest"
+	"path/filepath"
+	"strings"
+	"testing"
+
+	"github.com/go-chi/chi/v5"
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/api"
+	"github.com/prasannashrestha011/devOrbit/internal/logs"
+	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
+	serviceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
+	serviceService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
+	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
+	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
+	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
+)
+
+type services map[uuid.UUID]*serviceModel.Service
+
+func (s services) Get(_ context.Context, id uuid.UUID) (*serviceModel.Service, error) {
+	if v := s[id]; v != nil {
+		return v, nil
+	}
+	return nil, serviceRepo.ErrNotFound
+}
+func (services) Create(context.Context, *serviceModel.Service) error { return nil }
+func (services) List(context.Context, uuid.UUID, int, int) ([]*serviceModel.Service, error) {
+	return nil, nil
+}
+func (services) Update(context.Context, uuid.UUID, map[string]any) (*serviceModel.Service, error) {
+	return nil, nil
+}
+func (services) Delete(context.Context, uuid.UUID) error { return nil }
+func (services) NameExists(context.Context, uuid.UUID, string, uuid.UUID) (bool, error) {
+	return false, nil
+}
+
+type workspaces map[uuid.UUID]*workspaceModel.Workspace
+
+func (s workspaces) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
+	if v := s[id]; v != nil {
+		return v, nil
+	}
+	return nil, workspaceRepo.ErrNotFound
+}
+func (workspaces) Create(context.Context, *workspaceModel.Workspace) error { return nil }
+func (workspaces) List(context.Context, int, int) ([]*workspaceModel.Workspace, error) {
+	return nil, nil
+}
+func (workspaces) Update(context.Context, uuid.UUID, map[string]any) (*workspaceModel.Workspace, error) {
+	return nil, nil
+}
+func (workspaces) Delete(context.Context, uuid.UUID) error { return nil }
+
+func TestLogAPI(t *testing.T) {
+	m, err := logs.New(filepath.Join(t.TempDir(), "logs"))
+	if err != nil {
+		t.Fatal(err)
+	}
+	defer m.Close()
+	wid, sid := uuid.New(), uuid.New()
+	capture, err := m.Begin(wid, sid)
+	if err != nil {
+		t.Fatal(err)
+	}
+	_, _ = capture.Stdout.Write([]byte("one\ntwo\n"))
+	if err := capture.Close(); err != nil {
+		t.Fatal(err)
+	}
+	c := New(m, serviceService.New(services{sid: {ID: sid, WorkspaceID: wid}}, workspaces{wid: {ID: wid}}), workspaceService.New(workspaces{wid: {ID: wid}}))
+	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
+		r.Get("/services/{id}/logs", c.Service)
+		r.Get("/workspaces/{id}/logs", c.Workspace)
+	})
+	request := func(path, token string) *httptest.ResponseRecorder {
+		t.Helper()
+		r := httptest.NewRequest(http.MethodGet, path, nil)
+		if token != "" {
+			r.Header.Set("Authorization", "Bearer "+token)
+		}
+		w := httptest.NewRecorder()
+		handler.ServeHTTP(w, r)
+		return w
+	}
+	base := "/api/v1/services/" + sid.String() + "/logs"
+	for _, tc := range []struct {
+		path, token string
+		code        int
+	}{
+		{base, "", 401}, {"/api/v1/services/bad/logs", "token", 400},
+		{base + "?limit=0", "token", 400}, {"/api/v1/services/" + uuid.NewString() + "/logs", "token", 404},
+		{base + "?limit=1&limit=2", "token", 400}, {base + "?unknown=1", "token", 400},
+		{"/api/v1/workspaces/" + uuid.NewString() + "/logs", "token", 404},
+	} {
+		if w := request(tc.path, tc.token); w.Code != tc.code {
+			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
+		}
+	}
+	w := request(base+"?limit=1", "token")
+	if w.Code != 200 || !strings.Contains(w.Body.String(), `"message":"two\n"`) || strings.Contains(w.Body.String(), `"message":"one\n"`) {
+		t.Fatalf("service logs: %d %s", w.Code, w.Body.String())
+	}
+	w = request("/api/v1/workspaces/"+wid.String()+"/logs", "token")
+	if w.Code != 200 || !strings.Contains(w.Body.String(), `"message":"one\n"`) || !strings.Contains(w.Body.String(), `"message":"two\n"`) {
+		t.Fatalf("workspace logs: %d %s", w.Code, w.Body.String())
+	}
+}

# internal/modules/logs/controller/validation.go

diff --git a/internal/modules/logs/controller/validation.go b/internal/modules/logs/controller/validation.go
new file mode 100644
index 0000000..bab53ec
--- /dev/null
+++ b/internal/modules/logs/controller/validation.go
@@ -0,0 +1,35 @@
+package controller
+
+import (
+	"errors"
+	"net/http"
+	"strconv"
+
+	"github.com/go-chi/chi/v5"
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/logs"
+)
+
+func parseQuery(r *http.Request) (uuid.UUID, logs.Filter, error) {
+	for key, values := range r.URL.Query() {
+		if key != "limit" && key != "before" || len(values) != 1 || values[0] == "" {
+			return uuid.Nil, logs.Filter{}, errors.New("invalid log query")
+		}
+	}
+	id, err := uuid.Parse(chi.URLParam(r, "id"))
+	if err != nil || id == uuid.Nil {
+		return uuid.Nil, logs.Filter{}, errors.New("invalid id")
+	}
+	limit := 100
+	if value := r.URL.Query().Get("limit"); value != "" {
+		limit, err = strconv.Atoi(value)
+		if err != nil || limit < 1 || limit > 500 {
+			return uuid.Nil, logs.Filter{}, errors.New("limit must be between 1 and 500")
+		}
+	}
+	before := r.URL.Query().Get("before")
+	if len(before) > 128 {
+		return uuid.Nil, logs.Filter{}, errors.New("invalid cursor")
+	}
+	return id, logs.Filter{Limit: limit, Before: before}, nil
+}

# internal/modules/logs/dto/response.go

diff --git a/internal/modules/logs/dto/response.go b/internal/modules/logs/dto/response.go
new file mode 100644
index 0000000..d7e2086
--- /dev/null
+++ b/internal/modules/logs/dto/response.go
@@ -0,0 +1,20 @@
+package dto
+
+import "github.com/prasannashrestha011/devOrbit/internal/logs"
+
+type ListResponse struct {
+	Entries     []logs.Entry     `json:"entries"`
+	NextCursor  string           `json:"next_cursor"`
+	Diagnostics logs.Diagnostics `json:"diagnostics"`
+}
+
+func NewListResponse(entries []logs.Entry, diagnostics logs.Diagnostics) ListResponse {
+	if entries == nil {
+		entries = []logs.Entry{}
+	}
+	var cursor string
+	if len(entries) > 0 {
+		cursor = entries[len(entries)-1].ID
+	}
+	return ListResponse{Entries: entries, NextCursor: cursor, Diagnostics: diagnostics}
+}

# internal/modules/logs/dto/response_test.go

diff --git a/internal/modules/logs/dto/response_test.go b/internal/modules/logs/dto/response_test.go
new file mode 100644
index 0000000..39aa2e2
--- /dev/null
+++ b/internal/modules/logs/dto/response_test.go
@@ -0,0 +1,20 @@
+package dto
+
+import (
+	"encoding/json"
+	"strings"
+	"testing"
+
+	"github.com/prasannashrestha011/devOrbit/internal/logs"
+)
+
+func TestListResponseShape(t *testing.T) {
+	empty, err := json.Marshal(NewListResponse(nil, logs.Diagnostics{}))
+	if err != nil || !strings.Contains(string(empty), `"entries":[]`) || !strings.Contains(string(empty), `"next_cursor":""`) {
+		t.Fatalf("empty response: %s (%v)", empty, err)
+	}
+	result := NewListResponse([]logs.Entry{{ID: "first"}, {ID: "last"}}, logs.Diagnostics{})
+	if result.NextCursor != "last" {
+		t.Fatalf("cursor = %q", result.NextCursor)
+	}
+}

# internal/modules/logs/routes/routes.go

diff --git a/internal/modules/logs/routes/routes.go b/internal/modules/logs/routes/routes.go
new file mode 100644
index 0000000..dbb1ea8
--- /dev/null
+++ b/internal/modules/logs/routes/routes.go
@@ -0,0 +1,20 @@
+package routes
+
+import (
+	"github.com/go-chi/chi/v5"
+	"github.com/prasannashrestha011/devOrbit/internal/logs"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/logs/controller"
+	serviceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
+	serviceService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
+	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
+	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
+	"gorm.io/gorm"
+)
+
+func RegisterRoutes(r chi.Router, db *gorm.DB, manager *logs.Manager) {
+	workspaces := workspaceService.New(workspaceRepo.New(db))
+	configs := serviceService.New(serviceRepo.New(db), workspaceRepo.New(db))
+	c := controller.New(manager, configs, workspaces)
+	r.Get("/services/{id}/logs", c.Service)
+	r.Get("/workspaces/{id}/logs", c.Workspace)
+}

# internal/modules/process/controller/controller.go

diff --git a/internal/modules/process/controller/controller.go b/internal/modules/process/controller/controller.go
new file mode 100644
index 0000000..1230c04
--- /dev/null
+++ b/internal/modules/process/controller/controller.go
@@ -0,0 +1,100 @@
+package controller
+
+import (
+	"encoding/json"
+	"errors"
+	"net/http"
+
+	"github.com/go-chi/chi/v5"
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
+	"github.com/prasannashrestha011/devOrbit/internal/process"
+)
+
+type Controller struct{ service *service.Service }
+
+func New(s *service.Service) *Controller { return &Controller{service: s} }
+
+func (c *Controller) Start(w http.ResponseWriter, r *http.Request) {
+	id, ok := parseID(w, r)
+	if !ok {
+		return
+	}
+	status, err := c.service.Start(r.Context(), id)
+	if err != nil {
+		writeOperationError(w, err)
+		return
+	}
+	writeJSON(w, http.StatusAccepted, status)
+}
+
+func (c *Controller) Stop(w http.ResponseWriter, r *http.Request) {
+	id, ok := parseID(w, r)
+	if !ok {
+		return
+	}
+	status, err := c.service.Stop(r.Context(), id)
+	if err != nil {
+		writeOperationError(w, err)
+		return
+	}
+	writeJSON(w, http.StatusOK, status)
+}
+
+func (c *Controller) Restart(w http.ResponseWriter, r *http.Request) {
+	id, ok := parseID(w, r)
+	if !ok {
+		return
+	}
+	status, err := c.service.Restart(r.Context(), id)
+	if err != nil {
+		writeOperationError(w, err)
+		return
+	}
+	writeJSON(w, http.StatusOK, status)
+}
+
+func (c *Controller) Status(w http.ResponseWriter, r *http.Request) {
+	id, ok := parseID(w, r)
+	if !ok {
+		return
+	}
+	status, err := c.service.Status(r.Context(), id)
+	if err != nil {
+		writeOperationError(w, err)
+		return
+	}
+	writeJSON(w, http.StatusOK, status)
+}
+
+func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
+	id, err := uuid.Parse(chi.URLParam(r, "id"))
+	if err != nil {
+		writeError(w, http.StatusBadRequest, "invalid_request", "invalid id")
+		return uuid.Nil, false
+	}
+	return id, true
+}
+
+func writeOperationError(w http.ResponseWriter, err error) {
+	switch {
+	case service.IsNotFound(err):
+		writeError(w, http.StatusNotFound, "not_found", "workspace or service not found")
+	case errors.Is(err, process.ErrActive), errors.Is(err, service.ErrDisabled), errors.Is(err, process.ErrClosed):
+		writeError(w, http.StatusConflict, "conflict", err.Error())
+	case errors.Is(err, process.ErrInvalid):
+		writeError(w, http.StatusBadRequest, "invalid_request", "service execution configuration is invalid")
+	default:
+		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
+	}
+}
+
+func writeJSON(w http.ResponseWriter, code int, body any) {
+	w.Header().Set("Content-Type", "application/json; charset=utf-8")
+	w.WriteHeader(code)
+	_ = json.NewEncoder(w).Encode(body)
+}
+
+func writeError(w http.ResponseWriter, code int, name, message string) {
+	writeJSON(w, code, map[string]any{"error": map[string]string{"code": name, "message": message}})
+}

# internal/modules/process/controller/controller_test.go

diff --git a/internal/modules/process/controller/controller_test.go b/internal/modules/process/controller/controller_test.go
new file mode 100644
index 0000000..e27d063
--- /dev/null
+++ b/internal/modules/process/controller/controller_test.go
@@ -0,0 +1,164 @@
+package controller
+
+import (
+	"context"
+	"encoding/json"
+	"io"
+	"log/slog"
+	"net/http"
+	"net/http/httptest"
+	"os"
+	"path/filepath"
+	"strings"
+	"testing"
+	"time"
+
+	"github.com/go-chi/chi/v5"
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/api"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
+	configRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
+	configService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
+	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
+	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
+	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
+	"github.com/prasannashrestha011/devOrbit/internal/process"
+)
+
+type configStore map[uuid.UUID]*model.Service
+
+func (s configStore) Get(_ context.Context, id uuid.UUID) (*model.Service, error) {
+	if item := s[id]; item != nil {
+		return item, nil
+	}
+	return nil, configRepo.ErrNotFound
+}
+func (configStore) Create(context.Context, *model.Service) error { return nil }
+func (configStore) List(context.Context, uuid.UUID, int, int) ([]*model.Service, error) {
+	return nil, nil
+}
+func (configStore) Update(context.Context, uuid.UUID, map[string]any) (*model.Service, error) {
+	return nil, nil
+}
+func (configStore) Delete(context.Context, uuid.UUID) error { return nil }
+func (configStore) NameExists(context.Context, uuid.UUID, string, uuid.UUID) (bool, error) {
+	return false, nil
+}
+
+type workspaceStore map[uuid.UUID]*workspaceModel.Workspace
+
+func (s workspaceStore) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
+	if item := s[id]; item != nil {
+		return item, nil
+	}
+	return nil, workspaceRepo.ErrNotFound
+}
+func (workspaceStore) Create(context.Context, *workspaceModel.Workspace) error { return nil }
+func (workspaceStore) List(context.Context, int, int) ([]*workspaceModel.Workspace, error) {
+	return nil, nil
+}
+func (workspaceStore) Update(context.Context, uuid.UUID, map[string]any) (*workspaceModel.Workspace, error) {
+	return nil, nil
+}
+func (workspaceStore) Delete(context.Context, uuid.UUID) error { return nil }
+
+func TestHTTPChild(t *testing.T) {
+	if os.Getenv("DEVORBIT_HTTP_CHILD") == "1" {
+		for {
+			time.Sleep(time.Second)
+		}
+	}
+}
+
+func TestProcessAPI(t *testing.T) {
+	self, err := filepath.Abs(os.Args[0])
+	if err != nil {
+		t.Fatal(err)
+	}
+	wid, id, disabledID := uuid.New(), uuid.New(), uuid.New()
+	enabled, disabled := true, false
+	workspaces := workspaceStore{wid: {ID: wid, RootPath: t.TempDir()}}
+	configs := configStore{
+		id:         {ID: id, WorkspaceID: wid, Command: self, Args: []string{"-test.run=^TestHTTPChild$"}, Env: map[string]string{"DEVORBIT_HTTP_CHILD": "1"}, Enabled: &enabled},
+		disabledID: {ID: disabledID, WorkspaceID: wid, Command: self, Enabled: &disabled},
+	}
+	supervisor := process.New(100 * time.Millisecond)
+	t.Cleanup(func() { _ = supervisor.Shutdown(context.Background()) })
+	c := New(service.New(configService.New(configs, workspaces), workspaceService.New(workspaces), supervisor))
+	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
+		r.Post("/services/{id}/start", c.Start)
+		r.Post("/services/{id}/stop", c.Stop)
+		r.Post("/services/{id}/restart", c.Restart)
+		r.Get("/services/{id}/status", c.Status)
+	})
+	request := func(method, path, token string) *httptest.ResponseRecorder {
+		t.Helper()
+		req := httptest.NewRequest(method, path, nil)
+		if token != "" {
+			req.Header.Set("Authorization", "Bearer "+token)
+		}
+		w := httptest.NewRecorder()
+		handler.ServeHTTP(w, req)
+		return w
+	}
+	base := "/api/v1/services/" + id.String()
+	for _, tc := range []struct {
+		method, path, token string
+		want                int
+		state               process.State
+	}{
+		{http.MethodPost, base + "/start", "", 401, ""},
+		{http.MethodPost, base + "/stop", "", 401, ""},
+		{http.MethodPost, base + "/restart", "", 401, ""},
+		{http.MethodGet, base + "/status", "", 401, ""},
+		{http.MethodPost, "/api/v1/services/bad/start", "token", 400, ""},
+		{http.MethodPost, "/api/v1/services/bad/stop", "token", 400, ""},
+		{http.MethodPost, "/api/v1/services/bad/restart", "token", 400, ""},
+		{http.MethodGet, "/api/v1/services/" + uuid.NewString() + "/status", "token", 404, ""},
+		{http.MethodPost, "/api/v1/services/" + uuid.NewString() + "/stop", "token", 404, ""},
+		{http.MethodPost, "/api/v1/services/" + uuid.NewString() + "/restart", "token", 404, ""},
+		{http.MethodPost, "/api/v1/services/" + disabledID.String() + "/start", "token", 409, ""},
+		{http.MethodPost, "/api/v1/services/" + disabledID.String() + "/restart", "token", 409, ""},
+		{http.MethodPost, "/api/v1/services/" + disabledID.String() + "/stop", "token", 200, ""},
+		{http.MethodGet, base + "/status", "token", 200, process.Stopped},
+		{http.MethodPost, base + "/start", "token", 202, process.Running},
+		{http.MethodPost, base + "/start", "token", 409, ""},
+		{http.MethodGet, base + "/status", "token", 200, process.Running},
+	} {
+		w := request(tc.method, tc.path, tc.token)
+		if w.Code != tc.want {
+			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
+		}
+		if tc.state != "" {
+			var status process.Status
+			if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil || status.ServiceID != id || status.State != tc.state {
+				t.Fatalf("status response: %s (%v)", w.Body.String(), err)
+			}
+		}
+		if strings.Contains(w.Body.String(), "DEVORBIT_HTTP_CHILD") {
+			t.Fatalf("response exposed environment: %s", w.Body.String())
+		}
+	}
+	before := supervisor.Status(id)
+	w := request(http.MethodPost, base+"/restart", "token")
+	if w.Code != http.StatusOK {
+		t.Fatalf("restart: %d %s", w.Code, w.Body.String())
+	}
+	var restarted process.Status
+	if err := json.Unmarshal(w.Body.Bytes(), &restarted); err != nil || restarted.State != process.Running || *restarted.PID == *before.PID {
+		t.Fatalf("restart status: %+v %v", restarted, err)
+	}
+	w = request(http.MethodPost, base+"/stop", "token")
+	var stopped process.Status
+	if err := json.Unmarshal(w.Body.Bytes(), &stopped); err != nil || w.Code != http.StatusOK || stopped.State != process.Stopped || stopped.ExitedAt == nil {
+		t.Fatalf("stop: %d %+v %v", w.Code, stopped, err)
+	}
+	if w := request(http.MethodPost, base+"/stop", "token"); w.Code != http.StatusOK {
+		t.Fatalf("repeat stop: %d %s", w.Code, w.Body.String())
+	}
+	configs[id].Command = "missing-devorbit-executable"
+	if w := request(http.MethodPost, base+"/restart", "token"); w.Code != http.StatusBadRequest {
+		t.Fatalf("restart should use current config: %d %s", w.Code, w.Body.String())
+	}
+}

# internal/modules/process/routes/routes.go

diff --git a/internal/modules/process/routes/routes.go b/internal/modules/process/routes/routes.go
new file mode 100644
index 0000000..20e8dc8
--- /dev/null
+++ b/internal/modules/process/routes/routes.go
@@ -0,0 +1,23 @@
+package routes
+
+import (
+	"github.com/go-chi/chi/v5"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/process/controller"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
+	serviceconfig "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
+	configService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
+	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
+	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
+	"github.com/prasannashrestha011/devOrbit/internal/process"
+	"gorm.io/gorm"
+)
+
+func RegisterRoutes(r chi.Router, db *gorm.DB, supervisor *process.Supervisor) {
+	workspaces := workspaceService.New(workspaceRepo.New(db))
+	configs := configService.New(serviceconfig.New(db), workspaceRepo.New(db))
+	c := controller.New(service.New(configs, workspaces, supervisor))
+	r.Post("/services/{id}/start", c.Start)
+	r.Post("/services/{id}/stop", c.Stop)
+	r.Post("/services/{id}/restart", c.Restart)
+	r.Get("/services/{id}/status", c.Status)
+}

# internal/modules/process/service/service.go

diff --git a/internal/modules/process/service/service.go b/internal/modules/process/service/service.go
new file mode 100644
index 0000000..9d5db36
--- /dev/null
+++ b/internal/modules/process/service/service.go
@@ -0,0 +1,75 @@
+package service
+
+import (
+	"context"
+	"errors"
+
+	"github.com/google/uuid"
+	serviceconfig "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
+	workspace "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
+	"github.com/prasannashrestha011/devOrbit/internal/process"
+)
+
+var ErrDisabled = errors.New("service is disabled")
+
+type Service struct {
+	configs    *serviceconfig.Service
+	workspaces *workspace.Service
+	supervisor *process.Supervisor
+}
+
+func New(configs *serviceconfig.Service, workspaces *workspace.Service, supervisor *process.Supervisor) *Service {
+	return &Service{configs: configs, workspaces: workspaces, supervisor: supervisor}
+}
+
+func (s *Service) Start(ctx context.Context, id uuid.UUID) (process.Status, error) {
+	cfg, err := s.configuration(ctx, id)
+	if err != nil {
+		return process.Status{}, err
+	}
+	return s.supervisor.Start(id, cfg)
+}
+
+func (s *Service) Stop(ctx context.Context, id uuid.UUID) (process.Status, error) {
+	if _, err := s.configs.Get(ctx, id); err != nil {
+		return process.Status{}, err
+	}
+	return s.supervisor.Stop(ctx, id)
+}
+
+func (s *Service) Restart(ctx context.Context, id uuid.UUID) (process.Status, error) {
+	if _, err := s.configuration(ctx, id); err != nil {
+		return process.Status{}, err
+	}
+	return s.supervisor.Restart(ctx, id, func(ctx context.Context) (process.Config, error) {
+		return s.configuration(ctx, id)
+	})
+}
+
+func (s *Service) configuration(ctx context.Context, id uuid.UUID) (process.Config, error) {
+	config, err := s.configs.Get(ctx, id)
+	if err != nil {
+		return process.Config{}, err
+	}
+	if config.Enabled != nil && !*config.Enabled {
+		return process.Config{}, ErrDisabled
+	}
+	parent, err := s.workspaces.Get(ctx, config.WorkspaceID)
+	if err != nil {
+		return process.Config{}, err
+	}
+	dir, err := resolveDirectory(parent.RootPath, config.WorkingDirectory)
+	if err != nil {
+		return process.Config{}, err
+	}
+	return process.Config{WorkspaceID: config.WorkspaceID, Command: config.Command, Args: config.Args, Dir: dir, Env: config.Env}, nil
+}
+
+func (s *Service) Status(ctx context.Context, id uuid.UUID) (process.Status, error) {
+	if _, err := s.configs.Get(ctx, id); err != nil {
+		return process.Status{}, err
+	}
+	return s.supervisor.Status(id), nil
+}
+
+func IsNotFound(err error) bool { return serviceconfig.IsNotFound(err) || workspace.IsNotFound(err) }

# internal/modules/process/service/validation.go

diff --git a/internal/modules/process/service/validation.go b/internal/modules/process/service/validation.go
new file mode 100644
index 0000000..92b17b3
--- /dev/null
+++ b/internal/modules/process/service/validation.go
@@ -0,0 +1,24 @@
+package service
+
+import (
+	"path/filepath"
+	"strings"
+
+	"github.com/prasannashrestha011/devOrbit/internal/process"
+)
+
+func resolveDirectory(root, directory string) (string, error) {
+	if filepath.IsAbs(directory) {
+		return directory, nil
+	}
+	path := filepath.Join(root, directory)
+	resolved, err := filepath.EvalSymlinks(path)
+	if err != nil {
+		return "", process.ErrInvalid
+	}
+	relative, err := filepath.Rel(root, resolved)
+	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
+		return "", process.ErrInvalid
+	}
+	return resolved, nil
+}

# internal/modules/process/service/validation_test.go

diff --git a/internal/modules/process/service/validation_test.go b/internal/modules/process/service/validation_test.go
new file mode 100644
index 0000000..3de964c
--- /dev/null
+++ b/internal/modules/process/service/validation_test.go
@@ -0,0 +1,22 @@
+package service
+
+import (
+	"errors"
+	"path/filepath"
+	"testing"
+
+	"github.com/prasannashrestha011/devOrbit/internal/process"
+)
+
+func TestResolveDirectoryRejectsRelativeEscape(t *testing.T) {
+	root, err := filepath.EvalSymlinks(t.TempDir())
+	if err != nil {
+		t.Fatal(err)
+	}
+	if _, err := resolveDirectory(root, ".."); !errors.Is(err, process.ErrInvalid) {
+		t.Fatalf("relative escape: %v", err)
+	}
+	if got, err := resolveDirectory(root, "."); err != nil || got != root {
+		t.Fatalf("workspace root: %q, %v", got, err)
+	}
+}

# internal/modules/serviceconfiguration/dto/request.go

diff --git a/internal/modules/serviceconfiguration/dto/request.go b/internal/modules/serviceconfiguration/dto/request.go
new file mode 100644
index 0000000..1265433
--- /dev/null
+++ b/internal/modules/serviceconfiguration/dto/request.go
@@ -0,0 +1,117 @@
+package dto
+
+import (
+	"bytes"
+	"encoding/json"
+	"errors"
+	"io"
+	"mime"
+	"net/http"
+)
+
+type CreateInput struct {
+	Name             string
+	Command          string
+	Args             []string
+	WorkingDirectory string
+	Env              map[string]string
+	Enabled          *bool
+	AutoRestart      *bool
+}
+
+type PatchInput struct {
+	Name             *string
+	Command          *string
+	Args             *[]string
+	WorkingDirectory *string
+	Env              *map[string]string
+	Enabled          *bool
+	AutoRestart      *bool
+}
+
+func DecodeCreate(r *http.Request) (CreateInput, error) {
+	patch, err := DecodePatch(r)
+	if err != nil {
+		return CreateInput{}, err
+	}
+	input := CreateInput{Enabled: patch.Enabled, AutoRestart: patch.AutoRestart}
+	if patch.Name != nil {
+		input.Name = *patch.Name
+	}
+	if patch.Command != nil {
+		input.Command = *patch.Command
+	}
+	if patch.Args != nil {
+		input.Args = *patch.Args
+	}
+	if patch.WorkingDirectory != nil {
+		input.WorkingDirectory = *patch.WorkingDirectory
+	}
+	if patch.Env != nil {
+		input.Env = *patch.Env
+	}
+	return input, nil
+}
+
+func DecodePatch(r *http.Request) (PatchInput, error) {
+	var input PatchInput
+	fields, err := readFields(r)
+	if err != nil {
+		return input, err
+	}
+	for field, raw := range fields {
+		switch field {
+		case "name":
+			input.Name = new(string)
+			err = readValue(raw, input.Name)
+		case "command":
+			input.Command = new(string)
+			err = readValue(raw, input.Command)
+		case "args":
+			input.Args = new([]string)
+			err = readValue(raw, input.Args)
+		case "working_directory":
+			input.WorkingDirectory = new(string)
+			err = readValue(raw, input.WorkingDirectory)
+		case "env":
+			input.Env = new(map[string]string)
+			err = readValue(raw, input.Env)
+		case "enabled":
+			input.Enabled = new(bool)
+			err = readValue(raw, input.Enabled)
+		case "auto_restart":
+			input.AutoRestart = new(bool)
+			err = readValue(raw, input.AutoRestart)
+		default:
+			return input, errors.New("unsupported service field")
+		}
+		if err != nil {
+			return input, err
+		}
+	}
+	return input, nil
+}
+
+func readFields(r *http.Request) (map[string]json.RawMessage, error) {
+	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
+	if err != nil || mediaType != "application/json" {
+		return nil, errors.New("content type must be application/json")
+	}
+	decoder := json.NewDecoder(r.Body)
+	var fields map[string]json.RawMessage
+	if err := decoder.Decode(&fields); err != nil || fields == nil {
+		return nil, errors.New("invalid JSON request")
+	}
+	var trailing any
+	if err := decoder.Decode(&trailing); err != io.EOF {
+		return nil, errors.New("invalid JSON request")
+	}
+	return fields, nil
+}
+
+func readValue(raw json.RawMessage, target any) error {
+	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, target) != nil {
+		return errors.New("invalid service field value")
+	}
+	return nil
+}

# internal/modules/serviceconfiguration/dto/response.go

diff --git a/internal/modules/serviceconfiguration/dto/response.go b/internal/modules/serviceconfiguration/dto/response.go
new file mode 100644
index 0000000..0b1166b
--- /dev/null
+++ b/internal/modules/serviceconfiguration/dto/response.go
@@ -0,0 +1,39 @@
+package dto
+
+import (
+	"time"
+
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
+)
+
+type Response struct {
+	ID               uuid.UUID         `json:"id"`
+	WorkspaceID      uuid.UUID         `json:"workspace_id"`
+	Name             string            `json:"name"`
+	Command          string            `json:"command"`
+	Args             []string          `json:"args"`
+	WorkingDirectory string            `json:"working_directory"`
+	Env              map[string]string `json:"env"`
+	Enabled          *bool             `json:"enabled"`
+	AutoRestart      bool              `json:"auto_restart"`
+	CreatedAt        time.Time         `json:"created_at"`
+	UpdatedAt        time.Time         `json:"updated_at"`
+}
+
+func FromModel(item *model.Service) Response {
+	return Response{
+		ID: item.ID, WorkspaceID: item.WorkspaceID, Name: item.Name,
+		Command: item.Command, Args: item.Args, WorkingDirectory: item.WorkingDirectory,
+		Env: item.Env, Enabled: item.Enabled, AutoRestart: item.AutoRestart,
+		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
+	}
+}
+
+func FromModels(items []*model.Service) []Response {
+	responses := make([]Response, 0, len(items))
+	for _, item := range items {
+		responses = append(responses, FromModel(item))
+	}
+	return responses
+}

# internal/modules/serviceconfiguration/dto/response_test.go

diff --git a/internal/modules/serviceconfiguration/dto/response_test.go b/internal/modules/serviceconfiguration/dto/response_test.go
new file mode 100644
index 0000000..d5d9738
--- /dev/null
+++ b/internal/modules/serviceconfiguration/dto/response_test.go
@@ -0,0 +1,27 @@
+package dto
+
+import (
+	"encoding/json"
+	"testing"
+	"time"
+
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
+)
+
+func TestResponsePreservesServiceJSON(t *testing.T) {
+	enabled := false
+	item := &model.Service{
+		ID: uuid.New(), WorkspaceID: uuid.New(), Name: "worker", Command: "go",
+		Args: []string{"run"}, WorkingDirectory: "src", Env: map[string]string{"KEY": "value"},
+		Enabled: &enabled, AutoRestart: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
+	}
+	before, err := json.Marshal(item)
+	if err != nil {
+		t.Fatal(err)
+	}
+	after, err := json.Marshal(FromModel(item))
+	if err != nil || string(before) != string(after) {
+		t.Fatalf("response changed: %s -> %s (%v)", before, after, err)
+	}
+}

# internal/modules/serviceconfiguration/validation/validation.go

diff --git a/internal/modules/serviceconfiguration/validation/validation.go b/internal/modules/serviceconfiguration/validation/validation.go
new file mode 100644
index 0000000..ad5ee1f
--- /dev/null
+++ b/internal/modules/serviceconfiguration/validation/validation.go
@@ -0,0 +1,78 @@
+package validation
+
+import (
+	"fmt"
+	"os"
+	"path/filepath"
+	"strings"
+	"unicode/utf8"
+)
+
+type Error struct{ Field, Message string }
+
+func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
+
+func Name(name string) (string, error) {
+	name = strings.TrimSpace(name)
+	if name == "" || utf8.RuneCountInString(name) > 120 {
+		return "", &Error{Field: "name", Message: "must contain 1 to 120 characters"}
+	}
+	return name, nil
+}
+
+func Command(command string) (string, error) {
+	command = strings.TrimSpace(command)
+	if command == "" || strings.ContainsRune(command, 0) {
+		return "", &Error{Field: "command", Message: "must be a nonempty executable without null bytes"}
+	}
+	return command, nil
+}
+
+func Args(args []string) error {
+	for _, arg := range args {
+		if strings.ContainsRune(arg, 0) {
+			return &Error{Field: "args", Message: "must not contain null bytes"}
+		}
+	}
+	return nil
+}
+
+func Env(env map[string]string) error {
+	for key, value := range env {
+		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, 0) {
+			return &Error{Field: "env", Message: "variable names must be nonempty and cannot contain '=' or null bytes; values cannot contain null bytes"}
+		}
+	}
+	return nil
+}
+
+func Directory(root, directory string) (string, error) {
+	if strings.ContainsRune(directory, 0) {
+		return "", &Error{Field: "working_directory", Message: "invalid directory path"}
+	}
+	absolute := filepath.IsAbs(directory)
+	path := directory
+	if !absolute {
+		path = filepath.Join(root, directory)
+	}
+	canonical, err := filepath.EvalSymlinks(path)
+	if err != nil {
+		return "", &Error{Field: "working_directory", Message: "must be an accessible existing directory"}
+	}
+	info, err := os.Stat(canonical)
+	if err != nil || !info.IsDir() {
+		return "", &Error{Field: "working_directory", Message: "must be an accessible existing directory"}
+	}
+	canonical = filepath.Clean(canonical)
+	relative, err := filepath.Rel(root, canonical)
+	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
+		if relative == "." {
+			return "", nil
+		}
+		return relative, nil
+	}
+	if !absolute {
+		return "", &Error{Field: "working_directory", Message: "relative directory must remain inside the workspace"}
+	}
+	return canonical, nil
+}

# internal/modules/workspace/controller/orchestration.go

diff --git a/internal/modules/workspace/controller/orchestration.go b/internal/modules/workspace/controller/orchestration.go
new file mode 100644
index 0000000..0989787
--- /dev/null
+++ b/internal/modules/workspace/controller/orchestration.go
@@ -0,0 +1,90 @@
+package controller
+
+import (
+	"encoding/json"
+	"errors"
+	"net/http"
+
+	"github.com/go-chi/chi/v5"
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
+	"github.com/prasannashrestha011/devOrbit/internal/orchestration"
+	"github.com/prasannashrestha011/devOrbit/internal/process"
+)
+
+type OrchestrationController struct{ orchestrator *orchestration.Orchestrator }
+
+func NewOrchestration(o *orchestration.Orchestrator) *OrchestrationController {
+	return &OrchestrationController{orchestrator: o}
+}
+
+func (c *OrchestrationController) Start(w http.ResponseWriter, r *http.Request) {
+	id, ok := orchestrationID(w, r)
+	if !ok {
+		return
+	}
+	result, err := c.orchestrator.StartWorkspace(r.Context(), id)
+	if err != nil {
+		orchestrationError(w, err)
+		return
+	}
+	orchestrationJSON(w, http.StatusOK, result)
+}
+
+func (c *OrchestrationController) Stop(w http.ResponseWriter, r *http.Request) {
+	id, ok := orchestrationID(w, r)
+	if !ok {
+		return
+	}
+	result, err := c.orchestrator.StopWorkspace(r.Context(), id)
+	if err != nil {
+		orchestrationError(w, err)
+		return
+	}
+	orchestrationJSON(w, http.StatusOK, result)
+}
+
+func (c *OrchestrationController) Status(w http.ResponseWriter, r *http.Request) {
+	id, ok := orchestrationID(w, r)
+	if !ok {
+		return
+	}
+	result, err := c.orchestrator.GetWorkspaceStatus(r.Context(), id)
+	if err != nil {
+		orchestrationError(w, err)
+		return
+	}
+	orchestrationJSON(w, http.StatusOK, result)
+}
+
+func orchestrationID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
+	id, err := uuid.Parse(chi.URLParam(r, "id"))
+	if err != nil || id == uuid.Nil {
+		orchestrationError(w, orchestration.ErrInvalidID)
+		return uuid.Nil, false
+	}
+	return id, true
+}
+
+func orchestrationError(w http.ResponseWriter, err error) {
+	switch {
+	case errors.Is(err, orchestration.ErrInvalidID):
+		orchestrationErrorJSON(w, http.StatusBadRequest, "invalid_request", "invalid id")
+	case service.IsNotFound(err):
+		orchestrationErrorJSON(w, http.StatusNotFound, "not_found", "workspace not found")
+	case errors.Is(err, orchestration.ErrConflict), errors.Is(err, process.ErrClosed):
+		orchestrationErrorJSON(w, http.StatusConflict, "conflict", "workspace lifecycle operation is already active or shutting down")
+	default:
+		orchestrationErrorJSON(w, http.StatusInternalServerError, "internal_error", "internal server error")
+	}
+}
+
+func orchestrationJSON(w http.ResponseWriter, status int, value any) {
+	w.Header().Set("Content-Type", "application/json; charset=utf-8")
+	w.WriteHeader(status)
+	_ = json.NewEncoder(w).Encode(value)
+}
+
+func orchestrationErrorJSON(w http.ResponseWriter, status int, code, message string) {
+	orchestrationJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
+}

# internal/modules/workspace/controller/orchestration_test.go

diff --git a/internal/modules/workspace/controller/orchestration_test.go b/internal/modules/workspace/controller/orchestration_test.go
new file mode 100644
index 0000000..bb3fb2b
--- /dev/null
+++ b/internal/modules/workspace/controller/orchestration_test.go
@@ -0,0 +1,119 @@
+package controller
+
+import (
+	"context"
+	"encoding/json"
+	"io"
+	"log/slog"
+	"net/http"
+	"net/http/httptest"
+	"strings"
+	"testing"
+
+	"github.com/go-chi/chi/v5"
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/api"
+	configModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
+	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
+	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
+	"github.com/prasannashrestha011/devOrbit/internal/orchestration"
+	"github.com/prasannashrestha011/devOrbit/internal/process"
+)
+
+type orchestrationWorkspaces map[uuid.UUID]*workspaceModel.Workspace
+
+func (s orchestrationWorkspaces) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
+	if item := s[id]; item != nil {
+		return item, nil
+	}
+	return nil, workspaceRepo.ErrNotFound
+}
+
+type orchestrationServices []*configModel.Service
+
+func (s orchestrationServices) List(_ context.Context, id uuid.UUID, _, _ int) ([]*configModel.Service, error) {
+	items := make([]*configModel.Service, 0)
+	for _, item := range s {
+		if item.WorkspaceID == id {
+			items = append(items, item)
+		}
+	}
+	return items, nil
+}
+
+type orchestrationRuntime map[uuid.UUID]process.Status
+
+func (s orchestrationRuntime) Status(id uuid.UUID) process.Status {
+	if status, ok := s[id]; ok {
+		return status
+	}
+	return process.Status{ServiceID: id, State: process.Stopped}
+}
+func (orchestrationRuntime) Closed() bool { return false }
+
+type orchestrationLifecycle struct{ runtime orchestrationRuntime }
+
+func (s orchestrationLifecycle) Start(_ context.Context, id uuid.UUID) (process.Status, error) {
+	status := process.Status{ServiceID: id, State: process.Running}
+	s.runtime[id] = status
+	return status, nil
+}
+func (s orchestrationLifecycle) Stop(_ context.Context, id uuid.UUID) (process.Status, error) {
+	status := process.Status{ServiceID: id, State: process.Stopped}
+	s.runtime[id] = status
+	return status, nil
+}
+
+func TestOrchestrationAPI(t *testing.T) {
+	id, serviceID := uuid.New(), uuid.New()
+	workspaces := orchestrationWorkspaces{id: {ID: id, Name: "Web"}}
+	services := orchestrationServices{{ID: serviceID, WorkspaceID: id, Name: "API"}}
+	runtime := orchestrationRuntime{}
+	c := NewOrchestration(orchestration.New(workspaces, services, orchestrationLifecycle{runtime}, runtime))
+	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
+		r.Post("/workspaces/{id}/start", c.Start)
+		r.Post("/workspaces/{id}/stop", c.Stop)
+		r.Get("/workspaces/{id}/status", c.Status)
+	})
+	request := func(method, path, token string) *httptest.ResponseRecorder {
+		t.Helper()
+		req := httptest.NewRequest(method, path, nil)
+		if token != "" {
+			req.Header.Set("Authorization", "Bearer "+token)
+		}
+		w := httptest.NewRecorder()
+		handler.ServeHTTP(w, req)
+		return w
+	}
+	base := "/api/v1/workspaces/" + id.String()
+	for _, path := range []string{base + "/start", base + "/stop", base + "/status"} {
+		method := http.MethodPost
+		if strings.HasSuffix(path, "/status") {
+			method = http.MethodGet
+		}
+		if w := request(method, path, ""); w.Code != http.StatusUnauthorized {
+			t.Fatalf("unauthorized %s: %d", path, w.Code)
+		}
+		if w := request(method, "/api/v1/workspaces/bad"+path[strings.LastIndex(path, "/"):], "token"); w.Code != http.StatusBadRequest {
+			t.Fatalf("bad ID %s: %d %s", path, w.Code, w.Body.String())
+		}
+		if w := request(method, "/api/v1/workspaces/"+uuid.NewString()+path[strings.LastIndex(path, "/"):], "token"); w.Code != http.StatusNotFound {
+			t.Fatalf("missing workspace %s: %d %s", path, w.Code, w.Body.String())
+		}
+	}
+	if w := request(http.MethodPost, base+"/start", "token"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"outcome":"started"`) {
+		t.Fatalf("start: %d %s", w.Code, w.Body.String())
+	}
+	w := request(http.MethodGet, base+"/status", "token")
+	var status orchestration.WorkspaceStatus
+	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil || w.Code != http.StatusOK || status.AggregateState != "running" || status.Services[0].ServiceID != serviceID {
+		t.Fatalf("status: %d %s %v", w.Code, w.Body.String(), err)
+	}
+	if w := request(http.MethodPost, base+"/stop", "token"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"outcome":"stopped"`) {
+		t.Fatalf("stop: %d %s", w.Code, w.Body.String())
+	}
+	runtime[serviceID] = process.Status{ServiceID: serviceID, State: process.Failed, LastError: "private execution detail"}
+	if w := request(http.MethodGet, base+"/status", "token"); w.Code != http.StatusOK || strings.Contains(w.Body.String(), "private execution detail") || !strings.Contains(w.Body.String(), "service execution failed") {
+		t.Fatalf("status leaked internal error: %d %s", w.Code, w.Body.String())
+	}
+}

# internal/modules/workspace/dto/request.go

diff --git a/internal/modules/workspace/dto/request.go b/internal/modules/workspace/dto/request.go
new file mode 100644
index 0000000..5783cd6
--- /dev/null
+++ b/internal/modules/workspace/dto/request.go
@@ -0,0 +1,75 @@
+package dto
+
+import (
+	"bytes"
+	"encoding/json"
+	"errors"
+	"io"
+	"mime"
+	"net/http"
+)
+
+type CreateInput struct {
+	Name        string `json:"name"`
+	Description string `json:"description"`
+	RootPath    string `json:"root_path"`
+}
+
+type PatchInput struct {
+	Name        *string
+	Description *string
+	RootPath    *string
+}
+
+func DecodeCreate(r *http.Request) (CreateInput, error) {
+	var input CreateInput
+	if err := decode(r, &input); err != nil {
+		return input, err
+	}
+	return input, nil
+}
+
+func DecodePatch(r *http.Request) (PatchInput, error) {
+	var input PatchInput
+	var fields map[string]json.RawMessage
+	if err := decode(r, &fields); err != nil || fields == nil {
+		return input, errors.New("invalid JSON request")
+	}
+	for field, raw := range fields {
+		if bytes.Equal(raw, []byte("null")) {
+			return input, errors.New("null field value is not supported")
+		}
+		var value string
+		if err := json.Unmarshal(raw, &value); err != nil {
+			return input, errors.New("fields must be strings")
+		}
+		switch field {
+		case "name":
+			input.Name = &value
+		case "description":
+			input.Description = &value
+		case "root_path":
+			input.RootPath = &value
+		default:
+			return input, errors.New("unsupported workspace field")
+		}
+	}
+	return input, nil
+}
+
+func decode(r *http.Request, target any) error {
+	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
+	if err != nil || mediaType != "application/json" {
+		return errors.New("content type must be application/json")
+	}
+	decoder := json.NewDecoder(r.Body)
+	decoder.DisallowUnknownFields()
+	if err := decoder.Decode(target); err != nil {
+		return err
+	}
+	var trailing any
+	if err := decoder.Decode(&trailing); err != io.EOF {
+		return errors.New("trailing JSON value")
+	}
+	return nil
+}

# internal/modules/workspace/dto/response.go

diff --git a/internal/modules/workspace/dto/response.go b/internal/modules/workspace/dto/response.go
new file mode 100644
index 0000000..28ea750
--- /dev/null
+++ b/internal/modules/workspace/dto/response.go
@@ -0,0 +1,32 @@
+package dto
+
+import (
+	"time"
+
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
+)
+
+type Response struct {
+	ID          uuid.UUID `json:"id"`
+	Name        string    `json:"name"`
+	Description string    `json:"description"`
+	RootPath    string    `json:"root_path"`
+	CreatedAt   time.Time `json:"created_at"`
+	UpdatedAt   time.Time `json:"updated_at"`
+}
+
+func FromModel(item *model.Workspace) Response {
+	return Response{
+		ID: item.ID, Name: item.Name, Description: item.Description,
+		RootPath: item.RootPath, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
+	}
+}
+
+func FromModels(items []*model.Workspace) []Response {
+	responses := make([]Response, 0, len(items))
+	for _, item := range items {
+		responses = append(responses, FromModel(item))
+	}
+	return responses
+}

# internal/modules/workspace/dto/response_test.go

diff --git a/internal/modules/workspace/dto/response_test.go b/internal/modules/workspace/dto/response_test.go
new file mode 100644
index 0000000..e2ae12d
--- /dev/null
+++ b/internal/modules/workspace/dto/response_test.go
@@ -0,0 +1,22 @@
+package dto
+
+import (
+	"encoding/json"
+	"testing"
+	"time"
+
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
+)
+
+func TestResponsePreservesWorkspaceJSON(t *testing.T) {
+	item := &model.Workspace{ID: uuid.New(), Name: "project", Description: "notes", RootPath: "/project", CreatedAt: time.Now(), UpdatedAt: time.Now()}
+	before, err := json.Marshal(item)
+	if err != nil {
+		t.Fatal(err)
+	}
+	after, err := json.Marshal(FromModel(item))
+	if err != nil || string(before) != string(after) {
+		t.Fatalf("response changed: %s -> %s (%v)", before, after, err)
+	}
+}

# internal/modules/workspace/validation/validation.go

diff --git a/internal/modules/workspace/validation/validation.go b/internal/modules/workspace/validation/validation.go
new file mode 100644
index 0000000..b76593e
--- /dev/null
+++ b/internal/modules/workspace/validation/validation.go
@@ -0,0 +1,50 @@
+package validation
+
+import (
+	"fmt"
+	"os"
+	"path/filepath"
+	"strings"
+	"unicode/utf8"
+)
+
+type Error struct {
+	Field   string
+	Message string
+}
+
+func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
+
+func Name(name string) (string, error) {
+	name = strings.TrimSpace(name)
+	if name == "" || utf8.RuneCountInString(name) > 120 {
+		return "", &Error{Field: "name", Message: "must contain 1 to 120 characters"}
+	}
+	return name, nil
+}
+
+func Description(description string) error {
+	if utf8.RuneCountInString(description) > 1000 {
+		return &Error{Field: "description", Message: "must contain at most 1000 characters"}
+	}
+	return nil
+}
+
+func Root(root string) (string, error) {
+	if strings.TrimSpace(root) == "" {
+		return "", &Error{Field: "root_path", Message: "must be an existing directory"}
+	}
+	absolute, err := filepath.Abs(root)
+	if err != nil {
+		return "", &Error{Field: "root_path", Message: "invalid directory path"}
+	}
+	canonical, err := filepath.EvalSymlinks(absolute)
+	if err != nil {
+		return "", &Error{Field: "root_path", Message: "must be an accessible existing directory"}
+	}
+	info, err := os.Stat(canonical)
+	if err != nil || !info.IsDir() {
+		return "", &Error{Field: "root_path", Message: "must be an accessible existing directory"}
+	}
+	return filepath.Clean(canonical), nil
+}

# internal/orchestration/orchestrator.go

diff --git a/internal/orchestration/orchestrator.go b/internal/orchestration/orchestrator.go
new file mode 100644
index 0000000..6fe1753
--- /dev/null
+++ b/internal/orchestration/orchestrator.go
@@ -0,0 +1,290 @@
+package orchestration
+
+import (
+	"context"
+	"errors"
+	"sort"
+	"sync"
+
+	"github.com/google/uuid"
+	processService "github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
+	workspace "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
+	"github.com/prasannashrestha011/devOrbit/internal/process"
+)
+
+var ErrConflict = errors.New("workspace lifecycle operation is already active")
+var ErrInvalidID = errors.New("invalid workspace id")
+
+type WorkspaceStore interface {
+	Get(context.Context, uuid.UUID) (*workspace.Workspace, error)
+}
+
+type ServiceStore interface {
+	List(context.Context, uuid.UUID, int, int) ([]*model.Service, error)
+}
+
+type Lifecycle interface {
+	Start(context.Context, uuid.UUID) (process.Status, error)
+	Stop(context.Context, uuid.UUID) (process.Status, error)
+}
+
+type Runtime interface {
+	Status(uuid.UUID) process.Status
+	Closed() bool
+}
+
+type ServiceResult struct {
+	ServiceID   uuid.UUID `json:"service_id"`
+	ServiceName string    `json:"service_name"`
+	Outcome     string    `json:"outcome"`
+	PID         *int      `json:"pid"`
+	Error       *string   `json:"error"`
+}
+
+type OperationResult struct {
+	WorkspaceID uuid.UUID       `json:"workspace_id"`
+	Operation   string          `json:"operation"`
+	Total       int             `json:"total"`
+	Succeeded   int             `json:"succeeded"`
+	Failed      int             `json:"failed"`
+	Skipped     int             `json:"skipped"`
+	Results     []ServiceResult `json:"results"`
+}
+
+type ServiceStatus struct {
+	Name    string `json:"name"`
+	Enabled bool   `json:"enabled"`
+	process.Status
+}
+
+type WorkspaceStatus struct {
+	WorkspaceID      uuid.UUID       `json:"workspace_id"`
+	WorkspaceName    string          `json:"workspace_name"`
+	AggregateState   string          `json:"aggregate_state"`
+	TotalServices    int             `json:"total_services"`
+	EnabledServices  int             `json:"enabled_services"`
+	RunningServices  int             `json:"running_services"`
+	StoppedServices  int             `json:"stopped_services"`
+	FailedServices   int             `json:"failed_services"`
+	StartingServices int             `json:"starting_services"`
+	StoppingServices int             `json:"stopping_services"`
+	ExitedServices   int             `json:"exited_services"`
+	Services         []ServiceStatus `json:"services"`
+}
+
+type Orchestrator struct {
+	workspaces WorkspaceStore
+	services   ServiceStore
+	lifecycle  Lifecycle
+	runtime    Runtime
+	mu         sync.Mutex
+	active     map[uuid.UUID]bool
+}
+
+func New(workspaces WorkspaceStore, services ServiceStore, lifecycle Lifecycle, runtime Runtime) *Orchestrator {
+	return &Orchestrator{workspaces: workspaces, services: services, lifecycle: lifecycle, runtime: runtime, active: make(map[uuid.UUID]bool)}
+}
+
+func (o *Orchestrator) reserve(id uuid.UUID) error {
+	o.mu.Lock()
+	defer o.mu.Unlock()
+	if o.active[id] {
+		return ErrConflict
+	}
+	o.active[id] = true
+	return nil
+}
+
+func (o *Orchestrator) release(id uuid.UUID) {
+	o.mu.Lock()
+	delete(o.active, id)
+	o.mu.Unlock()
+}
+
+func (o *Orchestrator) load(ctx context.Context, id uuid.UUID) (*workspace.Workspace, []*model.Service, error) {
+	if id == uuid.Nil {
+		return nil, nil, ErrInvalidID
+	}
+	w, err := o.workspaces.Get(ctx, id)
+	if err != nil {
+		return nil, nil, err
+	}
+	services := make([]*model.Service, 0)
+	for offset := 0; ; offset += 100 {
+		page, err := o.services.List(ctx, id, 100, offset)
+		if err != nil {
+			return nil, nil, err
+		}
+		services = append(services, page...)
+		if len(page) < 100 {
+			break
+		}
+	}
+	sort.Slice(services, func(i, j int) bool {
+		if !services[i].CreatedAt.Equal(services[j].CreatedAt) {
+			return services[i].CreatedAt.Before(services[j].CreatedAt)
+		}
+		return services[i].ID.String() < services[j].ID.String()
+	})
+	return w, services, nil
+}
+
+func enabled(s *model.Service) bool { return s.Enabled == nil || *s.Enabled }
+
+func (o *Orchestrator) StartWorkspace(ctx context.Context, id uuid.UUID) (OperationResult, error) {
+	if id == uuid.Nil {
+		return OperationResult{}, ErrInvalidID
+	}
+	if o.runtime.Closed() {
+		return OperationResult{}, process.ErrClosed
+	}
+	if err := o.reserve(id); err != nil {
+		return OperationResult{}, err
+	}
+	defer o.release(id)
+	_, services, err := o.load(ctx, id)
+	if err != nil {
+		return OperationResult{}, err
+	}
+	result := OperationResult{WorkspaceID: id, Operation: "start", Total: len(services), Results: make([]ServiceResult, 0, len(services))}
+	for _, item := range services {
+		if err := ctx.Err(); err != nil {
+			return OperationResult{}, err
+		}
+		entry := ServiceResult{ServiceID: item.ID, ServiceName: item.Name}
+		state := o.runtime.Status(item.ID)
+		switch {
+		case !enabled(item):
+			entry.Outcome = "disabled"
+		case state.State == process.Running:
+			entry.Outcome, entry.PID = "already_running", state.PID
+		case state.State == process.Starting || state.State == process.Stopping:
+			entry.Outcome = "conflict"
+		case o.runtime.Closed():
+			entry.Outcome = "conflict"
+		default:
+			status, err := o.lifecycle.Start(ctx, item.ID)
+			switch {
+			case err == nil:
+				entry.Outcome, entry.PID = "started", status.PID
+			case errors.Is(err, process.ErrActive), errors.Is(err, process.ErrClosed):
+				entry.Outcome = "conflict"
+			case errors.Is(err, processService.ErrDisabled):
+				entry.Outcome = "disabled"
+			default:
+				entry.Outcome = "failed"
+			}
+		}
+		result.add(entry)
+	}
+	return result, nil
+}
+
+func (o *Orchestrator) StopWorkspace(ctx context.Context, id uuid.UUID) (OperationResult, error) {
+	if id == uuid.Nil {
+		return OperationResult{}, ErrInvalidID
+	}
+	if err := o.reserve(id); err != nil {
+		return OperationResult{}, err
+	}
+	defer o.release(id)
+	_, services, err := o.load(ctx, id)
+	if err != nil {
+		return OperationResult{}, err
+	}
+	result := OperationResult{WorkspaceID: id, Operation: "stop", Total: len(services), Results: make([]ServiceResult, 0, len(services))}
+	for i := len(services) - 1; i >= 0; i-- {
+		if err := ctx.Err(); err != nil {
+			return OperationResult{}, err
+		}
+		item := services[i]
+		entry := ServiceResult{ServiceID: item.ID, ServiceName: item.Name}
+		state := o.runtime.Status(item.ID)
+		switch state.State {
+		case process.Starting:
+			entry.Outcome = "conflict"
+		case process.Running, process.Stopping:
+			status, err := o.lifecycle.Stop(ctx, item.ID)
+			switch {
+			case err == nil && status.State == process.Stopped:
+				entry.Outcome = "stopped"
+			case err == nil && (status.State == process.Exited || status.State == process.Failed):
+				entry.Outcome = "already_stopped"
+			case errors.Is(err, process.ErrActive), errors.Is(err, process.ErrClosed):
+				entry.Outcome = "conflict"
+			default:
+				entry.Outcome = "failed"
+			}
+		default:
+			entry.Outcome = "already_stopped"
+		}
+		result.add(entry)
+	}
+	return result, nil
+}
+
+func (r *OperationResult) add(entry ServiceResult) {
+	switch entry.Outcome {
+	case "started", "stopped":
+		r.Succeeded++
+	case "already_running", "already_stopped", "disabled":
+		r.Skipped++
+	default:
+		r.Failed++
+		message := "service operation failed"
+		if entry.Outcome == "conflict" {
+			message = "service lifecycle operation is already active or shutting down"
+		}
+		entry.Error = &message
+	}
+	r.Results = append(r.Results, entry)
+}
+
+func (o *Orchestrator) GetWorkspaceStatus(ctx context.Context, id uuid.UUID) (WorkspaceStatus, error) {
+	w, services, err := o.load(ctx, id)
+	if err != nil {
+		return WorkspaceStatus{}, err
+	}
+	result := WorkspaceStatus{WorkspaceID: id, WorkspaceName: w.Name, TotalServices: len(services), Services: make([]ServiceStatus, 0, len(services))}
+	enabledRunning := 0
+	for _, item := range services {
+		status := o.runtime.Status(item.ID)
+		if status.LastError != "" {
+			status.LastError = "service execution failed"
+		}
+		isEnabled := enabled(item)
+		result.Services = append(result.Services, ServiceStatus{Name: item.Name, Enabled: isEnabled, Status: status})
+		if isEnabled {
+			result.EnabledServices++
+		}
+		switch status.State {
+		case process.Running:
+			result.RunningServices++
+			if isEnabled {
+				enabledRunning++
+			}
+		case process.Starting:
+			result.StartingServices++
+		case process.Stopping:
+			result.StoppingServices++
+		case process.Failed:
+			result.FailedServices++
+		case process.Exited:
+			result.ExitedServices++
+		default:
+			result.StoppedServices++
+		}
+	}
+	switch {
+	case result.StartingServices+result.StoppingServices > 0:
+		result.AggregateState = "transitioning"
+	case result.EnabledServices > 0 && enabledRunning == result.EnabledServices:
+		result.AggregateState = "running"
+	case result.RunningServices > 0:
+		result.AggregateState = "partial"
+	default:
+		result.AggregateState = "idle"
+	}
+	return result, nil
+}

# internal/orchestration/orchestrator_test.go

diff --git a/internal/orchestration/orchestrator_test.go b/internal/orchestration/orchestrator_test.go
new file mode 100644
index 0000000..430a08b
--- /dev/null
+++ b/internal/orchestration/orchestrator_test.go
@@ -0,0 +1,320 @@
+package orchestration
+
+import (
+	"context"
+	"errors"
+	"os"
+	"path/filepath"
+	"sync"
+	"testing"
+	"time"
+
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
+	workspace "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
+	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
+	"github.com/prasannashrestha011/devOrbit/internal/process"
+)
+
+type fakeWorkspaces map[uuid.UUID]*workspace.Workspace
+
+func (s fakeWorkspaces) Get(_ context.Context, id uuid.UUID) (*workspace.Workspace, error) {
+	if item := s[id]; item != nil {
+		return item, nil
+	}
+	return nil, workspaceRepo.ErrNotFound
+}
+
+type fakeServices []*model.Service
+
+func (s fakeServices) List(_ context.Context, id uuid.UUID, limit, offset int) ([]*model.Service, error) {
+	items := make([]*model.Service, 0)
+	for _, item := range s {
+		if item.WorkspaceID == id {
+			items = append(items, item)
+		}
+	}
+	if offset >= len(items) {
+		return nil, nil
+	}
+	end := min(offset+limit, len(items))
+	return items[offset:end], nil
+}
+
+type fakeRuntime struct {
+	mu       sync.Mutex
+	statuses map[uuid.UUID]process.Status
+	closed   bool
+}
+
+func (r *fakeRuntime) Status(id uuid.UUID) process.Status {
+	r.mu.Lock()
+	defer r.mu.Unlock()
+	if status, ok := r.statuses[id]; ok {
+		return status
+	}
+	return process.Status{ServiceID: id, State: process.Stopped}
+}
+
+func (r *fakeRuntime) Closed() bool {
+	r.mu.Lock()
+	defer r.mu.Unlock()
+	return r.closed
+}
+
+type fakeLifecycle struct {
+	runtime  *fakeRuntime
+	mu       sync.Mutex
+	calls    []uuid.UUID
+	startErr map[uuid.UUID]error
+	stopErr  map[uuid.UUID]error
+	gate     chan struct{}
+	entered  chan struct{}
+}
+
+func (l *fakeLifecycle) Start(_ context.Context, id uuid.UUID) (process.Status, error) {
+	l.mu.Lock()
+	l.calls = append(l.calls, id)
+	l.mu.Unlock()
+	if l.entered != nil {
+		l.entered <- struct{}{}
+	}
+	if l.gate != nil {
+		<-l.gate
+	}
+	if err := l.startErr[id]; err != nil {
+		return process.Status{}, err
+	}
+	status := process.Status{ServiceID: id, State: process.Running}
+	l.runtime.mu.Lock()
+	l.runtime.statuses[id] = status
+	l.runtime.mu.Unlock()
+	return status, nil
+}
+
+func (l *fakeLifecycle) Stop(_ context.Context, id uuid.UUID) (process.Status, error) {
+	l.mu.Lock()
+	l.calls = append(l.calls, id)
+	l.mu.Unlock()
+	if err := l.stopErr[id]; err != nil {
+		return process.Status{}, err
+	}
+	status := process.Status{ServiceID: id, State: process.Stopped}
+	l.runtime.mu.Lock()
+	l.runtime.statuses[id] = status
+	l.runtime.mu.Unlock()
+	return status, nil
+}
+
+func fixture() (uuid.UUID, fakeWorkspaces, fakeServices, *fakeRuntime, *fakeLifecycle) {
+	id := uuid.New()
+	other := uuid.New()
+	trueValue, falseValue := true, false
+	created := time.Now().Add(-time.Hour)
+	services := fakeServices{
+		{ID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), WorkspaceID: id, Name: "third", Enabled: &trueValue, CreatedAt: created.Add(time.Second)},
+		{ID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), WorkspaceID: id, Name: "second", Enabled: &falseValue, CreatedAt: created},
+		{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WorkspaceID: id, Name: "first", Enabled: &trueValue, CreatedAt: created},
+		{ID: uuid.New(), WorkspaceID: other, Name: "other", Enabled: &trueValue},
+	}
+	runtime := &fakeRuntime{statuses: make(map[uuid.UUID]process.Status)}
+	lifecycle := &fakeLifecycle{runtime: runtime, startErr: make(map[uuid.UUID]error), stopErr: make(map[uuid.UUID]error)}
+	return id, fakeWorkspaces{id: {ID: id, Name: "workspace"}, other: {ID: other, Name: "other"}}, services, runtime, lifecycle
+}
+
+func TestStartStopAndStatus(t *testing.T) {
+	id, workspaces, services, runtime, lifecycle := fixture()
+	first, second, third := services[2].ID, services[1].ID, services[0].ID
+	lifecycle.startErr[third] = errors.New("secret database error")
+	o := New(workspaces, services, lifecycle, runtime)
+	started, err := o.StartWorkspace(context.Background(), id)
+	if err != nil || started.Total != 3 || started.Succeeded != 1 || started.Failed != 1 || started.Skipped != 1 {
+		t.Fatalf("start: %+v %v", started, err)
+	}
+	if started.Results[0].ServiceID != first || started.Results[0].Outcome != "started" || started.Results[1].ServiceID != second || started.Results[1].Outcome != "disabled" || started.Results[2].ServiceID != third || started.Results[2].Outcome != "failed" || *started.Results[2].Error == "secret database error" {
+		t.Fatalf("outcomes or order: %+v", started.Results)
+	}
+	if len(lifecycle.calls) != 2 || lifecycle.calls[0] != first || lifecycle.calls[1] != third {
+		t.Fatalf("start calls: %v", lifecycle.calls)
+	}
+	status, err := o.GetWorkspaceStatus(context.Background(), id)
+	if err != nil || status.AggregateState != "partial" || status.RunningServices != 1 || status.EnabledServices != 2 || len(status.Services) != 3 {
+		t.Fatalf("status: %+v %v", status, err)
+	}
+	started, err = o.StartWorkspace(context.Background(), id)
+	if err != nil || started.Results[0].Outcome != "already_running" {
+		t.Fatalf("repeated start: %+v %v", started, err)
+	}
+	runtime.statuses[second] = process.Status{ServiceID: second, State: process.Running}
+	stopped, err := o.StopWorkspace(context.Background(), id)
+	if err != nil || stopped.Succeeded != 2 || stopped.Skipped != 1 || stopped.Results[0].ServiceID != third || stopped.Results[1].ServiceID != second || stopped.Results[2].ServiceID != first {
+		t.Fatalf("stop: %+v %v", stopped, err)
+	}
+	if len(lifecycle.calls) != 5 || lifecycle.calls[3] != second || lifecycle.calls[4] != first {
+		t.Fatalf("stop order: %v", lifecycle.calls)
+	}
+	status, _ = o.GetWorkspaceStatus(context.Background(), id)
+	if status.AggregateState != "idle" || status.StoppedServices != 3 {
+		t.Fatalf("idle: %+v", status)
+	}
+}
+
+func TestConflictAndIndependentWorkspace(t *testing.T) {
+	id, workspaces, services, runtime, lifecycle := fixture()
+	lifecycle.gate = make(chan struct{})
+	lifecycle.entered = make(chan struct{}, 1)
+	o := New(workspaces, services, lifecycle, runtime)
+	done := make(chan error, 1)
+	go func() { _, err := o.StartWorkspace(context.Background(), id); done <- err }()
+	<-lifecycle.entered
+	if _, err := o.StartWorkspace(context.Background(), id); !errors.Is(err, ErrConflict) {
+		t.Fatalf("same workspace start conflict: %v", err)
+	}
+	if _, err := o.StopWorkspace(context.Background(), id); !errors.Is(err, ErrConflict) {
+		t.Fatalf("same workspace stop conflict: %v", err)
+	}
+	other := services[3].WorkspaceID
+	if _, err := o.GetWorkspaceStatus(context.Background(), other); err != nil {
+		t.Fatalf("independent workspace status: %v", err)
+	}
+	close(lifecycle.gate)
+	if err := <-done; err != nil {
+		t.Fatal(err)
+	}
+	if _, err := o.StartWorkspace(context.Background(), id); err != nil {
+		t.Fatalf("reservation leaked: %v", err)
+	}
+	runtime.mu.Lock()
+	runtime.closed = true
+	runtime.mu.Unlock()
+	if _, err := o.StartWorkspace(context.Background(), id); !errors.Is(err, process.ErrClosed) {
+		t.Fatalf("shutdown conflict: %v", err)
+	}
+}
+
+func TestAggregationAndValidation(t *testing.T) {
+	id, workspaces, services, runtime, lifecycle := fixture()
+	o := New(workspaces, services, lifecycle, runtime)
+	if _, err := o.StartWorkspace(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidID) {
+		t.Fatalf("nil ID: %v", err)
+	}
+	if _, err := o.StopWorkspace(context.Background(), uuid.New()); !errors.Is(err, workspaceRepo.ErrNotFound) {
+		t.Fatalf("missing workspace: %v", err)
+	}
+	runtime.statuses[services[0].ID] = process.Status{ServiceID: services[0].ID, State: process.Starting}
+	status, _ := o.GetWorkspaceStatus(context.Background(), id)
+	if status.AggregateState != "transitioning" || status.StartingServices != 1 {
+		t.Fatalf("transitioning: %+v", status)
+	}
+	runtime.statuses[services[0].ID] = process.Status{ServiceID: services[0].ID, State: process.Running}
+	runtime.statuses[services[2].ID] = process.Status{ServiceID: services[2].ID, State: process.Running}
+	status, _ = o.GetWorkspaceStatus(context.Background(), id)
+	if status.AggregateState != "running" || status.RunningServices != 2 {
+		t.Fatalf("running: %+v", status)
+	}
+	other := services[3].WorkspaceID
+	status, _ = o.GetWorkspaceStatus(context.Background(), other)
+	if status.AggregateState != "idle" {
+		t.Fatalf("empty runtime: %+v", status)
+	}
+}
+
+func TestAllServicesAreLoaded(t *testing.T) {
+	id := uuid.New()
+	yes := true
+	services := make(fakeServices, 205)
+	for i := range services {
+		services[i] = &model.Service{ID: uuid.New(), WorkspaceID: id, Enabled: &yes}
+	}
+	runtime := &fakeRuntime{statuses: make(map[uuid.UUID]process.Status)}
+	lifecycle := &fakeLifecycle{runtime: runtime}
+	o := New(fakeWorkspaces{id: {ID: id}}, services, lifecycle, runtime)
+	result, err := o.StartWorkspace(context.Background(), id)
+	if err != nil || result.Total != 205 || result.Succeeded != 205 || len(result.Results) != 205 {
+		t.Fatalf("paginated start: %+v %v", result, err)
+	}
+}
+
+func TestLifecycleConflictsAndStopFailure(t *testing.T) {
+	id, workspaces, services, runtime, lifecycle := fixture()
+	first, third := services[2].ID, services[0].ID
+	runtime.statuses[first] = process.Status{ServiceID: first, State: process.Starting}
+	runtime.statuses[third] = process.Status{ServiceID: third, State: process.Stopping}
+	o := New(workspaces, services, lifecycle, runtime)
+	started, err := o.StartWorkspace(context.Background(), id)
+	if err != nil || started.Failed != 2 || started.Results[0].Outcome != "conflict" || started.Results[2].Outcome != "conflict" {
+		t.Fatalf("start conflicts: %+v %v", started, err)
+	}
+	runtime.statuses[first] = process.Status{ServiceID: first, State: process.Running}
+	lifecycle.stopErr[first] = errors.New("private stop detail")
+	lifecycle.stopErr[third] = process.ErrActive
+	stopped, err := o.StopWorkspace(context.Background(), id)
+	if err != nil || stopped.Failed != 2 || stopped.Results[0].Outcome != "conflict" || stopped.Results[2].Outcome != "failed" || *stopped.Results[2].Error == "private stop detail" {
+		t.Fatalf("stop outcomes: %+v %v", stopped, err)
+	}
+	if runtime.Status(first).State != process.Running {
+		t.Fatal("failed stop claimed termination")
+	}
+	noEnabledID := uuid.New()
+	no := false
+	workspaces[noEnabledID] = &workspace.Workspace{ID: noEnabledID}
+	services = append(services, &model.Service{ID: uuid.New(), WorkspaceID: noEnabledID, Enabled: &no})
+	o = New(workspaces, services, lifecycle, runtime)
+	status, err := o.GetWorkspaceStatus(context.Background(), noEnabledID)
+	if err != nil || status.AggregateState != "idle" || status.EnabledServices != 0 {
+		t.Fatalf("disabled-only workspace: %+v %v", status, err)
+	}
+}
+
+type realLifecycle struct {
+	supervisor *process.Supervisor
+	configs    map[uuid.UUID]process.Config
+}
+
+func (l realLifecycle) Start(_ context.Context, id uuid.UUID) (process.Status, error) {
+	return l.supervisor.Start(id, l.configs[id])
+}
+func (l realLifecycle) Stop(ctx context.Context, id uuid.UUID) (process.Status, error) {
+	return l.supervisor.Stop(ctx, id)
+}
+
+func TestOrchestrationChild(t *testing.T) {
+	if os.Getenv("DEVORBIT_ORCHESTRATION_CHILD") == "1" {
+		for {
+			time.Sleep(time.Second)
+		}
+	}
+}
+
+func TestRealSupervisorPartialStartAndStop(t *testing.T) {
+	id := uuid.New()
+	good, bad, last := uuid.New(), uuid.New(), uuid.New()
+	yes := true
+	services := fakeServices{
+		{ID: good, WorkspaceID: id, Name: "good", Enabled: &yes, CreatedAt: time.Unix(1, 0)},
+		{ID: bad, WorkspaceID: id, Name: "bad", Enabled: &yes, CreatedAt: time.Unix(2, 0)},
+		{ID: last, WorkspaceID: id, Name: "last", Enabled: &yes, CreatedAt: time.Unix(3, 0)},
+	}
+	self, err := filepath.Abs(os.Args[0])
+	if err != nil {
+		t.Fatal(err)
+	}
+	dir := t.TempDir()
+	cfg := process.Config{Command: self, Args: []string{"-test.run=^TestOrchestrationChild$"}, Dir: dir, Env: map[string]string{"DEVORBIT_ORCHESTRATION_CHILD": "1"}}
+	supervisor := process.New(30 * time.Millisecond)
+	t.Cleanup(func() { _ = supervisor.Shutdown(context.Background()) })
+	lifecycle := realLifecycle{supervisor: supervisor, configs: map[uuid.UUID]process.Config{
+		good: cfg,
+		bad:  {Command: "missing-devorbit-orchestration-executable", Dir: dir},
+		last: cfg,
+	}}
+	o := New(fakeWorkspaces{id: {ID: id, Name: "workspace"}}, services, lifecycle, supervisor)
+	started, err := o.StartWorkspace(context.Background(), id)
+	if err != nil || started.Succeeded != 2 || started.Failed != 1 || started.Results[1].Outcome != "failed" || supervisor.Status(last).State != process.Running {
+		t.Fatalf("partial start: %+v %v", started, err)
+	}
+	stopped, err := o.StopWorkspace(context.Background(), id)
+	if err != nil || stopped.Succeeded != 2 || stopped.Skipped != 1 || supervisor.Status(good).State != process.Stopped || supervisor.Status(last).State != process.Stopped {
+		t.Fatalf("stop after partial start: %+v %v", stopped, err)
+	}
+}

# internal/process/executable_unix.go

diff --git a/internal/process/executable_unix.go b/internal/process/executable_unix.go
new file mode 100644
index 0000000..1b01236
--- /dev/null
+++ b/internal/process/executable_unix.go
@@ -0,0 +1,18 @@
+//go:build !windows
+
+package process
+
+import (
+	"os"
+)
+
+func executableFile(path string, _ []string) (string, error) {
+	info, err := os.Stat(path)
+	if err != nil {
+		return "", err
+	}
+	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
+		return "", ErrInvalid
+	}
+	return path, nil
+}

# internal/process/executable_windows.go

diff --git a/internal/process/executable_windows.go b/internal/process/executable_windows.go
new file mode 100644
index 0000000..67160ed
--- /dev/null
+++ b/internal/process/executable_windows.go
@@ -0,0 +1,40 @@
+//go:build windows
+
+package process
+
+import (
+	"os"
+	"path/filepath"
+	"strings"
+)
+
+func executableFile(path string, env []string) (string, error) {
+	extensions := []string{".com", ".exe"}
+	for _, pair := range env {
+		key, value, ok := strings.Cut(pair, "=")
+		if ok && strings.EqualFold(key, "PATHEXT") {
+			extensions = nil
+			for _, ext := range strings.Split(strings.ToLower(value), ";") {
+				if ext == ".exe" || ext == ".com" {
+					extensions = append(extensions, ext)
+				}
+			}
+		}
+	}
+	if ext := strings.ToLower(filepath.Ext(path)); ext == ".bat" || ext == ".cmd" {
+		return "", ErrInvalid
+	}
+	candidates := []string{path}
+	if filepath.Ext(path) == "" {
+		for _, ext := range extensions {
+			candidates = append(candidates, path+ext)
+		}
+	}
+	for _, candidate := range candidates {
+		info, err := os.Stat(candidate)
+		if err == nil && info.Mode().IsRegular() {
+			return candidate, nil
+		}
+	}
+	return "", ErrInvalid
+}

# internal/process/supervisor.go

diff --git a/internal/process/supervisor.go b/internal/process/supervisor.go
new file mode 100644
index 0000000..6830ba5
--- /dev/null
+++ b/internal/process/supervisor.go
@@ -0,0 +1,553 @@
+package process
+
+import (
+	"context"
+	"errors"
+	"fmt"
+	"os"
+	"os/exec"
+	"path/filepath"
+	"runtime"
+	"sort"
+	"strings"
+	"sync"
+	"time"
+
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/events"
+	"github.com/prasannashrestha011/devOrbit/internal/logs"
+)
+
+type State string
+
+const (
+	Stopped  State = "stopped"
+	Starting State = "starting"
+	Running  State = "running"
+	Stopping State = "stopping"
+	Exited   State = "exited"
+	Failed   State = "failed"
+)
+
+var ErrActive = errors.New("service lifecycle operation is already active")
+var ErrClosed = errors.New("process supervisor is shutting down")
+var ErrInvalid = errors.New("invalid execution configuration")
+var errGraceUnavailable = errors.New("graceful signal unavailable")
+
+func invalid(reason string) error { return fmt.Errorf("%w: %s", ErrInvalid, reason) }
+
+type Config struct {
+	WorkspaceID uuid.UUID
+	Command     string
+	Args        []string
+	Dir         string
+	Env         map[string]string
+}
+
+type Status struct {
+	ServiceID   uuid.UUID  `json:"service_id"`
+	ExecutionID uuid.UUID  `json:"execution_id"`
+	PID         *int       `json:"pid"`
+	State       State      `json:"state"`
+	StartedAt   *time.Time `json:"started_at"`
+	ExitedAt    *time.Time `json:"exited_at"`
+	ExitCode    *int       `json:"exit_code"`
+	LastError   string     `json:"last_error"`
+	UpdatedAt   time.Time  `json:"updated_at"`
+}
+
+type entry struct {
+	status      Status
+	workspaceID uuid.UUID
+	cmd         *exec.Cmd
+	tree        *processTree
+	done        chan struct{}
+	op          string
+	opDone      *operation
+}
+
+type operation struct {
+	done   chan struct{}
+	status Status
+	err    error
+}
+
+type Supervisor struct {
+	mu      sync.Mutex
+	entries map[uuid.UUID]*entry
+	ops     sync.WaitGroup
+	closed  bool
+	grace   time.Duration
+	logs    *logs.Manager
+	events  *events.Broker
+}
+
+func New(grace time.Duration, manager ...*logs.Manager) *Supervisor {
+	s := &Supervisor{entries: make(map[uuid.UUID]*entry), grace: grace}
+	if len(manager) > 0 {
+		s.logs = manager[0]
+	}
+	return s
+}
+
+// SetBroker wires lifecycle publishing before the supervisor accepts requests.
+func (s *Supervisor) SetBroker(broker *events.Broker) { s.events = broker }
+
+func (s *Supervisor) publish(kind events.Type, e *entry) {
+	data := map[string]any{"state": e.status.State}
+	if e.status.PID != nil {
+		data["pid"] = *e.status.PID
+	}
+	if e.status.ExitCode != nil {
+		data["exit_code"] = *e.status.ExitCode
+	}
+	var executionID *uuid.UUID
+	if e.status.ExecutionID != uuid.Nil {
+		id := e.status.ExecutionID
+		executionID = &id
+	}
+	s.events.Publish(events.Event{Type: kind, Timestamp: e.status.UpdatedAt, WorkspaceID: e.workspaceID, ServiceID: e.status.ServiceID, ExecutionID: executionID, Data: data})
+}
+
+func (s *Supervisor) Status(id uuid.UUID) Status {
+	s.mu.Lock()
+	defer s.mu.Unlock()
+	if e := s.entries[id]; e != nil {
+		return snapshot(e.status)
+	}
+	return Status{ServiceID: id, State: Stopped}
+}
+
+func (s *Supervisor) Closed() bool {
+	s.mu.Lock()
+	defer s.mu.Unlock()
+	return s.closed
+}
+
+func (s *Supervisor) Start(id uuid.UUID, cfg Config) (Status, error) {
+	s.mu.Lock()
+	if s.closed {
+		s.mu.Unlock()
+		return Status{}, ErrClosed
+	}
+	if e := s.entries[id]; e != nil && (e.op != "" || e.cmd != nil) {
+		s.mu.Unlock()
+		return Status{}, ErrActive
+	}
+	now := time.Now().UTC()
+	e := &entry{status: Status{ServiceID: id, State: Starting, UpdatedAt: now}, op: "start"}
+	s.entries[id] = e
+	s.ops.Add(1)
+	s.mu.Unlock()
+	defer s.finish(e, nil)
+	return s.launch(e, cfg)
+}
+
+func (s *Supervisor) launch(e *entry, cfg Config) (Status, error) {
+	s.mu.Lock()
+	e.workspaceID = cfg.WorkspaceID
+	s.mu.Unlock()
+	cmd, err := command(cfg)
+	if err != nil {
+		return s.failStart(e, err), err
+	}
+	tree, err := newProcessTree(cmd)
+	if err != nil {
+		return s.failStart(e, err), err
+	}
+	var capture *logs.Capture
+	if s.logs != nil {
+		capture, err = s.logs.Begin(cfg.WorkspaceID, e.status.ServiceID)
+		if err != nil {
+			tree.close()
+			return s.failStart(e, errors.New("log capture unavailable")), errors.New("log capture unavailable")
+		}
+		cmd.Stdout, cmd.Stderr = capture.Stdout, capture.Stderr
+		cmd.WaitDelay = 2 * time.Second
+	}
+	if err := cmd.Start(); err != nil {
+		if capture != nil {
+			_ = capture.Close()
+		}
+		tree.close()
+		// Command errors may contain environment values; keep status and API errors generic.
+		err = invalid("process launch failed")
+		return s.failStart(e, err), err
+	}
+	if err := tree.attach(cmd); err != nil {
+		_ = cmd.Process.Kill()
+		_ = cmd.Wait()
+		if capture != nil {
+			_ = capture.Close()
+		}
+		tree.close()
+		err = errors.New("process ownership setup failed")
+		return s.failStart(e, err), err
+	}
+	s.mu.Lock()
+	now := time.Now().UTC()
+	e.cmd = cmd
+	e.tree = tree
+	e.done = make(chan struct{})
+	e.status.PID = &cmd.Process.Pid
+	if capture != nil {
+		e.status.ExecutionID = capture.ExecutionID()
+	}
+	e.status.StartedAt = &now
+	e.status.UpdatedAt = now
+	e.status.State = Running
+	status := snapshot(e.status)
+	closed := s.closed
+	s.publish(events.ProcessStarted, e)
+	if e.op == "restart" {
+		s.publish(events.ProcessRestarted, e)
+	}
+	s.mu.Unlock()
+	go s.wait(e, cmd, tree, capture)
+	if closed {
+		return status, ErrClosed
+	}
+	return status, nil
+}
+
+func (s *Supervisor) failStart(e *entry, err error) Status {
+	s.mu.Lock()
+	defer s.mu.Unlock()
+	now := time.Now().UTC()
+	e.status.State = Failed
+	e.status.PID = nil
+	e.status.StartedAt = nil
+	e.status.ExitedAt = nil
+	e.status.ExitCode = nil
+	e.status.LastError = err.Error()
+	e.status.UpdatedAt = now
+	s.publish(events.ProcessFailed, e)
+	return snapshot(e.status)
+}
+
+func (s *Supervisor) wait(e *entry, cmd *exec.Cmd, tree *processTree, capture *logs.Capture) {
+	err := cmd.Wait()
+	// A launcher such as go run may exit while its child is still in the owned tree.
+	_ = tree.kill()
+	tree.close()
+	if capture != nil {
+		_ = capture.Close()
+	}
+	s.mu.Lock()
+	if s.entries[e.status.ServiceID] == e && e.cmd == cmd {
+		now := time.Now().UTC()
+		code := cmd.ProcessState.ExitCode()
+		e.status.ExitCode = &code
+		e.status.ExitedAt = &now
+		e.status.UpdatedAt = now
+		if e.status.State == Stopping {
+			e.status.State = Stopped
+			e.status.LastError = ""
+			s.publish(events.ProcessStopped, e)
+		} else if err == nil {
+			e.status.State = Exited
+			s.publish(events.ProcessExited, e)
+		} else {
+			e.status.State = Failed
+			e.status.LastError = fmt.Sprintf("process exited with code %d", code)
+			s.publish(events.ProcessFailed, e)
+		}
+		e.cmd = nil
+		e.tree = nil
+	}
+	close(e.done)
+	s.mu.Unlock()
+}
+
+func (s *Supervisor) finish(e *entry, err error) {
+	s.mu.Lock()
+	e.op = ""
+	if e.opDone != nil {
+		e.opDone.status = snapshot(e.status)
+		e.opDone.err = err
+		close(e.opDone.done)
+		e.opDone = nil
+	}
+	s.mu.Unlock()
+	s.ops.Done()
+}
+
+func (s *Supervisor) Stop(ctx context.Context, id uuid.UUID) (Status, error) {
+	return s.stop(ctx, id, false)
+}
+
+func (s *Supervisor) stop(ctx context.Context, id uuid.UUID, shutdown bool) (Status, error) {
+	s.mu.Lock()
+	if s.closed && !shutdown {
+		s.mu.Unlock()
+		return Status{}, ErrClosed
+	}
+	e := s.entries[id]
+	if e == nil || e.cmd == nil && e.op == "" {
+		status := Status{ServiceID: id, State: Stopped}
+		if e != nil {
+			status = snapshot(e.status)
+		}
+		s.mu.Unlock()
+		return status, nil
+	}
+	if e.op == "stop" {
+		op := e.opDone
+		s.mu.Unlock()
+		select {
+		case <-op.done:
+			return op.status, op.err
+		case <-ctx.Done():
+			return Status{}, ctx.Err()
+		}
+	}
+	if e.op != "" {
+		s.mu.Unlock()
+		return Status{}, ErrActive
+	}
+	e.op, e.opDone = "stop", &operation{done: make(chan struct{})}
+	e.status.State = Stopping
+	e.status.UpdatedAt = time.Now().UTC()
+	s.publish(events.ProcessStopping, e)
+	s.ops.Add(1)
+	s.mu.Unlock()
+	err := s.stopProcess(ctx, e)
+	s.mu.Lock()
+	status := snapshot(e.status)
+	s.mu.Unlock()
+	s.finish(e, err)
+	return status, err
+}
+
+func (s *Supervisor) stopProcess(ctx context.Context, e *entry) error {
+	s.mu.Lock()
+	tree, done := e.tree, e.done
+	s.mu.Unlock()
+	if tree == nil {
+		return nil // The process completed between reserving the operation and stopping it.
+	}
+	err := tree.interrupt()
+	if errors.Is(err, errGraceUnavailable) {
+		err = nil
+	} else if err == nil {
+		timer := time.NewTimer(s.grace)
+		defer timer.Stop()
+		select {
+		case <-done:
+			return nil
+		case <-timer.C:
+		case <-ctx.Done():
+		}
+	}
+	if killErr := tree.kill(); killErr != nil {
+		return errors.Join(err, killErr)
+	}
+	<-done
+	return err
+}
+
+func (s *Supervisor) Restart(ctx context.Context, id uuid.UUID, load func(context.Context) (Config, error)) (Status, error) {
+	s.mu.Lock()
+	if s.closed {
+		s.mu.Unlock()
+		return Status{}, ErrClosed
+	}
+	e := s.entries[id]
+	if e != nil && e.op != "" {
+		s.mu.Unlock()
+		return Status{}, ErrActive
+	}
+	if e == nil {
+		e = &entry{status: Status{ServiceID: id}}
+		s.entries[id] = e
+	}
+	e.op = "restart"
+	running := e.cmd != nil
+	if running {
+		e.status.State = Stopping
+		e.status.UpdatedAt = time.Now().UTC()
+		s.publish(events.ProcessStopping, e)
+	}
+	s.ops.Add(1)
+	s.mu.Unlock()
+	defer s.finish(e, nil)
+	if running {
+		if err := s.stopProcess(ctx, e); err != nil {
+			return s.Status(id), err
+		}
+	}
+	if err := ctx.Err(); err != nil {
+		return s.Status(id), err
+	}
+	cfg, err := load(ctx)
+	if err != nil {
+		return s.Status(id), err
+	}
+	s.mu.Lock()
+	if s.closed {
+		s.mu.Unlock()
+		return Status{}, ErrClosed
+	}
+	e.status = Status{ServiceID: id, State: Starting, UpdatedAt: time.Now().UTC()}
+	s.mu.Unlock()
+	return s.launch(e, cfg)
+}
+
+func (s *Supervisor) Shutdown(ctx context.Context) error {
+	s.mu.Lock()
+	s.closed = true
+	s.mu.Unlock()
+	s.ops.Wait()
+	s.mu.Lock()
+	ids := make([]uuid.UUID, 0)
+	for id, e := range s.entries {
+		if e.cmd != nil {
+			ids = append(ids, id)
+		}
+	}
+	s.mu.Unlock()
+	var wg sync.WaitGroup
+	errorsByService := make(chan error, len(ids))
+	for _, id := range ids {
+		wg.Add(1)
+		go func() {
+			defer wg.Done()
+			_, err := s.stop(ctx, id, true)
+			if err != nil {
+				errorsByService <- err
+			}
+		}()
+	}
+	wg.Wait()
+	close(errorsByService)
+	for err := range errorsByService {
+		if err != nil {
+			return err
+		}
+	}
+	if ctx.Err() != nil {
+		return ctx.Err()
+	}
+	return nil
+}
+
+func snapshot(status Status) Status {
+	if status.PID != nil {
+		v := *status.PID
+		status.PID = &v
+	}
+	if status.StartedAt != nil {
+		v := *status.StartedAt
+		status.StartedAt = &v
+	}
+	if status.ExitedAt != nil {
+		v := *status.ExitedAt
+		status.ExitedAt = &v
+	}
+	if status.ExitCode != nil {
+		v := *status.ExitCode
+		status.ExitCode = &v
+	}
+	return status
+}
+
+func command(cfg Config) (*exec.Cmd, error) {
+	if cfg.Command == "" || cfg.Dir == "" || strings.ContainsRune(cfg.Command, 0) || strings.ContainsRune(cfg.Dir, 0) {
+		return nil, ErrInvalid
+	}
+	for _, arg := range cfg.Args {
+		if strings.ContainsRune(arg, 0) {
+			return nil, ErrInvalid
+		}
+	}
+	dir, err := filepath.Abs(cfg.Dir)
+	if err != nil {
+		return nil, ErrInvalid
+	}
+	info, err := os.Stat(dir)
+	if err != nil || !info.IsDir() {
+		return nil, invalid("working directory unavailable")
+	}
+	env, err := environment(cfg.Env)
+	if err != nil {
+		return nil, err
+	}
+	path, err := executable(cfg.Command, dir, env)
+	if err != nil {
+		return nil, invalid("executable unavailable or not executable")
+	}
+	cmd := exec.Command(path, cfg.Args...)
+	cmd.Dir = dir
+	cmd.Env = env
+	return cmd, nil
+}
+
+func environment(overrides map[string]string) ([]string, error) {
+	inherited := os.Environ()
+	env := make([]string, 0, len(inherited)+len(overrides))
+	index := make(map[string]int, len(inherited))
+	key := func(k string) string {
+		if runtime.GOOS == "windows" {
+			return strings.ToUpper(k)
+		}
+		return k
+	}
+	for _, pair := range inherited {
+		if pos := strings.IndexByte(pair, '='); pos > 0 {
+			if i, ok := index[key(pair[:pos])]; ok {
+				env[i] = pair
+				continue
+			}
+			index[key(pair[:pos])] = len(env)
+		}
+		env = append(env, pair)
+	}
+	keys := make([]string, 0, len(overrides))
+	for k, v := range overrides {
+		if k == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, 0) {
+			return nil, ErrInvalid
+		}
+		keys = append(keys, k)
+	}
+	sort.Strings(keys)
+	for _, k := range keys {
+		pair := k + "=" + overrides[k]
+		if i, ok := index[key(k)]; ok {
+			env[i] = pair
+		} else {
+			index[key(k)] = len(env)
+			env = append(env, pair)
+		}
+	}
+	return env, nil
+}
+
+func executable(name, dir string, env []string) (string, error) {
+	paths := []string{name}
+	if !filepath.IsAbs(name) {
+		if strings.ContainsAny(name, `/\`) {
+			paths = []string{filepath.Join(dir, name)}
+		} else {
+			paths = nil
+			pathValue := ""
+			for _, pair := range env {
+				k, v, ok := strings.Cut(pair, "=")
+				if ok && ((runtime.GOOS == "windows" && strings.EqualFold(k, "PATH")) || k == "PATH") {
+					pathValue = v
+				}
+			}
+			for _, part := range filepath.SplitList(pathValue) {
+				if !filepath.IsAbs(part) {
+					part = filepath.Join(dir, part)
+				}
+				paths = append(paths, filepath.Join(part, name))
+			}
+		}
+	}
+	for _, path := range paths {
+		if found, err := executableFile(path, env); err == nil {
+			return found, nil
+		}
+	}
+	return "", ErrInvalid
+}

# internal/process/supervisor_test.go

diff --git a/internal/process/supervisor_test.go b/internal/process/supervisor_test.go
new file mode 100644
index 0000000..7c8701c
--- /dev/null
+++ b/internal/process/supervisor_test.go
@@ -0,0 +1,559 @@
+package process
+
+import (
+	"context"
+	"errors"
+	"os"
+	"os/exec"
+	"os/signal"
+	"path/filepath"
+	"runtime"
+	"sync"
+	"testing"
+	"time"
+
+	"github.com/google/uuid"
+	"github.com/prasannashrestha011/devOrbit/internal/events"
+	"github.com/prasannashrestha011/devOrbit/internal/logs"
+)
+
+func privateTempDir(t *testing.T) string {
+	t.Helper()
+	dir := t.TempDir()
+	if err := os.Chmod(dir, 0700); err != nil {
+		t.Fatal(err)
+	}
+	return dir
+}
+
+func TestLifecycleAndLogEvents(t *testing.T) {
+	b := events.New()
+	defer b.Close()
+	m, err := logs.New(privateTempDir(t), b)
+	if err != nil {
+		t.Fatal(err)
+	}
+	s := New(50*time.Millisecond, m)
+	s.SetBroker(b)
+	t.Cleanup(func() { _ = s.Shutdown(context.Background()); _ = m.Close() })
+	wid, sid := uuid.New(), uuid.New()
+	stream, unsubscribe := b.Subscribe(events.Filter{ServiceID: sid})
+	defer unsubscribe()
+	next := func() events.Event {
+		t.Helper()
+		select {
+		case event := <-stream:
+			if event.WorkspaceID != wid || event.ServiceID != sid || event.ID == "" {
+				t.Fatalf("bad event: %+v", event)
+			}
+			return event
+		case <-time.After(5 * time.Second):
+			t.Fatal("timed out waiting for event")
+			return events.Event{}
+		}
+	}
+	cfg := helperConfig(t, "logs")
+	cfg.WorkspaceID = wid
+	first, err := s.Start(sid, cfg)
+	if err != nil {
+		t.Fatal(err)
+	}
+	counts := map[events.Type]int{}
+	for {
+		event := next()
+		counts[event.Type]++
+		if event.ExecutionID == nil || *event.ExecutionID != first.ExecutionID {
+			t.Fatalf("wrong execution: %+v", event)
+		}
+		if event.Type == events.ProcessExited {
+			break
+		}
+	}
+	if counts[events.ProcessStarted] != 1 || counts[events.LogCreated] != 3 || counts[events.ProcessExited] != 1 {
+		t.Fatalf("unexpected event counts: %+v", counts)
+	}
+	waitState(t, s, sid, Exited)
+	wait := helperConfig(t, "wait")
+	wait.WorkspaceID = wid
+	if _, err := s.Start(sid, wait); err != nil {
+		t.Fatal(err)
+	}
+	if event := next(); event.Type != events.ProcessStarted {
+		t.Fatalf("want started: %+v", event)
+	}
+	exit := helperConfig(t, "exit")
+	exit.WorkspaceID = wid
+	if _, err := s.Restart(context.Background(), sid, configLoader(exit)); err != nil {
+		t.Fatal(err)
+	}
+	for _, kind := range []events.Type{events.ProcessStopping, events.ProcessStopped, events.ProcessStarted, events.ProcessRestarted, events.ProcessExited} {
+		if event := next(); event.Type != kind {
+			t.Fatalf("want %s: %+v", kind, event)
+		}
+	}
+	waitState(t, s, sid, Exited)
+	exit.Command = "missing-devorbit-executable"
+	if _, err := s.Start(sid, exit); !errors.Is(err, ErrInvalid) {
+		t.Fatal(err)
+	}
+	if event := next(); event.Type != events.ProcessFailed || event.ExecutionID != nil {
+		t.Fatalf("want failed start without execution: %+v", event)
+	}
+}
+
+func TestProcessHelper(t *testing.T) {
+	switch os.Getenv("DEVORBIT_HELPER") {
+	case "wait":
+		for {
+			time.Sleep(time.Second)
+		}
+	case "graceful", "ignore":
+		if os.Getenv("DEVORBIT_HELPER") == "ignore" {
+			signal.Ignore(os.Interrupt)
+		} else {
+			interrupt := make(chan os.Signal, 1)
+			signal.Notify(interrupt, os.Interrupt)
+			defer signal.Stop(interrupt)
+			defer func() { _ = os.WriteFile(os.Getenv("DEVORBIT_TEST_MARKER"), []byte("stopped"), 0600) }()
+			_ = os.WriteFile(os.Getenv("DEVORBIT_TEST_READY"), []byte("ready"), 0600)
+			<-interrupt
+			return
+		}
+		_ = os.WriteFile(os.Getenv("DEVORBIT_TEST_READY"), []byte("ready"), 0600)
+		for {
+			time.Sleep(time.Second)
+		}
+	case "exit":
+		os.Exit(0)
+	case "logs":
+		_, _ = os.Stdout.WriteString("stdout line\npartial")
+		_, _ = os.Stderr.WriteString("stderr line\r\n")
+		os.Exit(0)
+	case "fail":
+		os.Exit(7)
+	case "env":
+		if err := os.WriteFile(os.Getenv("DEVORBIT_TEST_OUT"), []byte(os.Getenv("DEVORBIT_TEST_VALUE")), 0600); err != nil {
+			os.Exit(8)
+		}
+		os.Exit(0)
+	case "spawn":
+		child := exec.Command(os.Args[0], "-test.run=^TestProcessHelper$")
+		child.Env = append(os.Environ(), "DEVORBIT_HELPER=heartbeat")
+		if err := child.Start(); err != nil {
+			os.Exit(9)
+		}
+		if err := os.WriteFile(os.Getenv("DEVORBIT_TEST_READY"), []byte("ready"), 0600); err != nil {
+			os.Exit(10)
+		}
+		for {
+			time.Sleep(time.Second)
+		}
+	case "heartbeat":
+		for {
+			_ = os.WriteFile(os.Getenv("DEVORBIT_TEST_BEAT"), []byte(time.Now().Format(time.RFC3339Nano)), 0600)
+			time.Sleep(10 * time.Millisecond)
+		}
+	}
+}
+
+func TestSupervisorCapturesProcessOutput(t *testing.T) {
+	m, err := logs.New(privateTempDir(t))
+	if err != nil {
+		t.Fatal(err)
+	}
+	s := New(50*time.Millisecond, m)
+	t.Cleanup(func() { _ = s.Shutdown(context.Background()); _ = m.Close() })
+	id, workspace := uuid.New(), uuid.New()
+	cfg := helperConfig(t, "logs")
+	cfg.WorkspaceID = workspace
+	status, err := s.Start(id, cfg)
+	if err != nil || status.ExecutionID == uuid.Nil {
+		t.Fatalf("start = %+v, %v", status, err)
+	}
+	waitState(t, s, id, Exited)
+	entries, err := m.List(context.Background(), logs.Filter{ServiceID: id, Limit: 10})
+	if err != nil || len(entries) != 3 {
+		t.Fatalf("entries = %+v, %v", entries, err)
+	}
+	streams := map[string]bool{}
+	for _, e := range entries {
+		if e.ExecutionID != status.ExecutionID || e.WorkspaceID != workspace {
+			t.Fatalf("wrong execution: %+v", e)
+		}
+		streams[e.Stream+":"+e.Message] = true
+	}
+	for _, expected := range []string{"stdout:stdout line\n", "stdout:partial", "stderr:stderr line\r\n"} {
+		if !streams[expected] {
+			t.Fatalf("missing %q: %+v", expected, streams)
+		}
+	}
+}
+
+func waitFile(t *testing.T, path string) {
+	t.Helper()
+	deadline := time.Now().Add(5 * time.Second)
+	for time.Now().Before(deadline) {
+		if _, err := os.Stat(path); err == nil {
+			return
+		}
+		time.Sleep(10 * time.Millisecond)
+	}
+	t.Fatalf("helper did not create %s", path)
+}
+
+func TestStopAndRestart(t *testing.T) {
+	s := New(100 * time.Millisecond)
+	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
+	id := uuid.New()
+	if status, err := s.Stop(context.Background(), id); err != nil || status.State != Stopped {
+		t.Fatalf("stop never started: %+v %v", status, err)
+	}
+	cfg := helperConfig(t, "wait")
+	first, err := s.Start(id, cfg)
+	if err != nil {
+		t.Fatal(err)
+	}
+	second, err := s.Restart(context.Background(), id, configLoader(cfg))
+	if err != nil || second.State != Running || *second.PID == *first.PID {
+		t.Fatalf("restart: %+v %v", second, err)
+	}
+	stopped, err := s.Stop(context.Background(), id)
+	if err != nil || stopped.State != Stopped || stopped.ExitedAt == nil || stopped.ExitCode == nil || stopped.LastError != "" {
+		t.Fatalf("stop: %+v %v", stopped, err)
+	}
+	again, err := s.Stop(context.Background(), id)
+	if err != nil || again.State != Stopped {
+		t.Fatalf("idempotent stop: %+v %v", again, err)
+	}
+	third, err := s.Restart(context.Background(), id, configLoader(cfg))
+	if err != nil || third.State != Running || *third.PID == *second.PID {
+		t.Fatalf("restart stopped: %+v %v", third, err)
+	}
+	originalCommand := cfg.Command
+	cfg.Command = "missing-devorbit-executable"
+	if _, err := s.Restart(context.Background(), id, configLoader(cfg)); !errors.Is(err, ErrInvalid) {
+		t.Fatalf("failed replacement: %v", err)
+	}
+	if status := s.Status(id); status.State != Failed || status.PID != nil || status.LastError == "" {
+		t.Fatalf("failed replacement status: %+v", status)
+	}
+	cfg.Command = originalCommand
+	cfg.Env["DEVORBIT_HELPER"] = "exit"
+	if _, err := s.Restart(context.Background(), id, configLoader(cfg)); err != nil {
+		t.Fatalf("restart failed launch: %v", err)
+	}
+	waitState(t, s, id, Exited)
+	cfg.Env["DEVORBIT_HELPER"] = "wait"
+	if status, err := s.Restart(context.Background(), id, configLoader(cfg)); err != nil || status.State != Running {
+		t.Fatalf("restart natural exit: %+v %v", status, err)
+	}
+}
+
+func TestRestartLoadsAfterTermination(t *testing.T) {
+	s := New(50 * time.Millisecond)
+	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
+	id := uuid.New()
+	cfg := helperConfig(t, "wait")
+	if _, err := s.Start(id, cfg); err != nil {
+		t.Fatal(err)
+	}
+	loaded := false
+	status, err := s.Restart(context.Background(), id, func(context.Context) (Config, error) {
+		loaded = true
+		if old := s.Status(id); old.State != Stopped || old.ExitedAt == nil {
+			t.Fatalf("configuration loaded before old process ended: %+v", old)
+		}
+		cfg.Env["DEVORBIT_HELPER"] = "exit"
+		return cfg, nil
+	})
+	if err != nil || !loaded || status.State != Running {
+		t.Fatalf("restart: %+v %v", status, err)
+	}
+	waitState(t, s, id, Exited)
+}
+
+func TestGracefulAndForcedStop(t *testing.T) {
+	s := New(80 * time.Millisecond)
+	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
+	if runtime.GOOS != "windows" {
+		graceID := uuid.New()
+		grace := helperConfig(t, "graceful")
+		grace.Env["DEVORBIT_TEST_READY"] = filepath.Join(t.TempDir(), "ready")
+		grace.Env["DEVORBIT_TEST_MARKER"] = filepath.Join(t.TempDir(), "marker")
+		if _, err := s.Start(graceID, grace); err != nil {
+			t.Fatal(err)
+		}
+		waitFile(t, grace.Env["DEVORBIT_TEST_READY"])
+		if status, err := s.Stop(context.Background(), graceID); err != nil || status.State != Stopped {
+			t.Fatalf("graceful stop: %+v %v", status, err)
+		}
+		waitFile(t, grace.Env["DEVORBIT_TEST_MARKER"])
+	}
+	forcedID := uuid.New()
+	forced := helperConfig(t, "ignore")
+	forced.Env["DEVORBIT_TEST_READY"] = filepath.Join(t.TempDir(), "ready")
+	if _, err := s.Start(forcedID, forced); err != nil {
+		t.Fatal(err)
+	}
+	waitFile(t, forced.Env["DEVORBIT_TEST_READY"])
+	start := time.Now()
+	status, err := s.Stop(context.Background(), forcedID)
+	if err != nil || status.State != Stopped || runtime.GOOS != "windows" && time.Since(start) < 60*time.Millisecond {
+		t.Fatalf("forced stop: %+v %v", status, err)
+	}
+}
+
+func TestConcurrentLifecycle(t *testing.T) {
+	s := New(40 * time.Millisecond)
+	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
+	id := uuid.New()
+	cfg := helperConfig(t, "wait")
+	if _, err := s.Start(id, cfg); err != nil {
+		t.Fatal(err)
+	}
+	var wg sync.WaitGroup
+	results := make(chan error, 12)
+	for range 12 {
+		wg.Add(1)
+		go func() {
+			defer wg.Done()
+			_, err := s.Stop(context.Background(), id)
+			results <- err
+		}()
+	}
+	wg.Wait()
+	close(results)
+	for err := range results {
+		if err != nil {
+			t.Fatalf("concurrent stop: %v", err)
+		}
+	}
+	if status := s.Status(id); status.State != Stopped {
+		t.Fatalf("stop status: %+v", status)
+	}
+	if _, err := s.Start(id, cfg); err != nil {
+		t.Fatal(err)
+	}
+	results = make(chan error, 12)
+	for range 12 {
+		wg.Add(1)
+		go func() {
+			defer wg.Done()
+			_, err := s.Restart(context.Background(), id, configLoader(cfg))
+			results <- err
+		}()
+	}
+	wg.Wait()
+	close(results)
+	for err := range results {
+		if err != nil && !errors.Is(err, ErrActive) {
+			t.Fatalf("concurrent restart: %v", err)
+		}
+	}
+	if status := s.Status(id); status.State != Running {
+		t.Fatalf("restart status: %+v", status)
+	}
+	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
+	defer cancel()
+	if err := s.Shutdown(ctx); err != nil {
+		t.Fatal(err)
+	}
+	if _, err := s.Restart(ctx, id, configLoader(cfg)); !errors.Is(err, ErrClosed) {
+		t.Fatalf("restart after shutdown: %v", err)
+	}
+}
+
+func TestStopAfterNaturalCompletion(t *testing.T) {
+	s := New(time.Second)
+	if err := s.stopProcess(context.Background(), &entry{done: make(chan struct{})}); err != nil {
+		t.Fatalf("completed process stop: %v", err)
+	}
+}
+
+func TestLifecycleConflictsAndShutdown(t *testing.T) {
+	s := New(100 * time.Millisecond)
+	id := uuid.New()
+	cfg := helperConfig(t, "ignore")
+	cfg.Env["DEVORBIT_TEST_READY"] = filepath.Join(t.TempDir(), "ready")
+	if _, err := s.Start(id, cfg); err != nil {
+		t.Fatal(err)
+	}
+	waitFile(t, cfg.Env["DEVORBIT_TEST_READY"])
+	stopped := make(chan error, 1)
+	go func() { _, err := s.Stop(context.Background(), id); stopped <- err }()
+	waitState(t, s, id, Stopping)
+	if _, err := s.Start(id, cfg); !errors.Is(err, ErrActive) {
+		t.Fatalf("start during stop: %v", err)
+	}
+	if _, err := s.Restart(context.Background(), id, configLoader(cfg)); !errors.Is(err, ErrActive) {
+		t.Fatalf("restart during stop: %v", err)
+	}
+	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
+	defer cancel()
+	shutdown := make(chan error, 1)
+	go func() { shutdown <- s.Shutdown(ctx) }()
+	if err := <-stopped; err != nil {
+		t.Fatal(err)
+	}
+	if err := <-shutdown; err != nil {
+		t.Fatal(err)
+	}
+	if status := s.Status(id); status.State != Stopped {
+		t.Fatalf("shutdown status: %+v", status)
+	}
+	if _, err := s.Start(id, cfg); !errors.Is(err, ErrClosed) {
+		t.Fatalf("start after shutdown: %v", err)
+	}
+}
+
+func helperConfig(t *testing.T, mode string) Config {
+	t.Helper()
+	self, err := filepath.Abs(os.Args[0])
+	if err != nil {
+		t.Fatal(err)
+	}
+	return Config{Command: self, Args: []string{"-test.run=^TestProcessHelper$"}, Dir: t.TempDir(), Env: map[string]string{"DEVORBIT_HELPER": mode}}
+}
+
+func configLoader(cfg Config) func(context.Context) (Config, error) {
+	return func(context.Context) (Config, error) { return cfg, nil }
+}
+
+func waitState(t *testing.T, s *Supervisor, id uuid.UUID, state State) Status {
+	t.Helper()
+	deadline := time.Now().Add(5 * time.Second)
+	for time.Now().Before(deadline) {
+		status := s.Status(id)
+		if status.State == state {
+			return status
+		}
+		time.Sleep(10 * time.Millisecond)
+	}
+	t.Fatalf("status did not become %s: %+v", state, s.Status(id))
+	return Status{}
+}
+
+func TestLifecycleAndRetry(t *testing.T) {
+	s := New(100 * time.Millisecond)
+	id := uuid.New()
+	if status := s.Status(id); status.State != Stopped || status.PID != nil {
+		t.Fatalf("initial status: %+v", status)
+	}
+	cfg := helperConfig(t, "exit")
+	status, err := s.Start(id, cfg)
+	if err != nil || status.State != Running || status.PID == nil || status.StartedAt == nil {
+		t.Fatalf("start: %+v %v", status, err)
+	}
+	exited := waitState(t, s, id, Exited)
+	if exited.ExitCode == nil || *exited.ExitCode != 0 || exited.ExitedAt == nil {
+		t.Fatalf("exit: %+v", exited)
+	}
+	*exited.PID = -1
+	if *s.Status(id).PID == -1 {
+		t.Fatal("status exposed internal pointer")
+	}
+	cfg.Env["DEVORBIT_HELPER"] = "fail"
+	if _, err := s.Start(id, cfg); err != nil {
+		t.Fatal(err)
+	}
+	failed := waitState(t, s, id, Failed)
+	if failed.ExitCode == nil || *failed.ExitCode != 7 || failed.LastError == "" {
+		t.Fatalf("failed exit: %+v", failed)
+	}
+	cfg.Command = "missing-devorbit-executable"
+	if _, err := s.Start(id, cfg); !errors.Is(err, ErrInvalid) {
+		t.Fatalf("missing executable: %v", err)
+	}
+	if status := s.Status(id); status.State != Failed || status.PID != nil || status.LastError == "" {
+		t.Fatalf("failed launch: %+v", status)
+	}
+	if err := s.Shutdown(context.Background()); err != nil {
+		t.Fatal(err)
+	}
+}
+
+func TestConcurrentStartsAndShutdown(t *testing.T) {
+	s := New(100 * time.Millisecond)
+	id := uuid.New()
+	cfg := helperConfig(t, "wait")
+	var wg sync.WaitGroup
+	results := make(chan error, 20)
+	for range 20 {
+		wg.Add(1)
+		go func() {
+			defer wg.Done()
+			_, err := s.Start(id, cfg)
+			results <- err
+		}()
+	}
+	wg.Wait()
+	close(results)
+	started, conflicts := 0, 0
+	for err := range results {
+		switch {
+		case err == nil:
+			started++
+		case errors.Is(err, ErrActive):
+			conflicts++
+		default:
+			t.Fatalf("unexpected start error: %v", err)
+		}
+	}
+	if started != 1 || conflicts != 19 || s.Status(id).State != Running {
+		t.Fatalf("starts=%d conflicts=%d status=%+v", started, conflicts, s.Status(id))
+	}
+	other := uuid.New()
+	if _, err := s.Start(other, cfg); err != nil {
+		t.Fatalf("different service: %v", err)
+	}
+	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
+	defer cancel()
+	if err := s.Shutdown(ctx); err != nil {
+		t.Fatal(err)
+	}
+	for _, serviceID := range []uuid.UUID{id, other} {
+		if status := s.Status(serviceID); status.State != Stopped || status.ExitedAt == nil {
+			t.Fatalf("shutdown status: %+v", status)
+		}
+	}
+	if _, err := s.Start(uuid.New(), cfg); !errors.Is(err, ErrClosed) {
+		t.Fatalf("start after shutdown: %v", err)
+	}
+}
+
+func TestEnvironmentAndWorkingDirectoryResolution(t *testing.T) {
+	s := New(100 * time.Millisecond)
+	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
+	self, err := filepath.Abs(os.Args[0])
+	if err != nil {
+		t.Fatal(err)
+	}
+	out := filepath.Join(t.TempDir(), "env.txt")
+	t.Setenv("DEVORBIT_TEST_VALUE", "inherited")
+	cfg := helperConfig(t, "env")
+	cfg.Dir = filepath.Dir(self)
+	cfg.Command = filepath.Base(self)
+	cfg.Env["PATH"] = "."
+	cfg.Env["DEVORBIT_TEST_VALUE"] = "override"
+	cfg.Env["DEVORBIT_TEST_OUT"] = out
+	id := uuid.New()
+	if _, err := s.Start(id, cfg); err != nil {
+		t.Fatal(err)
+	}
+	if state := waitState(t, s, id, Exited); state.ExitCode == nil || *state.ExitCode != 0 {
+		t.Fatalf("env helper: %+v", state)
+	}
+	content, err := os.ReadFile(out)
+	if err != nil || string(content) != "override" || os.Getenv("DEVORBIT_TEST_VALUE") != "inherited" {
+		t.Fatalf("environment override: %q %v", content, err)
+	}
+	for _, invalid := range []Config{
+		{Command: self, Dir: filepath.Join(t.TempDir(), "missing")},
+		{Command: self, Dir: cfg.Dir, Env: map[string]string{"BAD=KEY": "x"}},
+		{Command: self, Dir: cfg.Dir, Args: []string{"a\x00b"}},
+	} {
+		if _, err := s.Start(uuid.New(), invalid); !errors.Is(err, ErrInvalid) {
+			t.Fatalf("invalid config accepted: %v", err)
+		}
+	}
+}

# internal/process/tree_unix.go

diff --git a/internal/process/tree_unix.go b/internal/process/tree_unix.go
new file mode 100644
index 0000000..88bdafc
--- /dev/null
+++ b/internal/process/tree_unix.go
@@ -0,0 +1,50 @@
+//go:build !windows
+
+package process
+
+import (
+	"errors"
+	"os/exec"
+	"sync"
+	"syscall"
+)
+
+type processTree struct {
+	mu     sync.Mutex
+	pid    int
+	closed bool
+}
+
+func newProcessTree(cmd *exec.Cmd) (*processTree, error) {
+	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
+	return &processTree{}, nil
+}
+
+func (t *processTree) attach(cmd *exec.Cmd) error {
+	t.mu.Lock()
+	t.pid = cmd.Process.Pid
+	t.mu.Unlock()
+	return nil
+}
+
+func (t *processTree) interrupt() error { return t.signal(syscall.SIGINT) }
+func (t *processTree) kill() error      { return t.signal(syscall.SIGKILL) }
+
+func (t *processTree) signal(sig syscall.Signal) error {
+	t.mu.Lock()
+	defer t.mu.Unlock()
+	if t.closed {
+		return nil
+	}
+	err := syscall.Kill(-t.pid, sig)
+	if errors.Is(err, syscall.ESRCH) {
+		return nil
+	}
+	return err
+}
+
+func (t *processTree) close() {
+	t.mu.Lock()
+	t.closed = true
+	t.mu.Unlock()
+}

# internal/process/tree_unix_test.go

diff --git a/internal/process/tree_unix_test.go b/internal/process/tree_unix_test.go
new file mode 100644
index 0000000..65ee52a
--- /dev/null
+++ b/internal/process/tree_unix_test.go
@@ -0,0 +1,39 @@
+//go:build !windows
+
+package process
+
+import (
+	"context"
+	"os"
+	"path/filepath"
+	"testing"
+	"time"
+
+	"github.com/google/uuid"
+)
+
+func TestStopManagedDescendant(t *testing.T) {
+	s := New(50 * time.Millisecond)
+	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
+	id := uuid.New()
+	cfg := helperConfig(t, "spawn")
+	cfg.Env["DEVORBIT_TEST_READY"] = filepath.Join(t.TempDir(), "ready")
+	cfg.Env["DEVORBIT_TEST_BEAT"] = filepath.Join(t.TempDir(), "beat")
+	if _, err := s.Start(id, cfg); err != nil {
+		t.Fatal(err)
+	}
+	waitFile(t, cfg.Env["DEVORBIT_TEST_READY"])
+	waitFile(t, cfg.Env["DEVORBIT_TEST_BEAT"])
+	if _, err := s.Stop(context.Background(), id); err != nil {
+		t.Fatal(err)
+	}
+	before, err := os.ReadFile(cfg.Env["DEVORBIT_TEST_BEAT"])
+	if err != nil {
+		t.Fatal(err)
+	}
+	time.Sleep(100 * time.Millisecond)
+	after, err := os.ReadFile(cfg.Env["DEVORBIT_TEST_BEAT"])
+	if err != nil || string(before) != string(after) {
+		t.Fatalf("descendant remained active: %q -> %q (%v)", before, after, err)
+	}
+}

# internal/process/tree_windows.go

diff --git a/internal/process/tree_windows.go b/internal/process/tree_windows.go
new file mode 100644
index 0000000..48034fd
--- /dev/null
+++ b/internal/process/tree_windows.go
@@ -0,0 +1,110 @@
+//go:build windows
+
+package process
+
+import (
+	"os/exec"
+	"sync"
+	"syscall"
+	"unsafe"
+)
+
+var kernel32 = syscall.NewLazyDLL("kernel32.dll")
+var createJobObject = kernel32.NewProc("CreateJobObjectW")
+var setJobInformation = kernel32.NewProc("SetInformationJobObject")
+var assignToJob = kernel32.NewProc("AssignProcessToJobObject")
+var terminateJob = kernel32.NewProc("TerminateJobObject")
+var consoleControl = kernel32.NewProc("GenerateConsoleCtrlEvent")
+
+type jobLimits struct {
+	ProcessTime int64
+	JobTime     int64
+	Flags       uint32
+	MinWorking  uintptr
+	MaxWorking  uintptr
+	MaxActive   uint32
+	Affinity    uintptr
+	Priority    uint32
+	Scheduling  uint32
+	IO          [6]uint64
+	ProcessMem  uintptr
+	JobMem      uintptr
+	PeakProcess uintptr
+	PeakJob     uintptr
+}
+
+type processTree struct {
+	mu     sync.Mutex
+	job    syscall.Handle
+	pid    int
+	closed bool
+}
+
+func newProcessTree(cmd *exec.Cmd) (*processTree, error) {
+	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
+	h, _, err := createJobObject.Call(0, 0)
+	if h == 0 {
+		return nil, err
+	}
+	t := &processTree{job: syscall.Handle(h)}
+	limits := jobLimits{Flags: 0x2000} // JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
+	r, _, err := setJobInformation.Call(h, 9, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
+	if r == 0 {
+		t.close()
+		return nil, err
+	}
+	return t, nil
+}
+
+func (t *processTree) attach(cmd *exec.Cmd) error {
+	// shortcut: Job assignment follows process creation; use atomic job assignment if early descendants must be contained.
+	h, err := syscall.OpenProcess(0x0100|0x0001, false, uint32(cmd.Process.Pid))
+	if err != nil {
+		return err
+	}
+	defer syscall.CloseHandle(h)
+	t.mu.Lock()
+	defer t.mu.Unlock()
+	r, _, err := assignToJob.Call(uintptr(t.job), uintptr(h))
+	if r == 0 {
+		return err
+	}
+	t.pid = cmd.Process.Pid
+	return nil
+}
+
+func (t *processTree) interrupt() error {
+	t.mu.Lock()
+	defer t.mu.Unlock()
+	if t.closed {
+		return nil
+	}
+	r, _, _ := consoleControl.Call(1, uintptr(t.pid)) // CTRL_BREAK_EVENT
+	if r == 0 {
+		// Console control is unavailable without a shared console; force the owned job.
+		return errGraceUnavailable
+	}
+	return nil
+}
+
+func (t *processTree) kill() error {
+	t.mu.Lock()
+	defer t.mu.Unlock()
+	if t.closed {
+		return nil
+	}
+	r, _, err := terminateJob.Call(uintptr(t.job), 1)
+	if r == 0 {
+		return err
+	}
+	return nil
+}
+
+func (t *processTree) close() {
+	t.mu.Lock()
+	defer t.mu.Unlock()
+	if !t.closed {
+		_ = syscall.CloseHandle(t.job)
+		t.closed = true
+	}
+}

# internal/security/token_unix.go

diff --git a/internal/security/token_unix.go b/internal/security/token_unix.go
new file mode 100644
index 0000000..4b30526
--- /dev/null
+++ b/internal/security/token_unix.go
@@ -0,0 +1,62 @@
+//go:build !windows
+
+package security
+
+import (
+	"errors"
+	"os"
+	"path/filepath"
+	"syscall"
+)
+
+func privateDirectory(path string) error {
+	var parts []string
+	for part := filepath.Clean(path); ; part = filepath.Dir(part) {
+		parts = append(parts, part)
+		if part == filepath.Dir(part) {
+			break
+		}
+	}
+	protected := false
+	for i := len(parts) - 1; i >= 0; i-- {
+		part := parts[i]
+		info, err := os.Lstat(part)
+		if err != nil || info.Mode()&os.ModeSymlink != 0 && info.Sys().(*syscall.Stat_t).Uid != 0 {
+			return errors.New("token directory redirects through an untrusted symlink")
+		}
+		actual, err := os.Stat(part)
+		if err != nil || !actual.IsDir() || !protected && actual.Mode().Perm()&0022 != 0 && actual.Mode()&os.ModeSticky == 0 {
+			return errors.New("token directory has a writable shared ancestor")
+		}
+		if actual.Sys().(*syscall.Stat_t).Uid == uint32(os.Geteuid()) && actual.Mode().Perm()&0077 == 0 {
+			protected = true
+		}
+	}
+	info, err := os.Stat(path)
+	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
+		return errors.New("token directory is not private")
+	}
+	return nil
+}
+
+func createToken(path string) (*os.File, error) {
+	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
+	if err != nil {
+		return nil, err
+	}
+	return os.NewFile(uintptr(fd), path), nil
+}
+
+func openToken(path string) (*os.File, error) {
+	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
+	if err != nil {
+		return nil, err
+	}
+	f := os.NewFile(uintptr(fd), path)
+	info, err := f.Stat()
+	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
+		_ = f.Close()
+		return nil, errors.New("token file is not private")
+	}
+	return f, nil
+}

# internal/security/token_windows.go

diff --git a/internal/security/token_windows.go b/internal/security/token_windows.go
new file mode 100644
index 0000000..c7d6b66
--- /dev/null
+++ b/internal/security/token_windows.go
@@ -0,0 +1,147 @@
+//go:build windows
+
+package security
+
+import (
+	"errors"
+	"os"
+	"path/filepath"
+	"strings"
+	"syscall"
+	"unsafe"
+)
+
+var advapi32 = syscall.NewLazyDLL("advapi32.dll")
+var getNamedSecurityInfo = advapi32.NewProc("GetNamedSecurityInfoW")
+var convertSDToString = advapi32.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")
+var convertStringToSD = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
+
+func currentUserSID() (string, error) {
+	token, err := syscall.OpenCurrentProcessToken()
+	if err != nil {
+		return "", err
+	}
+	defer token.Close()
+	user, err := token.GetTokenUser()
+	if err != nil {
+		return "", err
+	}
+	return user.User.Sid.String()
+}
+
+func privateDirectory(path string) error {
+	resolved, err := filepath.EvalSymlinks(path)
+	if err != nil || !strings.EqualFold(resolved, filepath.Clean(path)) {
+		return errors.New("token directory redirects through a link")
+	}
+	info, err := os.Lstat(path)
+	if err != nil || !info.IsDir() {
+		return errors.New("token directory is unavailable")
+	}
+	return privateACL(path)
+}
+
+func privateACL(path string) error {
+	sid, err := currentUserSID()
+	if err != nil {
+		return err
+	}
+	name, err := syscall.UTF16PtrFromString(path)
+	if err != nil {
+		return err
+	}
+	var sd uintptr
+	// SE_FILE_OBJECT, OWNER_SECURITY_INFORMATION | DACL_SECURITY_INFORMATION.
+	result, _, _ := getNamedSecurityInfo.Call(uintptr(unsafe.Pointer(name)), 1, 5, 0, 0, 0, 0, uintptr(unsafe.Pointer(&sd)))
+	if result != 0 {
+		return syscall.Errno(result)
+	}
+	defer syscall.LocalFree(syscall.Handle(sd))
+	var encoded *uint16
+	var length uint32
+	ok, _, callErr := convertSDToString.Call(sd, 1, 5, uintptr(unsafe.Pointer(&encoded)), uintptr(unsafe.Pointer(&length)))
+	if ok == 0 {
+		return callErr
+	}
+	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(encoded)))
+	sddl := syscall.UTF16ToString(unsafe.Slice(encoded, length))
+	ownerStart, daclStart := strings.Index(sddl, "O:"), strings.Index(sddl, "D:")
+	if ownerStart < 0 || daclStart < 0 || ownerStart >= daclStart {
+		return errors.New("token path has no private owner or DACL")
+	}
+	owner := sddl[ownerStart+2 : daclStart]
+	if i := strings.Index(owner, "G:"); i >= 0 {
+		owner = owner[:i]
+	}
+	if owner != sid {
+		return errors.New("token path is owned by another account")
+	}
+	aces := sddl[daclStart+2:]
+	if !strings.Contains(aces, "(") {
+		return errors.New("token path has no DACL entries")
+	}
+	for start := strings.IndexByte(aces, '('); start >= 0; start = strings.IndexByte(aces, '(') {
+		end := strings.IndexByte(aces[start:], ')')
+		if end < 0 {
+			return errors.New("invalid token path DACL")
+		}
+		fields := strings.Split(aces[start+1:start+end], ";")
+		if len(fields) != 6 || fields[0] != "A" || fields[5] != sid && fields[5] != "SY" && fields[5] != "BA" && fields[5] != "CO" {
+			return errors.New("token path grants access to another account")
+		}
+		aces = aces[start+end+1:]
+	}
+	return nil
+}
+
+func createToken(path string) (*os.File, error) {
+	sid, err := currentUserSID()
+	if err != nil {
+		return nil, err
+	}
+	sddl, err := syscall.UTF16PtrFromString("O:" + sid + "D:P(A;;GA;;;SY)(A;;GA;;;" + sid + ")")
+	if err != nil {
+		return nil, err
+	}
+	var sd uintptr
+	ok, _, callErr := convertStringToSD.Call(uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&sd)), 0)
+	if ok == 0 {
+		return nil, callErr
+	}
+	defer syscall.LocalFree(syscall.Handle(sd))
+	name, err := syscall.UTF16PtrFromString(path)
+	if err != nil {
+		return nil, err
+	}
+	sa := syscall.SecurityAttributes{Length: uint32(unsafe.Sizeof(syscall.SecurityAttributes{})), SecurityDescriptor: sd}
+	h, err := syscall.CreateFile(name, syscall.GENERIC_WRITE, 0, &sa, syscall.CREATE_NEW, syscall.FILE_ATTRIBUTE_NORMAL, 0)
+	if err == syscall.ERROR_FILE_EXISTS || err == syscall.ERROR_ALREADY_EXISTS {
+		return nil, os.ErrExist
+	}
+	if err != nil {
+		return nil, err
+	}
+	return os.NewFile(uintptr(h), path), nil
+}
+
+func openToken(path string) (*os.File, error) {
+	name, err := syscall.UTF16PtrFromString(path)
+	if err != nil {
+		return nil, err
+	}
+	h, err := syscall.CreateFile(name, syscall.GENERIC_READ|0x00020000, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0) // READ_CONTROL
+	if err != nil {
+		return nil, err
+	}
+	f := os.NewFile(uintptr(h), path)
+	var info syscall.ByHandleFileInformation
+	if err := syscall.GetFileInformationByHandle(h, &info); err != nil || info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
+		_ = f.Close()
+		return nil, errors.New("token path is not a regular file")
+	}
+	if err := privateACL(path); err != nil {
+		_ = f.Close()
+		return nil, err
+	}
+	return f, nil
+}

