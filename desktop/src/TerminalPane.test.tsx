// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import type { ProcessStatus, Service, ShellSession } from "./api";

const calls = vi.hoisted(() => ({ invoke: vi.fn(), focus: vi.fn(), fit: vi.fn(), options: null as { fontFamily: string } | null, onData: (_data: string) => {}, writes: [] as string[] }));
vi.mock("@tauri-apps/api/core", () => ({
  invoke: calls.invoke,
  Channel: class { onmessage = (_frame: unknown) => {}; },
}));
vi.mock("@xterm/xterm", () => ({
  Terminal: class {
    cols = 80; rows = 24;
    options: { fontFamily: string };
    constructor(options: { fontFamily: string }) { this.options = options; calls.options = options; }
    loadAddon() {}
    open() {}
    write(data: string | Uint8Array) { calls.writes.push(typeof data === "string" ? data : new TextDecoder().decode(data)); }
    onData(callback: (data: string) => void) { calls.onData = callback; }
    focus() { calls.focus(); }
    dispose() {}
  },
}));
vi.mock("@xterm/addon-fit", () => ({ FitAddon: class { fit() { calls.fit(); } } }));
vi.mock("@xterm/addon-search", () => ({ SearchAddon: class { findNext() {} } }));

import TerminalPane from "./TerminalPane";

const service = { id: "51d71407-759a-44c0-b46e-87a842998ddd", name: "API", enabled: true } as Service;
const status = { execution_id: "30c80606-7a89-4b84-94dd-084e33e88baa", state: "running" } as ProcessStatus;

afterEach(() => { cleanup(); calls.invoke.mockReset(); calls.focus.mockReset(); calls.fit.mockReset(); calls.options = null; calls.writes.length = 0; });

it("changes the font and refits the active terminal without reconnecting", async () => {
  vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
  calls.invoke.mockImplementation(async (command: string, args: { channel?: { onmessage: (frame: unknown) => void } }) => {
    if (command === "start_terminal") args.channel?.onmessage({ type: "connected" });
  });
  const props = { service, status, focused: true, expanded: true, busy: false, onFocus: () => {} };
  const view = render(<TerminalPane {...props} fontFamily="Fira Code" />);
  await waitFor(() => expect(calls.options?.fontFamily).toBe('"Fira Code", monospace'));
  const resizeCount = calls.invoke.mock.calls.filter(([name]) => name === "terminal_resize").length;
  view.rerender(<TerminalPane {...props} fontFamily="Liberation Mono" />);
  expect(calls.options?.fontFamily).toBe('"Liberation Mono", monospace');
  expect(calls.invoke.mock.calls.filter(([name]) => name === "terminal_resize").length).toBeGreaterThan(resizeCount);
  expect(calls.invoke.mock.calls.filter(([name]) => name === "start_terminal")).toHaveLength(1);
});

it("streams output and accepts input only from the focused pane, including a large paste", async () => {
  vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
  calls.invoke.mockImplementation(async (command: string, args: { channel?: { onmessage: (frame: unknown) => void } }) => {
    if (command === "start_terminal") {
      args.channel?.onmessage({ type: "connected" });
      args.channel?.onmessage({ type: "output", data: btoa("ready") });
    }
  });
  const props = { service, status, focused: false, expanded: false, busy: false, onFocus: vi.fn(), onExpand: vi.fn(), onAction: vi.fn() };
  const view = render(<TerminalPane {...props} />);
  await waitFor(() => expect(calls.writes.join("")).toContain("ready"));
  await waitFor(() => expect(calls.invoke).toHaveBeenCalledWith("terminal_resize", { serviceId: service.id, executionId: status.execution_id, cols: 80, rows: 24 }));
  calls.onData("ignored");
  expect(calls.invoke).not.toHaveBeenCalledWith("terminal_input", expect.anything());
  view.rerender(<TerminalPane {...props} focused />);
  expect(screen.getByText("INPUT → API")).toBeTruthy();
  expect(calls.focus).toHaveBeenCalled();
  calls.onData("🙂".repeat(1300));
  await waitFor(() => expect(calls.invoke.mock.calls.filter(([name]) => name === "terminal_input")).toHaveLength(2));
  const chunks = calls.invoke.mock.calls.filter(([name]) => name === "terminal_input").map(([, args]) => args.data as string);
  expect(chunks.join("")).toBe("🙂".repeat(1300));
  expect(chunks.every((chunk) => new TextEncoder().encode(chunk).length <= 4096)).toBe(true);
  view.unmount();
  await waitFor(() => expect(calls.invoke).toHaveBeenCalledWith("stop_terminal", expect.objectContaining({ serviceId: service.id, executionId: status.execution_id, viewerId: expect.any(String) })));
});

it("attaches a shell, routes input to its session, and detaches without closing it", async () => {
  vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
  calls.invoke.mockImplementation(async (command: string, args: { channel?: { onmessage: (frame: unknown) => void } }) => {
    if (command === "start_shell_terminal") args.channel?.onmessage({ type: "connected" });
  });
  const shell = { id: "af951022-bfcf-4773-82fe-f2682c307102", workspace_id: "30c80606-7a89-4b84-94dd-084e33e88baa", shell: "bash", state: "running", exit_code: null } as ShellSession;
  const view = render(<TerminalPane shell={shell} workspaceId={shell.workspace_id} focused expanded busy={false} onFocus={() => {}} />);
  await waitFor(() => expect(calls.invoke).toHaveBeenCalledWith("start_shell_terminal", expect.objectContaining({ workspaceId: shell.workspace_id, sessionId: shell.id })));
  await screen.findByText("INPUT → bash");
  calls.onData("pwd\r");
  await waitFor(() => expect(calls.invoke).toHaveBeenCalledWith("shell_terminal_input", { sessionId: shell.id, data: "pwd\r" }));
  view.unmount();
  await waitFor(() => expect(calls.invoke).toHaveBeenCalledWith("stop_shell_terminal", expect.objectContaining({ sessionId: shell.id, viewerId: expect.any(String) })));
  expect(calls.invoke.mock.calls.some(([name]) => name === "terminal_input" || name === "api_request")).toBe(false);
});

it("keeps a tmux service terminal interactive after the service exits", async () => {
  vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
  calls.invoke.mockImplementation(async (command: string, args: { channel?: { onmessage: (frame: unknown) => void } }) => {
    if (command === "start_terminal") {
      args.channel?.onmessage({ type: "connected" });
      args.channel?.onmessage({ type: "output", data: btoa("service output\r\n") });
      args.channel?.onmessage({ type: "output", data: btoa("shell prompt") });
    }
  });
  const props = { service, status, focused: true, expanded: true, busy: false, onFocus: () => {} };
  const view = render(<TerminalPane {...props} />);
  await waitFor(() => expect(calls.writes.join("")).toContain("shell prompt"));
  expect(calls.writes.join("")).toContain("service output");
  view.rerender(<TerminalPane {...props} status={{ ...status, state: "exited", exit_code: 0 }} />);
  expect(screen.getByText("INPUT → tmux shell")).toBeTruthy();
  expect(screen.getByText("tmux shell")).toBeTruthy();
  calls.onData("ls\r");
  await waitFor(() => expect(calls.invoke).toHaveBeenCalledWith("terminal_input", { serviceId: service.id, executionId: status.execution_id, data: "ls\r" }));
  expect(calls.invoke.mock.calls.some(([name]) => name === "start_shell_terminal")).toBe(false);
  view.unmount();
  await waitFor(() => expect(calls.invoke).toHaveBeenCalledWith("stop_terminal", expect.objectContaining({ serviceId: service.id })));
});
