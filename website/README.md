# DevOrbit website

A static Astro homepage, architecture overview, and Markdown documentation site. It runs independently of the desktop and Go backend, with no client framework or added UI dependencies.

## Run locally

Use Node.js 22.22+ and npm. From this directory:

```sh
npm ci
npm run dev
```

Open the local URL printed by Astro (normally http://localhost:4321). Build and check page metadata, routes, anchors, assets, search results, and asset budgets:

```sh
npm test
```

`npm run preview` serves the production build. Deploy `dist/` to a static host at the domain root and configure it to serve `404.html` for missing pages. No runtime server, database, secrets, or environment variables are required.

## Edit content

- `src/pages/index.astro`: homepage with the desktop preview, YAML example, workflow links, and source quick start.
- `src/pages/docs/*.md`: guides. Their h2/h3 headings populate the page outline and static search index.
- `src/pages/architecture.astro`: architecture guide and semantic HTML/CSS diagram.
- `src/data/navigation.ts`: guide groups and previous/next order.
- `src/layouts/{Site,Docs}.astro`: shared header, search dialog, and responsive documentation shell.
- `src/components`: reusable icons, code examples, and guide navigation.
- `src/styles/global.css`: light/dark tokens, typography, content styles, and responsive layouts. See `DESIGN.md` for the design contract.
- `src/pages/search-index.json.ts`: build-time guide and section index. Uses Astro's compiled Markdown so heading links match rendered anchors.
- `src/scripts/{site.ts,search.mjs}`: progressive enhancements and pure search ranking.

Search loads its static index when first opened. Ctrl/Cmd K opens it, arrow keys navigate results, Enter follows a result, and Escape closes it and restores focus. A failed fetch has a retry action. Queries stay in the browser. Search result text is inserted as text, never as HTML.

Code copying provides confirmation and selects code for manual copying if clipboard access fails. The theme follows the system until the user chooses a preference; that choice is stored locally. Guide navigation and mobile menus use regular links and native details elements. With JavaScript disabled, content, links, code, and menus remain available; search, copy, and the theme button are hidden.

The build is capped at 1 MiB total, with less than 16 KiB of client JavaScript. The budget includes all pages, the font, the screenshot, and the lazy-loaded search index. The homepage fetches the screenshot; documentation pages do not. Astro remains pinned to 7.4.1.

## Assets and source of truth

IBM Plex Sans is self-hosted in `public/fonts`, with its SIL Open Font License in `OFL.txt`. Source: [Google Fonts repository](https://github.com/google/fonts/tree/main/ofl/ibmplexsans).

`public/images/workspace.webp` is a capture of this repository's React desktop interface with synthetic workspace/service data supplied at the Tauri API boundary. It shows the real UI; the caption identifies the example data. It contains no personal workspace information. Capture viewport: 1440 × 900; compressed to WebP. Generated design concepts were used only as references and are not shipped assets.

Guides are curated from the root README, `server/README.md`, `docs/`, and relevant source files. Preserve the existing heading anchors when editing established guides. Update the website alongside product behavior changes, run `npm test`, and inspect desktop/mobile layouts and keyboard interactions before publishing.
