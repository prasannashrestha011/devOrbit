// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";

const invoke = vi.hoisted(() => vi.fn());
const open = vi.hoisted(() => vi.fn());
const homeDir = vi.hoisted(() => vi.fn());
vi.mock("@tauri-apps/api/core", () => ({ invoke }));
vi.mock("@tauri-apps/plugin-dialog", () => ({ open }));
vi.mock("@tauri-apps/api/path", () => ({ homeDir }));
vi.mock("@tauri-apps/api/event", () => ({
  listen: vi.fn(async () => () => {}),
}));
vi.mock("./TerminalPane", () => ({ default: ({ shell, service, status, fontFamily }: { shell?: { shell: string }; service?: { name: string }; status?: { state: string }; fontFamily?: string }) => <section aria-label={`${shell?.shell ?? service?.name} terminal`} data-state={status?.state} data-font={fontFamily}><div>{shell ? `${shell.shell} screen` : `${service?.name} screen`}</div></section> }));

import App from "./App";

const wid = "30c80606-7a89-4b84-94dd-084e33e88baa";
const sid = "51d71407-759a-44c0-b46e-87a842998ddd";
const tid = "af951022-bfcf-4773-82fe-f2682c307102";
const workspace = {
  id: wid,
  name: "Project Alpha",
  description: "Development tools",
  root_path: "/tmp/project",
  created_at: "",
  updated_at: "",
};
const service = {
  id: sid,
  workspace_id: wid,
  name: "API server",
  command: "go",
  args: ["run", "."],
  working_directory: "",
  git_branch: "",
  env: {},
  ports: {},
  enabled: true,
  auto_restart: false,
  created_at: "",
  updated_at: "",
};
const process = {
  service_id: sid,
  execution_id: "00000000-0000-0000-0000-000000000000",
  pid: null,
  listening_ports: [] as number[],
  state: "stopped",
  started_at: null,
  exited_at: null,
  exit_code: null as number | null,
  last_error: "",
  updated_at: "",
};
const status = {
  workspace_id: wid,
  workspace_name: workspace.name,
  aggregate_state: "idle",
  total_services: 1,
  enabled_services: 1,
  running_services: 0,
  stopped_services: 1,
  failed_services: 0,
  starting_services: 0,
  stopping_services: 0,
  exited_services: 0,
  services: [{ ...process, name: service.name, enabled: true }],
};
const configuration = { content: "services:\n  API server:\n    command: go run .\n", revision: "revision-1", config: { services: {} } };
const calls: string[] = [];
let shells: { id: string; workspace_id: string; shell: string; state: string; started_at: string; exit_code: number | null }[] = [];
let currentServices = [service];
let currentStatus = status;
let failServiceList = false;
let failWorkspaceStop = false;
let envFiles: Record<string, Record<string, { content: string; revision: string }>> = {};
let delayedEnvRead: Promise<{ status: number; body: unknown }> | null = null;
let dockerSnapshot: unknown = { availability: { state: "missing", message: "Docker CLI is unavailable." }, candidates: [] };

beforeEach(() => {
  localStorage.clear();
  calls.length = 0;
  shells = [];
  currentServices = [service];
  currentStatus = status;
  failServiceList = false;
  failWorkspaceStop = false;
  envFiles = { [sid]: {}, [tid]: {} };
  delayedEnvRead = null;
  dockerSnapshot = { availability: { state: "missing", message: "Docker CLI is unavailable." }, candidates: [] };
  invoke.mockReset();
  open.mockReset();
  homeDir.mockReset().mockResolvedValue("/home/tester");
  invoke.mockImplementation(
    async (command: string, args: { method: string; path: string; body?: unknown }) => {
      if (command === "list_system_fonts") return ["Fira Code", "JetBrains Mono", "Liberation Mono"];
      if (command !== "api_request") return undefined;
      calls.push(`${args.method} ${args.path}`);
      if (args.path.startsWith("/api/v1/workspaces?"))
        return { status: 200, body: [workspace] };
      if (args.path.startsWith(`/api/v1/workspaces/${wid}/services?`)) {
        if (failServiceList) throw new Error("service list unavailable");
        return { status: 200, body: currentServices };
      }
      if (args.path === `/api/v1/workspaces/${wid}/configuration` && args.method === "GET")
        return { status: 200, body: configuration };
      if (args.path === `/api/v1/workspaces/${wid}/services` && args.method === "POST")
        return { status: 201, body: { ...service, ...(args.body as object) } };
      if (args.path === `/api/v1/workspaces/${wid}/status`)
        return { status: 200, body: currentStatus };
      if (args.path === `/api/v1/workspaces/${wid}/docker` && args.method === "GET")
        return { status: 200, body: dockerSnapshot };
      if (args.path === `/api/v1/workspaces/${wid}/terminals` && args.method === "GET")
        return { status: 200, body: shells };
      if (args.path === `/api/v1/workspaces/${wid}/terminals` && args.method === "POST") {
        const shell = { id: tid, workspace_id: wid, shell: "bash", state: "running", started_at: "2026-10-19T00:00:00Z", exit_code: null };
        shells = [...shells, shell];
        return { status: 201, body: shell };
      }
      if (args.path === `/api/v1/workspaces/${wid}/terminals/${tid}` && args.method === "DELETE") {
        shells = shells.filter((item) => item.id !== tid);
        return { status: 204, body: null };
      }
      if (args.path === `/api/v1/workspaces/${wid}/stop`)
        return { status: 200, body: { workspace_id: wid, operation: "stop", total: 1, succeeded: failWorkspaceStop ? 0 : 1, failed: failWorkspaceStop ? 1 : 0, skipped: 0, results: failWorkspaceStop ? [{ service_id: sid, service_name: service.name, outcome: "failed", pid: null, error: "service operation failed" }] : [] } };
      if (args.path === `/api/v1/workspaces/${wid}` && args.method === "DELETE")
        return { status: 204, body: null };
      if (args.path === `/api/v1/workspaces/${wid}/start`) {
        currentStatus = { ...status, services: currentServices.map((item) => ({ ...process, service_id: item.id, execution_id: item.id, state: "running", name: item.name, enabled: true })) };
        return {
          status: 200,
          body: {
            workspace_id: wid,
            operation: "start",
            total: currentServices.length,
            succeeded: currentServices.length,
            failed: 0,
            skipped: 0,
            results: currentServices.map((item) => ({
                service_id: item.id,
                service_name: item.name,
                outcome: "started",
                pid: 22,
                error: null,
              })),
          },
        };
      }
      if (args.path === `/api/v1/services/${sid}/start`) {
        currentStatus = { ...status, services: [{ ...process, execution_id: sid, state: "running", name: service.name, enabled: true }] };
        return { status: 200, body: { ...process, execution_id: sid, state: "running" } };
      }
      if (args.path === `/api/v1/services/${sid}/stop`)
        return { status: 200, body: process };
      if (args.path === `/api/v1/services/${sid}?revision=revision-1` && args.method === "DELETE")
        return { status: 204, body: null };
      if (/^\/api\/v1\/services\/[^/]+\/logs\?/.test(args.path))
        return {
          status: 200,
          body: {
            entries: [],
            next_cursor: "",
            diagnostics: { dropped_entries: 0, storage_errors: 0 },
          },
        };
      const environment = args.path.match(/^\/api\/v1\/services\/([^/]+)\/environment(?:\/([^/]+))?$/);
      if (environment) {
        const [, serviceId, encodedName] = environment;
        const files = envFiles[serviceId] ?? {};
        if (!encodedName && args.method === "GET") return { status: 200, body: Object.keys(files).sort() };
        const name = decodeURIComponent(encodedName);
        if (args.method === "GET" && delayedEnvRead && serviceId === sid) return delayedEnvRead;
        if (args.method === "GET" && files[name]) return { status: 200, body: { name, ...files[name] } };
        if (args.method === "PUT" && files[name]) {
          const body = args.body as { content: string; revision: string };
          if (body.revision !== files[name].revision) return { status: 409, body: { error: { code: "conflict", message: "environment file changed; reload before saving" } } };
          files[name] = { content: body.content, revision: `${body.revision}-saved` };
          return { status: 200, body: { name, ...files[name] } };
        }
        return { status: 404, body: { error: { code: "not_found", message: "file not found" } } };
      }
      throw new Error(`Unexpected request ${args.method} ${args.path}`);
    },
  );
});

it("lists detected Docker services in the workspace overview when the Engine is unavailable", async () => {
  dockerSnapshot = {
    availability: { state: "engine_unavailable", message: "Docker Engine is inaccessible." },
    candidates: [{ file: "compose.yaml", files: ["compose.yaml"] }],
    project: { file: "compose.yaml", files: ["compose.yaml"], name: "project", created: 0, running: 0, stopped: 0,
      services: [{ name: "postgres", image: "postgres:16", build: false, ports: [{ target: 5432, published: "5432", protocol: "tcp" }], containers: [] }] },
  };
  render(<App />);
  const section = await screen.findByRole("region", { name: "Docker Compose" });
  await waitFor(() => expect(section.textContent).toContain("postgres"));
  expect(section.querySelector("h2")?.textContent).toContain("Docker services");
  expect(section.textContent).toContain("postgres:16");
  expect(section.textContent).toContain("Container status unavailable");
  expect(section.textContent).not.toContain("Defined · no container");
  expect(section.querySelector(".docker-actions button")?.hasAttribute("disabled")).toBe(true);
  const toggle = screen.getByRole("button", { name: "Compose services" });
  expect(toggle.getAttribute("aria-expanded")).toBe("true");
  fireEvent.click(toggle);
  expect(toggle.getAttribute("aria-expanded")).toBe("false");
  expect(section.textContent).not.toContain("postgres:16");
  expect(section.textContent).toContain("project");
  fireEvent.click(toggle);
  expect(toggle.getAttribute("aria-expanded")).toBe("true");
  expect(section.textContent).toContain("postgres:16");
});

it("lists installed fonts and keeps the chosen font across service switches", async () => {
  currentServices = [service, { ...service, id: tid, name: "Worker" }];
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Terminals" }));
  const settings = screen.getByRole("button", { name: "Settings" });
  expect(settings.closest(".sidebar-bottom")).toBeTruthy();
  expect(screen.queryByRole("combobox", { name: "Terminal font" })).toBeNull();
  fireEvent.click(settings);
  expect(settings.getAttribute("aria-expanded")).toBe("true");
  const select = await screen.findByRole("combobox", { name: "Terminal font" });
  await waitFor(() => expect(select.querySelectorAll("option")).toHaveLength(4));
  fireEvent.change(select, { target: { value: "Fira Code" } });
  expect(localStorage.getItem("devorbit.terminalFont")).toBe("Fira Code");
  expect((await screen.findByRole("region", { name: "API server terminal" })).getAttribute("data-font")).toBe("Fira Code");
  fireEvent.click(screen.getByRole("button", { name: /Worker stopped/ }));
  expect((screen.getByRole("combobox", { name: "Terminal font" }) as HTMLSelectElement).value).toBe("Fira Code");
  expect(screen.getByRole("region", { name: "Worker terminal" }).getAttribute("data-font")).toBe("Fira Code");
  fireEvent.click(settings);
  expect(screen.queryByRole("combobox", { name: "Terminal font" })).toBeNull();
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

it("opens one inline terminal and switches existing service sessions", async () => {
  currentServices = [service, { ...service, id: tid, name: "Worker" }];
  render(<App />);
  await screen.findByText("API server");
  fireEvent.click(screen.getByRole("button", { name: "Start all" }));
  const workspace = await screen.findByRole("region", { name: "Project Alpha terminal workspace" });
  expect(screen.queryByRole("dialog", { name: "Project Alpha terminals" })).toBeNull();
  expect(workspace.querySelectorAll('section[aria-label$=" terminal"]')).toHaveLength(1);
  expect(screen.getByText("API server screen")).toBeTruthy();
  expect(screen.queryByText("Worker screen")).toBeNull();
  expect(calls).toContain(`POST /api/v1/workspaces/${wid}/start`);
  expect(calls).not.toContain(`POST /api/v1/workspaces/${wid}/terminals`);
  fireEvent.click(screen.getByRole("button", { name: /Worker running/ }));
  expect(screen.getByText("Worker screen")).toBeTruthy();
  expect(screen.queryByText("API server screen")).toBeNull();
  expect(workspace.querySelectorAll('section[aria-label$=" terminal"]')).toHaveLength(1);
  fireEvent.click(screen.getByRole("button", { name: "Split right" }));
  expect(invoke).toHaveBeenCalledWith("terminal_input", { serviceId: tid, executionId: tid, data: "\x02%" });
  fireEvent.click(screen.getByRole("button", { name: "Split down" }));
  expect(invoke).toHaveBeenCalledWith("terminal_input", { serviceId: tid, executionId: tid, data: "\x02\"" });
});

it("restarts the workspace by stopping before starting", async () => {
  currentStatus = { ...status, aggregate_state: "running", running_services: 1, stopped_services: 0 };
  render(<App />);
  const restart = await screen.findByRole("button", { name: "Restart all" });
  await waitFor(() => expect(restart.hasAttribute("disabled")).toBe(false));
  fireEvent.click(restart);
  await screen.findByText("Restart all results");
  expect(calls.indexOf(`POST /api/v1/workspaces/${wid}/stop`)).toBeLessThan(calls.indexOf(`POST /api/v1/workspaces/${wid}/start`));
});

it("shows detected listening ports only for running services in the overview", async () => {
  currentServices = [
    { ...service, ports: { http: 1234 } },
    { ...service, id: tid, name: "Worker", ports: { http: 3000 } },
  ];
  currentStatus = {
    ...status,
    running_services: 1,
    services: [
      { ...process, state: "running", name: service.name, enabled: true, listening_ports: [8080, 9090] },
      { ...process, service_id: tid, name: "Worker", enabled: true, listening_ports: [3000] },
    ],
  };
  render(<App />);
  expect(await screen.findByText(":8080")).toBeTruthy();
  expect(screen.getByText(":9090")).toBeTruthy();
  expect(screen.queryByText(":3000")).toBeNull();
  expect(screen.queryByText(":1234")).toBeNull();
  expect(screen.getByText("PORT").getAttribute("title")).toContain("Listening TCP ports");
});

it("cancels restart all when a service cannot be stopped", async () => {
  currentStatus = { ...status, aggregate_state: "running", running_services: 1, stopped_services: 0 };
  failWorkspaceStop = true;
  render(<App />);
  const restart = await screen.findByRole("button", { name: "Restart all" });
  await waitFor(() => expect(restart.hasAttribute("disabled")).toBe(false));
  fireEvent.click(restart);
  await screen.findByText(/Some services could not be stopped\. Restart all was cancelled\./);
  expect(calls).toContain(`POST /api/v1/workspaces/${wid}/stop`);
  expect(calls).not.toContain(`POST /api/v1/workspaces/${wid}/start`);
});

it("shows each service Git branch in the overview and details", async () => {
  currentServices = [
    { ...service, git_branch: "api-feature" },
    { ...service, id: tid, name: "Worker", git_branch: "worker-fix" },
  ];
  render(<App />);
  expect((await screen.findByText("api-feature")).closest(".service-name-line")?.textContent).toBe("API serverapi-feature");
  expect(screen.getByText("worker-fix").closest(".service-name-line")?.textContent).toBe("Workerworker-fix");
  expect(screen.getByText("worker-fix").closest(".git-branch")?.querySelector(".lucide-git-branch")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: /Worker worker-fix Enabled/ }));
  expect(screen.getByRole("heading", { name: "Worker worker-fix" }).querySelector(".git-branch svg")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Workspace overview" }));
  fireEvent.click(screen.getByRole("button", { name: /API server api-feature Enabled/ }));
  expect(screen.getByRole("heading", { name: "API server api-feature" }).querySelector(".git-branch svg")).toBeTruthy();
});

it("opens the inline terminal after starting one service", async () => {
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Start API server" }));
  await screen.findByRole("region", { name: "Project Alpha terminal workspace" });
  expect(screen.getByText("API server screen")).toBeTruthy();
  expect(calls).toContain(`POST /api/v1/services/${sid}/start`);
});

it("keeps an exited service selectable and returns to the overview without stopping it", async () => {
  currentStatus = { ...status, services: [{ ...process, execution_id: sid, state: "exited", exit_code: 0, name: service.name, enabled: true }] };
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Terminals" }));
  expect(screen.getByRole("region", { name: "API server terminal" }).getAttribute("data-state")).toBe("exited");
  expect(screen.getByText("API server screen")).toBeTruthy();
  fireEvent.click(screen.getAllByRole("button", { name: "Project Alpha" })[1]);
  expect(screen.queryByRole("region", { name: "Project Alpha terminal workspace" })).toBeNull();
  expect(calls).not.toContain(`POST /api/v1/services/${sid}/stop`);
});

it("deletes an idle service through YAML without stopping it", async () => {
  vi.spyOn(window, "confirm").mockReturnValue(true);
  render(<App />);
  fireEvent.click(
    await screen.findByRole("button", { name: "Open API server" }),
  );
  fireEvent.click(
    await screen.findByRole("button", { name: "Delete service" }),
  );
  await waitFor(() =>
    expect(calls).toContain(`DELETE /api/v1/services/${sid}?revision=revision-1`),
  );
  expect(calls).not.toContain(`POST /api/v1/services/${sid}/stop`);
});

it("deletes a workspace even when its service list cannot be loaded", async () => {
  failServiceList = true;
  vi.spyOn(window, "confirm").mockReturnValue(true);
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Delete workspace" }));
  await waitFor(() => expect(calls).toContain(`DELETE /api/v1/workspaces/${wid}`));
});

it("closes workspace shell terminals before deleting the workspace", async () => {
  shells = [{ id: tid, workspace_id: wid, shell: "bash", state: "running", started_at: "2026-10-19T00:00:00Z", exit_code: null }];
  vi.spyOn(window, "confirm").mockReturnValue(true);
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Delete workspace" }));
  await waitFor(() => expect(calls).toContain(`DELETE /api/v1/workspaces/${wid}`));
  expect(calls.indexOf(`DELETE /api/v1/workspaces/${wid}/terminals/${tid}`)).toBeLessThan(calls.indexOf(`DELETE /api/v1/workspaces/${wid}`));
});

it("closes the workspace dialog with Escape", async () => {
  render(<App />);
  fireEvent.click(screen.getByRole("button", { name: "Create workspace" }));
  expect(screen.getByRole("dialog", { name: "New workspace" })).toBeTruthy();
  fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
});

it("creates a workspace without asking for a directory", async () => {
  render(<App />);
  fireEvent.click(screen.getByRole("button", { name: "Create workspace" }));
  expect(screen.queryByRole("textbox", { name: /Root directory/ })).toBeNull();
  fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "New project" } });
  fireEvent.submit(screen.getByRole("dialog", { name: "New workspace" }));
  await waitFor(() => expect(calls).toContain("POST /api/v1/workspaces"));
  const request = invoke.mock.calls.find(([, args]) => args?.path === "/api/v1/workspaces" && args.method === "POST");
  expect(request?.[1].body).toMatchObject({ name: "New project" });
  expect(request?.[1].body).not.toHaveProperty("root_path");
});

it("browses for a service directory and saves its workspace-relative path", async () => {
  open.mockResolvedValueOnce("/tmp/project/backend").mockResolvedValueOnce(null);
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Add service" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "Backend" } });
  fireEvent.change(screen.getByRole("textbox", { name: "Command" }), { target: { value: "go run ." } });
  fireEvent.click(screen.getByRole("button", { name: "Browse for service working directory" }));
  const directory = screen.getByRole("textbox", { name: "Working directory" });
  await waitFor(() => expect(directory.getAttribute("value")).toBe("/tmp/project/backend"));
  expect(open).toHaveBeenCalledWith({ directory: true, multiple: false, defaultPath: "/home/tester" });
  fireEvent.click(screen.getByRole("button", { name: "Browse for service working directory" }));
  await waitFor(() => expect(open).toHaveBeenCalledTimes(2));
  expect(open).toHaveBeenLastCalledWith({ directory: true, multiple: false, defaultPath: "/tmp/project/backend" });
  expect(directory.getAttribute("value")).toBe("/tmp/project/backend");
  expect(localStorage.getItem("devorbit.lastPickerDirectory")).toBe("/tmp/project/backend");
  fireEvent.submit(screen.getByRole("dialog", { name: "Add a service" }));
  await waitFor(() => expect(calls).toContain(`POST /api/v1/workspaces/${wid}/services`));
  const request = invoke.mock.calls.find(([, args]) => args?.path === `/api/v1/workspaces/${wid}/services` && args.method === "POST");
  expect(request?.[1].body).toMatchObject({ working_directory: "backend" });
});

it("uses the directory saved by an earlier picker session", async () => {
  localStorage.setItem("devorbit.lastPickerDirectory", "/tmp/previous-project");
  open.mockResolvedValue(null);
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Add service" }));
  fireEvent.click(screen.getByRole("button", { name: "Browse for service working directory" }));
  await waitFor(() => expect(open).toHaveBeenCalledWith({ directory: true, multiple: false, defaultPath: "/tmp/previous-project" }));
  expect(homeDir).not.toHaveBeenCalled();
});

it("shows the selected workspace root in the service directory field", async () => {
  open.mockResolvedValue("/tmp/project");
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Add service" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "Root service" } });
  fireEvent.change(screen.getByRole("textbox", { name: "Command" }), { target: { value: "npm run dev" } });
  fireEvent.click(screen.getByRole("button", { name: "Browse for service working directory" }));
  await waitFor(() => expect(screen.getByRole("textbox", { name: "Working directory" }).getAttribute("value")).toBe("/tmp/project"));
  fireEvent.submit(screen.getByRole("dialog", { name: "Add a service" }));
  await waitFor(() => expect(calls).toContain(`POST /api/v1/workspaces/${wid}/services`));
  const request = invoke.mock.calls.find(([, args]) => args?.path === `/api/v1/workspaces/${wid}/services` && args.method === "POST");
  expect(request?.[1].body).toMatchObject({ working_directory: "" });
});

it("saves a browsed service directory outside the workspace as an absolute path", async () => {
  open.mockResolvedValue("/tmp/another-project/backend");
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Add service" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "External backend" } });
  fireEvent.change(screen.getByRole("textbox", { name: "Command" }), { target: { value: "go run ." } });
  fireEvent.click(screen.getByRole("button", { name: "Browse for service working directory" }));
  await waitFor(() => expect(screen.getByRole("textbox", { name: "Working directory" }).getAttribute("value")).toBe("/tmp/another-project/backend"));
  fireEvent.submit(screen.getByRole("dialog", { name: "Add a service" }));
  await waitFor(() => expect(calls).toContain(`POST /api/v1/workspaces/${wid}/services`));
  const request = invoke.mock.calls.find(([, args]) => args?.path === `/api/v1/workspaces/${wid}/services` && args.method === "POST");
  expect(request?.[1].body).toMatchObject({ working_directory: "/tmp/another-project/backend" });
});

it("saves a typed command line as separate executable and arguments", async () => {
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Add service" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "Flutter" } });
  fireEvent.change(screen.getByRole("textbox", { name: "Command" }), { target: { value: "flutter run -d linux" } });
  fireEvent.submit(screen.getByRole("dialog", { name: "Add a service" }));
  await waitFor(() => expect(calls).toContain(`POST /api/v1/workspaces/${wid}/services`));
  const request = invoke.mock.calls.find(([, args]) => args?.path === `/api/v1/workspaces/${wid}/services` && args.method === "POST");
  expect(request?.[1].body).toMatchObject({ command: "flutter", args: ["run", "-d", "linux"] });
});

it("creates and closes an independent workspace shell through the same viewport", async () => {
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Terminals" }));
  fireEvent.click(screen.getByRole("button", { name: "New terminal" }));
  await waitFor(() => expect(calls).toContain(`POST /api/v1/workspaces/${wid}/terminals`));
  expect(await screen.findByText("bash screen")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Close shell" }));
  await waitFor(() => expect(calls).toContain(`DELETE /api/v1/workspaces/${wid}/terminals/${tid}`));
  expect(screen.queryByText("bash screen")).toBeNull();
});

it("discovers each service's files and keeps unsaved drafts isolated while navigating", async () => {
  currentServices = [service, { ...service, id: tid, name: "Worker" }];
  envFiles[sid] = { ".env": { content: "# API\r\nPORT=8080\r\n", revision: "api-1" }, ".env.local": { content: "LOCAL=1\n", revision: "api-2" } };
  envFiles[tid] = { ".env.production": { content: "WORKER=1\n", revision: "worker-1" } };
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: /API server Enabled/ }));
  await waitFor(() => expect(calls).toContain(`GET /api/v1/services/${sid}/environment`));
  fireEvent.click(screen.getByRole("tab", { name: "Environment" }));
  const editor = await screen.findByRole("textbox", { name: "Environment file contents" }) as HTMLTextAreaElement;
  await waitFor(() => expect(editor.value).toBe("# API\nPORT=8080\n"));
  expect(screen.getByRole("button", { name: ".env.local" })).toBeTruthy();
  fireEvent.change(editor, { target: { value: "# API\r\nPORT=9090\r\n" } });
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Workspace overview" }));
  fireEvent.click(screen.getByRole("button", { name: /Worker Enabled/ }));
  await waitFor(() => expect(calls).toContain(`GET /api/v1/services/${tid}/environment`));
  fireEvent.click(screen.getByRole("tab", { name: "Environment" }));
  await waitFor(() => expect((screen.getByRole("textbox", { name: "Environment file contents" }) as HTMLTextAreaElement).value).toBe("WORKER=1\n"));
  fireEvent.click(screen.getByRole("button", { name: "Workspace overview" }));
  fireEvent.click(screen.getByRole("button", { name: /API server Enabled/ }));
  fireEvent.click(screen.getByRole("tab", { name: /Environment/ }));
  await waitFor(() => expect((screen.getByRole("textbox", { name: "Environment file contents" }) as HTMLTextAreaElement).value).toBe("# API\nPORT=9090\n"));
  expect(envFiles[sid][".env"].content).toBe("# API\r\nPORT=8080\r\n");
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(envFiles[sid][".env"].content).toBe("# API\r\nPORT=9090\r\n"));
  expect(envFiles[sid][".env.local"].content).toBe("LOCAL=1\n");
  expect(envFiles[tid][".env.production"].content).toBe("WORKER=1\n");
});

it("ignores a stale file read after switching services and reports save conflicts", async () => {
  currentServices = [service, { ...service, id: tid, name: "Worker" }];
  envFiles[sid] = { ".env": { content: "API=1\n", revision: "api-1" } };
  envFiles[tid] = { ".env": { content: "WORKER=1\n", revision: "worker-1" } };
  let release!: (value: { status: number; body: unknown }) => void;
  delayedEnvRead = new Promise((resolve) => { release = resolve; });
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: /API server Enabled/ }));
  fireEvent.click(screen.getByRole("tab", { name: "Environment" }));
  await waitFor(() => expect(calls).toContain(`GET /api/v1/services/${sid}/environment/.env`));
  fireEvent.click(screen.getByRole("button", { name: "Workspace overview" }));
  fireEvent.click(screen.getByRole("button", { name: /Worker Enabled/ }));
  fireEvent.click(screen.getByRole("tab", { name: "Environment" }));
  await waitFor(() => expect((screen.getByRole("textbox", { name: "Environment file contents" }) as HTMLTextAreaElement).value).toBe("WORKER=1\n"));
  release({ status: 200, body: { name: ".env", content: "API=1\n", revision: "api-1" } });
  await waitFor(() => expect((screen.getByRole("textbox", { name: "Environment file contents" }) as HTMLTextAreaElement).value).toBe("WORKER=1\n"));
  fireEvent.change(screen.getByRole("textbox", { name: "Environment file contents" }), { target: { value: "WORKER=2\n" } });
  envFiles[tid][".env"].revision = "external";
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByRole("alert")).toHaveProperty("textContent", "environment file changed; reload before saving");
  expect(envFiles[tid][".env"].content).toBe("WORKER=1\n");
  expect((screen.getByRole("textbox", { name: "Environment file contents" }) as HTMLTextAreaElement).value).toBe("WORKER=2\n");
});

it("keeps the original file revision with an unsaved draft after navigation", async () => {
  envFiles[sid] = { ".env": { content: "PORT=8080\n", revision: "original" } };
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: /API server Enabled/ }));
  fireEvent.click(screen.getByRole("tab", { name: "Environment" }));
  const editor = await screen.findByRole("textbox", { name: "Environment file contents" }) as HTMLTextAreaElement;
  await waitFor(() => expect(editor.value).toBe("PORT=8080\n"));
  fireEvent.change(editor, { target: { value: "PORT=9090\n" } });
  fireEvent.click(screen.getByRole("button", { name: "Workspace overview" }));
  envFiles[sid][".env"] = { content: "PORT=7000\n", revision: "external" };
  fireEvent.click(screen.getByRole("button", { name: /API server Enabled/ }));
  fireEvent.click(screen.getByRole("tab", { name: /Environment/ }));
  await waitFor(() => expect(calls.filter((call) => call === `GET /api/v1/services/${sid}/environment/.env`)).toHaveLength(2));
  expect((screen.getByRole("textbox", { name: "Environment file contents" }) as HTMLTextAreaElement).value).toBe("PORT=9090\n");
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByRole("alert")).toHaveProperty("textContent", "environment file changed; reload before saving");
  expect(envFiles[sid][".env"].content).toBe("PORT=7000\n");
});
