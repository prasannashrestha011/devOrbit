---
layout: ../../layouts/Docs.astro
label: Installation
title: Run a prebuilt release.
description: Install the desktop and its matching backend, then connect them to a local PostgreSQL database.
source: docs/install-release.md
---

<aside class="callout">Download both the desktop installer and the matching backend archive. The desktop does not bundle or start the backend yet.</aside>

## Choose your platform

Check the [GitHub releases](https://github.com/prasannashrestha011/devOrbit/releases) for available assets. Tagged builds create draft releases; a draft must be published before it is publicly downloadable. If no release is available, [build from source](/docs/).

| Platform | Desktop installer | Backend archive suffix |
| --- | --- | --- |
| Linux x64 (Ubuntu/Debian) | `.deb` | `linux-x64.tar.gz` |
| Windows x64 | `-setup.exe` | `windows-x64.tar.gz` |
| macOS Apple Silicon | `_aarch64.dmg` | `macos-arm64.tar.gz` |
| macOS Intel | `_x64.dmg` | `macos-x64.tar.gz` |

Development releases are unsigned: macOS builds use ad-hoc signing without notarization, and Windows builds have no publisher certificate. Your OS may warn or block installation.

## Install and configure

Install Docker with Compose v2 or use an existing PostgreSQL 17 database. Linux and macOS need `tmux` for service terminals. Your managed projects still need their own runtimes; Go, Node.js, and Rust are not needed just to run the downloaded DevOrbit programs.

Install the desktop package for your OS. On Ubuntu, use `sudo apt install ./<downloaded-file>.deb` so native dependencies are installed. On macOS, copy DevOrbit from the disk image into Applications; on Windows, run the installer.

Extract the backend archive into a permanent directory. Copy `server/.env.example` to `server/.env`, then replace both `change-me` password entries with the same private password. URL-encode special characters in the database URL password.

## Start both programs

From the extracted backend directory:

```sh
docker compose -f server/compose.yaml up -d --wait
```

Start the backend and leave its terminal open:

```sh
# Linux / macOS
./devorbit
```

```powershell
# Windows PowerShell
.\devorbit.exe
```

Open the desktop as the same OS user. Both programs default to `127.0.0.1:7456` and the user's DevOrbit config directory. For custom paths or ports, use the same [exported settings](/docs/configuration/#backend-settings) for both programs.

## Verify and upgrade

Release assets include `SHA256SUMS`. Compare each downloaded asset's SHA-256 digest with its entry. Linux can use `sha256sum -c SHA256SUMS`; macOS can use `shasum -a 256 -c SHA256SUMS`. On Windows, use `Get-FileHash <downloaded-file> -Algorithm SHA256`. Missing files are reported if you downloaded only one platform.

Back up PostgreSQL and the private DevOrbit data directory before upgrading: the backend applies migrations on startup. Keep the existing `.env` and database volume. To stop the database without deleting it:

```sh
docker compose -f server/compose.yaml stop
```

`down -v` deletes the database volume. Changing `.env` does not change the password of an already initialized PostgreSQL role.
