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
vi.mock("@tauri-apps/api/core", () => ({ invoke }));
vi.mock("@tauri-apps/plugin-dialog", () => ({ open }));
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
  env: {},
  enabled: true,
  auto_restart: false,
  created_at: "",
  updated_at: "",
};
const process = {
  service_id: sid,
  execution_id: "00000000-0000-0000-0000-000000000000",
  pid: null,
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
const calls: string[] = [];
let shells: { id: string; workspace_id: string; shell: string; state: string; started_at: string; exit_code: number | null }[] = [];
let currentServices = [service];
let currentStatus = status;

beforeEach(() => {
  localStorage.clear();
  calls.length = 0;
  shells = [];
  currentServices = [service];
  currentStatus = status;
  invoke.mockReset();
  open.mockReset();
  invoke.mockImplementation(
    async (command: string, args: { method: string; path: string; body?: unknown }) => {
      if (command === "list_system_fonts") return ["Fira Code", "JetBrains Mono", "Liberation Mono"];
      if (command !== "api_request") return undefined;
      calls.push(`${args.method} ${args.path}`);
      if (args.path.startsWith("/api/v1/workspaces?"))
        return { status: 200, body: [workspace] };
      if (args.path.startsWith(`/api/v1/workspaces/${wid}/services?`))
        return { status: 200, body: currentServices };
      if (args.path === `/api/v1/workspaces/${wid}/services` && args.method === "POST")
        return { status: 201, body: { ...service, ...(args.body as object) } };
      if (args.path === `/api/v1/workspaces/${wid}/status`)
        return { status: 200, body: currentStatus };
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
      if (args.path === `/api/v1/services/${sid}` && args.method === "DELETE")
        return { status: 204, body: null };
      if (args.path.startsWith(`/api/v1/services/${sid}/logs?`))
        return {
          status: 200,
          body: {
            entries: [],
            next_cursor: "",
            diagnostics: { dropped_entries: 0, storage_errors: 0 },
          },
        };
      throw new Error(`Unexpected request ${args.method} ${args.path}`);
    },
  );
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
  expect(screen.getByRole("region", { name: "API server terminal" }).getAttribute("data-font")).toBe("Fira Code");
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

it("stops a service before deleting its configuration", async () => {
  vi.spyOn(window, "confirm").mockReturnValue(true);
  render(<App />);
  fireEvent.click(
    await screen.findByRole("button", { name: "Open API server" }),
  );
  fireEvent.click(
    await screen.findByRole("button", { name: "Delete service" }),
  );
  await waitFor(() =>
    expect(calls).toContain(`DELETE /api/v1/services/${sid}`),
  );
  expect(calls.indexOf(`POST /api/v1/services/${sid}/stop`)).toBeLessThan(
    calls.indexOf(`DELETE /api/v1/services/${sid}`),
  );
});

it("closes the workspace dialog with Escape", async () => {
  render(<App />);
  fireEvent.click(screen.getByRole("button", { name: "Create workspace" }));
  expect(screen.getByRole("dialog", { name: "New workspace" })).toBeTruthy();
  fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
});

it("browses for a workspace root and keeps it when selection is cancelled", async () => {
  open.mockResolvedValueOnce("/tmp/chosen-project").mockResolvedValueOnce(null);
  render(<App />);
  fireEvent.click(screen.getByRole("button", { name: "Create workspace" }));
  fireEvent.click(screen.getByRole("button", { name: "Browse for workspace directory" }));
  await waitFor(() => expect(screen.getByRole("textbox", { name: /Root directory/ }).getAttribute("value")).toBe("/tmp/chosen-project"));
  fireEvent.click(screen.getByRole("button", { name: "Browse for workspace directory" }));
  await waitFor(() => expect(open).toHaveBeenCalledTimes(2));
  expect(screen.getByRole("textbox", { name: /Root directory/ }).getAttribute("value")).toBe("/tmp/chosen-project");
  expect(open).toHaveBeenCalledWith({ directory: true, multiple: false });
});

it("browses for a service working directory", async () => {
  open.mockResolvedValue("/tmp/project/api");
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: "Add service" }));
  fireEvent.click(screen.getByRole("button", { name: "Browse for service working directory" }));
  await waitFor(() => expect(screen.getByRole("textbox", { name: /Working directory/ }).getAttribute("value")).toBe("/tmp/project/api"));
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
