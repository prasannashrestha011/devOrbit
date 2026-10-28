---
layout: ../../layouts/Docs.astro
label: Contributing
title: Contributing
description: Find your way around the repository, run the relevant checks, and propose a focused change.
source: docs/ci-cd.md
---

## Set up the project

Start with the [source setup guide](/docs/#before-you-start), then run DevOrbit locally. Check existing [issues](https://github.com/prasannashrestha011/devOrbit/issues) and [pull requests](https://github.com/prasannashrestha011/devOrbit/pulls) before starting a change.

## Find the right area

| Directory | Responsibility |
| --- | --- |
| `desktop/src` | React desktop interface |
| `desktop/src-tauri` | Rust host, native windows, backend lifecycle, terminal bridge |
| `server` | Go API, process supervision, workspace configuration, local storage |
| `website` | Astro homepage and documentation |
| `docs` | Installation, release, and architecture reference documents |

Read the [architecture overview](/architecture/) for the boundaries between the desktop and backend.

## Validate your change

Run checks from the directory you changed. Backend tests need the development prerequisites, including a C compiler and tmux on Linux or macOS.

```sh
# From server/
go test -race ./...
go vet ./...
go build ./...
```

```sh
# From desktop/
npm ci
npm test
npm run build
```

```sh
# From website/
npm ci
npm test
```

Desktop host changes also need the Rust checks described in the [CI workflow](https://github.com/prasannashrestha011/devOrbit/blob/main/.github/workflows/ci.yml). For website changes, inspect the rendered pages at desktop and mobile widths and exercise keyboard navigation.

## Open a pull request

Explain the problem, the resulting behavior, and how you checked it. Include screenshots for visible interface changes and update affected documentation. Keep unrelated changes separate so the review can focus on one problem.

See [CI/CD and releases](https://github.com/prasannashrestha011/devOrbit/blob/main/docs/ci-cd.md) for packaging and release checks.
