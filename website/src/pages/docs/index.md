---
layout: ../../layouts/Docs.astro
label: Getting started
title: From checkout to running services.
description: Set up the local backend, launch the desktop, and bring your first project into DevOrbit.
source: README.md
---

<aside class="callout">The desktop and backend run separately. The development script starts both for you.</aside>

## Before you start

Clone the [DevOrbit repository](https://github.com/prasannashrestha011/devOrbit) and open a terminal in its root directory. To build from source, install:

| Tool | Used for |
| --- | --- |
| Go 1.24+ | Building the backend |
| PostgreSQL 17 | Workspace metadata and service identities |
| Node.js + npm | Building the desktop interface |
| Rust + Cargo | Building the Tauri desktop host |
| tmux (Linux / macOS) | Interactive service terminals |
| [Tauri system dependencies](https://v2.tauri.app/start/prerequisites/) | Native desktop libraries for your OS |

Docker with Compose v2 is needed for the database commands below. If you prefer prebuilt programs, follow the [release installation guide](/docs/installation/).

## 1. Prepare the backend

Copy the example environment file:

```sh
cd server
cp -n .env.example .env
```

Edit `.env` before starting the database. Replace `change-me` with the same private password in `DEVORBIT_DB_PASSWORD` and `DEVORBIT_DATABASE_URL`. URL-encode special characters in the connection-string password.

```sh
docker compose up -d --wait
```

Already running PostgreSQL 17? Create a database and set `DEVORBIT_DATABASE_URL` in `.env` instead. See the [backend setup instructions](https://github.com/prasannashrestha011/devOrbit/blob/main/server/README.md#requirements-and-setup).

## 2. Launch DevOrbit

From the `server/` directory, return to the repository root, install the desktop dependencies, and launch:

```sh
cd ..
cd desktop && npm ci && cd ..
./dev.sh
```

The Bash script builds the backend, waits for its health check, and opens the desktop. Closing the desktop or pressing Ctrl+C stops the backend. PostgreSQL keeps running; `docker compose -f server/compose.yaml stop` stops it while keeping your data.

On Windows, use separate terminals: run `go run ./cmd/devorbit` from `server/`, then `npm ci` and `npm run tauri -- dev` from `desktop/`. Leave the backend terminal open while using the desktop.

## 3. Add your first services

Create a workspace in the desktop, then add services through its service form or Workspace YAML editor. Each workspace has a `.devorbit/workspace.yaml` file. Use the workspace root shown in the app when choosing working directories; an absolute `cwd` can point to an existing project elsewhere on your machine.

Start a service, open its terminal, and follow its logs. A running status means the process launched; your application may still be starting up. Continue with [workspace configuration](/docs/configuration/) for command and path examples.

## If the desktop cannot connect

Check that PostgreSQL and the backend are running. The default health endpoint is `http://127.0.0.1:7456/health`; it returns `503` if the database is unavailable.

If you change the host, port, data directory, or token path, export the same settings for both processes. The desktop does not read `server/.env`. See [connection settings](/docs/configuration/#backend-settings).
