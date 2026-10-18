# DevOrbit Backend V1 — Implementation Phase 02

## Objective

Implement the **Workspace Management module** using Go, GORM, GORM Gen, and PostgreSQL 17.

Follow the existing DevOrbit Master Project Context and preserve the Phase 01 architecture.

**Implement Phase 02 only. Do not implement any later phases.**

## 1. Pre-Implementation Inspection

Before modifying code:

1. Inspect the current project structure.
2. Review the database initialization and AutoMigrate entry point.
3. Review the existing Chi router, authentication, configuration, and error response conventions.
5. Follow existing project patterns wherever appropriate.

Do not recreate existing infrastructure or unnecessarily refactor Phase 01.

## 2. Workspace Model

Introduce the first persistent domain model: `Workspace`.

Required fields:

| Field | Type | Rules |
|---|---|---|
| ID | UUID | Primary key, generated in Go |
| Name | string | Required, maximum 120 characters |
| Description | string | Optional, maximum 1000 characters |
| RootPath | string | Required, normalized absolute directory |
| CreatedAt | time.Time | Set automatically |
| UpdatedAt | time.Time | Updated automatically |

Requirements:

- Use appropriate GORM model tags.
- Store UUIDs using PostgreSQL's native `uuid` type.
- Generate UUIDs in the application, not through database-specific defaults.
- Use a table named `workspaces`.
- Do not add unnecessary fields.
- Do not introduce soft deletion.
- Allow multiple workspaces to reference the same root directory.
- Use appropriate database constraints for required fields.

Keep persistence models separate from HTTP request and response DTOs.

## 3. Workspace Validation

Implement reusable validation rules.

### Name
- Trim surrounding whitespace.
- Reject empty or whitespace-only names.
- Enforce a maximum length of 120 characters.

### Description
- Optional.
- Enforce a maximum length of 1000 characters.
- Allow an empty string to clear the description during updates.

### RootPath
- Require a valid directory path.
- Convert to an absolute path.
- Normalize using platform-aware filesystem functions.
- Confirm the directory exists.
- Reject paths pointing to regular files.
- Handle filesystem permission errors.
- Consider symbolic links and canonicalization.
- Do not make assumptions about Unix-style paths on Windows.

Filesystem validation must happen on the machine running DevOrbit.

Avoid custom path parsing when Go's `path/filepath` package is sufficient.

## 4. AutoMigrate Integration

Integrate the Workspace model with the existing Phase 01 AutoMigrate mechanism.

Requirements:

1. Register the Workspace model explicitly.
2. Ensure AutoMigrate executes during database initialization.
3. Use the existing GORM connection.
4. Do not introduce SQL migration files.
5. Do not create a new migration framework.
6. Preserve the existing database initialization architecture.

Schema initialization must remain safe to repeat.

## 5. GORM Gen Integration

Implement the first actual GORM Gen query generation workflow.

Requirements:

- Use `gorm.io/gen`.
- Generate type-safe queries for the Workspace model.
- Generate from the existing Go model using `ApplyBasic`.
- Do not introspect every PostgreSQL table.
- Provide a repeatable generation command such as `go run ./cmd/gen`.
- Keep generated code in a dedicated package such as `internal/dal/query`.
- Do not manually modify generated files.
- Ensure generated queries are context-aware.
- Prefer `query.Use(db)` with an injected database connection rather than relying on global query initialization.
- Avoid import cycles between models, repositories, and generated queries.
- Ensure generation can be reproduced consistently.

Generated code should support workspace creation, retrieval, listing, updating, and deletion.

Document how to regenerate queries after a model changes.

## 6. Workspace Repository

Implement a dedicated workspace repository using the generated GORM Gen query API.

Required operations:

- Create workspace
- Get workspace by UUID
- List workspaces
- Update workspace
- Delete workspace

Requirements:

- Accept `context.Context` for database operations.
- Use generated type-safe queries.
- Do not access the database directly from HTTP handlers.
- Handle `gorm.ErrRecordNotFound` appropriately.
- Distinguish expected domain errors from unexpected database failures.
- Propagate errors without exposing internal SQL details.
- Use transactions only when necessary.
- Do not introduce unnecessary generic repository abstractions.

Repository methods must be suitable for use by the application/service layer.

## 7. Workspace Service Layer

Implement business operations separately from HTTP handlers.

Responsibilities:

- Validate workspace input.
- Normalize filesystem paths.
- Generate workspace UUIDs.
- Coordinate repository operations.
- Implement update validation.
- Handle missing workspaces.
- Return meaningful domain errors.

Use constructor-based dependency injection.

Avoid creating interfaces unless they represent useful boundaries or improve testability.

Keep the implementation straightforward and idiomatic.

## 8. REST API

Register the workspace endpoints under the existing authenticated `/api/v1` router.

| Method | Endpoint | Purpose |
|---|---|---|
| POST | `/api/v1/workspaces` | Create |
| GET | `/api/v1/workspaces` | List |
| GET | `/api/v1/workspaces/{id}` | Retrieve |
| PATCH | `/api/v1/workspaces/{id}` | Update |
| DELETE | `/api/v1/workspaces/{id}` | Delete |

### Create Workspace

Example request:

```json
{
  "name": "RMS Development",
  "description": "Restaurant Management System",
  "root_path": "/home/developer/projects/rms"
}
```

Expected behavior:

- Validate request fields.
- Confirm the directory exists.
- Normalize and persist the root path.
- Generate a UUID.
- Return the created workspace with HTTP 201.

### List Workspaces

- Return persisted workspaces.
- Use deterministic ordering.
- Support optional `limit` and `offset` parameters.
- Default limit: 50.
- Maximum limit: 100.
- Validate pagination parameters.
- Return an empty collection when no workspaces exist.

### Get Workspace

- Validate UUID format.
- Return the matching workspace.
- Return HTTP 404 when not found.

### Update Workspace

Support partial updates using PATCH.

Editable fields:

- name
- description
- root_path

Requirements:

- Update only provided fields.
- Preserve omitted fields.
- Support clearing description with an empty string.
- Reject unsupported or invalid update fields.
- Reject empty updates.
- Validate a new root path before persisting it.
- Preserve ID and creation timestamp.
- Update the modification timestamp.

Avoid zero-value handling bugs in GORM updates.

### Delete Workspace

- Delete the workspace permanently.
- Return HTTP 204 on success.
- Return HTTP 404 when the workspace does not exist.

No running-service restrictions are required yet because services are introduced in Phase 03.

Do not implement service deletion or process cleanup in this phase.

### API Standards

Reuse Phase 01 conventions for:

- Authentication
- JSON serialization
- Error envelopes
- Request logging
- Validation errors
- Panic recovery

Expected status codes:

- 200 — successful retrieval or update
- 201 — created
- 204 — deleted
- 400 — invalid request
- 401 — unauthorized
- 404 — workspace not found
- 409 — genuine resource conflict, where applicable
- 500 — unexpected failure

Do not expose SQL errors, database credentials, stack traces, or internal filesystem details unnecessarily.

## 9. Testing

Write unit tests and PostgreSQL integration tests.

### Validation Tests

Test:

- Valid workspace
- Empty name
- Whitespace-only name
- Name exceeding maximum length
- Invalid directory
- Path pointing to a file
- Relative path normalization
- Empty description
- Excessively long description
- Invalid UUID

### Repository Tests

Test:

- Create and retrieve
- List multiple workspaces
- Update fields
- Delete workspace
- Retrieve nonexistent workspace
- Persist data across separate database connections
- AutoMigrate repeatability
- Regeneration and compilation of GORM Gen queries

### API Tests

Test:

- Successful creation
- Successful retrieval
- Successful listing
- Successful partial update
- Successful deletion
- Invalid request payloads
- Unauthorized access
- Missing resources
- Empty PATCH request
- Invalid pagination
- Invalid UUID parameters

### Test Database Safety

Use `DEVORBIT_TEST_DATABASE_URL` for PostgreSQL integration tests.

Never run destructive test operations against the normal development database.

Use an isolated test database or a unique test schema with controlled cleanup.

If the test database is unavailable, report integration tests as skipped rather than passed.

Also run