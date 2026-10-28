# README.md

diff --git a/README.md b/README.md
index 61766c9..f52fa1e 100644
--- a/README.md
+++ b/README.md
@@ -41,6 +41,8 @@ Use **Quick controls** in the desktop toolbar to open an always-on-top window fo
 
 `npm run build` checks and builds the web UI; `npm test` runs its focused API and lifecycle tests. `npm run tauri build` builds a native package with the Go backend once the OS prerequisites are installed. Run `npm run build:backend` before a direct `cargo test` from `desktop/src-tauri`.
 
+To check a packaged build locally, run `./build.sh linux` for a `.deb`, `./build.sh windows` in Windows Git Bash for an `.exe`, or `./build.sh macos` for a `.dmg`. Run it on the named OS. The installer appears under `desktop/src-tauri/target/release/bundle/`.
+
 ## Storage and releases
 
 Workspace metadata is stored in `<DEVORBIT_DATA_DIR>/devorbit.db`. The backend and desktop run natively; Docker is only needed when managing Docker Compose projects. See [CI/CD and releases](docs/ci-cd.md) for GitHub Actions checks and creating downloadable builds for Linux, Windows, and macOS.

# desktop/package-lock.json

diff --git a/desktop/package-lock.json b/desktop/package-lock.json
index ff19dd6..ac6fc1a 100644
--- a/desktop/package-lock.json
+++ b/desktop/package-lock.json
@@ -10,6 +10,7 @@
       "dependencies": {
         "@tauri-apps/api": "^2.0.0",
         "@tauri-apps/plugin-dialog": "^2.8.3",
+        "@xterm/addon-canvas": "0.7.0",
         "@xterm/addon-fit": "^0.10.0",
         "@xterm/addon-search": "^0.15.0",
         "@xterm/xterm": "^5.5.0",
@@ -1842,6 +1843,15 @@
         "url": "https://opencollective.com/vitest"
       }
     },
+    "node_modules/@xterm/addon-canvas": {
+      "version": "0.7.0",
+      "resolved": "https://registry.npmjs.org/@xterm/addon-canvas/-/addon-canvas-0.7.0.tgz",
+      "integrity": "sha512-LF5LYcfvefJuJ7QotNRdRSPc9YASAVDeoT5uyXS/nZshZXjYplGXRECBGiznwvhNL2I8bq1Lf5MzRwstsYQ2Iw==",
+      "license": "MIT",
+      "peerDependencies": {
+        "@xterm/xterm": "^5.0.0"
+      }
+    },
     "node_modules/@xterm/addon-fit": {
       "version": "0.10.0",
       "resolved": "https://registry.npmjs.org/@xterm/addon-fit/-/addon-fit-0.10.0.tgz",

# desktop/package.json

diff --git a/desktop/package.json b/desktop/package.json
index 65b42e5..f44cbfa 100644
--- a/desktop/package.json
+++ b/desktop/package.json
@@ -14,6 +14,7 @@
   "dependencies": {
     "@tauri-apps/api": "^2.0.0",
     "@tauri-apps/plugin-dialog": "^2.8.3",
+    "@xterm/addon-canvas": "0.7.0",
     "@xterm/addon-fit": "^0.10.0",
     "@xterm/addon-search": "^0.15.0",
     "@xterm/xterm": "^5.5.0",

# desktop/src/App.test.tsx

diff --git a/desktop/src/App.test.tsx b/desktop/src/App.test.tsx
index b629969..dfe751f 100644
--- a/desktop/src/App.test.tsx
+++ b/desktop/src/App.test.tsx
@@ -218,6 +218,21 @@ it("lists detected Docker services in the workspace overview when the Engine is
   expect(section.textContent).toContain("postgres:16");
 });
 
+it("shows actionable Compose errors and removes them after a successful refresh", async () => {
+  const snapshot = {
+    availability: { state: "ready", message: "Docker Compose and Docker Engine are ready." },
+    candidates: [{ file: "compose.yaml", files: ["compose.yaml"] }],
+    error: "docker compose config --format json: required variable MINIO_ROOT_USER is missing a value: set MINIO_ROOT_USER in compose.env: exit status 1",
+  };
+  dockerSnapshot = snapshot;
+  render(<App />);
+  expect((await screen.findByRole("alert")).textContent).toContain("Compose configuration is incomplete");
+  expect(screen.getByRole("listitem").textContent).toBe("MINIO_ROOT_USER");
+  dockerSnapshot = { ...snapshot, error: "" };
+  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
+  await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
+});
+
 it.each(["ready", "engine_unavailable"])("opens a workspace containing a Docker service with null ports (%s)", async (state) => {
   const other = { ...workspace, id: tid, name: "Docker Worker" };
   const handleRequest = invoke.getMockImplementation()!;

# desktop/src/DockerPanel.tsx

diff --git a/desktop/src/DockerPanel.tsx b/desktop/src/DockerPanel.tsx
index 2cab128..d6fee4c 100644
--- a/desktop/src/DockerPanel.tsx
+++ b/desktop/src/DockerPanel.tsx
@@ -1,6 +1,7 @@
 import { useCallback, useEffect, useState } from "react";
 import { Box, ChevronDown, FileCode2, FileText, Hammer, MoreHorizontal, Play, RefreshCw, RotateCw, Square, Trash2 } from "lucide-react";
 import { api, type DockerAction, type DockerOperation, type DockerSnapshot } from "./api";
+import DockerError from "./DockerError";
 import DockerServiceDetails, { type DockerServiceTab } from "./DockerServiceDetails";
 
 function message(error: unknown) { return error instanceof Error ? error.message : String(error); }
@@ -132,7 +133,7 @@ export default function DockerPanel({ workspaceId }: { workspaceId: string }) {
           </div></details>
         </div></div>}
       </div>
-      {snapshot.error && <p className="docker-error" role="alert">{snapshot.error}</p>}
+      {snapshot.error && <DockerError error={snapshot.error} title="Could not load Compose services" />}
       {project && <div className="docker-services">
         <div className="docker-services-heading"><h3><button type="button" className="docker-services-toggle" aria-expanded={servicesExpanded} onClick={() => setServicesExpanded((expanded) => !expanded)}>Compose services <ChevronDown size={16} aria-hidden="true" /></button></h3><span>{project.services.length} defined in {project.name}</span></div>
         {servicesExpanded && project.services.map((service) => {
@@ -172,6 +173,6 @@ export default function DockerPanel({ workspaceId }: { workspaceId: string }) {
     </>}
     {busy && <p role="status" className="docker-muted">{pending ? "Starting Docker operation…" : `${operation?.action} in progress…`} {operation?.state === "running" && <button className="text-button" onClick={() => api.cancelDockerOperation(workspaceId, operation.id).catch((problem) => setError(message(problem)))}>Cancel</button>}</p>}
     {notice && <p role="status" className="docker-availability ready">{notice}</p>}
-    {error && <p role="alert" className="docker-error">{error}</p>}
+    {error && <DockerError error={error} />}
   </section>;
 }

# desktop/src/DockerServiceDetails.tsx

diff --git a/desktop/src/DockerServiceDetails.tsx b/desktop/src/DockerServiceDetails.tsx
index 0f89c14..5cee95f 100644
--- a/desktop/src/DockerServiceDetails.tsx
+++ b/desktop/src/DockerServiceDetails.tsx
@@ -1,6 +1,7 @@
 import { useEffect, useRef, useState } from "react";
 import { Channel, invoke } from "@tauri-apps/api/core";
 import { api, type DockerSnapshot, type DockerTerminal } from "./api";
+import DockerError from "./DockerError";
 import TerminalPane from "./TerminalPane";
 import TrafficInspector from "./TrafficInspector";
 
@@ -107,7 +108,7 @@ export default function DockerServiceDetails({ workspaceId, file, service, tab,
       {terminal ? <TerminalPane key={terminal.id} docker={terminal} workspaceId={workspaceId} focused expanded busy={busy} onFocus={() => {}} onClose={() => void closeTerminal()} onStopped={() => void closeTerminal()} /> : <div className="docker-detail-terminal-start">
         {running.length ? <><label>Container <select aria-label="Terminal container" value={selected} onChange={(event) => setSelectedContainer(event.target.value)}>{running.map((container) => <option key={container.id} value={container.id}>{container.name}</option>)}</select></label><button className="button primary" disabled={busy} onClick={() => { setBusy(true); setError(""); api.createDockerTerminal(workspaceId, file, selected).then(setTerminal).catch((problem) => setError(String(problem))).finally(() => setBusy(false)); }}>Open terminal</button></> : <p className="docker-muted">Start a container to open its terminal.</p>}
       </div>}
-      {error && <p className="docker-error" role="alert">{error}</p>}
+      {error && <DockerError error={error} />}
     </div>}
     {tab === "inspect" && <pre className="docker-detail-inspect" role="tabpanel">{JSON.stringify(service, null, 2)}</pre>}
     {tab === "traffic" && <TrafficInspector workspaceId={workspaceId} kind="docker" serviceKey={service.name} file={file} ports={[...new Map(service.containers.filter((container) => container.state === "running").flatMap((container) => container.ports).filter((port) => port.published && (!port.protocol || port.protocol === "tcp") && (!port.host || port.host === "0.0.0.0" || port.host.startsWith("127."))).map((port) => [port.published, { key: port.published, label: `${port.published} → ${port.target}/tcp` }])).values()]} />}

# desktop/src/TerminalPane.test.tsx

diff --git a/desktop/src/TerminalPane.test.tsx b/desktop/src/TerminalPane.test.tsx
index badef39..14068c1 100644
--- a/desktop/src/TerminalPane.test.tsx
+++ b/desktop/src/TerminalPane.test.tsx
@@ -3,7 +3,7 @@ import { afterEach, expect, it, vi } from "vitest";
 import { cleanup, render, screen, waitFor } from "@testing-library/react";
 import type { ProcessStatus, Service, ShellSession } from "./api";
 
-const calls = vi.hoisted(() => ({ invoke: vi.fn(), focus: vi.fn(), fit: vi.fn(), options: null as { fontFamily: string } | null, onData: (_data: string) => {}, writes: [] as string[] }));
+const calls = vi.hoisted(() => ({ invoke: vi.fn(), focus: vi.fn(), fit: vi.fn(), options: null as { fontFamily: string } | null, onData: (_data: string) => {}, writes: [] as string[], lifecycle: [] as string[] }));
 vi.mock("@tauri-apps/api/core", () => ({
   invoke: calls.invoke,
   Channel: class { onmessage = (_frame: unknown) => {}; },
@@ -13,14 +13,15 @@ vi.mock("@xterm/xterm", () => ({
     cols = 80; rows = 24;
     options: { fontFamily: string };
     constructor(options: { fontFamily: string }) { this.options = options; calls.options = options; }
-    loadAddon() {}
-    open() {}
+    loadAddon(addon: { renderer?: string }) { if (addon.renderer) calls.lifecycle.push(addon.renderer); }
+    open() { calls.lifecycle.push("open"); }
     write(data: string | Uint8Array) { calls.writes.push(typeof data === "string" ? data : new TextDecoder().decode(data)); }
     onData(callback: (data: string) => void) { calls.onData = callback; }
     focus() { calls.focus(); }
     dispose() {}
   },
 }));
+vi.mock("@xterm/addon-canvas", () => ({ CanvasAddon: class { renderer = "canvas"; } }));
 vi.mock("@xterm/addon-fit", () => ({ FitAddon: class { fit() { calls.fit(); } } }));
 vi.mock("@xterm/addon-search", () => ({ SearchAddon: class { findNext() {} } }));
 
@@ -29,7 +30,14 @@ import TerminalPane from "./TerminalPane";
 const service = { id: "51d71407-759a-44c0-b46e-87a842998ddd", name: "API", enabled: true } as Service;
 const status = { execution_id: "30c80606-7a89-4b84-94dd-084e33e88baa", state: "running" } as ProcessStatus;
 
-afterEach(() => { cleanup(); calls.invoke.mockReset(); calls.focus.mockReset(); calls.fit.mockReset(); calls.options = null; calls.writes.length = 0; });
+afterEach(() => { cleanup(); calls.invoke.mockReset(); calls.focus.mockReset(); calls.fit.mockReset(); calls.options = null; calls.writes.length = 0; calls.lifecycle.length = 0; });
+
+it("loads the canvas renderer before opening to support strict release CSP", async () => {
+  vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
+  calls.invoke.mockResolvedValue(undefined);
+  render(<TerminalPane service={service} status={status} focused expanded busy={false} onFocus={() => {}} />);
+  await waitFor(() => expect(calls.lifecycle).toEqual(["canvas", "open"]));
+});
 
 it("changes the font and refits the active terminal without reconnecting", async () => {
   vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
@@ -51,12 +59,12 @@ it("streams output and accepts input only from the focused pane, including a lar
   calls.invoke.mockImplementation(async (command: string, args: { channel?: { onmessage: (frame: unknown) => void } }) => {
     if (command === "start_terminal") {
       args.channel?.onmessage({ type: "connected" });
-      args.channel?.onmessage({ type: "output", data: btoa("ready") });
+      args.channel?.onmessage({ type: "output", data: btoa("\x1b[31mready\x1b[0m") });
     }
   });
   const props = { service, status, focused: false, expanded: false, busy: false, onFocus: vi.fn(), onExpand: vi.fn(), onAction: vi.fn() };
   const view = render(<TerminalPane {...props} />);
-  await waitFor(() => expect(calls.writes.join("")).toContain("ready"));
+  await waitFor(() => expect(calls.writes.join("")).toContain("\x1b[31mready\x1b[0m"));
   await waitFor(() => expect(calls.invoke).toHaveBeenCalledWith("terminal_resize", { serviceId: service.id, executionId: status.execution_id, cols: 80, rows: 24 }));
   calls.onData("ignored");
   expect(calls.invoke).not.toHaveBeenCalledWith("terminal_input", expect.anything());

# desktop/src/TerminalPane.tsx

diff --git a/desktop/src/TerminalPane.tsx b/desktop/src/TerminalPane.tsx
index e2da51a..6421238 100644
--- a/desktop/src/TerminalPane.tsx
+++ b/desktop/src/TerminalPane.tsx
@@ -63,8 +63,8 @@ export default function TerminalPane({ service, status, shell, docker, workspace
         });
     };
     async function mount() {
-      const [{ Terminal }, { FitAddon }, { SearchAddon }] = await Promise.all([
-        import("@xterm/xterm"), import("@xterm/addon-fit"), import("@xterm/addon-search"),
+      const [{ Terminal }, { FitAddon }, { SearchAddon }, { CanvasAddon }] = await Promise.all([
+        import("@xterm/xterm"), import("@xterm/addon-fit"), import("@xterm/addon-search"), import("@xterm/addon-canvas"),
       ]);
       if (disposed || !host.current) return;
       instance = new Terminal({ scrollback: 1000, convertEol: false, screenReaderMode: true, cursorBlink: true, fontSize: 14, fontFamily: fontFamilyRef.current ? `${JSON.stringify(fontFamilyRef.current)}, monospace` : "monospace", theme: { background: "#080b10", foreground: "#dbe5f2", cursor: "#58d6c0" } });
@@ -72,6 +72,8 @@ export default function TerminalPane({ service, status, shell, docker, workspace
       const finder = new SearchAddon();
       instance.loadAddon(fit);
       instance.loadAddon(finder);
+      // Load before open so strict release CSP never sees the DOM renderer's inline CSS.
+      instance.loadAddon(new CanvasAddon());
       instance.open(host.current);
       if (focusedRef.current) instance.focus();
       terminal.current = instance;

# desktop/src/styles.css

diff --git a/desktop/src/styles.css b/desktop/src/styles.css
index a909997..59bbe84 100644
--- a/desktop/src/styles.css
+++ b/desktop/src/styles.css
@@ -2803,9 +2803,52 @@ h2 {
 }
 
 .docker-error {
+  min-width: 0;
+  padding: 16px;
+  border: 1px solid var(--danger);
+  border-radius: 8px;
+  background: var(--surface-inset);
+  color: var(--text-main);
+  font-size: 14px;
+  line-height: 1.6;
+}
+
+.docker-error > strong {
   color: var(--danger);
 }
 
+.docker-error p {
+  max-width: 75ch;
+  margin: 8px 0;
+}
+
+.docker-error ul {
+  margin: 8px 0;
+  padding-left: 24px;
+}
+
+.docker-error summary {
+  width: fit-content;
+  margin-top: 12px;
+  color: var(--text-secondary);
+  cursor: pointer;
+}
+
+.docker-error summary:focus-visible {
+  outline: 2px solid var(--focus-ring);
+  outline-offset: 4px;
+}
+
+.docker-error pre {
+  max-height: 280px;
+  overflow: auto;
+  margin: 12px 0 0;
+  color: var(--text-secondary);
+  font: 13px/1.6 "JetBrains Mono", monospace;
+  white-space: pre-wrap;
+  overflow-wrap: anywhere;
+}
+
 @container (max-width: 680px) {
   .docker-project-card {
     align-items: flex-start;

# server/README.md

diff --git a/server/README.md b/server/README.md
index fe7ce21..3ca223b 100644
--- a/server/README.md
+++ b/server/README.md
@@ -49,7 +49,7 @@ Back up the private data directory with the backend stopped, including `devorbit
 - `PATCH /api/v1/workspaces/{id}` updates supplied `name` or `description` fields; an empty description clears it.
 - `DELETE /api/v1/workspaces/{id}` deletes workspace metadata, service identities, and `.devorbit/workspace.yaml`; returns `409` while services or shell terminals are running. Empty managed workspace directories are removed.
 - `GET /api/v1/workspaces/{id}/terminals` lists that workspace's shell sessions, including exited sessions that have not been closed.
-- `POST /api/v1/workspaces/{id}/terminals` starts the default shell in the workspace root and returns its session. At most 16 sessions may exist per workspace.
+- `POST /api/v1/workspaces/{id}/terminals` starts the default shell in the user's home directory and returns its session. At most 16 sessions may exist per workspace.
 - `DELETE /api/v1/workspaces/{id}/terminals/{terminal_id}` stops and removes a shell session.
 - `GET /api/v1/workspaces/{id}/terminals/{terminal_id}/stream` upgrades to an authenticated WebSocket. It uses the same input, resize, close, sequence, replay, and gap frames as service terminals. Closing the WebSocket only detaches the viewer.
 - `GET /api/v1/workspaces/{id}/configuration` returns the YAML content and its revision.
@@ -100,7 +100,7 @@ The process supervisor runs executables directly with their argument arrays, res
 
 Each successful process start has a new `execution_id` in its runtime status and owns a PTY. On Linux and macOS, the backend creates a private tmux session named after the service and attaches that PTY to it. The desktop shows one inline terminal viewport for the selected workspace with a compact service-session switcher; switching reattaches to an existing stream. The reported PID is the original tmux pane's shell process. Developers can split tmux panes with its normal prefix keys or the viewport controls and click a pane to focus it; Stop still interrupts the original service pane. When the service command exits, status and logs close for that execution while the tmux pane becomes an interactive shell. Service Start or Restart replaces the previous tmux session. `tmux` must be installed; a missing executable causes service start to fail clearly. Windows continues to use a direct PTY and does not have the shell-after-exit behavior. New executions merge stdout and stderr into a readable `terminal` log stream under `DEVORBIT_LOG_DIR/services/<service-id>/<execution-id>/`; older `stdout` and `stderr` entries remain available. ANSI control sequences are removed from new terminal log entries, while the WebSocket sends raw terminal bytes. The PTY echoes normal typed input, so commands and other echoed text can also appear in service logs; programs can disable echo for sensitive prompts. Entries contain workspace, service, execution, stream, UTC timestamp, sequence, message, and an ID. Output without newlines is captured while the process runs; long output is divided into 16 KiB chunks without truncation. Read boundaries may split one line across entries. The per-service memory cache holds at most 500 entries and 2 MiB of message data. The bounded ingestion queue drops entries if persistence cannot keep up; the service log response includes `diagnostics.dropped_entries` and `diagnostics.storage_errors` for the current backend session.
 
-Shell terminals are independent of configured services. They use the user's default shell in the workspace root, stay running while the desktop view is closed, and retain up to 1 MiB of recent output in memory for reattachment. Exited sessions remain listed until deleted. Shell input and output are not written to service logs, and all shell sessions end when the backend shuts down. Creating a shell requires the same API token that permits service command execution; DevOrbit does not sandbox either kind of process.
+Shell terminals are independent of configured services. They use the user's default shell in the user's home directory, stay running while the desktop view is closed, and retain up to 1 MiB of recent output in memory for reattachment. Exited sessions remain listed until deleted. Shell input and output are not written to service logs, and all shell sessions end when the backend shuts down. Creating a shell requires the same API token that permits service command execution; DevOrbit does not sandbox either kind of process.
 
 Log files rotate at 10 MiB, keep five files per execution, and retain at most 20 executions per service for seven days. Expired executions are removed at startup and when that service starts again. Log queries read persisted files, including after a backend restart. They accept `limit` (1–500, default 100) and `before` (the previous page's `next_cursor`); entries are returned newest first. Logs may contain secrets printed by child processes, so log retrieval requires the normal API token. Log retrieval scans the retained files and may be slower for large histories.
 

# server/internal/app/app.go

diff --git a/server/internal/app/app.go b/server/internal/app/app.go
index 1ef0ffb..01b492d 100644
--- a/server/internal/app/app.go
+++ b/server/internal/app/app.go
@@ -9,6 +9,7 @@ import (
 	"net/http"
 	"os"
 	"path/filepath"
+	"runtime"
 	"strconv"
 	"time"
 
@@ -32,6 +33,11 @@ import (
 )
 
 func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
+	if runtime.GOOS != "windows" && os.Getenv("DEVORBIT_DESKTOP_CHILD") == "1" {
+		if err := loadShellPath(ctx); err != nil {
+			log.Warn("Could not load shell PATH; using the desktop environment")
+		}
+	}
 	if ip := net.ParseIP(cfg.Host); ip == nil || !ip.IsLoopback() || cfg.Port < 1 || cfg.Port > 65535 {
 		return errors.New("API listen address must be a loopback IP and valid port")
 	}

# server/internal/modules/shellterminal/controller/controller.go

diff --git a/server/internal/modules/shellterminal/controller/controller.go b/server/internal/modules/shellterminal/controller/controller.go
index 2177424..69361d3 100644
--- a/server/internal/modules/shellterminal/controller/controller.go
+++ b/server/internal/modules/shellterminal/controller/controller.go
@@ -101,7 +101,7 @@ func writeOperationError(w http.ResponseWriter, err error) {
 	case errors.Is(err, shellterminal.ErrLimit):
 		writeError(w, http.StatusConflict, "terminal_limit", "workspace terminal limit reached")
 	case errors.Is(err, process.ErrInvalid):
-		writeError(w, http.StatusUnprocessableEntity, "terminal_unavailable", "shell or workspace directory unavailable")
+		writeError(w, http.StatusUnprocessableEntity, "terminal_unavailable", "shell or home directory unavailable")
 	case errors.Is(err, process.ErrClosed):
 		writeError(w, http.StatusServiceUnavailable, "shutting_down", "terminal manager is shutting down")
 	default:

# server/internal/shellterminal/manager.go

diff --git a/server/internal/shellterminal/manager.go b/server/internal/shellterminal/manager.go
index ce15e05..ae25415 100644
--- a/server/internal/shellterminal/manager.go
+++ b/server/internal/shellterminal/manager.go
@@ -66,8 +66,7 @@ func (m *Manager) Create(ctx context.Context, workspaceID uuid.UUID) (Session, e
 	if m.closed {
 		return Session{}, process.ErrClosed
 	}
-	workspace, err := m.workspaces.Get(ctx, workspaceID)
-	if err != nil {
+	if _, err := m.workspaces.Get(ctx, workspaceID); err != nil {
 		return Session{}, err
 	}
 	count := 0
@@ -79,6 +78,10 @@ func (m *Manager) Create(ctx context.Context, workspaceID uuid.UUID) (Session, e
 	if count >= maxPerWorkspace {
 		return Session{}, ErrLimit
 	}
+	home, err := os.UserHomeDir()
+	if err != nil || !filepath.IsAbs(home) {
+		return Session{}, process.ErrInvalid
+	}
 	id, err := uuid.NewRandom()
 	if err != nil {
 		return Session{}, err
@@ -88,7 +91,7 @@ func (m *Manager) Create(ctx context.Context, workspaceID uuid.UUID) (Session, e
 	if runtime.GOOS != "windows" {
 		env = map[string]string{"TERM": "xterm-256color"}
 	}
-	status, err := m.supervisor.Start(id, process.Config{WorkspaceID: workspaceID, Command: command, Dir: workspace.RootPath, Env: env, Interactive: true})
+	status, err := m.supervisor.Start(id, process.Config{WorkspaceID: workspaceID, Command: command, Dir: home, Env: env, Interactive: true})
 	if err != nil {
 		_ = m.supervisor.Forget(id)
 		return Session{}, err

# server/internal/shellterminal/manager_test.go

diff --git a/server/internal/shellterminal/manager_test.go b/server/internal/shellterminal/manager_test.go
index d3b6a9c..d92b952 100644
--- a/server/internal/shellterminal/manager_test.go
+++ b/server/internal/shellterminal/manager_test.go
@@ -5,6 +5,7 @@ package shellterminal
 import (
 	"context"
 	"errors"
+	"path/filepath"
 	"strings"
 	"testing"
 	"time"
@@ -31,8 +32,10 @@ func (s *workspaceStore) Delete(_ context.Context, id uuid.UUID) error {
 	return nil
 }
 
-func TestShellSessionsAreIndependentAndRetainOutputUntilClosed(t *testing.T) {
+func TestShellSessionsStartInHomeAndRetainOutputUntilClosed(t *testing.T) {
 	t.Setenv("SHELL", "/bin/sh")
+	home := t.TempDir()
+	t.Setenv("HOME", home)
 	root := t.TempDir()
 	store := &workspaceStore{workspace: &model.Workspace{ID: uuid.New(), RootPath: root}}
 	m := New(store, 50*time.Millisecond)
@@ -81,8 +84,8 @@ func TestShellSessionsAreIndependentAndRetainOutputUntilClosed(t *testing.T) {
 	for _, frame := range frames {
 		output.Write(frame.Data)
 	}
-	if !strings.Contains(output.String(), root) {
-		t.Fatalf("shell did not run in workspace root: %q", output.String())
+	if !strings.Contains(output.String(), home) || strings.Contains(output.String(), root) {
+		t.Fatalf("shell did not run in user home directory: %q", output.String())
 	}
 	select {
 	case <-live:
@@ -162,3 +165,22 @@ func TestShutdownEndsShellSessions(t *testing.T) {
 		t.Fatalf("create after shutdown: %v", err)
 	}
 }
+
+func TestShellRejectsUnavailableHome(t *testing.T) {
+	for _, home := range []string{"", ".", filepath.Join(t.TempDir(), "missing")} {
+		t.Run(home, func(t *testing.T) {
+			t.Setenv("SHELL", "/bin/sh")
+			t.Setenv("HOME", home)
+			store := &workspaceStore{workspace: &model.Workspace{ID: uuid.New(), RootPath: t.TempDir()}}
+			m := New(store, 50*time.Millisecond)
+			t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
+			if _, err := m.Create(context.Background(), store.workspace.ID); !errors.Is(err, process.ErrInvalid) {
+				t.Fatalf("create with unavailable home: %v", err)
+			}
+			sessions, err := m.List(context.Background(), store.workspace.ID)
+			if err != nil || len(sessions) != 0 {
+				t.Fatalf("failed create left sessions: %+v, %v", sessions, err)
+			}
+		})
+	}
+}

# website/README.md

diff --git a/website/README.md b/website/README.md
index ea29bf7..338cbf7 100644
--- a/website/README.md
+++ b/website/README.md
@@ -1,6 +1,6 @@
 # DevOrbit website
 
-A static introduction, architecture overview, and Markdown documentation site. It runs independently of the desktop and Go backend.
+A static Astro homepage, architecture overview, and Markdown documentation site. It runs independently of the desktop and Go backend, with no client framework or added UI dependencies.
 
 ## Run locally
 
@@ -11,26 +11,36 @@ npm ci
 npm run dev
 ```
 
-Open http://localhost:4321. To build and check all local links, heading anchors, page metadata, and the zero-JavaScript budget:
+Open the local URL printed by Astro (normally http://localhost:4321). Build and check page metadata, routes, anchors, assets, search results, and asset budgets:
 
 ```sh
 npm test
 ```
 
-`npm run preview` serves the production build locally after building. Deploy the `dist/` directory to any static host at the domain root. No runtime server, database, secrets, or environment variables are required. Configure the host to serve `404.html` for missing pages.
+`npm run preview` serves the production build. Deploy `dist/` to a static host at the domain root and configure it to serve `404.html` for missing pages. No runtime server, database, secrets, or environment variables are required.
 
-## Why Astro
+## Edit content
 
-[Astro](https://docs.astro.build/en/concepts/why-astro/) fits a content-focused site: Markdown pages, shared layouts, and static HTML with no client framework runtime. This site ships no JavaScript, external fonts, or image assets beyond a small SVG favicon. Astro is pinned to 7.4.1, which supports the repository environment's Node.js version; check Node requirements when upgrading.
+- `src/pages/index.astro`: homepage with the desktop preview, YAML example, workflow links, and source quick start.
+- `src/pages/docs/*.md`: guides. Their h2/h3 headings populate the page outline and static search index.
+- `src/pages/architecture.astro`: architecture guide and semantic HTML/CSS diagram.
+- `src/data/navigation.ts`: guide groups and previous/next order.
+- `src/layouts/{Site,Docs}.astro`: shared header, search dialog, and responsive documentation shell.
+- `src/components`: reusable icons, code examples, and guide navigation.
+- `src/styles/global.css`: light/dark tokens, typography, content styles, and responsive layouts. See `DESIGN.md` for the design contract.
+- `src/pages/search-index.json.ts`: build-time guide and section index. Uses Astro's compiled Markdown so heading links match rendered anchors.
+- `src/scripts/{site.ts,search.mjs}`: progressive enhancements and pure search ranking.
 
-React remains appropriate for DevOrbit's existing interactive desktop. Adding another React app, a backend, or a CMS is unnecessary for this website.
+Search loads its static index when first opened. Ctrl/Cmd K opens it, arrow keys navigate results, Enter follows a result, and Escape closes it and restores focus. A failed fetch has a retry action. Queries stay in the browser. Search result text is inserted as text, never as HTML.
 
-## Edit content
+Code copying provides confirmation and selects code for manual copying if clipboard access fails. The theme follows the system until the user chooses a preference; that choice is stored locally. Guide navigation and mobile menus use regular links and native details elements. With JavaScript disabled, content, links, code, and menus remain available; search, copy, and the theme button are hidden.
+
+The build is capped at 1 MiB total, with less than 16 KiB of client JavaScript. The budget includes all pages, the font, the screenshot, and the lazy-loaded search index. The homepage fetches the screenshot; documentation pages do not. Astro remains pinned to 7.4.1.
+
+## Assets and source of truth
+
+IBM Plex Sans is self-hosted in `public/fonts`, with its SIL Open Font License in `OFL.txt`. Source: [Google Fonts repository](https://github.com/google/fonts/tree/main/ofl/ibmplexsans).
 
-- `src/pages/index.astro`: product introduction.
-- `src/pages/architecture.astro`: architecture and native HTML/CSS diagram.
-- `src/pages/docs/*.md`: documentation. Each file uses `src/layouts/Docs.astro`; its second-level headings automatically populate the table of contents.
-- `src/layouts/Docs.astro`: guide order and sidebar links. Add new guide routes here.
-- `src/styles/global.css`: shared design and responsive layouts.
+`public/images/workspace.webp` is a capture of this repository's React desktop interface with synthetic workspace/service data supplied at the Tauri API boundary. It shows the real UI; the caption identifies the example data. It contains no personal workspace information. Capture viewport: 1440 × 900; compressed to WebP. Generated design concepts were used only as references and are not shipped assets.
 
-Guides are curated from `../README.md`, `../server/README.md`, and `../docs/`. Update the relevant website guide when product behavior changes; source links lead back to those technical documents. Run `npm test` before publishing.
+Guides are curated from the root README, `server/README.md`, `docs/`, and relevant source files. Preserve the existing heading anchors when editing established guides. Update the website alongside product behavior changes, run `npm test`, and inspect desktop/mobile layouts and keyboard interactions before publishing.

# website/src/components/Icon.astro

diff --git a/website/src/components/Icon.astro b/website/src/components/Icon.astro
index 8e28575..4769d47 100644
--- a/website/src/components/Icon.astro
+++ b/website/src/components/Icon.astro
@@ -1,9 +1,17 @@
 ---
 interface Props {
-  name?: 'arrow' | 'monitor' | 'terminal' | 'file' | 'database' | 'folder' | 'settings';
+  name?: 'arrow' | 'monitor' | 'terminal' | 'file' | 'database' | 'folder' | 'settings' | 'search' | 'sun' | 'moon' | 'github' | 'close' | 'chevron' | 'layers' | 'activity';
 }
 const { name = 'arrow' } = Astro.props;
 const paths = {
+  search: 'M21 21l-5-5 M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0',
+  sun: 'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M12 2v2 M12 20v2 M2 12h2 M20 12h2 M5 5l1.5 1.5 M17.5 17.5L19 19 M5 19l1.5-1.5 M17.5 6.5L19 5',
+  moon: 'M20.5 14A9 9 0 0 1 10 3a9 9 0 1 0 10.5 11Z',
+  github: 'M9 19c-4 1-4-2-6-2 M15 22v-4a3.5 3.5 0 0 0-1-2.7c3.3-.4 6.8-1.6 6.8-7.3A5.7 5.7 0 0 0 19.3 4c.2-1 .2-2-.3-3 0 0-1.2-.4-4 1.5a14 14 0 0 0-7 0C5.2.6 4 1 4 1c-.5 1-.5 2-.3 3a5.7 5.7 0 0 0-1.5 4c0 5.7 3.5 6.9 6.8 7.3A3.5 3.5 0 0 0 8 18v4',
+  close: 'M6 6l12 12 M6 18 18 6',
+  chevron: 'm6 9 6 6 6-6',
+  layers: 'm12 3 10 5-10 5L2 8z M2 12l10 5 10-5 M2 16l10 5 10-5',
+  activity: 'M2 12h5l3-8 4 16 3-8h5',
   arrow: 'M4 12h16m-6-6 6 6-6 6',
   monitor: 'M3 3h18v13H3z M8 21h8 M12 16v5',
   terminal: 'M3 3h18v18H3z M7 8l4 4-4 4 M13 16h4',

# website/src/layouts/Docs.astro

diff --git a/website/src/layouts/Docs.astro b/website/src/layouts/Docs.astro
index 4fb5437..3a4b6f2 100644
--- a/website/src/layouts/Docs.astro
+++ b/website/src/layouts/Docs.astro
@@ -1,34 +1,37 @@
 ---
-import type { MarkdownLayoutProps } from 'astro';
 import Site from './Site.astro';
 import Icon from '../components/Icon.astro';
-type Props = MarkdownLayoutProps<{ title: string; description: string; label: string; source: string }>;
+import DocsNav from '../components/DocsNav.astro';
+import { guides, repo } from '../data/navigation';
+interface Props {
+  frontmatter: { title: string; description: string; label: string; source: string };
+  headings: { depth: number; slug: string; text: string }[];
+}
 const { frontmatter } = Astro.props;
-const headings = Astro.props.headings.filter(({ depth }) => depth === 2);
-const guides = [
-  { href: '/docs/', label: 'Getting started' },
-  { href: '/docs/installation/', label: 'Installation' },
-  { href: '/docs/configuration/', label: 'Workspace configuration' },
-  { href: '/docs/services/', label: 'Services & terminals' },
-  { href: '/docs/api/', label: 'API reference' },
-];
+const headings = Astro.props.headings.filter(({ depth }) => depth === 2 || depth === 3);
 const current = guides.findIndex(({ href }) => href === Astro.url.pathname);
+const previous = guides[current - 1];
 const next = guides[current + 1];
+const source = `${repo}/blob/main/${frontmatter.source}`;
 ---
 <Site title={frontmatter.label} description={frontmatter.description}>
-  <main id="main" class="container docs-layout">
-    <aside class="docs-sidebar">
-      <nav class="desktop-docs-nav" aria-label="Documentation"><span class="rail-label">Documentation</span>{guides.map(({ href, label }) => <a href={href} aria-current={href === Astro.url.pathname ? 'page' : undefined}>{label}</a>)}</nav>
-      <details class="mobile-docs-nav"><summary>Documentation menu</summary><nav aria-label="Documentation">{guides.map(({ href, label }) => <a href={href} aria-current={href === Astro.url.pathname ? 'page' : undefined}>{label}</a>)}</nav></details>
-      <a class="source-link" href={`https://github.com/prasannashrestha011/devOrbit/blob/main/${frontmatter.source}`}>View source on GitHub <Icon /></a>
-    </aside>
-    <article class="docs-article">
-      <div class="breadcrumb"><a href="/docs/">Documentation</a><span aria-hidden="true">/</span><span>{frontmatter.label}</span></div>
-      <h1>{frontmatter.title}</h1>
-      <p class="docs-intro">{frontmatter.description}</p>
-      <div class="prose"><slot /></div>
-      {next && <a class="next-guide" href={next.href}>Next: {next.label} <Icon /></a>}
-    </article>
-    <nav class="toc" aria-label="On this page"><span class="rail-label">On this page</span>{headings.map(({ slug, text }) => <a href={`#${slug}`}>{text.replace(/^\d+\. /, '')}</a>)}</nav>
+  <main id="main" class="docs-layout">
+    <aside class="docs-sidebar"><DocsNav /></aside>
+    <div class="docs-center">
+      <details class="mobile-docs-nav"><summary>Documentation menu <Icon name="chevron" /></summary><DocsNav /></details>
+      <article class="docs-article">
+        <nav class="breadcrumb" aria-label="Breadcrumb"><a href="/docs/">Documentation</a><span aria-hidden="true">/</span><span>{frontmatter.label}</span></nav>
+        <h1>{frontmatter.title}</h1>
+        <p class="docs-intro">{frontmatter.description}</p>
+        <details class="mobile-toc"><summary>On this page <Icon name="chevron" /></summary><nav aria-label="On this page">{headings.map(({ slug, text, depth }) => <a class:list={{ nested: depth === 3 }} href={`#${slug}`}>{text}</a>)}</nav></details>
+        <div class="prose"><slot /></div>
+        <a class="source-link" href={source}>View source on GitHub <Icon /></a>
+        <nav class="page-pagination" aria-label="Documentation pages">
+          {previous ? <a href={previous.href}><span>Previous</span><strong>{previous.label}</strong></a> : <span />}
+          {next && <a class="next-page" href={next.href}><span>Next</span><strong>{next.label} <Icon /></strong></a>}
+        </nav>
+      </article>
+    </div>
+    <aside class="toc"><nav aria-label="On this page"><p>On this page</p>{headings.map(({ slug, text, depth }) => <a class:list={{ nested: depth === 3 }} href={`#${slug}`}>{text}</a>)}</nav><a class="source-link" href={source}>View source <Icon /></a></aside>
   </main>
 </Site>

# website/src/layouts/Site.astro

diff --git a/website/src/layouts/Site.astro b/website/src/layouts/Site.astro
index 3771989..177f4f4 100644
--- a/website/src/layouts/Site.astro
+++ b/website/src/layouts/Site.astro
@@ -1,14 +1,15 @@
 ---
 import '../styles/global.css';
+import Icon from '../components/Icon.astro';
+import { repo } from '../data/navigation';
 interface Props { title: string; description: string; }
 const { title, description } = Astro.props;
 const path = Astro.url.pathname;
 const links = [
   { href: '/', label: 'Overview' },
-  { href: '/architecture/', label: 'Architecture' },
   { href: '/docs/', label: 'Documentation' },
+  { href: '/architecture/', label: 'Architecture' },
 ];
-const repo = 'https://github.com/prasannashrestha011/devOrbit';
 ---
 <!doctype html>
 <html lang="en">
@@ -16,27 +17,48 @@ const repo = 'https://github.com/prasannashrestha011/devOrbit';
     <meta charset="UTF-8" />
     <meta name="viewport" content="width=device-width, initial-scale=1" />
     <meta name="description" content={description} />
-    <meta name="theme-color" content="#087f6f" />
+    <meta name="theme-color" content="#ffffff" />
     <meta property="og:title" content={`${title} | DevOrbit`} />
     <meta property="og:description" content={description} />
     <meta property="og:type" content="website" />
     <link rel="icon" href="/favicon.svg" type="image/svg+xml" />
+    <link rel="preload" href="/fonts/ibm-plex-sans.ttf" as="font" type="font/ttf" crossorigin />
     <title>{title} | DevOrbit</title>
+    <script is:inline>
+      try { const theme = localStorage.getItem('devorbit.theme'); if (theme === 'light' || theme === 'dark') document.documentElement.dataset.theme = theme; } catch {}
+    </script>
   </head>
   <body>
     <a class="skip-link" href="#main">Skip to content</a>
     <header class="site-header">
-      <div class="container header-inner">
+      <div class="header-inner">
         <a class="brand" href="/" aria-label="DevOrbit home"><span class="brand-icon" aria-hidden="true"><span></span></span><span>Dev<span class="brand-light">Orbit</span></span></a>
         <nav class="primary-nav" aria-label="Main navigation">
           {links.map(({ href, label }) => <a href={href} aria-current={(href === '/' ? path === href : path.startsWith(href)) ? 'page' : undefined}>{label}</a>)}
-          <a href={repo}>GitHub</a>
         </nav>
+        <div class="header-tools">
+          <button class="search-trigger" type="button" data-search-open hidden aria-label="Search documentation"><Icon name="search" /><span>Search docs…</span><kbd>Ctrl K</kbd></button>
+          <a class="github-link" href={repo} aria-label="DevOrbit on GitHub"><Icon name="github" /><span>GitHub</span></a>
+          <button class="icon-button theme-toggle" type="button" data-theme-toggle hidden aria-label="Switch color theme"><span class="light-icon"><Icon name="sun" /></span><span class="dark-icon"><Icon name="moon" /></span></button>
+          <details class="mobile-site-nav">
+            <summary>Menu <Icon name="chevron" /></summary>
+            <nav aria-label="Main navigation">{links.map(({ href, label }) => <a href={href}>{label}</a>)}</nav>
+          </details>
+        </div>
       </div>
     </header>
     <slot />
     <footer class="site-footer container">
-      <a href="/">DevOrbit</a><span>Built for local development.</span><a href={repo}>GitHub</a>
+      <div><a class="footer-brand" href="/">DevOrbit</a><span>Built for local development.</span></div>
+      <nav aria-label="Footer"><a href="/docs/">Documentation</a><a href="/docs/contributing/">Contribute <Icon /></a></nav>
     </footer>
+    <dialog class="search-dialog" aria-labelledby="search-title">
+      <div class="search-heading"><h2 id="search-title">Search documentation</h2><button type="button" class="icon-button" data-search-close aria-label="Close search"><Icon name="close" /></button></div>
+      <form role="search"><label class="sr-only" for="docs-search">Search documentation</label><div class="search-field"><Icon name="search" /><input id="docs-search" name="q" type="search" placeholder="Search guides, commands, or concepts…" autocomplete="off" spellcheck="false" /></div></form>
+      <div class="search-body"><p class="search-status" role="status" aria-live="polite"></p><ul class="search-results"></ul><button class="button secondary" type="button" data-search-retry hidden>Retry search</button></div>
+      <div class="search-help"><span><kbd>↑</kbd> <kbd>↓</kbd> to navigate · <kbd>Enter</kbd> to open</span><span><kbd>Esc</kbd> to close</span></div>
+    </dialog>
+    <div class="sr-only" id="copy-status" role="status" aria-live="polite"></div>
+    <script src="../scripts/site.ts"></script>
   </body>
 </html>

# website/src/pages/architecture.astro

diff --git a/website/src/pages/architecture.astro b/website/src/pages/architecture.astro
index 7c8bc7f..cb10396 100644
--- a/website/src/pages/architecture.astro
+++ b/website/src/pages/architecture.astro
@@ -1,29 +1,39 @@
 ---
-import Site from '../layouts/Site.astro';
+import Docs from '../layouts/Docs.astro';
 import Icon from '../components/Icon.astro';
+const frontmatter = { label: 'Architecture', title: 'Architecture', description: 'A desktop control surface, a local process supervisor, and configuration you can read.', source: 'server/README.md' };
+const headings = [
+  { depth: 2, slug: 'system-overview', text: 'System overview' },
+  { depth: 2, slug: 'desktop', text: 'Desktop' },
+  { depth: 2, slug: 'backend', text: 'Backend' },
+  { depth: 2, slug: 'storage', text: 'Storage' },
+  { depth: 2, slug: 'startup-and-shutdown', text: 'Startup and shutdown' },
+];
 ---
-<Site title="Architecture" description="How DevOrbit's React and Tauri desktop, Go process supervisor, workspace YAML, SQLite, and local logs work together.">
-  <main id="main" class="container architecture-page">
-    <header class="page-heading"><h1>A small set of pieces.<br />Clear responsibilities.</h1><p>The desktop is the control surface. The Go backend owns execution. YAML holds service configuration.</p></header>
-    <figure class="architecture-diagram" aria-label="DevOrbit architecture: the desktop connects to the Go backend through an authenticated local API. The backend manages processes and reads and writes workspace YAML, SQLite metadata, and local log files.">
-      <div class="diagram-label">On your machine</div>
-      <div class="architecture-node desktop-node"><Icon name="monitor" /><strong>React + Tauri desktop</strong></div>
-      <div class="api-connector"><span>Authenticated local API</span></div>
-      <div class="execution-row">
-        <div class="architecture-node backend-node"><Icon name="settings" /><div><strong>Go backend</strong><span>API · Orchestration · Process supervisor</span></div></div>
-        <div class="architecture-node process-node"><Icon name="terminal" /><div><strong>Managed processes</strong><span>PTY terminals + live events</span></div></div>
-      </div>
-      <div class="storage-row">
-        <div class="architecture-node"><Icon name="file" /><div><strong>Workspace YAML</strong><span>Service commands</span></div></div>
-        <div class="architecture-node"><Icon name="database" /><div><strong>SQLite</strong><span>Metadata + identities</span></div></div>
-        <div class="architecture-node"><Icon name="folder" /><div><strong>Local files</strong><span>Execution logs</span></div></div>
-      </div>
-    </figure>
-    <section class="responsibilities" aria-label="Component responsibilities">
-      <div><span class="responsibility-number">01</span><div><h2>Desktop</h2><p>React renders the interface. Rust forwards authenticated requests and connects terminal streams.</p></div></div>
-      <div><span class="responsibility-number">02</span><div><h2>Backend</h2><p>Go validates configuration, supervises services, and streams status, logs, and terminal output.</p></div></div>
-      <div><span class="responsibility-number">03</span><div><h2>Storage</h2><p>YAML is the source of service configuration. SQLite stores metadata. Logs stay on disk.</p></div></div>
-    </section>
-    <aside class="architecture-note"><Icon name="terminal" /><div><h2>Two processes, one workflow</h2><p>The desktop currently requires the backend to run separately. The development script starts both.</p></div><a class="text-link" href="/docs/">Set up DevOrbit <Icon /></a></aside>
-  </main>
-</Site>
+<Docs frontmatter={frontmatter} headings={headings}>
+  <h2 id="system-overview">System overview</h2>
+  <p>DevOrbit runs on your machine. The desktop sends authenticated requests to a Go backend that manages service processes and their local data.</p>
+  <figure class="architecture-diagram">
+    <div class="architecture-node"><Icon name="monitor" /><strong>React + Tauri desktop</strong></div>
+    <div class="api-connector">Authenticated local API &amp; terminal streams</div>
+    <div class="architecture-node"><Icon name="settings" /><div><strong>Go backend</strong><span>API · Orchestration · Process supervisor</span></div></div>
+    <div class="api-connector">Manages processes · Reads and writes local data</div>
+    <div class="architecture-node"><Icon name="terminal" /><div><strong>Managed processes</strong><span>PTY terminals + live events</span></div></div>
+    <div class="storage-row">
+      <div class="architecture-node"><Icon name="file" /><div><strong>Workspace YAML</strong><span>Service commands</span></div></div>
+      <div class="architecture-node"><Icon name="database" /><div><strong>SQLite</strong><span>Metadata + identities</span></div></div>
+      <div class="architecture-node"><Icon name="folder" /><div><strong>Local files</strong><span>Logs + traffic captures</span></div></div>
+    </div>
+    <figcaption>The desktop is the control surface. The backend owns execution.</figcaption>
+  </figure>
+  <h2 id="desktop">Desktop</h2>
+  <p>React renders the interface. The Rust host forwards authenticated API requests, connects terminal streams, and manages the bundled backend. The bearer token stays out of the renderer.</p>
+  <h2 id="backend">Backend</h2>
+  <p>Go validates configuration, supervises services, and streams status, logs, and terminal output. Processes run as the current OS user. DevOrbit does not sandbox service commands or automatically resolve their dependencies.</p>
+  <h2 id="storage">Storage</h2>
+  <p>Workspace YAML is the source of service configuration. SQLite stores metadata and stable service identities. Logs and traffic captures stay on disk in the private data directory.</p>
+  <p>Stop the backend before backing up its data directory. Keep the database on a local disk. Read <a href="/docs/configuration/#backend-settings">backend settings</a> for paths and configuration.</p>
+  <h2 id="startup-and-shutdown">Startup and shutdown</h2>
+  <p>The desktop starts its bundled backend automatically and stops it on exit. The source development script launches Tauri, which builds and starts the backend. SQLite data remains on disk for the next launch.</p>
+  <p>Continue with <a href="/docs/">getting started</a> or the <a href="/docs/api/">local API reference</a>.</p>
+</Docs>

# website/src/pages/docs/api.md

diff --git a/website/src/pages/docs/api.md b/website/src/pages/docs/api.md
index e1c5f59..5f72c0f 100644
--- a/website/src/pages/docs/api.md
+++ b/website/src/pages/docs/api.md
@@ -1,7 +1,7 @@
 ---
 layout: ../../layouts/Docs.astro
 label: API reference
-title: A local API for your stack.
+title: API reference
 description: The core routes for workspace configuration, service lifecycle, terminal streams, and persisted logs.
 source: server/README.md
 ---

# website/src/pages/docs/configuration.md

diff --git a/website/src/pages/docs/configuration.md b/website/src/pages/docs/configuration.md
index 934a1b1..e7c2c8e 100644
--- a/website/src/pages/docs/configuration.md
+++ b/website/src/pages/docs/configuration.md
@@ -1,7 +1,7 @@
 ---
 layout: ../../layouts/Docs.astro
 label: Workspace configuration
-title: Your stack, in a YAML file.
+title: Workspace configuration
 description: Keep service commands in one readable file, with a clear source of truth for every workspace.
 source: server/README.md
 ---

# website/src/pages/docs/index.md

diff --git a/website/src/pages/docs/index.md b/website/src/pages/docs/index.md
index 6cfa36e..d99558e 100644
--- a/website/src/pages/docs/index.md
+++ b/website/src/pages/docs/index.md
@@ -1,7 +1,7 @@
 ---
 layout: ../../layouts/Docs.astro
 label: Getting started
-title: From installation to running services.
+title: Getting started
 description: Install DevOrbit or build it from source, then bring your first project into the app.
 source: README.md
 ---

# website/src/pages/docs/installation.md

diff --git a/website/src/pages/docs/installation.md b/website/src/pages/docs/installation.md
index 594c6ea..e80866b 100644
--- a/website/src/pages/docs/installation.md
+++ b/website/src/pages/docs/installation.md
@@ -1,7 +1,7 @@
 ---
 layout: ../../layouts/Docs.astro
 label: Installation
-title: Install and open DevOrbit.
+title: Installation
 description: One desktop installer includes the backend and its local SQLite database.
 source: docs/install-release.md
 ---

# website/src/pages/docs/services.md

diff --git a/website/src/pages/docs/services.md b/website/src/pages/docs/services.md
index 63312d2..9d84973 100644
--- a/website/src/pages/docs/services.md
+++ b/website/src/pages/docs/services.md
@@ -1,7 +1,7 @@
 ---
 layout: ../../layouts/Docs.astro
 label: Services & terminals
-title: Follow every process.
+title: Services & terminals
 description: Start your services, work in their terminals, and inspect the output without losing your workspace context.
 source: server/README.md
 ---
@@ -20,7 +20,7 @@ Every service execution has a PTY session. The desktop uses xterm.js for the ter
 
 On Linux and macOS, service terminals use private tmux sessions; Windows uses ConPTY. Closing a terminal viewer detaches it without stopping its service. The backend continues draining output and retains up to 1 MiB for reconnect replay.
 
-Workspace shell terminals are separate sessions in the workspace root. Up to 16 sessions can exist per workspace, including exited sessions not yet closed. Shell output is not written to service logs. Backend shutdown ends these sessions.
+Workspace shell terminals are separate sessions in the user's home directory. Up to 16 sessions can exist per workspace, including exited sessions not yet closed. Shell output is not written to service logs. Backend shutdown ends these sessions.
 
 ## Read logs and events
 

# website/src/pages/index.astro

diff --git a/website/src/pages/index.astro b/website/src/pages/index.astro
index ed7addc..5cfe889 100644
--- a/website/src/pages/index.astro
+++ b/website/src/pages/index.astro
@@ -1,35 +1,40 @@
 ---
 import Site from '../layouts/Site.astro';
 import Icon from '../components/Icon.astro';
-const services = [
-  ['api', 'go run ./cmd/api'],
-  ['web', 'npm run dev'],
-  ['worker', 'python worker.py'],
-];
+import CodeBlock from '../components/CodeBlock.astro';
+import { repo } from '../data/navigation';
+const configuration = `services:
+  api:
+    command: go run ./cmd/api
+    cwd: backend
+  web:
+    command: npm run dev
+    cwd: frontend`;
+const quickstart = `cd desktop && npm ci && cd ..
+./dev.sh`;
 ---
-<Site title="Your local stack, in one place" description="DevOrbit brings your local project services, terminals, and logs into a single desktop workspace. Explore the architecture and get started.">
-  <main id="main" class="container">
-    <section class="hero" aria-labelledby="hero-title">
-      <div class="hero-copy">
-        <h1 id="hero-title">Your local stack.<br />One place to<br /><span>keep it running.</span></h1>
-        <p class="lead">Bring your project services, terminals, and logs into a single desktop workspace. Define your stack in YAML. Run it with DevOrbit.</p>
-        <div class="hero-actions"><a class="button" href="/docs/">Get started <Icon /></a><a class="text-link" href="/architecture/">Explore the architecture <Icon /></a></div>
+<Site title="Your local development workspace" description="Manage services, terminals, Docker Compose, and local HTTP traffic in one desktop workspace. Meet DevOrbit.">
+  <main id="main">
+    <section class="hero container">
+      <div class="hero-copy"><h1>Your local stack.<br />One place to run it.</h1><p>Services, terminals, Docker Compose, and HTTP traffic. Bring your development workflow into one desktop workspace.</p><div class="hero-actions"><a class="button" href="/docs/">Get started <Icon /></a><a class="button secondary" href={repo}><Icon name="github" /> View on GitHub</a></div><p class="hero-note">A desktop app for the projects on your machine.</p></div>
+      <figure class="product-preview"><a href="/images/workspace.webp" aria-label="View full-size DevOrbit desktop screenshot"><img src="/images/workspace.webp" width="1440" height="900" alt="DevOrbit workspace showing API and web services running, a stopped worker, service controls, and terminal access." fetchpriority="high" /></a><figcaption><span>DevOrbit desktop · Example workspace data</span><a href="/images/workspace.webp">View full size <Icon /></a></figcaption></figure>
+    </section>
+    <section class="configuration-section container">
+      <div><h2>Your commands.<br />Your configuration.</h2><p>Describe your services in a workspace YAML file. Keep commands and working directories together, then start, stop, and restart from DevOrbit.</p><a class="text-link" href="/docs/configuration/">Explore workspace configuration <Icon /></a></div>
+      <div><CodeBlock code={configuration} lang="yaml" title=".devorbit/workspace.yaml" /><p class="code-caption">An example API and frontend, configured in one file.</p></div>
+    </section>
+    <section class="workflow-section container" aria-labelledby="workflow-title">
+      <div class="section-heading"><h2 id="workflow-title">Stay close to what’s running.</h2><p>Follow the process, from command to request.</p></div>
+      <div class="workflow-links">
+        <a href="/docs/services/#use-interactive-terminals"><Icon name="terminal" /><h3>Services & terminals <Icon /></h3><p>Control processes, open interactive terminals, and follow their output.</p></a>
+        <a href="/docs/docker/"><Icon name="layers" /><h3>Docker Compose <Icon /></h3><p>Find Compose projects and manage their services alongside your local stack.</p></a>
+        <a href="/docs/traffic/"><Icon name="activity" /><h3>Traffic inspector <Icon /></h3><p>Route local HTTP requests through a gateway to inspect exchanges.</p></a>
       </div>
-      <figure class="workspace-preview">
-        <div class="diagram-label">Your workspace</div>
-        <div class="service-list">
-          {services.map(([name, command]) => <div class="service-row"><span class="service-name"><span class="status-dot" aria-hidden="true"></span>{name}</span><code>{command}</code></div>)}
-        </div>
-        <div class="workspace-connector" aria-hidden="true"><Icon /></div>
-        <div class="workspace-control"><strong>DevOrbit</strong><span>Start · Stop · Restart · Inspect</span></div>
-        <figcaption>A workflow illustration. Your commands, your runtimes.</figcaption>
-      </figure>
     </section>
-    <section class="feature-strip" aria-label="Why DevOrbit">
-      <div><span class="feature-number">01</span><h2>Configuration that travels</h2><p>Keep service commands in <code>.devorbit/workspace.yaml</code>, alongside your project.</p></div>
-      <div><span class="feature-number">02</span><h2>A clear view of your stack</h2><p>Manage processes, open terminals, and follow logs from one workspace.</p></div>
-      <div><span class="feature-number">03</span><h2>Local by design</h2><p>A Tauri desktop app connects to a Go backend on your machine.</p></div>
+    <section class="architecture-teaser container"><div><h2>Local by design.</h2><p>A Tauri desktop, a Go backend, and configuration you can read.</p></div><a class="text-link" href="/architecture/">See how it fits together <Icon /></a></section>
+    <section class="quickstart-section container" aria-labelledby="quickstart-title">
+      <div><h2 id="quickstart-title">Start with your next project.</h2><p>Choose a desktop build for your platform, or run DevOrbit from source.</p><a class="button" href="/docs/installation/">Installation guide <Icon /></a></div>
+      <div class="source-start"><h3>Building from source?</h3><p>From a repository checkout, after installing the <a href="/docs/#before-you-start">prerequisites</a>:</p><CodeBlock code={quickstart} title="Repository root · Linux / macOS" /><a class="text-link" href="/docs/#2-launch-devorbit">Full setup and Windows instructions <Icon /></a></div>
     </section>
-    <section class="docs-cta"><h2>Understand the pieces. Make them yours.</h2><a class="text-link" href="/docs/">Read the documentation <Icon /></a></section>
   </main>
 </Site>

# website/src/styles/global.css

diff --git a/website/src/styles/global.css b/website/src/styles/global.css
index 506614a..2ff0d24 100644
--- a/website/src/styles/global.css
+++ b/website/src/styles/global.css
@@ -1,242 +1,201 @@
+@font-face { font-family: 'IBM Plex Sans'; src: url('/fonts/ibm-plex-sans.ttf') format('truetype'); font-weight: 100 700; font-display: swap; }
 :root {
   color-scheme: light;
-  --ink: #172523;
-  --muted: #61716f;
-  --accent: #087f6f;
-  --accent-dark: #066b5e;
-  --mint: #eff9f6;
-  --border: #dce6e3;
-  font-family: Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
-  color: var(--ink);
-  background: #fff;
-  font-synthesis: none;
-  text-rendering: optimizeLegibility;
-  -webkit-font-smoothing: antialiased;
+  --bg: #fff; --surface: #f6f8f7; --ink: #182c28; --muted: #576a65; --line: #dce5e1;
+  --accent: #087f6f; --accent-hover: #056557; --accent-soft: #eaf5f1; --code-bg: #182322;
+  --warning-bg: #faf5e8; --warning-ink: #665120; --focus: #087f6f;
+  --space-1: .25rem; --space-2: .5rem; --space-3: .75rem; --space-4: 1rem; --space-6: 1.5rem; --space-8: 2rem; --space-12: 3rem; --space-16: 4rem;
+  --radius: 6px; --mono: 'SFMono-Regular', Consolas, 'Liberation Mono', monospace;
+  --search-shadow: 0 20px 64px #081f2930;
+}
+:root[data-theme='dark'] {
+  color-scheme: dark; --bg: #121b19; --surface: #192421; --ink: #e5ede9; --muted: #a5b7b0; --line: #30423c;
+  --accent: #71d9bb; --accent-hover: #9ee9d4; --accent-soft: #203c33; --code-bg: #101816;
+  --warning-bg: #302b1c; --warning-ink: #e4ce93; --focus: #71d9bb;
+}
+@media (prefers-color-scheme: dark) {
+  :root:not([data-theme='light']) {
+    color-scheme: dark; --bg: #121b19; --surface: #192421; --ink: #e5ede9; --muted: #a5b7b0; --line: #30423c;
+    --accent: #71d9bb; --accent-hover: #9ee9d4; --accent-soft: #203c33; --code-bg: #101816;
+    --warning-bg: #302b1c; --warning-ink: #e4ce93; --focus: #71d9bb;
+  }
 }
 * { box-sizing: border-box; }
-body { margin: 0; font-size: 16px; line-height: 1.65; }
-a { color: var(--accent); text-underline-offset: 4px; }
-a:hover { color: var(--accent-dark); }
-a:focus-visible, summary:focus-visible { outline: 3px solid var(--accent); outline-offset: 5px; border-radius: 2px; }
+html { scroll-padding-top: 100px; scrollbar-color: var(--line) var(--bg); }
+body { margin: 0; background: var(--bg); color: var(--ink); font-family: 'IBM Plex Sans', sans-serif; font-size: 16px; line-height: 1.6; -webkit-font-smoothing: antialiased; }
+::selection { background: var(--accent); color: var(--bg); }
+a { color: inherit; text-underline-offset: .2em; }
+a, button, summary, input { -webkit-tap-highlight-color: transparent; touch-action: manipulation; }
+a:hover { color: var(--accent); }
+button, input { font: inherit; }
+button, summary { cursor: pointer; }
+button { color: inherit; }
+button:disabled { opacity: .55; cursor: not-allowed; }
+:focus-visible { outline: 2px solid var(--focus); outline-offset: 4px; }
+[hidden] { display: none !important; }
 h1, h2, h3, p, figure { margin: 0; }
-h1, h2, h3 { line-height: 1.15; letter-spacing: -.035em; }
-h1 { font-weight: 750; }
-h2 { font-weight: 700; }
-code, pre { font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace; }
-code { font-size: .9em; }
-.container { width: min(1280px, calc(100% - 112px)); margin-inline: auto; }
-.icon { width: 22px; height: 22px; flex-shrink: 0; vertical-align: middle; }
-.skip-link { position: absolute; left: 24px; top: -80px; z-index: 10; background: #fff; padding: 10px 20px; }
-.skip-link:focus { top: 12px; }
-.site-header { border-bottom: 1px solid var(--border); }
-.header-inner { min-height: 82px; display: flex; align-items: center; justify-content: space-between; gap: 24px; }
-.brand { display: inline-flex; align-items: center; gap: 16px; color: var(--ink); font-size: 30px; font-weight: 750; letter-spacing: -.05em; text-decoration: none; white-space: nowrap; }
-.brand-light { font-weight: 550; }
-.brand-icon { width: 27px; height: 27px; margin-inline: 4px; border: 2px solid var(--accent); border-radius: 50%; position: relative; transform: rotate(-28deg); }
-.brand-icon::after { content: ""; position: absolute; width: 33px; height: 14px; border: 2px solid var(--accent); border-radius: 50%; left: -5px; top: 4px; }
-.brand-icon span { position: absolute; right: -7px; top: 0; width: 6px; height: 6px; background: var(--accent); border-radius: 50%; }
-.primary-nav { display: flex; gap: 38px; align-self: stretch; }
-.primary-nav a { color: var(--ink); display: flex; align-items: center; font-size: 16px; text-decoration: none; border-bottom: 2px solid transparent; padding-top: 2px; }
-.primary-nav a[aria-current], .primary-nav a:hover { color: var(--accent); border-bottom-color: var(--accent); }
-.hero { display: grid; grid-template-columns: 1.04fr 1fr; gap: 54px; align-items: center; padding: 78px 0 64px; }
-.hero h1 { font-size: clamp(44px, 5.3vw, 76px); line-height: 1.12; letter-spacing: -.055em; }
-.hero h1 span { color: var(--accent); }
-.lead { font-size: 22px; color: var(--muted); line-height: 1.6; margin-top: 30px; }
-.hero-actions { display: flex; align-items: center; gap: 30px; margin-top: 28px; }
-.button, .text-link { display: inline-flex; align-items: center; justify-content: center; gap: 12px; text-decoration: none; font-size: 17px; font-weight: 550; }
-.button { padding: 14px 24px; border-radius: 5px; background: var(--accent); color: #fff; white-space: nowrap; }
-.button:hover { background: var(--accent-dark); color: #fff; }
-.hero-actions .text-link { color: var(--ink); }
+h1, h2, h3 { line-height: 1.18; font-weight: 600; text-wrap: balance; }
+h1 { letter-spacing: -.035em; } h2 { letter-spacing: -.025em; } h3 { letter-spacing: -.015em; }
+.icon { width: 20px; height: 20px; flex-shrink: 0; vertical-align: middle; }
+.container { width: min(1200px, calc(100% - 80px)); margin-inline: auto; }
+.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
+.skip-link { position: fixed; top: 8px; left: 8px; z-index: 100; transform: translateY(-180%); background: var(--bg); padding: 10px 16px; border: 2px solid var(--accent); border-radius: var(--radius); }
+.skip-link:focus { transform: translateY(0); }
+.site-header { position: sticky; top: 0; z-index: 20; background: var(--bg); border-bottom: 1px solid var(--line); }
+.header-inner { max-width: 1400px; padding-inline: 40px; height: 72px; margin: auto; display: flex; align-items: center; gap: 40px; }
+.brand { display: inline-flex; align-items: center; gap: 10px; font-weight: 600; font-size: 23px; text-decoration: none; letter-spacing: -.035em; white-space: nowrap; }
+.brand-light { font-weight: 400; }
+.brand-icon { width: 26px; height: 26px; border: 1.8px solid var(--accent); border-radius: 50%; display: inline-block; position: relative; transform: rotate(-30deg); }
+.brand-icon::before { content: ''; position: absolute; inset: 5px -5px; border: 1.5px solid var(--accent); border-radius: 50%; }
+.brand-icon span { width: 5px; height: 5px; border-radius: 50%; background: var(--accent); position: absolute; right: 0; top: -4px; }
+.primary-nav { display: flex; align-items: center; gap: 28px; align-self: stretch; }
+.primary-nav a { display: flex; align-items: center; font-size: 14px; font-weight: 500; text-decoration: none; color: var(--muted); border-bottom: 2px solid transparent; padding-top: 2px; }
+.primary-nav a[aria-current], .primary-nav a:hover { color: var(--accent); border-color: var(--accent); }
+.header-tools { display: flex; align-items: center; gap: 18px; margin-left: auto; }
+.search-trigger { display: flex; gap: 10px; align-items: center; background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); padding: 7px 10px; font-size: 14px; color: var(--muted); }
+.search-trigger .icon { width: 16px; }
+kbd { font-family: var(--mono); font-size: 11px; border: 1px solid var(--line); border-radius: 3px; padding: 1px 4px; }
+.search-trigger kbd { margin-left: 16px; }
+.github-link { display: inline-flex; align-items: center; gap: 7px; text-decoration: none; font-size: 14px; }
+.icon-button { display: inline-flex; align-items: center; justify-content: center; width: 36px; height: 36px; padding: 0; border: 0; background: transparent; border-radius: var(--radius); }
+.icon-button:hover { background: var(--surface); color: var(--accent); }
+.dark-icon { display: none; }
+:root[data-theme='dark'] .dark-icon { display: inline; } :root[data-theme='dark'] .light-icon { display: none; }
+@media (prefers-color-scheme: dark) { :root:not([data-theme='light']) .dark-icon { display: inline; } :root:not([data-theme='light']) .light-icon { display: none; } }
+.mobile-site-nav { display: none; position: relative; }
+summary { list-style: none; } summary::-webkit-details-marker { display: none; }
+summary .icon { width: 16px; } details[open] > summary > .icon { transform: rotate(180deg); }
+.mobile-site-nav summary { display: flex; align-items: center; gap: 5px; font-size: 14px; min-height: 44px; }
+.mobile-site-nav nav { position: absolute; right: 0; top: 48px; min-width: 190px; padding: 8px; background: var(--bg); border: 1px solid var(--line); border-radius: var(--radius); }
+.mobile-site-nav nav a { display: block; padding: 10px; text-decoration: none; }
+.button { display: inline-flex; justify-content: center; align-items: center; gap: 10px; border: 1px solid transparent; padding: 10px 17px; border-radius: var(--radius); background: var(--accent); color: var(--bg); font-size: 15px; font-weight: 500; text-decoration: none; min-height: 44px; }
+.button:hover { background: var(--accent-hover); color: var(--bg); }
+.button.secondary { background: var(--bg); border-color: var(--line); color: var(--ink); }
+.button.secondary:hover { border-color: var(--accent); color: var(--accent); }
+.text-link { display: inline-flex; align-items: center; gap: 8px; font-size: 15px; font-weight: 500; color: var(--accent); text-decoration: none; }
 .text-link:hover { text-decoration: underline; }
-.workspace-preview { --service-indent: 54px; padding: 24px; border: 1px solid var(--border); border-radius: 7px; }
-.diagram-label { font-size: 12px; text-transform: uppercase; letter-spacing: .08em; font-weight: 650; color: var(--muted); }
-.service-list { margin: 18px 0 0 var(--service-indent); position: relative; display: grid; gap: 14px; }
-.service-list::before { content: ""; position: absolute; top: 32px; bottom: -31px; left: -34px; border: 2px solid #a6bcb7; border-right: 0; border-radius: 8px 0 0 8px; width: 34px; }
-.service-row { position: relative; border: 1px solid var(--border); border-radius: 7px; padding: 14px 16px; display: flex; align-items: center; justify-content: space-between; gap: 12px; }
-.service-row::before { content: ""; position: absolute; left: -34px; width: 33px; height: 2px; background: #a6bcb7; }
-.service-row:first-child::before { display: none; }
-.service-name { display: flex; align-items: center; gap: 16px; font-size: 20px; font-weight: 650; }
-.status-dot { width: 11px; height: 11px; background: #27b554; border-radius: 50%; flex-shrink: 0; }
-.service-row code { width: 232px; max-width: 65%; padding: 8px 12px; background: #f3f6f5; border-radius: 5px; color: #405550; font-size: 15px; line-height: 1.5; }
-.workspace-connector { height: 66px; position: relative; }
-.workspace-connector::before { content: ""; position: absolute; left: var(--service-indent); right: 50%; top: 29px; height: 18px; border-top: 2px solid #a6bcb7; border-right: 2px solid #a6bcb7; border-radius: 0 7px 0 0; }
-.workspace-connector .icon { color: #a6bcb7; position: absolute; left: calc(50% - 12px); top: 38px; transform: rotate(90deg); }
-.workspace-control { background: #08796b; color: #fff; text-align: center; padding: 12px; border-radius: 9px; display: grid; }
-.workspace-control strong { font-size: 24px; }
-.workspace-control span { font-family: monospace; color: #d1eee8; font-size: 14px; margin-top: 1px; }
-.workspace-preview figcaption { color: var(--muted); font-size: 13px; text-align: center; margin-top: 16px; }
-.feature-strip { display: grid; grid-template-columns: repeat(3, 1fr); border-top: 1px solid var(--border); }
-.feature-strip > div { padding: 30px 36px 34px; }
-.feature-strip > div:first-child { padding-left: 18px; }
-.feature-strip > div + div { border-left: 1px solid var(--border); }
-.feature-number { display: block; color: var(--accent); font-size: 14px; font-weight: 600; margin-bottom: 14px; }
-.feature-strip h2 { font-size: 25px; margin-bottom: 12px; }
-.feature-strip p { color: var(--muted); font-size: 19px; line-height: 1.55; }
-.feature-strip code { overflow-wrap: anywhere; }
-.docs-cta { display: flex; align-items: center; justify-content: space-between; gap: 24px; background: var(--mint); border-radius: 7px; padding: 30px 34px; margin-bottom: 22px; }
-.docs-cta h2 { font-size: 26px; }
-.docs-cta .text-link { white-space: nowrap; }
-.site-footer { border-top: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; gap: 20px; padding-block: 18px; font-size: 13px; color: var(--muted); }
-.site-footer a { color: inherit; text-decoration: none; }
-.site-footer a:hover { color: var(--accent); text-decoration: underline; }
-.architecture-page { max-width: 1200px; }
-.page-heading { text-align: center; padding: 34px 0 32px; }
-.page-heading h1 { font-size: 52px; }
-.page-heading p { color: var(--muted); font-size: 19px; margin-top: 20px; }
-.architecture-diagram { padding: 28px 26px 36px; border: 1px solid #c6ded7; border-radius: 6px; background-color: #f8fbfa; background-image: linear-gradient(#eaf2ef 1px, transparent 1px), linear-gradient(90deg, #eaf2ef 1px, transparent 1px); background-size: 20px 20px; position: relative; }
-.architecture-diagram > .diagram-label { position: absolute; top: 20px; left: 24px; font-family: monospace; }
-.architecture-node { min-height: 82px; padding: 20px; border: 1px solid #b5d2ca; border-radius: 5px; background: #fff; display: flex; align-items: center; justify-content: center; gap: 20px; position: relative; }
-.architecture-node .icon { width: 30px; height: 30px; color: var(--accent); }
-.architecture-node strong { display: block; font-size: 16px; line-height: 1.4; }
-.architecture-node span { display: block; color: var(--muted); font-size: 13px; line-height: 1.5; margin-top: 3px; }
-.desktop-node { width: 340px; margin-inline: auto; }
-.api-connector { height: 94px; position: relative; display: grid; place-items: center; }
-.api-connector::before { content: ""; position: absolute; top: 0; bottom: 0; width: 1px; background: #78918b; }
-.api-connector::after, .storage-row > div::after { content: ""; position: absolute; bottom: 0; left: calc(50% - 5px); border: 5px solid transparent; border-bottom: 0; border-top: 8px solid #78918b; }
-.api-connector span { position: relative; background: #e2f3ee; padding: 4px 10px; border-radius: 16px; font-size: 12px; color: var(--ink); }
-.execution-row { display: grid; grid-template-columns: 1fr 340px 1fr; align-items: stretch; }
-.backend-node { grid-column: 2; background: #08796b; border-color: #08796b; color: #fff; }
-.backend-node .icon { color: #fff; }
-.backend-node span { color: #d1eee8; }
-.process-node { margin-left: 70px; padding-inline: 16px; gap: 14px; }
-.process-node::before { content: ""; position: absolute; width: 70px; height: 1px; background: #78918b; left: -71px; top: 50%; }
-.process-node::after { content: ""; position: absolute; left: -8px; top: calc(50% - 5px); border: 5px solid transparent; border-right: 0; border-left: 8px solid #78918b; }
-.storage-row { width: 72%; margin: 78px auto 0; display: grid; grid-template-columns: repeat(3, 1fr); gap: 22px; position: relative; }
-.storage-row::before { content: ""; position: absolute; width: 1px; height: 78px; background: #78918b; left: 50%; top: -78px; }
-.storage-row::after { content: ""; position: absolute; left: calc((100% - 44px) / 6); right: calc((100% - 44px) / 6); top: -38px; height: 38px; border: 1px solid #78918b; border-bottom: 0; border-radius: 6px 6px 0 0; }
-.storage-row > div::after { bottom: auto; top: -8px; }
-.storage-row .architecture-node { padding: 18px 14px; gap: 16px; }
-.responsibilities { display: grid; grid-template-columns: repeat(3, 1fr); margin: 30px 0; }
-.responsibilities > div { display: flex; gap: 24px; padding-inline: 26px; }
-.responsibilities > div:first-child { padding-left: 0; }
-.responsibilities > div + div { border-left: 1px solid var(--border); }
-.responsibility-number { display: grid; place-items: center; flex-shrink: 0; width: 40px; height: 40px; background: #e2f3ee; color: var(--accent); border-radius: 50%; font-size: 14px; font-weight: 600; }
-.responsibilities h2, .architecture-note h2 { font-size: 18px; margin: 10px 0 8px; letter-spacing: -.02em; }
-.responsibilities p, .architecture-note p { color: var(--muted); font-size: 14px; line-height: 1.55; }
-.architecture-note { display: flex; align-items: center; gap: 28px; border: 1px solid #c6ded7; background: #f8fbfa; border-radius: 5px; padding: 18px 24px; margin-bottom: 28px; }
-.architecture-note > .icon { width: 32px; height: 32px; color: var(--accent); }
-.architecture-note h2 { margin-top: 0; }
-.architecture-note .text-link { margin-left: auto; flex-shrink: 0; font-size: 14px; }
-.docs-layout { max-width: 1226px; display: grid; grid-template-columns: 244px minmax(0, 1fr) 188px; gap: 22px; padding-top: 24px; padding-bottom: 40px; }
-.docs-sidebar { border-right: 1px solid var(--border); padding: 16px 22px 0 0; }
-.desktop-docs-nav { position: sticky; top: 24px; }
-.rail-label { display: block; font-size: 11px; text-transform: uppercase; letter-spacing: .04em; color: var(--muted); margin-bottom: 16px; font-weight: 550; }
-.docs-sidebar nav a { display: block; padding: 9px 12px; border-left: 3px solid transparent; color: var(--muted); font-size: 14px; text-decoration: none; line-height: 1.5; }
-.docs-sidebar nav a:hover { color: var(--accent); }
-.docs-sidebar nav a[aria-current] { background: var(--mint); border-left-color: var(--accent); color: var(--accent-dark); font-weight: 600; }
-.source-link { border-top: 1px solid var(--border); margin-top: 18px; padding-top: 16px; display: flex; align-items: center; gap: 8px; color: var(--muted); font-size: 12px; text-decoration: none; }
-.source-link .icon { width: 15px; height: 15px; transform: rotate(-45deg); }
-.mobile-docs-nav { display: none; }
-.breadcrumb { font-size: 12px; display: flex; flex-wrap: wrap; gap: 10px; color: var(--muted); margin: 0 0 18px; }
-.breadcrumb a { color: inherit; text-decoration: none; }
-.docs-article h1 { font-size: clamp(36px, 3.6vw, 50px); max-width: 590px; letter-spacing: -.045em; }
-.docs-intro { color: var(--muted); font-size: 18px; line-height: 1.6; margin-top: 14px; }
-.prose { font-size: 15px; color: #4c605b; line-height: 1.75; overflow-wrap: anywhere; }
-.prose h2 { font-size: 29px; color: var(--ink); margin: 32px 0 12px; scroll-margin-top: 24px; }
-.prose h3 { font-size: 21px; margin: 24px 0 12px; color: var(--ink); }
-.prose p { margin-block: 14px; }
-.prose strong { color: var(--ink); }
-.prose code { background: var(--mint); color: #246254; padding: 2px 5px; border-radius: 4px; font-size: .87em; }
-.prose pre { padding: 18px 20px; border-radius: 6px; overflow: auto; font-size: 13px; line-height: 1.75; margin: 16px 0; background: #1c2c29 !important; }
-.prose pre code { padding: 0; background: transparent; color: inherit; font-size: inherit; }
-.prose table { width: 100%; table-layout: fixed; border-collapse: collapse; margin: 16px 0 20px; font-size: 14px; line-height: 1.5; }
-.prose th, .prose td { text-align: left; border-bottom: 1px solid var(--border); padding: 5px 12px; vertical-align: top; }
-.prose th { font-weight: 600; color: var(--ink); background: #f8fbfa; }
-.prose td:first-child { color: var(--ink); }
-.callout { color: #325a50; background: var(--mint); border: 1px solid #cce7de; border-radius: 6px; padding: 15px 18px; margin: 20px 0 24px; font-size: 14px; }
-.toc { align-self: start; position: sticky; top: 24px; border-left: 1px solid var(--border); padding: 16px 0 16px 20px; }
-.toc a { display: block; color: var(--muted); font-size: 13px; line-height: 1.5; margin: 12px 0; text-decoration: none; }
-.toc a:hover { color: var(--accent); text-decoration: underline; }
-.next-guide { border-top: 1px solid var(--border); margin-top: 30px; padding-top: 20px; display: flex; justify-content: flex-end; gap: 14px; align-items: center; text-decoration: none; font-size: 14px; }
-.next-guide:hover { text-decoration: underline; }
-.not-found { padding-block: 110px; min-height: 70vh; }
-.not-found h1 { font-size: clamp(38px, 6vw, 64px); }
-.not-found p { margin-top: 24px; color: var(--muted); }
-@media (max-width: 1100px) {
-  .container { width: calc(100% - 64px); }
-  .hero { gap: 32px; }
-  .hero h1 { font-size: 46px; }
-  .lead { font-size: 18px; }
-  .hero-actions { align-items: flex-start; flex-direction: column; gap: 20px; }
-  .workspace-preview { --service-indent: 32px; }
-  .service-row { padding: 12px; }
-  .service-row code { font-size: 11px; padding: 7px; width: auto; max-width: none; }
-  .service-name { gap: 10px; font-size: 16px; }
-  .feature-strip > div { padding-inline: 24px; }
-  .feature-strip h2 { font-size: 21px; }
-  .docs-cta h2 { font-size: 23px; }
-  .page-heading h1 { font-size: 44px; }
-  .execution-row { grid-template-columns: 1fr 280px 1fr; }
-  .desktop-node { width: 280px; }
-  .process-node { margin-left: 24px; }
-  .process-node::before { width: 24px; left: -25px; }
-  .architecture-node { padding: 16px; gap: 12px; }
-  .architecture-node .icon { width: 24px; height: 24px; }
-  .architecture-node strong { font-size: 13px; }
-  .storage-row { width: 90%; }
-  .docs-layout { grid-template-columns: 200px minmax(0, 1fr); gap: 28px; }
-  .toc { display: none; }
-  .responsibilities > div { padding-inline: 16px; gap: 14px; }
-  .architecture-note { flex-wrap: wrap; }
-  .architecture-note .text-link { margin-left: 60px; }
-}
-@media (max-width: 760px) {
-  .container { width: calc(100% - 48px); }
-  .header-inner { flex-wrap: wrap; gap: 12px; padding-top: 20px; }
-  .brand { font-size: 25px; }
-  .primary-nav { width: 100%; justify-content: space-between; gap: 12px; }
-  .primary-nav a { font-size: 13px; padding: 12px 0; }
-  .hero { grid-template-columns: 1fr; padding: 46px 0 40px; gap: 36px; }
-  .hero h1 { font-size: clamp(35px, 7.7vw, 54px); }
-  .lead { margin-top: 22px; }
-  .hero-actions { flex-direction: row; align-items: center; flex-wrap: wrap; gap: 20px; }
-  .hero-actions .text-link { font-size: 15px; }
-  .workspace-preview { padding: 20px; }
-  .service-row code { font-size: 12px; }
-  .feature-strip { grid-template-columns: 1fr; }
-  .feature-strip > div, .feature-strip > div:first-child { padding: 25px 0; }
-  .feature-strip > div + div { border-left: 0; border-top: 1px solid var(--border); }
-  .feature-number { margin-bottom: 8px; }
-  .feature-strip h2 { font-size: 23px; }
-  .docs-cta { align-items: flex-start; flex-direction: column; padding: 25px; margin-top: 12px; }
-  .docs-cta .text-link { white-space: normal; }
-  .site-footer { flex-wrap: wrap; gap: 8px; }
-  .site-footer span { order: 3; width: 100%; }
-  .page-heading { padding: 38px 0 28px; text-align: left; }
-  .page-heading h1 { font-size: clamp(32px, 7vw, 44px); }
-  .page-heading p { font-size: 17px; }
-  .architecture-diagram { padding: 58px 20px 24px; }
-  .desktop-node { width: 100%; }
-  .execution-row { display: flex; flex-direction: column; }
-  .api-connector { height: 80px; }
-  .process-node { margin: 36px 0 0; }
-  .process-node::before { width: 1px; height: 35px; left: 50%; top: -36px; }
-  .process-node::after { left: calc(50% - 5px); top: -8px; border: 5px solid transparent; border-bottom: 0; border-top: 8px solid #78918b; }
-  .storage-row { width: 100%; margin-top: 32px; grid-template-columns: 1fr; gap: 12px; border-top: 1px solid var(--border); padding-top: 36px; }
-  .storage-row::before { content: 'Backend storage'; height: auto; width: 100%; top: 7px; left: 0; font-size: 12px; text-align: center; color: var(--muted); background: none; }
-  .storage-row::after, .storage-row > div::after { display: none; }
-  .architecture-node strong { font-size: 15px; }
-  .responsibilities { grid-template-columns: 1fr; margin: 8px 0 24px; }
-  .responsibilities > div, .responsibilities > div:first-child { padding: 20px 0; }
-  .responsibilities > div + div { border-left: 0; border-top: 1px solid var(--border); }
-  .responsibilities h2 { margin-top: 8px; }
-  .architecture-note { gap: 14px; }
-  .architecture-note > .icon { display: none; }
-  .architecture-note .text-link { margin-left: 0; }
-  .docs-layout { display: block; padding-top: 18px; }
-  .docs-sidebar { padding: 0; border: 0; margin-bottom: 28px; }
-  .desktop-docs-nav { display: none; }
-  .mobile-docs-nav { display: block; border: 1px solid var(--border); border-radius: 5px; background: #fff; }
-  .mobile-docs-nav summary { cursor: pointer; padding: 10px 14px; font-size: 14px; color: var(--accent-dark); }
-  .mobile-docs-nav nav { padding: 4px 12px 12px; }
-  .source-link { margin-top: 10px; padding-top: 0; border: 0; }
-  .docs-article h1 { font-size: 37px; }
-  .docs-intro { font-size: 17px; }
-  .prose { font-size: 15px; }
-  .prose h2 { font-size: 26px; }
-  .prose th, .prose td { padding: 8px; }
-  .prose pre { padding: 16px; }
-}
-@media (max-width: 360px) {
-  .primary-nav { display: grid; grid-template-columns: 1fr 1fr; gap: 0 24px; }
-}
+.hero { display: grid; grid-template-columns: .95fr 1.25fr; gap: 52px; padding-block: 88px 76px; align-items: center; }
+.hero h1 { font-size: clamp(38px, 3.65vw, 54px); line-height: 1.08; }
+.hero-copy > p { font-size: 18px; color: var(--muted); line-height: 1.6; margin-top: 22px; max-width: 43ch; }
+.hero-actions { display: flex; gap: 12px; margin-top: 28px; flex-wrap: wrap; }
+.hero-copy .hero-note { font-size: 13px; margin-top: 18px; }
+.product-preview { min-width: 0; }
+.product-preview > a { display: block; }
+.product-preview img { display: block; width: 100%; height: auto; border-radius: 8px; border: 1px solid var(--line); }
+.product-preview figcaption { display: flex; justify-content: space-between; gap: 12px; margin-top: 12px; font-size: 11px; color: var(--muted); }
+.product-preview figcaption a { white-space: nowrap; text-decoration: none; }
+.product-preview figcaption .icon { width: 12px; }
+.configuration-section { border-top: 1px solid var(--line); padding-block: 64px; display: grid; grid-template-columns: .9fr 1.1fr; gap: 120px; align-items: center; }
+.configuration-section h2, .quickstart-section h2 { font-size: 34px; }
+.configuration-section p, .quickstart-section > div > p { color: var(--muted); margin: 18px 0 24px; max-width: 46ch; }
+.configuration-section .code-caption { margin: 10px 0 0; font-size: 12px; }
+.code-example { min-width: 0; }
+.code-example pre { margin: 0; }
+.code-frame { background: var(--code-bg); border-radius: 8px; overflow: hidden; margin: 24px 0; }
+.code-example .code-frame { margin: 0; }
+.code-toolbar { padding: 8px 15px; display: flex; align-items: center; justify-content: space-between; gap: 16px; border-bottom: 1px solid #ffffff1f; color: #c4d5cf; font-family: var(--mono); font-size: 11px; }
+.copy-code { color: #dce9e4; border: 1px solid #50655d; border-radius: 4px; font-size: 11px; background: transparent; padding: 4px 9px; min-height: 28px; }
+.copy-code:hover { background: #ffffff16; }
+pre { background: var(--code-bg) !important; padding: 20px; overflow-x: auto; font-size: 13px; line-height: 1.8; tab-size: 2; border-radius: 8px; }
+.code-frame pre { border-radius: 0; margin: 0; }
+code { font-family: var(--mono); }
+.workflow-section { padding-block: 24px 56px; }
+.section-heading h2 { font-size: 30px; }
+.section-heading > p { color: var(--muted); margin-top: 10px; }
+.workflow-links { display: grid; grid-template-columns: repeat(3, 1fr); gap: 48px; margin-top: 36px; }
+.workflow-links > a { text-decoration: none; padding-top: 22px; border-top: 1px solid var(--line); }
+.workflow-links > a:hover { border-color: var(--accent); }
+.workflow-links > a > .icon { color: var(--accent); margin-bottom: 17px; width: 24px; height: 24px; }
+.workflow-links h3 { font-size: 18px; display: flex; align-items: center; gap: 8px; }
+.workflow-links h3 .icon { width: 16px; }
+.workflow-links p { font-size: 15px; margin-top: 12px; color: var(--muted); line-height: 1.6; max-width: 34ch; }
+.architecture-teaser { display: flex; justify-content: space-between; align-items: center; gap: 24px; padding-block: 28px; border-block: 1px solid var(--line); }
+.architecture-teaser h2 { font-size: 22px; }
+.architecture-teaser p { margin-top: 7px; font-size: 15px; color: var(--muted); }
+.quickstart-section { display: grid; grid-template-columns: .9fr 1.1fr; gap: 120px; padding-block: 64px 80px; }
+.source-start h3 { font-size: 18px; }
+.source-start > p { font-size: 14px; }
+.source-start > .text-link { margin-top: 15px; font-size: 13px; }
+.site-footer { border-top: 1px solid var(--line); padding-block: 30px; display: flex; align-items: center; justify-content: space-between; gap: 24px; color: var(--muted); font-size: 13px; }
+.site-footer > div, .site-footer nav { display: flex; align-items: center; gap: 24px; }
+.site-footer a { text-decoration: none; }
+.site-footer .footer-brand { font-size: 16px; color: var(--ink); font-weight: 500; }
+.site-footer .icon { width: 14px; }
+.docs-layout { max-width: 1400px; padding-inline: 40px; margin: auto; display: grid; grid-template-columns: 216px minmax(0, 720px) 190px; gap: 52px; align-items: start; min-height: calc(100vh - 170px); }
+.docs-sidebar, .toc { position: sticky; top: 72px; max-height: calc(100vh - 72px); overflow-y: auto; padding-block: 36px; scrollbar-width: thin; }
+.docs-sidebar { padding-right: 16px; border-right: 1px solid var(--line); }
+.nav-group + .nav-group { margin-top: 23px; }
+.nav-group p { font-size: 13px; font-weight: 600; padding: 0 12px 8px; }
+.docs-nav a { display: block; padding: 7px 12px; text-decoration: none; font-size: 14px; color: var(--muted); border-radius: 4px; line-height: 1.4; margin-block: 2px; }
+.docs-nav a:hover { color: var(--accent); background: var(--surface); }
+.docs-nav a[aria-current] { background: var(--accent-soft); color: var(--accent); font-weight: 500; }
+.docs-center { min-width: 0; }
+.docs-article { padding-block: 42px 64px; }
+.breadcrumb { display: flex; gap: 10px; color: var(--muted); font-size: 12px; margin-bottom: 20px; flex-wrap: wrap; }
+.breadcrumb a { text-decoration: none; }
+.docs-article h1 { font-size: 40px; line-height: 1.15; }
+.docs-intro { font-size: 18px; line-height: 1.65; color: var(--muted); margin-top: 18px; padding-bottom: 28px; border-bottom: 1px solid var(--line); }
+.prose { font-size: 16px; line-height: 1.75; overflow-wrap: break-word; }
+.prose h2 { font-size: 25px; margin: 42px 0 16px; }
+.prose h3 { font-size: 19px; margin: 30px 0 12px; }
+.prose p { margin-block: 16px; }
+.prose ul, .prose ol { padding-left: 24px; margin-block: 16px; }
+.prose li { padding-left: 4px; margin-block: 7px; }
+.prose a { color: var(--accent); }
+.prose strong { font-weight: 600; }
+.prose :not(pre) > code { background: var(--surface); border: 1px solid var(--line); padding: 1px 4px; border-radius: 4px; font-size: .84em; overflow-wrap: anywhere; }
+.heading-anchor { margin-left: 8px; font-weight: 400; font-size: .8em; opacity: 0; text-decoration: none; }
+.prose :is(h2,h3):hover .heading-anchor, .heading-anchor:focus { opacity: 1; }
+.table-scroll { overflow-x: auto; margin: 24px 0; }
+.prose table { width: 100%; border-collapse: collapse; text-align: left; font-size: 14px; font-variant-numeric: tabular-nums; }
+.prose th { font-weight: 500; background: var(--surface); }
+.prose :is(th,td) { border-bottom: 1px solid var(--line); padding: 12px; vertical-align: top; min-width: 100px; }
+.prose tr > :first-child { padding-left: 10px; }
+.prose .callout, .prose blockquote { background: var(--accent-soft); padding: 16px 20px; border-radius: var(--radius); margin: 24px 0; border: 1px solid var(--line); font-size: 15px; }
+.prose .callout > :first-child { margin-top: 0; } .prose .callout > :last-child { margin-bottom: 0; }
+.prose .warning { background: var(--warning-bg); color: var(--warning-ink); }
+.source-link { display: inline-flex; gap: 6px; align-items: center; color: var(--muted); font-size: 12px; text-decoration: none; margin-top: 36px; }
+.source-link .icon { width: 14px; }
+.page-pagination { display: grid; grid-template-columns: 1fr 1fr; gap: 24px; border-top: 1px solid var(--line); margin-top: 24px; padding-top: 24px; }
+.page-pagination a { text-decoration: none; }
+.page-pagination span { display: block; font-size: 12px; color: var(--muted); margin-bottom: 6px; }
+.page-pagination strong { font-size: 14px; font-weight: 500; }
+.next-page { text-align: right; }.next-page .icon { width: 15px; margin-left: 5px; }
+.toc { font-size: 12px; }
+.toc nav > p { font-weight: 600; margin-bottom: 12px; }
+.toc nav a, .mobile-toc nav a { display: block; border-left: 1px solid var(--line); padding: 6px 0 6px 12px; color: var(--muted); text-decoration: none; line-height: 1.5; }
+.toc nav a[aria-current], .toc nav a:hover { color: var(--accent); border-color: var(--accent); }
+.toc nav .nested, .mobile-toc nav .nested { padding-left: 24px; }
+.mobile-docs-nav, .mobile-toc { display: none; }
+.architecture-diagram { border: 1px solid var(--line); border-radius: 8px; padding: 24px; margin-block: 24px; background: var(--surface); }
+.architecture-node { display: flex; align-items: center; justify-content: center; gap: 12px; border: 1px solid var(--line); padding: 16px; background: var(--bg); border-radius: var(--radius); font-size: 14px; }
+.architecture-node > .icon { color: var(--accent); }
+.architecture-node span { display: block; font-size: 12px; color: var(--muted); }
+.api-connector { text-align: center; padding: 14px; font-size: 12px; color: var(--muted); }
+.storage-row { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; margin-top: 12px; }
+.storage-row .architecture-node { flex-direction: column; gap: 6px; text-align: center; padding: 12px 6px; }
+.architecture-diagram figcaption { margin-top: 16px; font-size: 12px; text-align: center; color: var(--muted); }
+.not-found { padding-block: 100px; min-height: 65vh; } .not-found h1 { font-size: 42px; } .not-found p { margin: 20px 0; }
+.search-dialog { width: min(620px, calc(100% - 32px)); max-height: min(680px, calc(100dvh - 100px)); margin: 12vh auto auto; padding: 0; background: var(--bg); color: var(--ink); border: 0; border-radius: 12px; box-shadow: var(--search-shadow); overflow-y: auto; overscroll-behavior: contain; }
+.search-dialog::backdrop { background: #071a2470; }
+.search-heading { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px 8px; }
+.search-heading h2 { font-size: 16px; }
+.search-field { margin: 4px 20px 16px; display: flex; align-items: center; gap: 12px; border: 1px solid var(--line); border-radius: var(--radius); padding: 0 12px; }
+.search-field:focus-within { border-color: var(--accent); outline: 2px solid var(--accent-soft); }
+.search-field .icon { color: var(--muted); width: 18px; }
+.search-field input { width: 100%; min-width: 0; height: 46px; border: 0; outline: none; background: transparent; color: var(--ink); caret-color: var(--accent); font-size: 15px; }
+input::placeholder { color: var(--muted); opacity: 1; }
+.search-body { padding: 0 12px 12px; min-height: 100px; }
+.search-status { font-size: 12px; color: var(--muted); padding: 0 8px 8px; }
+.search-results { list-style: none; padding: 0; margin: 0; }
+.search-results a { display: block; padding: 12px 10px; border-radius: var(--radius); text-decoration: none; }
+.search-results a:hover, .search-results a:focus { background: var(--accent-soft); }
+.search-results strong { display: block; font-size: 15px; font-weight: 500; }
+.search-results p { font-size: 12px; color: var(--muted); margin-top: 4px; line-height: 1.6; }
+.search-help { display: flex; justify-content: space-between; gap: 12px; border-top: 1px solid var(--line); padding: 12px 20px; font-size: 11px; color: var(--muted); }
+@media (min-width: 1500px) { .hero { padding-block: 100px 84px; } }
+@media (max-width: 1250px) { .header-inner { gap: 28px; } .header-tools { gap: 14px; } .search-trigger kbd { display: none; } .docs-layout { grid-template-columns: 190px minmax(0,1fr) 160px; gap: 32px; } .configuration-section, .quickstart-section { gap: 64px; } }
+@media (max-width: 1100px) { .toc { display: none; } .docs-layout { grid-template-columns: 200px minmax(0,1fr); gap: 40px; } .docs-article { max-width: 720px; } .mobile-toc { display: block; margin: 22px 0 30px; border: 1px solid var(--line); border-radius: var(--radius); padding: 12px 16px; font-size: 13px; } .mobile-toc summary { min-height: 24px; display: flex; justify-content: space-between; align-items: center; } .mobile-toc nav { margin-top: 12px; } .search-trigger > span { display: none; } .search-trigger { border: 0; background: transparent; padding: 8px; } .hero { gap: 28px; } .hero h1 { font-size: 40px; } .hero-copy > p { font-size: 16px; } .hero-actions { gap: 8px; } .hero-actions .button { padding-inline: 12px; font-size: 14px; } .product-preview figcaption { display: block; } .product-preview figcaption a { display: inline-block; margin-left: 8px; } }
+@media (max-width: 850px) { .primary-nav { display: none; } .mobile-site-nav { display: block; } .header-inner { padding-inline: 24px; height: 64px; } .container { width: calc(100% - 48px); } .hero { grid-template-columns: 1fr; gap: 36px; padding-block: 48px; } .hero h1 { font-size: 46px; } .hero-copy > p { max-width: 52ch; font-size: 18px; } .product-preview { max-width: 680px; } .product-preview figcaption { display: flex; } .configuration-section, .quickstart-section { gap: 40px; padding-block: 48px; } .configuration-section h2, .quickstart-section h2 { font-size: 29px; } .workflow-links { gap: 28px; } .workflow-links h3 { font-size: 17px; } .architecture-teaser { align-items: start; } .architecture-teaser .text-link { max-width: 180px; } .docs-layout { display: block; padding-inline: 24px; } .docs-sidebar { display: none; } .docs-article { margin: auto; padding-top: 28px; } .mobile-docs-nav { display: block; border-bottom: 1px solid var(--line); padding-block: 13px; } .mobile-docs-nav > summary { min-height: 28px; display: flex; justify-content: space-between; font-size: 14px; align-items: center; } .mobile-docs-nav .docs-nav { padding-block: 20px 10px; display: grid; grid-template-columns: repeat(3,1fr); gap: 20px; } .mobile-docs-nav .nav-group { margin: 0; } .docs-article h1 { font-size: 36px; } .docs-intro { font-size: 17px; } }
+@media (max-width: 600px) { .header-inner { padding-inline: 20px; gap: 12px; } .brand { font-size: 21px; gap: 8px; } .brand-icon { width: 23px; height: 23px; } .header-tools { gap: 8px; } .github-link { display: none; } .icon-button { width: 32px; } .container { width: calc(100% - 40px); } .hero { padding-block: 40px; gap: 32px; } .hero h1 { font-size: 39px; } .hero-copy > p { font-size: 17px; } .hero-note { max-width: none !important; } .product-preview figcaption { font-size: 10px; gap: 4px; flex-wrap: wrap; } .product-preview figcaption a { margin: 0; } .configuration-section, .quickstart-section { grid-template-columns: 1fr; gap: 28px; padding-block: 36px; } .configuration-section h2, .quickstart-section h2 { font-size: 30px; } .workflow-section { padding-block: 16px 36px; } .section-heading h2 { font-size: 28px; } .workflow-links { grid-template-columns: 1fr; margin-top: 24px; gap: 22px; } .workflow-links > a { position: relative; padding-left: 40px; padding-top: 20px; } .workflow-links > a > .icon { position: absolute; left: 0; top: 20px; width: 22px; } .workflow-links p { max-width: none; margin-top: 8px; } .architecture-teaser { flex-direction: column; gap: 16px; } .architecture-teaser .text-link { max-width: none; } .site-footer { flex-direction: column; align-items: start; gap: 18px; padding-block: 24px; } .site-footer > div { gap: 16px; } .docs-layout { padding-inline: 20px; } .docs-article h1 { font-size: 33px; } .prose { font-size: 16px; } .prose h2 { font-size: 23px; } .prose pre { font-size: 12px; padding: 16px; } .mobile-docs-nav .docs-nav { grid-template-columns: 1fr; gap: 24px; } .mobile-docs-nav .docs-nav a { min-height: 40px; } .storage-row { grid-template-columns: 1fr; } .storage-row .architecture-node { flex-direction: row; text-align: left; justify-content: start; padding: 12px; } .architecture-diagram { padding: 16px; } .search-dialog { margin-top: 24px; max-height: calc(100dvh - 48px); } .search-help { font-size: 10px; padding-inline: 16px; } .search-field { margin-inline: 16px; } }
+@media (prefers-reduced-motion: reduce) { *, *::before, *::after { scroll-behavior: auto !important; transition: none !important; animation: none !important; } }

# website/tests/site.test.mjs

diff --git a/website/tests/site.test.mjs b/website/tests/site.test.mjs
index 3ca4b97..aebd689 100644
--- a/website/tests/site.test.mjs
+++ b/website/tests/site.test.mjs
@@ -2,6 +2,7 @@ import assert from 'node:assert/strict';
 import { readFileSync, readdirSync, statSync } from 'node:fs';
 import { resolve, sep } from 'node:path';
 import { test } from 'node:test';
+import { searchEntries, excerpt } from '../src/scripts/search.mjs';
 
 const dist = resolve('dist');
 const files = readdirSync(dist, { recursive: true }).filter((file) => statSync(resolve(dist, file)).isFile());
@@ -10,7 +11,7 @@ const pages = new Map(files.filter((file) => file.endsWith('.html')).map((file)
 ]));
 
 test('all requested pages have a title, description, main landmark, and one heading', () => {
-  for (const route of ['', 'architecture/', 'docs/', 'docs/installation/', 'docs/configuration/', 'docs/services/', 'docs/api/']) {
+  for (const route of ['', 'architecture/', 'docs/', ...['installation', 'configuration', 'services', 'api', 'workspaces', 'docker', 'traffic', 'troubleshooting', 'contributing'].map((guide) => `docs/${guide}/`)]) {
     assert.ok(pages.has(`/${route}index.html`), `Missing page: /${route}`);
   }
   assert.ok(pages.has('/404.html'));
@@ -43,9 +44,39 @@ test('every internal link, fragment, and asset resolves in the static build', ()
   assert.ok(checked > 100, 'Expected navigation and table-of-contents links to be rendered');
 });
 
-test('the website ships no JavaScript and stays below a 256 KiB total asset budget', () => {
-  for (const [path, html] of pages) assert.doesNotMatch(html, /<script\b/i, path);
-  assert.ok(!files.some((file) => /\.(?:m?js)$/.test(file)));
+test('the static site stays below 1 MiB with less than 16 KiB of client JavaScript', () => {
+  const scripts = files.filter((file) => /\.(?:m?js)$/.test(file));
+  assert.ok(scripts.length > 0);
+  assert.ok(scripts.reduce((total, file) => total + statSync(resolve(dist, file)).size, 0) < 16 * 1024);
+  for (const [path, html] of pages) assert.doesNotMatch(html, /<script[^>]+src="https?:/i, path);
   const bytes = files.reduce((total, file) => total + statSync(resolve(dist, file)).size, 0);
-  assert.ok(bytes < 256 * 1024, `Static output is ${bytes} bytes`);
+  assert.ok(bytes < 1024 * 1024, `Static output is ${bytes} bytes`);
+});
+
+const index = JSON.parse(readFileSync(resolve(dist, 'search-index.json'), 'utf8'));
+
+test('search indexes every guide and all result links resolve to real sections', () => {
+  assert.ok(index.length > 35);
+  for (const entry of index) {
+    const [path, anchor] = entry.href.split('#');
+    const html = pages.get(`${path}index.html`);
+    assert.ok(html, entry.href);
+    if (anchor) assert.ok(html.includes(`id="${anchor}"`), entry.href);
+    assert.ok(entry.title && entry.text, entry.href);
+    assert.doesNotMatch(entry.text, /<\/?(?:span|p|code|h[1-6])\b/);
+  }
+  for (const path of pages.keys()) {
+    if (path.startsWith('/docs/')) assert.ok(index.some((entry) => `${entry.href}index.html` === path), path);
+  }
+});
+
+test('search ranks titles, matches body text and multiple terms, and handles empty results', () => {
+  assert.equal(searchEntries(index, 'workspace configuration')[0].href, '/docs/configuration/');
+  assert.ok(searchEntries(index, 'ConPTY').some((entry) => entry.href.includes('terminals')));
+  assert.ok(searchEntries(index, 'compose override').some((entry) => entry.href.startsWith('/docs/docker/')));
+  assert.equal(searchEntries(index, 'xyz-nonexistent').length, 0);
+  assert.equal(searchEntries(index, '<script>alert(1)</script>').length, 0);
+  assert.equal(searchEntries(index, '   ').length, 5);
+  assert.ok(searchEntries(index, 'DOCKER').length > 0);
+  assert.ok(excerpt('a'.repeat(200) + ' tmux is required', 'tmux').includes('tmux'));
 });

# desktop/src/DockerError.test.tsx

diff --git a/desktop/src/DockerError.test.tsx b/desktop/src/DockerError.test.tsx
new file mode 100644
index 0000000..b4c0063
--- /dev/null
+++ b/desktop/src/DockerError.test.tsx
@@ -0,0 +1,31 @@
+// @vitest-environment jsdom
+import { afterEach, expect, it } from "vitest";
+import { cleanup, fireEvent, render, screen } from "@testing-library/react";
+import DockerError from "./DockerError";
+
+afterEach(cleanup);
+
+it("summarizes and deduplicates missing Compose variables while preserving diagnostics", () => {
+  const diagnostic = "docker compose --project-directory /tmp/project -f /tmp/project/compose.yaml config --format json: " +
+    ["MINIO_ROOT_PASSWORD", "MINIO_ROOT_USER", "MINIO_ROOT_PASSWORD", "MINIO_ROOT_USER"].map((name) =>
+      `error while interpolating services.minio.environment.${name}: required variable ${name} is missing a value: set ${name} in compose.env`
+    ).join("\n") + ": exit status 1";
+  render(<DockerError error={diagnostic} />);
+  expect(screen.getByRole("alert").textContent).toContain("Compose configuration is incomplete");
+  expect(screen.getAllByRole("listitem").map((item) => item.textContent)).toEqual(["MINIO_ROOT_PASSWORD", "MINIO_ROOT_USER"]);
+  expect(screen.getByText(/Provide non-empty values/)).toBeTruthy();
+  const details = screen.getByText("Technical details").closest("details")!;
+  expect(details.open).toBe(false);
+  expect(details.querySelector("pre")?.textContent).toBe(diagnostic);
+  fireEvent.click(screen.getByText("Technical details"));
+  expect(details.open).toBe(true);
+});
+
+it("keeps unrecognized errors and line breaks visible without guessing a cause", () => {
+  const diagnostic = "Cannot connect to the Docker daemon.\nCheck the socket permissions.";
+  render(<DockerError error={diagnostic} title="Could not load Compose services" />);
+  expect(screen.getByText("Could not load Compose services")).toBeTruthy();
+  expect(screen.getByRole("alert").querySelector("pre")?.textContent).toBe(diagnostic);
+  expect(screen.queryByRole("list")).toBeNull();
+  expect(screen.queryByText("Technical details")).toBeNull();
+});

# desktop/src/DockerError.tsx

diff --git a/desktop/src/DockerError.tsx b/desktop/src/DockerError.tsx
new file mode 100644
index 0000000..941cbe4
--- /dev/null
+++ b/desktop/src/DockerError.tsx
@@ -0,0 +1,12 @@
+export default function DockerError({ error, title = "Docker operation failed" }: { error: string; title?: string }) {
+  const variables = [...new Set(Array.from(error.matchAll(/required variable ([A-Za-z_][A-Za-z0-9_]*) is missing a value/g), (match) => match[1]))];
+  return <div className="docker-error" role="alert">
+    <strong>{variables.length ? "Compose configuration is incomplete" : title}</strong>
+    {variables.length ? <>
+      <p>Docker Compose cannot continue because these required environment variables are missing or empty:</p>
+      <ul>{variables.map((name) => <li key={name}><code>{name}</code></li>)}</ul>
+      <p>Provide non-empty values in the environment used by Docker Compose, then refresh or retry the operation.</p>
+      <details><summary>Technical details</summary><pre>{error}</pre></details>
+    </> : <pre>{error}</pre>}
+  </div>;
+}

# server/internal/app/shellpath.go

diff --git a/server/internal/app/shellpath.go b/server/internal/app/shellpath.go
new file mode 100644
index 0000000..e27975d
--- /dev/null
+++ b/server/internal/app/shellpath.go
@@ -0,0 +1,39 @@
+package app
+
+import (
+	"context"
+	"errors"
+	"os"
+	"os/exec"
+	"strings"
+	"time"
+)
+
+// Desktop launchers do not read shell startup files where SDKs extend PATH.
+func loadShellPath(ctx context.Context) error {
+	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
+	defer cancel()
+	shell := os.Getenv("SHELL")
+	if shell == "" {
+		shell = "/bin/sh"
+	}
+	home, err := os.UserHomeDir()
+	if err != nil {
+		return err
+	}
+	cmd := exec.CommandContext(ctx, shell, "-ilc", `printf '\000DEVORBIT_PATH\000%s\000' "$PATH"`)
+	cmd.Dir = home
+	cmd.WaitDelay = 100 * time.Millisecond
+	output, err := cmd.Output()
+	if err != nil {
+		return err
+	}
+	// Startup banners must not become part of PATH or backend readiness output.
+	const marker = "\x00DEVORBIT_PATH\x00"
+	_, value, found := strings.Cut(string(output), marker)
+	path, _, terminated := strings.Cut(value, "\x00")
+	if !found || !terminated || path == "" {
+		return errors.New("shell PATH unavailable")
+	}
+	return os.Setenv("PATH", path)
+}

# server/internal/app/shellpath_test.go

diff --git a/server/internal/app/shellpath_test.go b/server/internal/app/shellpath_test.go
new file mode 100644
index 0000000..00e0695
--- /dev/null
+++ b/server/internal/app/shellpath_test.go
@@ -0,0 +1,68 @@
+package app
+
+import (
+	"context"
+	"os"
+	"os/exec"
+	"path/filepath"
+	"runtime"
+	"testing"
+	"time"
+)
+
+func TestLoadShellPath(t *testing.T) {
+	if runtime.GOOS == "windows" {
+		t.Skip("Unix shell startup")
+	}
+	for _, tc := range []struct {
+		name, script string
+		fail         bool
+	}{
+		{"startup PATH", `
+[ "$1" = "-ilc" ] || exit 1
+printf 'startup banner\n'
+export PATH="$HOME/sdk bin:$PATH"
+/bin/sh -c "$2"
+printf 'logout banner\n'
+`, false},
+		{"failed shell", "exit 1", true},
+		{"missing marker", "printf 'banner only'", true},
+		{"empty PATH", `printf '\000DEVORBIT_PATH\000\000'`, true},
+		{"unterminated PATH", `printf '\000DEVORBIT_PATH\000/bad'`, true},
+		{"hung shell", "exec /bin/sleep 10", true},
+	} {
+		t.Run(tc.name, func(t *testing.T) {
+			home := t.TempDir()
+			sdk := filepath.Join(home, "sdk bin")
+			if err := os.Mkdir(sdk, 0700); err != nil {
+				t.Fatal(err)
+			}
+			if err := os.WriteFile(filepath.Join(sdk, "flutter"), []byte("#!/bin/sh\nprintf 'flutter found'\n"), 0700); err != nil {
+				t.Fatal(err)
+			}
+			shell := filepath.Join(home, "shell")
+			if err := os.WriteFile(shell, []byte("#!/bin/sh\n"+tc.script), 0700); err != nil {
+				t.Fatal(err)
+			}
+			t.Setenv("HOME", home)
+			t.Setenv("SHELL", shell)
+			t.Setenv("PATH", "/usr/bin:/bin")
+			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
+			defer cancel()
+			err := loadShellPath(ctx)
+			if tc.fail {
+				if err == nil || os.Getenv("PATH") != "/usr/bin:/bin" {
+					t.Fatalf("failed shell changed PATH: %v", err)
+				}
+				return
+			}
+			if err != nil {
+				t.Fatal(err)
+			}
+			output, err := exec.Command("flutter", "run", "-d", "linux").Output()
+			if err != nil || string(output) != "flutter found" {
+				t.Fatalf("SDK command lookup failed: %q, %v", output, err)
+			}
+		})
+	}
+}

# website/PRODUCT.md

diff --git a/website/PRODUCT.md b/website/PRODUCT.md
new file mode 100644
index 0000000..aca3f71
--- /dev/null
+++ b/website/PRODUCT.md
@@ -0,0 +1,9 @@
+# DevOrbit website
+
+DevOrbit is a local development desktop app. Its website helps developers understand the product, install or build it, and find accurate guides while using it. The implementation is a static Astro site; the React/Tauri desktop and Go backend are separate products in this repository.
+
+The homepage explains the workspace and shows the actual desktop interface using labeled example data. Documentation supports installation, workspaces, YAML configuration, services and terminals, Docker Compose, HTTP traffic inspection, troubleshooting, architecture, contribution, and the local API.
+
+The website preserves the teal DevOrbit identity. Content takes priority over decoration. No invented release availability, usage statistics, automatic dependency orchestration, sandboxing, or network interception claims. Product behavior comes from the repository documentation and implementation. Existing guide routes and heading anchors remain stable.
+
+Primary tasks: understand the app, get started, search a technical concept, copy a command, navigate a guide, and inspect the architecture. Documentation must remain readable on phones and wide desktop displays, in light and dark themes, and through keyboard navigation.

# website/public/fonts/OFL.txt

diff --git a/website/public/fonts/OFL.txt b/website/public/fonts/OFL.txt
new file mode 100644
index 0000000..c35c4c6
--- /dev/null
+++ b/website/public/fonts/OFL.txt
@@ -0,0 +1,93 @@
+Copyright © 2017 IBM Corp. with Reserved Font Name "Plex"
+
+This Font Software is licensed under the SIL Open Font License, Version 1.1.
+
+This license is copied below, and is also available with a FAQ at: http://scripts.sil.org/OFL
+
+
+-----------------------------------------------------------
+SIL OPEN FONT LICENSE Version 1.1 - 26 February 2007
+-----------------------------------------------------------
+
+PREAMBLE
+The goals of the Open Font License (OFL) are to stimulate worldwide
+development of collaborative font projects, to support the font creation
+efforts of academic and linguistic communities, and to provide a free and
+open framework in which fonts may be shared and improved in partnership
+with others.
+
+The OFL allows the licensed fonts to be used, studied, modified and
+redistributed freely as long as they are not sold by themselves. The
+fonts, including any derivative works, can be bundled, embedded, 
+redistributed and/or sold with any software provided that any reserved
+names are not used by derivative works. The fonts and derivatives,
+however, cannot be released under any other type of license. The
+requirement for fonts to remain under this license does not apply
+to any document created using the fonts or their derivatives.
+
+DEFINITIONS
+"Font Software" refers to the set of files released by the Copyright
+Holder(s) under this license and clearly marked as such. This may
+include source files, build scripts and documentation.
+
+"Reserved Font Name" refers to any names specified as such after the
+copyright statement(s).
+
+"Original Version" refers to the collection of Font Software components as
+distributed by the Copyright Holder(s).
+
+"Modified Version" refers to any derivative made by adding to, deleting,
+or substituting -- in part or in whole -- any of the components of the
+Original Version, by changing formats or by porting the Font Software to a
+new environment.
+
+"Author" refers to any designer, engineer, programmer, technical
+writer or other person who contributed to the Font Software.
+
+PERMISSION & CONDITIONS
+Permission is hereby granted, free of charge, to any person obtaining
+a copy of the Font Software, to use, study, copy, merge, embed, modify,
+redistribute, and sell modified and unmodified copies of the Font
+Software, subject to the following conditions:
+
+1) Neither the Font Software nor any of its individual components,
+in Original or Modified Versions, may be sold by itself.
+
+2) Original or Modified Versions of the Font Software may be bundled,
+redistributed and/or sold with any software, provided that each copy
+contains the above copyright notice and this license. These can be
+included either as stand-alone text files, human-readable headers or
+in the appropriate machine-readable metadata fields within text or
+binary files as long as those fields can be easily viewed by the user.
+
+3) No Modified Version of the Font Software may use the Reserved Font
+Name(s) unless explicit written permission is granted by the corresponding
+Copyright Holder. This restriction only applies to the primary font name as
+presented to the users.
+
+4) The name(s) of the Copyright Holder(s) or the Author(s) of the Font
+Software shall not be used to promote, endorse or advertise any
+Modified Version, except to acknowledge the contribution(s) of the
+Copyright Holder(s) and the Author(s) or with their explicit written
+permission.
+
+5) The Font Software, modified or unmodified, in part or in whole,
+must be distributed entirely under this license, and must not be
+distributed under any other license. The requirement for fonts to
+remain under this license does not apply to any document created
+using the Font Software.
+
+TERMINATION
+This license becomes null and void if any of the above conditions are
+not met.
+
+DISCLAIMER
+THE FONT SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
+EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO ANY WARRANTIES OF
+MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT
+OF COPYRIGHT, PATENT, TRADEMARK, OR OTHER RIGHT. IN NO EVENT SHALL THE
+COPYRIGHT HOLDER BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
+INCLUDING ANY GENERAL, SPECIAL, INDIRECT, INCIDENTAL, OR CONSEQUENTIAL
+DAMAGES, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
+FROM, OUT OF THE USE OR INABILITY TO USE THE FONT SOFTWARE OR FROM
+OTHER DEALINGS IN THE FONT SOFTWARE.

# website/public/fonts/ibm-plex-sans.ttf

diff --git a/website/public/fonts/ibm-plex-sans.ttf b/website/public/fonts/ibm-plex-sans.ttf
new file mode 100644
index 0000000..30f837e
Binary files /dev/null and b/website/public/fonts/ibm-plex-sans.ttf differ

# website/public/images/workspace.webp

diff --git a/website/public/images/workspace.webp b/website/public/images/workspace.webp
new file mode 100644
index 0000000..7852f64
Binary files /dev/null and b/website/public/images/workspace.webp differ

# website/src/components/CodeBlock.astro

diff --git a/website/src/components/CodeBlock.astro b/website/src/components/CodeBlock.astro
new file mode 100644
index 0000000..4e05362
--- /dev/null
+++ b/website/src/components/CodeBlock.astro
@@ -0,0 +1,8 @@
+---
+import { Code } from 'astro:components';
+interface Props { code: string; lang?: string; title: string; }
+const { code, lang = 'sh', title } = Astro.props;
+---
+<div class="code-example" data-code-title={title}>
+  <Code code={code} lang={lang} theme="github-dark" />
+</div>

# website/src/components/DocsNav.astro

diff --git a/website/src/components/DocsNav.astro b/website/src/components/DocsNav.astro
new file mode 100644
index 0000000..899b04b
--- /dev/null
+++ b/website/src/components/DocsNav.astro
@@ -0,0 +1,9 @@
+---
+import { guideGroups } from '../data/navigation';
+---
+<nav class="docs-nav" aria-label="Documentation">
+  {guideGroups.map(({ label, links }) => <div class="nav-group">
+    <p>{label}</p>
+    {links.map(({ href, label }) => <a href={href} aria-current={href === Astro.url.pathname ? 'page' : undefined}>{label}</a>)}
+  </div>)}
+</nav>

# website/src/data/navigation.ts

diff --git a/website/src/data/navigation.ts b/website/src/data/navigation.ts
new file mode 100644
index 0000000..5172967
--- /dev/null
+++ b/website/src/data/navigation.ts
@@ -0,0 +1,23 @@
+export const repo = 'https://github.com/prasannashrestha011/devOrbit';
+
+export const guideGroups = [
+  { label: 'Get started', links: [
+    { href: '/docs/', label: 'Introduction' },
+    { href: '/docs/installation/', label: 'Installation' },
+  ] },
+  { label: 'Use DevOrbit', links: [
+    { href: '/docs/workspaces/', label: 'Workspaces' },
+    { href: '/docs/configuration/', label: 'Configuration' },
+    { href: '/docs/services/', label: 'Services & terminals' },
+    { href: '/docs/docker/', label: 'Docker integration' },
+    { href: '/docs/traffic/', label: 'Traffic inspector' },
+    { href: '/docs/troubleshooting/', label: 'Troubleshooting' },
+  ] },
+  { label: 'Go deeper', links: [
+    { href: '/architecture/', label: 'Architecture' },
+    { href: '/docs/api/', label: 'API reference' },
+    { href: '/docs/contributing/', label: 'Contributing' },
+  ] },
+];
+
+export const guides = guideGroups.flatMap(({ links }) => links);

# website/src/pages/docs/contributing.md

diff --git a/website/src/pages/docs/contributing.md b/website/src/pages/docs/contributing.md
new file mode 100644
index 0000000..aa329d2
--- /dev/null
+++ b/website/src/pages/docs/contributing.md
@@ -0,0 +1,55 @@
+---
+layout: ../../layouts/Docs.astro
+label: Contributing
+title: Contributing
+description: Find your way around the repository, run the relevant checks, and propose a focused change.
+source: docs/ci-cd.md
+---
+
+## Set up the project
+
+Start with the [source setup guide](/docs/#before-you-start), then run DevOrbit locally. Check existing [issues](https://github.com/prasannashrestha011/devOrbit/issues) and [pull requests](https://github.com/prasannashrestha011/devOrbit/pulls) before starting a change.
+
+## Find the right area
+
+| Directory | Responsibility |
+| --- | --- |
+| `desktop/src` | React desktop interface |
+| `desktop/src-tauri` | Rust host, native windows, backend lifecycle, terminal bridge |
+| `server` | Go API, process supervision, workspace configuration, local storage |
+| `website` | Astro homepage and documentation |
+| `docs` | Installation, release, and architecture reference documents |
+
+Read the [architecture overview](/architecture/) for the boundaries between the desktop and backend.
+
+## Validate your change
+
+Run checks from the directory you changed. Backend tests need the development prerequisites, including a C compiler and tmux on Linux or macOS.
+
+```sh
+# From server/
+go test -race ./...
+go vet ./...
+go build ./...
+```
+
+```sh
+# From desktop/
+npm ci
+npm test
+npm run build
+```
+
+```sh
+# From website/
+npm ci
+npm test
+```
+
+Desktop host changes also need the Rust checks described in the [CI workflow](https://github.com/prasannashrestha011/devOrbit/blob/main/.github/workflows/ci.yml). For website changes, inspect the rendered pages at desktop and mobile widths and exercise keyboard navigation.
+
+## Open a pull request
+
+Explain the problem, the resulting behavior, and how you checked it. Include screenshots for visible interface changes and update affected documentation. Keep unrelated changes separate so the review can focus on one problem.
+
+See [CI/CD and releases](https://github.com/prasannashrestha011/devOrbit/blob/main/docs/ci-cd.md) for packaging and release checks.

# website/src/pages/docs/docker.md

diff --git a/website/src/pages/docs/docker.md b/website/src/pages/docs/docker.md
new file mode 100644
index 0000000..aeb7cc1
--- /dev/null
+++ b/website/src/pages/docs/docker.md
@@ -0,0 +1,36 @@
+---
+layout: ../../layouts/Docs.astro
+label: Docker integration
+title: Docker integration
+description: Discover Docker Compose projects and manage their services from your workspace.
+source: server/internal/modules/dockercompose/manager.go
+---
+
+## Connect a Compose project
+
+Install Docker with Compose v2 and start the Docker daemon. DevOrbit itself runs natively; Docker is needed only for Compose projects.
+
+DevOrbit looks for `compose.yaml`, `compose.yml`, `docker-compose.yaml`, or `docker-compose.yml` in the workspace root and configured service working directories. Matching Compose override files are included. When several configurations are found, choose one in the **Compose file** selector.
+
+For a project outside the workspace root, configure a service's `cwd` to point to that project directory. See [workspaces](/docs/workspaces/#connect-an-existing-project).
+
+## Manage containers
+
+The Docker panel shows defined services, container states, published ports, and health information when available.
+
+| Action | What it does |
+| --- | --- |
+| Up | Creates or recreates containers and may build images |
+| Start stopped | Starts existing stopped containers |
+| Stop | Stops containers without removing them |
+| Restart | Restarts running containers |
+| Build images | Builds services that define a build configuration |
+| Down | Removes the project's containers and networks, keeping volumes |
+
+**Down** asks for confirmation. Check the selected project before confirming. Operations report their progress and errors in the panel; completion of a command does not guarantee that a container is healthy.
+
+## Inspect a service
+
+Open **Logs** or expand a service's details to inspect its containers. Check the reported state and health when a container is restarting or unhealthy, then inspect its logs for the cause.
+
+Use **Refresh** after changing Compose configuration or Docker availability. If the panel has no project to show, check the file location and the configured working directories. See [Docker troubleshooting](/docs/troubleshooting/#docker-is-unavailable).

# website/src/pages/docs/traffic.md

diff --git a/website/src/pages/docs/traffic.md b/website/src/pages/docs/traffic.md
new file mode 100644
index 0000000..f20bd20
--- /dev/null
+++ b/website/src/pages/docs/traffic.md
@@ -0,0 +1,29 @@
+---
+layout: ../../layouts/Docs.astro
+label: Traffic inspector
+title: Traffic inspector
+description: Inspect local HTTP requests and responses through a temporary gateway.
+source: server/README.md
+---
+
+## Start an inspection session
+
+Start your service and open its traffic inspector. DevOrbit detects listening TCP ports from the process and its children. Choose the HTTP port when more than one is available; DevOrbit checks that it responds to HTTP before opening an inspection gateway.
+
+Send requests to the gateway URL shown in the inspector. Requests sent directly to the original service port bypass inspection.
+
+## Read an exchange
+
+The inspector records the request path, query, headers, cookies, and body, along with the forwarded response's status, headers, trailers, and body. Use the exchange details to investigate what your client sent and what your service returned.
+
+The detail API returns body previews up to 256 KiB. Body download endpoints provide all retained bytes. HTTPS interception and WebSocket frame capture are not supported.
+
+## Manage captured data
+
+Captures are stored privately under `DEVORBIT_DATA_DIR/traffic`. History survives disabling inspection and restarting DevOrbit. Clear it in the inspector when you no longer need it.
+
+Each session retains its newest 500 exchanges for up to seven days. All sessions share a 512 MiB storage quota. When that quota is reached, forwarding continues, but a capture may contain only part of a body.
+
+<aside class="callout">Requests and responses can contain credentials or other secrets. Keep the data directory private and clear sensitive captures when you finish.</aside>
+
+If no traffic appears, check that the service is running and your client is using the gateway URL. See [troubleshooting](/docs/troubleshooting/#the-inspector-is-empty).

# website/src/pages/docs/troubleshooting.md

diff --git a/website/src/pages/docs/troubleshooting.md b/website/src/pages/docs/troubleshooting.md
new file mode 100644
index 0000000..cbcc2bc
--- /dev/null
+++ b/website/src/pages/docs/troubleshooting.md
@@ -0,0 +1,47 @@
+---
+layout: ../../layouts/Docs.astro
+label: Troubleshooting
+title: Troubleshooting
+description: Resolve connection, service, terminal, and inspection problems in your local workspace.
+source: server/README.md
+---
+
+## The desktop cannot connect
+
+Reopen DevOrbit to restart its bundled backend. If startup fails, read the error in the startup dialog. The default health endpoint is `http://127.0.0.1:7456/health`.
+
+### Check connection settings
+
+Export custom host, port, data directory, or token settings before opening the desktop. The desktop passes these settings to its backend. A port already in use or an inaccessible data directory can prevent startup. Review the [backend settings](/docs/configuration/#backend-settings).
+
+### Check authentication
+
+The health endpoint is public; `/api/v1` requests require a bearer token. The desktop's Rust host supplies it automatically. For direct API calls, read the configured token file locally and pass the token as described in [API authentication](/docs/api/#connect-and-authenticate).
+
+## A service fails to start
+
+Check its command, working directory, installed dependencies, and terminal output. Commands run as executable arguments: pipes, shell expansion, and `&&` are not interpreted automatically. Use an explicit shell when your command needs shell syntax.
+
+Relative working directories must remain inside the workspace root. Use an absolute `cwd` for a project elsewhere. After saving a configuration change, restart the service to apply it. See [workspace configuration](/docs/configuration/).
+
+A **running** state confirms that the process launched, not that the application is ready. DevOrbit does not automatically restart failed processes or resolve startup dependencies.
+
+## A terminal will not open
+
+Linux and macOS service terminals require `tmux`. Windows uses ConPTY. For workspace shell terminals, the limit is 16 sessions, including exited sessions that have not been closed; close an unused session before creating another.
+
+Closing a terminal viewer detaches it without stopping the service. Reopen it to reconnect. See [interactive terminals](/docs/services/#use-interactive-terminals).
+
+## Docker is unavailable
+
+Confirm that Docker is installed, the daemon is running, and Compose v2 is available to the user running DevOrbit. Check that a supported Compose file is in the workspace root or a configured service working directory, then use **Refresh** in the Docker panel.
+
+If a container keeps restarting, open its logs and inspect its health status. See [Docker integration](/docs/docker/).
+
+## The inspector is empty
+
+Start the service, select its HTTP port, and enable inspection. Send requests to the temporary gateway URL shown in DevOrbit; direct calls to the service are not captured. HTTPS interception and WebSocket frame capture are not supported.
+
+## Report a reproducible issue
+
+Include your operating system, DevOrbit version or source revision, steps to reproduce, and relevant error text in a [GitHub issue](https://github.com/prasannashrestha011/devOrbit/issues). Remove tokens, credentials, private paths, and sensitive request bodies from shared logs or captures.

# website/src/pages/docs/workspaces.md

diff --git a/website/src/pages/docs/workspaces.md b/website/src/pages/docs/workspaces.md
new file mode 100644
index 0000000..fda1132
--- /dev/null
+++ b/website/src/pages/docs/workspaces.md
@@ -0,0 +1,36 @@
+---
+layout: ../../layouts/Docs.astro
+label: Workspaces
+title: Workspaces
+description: Group the services for a project and keep their configuration in one place.
+source: server/internal/modules/workspace/service/service.go
+---
+
+## Create a workspace
+
+Create a workspace from the desktop and give it a name and description. DevOrbit creates a managed root under `DEVORBIT_DATA_DIR/workspaces` and initializes `.devorbit/workspace.yaml` there.
+
+The desktop shows the workspace configuration path. Use that root when resolving relative working directories; creating a workspace does not clone or move an existing project.
+
+## Connect an existing project
+
+Add a service through the service form or **Workspace YAML** editor. Set `cwd` to the absolute path of your existing project when it lives outside the managed workspace root.
+
+```yaml
+services:
+  web:
+    command: npm run dev
+    cwd: /home/you/projects/web
+```
+
+Replace the example directory with your own project path. On Windows, a YAML single-quoted path such as `'C:\Users\you\projects\web'` keeps backslashes literal. Install the project's dependencies before starting the service.
+
+For projects inside the workspace root, use a relative directory. Read [commands and directories](/docs/configuration/#commands-and-directories) for validation rules.
+
+## Organize your workflow
+
+Select a workspace in the sidebar to see its services. Use workspace controls to start, stop, or restart them together, or control one service at a time. A running process is not necessarily ready to receive requests.
+
+Open **Terminals** for independent shell sessions, or open a service terminal to interact with its current execution. **Quick controls** provides an always-on-top window that follows the selected workspace.
+
+Continue with [workspace configuration](/docs/configuration/) to edit YAML, or [services and terminals](/docs/services/) for process behavior.

# website/src/pages/search-index.json.ts

diff --git a/website/src/pages/search-index.json.ts b/website/src/pages/search-index.json.ts
new file mode 100644
index 0000000..e3a4e3c
--- /dev/null
+++ b/website/src/pages/search-index.json.ts
@@ -0,0 +1,23 @@
+import type { APIRoute, MarkdownInstance } from 'astro';
+import { guides } from '../data/navigation';
+
+const documents = import.meta.glob<MarkdownInstance<{ title: string; label: string; description: string }>>('./docs/*.md', { eager: true });
+const plainText = (html: string) => html.replace(/<[^>]*>/g, ' ').replace(/&(?:amp|lt|gt|quot|#39);/g, (entity) => ({ '&amp;': '&', '&lt;': '<', '&gt;': '>', '&quot;': '"', '&#39;': "'" })[entity]!).replace(/\s+/g, ' ').trim();
+
+export const GET: APIRoute = async () => {
+  const entries = (await Promise.all(guides.map(async ({ href }) => {
+    const doc = Object.values(documents).find((doc) => doc.url === href || `${doc.url}/` === href);
+    if (!doc) return [];
+    const sections = (await doc.compiledContent()).split(/(?=<h[23]\b)/);
+    return [
+      { href, title: doc.frontmatter.title, text: doc.frontmatter.description },
+      ...sections.flatMap((section) => {
+        const heading = section.match(/^<h[23]\b[^>]*id="([^"]+)"[^>]*>(.*?)<\/h[23]>/);
+        if (!heading) return [];
+        return [{ href: `${href}#${heading[1]}`, title: `${plainText(heading[2])} · ${doc.frontmatter.label}`, text: plainText(section.slice(heading[0].length)) }];
+      }),
+    ];
+  }))).flat();
+  entries.push({ href: '/architecture/', title: 'Architecture', text: 'React and Tauri desktop, authenticated local API, Go backend, process supervisor, workspace YAML, SQLite metadata, local logs. The desktop starts its bundled backend automatically.' });
+  return new Response(JSON.stringify(entries), { headers: { 'Content-Type': 'application/json' } });
+};

# website/src/scripts/search.mjs

diff --git a/website/src/scripts/search.mjs b/website/src/scripts/search.mjs
new file mode 100644
index 0000000..87f8358
--- /dev/null
+++ b/website/src/scripts/search.mjs
@@ -0,0 +1,21 @@
+const normalize = (value) => value.toLocaleLowerCase().normalize('NFKC');
+
+export function searchEntries(entries, query) {
+  const terms = normalize(query).trim().split(/\s+/).filter(Boolean);
+  if (!terms.length) return entries.filter((entry) => !entry.href.includes('#')).slice(0, 5);
+  return entries.map((entry) => {
+    const title = normalize(entry.title);
+    const text = normalize(entry.text);
+    const haystack = `${title} ${text}`;
+    if (!terms.every((term) => haystack.includes(term))) return null;
+    const score = terms.reduce((total, term) => total + (title.includes(term) ? 10 : 1), 0) + (title === normalize(query.trim()) ? 30 : 0);
+    return { ...entry, score };
+  }).filter(Boolean).sort((a, b) => b.score - a.score).slice(0, 8);
+}
+
+export function excerpt(text, query) {
+  const terms = normalize(query).trim().split(/\s+/).filter(Boolean);
+  const position = terms.map((term) => normalize(text).indexOf(term)).filter((index) => index >= 0).sort((a, b) => a - b)[0] ?? 0;
+  const start = Math.max(0, position - 45);
+  return `${start ? '…' : ''}${text.slice(start, start + 160)}${text.length > start + 160 ? '…' : ''}`;
+}

# website/src/scripts/site.ts

diff --git a/website/src/scripts/site.ts b/website/src/scripts/site.ts
new file mode 100644
index 0000000..3524dc7
--- /dev/null
+++ b/website/src/scripts/site.ts
@@ -0,0 +1,165 @@
+import { searchEntries, excerpt } from './search.mjs';
+
+const root = document.documentElement;
+const themeToggle = document.querySelector<HTMLButtonElement>('[data-theme-toggle]')!;
+const systemTheme = matchMedia('(prefers-color-scheme: dark)');
+const isDark = () => root.dataset.theme ? root.dataset.theme === 'dark' : systemTheme.matches;
+const updateThemeLabel = () => {
+  themeToggle.setAttribute('aria-label', `Switch to ${isDark() ? 'light' : 'dark'} theme`);
+  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', isDark() ? '#121b19' : '#ffffff');
+};
+themeToggle.hidden = false;
+updateThemeLabel();
+systemTheme.addEventListener('change', updateThemeLabel);
+themeToggle.addEventListener('click', () => {
+  root.dataset.theme = isDark() ? 'light' : 'dark';
+  try { localStorage.setItem('devorbit.theme', root.dataset.theme); } catch {}
+  updateThemeLabel();
+});
+
+const dialog = document.querySelector<HTMLDialogElement>('.search-dialog')!;
+const input = dialog.querySelector<HTMLInputElement>('input')!;
+const results = dialog.querySelector<HTMLUListElement>('.search-results')!;
+const status = dialog.querySelector<HTMLParagraphElement>('.search-status')!;
+const retry = dialog.querySelector<HTMLButtonElement>('[data-search-retry]')!;
+const opener = document.querySelector<HTMLButtonElement>('[data-search-open]')!;
+type Entry = { title: string; text: string; href: string };
+let entries: Entry[] | undefined;
+let loading = false;
+let previousFocus: HTMLElement | null;
+opener.hidden = false;
+if (/Mac|iPhone|iPad/.test(navigator.platform)) opener.querySelector('kbd')!.textContent = '⌘ K';
+
+function renderResults() {
+  if (!entries) return;
+  const query = input.value.trim();
+  const matches: Entry[] = searchEntries(entries, query);
+  status.textContent = !query ? 'Suggested guides' : matches.length ? `${matches.length} ${matches.length === 1 ? 'result' : 'results'}` : `No results for “${query}”. Try a service name, “Docker”, or “configuration”.`;
+  results.replaceChildren(...matches.map((entry) => {
+    const li = document.createElement('li');
+    const link = document.createElement('a');
+    link.href = entry.href;
+    const title = document.createElement('strong');
+    title.textContent = entry.title;
+    const description = document.createElement('p');
+    description.textContent = excerpt(entry.text, query);
+    link.append(title, description);
+    link.addEventListener('click', () => dialog.close());
+    li.append(link);
+    return li;
+  }));
+}
+
+async function loadSearch() {
+  if (entries) { renderResults(); return; }
+  if (loading) return;
+  loading = true;
+  status.textContent = 'Loading documentation…';
+  retry.hidden = true;
+  try {
+    const response = await fetch('/search-index.json');
+    if (!response.ok) throw new Error('Search unavailable');
+    entries = await response.json();
+    renderResults();
+  } catch {
+    status.textContent = 'Documentation search couldn’t load. Check your connection and retry, or close search to browse the guides.';
+    retry.hidden = false;
+  } finally { loading = false; }
+}
+
+function openSearch() {
+  if (dialog.open) return;
+  previousFocus = document.activeElement as HTMLElement;
+  dialog.showModal();
+  document.body.style.overflow = 'hidden';
+  input.focus();
+  void loadSearch();
+}
+opener.addEventListener('click', openSearch);
+retry.addEventListener('click', () => void loadSearch());
+dialog.querySelector('[data-search-close]')!.addEventListener('click', () => dialog.close());
+dialog.addEventListener('close', () => { document.body.style.overflow = ''; previousFocus?.focus(); });
+dialog.addEventListener('click', (event) => { if (event.target === dialog) { const box = dialog.getBoundingClientRect(); if (event.clientX < box.left || event.clientX > box.right || event.clientY < box.top || event.clientY > box.bottom) dialog.close(); } });
+input.addEventListener('input', renderResults);
+dialog.querySelector('form')!.addEventListener('submit', (event) => { event.preventDefault(); results.querySelector('a')?.click(); });
+document.addEventListener('keydown', (event) => {
+  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); if (dialog.open) dialog.close(); else openSearch(); }
+});
+dialog.addEventListener('keydown', (event) => {
+  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
+  const links = [...results.querySelectorAll('a')];
+  if (!links.length) return;
+  event.preventDefault();
+  const index = links.indexOf(document.activeElement as HTMLAnchorElement);
+  if (index === -1) links[event.key === 'ArrowDown' ? 0 : links.length - 1].focus();
+  else if (event.key === 'ArrowUp' && index === 0) input.focus();
+  else links[(index + (event.key === 'ArrowDown' ? 1 : -1) + links.length) % links.length].focus();
+});
+
+for (const pre of document.querySelectorAll<HTMLPreElement>('pre')) {
+  const frame = document.createElement('div');
+  frame.className = 'code-frame';
+  const toolbar = document.createElement('div');
+  toolbar.className = 'code-toolbar';
+  const label = document.createElement('span');
+  label.textContent = pre.closest<HTMLElement>('[data-code-title]')?.dataset.codeTitle || pre.dataset.language || 'Code';
+  const copy = document.createElement('button');
+  copy.type = 'button';
+  copy.className = 'copy-code';
+  copy.textContent = 'Copy';
+  copy.setAttribute('aria-label', `Copy ${label.textContent}`);
+  copy.addEventListener('click', async () => {
+    try {
+      await navigator.clipboard.writeText(pre.querySelector('code')?.textContent ?? pre.textContent ?? '');
+      copy.textContent = 'Copied';
+      document.querySelector('#copy-status')!.textContent = `${label.textContent} copied to clipboard.`;
+      window.setTimeout(() => { copy.textContent = 'Copy'; }, 2000);
+    } catch {
+      const range = document.createRange();
+      range.selectNodeContents(pre);
+      window.getSelection()?.removeAllRanges();
+      window.getSelection()?.addRange(range);
+      document.querySelector('#copy-status')!.textContent = 'Clipboard unavailable. Code selected; use your device’s copy command.';
+      copy.textContent = 'Code selected';
+    }
+  });
+  toolbar.append(label, copy);
+  pre.before(frame);
+  frame.append(toolbar, pre);
+  pre.tabIndex = 0;
+  pre.setAttribute('aria-label', `${label.textContent} code`);
+}
+for (const table of document.querySelectorAll('.prose table')) {
+  const wrapper = document.createElement('div');
+  wrapper.className = 'table-scroll';
+  wrapper.tabIndex = 0;
+  wrapper.setAttribute('role', 'region');
+  wrapper.setAttribute('aria-label', 'Scrollable reference table');
+  table.before(wrapper);
+  wrapper.append(table);
+}
+const headings = [...document.querySelectorAll<HTMLElement>('.prose h2[id], .prose h3[id]')];
+for (const heading of headings) {
+  const anchor = document.createElement('a');
+  anchor.href = `#${heading.id}`;
+  anchor.className = 'heading-anchor';
+  anchor.setAttribute('aria-label', `Link to ${heading.textContent}`);
+  anchor.textContent = '#';
+  heading.append(anchor);
+}
+const tocLinks = [...document.querySelectorAll<HTMLAnchorElement>('.toc nav a')];
+if (tocLinks.length) {
+  let scheduled = false;
+  const updateOutline = () => {
+    let current = headings[0]?.id;
+    for (const heading of headings) if (heading.getBoundingClientRect().top <= 140) current = heading.id;
+    if (innerHeight + scrollY >= document.documentElement.scrollHeight - 4) current = headings.at(-1)?.id;
+    for (const link of tocLinks) {
+      if (link.hash === `#${current}`) link.setAttribute('aria-current', 'location');
+      else link.removeAttribute('aria-current');
+    }
+    scheduled = false;
+  };
+  addEventListener('scroll', () => { if (!scheduled) { scheduled = true; requestAnimationFrame(updateOutline); } }, { passive: true });
+  updateOutline();
+}

