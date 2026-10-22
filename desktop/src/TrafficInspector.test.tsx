// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

const mocks = vi.hoisted(() => ({
  trafficSessions: vi.fn(), startTraffic: vi.fn(), stopTraffic: vi.fn(),
  trafficRecords: vi.fn(), trafficExchange: vi.fn(), clearTraffic: vi.fn(), listen: vi.fn(),
}));
vi.mock("./api", () => ({ api: mocks, ApiError: class ApiError extends Error { status = 0; } }));
vi.mock("@tauri-apps/api/event", () => ({ listen: mocks.listen }));

import TrafficInspector from "./TrafficInspector";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it("starts inspection, shows captured history, and stops the temporary gateway", async () => {
  const session = { id: "session", target: { workspace_id: "workspace", kind: "local", service_key: "service", port_key: "3000" }, upstream: "http://127.0.0.1:3000", proxy_url: "http://127.0.0.1:4000", state: "active", started_at: "2026-10-20T00:00:00Z" };
  const record = { id: "record", session_id: "session", sequence: 1, started_at: "2026-10-20T00:00:00Z", method: "GET", path: "/health", status_code: 200, duration_ms: 3, response_bytes: 2, outcome: "completed" };
  mocks.trafficSessions.mockResolvedValue([]);
  mocks.startTraffic.mockResolvedValue(session);
  mocks.stopTraffic.mockResolvedValue(null);
  mocks.clearTraffic.mockResolvedValue(null);
  mocks.trafficRecords.mockImplementation(async (_wid: string, _sid: string, after: number) => ({ records: after ? [] : [record], latest_sequence: 1, gap: false }));
  mocks.listen.mockResolvedValue(() => {});
  vi.spyOn(window, "confirm").mockReturnValue(true);

  render(<TrafficInspector workspaceId="workspace" kind="local" serviceKey="service" ports={[{ key: "3000", label: "3000" }]} />);
  fireEvent.click(await screen.findByRole("button", { name: "Inspect HTTP traffic" }));
  await waitFor(() => expect(mocks.startTraffic).toHaveBeenCalledWith("workspace", { kind: "local", service_key: "service", port_key: "3000", file: undefined }));
  expect(await screen.findByText("http://127.0.0.1:4000")).toBeTruthy();
  expect(await screen.findByText("/health")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Disable inspection" }));
  await waitFor(() => expect(mocks.stopTraffic).toHaveBeenCalledWith("workspace", "session"));
  expect(await screen.findByRole("button", { name: "Inspect HTTP traffic" })).toBeTruthy();
});

it("filters the labeled history and inspects request and response metadata", async () => {
  const session = { id: "session", target: { workspace_id: "workspace", kind: "local", service_key: "service", port_key: "3000" }, upstream: "http://127.0.0.1:3000", proxy_url: "http://127.0.0.1:4000", state: "active", started_at: "2026-10-20T00:00:00Z" };
  const base = { session_id: "session", started_at: "2026-10-20T00:00:00Z", duration_ms: 3, outcome: "completed" };
  const records = [
    { ...base, id: "get", sequence: 1, method: "GET", path: "/api/menu", status_code: 200, request_bytes: 0, response_bytes: 839 },
    { ...base, id: "head", sequence: 2, method: "HEAD", path: "/health", status_code: 200, request_bytes: 0, response_bytes: 0 },
    { ...base, id: "failed", sequence: 3, method: "POST", path: "/orders", status_code: 502, response_bytes: 0, outcome: "upstream_error", error: "upstream unavailable" },
  ];
  mocks.trafficSessions.mockResolvedValue([session]);
  mocks.trafficRecords.mockImplementation(async (_wid: string, _sid: string, after: number) => ({ records: after ? [] : records, latest_sequence: 3, gap: false }));
  mocks.listen.mockResolvedValue(() => {});
  mocks.trafficExchange.mockImplementation(async (_wid: string, _sid: string, id: string) => {
    const record = records.find((item) => item.id === id);
    const message = () => ({ protocol: "HTTP/1.1", headers: {}, trailers: {}, body: { bytes: 0, captured_bytes: 0, complete: true } });
    return { ...record, host: "localhost", raw_query: "", request: message(), response: message(), request_preview: { text: "", binary: false, truncated: false }, response_preview: { text: "", binary: false, truncated: false } };
  });

  render(<TrafficInspector workspaceId="workspace" kind="local" serviceKey="service" ports={[]} />);
  const table = await screen.findByRole("table", { name: "Captured requests" });
  await screen.findByText("3 of 3 requests");
  expect(within(table).getAllByRole("columnheader").map((cell) => cell.textContent)).toEqual(["#", "Time", "Method", "Path", "Status", "Duration", "Request", "Response"]);
  expect(within(table).getByText("839 B")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Hide" })).toBeNull();

  const select = screen.getByRole("button", { name: "Inspect request #1: GET /api/menu" });
  fireEvent.click(select);
  expect(select.getAttribute("aria-pressed")).toBe("true");
  expect(within(await screen.findByRole("region", { name: "Request details" })).getByText("0 B")).toBeTruthy();
  expect(within(screen.getByRole("region", { name: "Response details" })).getByText("839 B")).toBeTruthy();
  fireEvent.click(select);
  expect(select.getAttribute("aria-pressed")).toBe("false");
  expect(screen.queryByRole("region", { name: "Request details" })).toBeNull();
  fireEvent.click(select);
  fireEvent.click(screen.getByRole("button", { name: "Hide" }));
  expect(screen.queryByRole("region", { name: "Response details" })).toBeNull();
  fireEvent.click(select);

  fireEvent.change(screen.getByLabelText("Method"), { target: { value: "HEAD" } });
  expect(screen.getByText("1 of 3 requests")).toBeTruthy();
  expect(within(table).getByText("/health")).toBeTruthy();
  expect(screen.queryByRole("region", { name: "Request details" })).toBeNull();
  fireEvent.change(screen.getByLabelText("Method"), { target: { value: "" } });
  fireEvent.change(screen.getByLabelText("Status"), { target: { value: "5" } });
  expect(within(table).queryByText("/api/menu")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Inspect request #3: POST /orders" }));
  expect(within(await screen.findByRole("region", { name: "Request details" })).getByText("Unknown")).toBeTruthy();
  const response = screen.getByRole("region", { name: "Response details" });
  expect(within(response).getByText("upstream error")).toBeTruthy();
  expect(within(response).getByText("upstream unavailable")).toBeTruthy();

  fireEvent.change(screen.getByLabelText("Search paths"), { target: { value: "/missing" } });
  expect(screen.getByText("0 of 3 requests")).toBeTruthy();
  expect(screen.getByText(/No requests match these filters/)).toBeTruthy();
  fireEvent.change(screen.getByLabelText("Status"), { target: { value: "" } });
  fireEvent.change(screen.getByLabelText("Search paths"), { target: { value: "MENU" } });
  expect(screen.getByText("1 of 3 requests")).toBeTruthy();
  expect(within(table).getByText("/api/menu")).toBeTruthy();

  fireEvent.click(screen.getByRole("button", { name: "Pause" }));
  expect(screen.getByRole("status").textContent).toBe("Display paused; gateway still forwarding");
  mocks.trafficRecords.mockClear();
  fireEvent.click(screen.getByRole("button", { name: "Resume" }));
  await waitFor(() => expect(mocks.trafficRecords).toHaveBeenCalledWith("workspace", "session", 3));
  mocks.trafficRecords.mockResolvedValue({ records: [], latest_sequence: 3, gap: false });
  mocks.clearTraffic.mockResolvedValue(null);
  fireEvent.click(screen.getByRole("button", { name: "Clear" }));
  await waitFor(() => expect(mocks.clearTraffic).toHaveBeenCalledWith("workspace", "session"));
  expect(await screen.findByText("0 of 0 requests")).toBeTruthy();
  expect(screen.getByText(/No requests captured yet/)).toBeTruthy();
});
