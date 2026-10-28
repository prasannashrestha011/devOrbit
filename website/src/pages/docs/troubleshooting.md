---
layout: ../../layouts/Docs.astro
label: Troubleshooting
title: Troubleshooting
description: Resolve connection, service, terminal, and inspection problems in your local workspace.
source: server/README.md
---

## The desktop cannot connect

Reopen DevOrbit to restart its bundled backend. If startup fails, read the error in the startup dialog. The default health endpoint is `http://127.0.0.1:7456/health`.

### Check connection settings

Export custom host, port, data directory, or token settings before opening the desktop. The desktop passes these settings to its backend. A port already in use or an inaccessible data directory can prevent startup. Review the [backend settings](/docs/configuration/#backend-settings).

### Check authentication

The health endpoint is public; `/api/v1` requests require a bearer token. The desktop's Rust host supplies it automatically. For direct API calls, read the configured token file locally and pass the token as described in [API authentication](/docs/api/#connect-and-authenticate).

## A service fails to start

Check its command, working directory, installed dependencies, and terminal output. Commands run as executable arguments: pipes, shell expansion, and `&&` are not interpreted automatically. Use an explicit shell when your command needs shell syntax.

Relative working directories must remain inside the workspace root. Use an absolute `cwd` for a project elsewhere. After saving a configuration change, restart the service to apply it. See [workspace configuration](/docs/configuration/).

A **running** state confirms that the process launched, not that the application is ready. DevOrbit does not automatically restart failed processes or resolve startup dependencies.

## A terminal will not open

Linux and macOS service terminals require `tmux`. Windows uses ConPTY. For workspace shell terminals, the limit is 16 sessions, including exited sessions that have not been closed; close an unused session before creating another.

Closing a terminal viewer detaches it without stopping the service. Reopen it to reconnect. See [interactive terminals](/docs/services/#use-interactive-terminals).

## Docker is unavailable

Confirm that Docker is installed, the daemon is running, and Compose v2 is available to the user running DevOrbit. Check that a supported Compose file is in the workspace root or a configured service working directory, then use **Refresh** in the Docker panel.

If a container keeps restarting, open its logs and inspect its health status. See [Docker integration](/docs/docker/).

## The inspector is empty

Start the service, select its HTTP port, and enable inspection. Send requests to the temporary gateway URL shown in DevOrbit; direct calls to the service are not captured. HTTPS interception and WebSocket frame capture are not supported.

## Report a reproducible issue

Include your operating system, DevOrbit version or source revision, steps to reproduce, and relevant error text in a [GitHub issue](https://github.com/prasannashrestha011/devOrbit/issues). Remove tokens, credentials, private paths, and sensitive request bodies from shared logs or captures.
