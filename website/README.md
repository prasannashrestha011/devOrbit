# DevOrbit website

A static introduction, architecture overview, and Markdown documentation site. It runs independently of the desktop and Go backend.

## Run locally

Use Node.js 22.22+ and npm. From this directory:

```sh
npm ci
npm run dev
```

Open http://localhost:4321. To build and check all local links, heading anchors, page metadata, and the zero-JavaScript budget:

```sh
npm test
```

`npm run preview` serves the production build locally after building. Deploy the `dist/` directory to any static host at the domain root. No runtime server, database, secrets, or environment variables are required. Configure the host to serve `404.html` for missing pages.

## Why Astro

[Astro](https://docs.astro.build/en/concepts/why-astro/) fits a content-focused site: Markdown pages, shared layouts, and static HTML with no client framework runtime. This site ships no JavaScript, external fonts, or image assets beyond a small SVG favicon. Astro is pinned to 7.4.1, which supports the repository environment's Node.js version; check Node requirements when upgrading.

React remains appropriate for DevOrbit's existing interactive desktop. Adding another React app, a backend, or a CMS is unnecessary for this website.

## Edit content

- `src/pages/index.astro`: product introduction.
- `src/pages/architecture.astro`: architecture and native HTML/CSS diagram.
- `src/pages/docs/*.md`: documentation. Each file uses `src/layouts/Docs.astro`; its second-level headings automatically populate the table of contents.
- `src/layouts/Docs.astro`: guide order and sidebar links. Add new guide routes here.
- `src/styles/global.css`: shared design and responsive layouts.

Guides are curated from `../README.md`, `../server/README.md`, and `../docs/`. Update the relevant website guide when product behavior changes; source links lead back to those technical documents. Run `npm test` before publishing.
