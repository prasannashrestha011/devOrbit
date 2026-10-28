---
layout: ../../layouts/Docs.astro
label: Workspace configuration
title: Workspace configuration
description: Keep service commands in one readable file, with a clear source of truth for every workspace.
source: server/README.md
---

## Define your services

DevOrbit creates `.devorbit/workspace.yaml` in each workspace root. Add services in the desktop, import YAML through the Workspace YAML editor, or edit that file directly.

```yaml
services:
  api:
    command: go run ./cmd/api
    cwd: backend
  web:
    command: npm run dev
    cwd: frontend
  worker:
    command: python worker.py
```

These are example commands; the corresponding files and runtimes must exist in your project. The format accepts a top-level `services` mapping, with `command` and an optional `cwd` per service. Port mappings are not part of this format.

## Commands and directories

Commands are split into an executable and arguments and run directly. Shell expansion, pipes, `&&`, and shell variable substitution are not performed. Put compound commands in a project script and run that script with its interpreter when needed.

Omitting `cwd` uses the workspace root. Relative paths must stay within that root. An absolute path may point to another existing directory, but makes the configuration specific to your machine.

Services inherit the backend's environment. Export required variables before starting the backend. Only run configurations you trust: services execute as your OS user, and DevOrbit is not a process sandbox.

## Save, start, and restart

The API validates YAML before saving it atomically. Each start loads a fresh, validated configuration snapshot. Editing a running service affects its next start; changes never trigger an automatic restart.

Keeping a service's YAML key preserves its UUID and log association. Renaming the key creates a new service identity. SQLite stores these identities and workspace metadata; YAML is the only source of service configuration.

API writes require the current configuration `revision`. If another edit wins first, the API returns `409`; fetch the latest content, reconcile the edit, and retry. External file edits appear on the next request and desktop refresh.

## Backend settings

When running the backend alone for development, it reads `server/.env`; exported environment variables take precedence. The desktop starts its bundled backend with exported settings and does not read `server/.env`. Export `DEVORBIT_HOST`, `DEVORBIT_PORT`, `DEVORBIT_DATA_DIR`, or `DEVORBIT_AUTH_TOKEN_PATH` before launching the desktop if you need custom values.

| Variable | Default |
| --- | --- |
| `DEVORBIT_HOST` | `127.0.0.1` (loopback IPs only) |
| `DEVORBIT_PORT` | `7456` |
| `DEVORBIT_DATA_DIR` | User config directory + `devorbit` |
| `DEVORBIT_LOG_DIR` | Data directory + `/logs` |
| `DEVORBIT_AUTH_TOKEN_PATH` | Data directory + `/auth-token` |
| `DEVORBIT_SHUTDOWN_TIMEOUT` | `10s` |

Custom paths must be absolute. Keep the data directory and authentication token private. The backend rejects token files or token directories with permissions broader than its private-access requirements. Logs and captured HTTP traffic can contain secrets from your applications.

The backend creates `devorbit.db` in the data directory automatically. Stop the backend before backing up that directory.
