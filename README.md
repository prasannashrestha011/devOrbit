# DevOrbit

DevOrbit runs local project services through a Go backend and a React/Tauri desktop window. The backend manages processes, persists logs, and owns the API and bearer token. The development script starts both processes; the desktop app does not bundle the backend yet.

Creating a workspace creates `.devorbit/workspace.yaml` in its project root. Add services in the desktop UI, import a YAML file in the Workspace YAML editor, or edit that file directly. DevOrbit validates the file before saving or starting services. PostgreSQL stores workspace metadata and stable service IDs, while YAML is the only service configuration source.

## Run locally

You need Go 1.24, PostgreSQL 17, Node.js, npm, Rust/Cargo, and the [Tauri system prerequisites](https://v2.tauri.app/start/prerequisites/) for your OS. On Linux and macOS, install `tmux` for interactive service terminals. Set up the database as described in the [backend README](server/README.md).

On Ubuntu, install the Tauri build dependencies and Rust:

```sh
sudo apt update
sudo apt install libwebkit2gtk-4.1-dev build-essential curl wget file libxdo-dev libssl-dev libayatana-appindicator3-dev librsvg2-dev tmux
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
. "$HOME/.cargo/env"
```

Rustup installs `cargo` under `~/.cargo/bin`; open a new terminal or source the environment file above before running `./dev.sh`.

Install the desktop dependencies once:

```sh
cd desktop && npm ci && cd ..
```

Then start both processes from the repository root:

```sh
./dev.sh
```

The script builds and starts the Go backend, waits for `/health`, then opens the Tauri desktop. The backend reads `server/.env`; exported variables take precedence. Closing the desktop or pressing Ctrl+C stops the backend. If you change `DEVORBIT_HOST`, `DEVORBIT_PORT`, `DEVORBIT_DATA_DIR`, or `DEVORBIT_AUTH_TOKEN_PATH`, export them before running the script so the desktop receives the same settings. The host must be a loopback IP. The desktop reads the private token file on the Tauri side and sends authenticated requests to the configured local backend. The UI does not store the token.

Use **Quick controls** in the desktop toolbar to open an always-on-top window for the selected workspace. It follows workspace selection and shows only local and Docker Compose service names with start, stop, and restart actions. Click its grip to collapse or expand it; drag the surrounding bar to move it. Closing it leaves services running.

`npm run build` checks and builds the web UI; `npm test` runs its focused API and lifecycle tests. `npm run tauri build` builds a native package once the OS prerequisites are installed. Backend sidecar packaging is later work.

## Docker and releases

PostgreSQL runs through `server/compose.yaml`; the backend and desktop stay native to manage your local projects. See [Docker and CI/CD](docs/ci-cd.md) for setup, GitHub Actions checks, and creating downloadable release builds for Linux, Windows, and macOS.

Tagged builds create draft releases with desktop installers and separate backend archives. Follow [the release installation guide](docs/install-release.md) to run them. The desktop still requires the backend to be started separately.
