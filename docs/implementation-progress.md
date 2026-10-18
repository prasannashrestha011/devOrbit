# Implementation progress

## Phase 01 — foundation

Implemented local HTTP server, Chi routing, token authentication, PostgreSQL connection, health check, structured logging, graceful shutdown, and database migration entry point. Unit tests and platform builds passed previously. PostgreSQL 17 startup and migration remain unverified in a live environment; Phase 01 acceptance is pending that check.

## Phase 02 — workspace management

Implemented the Workspace persistence model, explicit AutoMigrate registration, reproducible GORM Gen query generation, DB-injected repository, validation service, and authenticated CRUD routes. Workspace names are trimmed, paths are canonicalized on the host, and PATCH uses a supplied-field map so empty descriptions can be saved. Duplicate root paths are allowed.

DevOrbit uses one application DB, passed directly to `repository.New(db)` and `query.Use(db)`. Request contexts still flow through controller, service, and repository methods to GORM Gen. A transaction can be injected explicitly when one is needed; DevOrbit has no tenant database or schema middleware.

Module endpoint definitions live with their modules. The central `internal/routes` package registers all modules under the authenticated API router.

Validation and API tests passed with `go test -race ./...`; `go vet ./...` and Linux, Windows, and macOS builds passed. Query regeneration produced identical file hashes on a second run. PostgreSQL 17 integration tests cover migration repeatability, CRUD across separate connections, and transaction rollback. They passed on 2026-10-18 against a temporary isolated PostgreSQL 17 container using `DEVORBIT_TEST_DATABASE_URL`. Phase 02 acceptance is complete.

## Phase 03 — service configuration management

Implemented the Service persistence model, Workspace foreign key and delete restriction, JSONB arguments and environment overrides, workspace-scoped unique names, GORM Gen queries, service repository and validation layer, and five authenticated API endpoints. PATCH uses supplied-field maps so false booleans, empty argument arrays, and empty environment maps can be saved. Working directories are checked against the local filesystem and stored relative to the workspace root when inside it. Configuration never launches commands. `go generate ./...` now invokes the existing model-first generator.

Unit and HTTP API tests cover validation, defaults, duplicate names, authentication, bad IDs and JSON types, partial updates, deletion, and workspace deletion conflict response. A PostgreSQL 17 integration test covers repeated migration, foreign key and unique constraints, JSONB round trips, repository CRUD, concurrent duplicate creation, and workspace deletion conflict. The full `go test -race -count=1 ./...` suite passed on 2026-10-18 against the temporary isolated PostgreSQL 17 container. `go vet ./...`, Linux, Windows, and macOS builds passed. Repeated `go generate ./...` produced identical hashes. Phase 03 acceptance is complete.

## Phase 04 — process supervisor core

Implemented an in-memory process supervisor independent of HTTP and GORM. It reserves each service ID before launching, runs registered executables directly with separate arguments and resolved working directories, applies environment overrides, tracks PID and lifecycle status, observes exits asynchronously, supports retries and concurrent distinct services, and kills/reaps active direct children at backend shutdown. New authenticated start and status routes use the existing service and workspace application layers. Automatic restart and readiness detection remain outside this phase.

Unit tests execute real child processes to cover concurrent starts, immediate exits, retries, failure status, environment and working directory resolution, invalid configuration, and shutdown cleanup. HTTP tests cover authentication, invalid IDs, missing and disabled services, start conflict, and status responses. The full `go test -race -count=1 -buildvcs=false ./...` suite passed on 2026-10-18; focused supervisor race tests passed 20 consecutive runs. `go vet` and Linux, macOS, and Windows builds passed. Phase 04 acceptance is complete.

## Phase 05 — process lifecycle management

Implemented stop and restart in the existing in-memory supervisor, with `stopping` state, per-service operation coordination, graceful interruption, timeout escalation, process-tree cleanup, and concurrent shutdown. Restart reloads the current persisted service configuration after the previous execution terminates. New authenticated stop and restart API endpoints preserve existing response and error conventions. Linux and macOS use isolated process groups; Windows uses Job Objects and attempts console control before force termination.

Unit tests cover idempotent stop, graceful and forced stop, replacement PIDs, restart after stopped, failed, and naturally exited states, configuration reload ordering, concurrent stops and restarts, lifecycle conflicts, shutdown during a stop, and Unix descendant termination. API tests cover authentication, UUID validation, missing and disabled services, stop, restart, and current configuration use. The full `go test -count=1 -buildvcs=false ./...` and `go test -race -count=1 -buildvcs=false ./...` suites passed on 2026-10-18. `go vet -buildvcs=false ./...` and Linux, macOS, and Windows/amd64 builds passed. PostgreSQL integration tests were skipped because `DEVORBIT_TEST_DATABASE_URL` was unset. Phase 05 implementation is complete. Native macOS and Windows process behavior remains unverified, and the Windows launch gap is noted below.

## Phase 06 — workspace orchestration

Added a workspace orchestrator that lists every registered service, sorts by creation time and ID, starts enabled services in that order, stops active services in reverse order, and returns individual outcomes and summary counts even when one service fails. It delegates execution and lifecycle conflicts to the existing process service and supervisor. Per-workspace operation reservations reject overlapping workspace starts and stops without blocking independent workspaces. A status endpoint combines persisted workspace and service details with best-effort in-memory runtime snapshots. Runtime state remains unpersisted.

Authenticated `POST /api/v1/workspaces/{id}/start`, `POST /api/v1/workspaces/{id}/stop`, and `GET /api/v1/workspaces/{id}/status` are registered through the central router. Unit tests cover deterministic ordering, pagination beyond 100 services, disabled and already-running behavior, partial failure, aggregate states, conflicts, shutdown, missing IDs, and real supervisor process launch and cleanup; HTTP tests cover authentication, IDs, missing workspaces, and responses. The full `go test -race -count=1 -buildvcs=false ./...` suite, `go vet -buildvcs=false ./...`, and Linux, macOS/amd64, and Windows/amd64 builds passed on 2026-10-18. PostgreSQL integration tests were skipped because `DEVORBIT_TEST_DATABASE_URL` was unset. Phase 06's supplied prompt ends mid-sentence in the concurrency section; implementation follows all complete requirements present in that file.

## Phase 07 — centralized process logs

Added a filesystem log manager integrated with the existing supervisor's stdout and stderr writers. Each execution gets a unique ID; output is captured in bounded chunks, queued without blocking on file writes, persisted as JSONL, and cached in a bounded per-service memory buffer. The manager rotates files, removes old executions, exposes dropped-entry and storage-error counts, and retrieves service or workspace history with cursor pagination. The authenticated service and workspace log routes use the central router. Tests cover concurrent stream output, partial lines, UTF-8 chunk boundaries, distinct executions, process integration, pagination, persistence across restart, memory bounds, rotation, retention, and API validation. The full Go race suite, vet, and Linux, macOS/amd64, and Windows/amd64 builds passed on 2026-10-18. PostgreSQL integration tests were skipped because `DEVORBIT_TEST_DATABASE_URL` was unset.

The supplied Phase 07 prompt ends mid-sentence in its persistence section. The implementation follows its complete requirements and the stated objective; live streaming remains out of scope.

## Phase 08 — real-time SSE events

Added an in-memory event broker with workspace and service filtering, fan-out to multiple subscribers, bounded per-client channels, slow-client disconnection, and clean closure. The existing supervisor publishes started, stopping, stopped, exited, failed, and restarted events at state transitions; the log manager publishes created entries with the original log timestamp and metadata. Lifecycle payloads exclude service configuration and environment values; log payloads reflect captured output and may contain secrets printed by child processes. The authenticated `GET /api/v1/events` route supports optional workspace and service filters, validates IDs against persisted records, streams SSE frames with heartbeat comments, and clears the request's write deadline for long-lived connections. Application shutdown closes the broker to release streams. Unit and HTTP tests cover filtering, overflow, closure, lifecycle and log publication, authentication, and live delivery. The full Go race suite, vet, and macOS/Windows builds passed on 2026-10-18. PostgreSQL integration tests were skipped because `DEVORBIT_TEST_DATABASE_URL` was unset.

The supplied Phase 08 prompt ends at the first broker requirement. The implementation follows its complete sections and stated objective. SSE has no replay; persisted log endpoints provide history.

## Phase 09 — security and reliability hardening

Reviewed API authentication, Host and Origin checks, token storage, process launch and termination, filesystem paths, logs, events, SSE, and shutdown. Added a strict HTTP Host check at the production server, rejected multiple Origin headers, capped API bodies at 1 MiB, required JSON content type for configuration writes, and tightened SSE and log query filters. Token loading now reads from verified file handles; Unix rejects untrusted symlink paths, wrong owners, and broad permissions. Windows creates the token with an explicit current-user/System DACL and refuses an existing token or directory whose owner or allow ACEs cannot be verified as private. Log storage rejects redirected roots and service directories and skips symlinked log files. Process launch failures now keep OS error details out of status, stop handles natural completion during the transition, SSE writes have rolling deadlines, and log shutdown runs even if process shutdown fails.

The complete Go race suite, vet, and Linux, macOS/amd64, and Windows/amd64 builds passed on 2026-10-18. Windows ACL and process-tree behavior still need native Windows tests; macOS process-tree behavior still needs native macOS tests. PostgreSQL integration tests were skipped because `DEVORBIT_TEST_DATABASE_URL` was unset. See [security review](security-review.md) for findings and residual risks. The supplied Phase 09 prompt ends mid-list in its reconnection section; work follows its complete requirements.

## Known limitations and next phase

GORM AutoMigrate has no general rollback; review schema changes before upgrading a populated database. Native Windows token ACL behavior remains unverified. Environment overrides are plaintext in PostgreSQL and should not contain secrets. Windows Job Object assignment happens immediately after process creation, leaving a brief window in which an early spawned descendant may escape ownership. Windows graceful console control depends on an available shared console; otherwise DevOrbit forces the owned job to terminate. Updating or deleting a running service configuration does not stop its current process. Automatic restart remains pending for later phases. Workspace orchestration has no dependency ordering or readiness detection; creation time and ID determine launch order.
