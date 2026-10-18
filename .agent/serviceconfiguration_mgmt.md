# DevOrbit Backend V1 — Implementation Phase 03

## Objective

Implement the **Service Configuration Management module** for DevOrbit using Go, GORM, GORM Gen, and PostgreSQL 17.

Follow the existing DevOrbit Master Project Context and preserve the architecture established in Phases 01 and 02.

**Implement Phase 03 only. Do not implement process execution, supervision, or later phases.**

## 1. Pre-Implementation Inspection

Before writing code:

1. Inspect the existing repository structure.
2. Review the Workspace model and its validation rules.
3. Review the Workspace service, repository, and HTTP handlers.
4. Review how GORM Gen queries are generated and initialized.
5. Review the existing AutoMigrate registration mechanism.
6. Review API authentication, request validation, and error handling.
7. Read `docs/implementation-progress.md`.

Reuse the existing architecture and patterns.

Do not rewrite working Phase 01 or Phase 02 components unnecessarily.

If Phase 02 has unresolved integration-test failures, report them and avoid claiming the new phase is fully verified.

## 2. Service Configuration Model

A workspace can contain multiple independently executable services.

Example:

Workspace: RMS Development

Services:
- Go Backend
- React Frontend
- Flutter Desktop
- Background Worker

Create a persistent `Service` model.

### Required Fields

| Field | Type | Description |
|---|---|---|
| ID | UUID | Primary key, generated in Go |
| WorkspaceID | UUID | Foreign key referencing Workspace |
| Name | string | Human-readable service name |
| Command | string | Executable name or path |
| Args | []string | Command arguments |
| WorkingDirectory | string | Directory used for execution |
| Env | map[string]string | Environment variable overrides |
| Enabled | bool | Whether service participates in Start All |
| AutoRestart | bool | Future process restart configuration |
| CreatedAt | time.Time | Creation timestamp |
| UpdatedAt | time.Time | Modification timestamp |

### Requirements

- Table name: `services`
- Use PostgreSQL native UUID columns.
- Use PostgreSQL JSONB for `args` and `env`.
- Default `enabled` to true.
- Default `auto_restart` to false.
- Add an index on `workspace_id`.
- Enforce uniqueness of service names within a workspace.
- Allow identical service names in different workspaces.
- Add an appropriate foreign key constraint.
- Use database-level constraints where appropriate.
- Do not implement soft deletion.

Use GORM-compatible serialization or custom types where necessary.

Do not add runtime process fields to this persistence model.

PID, execution state, exit code, and runtime timestamps belong to the future process supervisor, not the service configuration table.

## 3. Workspace Relationship

Each service belongs to exactly one workspace.

Requirements:

- A service cannot exist without a valid workspace.
- Creating a service for a nonexistent workspace must return 404.
- Service names must be unique within their workspace.
- Enforce referential integrity in PostgreSQL.
- Do not automatically delete services when a workspace is deleted.

### Workspace Deletion Behavior

Extend existing workspace deletion behavior safely.

If a workspace contains registered services, reject deletion with HTTP 409 Conflict.

This prevents accidental loss of service configurations.

The user must explicitly remove the services before deleting the workspace.

Avoid cascade deletion for V1.

Do not introduce service execution checks, because the process supervisor does not exist yet.

## 4. Service Validation

Implement reusable service configuration validation.

### Name

- Required.
- Trim leading and trailing whitespace.
- Maximum length: 120 characters.
- Reject empty or whitespace-only names.
- Enforce uniqueness within the workspace.

### Command

The command represents an executable, not a complete shell command.

Examples:

Valid:
- `go`
- `npm`
- `flutter`
- `python3`
- `/usr/local/bin/custom-tool`

Invalid:
- Empty command
- Whitespace-only command
- A command containing a null byte

Store arguments separately.

Example:

Command: `go`

Arguments:
`["run", "./cmd/api"]`

Do not concatenate these into a shell command.

Do not execute commands during configuration validation.

Executable resolution and launch errors will be handled by the process supervisor in Phase 04.

### Arguments

- Store arguments as a JSONB array of strings.
- Preserve argument order.
- Support an empty argument array.
- Reject invalid JSON types.
- Preserve empty-string arguments where intentionally supplied.
- Do not interpret shell operators, pipes, or redirection.

### Working Directory

Support:

- Relative directories resolved against the workspace root.
- Absolute directory paths.
- An omitted or empty working directory, which means the workspace root.

Requirements:

1. Resolve relative paths against the workspace root.
2. Normalize paths using `path/filepath`.
3. Validate that the resolved directory exists.
4. Reject paths pointing to regular files.
5. Handle filesystem permission errors.
6. Preserve cross-platform filesystem compatibility.
7. Do not create directories automatically.

Choose one consistent persistence representation and document it.

Prefer storing a normalized relative path for directories inside the workspace, so moving a workspace remains easier.

Absolute paths may be stored when explicitly supplied.

Do not turn service working-directory validation into a generic filesystem access or command-execution endpoint.

### Environment Variables

Use:

`map[string]string`

Example:

```json
{
  "APP_ENV": "development",
  "PORT": "8080",
  "LOG_LEVEL": "debug"
}
```

Requirements:

- Support an empty environment map.
- Validate variable names.
- Reject empty names and names containing `=`.
- Reject null bytes in keys or values.
- Preserve case because environment behavior differs across operating systems.
- Do not expand variables in this phase.
- Do not load `.env` files yet.
- Do not execute anything using these values.
- Never log environment values unnecessarily.

Environment overrides are stored as plaintext configuration in PostgreSQL for V1. Document that sensitive credentials should not be stored using this mechanism until a secure secret-management approach is introduced.

Protect all service configuration endpoints with existing authentication.

### Enabled and AutoRestart

`Enabled` defaults to true.

`AutoRestart` defaults to false.

Support explicitly setting either value to false or true.

Use request DTOs that distinguish omitted boolean fields from explicitly supplied `false`.

Do not implement automatic restart behavior in this phase.

## 5. GORM AutoMigrate Integration

Register the Service model in the existing schema initialization mechanism.

Requirements:

- Preserve Workspace model registration.
- Register Service after Workspace to respect model relationships.
- Run GORM AutoMigrate during application initialization.
- Ensure repeated AutoMigrate execution is safe.
- Do not introduce SQL migration files.
- Do not drop existing tables or data.
- Verify foreign key and unique constraints.

Avoid adding schema logic directly inside HTTP handlers or domain services.

## 6. GORM Gen Integration

Extend the existing GORM Gen generator.

Requirements:

1. Register the Service model with the generator.
2. Preserve generation of Workspace queries.
3. Generate type-safe Service query APIs.
4. Keep the generation process reproducible.
5. Do not modify generated files manually.
6. Avoid import cycles.
7. Use generated context-aware queries in repositories.
8. Preserve existing Workspace repository functionality.

Prefer model-first generation with `ApplyBasic`.

Use the project's existing generator rather than creating a second query generation mechanism.

## 7. Service Repository

Implement persistence operations using GORM Gen.

Required operations:

- Create service
- Get service by ID
- List services belonging to a workspace
- Update service
- Delete service
- Check whether a workspace contains services
- Check whether a service name already exists within a workspace

Requirements:

- Accept `context.Context`.
- Use generated queries.
- Keep database operations outside HTTP handlers.
- Handle missing records.
- Handle duplicate names.
- Handle foreign key violations.
- Translate expected database constraints into domain errors.
- Avoid exposing internal SQL error messages.
- Use transactions only where necessary.

Ensure uniqueness validation remains safe under concurrent requests through database constraints.

## 8. Service Application Layer

Introduce a service-configuration application layer.

Responsibilities:

- Validate incoming configurations.
- Confirm workspace existence.
- Normalize executable configuration.
- Resolve working directories.
- Apply defaults.
- Generate service UUIDs.
- Coordinate persistence.
- Validate updates.
- Translate repository errors into domain errors.

Keep HTTP handlers independent of database implementation details.

Use constructor-based dependency injection.

Avoid unnecessary generic interfaces.

## 9. REST API

Implement the following authenticated endpoints.

### Workspace Service Endpoints

| Method | Endpoint | Action |
|---|---|---|
| POST | `/api/v1/workspaces/{id}/services` | Create service |
| GET | `/api/v1/workspaces/{id}/services` | List services |

### Individual Service Endpoints

| Method | Endpoint | Action |
|---|---|---|
| GET | `/api/v1/services/{id}` | Get service |
| PATCH | `/api/v1/services/{id}` | Update service |
| DELETE | `/api/v1/services/{id}` | Delete service |

Reuse existing Chi routing, authentication, and error response conventions.

### Create Service

Example request:

```json
{
  "name": "Go Backend",
  "command": "go",
  "args": ["run", "./cmd/api"],
  "working_directory": "./backend",
  "env": {
    "APP_ENV": "development",
    "PORT": "8080"
  },
  "enabled": true,
  "auto_restart": false
}
```

Expected behavior:

1. Validate workspace UUID.
2. Confirm workspace exists.
3. Validate request.
4. Resolve and validate working directory.
5. Check duplicate service names.
6. Apply defaults.
7. Persist service.
8. Return HTTP 201 with the created service.

### List Services

Return services belonging to the requested workspace.

Requirements:

- Validate workspace UUID.
- Return 404 for nonexistent workspace.
- Use deterministic ordering.
- Support pagination using the project's existing convention.
- Return an empty list for a workspace without services.
- Do not return services belonging to other workspaces.

### Get Service

Return an individual service by UUID.

Return 404 when it does not exist.

### Update Service

Use PATCH semantics.

Editable fields:

- name
- command
- args
- working_directory
- env
- enabled
- auto_restart

Requirements:

- Update only supplied fields.
- Preserve omitted fields.
- Support explicitly setting boolean values to false.
- Support replacing arguments with an empty array.
- Support clearing environment overrides with an empty object.
- Reject empty PATCH requests.
- Reject unknown fields.
- Validate changed configuration.
- Preserve ID and WorkspaceID.
- Preserve CreatedAt.
- Update UpdatedAt.
- Prevent duplicate names within a workspace.

For V1, treat supplied `args` and `env` values as full replacements rather than merging individual elements or environment keys.

Document this behavior.

Be careful with GORM zero-value updates.

Do not allow a service to move to another workspace through PATCH.

### Delete Service

Requirements:

- Permanently delete the service.
- Return HTTP 204 on success.
- Return 404 if service does not exist.
- Preserve the parent workspace.
- Do not implement process termination.

### Expected HTTP Status Codes

- 200: Success
- 201: Created
- 204: Deleted
- 400: Invalid request
- 401: Unauthorized
- 404: Workspace or service not found
- 409: Duplicate service name or workspace deletion conflict
- 500: Unexpected internal failure

Follow existing response formats.

Do not leak database credentials, SQL errors, or sensitive configuration values in logs.

## 10. Testing Requirements

Implement unit and PostgreSQL integration tests.

### Model and Validation Tests

Test:

- Valid service configuration
- Empty service name
- Duplicate service name
- Same service name in different workspaces
- Empty command
- Invalid command
- Empty argument array
- Invalid argument types
- Relative working directory
- Absolute working directory
- Invalid working directory
- Environment key validation
- Enabled true and false
- AutoRestart true and false
- Default values

### Repository Tests

Test:

- Create service
- Retrieve service
- List services by workspace
- Update service
- Delete service
- Foreign key enforcement
- Unique name constraint
- Concurrent duplicate creation
- Persist JSONB arguments
- Persist JSONB environment variables
- AutoMigrate repeatability

### API Tests

Test:

- Successful service creation
- Successful retrieval
- Successful listing
- Successful update
- Successful deletion
- Unauthorized requests
- Invalid service UUID
- Invalid workspace UUID
- Nonexistent workspace
- Nonexistent service
- Duplicate service name
- Empty PATCH
- PATCH with `enabled: false`
- PATCH with `auto_restart: false`
- PATCH with empty args
- PATCH with empty env
- Unknown request fields
- Workspace deletion conflict when services exist

### Integration Testing

Use `DEVORBIT_TEST_DATABASE_URL`.

Never run destructive tests against the development or production database.

Use isolated test schemas or dedicated test databases.

Run applicable tests and validation commands:

```bash
go generate ./...
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/devorbit
```

Verify generated query code compiles.

Report tests actually executed.

Do not mark database integration tests successful if PostgreSQL was unavailable.

## 11. Documentation

Update:

- README.md
- docs/implementation-progress.md
- Existing API documentation, if present

Document:

- Service configuration schema
- Service API endpoints
- Example requests and responses
- Working-directory behavior
- Argument handling
- Environment variable behavior and security limitation
- GORM Gen regeneration
- Relationship between Workspace and Service
- Workspace deletion restrictions
- Testing instructions

Do not rewrite unrelated documentation.

## 12. Explicitly Out of Scope

Do not implement:

- Process execution
- Process supervisor
- Start/Stop/Restart endpoints
- Runtime process status
- Process ID management
- Workspace Start All / Stop All
- Automatic process restart
- Live log streaming
- SSE event broadcasting
- Interactive terminals
- Health checks for services
- Docker Engine integration
- Electron desktop interface

Service configuration must remain strictly separate from runtime execution.

## 13. Acceptance Criteria

Phase 03 is complete when:

1. Service persistence model is implemented.
2. Services belong to valid workspaces.
3. GORM AutoMigrate supports the new model.
4. GORM Gen generates usable Service queries.
5. Service repository CRUD works.
6. Service validation works.
7. All five service endpoints are implemented.
8. Service names are unique within a workspace.
9. Arguments and environment variables persist correctly.
10. Partial updates correctly handle zero values.
11. Existing authentication protects all endpoints.
12. Workspace deletion is rejected when services exist.
13. Existing Phase 01 and 02 functionality remains working.
14. Relevant automated tests pass.
15. PostgreSQL integration tests pass before full completion is reported.
16. Documentation and progress tracking are updated.
17. No process execution functionality has been introduced.

## 14. Final Instructions

Implement **Phase 03 — Service Configuration Management** only.

Respect the existing architecture.

Avoid overengineering, unnecessary abstractions, and unrelated changes.

Do not implement Phase 04.

After finishing, provide:

**Completed:** Functionality implemented.

**Files Changed:** Important files added or modified.

**Architecture Decisions:** Significant design choices.

**Validation:** Actual test/build results.

**Limitations:** Remaining issues or unverified behavior.

**Next Phase:** Phase 04 — Process Supervisor Core.

Then stop and wait for the next implementation instruction.