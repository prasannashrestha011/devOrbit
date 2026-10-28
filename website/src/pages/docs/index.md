---
layout: ../../layouts/Docs.astro
label: Getting started
title: Getting started
description: Install DevOrbit or build it from source, then bring your first project into the app.
source: README.md
---

<aside class="callout">The desktop starts its bundled backend automatically. Install a release and open DevOrbit, or build from source below.</aside>

## Before you start

Clone the [DevOrbit repository](https://github.com/prasannashrestha011/devOrbit) and open a terminal in its root directory. To build from source, install:

| Tool | Used for |
| --- | --- |
| Go 1.24+ | Building the backend |
| C compiler (GCC / Clang / MinGW-w64) | Building the embedded SQLite driver with CGO |
| Node.js + npm | Building the desktop interface |
| Rust + Cargo | Building the Tauri desktop host |
| tmux (Linux / macOS) | Interactive service terminals |
| [Tauri system dependencies](https://v2.tauri.app/start/prerequisites/) | Native desktop libraries for your OS |

Docker with Compose v2 is only needed to manage Docker Compose projects. If you prefer prebuilt programs, follow the [release installation guide](/docs/installation/).

## 1. Prepare a source build

SQLite is embedded in the backend. It creates `devorbit.db` in the private DevOrbit data directory on first startup. No database server or credentials are required.

Export optional settings before launching the desktop. See the [backend setup instructions](https://github.com/prasannashrestha011/devOrbit/blob/main/server/README.md#requirements-and-setup) for compiler requirements and configuration.

## 2. Launch DevOrbit

From the repository root, install the desktop dependencies and launch:

```sh
cd desktop && npm ci && cd ..
./dev.sh
```

The Bash script launches Tauri, which builds and starts the backend. Closing the desktop stops the backend. The SQLite database stays on disk for the next launch.

On Windows, run `npm ci` and `npm run tauri -- dev` from `desktop/`; Tauri builds and starts the backend there too.

## 3. Add your first services

Create a workspace in the desktop, then add services through its service form or Workspace YAML editor. Each workspace has a `.devorbit/workspace.yaml` file. Use the workspace root shown in the app when choosing working directories; an absolute `cwd` can point to an existing project elsewhere on your machine.

Start a service, open its terminal, and follow its logs. A running status means the process launched; your application may still be starting up. Continue with [workspace configuration](/docs/configuration/) for command and path examples.

## If the desktop cannot connect

Reopen DevOrbit to restart its backend. Startup errors are shown in a dialog. The default health endpoint is `http://127.0.0.1:7456/health`.

If you change the host, port, data directory, or token path, export the settings before launching DevOrbit. The desktop passes them to its backend. See [connection settings](/docs/configuration/#backend-settings).
