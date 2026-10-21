# DevOrbit — Service Environment File Discovery and Integrated Editor

## Objective

Enhance the existing DevOrbit application to automatically discover and manage `.env` files belonging to each configured workspace service.

Users should be able to view, edit, and save environment files directly inside DevOrbit without opening external editors or modal dialogs.

**CRITICAL: This is an incremental enhancement to the current implementation, not a rewrite. Preserve all existing service functionality, architecture, UI components, process management, and navigation behavior.**

## 1. Automatic Environment File Discovery

When a user selects or navigates to a service, DevOrbit should automatically discover environment files belonging to that specific service.

Supported filenames include:

- `.env`
- `.env.local`
- `.env.development`
- `.env.production`
- `.env.test`
- `.env.example`
- Other files matching `.env.*`

Requirements:

- Use the configured service working directory as the discovery root.
- Associate discovered files with their respective service.
- Support services with one, multiple, or no environment files.
- Discover files when opening the service and support refreshing the list.
- Do not scan unrelated services or workspace directories.
- If nested discovery is necessary, keep it bounded and exclude irrelevant folders such as `node_modules`, `.git`, `dist`, and build output.
- Never automatically create or modify environment files during discovery.

Example:

```text
Workspace
├── backend/
│   ├── .env
│   ├── .env.development
│   └── main.go
├── frontend/
│   ├── .env.local
│   └── package.json
└── worker/
    ├── .env.production
    └── main.go
```

Expected discovery:

- Backend → `.env`, `.env.development`
- Frontend → `.env.local`
- Worker → `.env.production`

Each service must display only its own environment files.

## 2. Integrated Environment File Editor

Add an environment-file editor within the existing service details interface.

**Do not use modal dialogs, popups, or separate windows for editing.**

Expected behavior:

1. User navigates to a service.
2. DevOrbit discovers its `.env` files.
3. The service interface displays the discovered files in an appropriate environment section or tab.
4. User selects an environment file.
5. Its raw contents open in an integrated editor within the existing layout.
6. User edits the content.
7. User explicitly saves the changes.

Editor requirements:

- Prefer the code editor already used in the application. If none exists, use an appropriate integrated text editor consistent with the current stack.
- Provide syntax highlighting for dotenv files when supported.
- Display the selected filename and service-relative path.
- Support multiline editing, scrolling, and standard keyboard shortcuts.
- Provide Save and Reload actions.
- Indicate unsaved changes.
- Show clear loading, empty, and error states.
- Never silently overwrite unsaved changes.
- Never automatically save on navigation.

**Important:** Treat `.env` files as raw text. Do not rebuild the entire file using a key-value parser, as this can remove comments, blank lines, ordering, quoting, or formatting.

## 3. Preserve Existing Service Implementations

This is a non-destructive feature addition.

Do NOT:

- Rewrite existing service management functionality.
- Replace the current service details interface.
- Change service start, stop, restart, or terminal behavior.
- Modify process supervisor logic unless absolutely necessary.
- Alter workspace configuration behavior unnecessarily.
- Override existing implementations when navigating between services.
- Change unrelated service settings.
- Refactor unrelated components simply to accommodate the new feature.

Reuse the current service IDs, workspace relationships, backend conventions, API patterns, and frontend architecture.

Maintain a clear separation between environment-file management and existing service execution functionality.

## 4. Service Navigation and State Isolation

Environment editor state must be isolated by workspace, service, and file.

Example:

```text
Backend Service
  .env → Editor A

Frontend Service
  .env.local → Editor B

Worker Service
  .env.production → Editor C
```

Switching from Backend to Frontend must not replace, save, or corrupt Backend's environment content.

Requirements:

- Always associate file reads and writes with the correct workspace and service.
- Avoid shared mutable editor state that could mix service contents.
- Preserve unsaved edits when navigating between services during the current session, or request confirmation before discarding them.
- On returning to a service, restore its unsaved draft when available.
- Keep existing service tabs, terminal sessions, logs, statuses, and operations functional.
- Ensure asynchronous reads cannot display a previous service's file after navigation.

## 5. Backend Implementation — Go

Extend the existing Go backend using its current module structure and established API conventions.

Implement the minimum necessary operations:

- List environment files for a specific service.
- Read the raw contents of a selected environment file.
- Save updated raw contents to that same file.

Requirements:

- Resolve paths from the existing service configuration.
- Validate workspace ownership and service identity.
- Prevent directory traversal and unauthorized access outside the permitted service directory.
- Handle symbolic links safely.
- Do not allow arbitrary filesystem paths supplied by the frontend.
- Preserve existing file permissions where possible.
- Use safe write behavior, preferably an atomic replacement strategy.
- Detect external file changes before saving and prevent silent overwrites.
- Return meaningful errors for missing files, permission failures, and concurrent modifications.
- Never log environment file contents or secret values.

Do not introduce a new persistence model unless required. Environment files should remain filesystem-backed.

## 6. Frontend Implementation — Electron + TypeScript

Integrate the feature into the existing service details view.

Suggested interface:

```text
┌──────────────────────────────────────────────────┐
│ Backend Service                                  │
├──────────────────────────────────────────────────┤
│ Overview | Terminal | Logs | Environment         │
├──────────────────────────────────────────────────┤
│ Environment Files                       Refresh  │
│                                                  │
│  .env                 Selected                  │
│  .env.development                               │
│                                                  │
│  Editor: .env                                    │
│ ┌──────────────────────────────────────────────┐ │
│ │ PORT=8080                                    │ │
│ │ DATABASE_HOST=localhost                     │ │
│ │ APP_ENV=development                          │ │
│ │                                              │ │
│ └──────────────────────────────────────────────┘ │
│                                   Reload  Save  │
└──────────────────────────────────────────────────┘
```

This is a conceptual layout, not a requirement to replace the current UI.

Follow existing navigation, styling, component patterns, and state management.

If no `.env` file is discovered, display a friendly empty state without affecting other service controls.

## 7. Security and Data Integrity

Environment files may contain sensitive credentials.

Implement appropriate safeguards:

- Do not expose file contents in logs, telemetry, or error messages.
- Do not copy raw environment values into unrelated service configuration or database records.
- Only read files when needed.
- Do not automatically inject edited values into running processes.
- Do not automatically restart services after saving.
- Keep the original file unchanged until the user explicitly saves.
- Warn about potential conflicts when the file has changed externally.
- Preserve the file's original content and formatting unless the user changes it.

Saving a `.env` file and restarting a service must remain separate operations.

## 8. Testing and Acceptance Criteria

Implement tests for the following scenarios:

1. A service containing one `.env` file displays that file correctly.
2. A service containing multiple `.env` files displays all supported files.
3. A service without environment files shows an appropriate empty state.
4. Editing and saving updates only the selected file.
5. Switching services does not mix environment contents.
6. Unsaved content is not silently discarded or saved during navigation.
7. Existing terminal, log, and service lifecycle operations remain unaffected.
8. Malicious paths cannot access files outside the permitted directory.
9. External modifications produce conflict handling instead of silent overwriting.
10. Comments, blank lines, formatting, and line endings are preserved when editing and saving.
11. A stale asynchronous file response cannot replace the editor contents of a newly selected service.

## 9. Implementation Approach

Before modifying code:

1. Inspect the current workspace and service architecture.
2. Identify the existing service detail components, routes, backend handlers, and frontend state management.
3. Check whether environment-file discovery or editing is already partially implemented.
4. Reuse existing functionality rather than creating duplicate implementations.
5. Make focused, minimal changes to extend the current system.
6. Add or update relevant tests.
7. Run applicable checks and verify no existing features have regressed.

Do not introduce unnecessary dependencies or architectural changes.

## Expected Deliverables

After implementation, provide:

- A summary of existing behavior that was reused.
- The files changed and why.
- The new environment-file discovery and editing flow.
- Any API endpoints added or modified.
- Details of service-specific state isolation.
- Tests executed and their results.
- Any limitations or unfinished items.

**Final requirement:** DevOrbit should automatically show the environment files available for each selected service and let users edit them within the existing service interface, without modal dialogs, accidental overwrites, or disruption to any previously implemented service feature.
