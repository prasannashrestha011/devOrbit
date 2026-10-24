# DevOrbit

DevOrbit runs local project services through a Go backend and a React/Tauri desktop window. The backend manages processes, persists logs, and owns the API and bearer token. The desktop app bundles and starts the backend when it opens.

Creating a workspace creates `.devorbit/workspace.yaml` in its project root. Add services in the desktop UI, import a YAML file in the Workspace YAML editor, or edit that file directly. DevOrbit validates the file before saving or starting services. SQLite stores workspace metadata and stable service IDs, while YAML is the only service configuration source.

## Install

Download the installer for your platform from a DevOrbit release and open the app. It starts its bundled backend and creates the SQLite database automatically. See the [release installation guide](docs/install-release.md).

## Build from source

You need Go 1.24, a C compiler for SQLite, Node.js, npm, Rust/Cargo, and the [Tauri system prerequisites](https://v2.tauri.app/start/prerequisites/) for your OS. On Linux and macOS, install `tmux` for interactive service terminals. The backend creates its SQLite database automatically. See the [backend README](server/README.md) for compiler requirements and optional settings.

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

Then launch DevOrbit from the repository root:

```sh
./dev.sh
```

The script starts Tauri, which builds and launches the bundled Go backend. Closing the desktop stops the backend. Export optional `DEVORBIT_*` settings before launching; the desktop and its backend receive the same values. The host must be a loopback IP. The desktop reads the private token file on the Tauri side and sends authenticated requests to the local backend. The UI does not store the token.

Use **Quick controls** in the desktop toolbar to open an always-on-top window for the selected workspace. It follows workspace selection and shows only local and Docker Compose service names with start, stop, and restart actions. Click its grip to collapse or expand it; drag the surrounding bar to move it. Closing it leaves services running.

`npm run build` checks and builds the web UI; `npm test` runs its focused API and lifecycle tests. `npm run tauri build` builds a native package with the Go backend once the OS prerequisites are installed. Run `npm run build:backend` before a direct `cargo test` from `desktop/src-tauri`.

## Storage and releases

Workspace metadata is stored in `<DEVORBIT_DATA_DIR>/devorbit.db`. The backend and desktop run natively; Docker is only needed when managing Docker Compose projects. See [CI/CD and releases](docs/ci-cd.md) for GitHub Actions checks and creating downloadable builds for Linux, Windows, and macOS.

Tagged builds create draft releases with one desktop installer per platform. Follow [the release installation guide](docs/install-release.md) to run them.
