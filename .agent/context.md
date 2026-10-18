# DevOrbit — Master Project Context (Backend V1)

## 1. Agent Role

You are a senior Go backend engineer and software architect helping develop DevOrbit, an enterprise-oriented Developer Environment Orchestrator.

**IMPORTANT: THIS DOCUMENT PROVIDES PROJECT CONTEXT ONLY. IT IS NOT AN IMPLEMENTATION INSTRUCTION.**

Do not create, edit, delete, or generate project files upon receiving this document.

Implementation will happen through separate, narrowly scoped sub-prompts.

For every implementation request:

- Follow this master context.
- Implement only the explicitly requested phase.
- Do not proactively develop future modules.
- Inspect existing code before making changes.
- Preserve working functionality.
- Write relevant tests.
- Report completed work and limitations.
- Stop after the requested phase is complete.

## 2. Project Overview

**Project:** DevOrbit

**Category:** Developer Tools / Developer Environment Orchestration

**Architecture:** Local-first modular monolith

**Target OS:** macOS, Linux, Windows

DevOrbit allows developers to manage multiple applications belonging to a development workspace from a centralized desktop interface.

Example development environment:

- Go backend: `go run ./cmd/api`
- React frontend: `npm run dev`
- Flutter application: `flutter run -d macos`
- Background worker: `go run ./cmd/worker`

The platform will eventually manage startup, termination, restart, monitoring, logs, and orchestration of these development services.

It must provide significantly more value than a collection of shell scripts or tmux sessions through reliable process supervision, configuration management, monitoring, and a unified developer interface.

## 3. Technology Stack

### Backend

- Language: Go
- HTTP Router: Chi
- ORM: GORM
- Database: PostgreSQL 17
- Query Generator: GORM Gen (`gorm.io/gen`)
- Schema Management: GORM AutoMigrate
- Structured Logging: `log/slog`
- HTTP API: REST JSON
- Live Events: Server-Sent Events (SSE)
- Process Execution: `os/exec`
- Tests: Go standard testing package

### Desktop Frontend

- Desktop Runtime: Electron
- Language: TypeScript
- UI Framework: React
- Backend Communication: REST and SSE

Electron frontend development is outside the initial backend implementation phases.

The future Electron main process should communicate securely with the local Go backend. Authentication tokens must not be exposed unnecessarily to renderer processes.

### Architecture

Use a modular monolith.

Do not introduce microservices, distributed message brokers, Kubernetes, or cloud dependencies in V1.

## 4. Database and Query Architecture

### PostgreSQL

Use PostgreSQL 17 as the authoritative persistent datastore.

Persist:

- Workspaces
- Service configurations
- Application configuration where appropriate
- Relevant persistent metadata

Do not use SQLite.

### GORM

Use GORM for:

- Database connectivity
- Schema synchronization through AutoMigrate
- Transactions
- Model mapping
- Standard persistence operations where appropriate

### GORM Gen

Use `gorm.io/gen` to generate type-safe database query code.

Requirements:

- Maintain clearly defined persistence models.
- Organize generated query code separately from business logic.
- Keep generated code reproducible.
- Do not manually edit generated files.
- Integrate regeneration into the development workflow.
- Prefer generated, context-aware queries for data access once the corresponding models are implemented.

Avoid unnecessary mixing of handwritten raw SQL, GORM dynamic queries, and generated DAO patterns without a specific reason.

### AutoMigrate

For V1, use GORM AutoMigrate rather than a versioned SQL migration framework.

Rules:

1. AutoMigrate must run during database initialization.
2. Apply AutoMigrate only to explicitly registered persistence models.
3. Models must be introduced incrementally with their implementation phases.
4. Do not create separate SQL migration files.
5. Do not use destructive schema operations automatically.
6. Never assume AutoMigrate can safely perform every schema evolution.
7. Record known schema limitations in documentation.

A versioned migration system may be introduced in a future release if deployment requirements justify it.

## 5. Backend V1 Functional Scope

### Workspace Management

- Create, read, update, and delete workspaces.
- Persist workspaces in PostgreSQL.
- Support configurable project root paths.
- Validate workspace paths.

### Service Configuration

- Register services within a workspace.
- Store executables, arguments, working directories, and environment overrides.
- Enable and disable services.
- Persist service definitions.

### Process Supervisor

- Start services.
- Stop services.
- Restart services.
- Run multiple services concurrently.
- Track process IDs.
- Maintain runtime lifecycle states.
- Detect unexpected termination.
- Prevent duplicate execution.
- Handle graceful and forced termination safely.
- Clean up managed process trees.

### Workspace Orchestration

- Start all enabled workspace services.
- Stop all running workspace services.
- Return per-service operation results.
- Retrieve aggregate workspace status.

### Logging and Events

- Capture stdout and stderr.
- Stream real-time logs.
- Retain bounded recent logs.
- Provide log history.
- Broadcast lifecycle events using SSE.

### Security

- Bind the local API to loopback by default.
- Require authentication for sensitive endpoints.
- Validate browser origins.
- Protect environment variables and credentials.
- Never expose unrestricted remote command execution.
- Run processes with the current user's permissions.

## 6. Architectural Rules

### Package Organization

Organize code by cohesive responsibilities, potentially including:

- `internal/config`
- `internal/database`
- `internal/api`
- `internal/workspace`
- `internal/service`
- `internal/process`
- `internal/logs`
- `internal/events`
- `internal/platform`
- `internal/security`
- `internal/app`

These are suggested responsibilities, not instructions to create every package immediately.

### Engineering Principles

1. Keep HTTP handlers thin.
2. Separate domain logic from infrastructure.
3. Use dependency injection where appropriate.
4. Avoid unnecessary interfaces and abstraction layers.
5. Use context-aware database operations.
6. Use structured errors.
7. Make shared runtime state concurrency-safe.
8. Handle application shutdown properly.
9. Prefer small, testable components.
10. Keep generated persistence code outside domain logic.
11. Avoid premature optimization.
12. Keep implementation understandable and maintainable.

## 7. Important Process Management Rules

Process execution is the most critical subsystem.

- Use executable paths and argument arrays instead of implicit shell execution.
- Prevent duplicate starts, including concurrent requests.
- Differentiate running processes from healthy applications.
- Track explicit lifecycle states.
- Call process Wait correctly.
- Capture output without deadlocks.
- Avoid goroutine leaks.
- Use bounded log buffers.
- Do not terminate unrelated processes.
- Implement platform-specific process termination safely.
- Do not assume macOS process handling works identically on Windows.
- Keep automatic restart disabled by default.

OS-specific behavior must be isolated where necessary.

## 8. API Standards

Use `/api/v1` as the HTTP API prefix.

Follow consistent conventions for:

- JSON requests and responses
- Error responses
- Input validation
- HTTP status codes
- Pagination where appropriate
- Authentication
- Runtime status reporting

Use SSE for real-time log and process lifecycle events.

Do not expose sensitive API operations without authentication.

## 9. V1 Non-Goals

The following features are outside Backend V1:

- Desktop UI implementation
- Interactive PTY terminals
- Docker Engine integration
- Kubernetes
- Remote SSH execution
- Remote execution agents
- Multi-tenant cloud SaaS infrastructure
- Complex dependency DAGs
- Advanced application readiness checks
- AI debugging
- Production deployment management

Do not implement these without a separate instruction.

## 10. Incremental Development Policy

Development will proceed through small implementation phases.

Each phase must be:

- Clearly scoped
- Independently reviewable
- Testable
- Consistent with existing architecture
- Documented after implementation

Before modifying code, inspect relevant existing components.

Do not create unused abstractions for modules that are not yet implemented.

Keep the application compiling throughout development.

## 11. Progress Documentation

Maintain:

`docs/implementation-progress.md`

Document:

- Completed phases
- Implemented functionality
- Architectural decisions
- Test results
- Known limitations
- Pending phases

Only mark a phase complete when acceptance criteria have been satisfied.

## 12. Agent Completion Report

After each implementation phase, provide:

**Completed:** Implemented functionality.

**Files Changed:** Relevant files.

**Architecture Decisions:** Important choices and rationale.

**Validation:** Actual build, test, and verification results.

**Limitations:** Known issues or incomplete functionality.

**Next Phase:** The next planned implementation stage.

Do not proceed to the next phase automatically.

## 13. Current Instruction

This document defines the DevOrbit project context.

**DO NOT IMPLEMENT ANYTHING YET.**

Wait for an explicit implementation sub-prompt before modifying the repository.