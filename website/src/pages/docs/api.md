---
layout: ../../layouts/Docs.astro
label: API reference
title: API reference
description: The core routes for workspace configuration, service lifecycle, terminal streams, and persisted logs.
source: server/README.md
---

## Connect and authenticate

The default base URL is `http://127.0.0.1:7456`. `GET /health` is unauthenticated and checks SQLite connectivity; it returns `503` when the database is unavailable.

Every `/api/v1` route requires `Authorization: Bearer <token>`. The token lives at `DEVORBIT_AUTH_TOKEN_PATH` (by default, `auth-token` in the DevOrbit data directory). Keep it private and out of URLs, browser code, and source control. The Tauri host handles authentication for the desktop.

```sh
curl --fail http://127.0.0.1:7456/health
```

JSON writes require `Content-Type: application/json`, with a 1 MiB request-body limit. The server checks the configured Host and restricts browser Origin headers to the API's own origin.

## Workspaces and configuration

All paths below start with `/api/v1`.

| Method | Path | Purpose |
| --- | --- | --- |
| GET / POST | `/workspaces` | List or create workspaces |
| GET / PATCH / DELETE | `/workspaces/{id}` | Read, update, or delete a workspace |
| POST | `/workspaces/{id}/start` | Attempt all services in YAML order |
| POST | `/workspaces/{id}/stop` | Stop active managed services |
| GET | `/workspaces/{id}/status` | Runtime states and aggregate counts |
| GET / PUT | `/workspaces/{id}/configuration` | Read or save YAML with a revision |
| POST | `/workspaces/{id}/configuration/validate` | Validate content without saving |

Workspace and service lists accept `limit` (1–100, default 50) and `offset` (nonnegative, default 0). A workspace cannot be deleted while its services or shell terminals are running.

## Services and lifecycle

| Method | Path | Purpose |
| --- | --- | --- |
| GET / POST | `/workspaces/{id}/services` | List services or add one to YAML |
| GET / PATCH / DELETE | `/services/{id}` | Read, update, or remove a service |
| POST | `/services/{id}/start` | Launch; returns `202` |
| POST | `/services/{id}/stop` | Stop; returns `200`, including already stopped |
| POST | `/services/{id}/restart` | Reload and restart; returns `200` |
| GET | `/services/{id}/status` | Current runtime status |

Create a service with a JSON body such as:

```json
{
  "name": "api",
  "command": "go",
  "args": ["run", "./cmd/api"],
  "working_directory": "backend",
  "revision": "<current YAML revision>"
}
```

Service create and patch bodies require the current YAML `revision`; deletion uses `?revision=<revision>`. Stale revisions, duplicate keys, and conflicting lifecycle operations return `409`. Removing a running service is rejected.

## Terminals, logs, and events

| Transport | Path | Purpose |
| --- | --- | --- |
| WebSocket | `/services/{id}/terminal/{execution_id}` | Service input, output, and resize |
| GET / POST | `/workspaces/{id}/terminals` | List or create shell sessions |
| DELETE | `/workspaces/{id}/terminals/{terminal_id}` | Stop and remove a shell session |
| WebSocket | `/workspaces/{id}/terminals/{terminal_id}/stream` | Workspace shell stream |
| GET | `/services/{id}/logs` | Persisted service logs |
| GET | `/workspaces/{id}/logs` | Persisted workspace logs |
| SSE | `/events` | Live process and log events |

Terminal frames accept `{"type":"input","data":"text"}`, `{"type":"resize","cols":80,"rows":24}`, and `{"type":"close"}`. Output is base64-encoded with a sequence number. `?after=<sequence>` requests recent output replay; a `gap` frame reports expired output.

Log queries accept `limit` (1–500, default 100) and `before` (the previous response's `next_cursor`). Entries arrive newest first. SSE can be filtered by `workspace_id` or `service_id` and does not replay missed events.

## Errors and full reference

```json
{
  "error": {
    "code": "...",
    "message": "..."
  }
}
```

Invalid YAML returns `400`, and successful deletes return `204` without a body. For the full behavior, retention details, and current limitations, read the [backend reference](https://github.com/prasannashrestha011/devOrbit/blob/main/server/README.md#api). The [traffic inspector routes](https://github.com/prasannashrestha011/devOrbit/blob/main/server/internal/modules/traffic_inspector/routes.go) define the inspection-session and captured-body endpoints.
