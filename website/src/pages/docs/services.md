---
layout: ../../layouts/Docs.astro
label: Services & terminals
title: Follow every process.
description: Start your services, work in their terminals, and inspect the output without losing your workspace context.
source: server/README.md
---

## Control a service

**Start** loads the saved configuration and launches the executable. **Stop** asks it to terminate, then forces termination after the shutdown timeout. **Restart** stops the current execution, reloads configuration, and launches a new execution.

Runtime states are `stopped`, `starting`, `running`, `stopping`, `exited`, and `failed`. A running state confirms that the OS started the process; it is not an application readiness check. An exited or failed service can be started again. Runtime state resets when the backend restarts.

Workspace start attempts services in YAML key order. A failure does not prevent other services from being attempted, and successful starts are not rolled back. DevOrbit does not resolve service dependencies or automatically restart failed processes.

## Use interactive terminals

Every service execution has a PTY session. The desktop uses xterm.js for the terminal, with input, resizing, and raw output forwarded over an authenticated WebSocket. The Rust host keeps the bearer token out of the renderer.

On Linux and macOS, service terminals use private tmux sessions; Windows uses ConPTY. Closing a terminal viewer detaches it without stopping its service. The backend continues draining output and retains up to 1 MiB for reconnect replay.

Workspace shell terminals are separate sessions in the workspace root. Up to 16 sessions can exist per workspace, including exited sessions not yet closed. Shell output is not written to service logs. Backend shutdown ends these sessions.

## Read logs and events

Service logs are persisted locally and remain queryable after a backend restart. New PTY output uses the merged `terminal` stream; older `stdout` and `stderr` entries remain available. Logs rotate at 10 MiB, with five files per execution and up to 20 executions per service retained for seven days.

Live status and log updates use Server-Sent Events. SSE has no reconnect replay; use the persisted log endpoints for history. Applications may print secrets, so both streams and saved logs require authentication.

## Inspect local HTTP traffic

Choose a detected HTTP port from a running service and enable its temporary inspection gateway. Send requests to the gateway URL to capture requests and responses. Calls directly to the service bypass inspection.

History is saved privately in the data directory, retaining the newest 500 exchanges per session for seven days, within a shared 512 MiB quota. When full, forwarding continues but a capture may retain only part of its body. Captures can contain credentials. HTTPS interception and WebSocket frame capture are not supported.

## Keep controls nearby

Use **Quick controls** in the desktop toolbar for an always-on-top window that follows the selected workspace. It shows local and Docker Compose service names with start, stop, and restart actions. Closing this window leaves services running.
