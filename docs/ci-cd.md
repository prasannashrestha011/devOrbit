# Docker and CI/CD

DevOrbit is a local desktop application. Docker runs PostgreSQL; the Go backend and Tauri desktop run on the host so they can access local processes, project directories, runtimes, and Docker Compose. A backend container would run managed commands inside that container and would not replace the native process supervisor.

## Local Docker setup

From the repository root, copy `server/.env.example` to `server/.env` if it does not exist, then set a private database password in both entries. Start the existing PostgreSQL service:

```sh
docker compose -f server/compose.yaml up -d --wait
./dev.sh
```

The database listens only on `127.0.0.1:5432`, uses the existing `postgres_data` volume, waits for PostgreSQL readiness, and restarts unless explicitly stopped. A 30-second grace period lets PostgreSQL shut down cleanly. Install the development prerequisites in the root README before using `dev.sh`.

Useful commands, also from the repository root:

```sh
docker compose -f server/compose.yaml ps
docker compose -f server/compose.yaml logs -f devorbit-db
docker compose -f server/compose.yaml stop
```

The password is supplied through the ignored `server/.env`, not committed configuration. Changing it does not rotate an existing database's password. Keep the same Compose project name when reusing existing data; `down -v` deletes the database volume.

## GitHub Actions

`.github/workflows/ci.yml` runs on pull requests targeting `main`, pushes to `main`, `v*` tags, and manual dispatches.

- **Backend checks:** Go race tests, PostgreSQL integration tests against an isolated CI database, `go vet`, a backend build, and Compose configuration validation. `tmux` is installed so terminal tests run.
- **Desktop checks:** npm's locked install, React tests, TypeScript/Vite build, Rust tests with the Cargo lockfile, and version consistency across app manifests and lockfiles.
- **Release packaging:** after checks pass, manual dispatches and version tags build Linux x64 `.deb`, Windows x64 NSIS, and macOS Intel/Apple Silicon `.dmg` installers. Each target also produces a backend archive containing Compose, the example environment file, and startup instructions. Normal branch and pull request runs skip packaging.
- **Delivery:** artifacts are available on the workflow run for 14 days. Tags also create a draft GitHub Release with all four platforms and SHA-256 checksums, after every build succeeds. Reruns can update a draft, but fail rather than overwrite a published release.

Only the release job has repository write permission. The generated `GITHUB_TOKEN` is sufficient; no personal token, registry credentials, or deployment server is needed. GitHub Actions must be enabled and repository/organization policy must permit the referenced actions and release write permission.

For branch protection, require **Backend checks** and **Desktop checks** after their first run. Manual dispatch becomes available once this workflow exists on the default branch. Choose **Actions → CI and release → Run workflow** to try downloadable builds without creating a release tag.

## Create a release

1. Keep the version synchronized in `desktop/package.json`, `desktop/package-lock.json` (top level and root package), `desktop/src-tauri/tauri.conf.json`, `desktop/src-tauri/Cargo.toml`, and the `devorbit-desktop` entry in `desktop/src-tauri/Cargo.lock`.
2. Commit the release changes and push `main`. After its checks pass, tag that commit with the matching version. For the current `0.1.0`:

   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. Wait for the tag workflow. Download the installers and backend archives from the draft, follow [the installation instructions](install-release.md), and test the app on each target platform.
4. Review the release notes and publish the draft when ready.

The workflow generates platform icons from the existing PNG and uses Tauri's native packaging. It does not publish automatically, deploy a server, install an updater, or bundle/start the backend inside the desktop. Windows code signing and macOS Developer ID signing/notarization still require certificates and a separate setup; these builds are for initial distribution and testing.

References: [Tauri's GitHub Actions guide](https://v2.tauri.app/distribute/pipelines/github/), [macOS signing](https://v2.tauri.app/distribute/sign/macos/), [Docker Compose service settings](https://docs.docker.com/reference/compose-file/services/), and [GitHub release creation](https://cli.github.com/manual/gh_release_create).
