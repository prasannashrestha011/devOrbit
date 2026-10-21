# DevOrbit — YAML Workspace Configuration Requirements

## Objective

Allow developers to define and manage workspace services using a simple YAML configuration file instead of manually registering every service.

**This is a requirements document, not an implementation instruction. Analyze the requirements and propose an implementation plan. Do not modify any code.**

## 1. YAML Configuration

The default configuration file should be:

`.devorbit/workspace.yaml`

### Final YAML Schema

```yaml
services:
  backend:
    command: go run ./cmd/api
    cwd: ./backend
    ports:
      http: 8080
      metrics: 9090

  frontend:
    command: npm run dev -- --port 5173
    cwd: ./frontend
    ports:
      http: 5173

  worker:
    command: go run ./cmd/worker
    cwd: ./backend
```

### Supported Fields

- `services`: Required mapping of service definitions.
- Service key: Required unique identifier within the workspace.
- `command`: Required executable command with arguments.
- `cwd`: Optional working directory, relative to the workspace root. Defaults to the workspace root.
- `ports`: Optional mapping of named ports to integers between 1 and 65535.

The YAML must remain minimal.

Do not introduce version fields, workspace metadata, dependencies, readiness checks, tasks, setup commands, or additional configuration sections.

Ports represent expected listening ports, not port mappings. Declaring ports must not automatically modify commands, bind sockets, or inject environment variables.

## 2. Workspace Configuration Options

After creating or opening a workspace, developers should be able to:

1. Create and write YAML directly inside DevOrbit.
2. Import an existing YAML file.
3. Edit an existing configuration.
4. Validate the configuration.
5. Preview the defined services.
6. Save the configuration.
7. Apply the configuration to the workspace.

Both manually written and imported YAML must follow the same validation and application rules.

## 3. Validation

Validate:

- YAML syntax and structure.
- Required service definitions.
- Unique and valid service keys.
- Required executable commands.
- Working directory existence and validity.
- Port names and valid port ranges.
- Unsupported fields.
- Invalid configuration types.
- Unsupported shell syntax.

The system should display meaningful validation errors before applying changes.

Parsing, previewing, importing, and saving YAML must never execute commands.

## 4. Command Execution

Commands are defined as human-readable strings.

Example:

`command: go run ./cmd/api`

DevOrbit should interpret this as an executable and arguments using its existing process execution system.

Requirements:

- Support ordinary commands with arguments.
- Handle quoted arguments correctly.
- Do not implicitly execute commands through a system shell.
- Reject unsupported shell operators.
- Preserve existing process and terminal behavior.

## 5. Configuration Synchronization

YAML should be the source of truth for YAML-managed service configuration.

Applying YAML should synchronize services with the workspace's existing service management system.

Requirements:

- Create newly defined services.
- Update existing services when configurations change.
- Preserve unchanged services.
- Preserve stable service identities.
- Detect removed services and preview their removal.
- Require explicit confirmation before destructive changes.
- Handle running services safely.
- Avoid partially applied changes.
- Detect unapplied YAML changes.

Saving YAML and applying YAML must be separate operations.

Applying YAML must not automatically start or restart services.

## 6. Backward Compatibility

Existing workspaces and manually configured services must continue functioning.

Requirements:

- YAML configuration must be optional.
- Do not unexpectedly overwrite manually configured services.
- Preserve existing service settings that are not represented in YAML.
- Maintain compatibility with existing terminals, logs, process supervision, and workspace orchestration.
- Define clear behavior when converting a manually managed workspace to YAML management.

## 7. User Experience

The workspace interface should provide:

- YAML editor with syntax highlighting.
- Import YAML option.
- Validation errors and warnings.
- Structured service preview.
- Configuration change preview showing added, updated, unchanged, and removed services.
- Save and Apply actions.
- Confirmation before applying configuration changes.
- Clear feedback for successful or failed operations.

Importing an external YAML file must not silently overwrite an existing configuration.

## 8. Security and Safety

- Treat imported configurations as untrusted until reviewed.
- Never execute commands during import, preview, or save.
- Require explicit user action to execute services.
- Prevent unintended deletion of running services.
- Validate filesystem paths.
- Preserve existing authentication and execution security boundaries.
- Do not introduce a separate process execution system.

## 9. Agent Planning Instructions

Analyze the existing DevOrbit project and prepare an implementation plan.

Your response should include:

1. **Requirement Analysis:** Explain your understanding of the feature.
2. **Existing Functionality:** Identify which current components can be reused.
3. **Proposed Design:** Describe how YAML creation, import, validation, saving, and application should work.
4. **Configuration Synchronization:** Explain how YAML services map to existing services while preserving identities.
5. **Required Changes:** Identify affected backend and desktop components.
6. **Implementation Phases:** Divide the feature into small, independently testable implementation stages.
7. **Edge Cases:** Identify conflicts, failures, and backward-compatibility concerns.
8. **Open Decisions:** Recommend solutions for any unresolved design questions.

Keep the solution simple and aligned with the minimal YAML schema.

**Do not implement anything yet. Do not modify files. Return the proposed implementation plan and wait for separate implementation instructions.**