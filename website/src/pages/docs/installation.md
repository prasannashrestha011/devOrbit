---
layout: ../../layouts/Docs.astro
label: Installation
title: Installation
description: One desktop installer includes the backend and its local SQLite database.
source: docs/install-release.md
---

<aside class="callout">Download one desktop installer for your platform. Opening DevOrbit starts its bundled backend automatically.</aside>

## Choose your platform

Check the [GitHub releases](https://github.com/prasannashrestha011/devOrbit/releases) for available assets. Tagged builds create draft releases; a draft must be published before it is publicly downloadable. If no release is available, [build from source](/docs/).

| Platform | Installer |
| --- | --- |
| Linux x64 (Ubuntu/Debian) | `.deb` |
| Windows x64 | `-setup.exe` |
| macOS Apple Silicon | `_aarch64.dmg` |
| macOS Intel | `_x64.dmg` |

Development releases are unsigned: macOS builds use ad-hoc signing without notarization, and Windows builds have no publisher certificate. Your OS may warn or block installation.

## Install and launch

Linux and macOS need `tmux` for local service terminals. Your managed projects still need their own runtimes; Go, Node.js, Rust, a C compiler, and SQLite are not needed just to run DevOrbit. Docker with Compose v2 is needed only for Docker Compose projects.

On Ubuntu, use `sudo apt install ./<downloaded-file>.deb` so native dependencies are installed. On macOS, copy DevOrbit from the disk image into Applications; on Windows, run the installer. Open DevOrbit normally. It creates `devorbit.db` and a private API token in your DevOrbit config directory, then starts its bundled backend.

DevOrbit shows an error if the backend cannot start or its local port is already in use. For custom paths or ports, export [backend settings](/docs/configuration/#backend-settings) before opening the app. Installed builds do not read `server/.env`.

## Verify and upgrade

Release assets include `SHA256SUMS`. Compare your installer's SHA-256 digest with its entry. Linux can use `sha256sum -c SHA256SUMS`; macOS can use `shasum -a 256 -c SHA256SUMS`. On Windows, use `Get-FileHash <downloaded-file> -Algorithm SHA256`. Missing files are reported if you downloaded only one platform.

Close DevOrbit and back up the private data directory before upgrading, including `devorbit.db`, workspace YAML, logs, and captured traffic. The backend applies schema migrations on startup. Keep the data directory on a local disk with private permissions. This release starts with a fresh SQLite database; earlier PostgreSQL workspaces are not imported.
