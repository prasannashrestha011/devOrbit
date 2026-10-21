// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const mocks = vi.hoisted(() => ({
  trafficSessions: vi.fn(), startTraffic: vi.fn(), stopTraffic: vi.fn(),
  trafficRecords: vi.fn(), clearTraffic: vi.fn(), listen: vi.fn(),
}));
vi.mock("./api", () => ({ api: mocks, ApiError: class ApiError extends Error { status = 0; } }));
vi.mock("@tauri-apps/api/event", () => ({ listen: mocks.listen }));

import TrafficInspector from "./TrafficInspector";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it("starts inspection, shows captured history, and stops the temporary gateway", async () => {
  const session = { id: "session", target: { workspace_id: "workspace", kind: "local", service_key: "service", port_key: "http" }, upstream: "http://127.0.0.1:3000", proxy_url: "http://127.0.0.1:4000", state: "active", started_at: "2026-10-20T00:00:00Z" };
  const record = { id: "record", session_id: "session", sequence: 1, started_at: "2026-10-20T00:00:00Z", method: "GET", path: "/health", status_code: 200, duration_ms: 3, response_bytes: 2, outcome: "completed" };
  mocks.trafficSessions.mockResolvedValue([]);
  mocks.startTraffic.mockResolvedValue(session);
  mocks.stopTraffic.mockResolvedValue(null);
  mocks.clearTraffic.mockResolvedValue(null);
  mocks.trafficRecords.mockImplementation(async (_wid: string, _sid: string, after: number) => ({ records: after ? [] : [record], latest_sequence: 1, gap: false }));
  mocks.listen.mockResolvedValue(() => {});
  vi.spyOn(window, "confirm").mockReturnValue(true);

  render(<TrafficInspector workspaceId="workspace" kind="local" serviceKey="service" ports={[{ key: "http", label: "http · 3000" }]} />);
  fireEvent.click(await screen.findByRole("button", { name: "Inspect HTTP traffic" }));
  await waitFor(() => expect(mocks.startTraffic).toHaveBeenCalledWith("workspace", { kind: "local", service_key: "service", port_key: "http", file: undefined }));
  expect(await screen.findByText("http://127.0.0.1:4000")).toBeTruthy();
  expect(await screen.findByText("/health")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Disable inspection" }));
  await waitFor(() => expect(mocks.stopTraffic).toHaveBeenCalledWith("workspace", "session"));
  expect(await screen.findByRole("button", { name: "Inspect HTTP traffic" })).toBeTruthy();
});
