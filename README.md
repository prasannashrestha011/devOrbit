# DevOrbit backend

DevOrbit has a local Go API foundation and a Workspace Management module. The Go module lives at the repository root.

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
| `DEVORBIT_LOG_DIR` | `<data dir>/logs` | Reserved application log directory |
| `DEVORBIT_SHUTDOWN_TIMEOUT` | `10s` | Graceful HTTP shutdown deadline |
| `DEVORBIT_AUTH_TOKEN_PATH` | `<data dir>/auth-token` | Local bearer token file |

Path settings must be absolute. The process creates missing data and log directories with private permissions. On Unix, the token file is created with mode `0600`; an existing token or its parent directory with broader permissions is rejected. On Windows, use a token location within the current user's private profile and ensure its ACL excludes other users.

## API

- `GET /health` is unauthenticated and checks PostgreSQL connectivity. It returns `503` when the database is unavailable.
- `GET /api/v1/system/info` requires `Authorization: Bearer <token>`.
- `POST /api/v1/workspaces` creates a workspace from `name`, optional `description`, and an existing `root_path`.
- `GET /api/v1/workspaces` lists workspaces in creation order, with optional `limit` (1–100, default 50) and `offset` (nonnegative, default 0).
- `GET /api/v1/workspaces/{id}` retrieves one workspace.
- `PATCH /api/v1/workspaces/{id}` updates supplied `name`, `description`, or `root_path` fields; an empty description clears it.
- `DELETE /api/v1/workspaces/{id}` permanently deletes a workspace.

The token is generated once using a cryptographic random source and stored at `DEVORBIT_AUTH_TOKEN_PATH`. A future Electron main process should read it directly and keep it out of renderer code. Browser requests with an `Origin` header are accepted only from the API's own origin. No wildcard CORS policy is enabled. Requests are logged by method, path, status, and duration without query strings or authorization headers.

## Schema and generated queries

Startup calls `database.Migrate` with the explicitly registered Workspace model. GORM AutoMigrate does not provide a safe general rollback or every schema evolution; review schema changes before upgrading a populated database.

After changing the Workspace model, run `go run ./cmd/gen` to regenerate `internal/dal/query`; do not edit generated files. The generator uses `ApplyBasic` on the Go model and does not require a live database. Workspace repositories resolve the active GORM connection or transaction from `context.Context` when constructed and use `query.Use(db)` for generated queries. The HTTP route supplies the application's DB in request context.

## Tests

```sh
go test ./...
go vet ./...
go build -buildvcs=false ./cmd/devorbit
```

Set `DEVORBIT_TEST_DATABASE_URL` to a separate PostgreSQL 17 database to run migration and repository integration tests. They create and drop only unique temporary schemas. Without that variable, these tests are skipped.

Current limitations: no service CRUD, process supervisor, log streaming, SSE, or desktop UI. The log directory is prepared but file logging is not implemented; structured logs go to standard error.
