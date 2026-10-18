# DevOrbit Backend V1 — Implementation Phase 01

## Objective

Implement the project foundation for DevOrbit using Go, Chi, GORM, and PostgreSQL 17.

Follow the DevOrbit Master Project Context.

**Implement Phase 01 only. Do not implement future phases.**

## Tasks

### 1. Initialize Go Backend

Inspect the repository before making changes.

If necessary, initialize the Go module using the appropriate project module path.

Create a minimal project structure:

- `cmd/devorbit/main.go`
- `internal/config`
- `internal/database`
- `internal/api`
- `internal/app`

Additional packages may be introduced only when directly required.

Do not create placeholders for future domain modules.

### 2. Configuration System

Implement environment-based configuration with:

- `DEVORBIT_HOST` — default `127.0.0.1`
- `DEVORBIT_PORT` — default `7456`
- `DEVORBIT_DATABASE_URL` — PostgreSQL connection string
- `DEVORBIT_DATA_DIR` — local application data directory
- `DEVORBIT_LOG_DIR` — application log directory
- `DEVORBIT_SHUTDOWN_TIMEOUT` — graceful shutdown duration
- `DEVORBIT_AUTH_TOKEN_PATH` — local authentication token file

Validate configuration and provide clear errors.

Never print database credentials or API tokens in logs.

Provide `.env.example` for reference, but avoid requiring an additional configuration framework unless justified.

### 3. PostgreSQL Connection

Implement database initialization using:

- GORM
- `gorm.io/driver/postgres`
- PostgreSQL 17

Requirements:

- Establish database connection.
- Verify connectivity using the underlying SQL connection.
- Configure connection pooling appropriately for a local backend.
- Handle connection failures cleanly.
- Close database resources during shutdown.
- Avoid embedding database credentials in source code.

PostgreSQL must be running and accessible before backend startup.

Do not attempt to install or automatically manage PostgreSQL.

### 4. GORM AutoMigrate Foundation

Create an explicit database schema initialization mechanism based on GORM AutoMigrate.

Requirements:

- Support registration of persistence models.
- Run AutoMigrate during application initialization.
- Return and log migration errors safely.
- Avoid destructive schema operations.
- Do not use SQL migration files or third-party migration frameworks.

There are no workspace or service models in Phase 01.

Do not invent production persistence models solely to populate AutoMigrate.

You may verify AutoMigrate behavior with a test-only model using an isolated test database or schema.

### 5. GORM Gen Preparation

The project will use `gorm.io/gen` to generate type-safe query code from GORM models.

For Phase 01:

- Establish appropriate dependency and code organization conventions.
- Document how generation will work.
- Do not generate queries for nonexistent workspace or service models.
- Do not implement repositories or business-domain DAO code yet.

Actual model-specific code generation begins in Phase 02.

### 6. HTTP Server

Use Chi as the HTTP router.

Implement:

- `GET /health`
- `GET /api/v1/system/info`
- Request logging
- Panic recovery
- Common JSON response helpers
- Consistent error response formatting
- HTTP server timeouts

`/health` should report application/database health and return an appropriate unhealthy HTTP status when PostgreSQL is unavailable.

The system-info endpoint must not reveal secrets.

### 7. Local API Authentication

Implement local API security:

- Bind to loopback by default.
- Generate or load a cryptographically random authentication token.
- Store the token with restrictive OS-appropriate permissions.
- Require bearer token authentication for `/api/v1` endpoints.
- Leave `/health` unauthenticated.
- Reject untrusted browser origins for sensitive requests.
- Do not enable permissive wildcard CORS.
- Avoid exposing tokens in application logs.

Design authentication so the future Electron main process can access the backend securely without exposing the token to untrusted renderer code.

### 8. Application Lifecycle

Implement:

1. Load configuration.
2. Initialize structured logging.
3. Initialize local application directories.
4. Connect to PostgreSQL.
5. Run registered GORM AutoMigrate models.
6. Initialize local API authentication.
7. Initialize Chi HTTP router.
8. Start HTTP server.
9. Handle shutdown signals.
10. Gracefully stop HTTP requests and close database resources.

Do not implement process supervision or managed child processes in this phase.

### 9. Testing

Write tests for:

- Configuration defaults and validation
- Database initialization and connectivity handling
- AutoMigrate initialization and repeatability
- HTTP health endpoint
- Authentication middleware
- System-info endpoint
- Graceful shutdown where practical

For integration tests requiring PostgreSQL, use a dedicated test database or isolated schema.

Never run destructive tests against the developer's normal database.

If PostgreSQL is unavailable, distinguish tests actually passed from integration tests not executed.

Run:

- `go test ./...`
- `go vet ./...`
- `go build ./cmd/devorbit`

Run race detection where practical and report the actual result.

### 10. Documentation

Create or update:

- `README.md`
- `.env.example`
- `docs/implementation-progress.md`

Explain:

- How to configure PostgreSQL 17
- How to start the Go backend
- Environment variables
- How local authentication works
- How GORM AutoMigrate is invoked
- Planned GORM Gen workflow
- Important Phase 01 limitations

## Out of Scope

Do not implement:

- Workspace models or CRUD
- Service models or CRUD
- Generated domain query code
- Process execution
- Process supervision
- Log aggregation
- SSE events
- Workspace orchestration
- Electron desktop application
- Docker integration

## Acceptance Criteria

Phase 01 is complete when:

1. The backend compiles successfully.
2. The application connects to PostgreSQL 17.
3. GORM database initialization works.
4. AutoMigrate registration and execution infrastructure works.
5. The server starts on the configured local address.
6. `/health` returns meaningful application/database health.
7. Protected endpoints reject unauthorized requests.
8. Authenticated system-info requests succeed.
9. The application shuts down gracefully.
10. Relevant tests pass.
11. Documentation and progress records are updated.

## Final Instruction

Implement Phase 01 only.

Do not implement Phase 02.

Provide the implementation report defined in the master context, including actual test results and known limitations.

Stop after Phase 01 is complete.