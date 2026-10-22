# Install a DevOrbit release

Download the desktop installer and the matching `devorbit-backend-<version>-<platform>.tar.gz`. Both are required: the desktop does not start or bundle the backend yet.

| Platform | Desktop installer | Backend archive suffix |
| --- | --- | --- |
| Linux x64 (Ubuntu/Debian) | `.deb` | `linux-x64.tar.gz` |
| Windows x64 | `-setup.exe` | `windows-x64.tar.gz` |
| macOS Apple Silicon | `_aarch64.dmg` | `macos-arm64.tar.gz` |
| macOS Intel | `_x64.dmg` | `macos-x64.tar.gz` |

These are unsigned development releases. macOS builds use ad-hoc signing, without Apple notarization; Windows builds have no publisher certificate. Your OS may block or warn about them. Review the draft and test installation before publishing it to users.

## Prerequisites

Install Docker with Compose v2 for PostgreSQL, or use an existing PostgreSQL 17 database. Install `tmux` on Linux/macOS for service terminals. The projects you manage still need their own runtimes and tools on your machine. Go, Node.js, and Rust are not needed just to run the downloaded DevOrbit programs.

On Ubuntu, install the `.deb` with `sudo apt install ./<downloaded-file>.deb` so its desktop dependencies are installed. On Windows, use the installer; on macOS, copy DevOrbit from the disk image into Applications.

## Start the database and backend

Extract the backend archive into a permanent directory. It contains the backend executable, this README, and a `server/` directory with Compose and an example environment file.

1. Copy `server/.env.example` to `server/.env` if you do not already have one. Replace `change-me` in both password entries with the same private password. URL-encode the password in `DEVORBIT_DATABASE_URL` if it contains URL-special characters.
2. From the extracted directory, start PostgreSQL:

   ```sh
   docker compose -f server/compose.yaml up -d --wait
   ```

3. Start the backend from that same directory and leave the terminal open:

   ```sh
   # Linux/macOS
   ./devorbit
   ```

   ```powershell
   # Windows PowerShell
   .\devorbit.exe
   ```

4. Open the installed desktop app as the same OS user. The backend creates the private token the desktop reads. Both default to `127.0.0.1:7456` and the user's DevOrbit config directory.

If using an existing database, skip the Compose command and set `DEVORBIT_DATABASE_URL` in `server/.env` instead. If you change host, port, data directory, or token path, launch both programs with the same exported settings; the desktop does not read `server/.env`.

PostgreSQL remains available after you close DevOrbit. Ctrl+C in the backend terminal stops its managed processes. To stop PostgreSQL while retaining data:

```sh
docker compose -f server/compose.yaml stop
```

Keep the existing environment file and database volume when upgrading. The Compose project directory remains `server/`, so the same named database volume is reused. Do not use `down -v` unless you intend to delete the database. Changing the password in `.env` does not change an already initialized PostgreSQL role's password.

The backend applies database migrations at startup. Back up the database and the private DevOrbit data directory before upgrading. The data directory contains workspace YAML, logs, and captured traffic and can contain secrets.

## Verify downloads

Tagged releases include `SHA256SUMS`. After downloading all release assets into one directory, run `sha256sum -c SHA256SUMS` on Linux or `shasum -a 256 -c SHA256SUMS` on macOS. On Windows, compare `Get-FileHash <downloaded-file> -Algorithm SHA256` with the corresponding entry. Missing assets will be reported if you downloaded only one platform.
