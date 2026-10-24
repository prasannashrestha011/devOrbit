# CI/CD and releases

DevOrbit is a local desktop application. The Go backend embeds SQLite and runs alongside the Tauri desktop on the host to manage local processes and project directories. Docker with Compose v2 is only required when managing Docker Compose projects.

## Local setup

Install the development prerequisites in the root README, including a C compiler for the SQLite driver, then run:

```sh
./dev.sh
```

The backend creates `<DEVORBIT_DATA_DIR>/devorbit.db` automatically. Optional settings are in `server/.env.example`; no database server or credentials are required. Stop the backend before copying the private data directory for backup.

## GitHub Actions

`.github/workflows/ci.yml` runs on pull requests targeting `main`, pushes to `main`, `v*` tags, and manual dispatches.

- **Backend checks:** Go race tests (including SQLite integration tests against temporary files), `go vet`, and a backend build. `tmux` is installed so terminal tests run.
- **Desktop checks:** npm's locked install, React tests, TypeScript/Vite build, bundled backend build, Rust tests with the Cargo lockfile, and version consistency across app manifests and lockfiles.
- **Release packaging:** after checks pass, manual dispatches and version tags build Linux x64 `.deb`, Windows x64 NSIS, and macOS Intel/Apple Silicon `.dmg` installers containing the Go backend. Each target runs the SQLite database tests. Backend builds enable CGO and use the native runner's C compiler; the SQLite library is compiled into the executable. Normal branch and pull request runs skip packaging.
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

3. Wait for the tag workflow. Download the installers from the draft, follow [the installation instructions](install-release.md), and test the app on each target platform.
4. Review the release notes and publish the draft when ready.

The workflow generates platform icons from the existing PNG and uses Tauri's native packaging. The desktop starts its bundled backend and stops it on exit. The workflow does not publish automatically, deploy a server, or install an updater. Windows code signing and macOS Developer ID signing/notarization still require certificates and a separate setup; these builds are for initial distribution and testing.

References: [Tauri's GitHub Actions guide](https://v2.tauri.app/distribute/pipelines/github/), [macOS signing](https://v2.tauri.app/distribute/sign/macos/), and [GitHub release creation](https://cli.github.com/manual/gh_release_create).
