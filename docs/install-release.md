# Install a DevOrbit release

Download the desktop installer for your platform. It includes the Go backend and SQLite; opening DevOrbit starts the backend and creates the local database automatically.

| Platform | Installer |
| --- | --- |
| Linux x64 (Ubuntu/Debian) | `.deb` |
| Windows x64 | `-setup.exe` |
| macOS Apple Silicon | `_aarch64.dmg` |
| macOS Intel | `_x64.dmg` |

These are unsigned development releases. macOS builds use ad-hoc signing without Apple notarization; Windows builds have no publisher certificate. Your OS may block or warn about them. Review the draft and test installation before publishing it to users.

## Install and run

On Ubuntu, install the `.deb` with `sudo apt install ./<downloaded-file>.deb` so its desktop dependencies are installed. On Windows, run the installer. On macOS, copy DevOrbit from the disk image into Applications. Then open DevOrbit normally.

Install `tmux` on Linux/macOS to use local service terminals. Docker with Compose v2 is required only for managing Docker Compose projects. Managed projects still need their own runtimes and tools. Go, Node.js, Rust, a C compiler, and a SQLite CLI are not needed to run the installed app.

DevOrbit starts its bundled backend on `127.0.0.1:7456`, waits for it to become ready, and stops it when the desktop closes. If that port is already in use or startup fails, DevOrbit shows an error. Optional `DEVORBIT_*` settings must be exported before opening the app; the desktop passes its data directory and token path to the backend. Installed builds do not read `server/.env`.

The backend creates `devorbit.db` and the private API token in the user's DevOrbit config directory. Before upgrading, close DevOrbit and back up the entire private data directory, including `devorbit.db`, workspace YAML, logs, and captured traffic. The backend applies schema migrations on startup. Keep the data directory on a local disk with private permissions.

This release starts with a fresh SQLite database. Earlier PostgreSQL workspaces are not imported.

## Verify downloads

Tagged releases include `SHA256SUMS`. After downloading the installer and checksum file, compare its digest with the listed value using `sha256sum -c SHA256SUMS` on Linux, `shasum -a 256 -c SHA256SUMS` on macOS, or `Get-FileHash <downloaded-file> -Algorithm SHA256` on Windows. The Linux/macOS commands report missing files when other platforms' installers are not downloaded; compare the line for your installer.
