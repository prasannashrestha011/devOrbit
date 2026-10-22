// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import type { TrafficExchange as Exchange } from "./api";

const { trafficExchange } = vi.hoisted(() => ({ trafficExchange: vi.fn() }));
vi.mock("./api", () => ({ api: { trafficExchange } }));
import TrafficExchange from "./TrafficExchange";

afterEach(() => { cleanup(); trafficExchange.mockReset(); });

it("shows both HTTP messages and JSON bodies without interpreting captured markup", async () => {
  const requestBody = '{"name":"<script>alert(1)</script>","count":2,"ok":true}';
  const responseBody = '{"accepted":false,"value":null}';
  const message = (text: string) => ({ protocol: "HTTP/1.1", headers: { "Content-Type": ["application/json; charset=utf-8"] }, trailers: null, body: { bytes: text.length, captured_bytes: text.length, complete: true } });
  const exchange: Exchange = {
    id: "1", session_id: "session", sequence: 1, started_at: "2026-10-20T00:00:00Z", method: "POST", path: "/items", status_code: 200, duration_ms: 1, request_bytes: requestBody.length, response_bytes: responseBody.length, outcome: "completed", host: "localhost", raw_query: "",
    request: message(requestBody), response: message(responseBody),
    request_preview: { text: requestBody, binary: false, truncated: false }, response_preview: { text: responseBody, binary: false, truncated: false },
  };
  trafficExchange.mockResolvedValue(exchange);
  render(<TrafficExchange workspaceId="workspace" sessionId="session" record={exchange} />);

  const request = await screen.findByRole("region", { name: "Request details" });
  const response = screen.getByRole("region", { name: "Response details" });
  expect(within(request).getByRole("button", { name: "Pretty" }).getAttribute("aria-pressed")).toBe("true");
  const prettyRequest = within(request).getByLabelText("Request pretty message");
  expect(prettyRequest.textContent).toBe(`POST /items HTTP/1.1\nContent-Type: application/json; charset=utf-8\n\n${JSON.stringify(JSON.parse(requestBody), null, 2)}`);
  expect(within(response).getByLabelText("Response pretty message").textContent).toBe(`HTTP/1.1 200\nContent-Type: application/json; charset=utf-8\n\n${JSON.stringify(JSON.parse(responseBody), null, 2)}`);
  expect(prettyRequest.querySelector("script")).toBeNull();
  fireEvent.click(within(request).getByRole("button", { name: "Raw" }));
  fireEvent.click(within(response).getByRole("button", { name: "Raw" }));
  const rawRequest = within(request).getByLabelText("Request raw message");
  const rawResponse = within(response).getByLabelText("Response raw message");
  expect(rawRequest.textContent).toBe(`POST /items HTTP/1.1\nContent-Type: application/json; charset=utf-8\n\n${requestBody}`);
  expect(rawResponse.textContent).toBe(`HTTP/1.1 200\nContent-Type: application/json; charset=utf-8\n\n${responseBody}`);
  expect(rawRequest.querySelector("script")).toBeNull();

  fireEvent.click(within(request).getByRole("button", { name: "Body" }));
  fireEvent.click(within(response).getByRole("button", { name: "Body" }));
  const requestBodyView = within(request).getByLabelText("Request body");
  const responseBodyView = within(response).getByLabelText("Response body");
  expect(requestBodyView.textContent).toBe(JSON.stringify(JSON.parse(requestBody), null, 2));
  expect(responseBodyView.textContent).toBe(JSON.stringify(JSON.parse(responseBody), null, 2));
  fireEvent.click(within(request).getByLabelText("Format JSON"));
  expect(requestBodyView.textContent).toBe(requestBody);
  fireEvent.click(within(request).getByRole("button", { name: "Pretty" }));
  expect(within(request).getByLabelText("Request pretty message").textContent).toContain(JSON.stringify(JSON.parse(requestBody), null, 2));
});

it.each([
  { name: "malformed JSON", text: '{"missing":', contentType: "application/json", truncated: false, binary: false },
  { name: "truncated JSON", text: '{"ok":true}', contentType: "application/json", truncated: true, binary: false },
  { name: "plain text", text: '{"ok":true}', contentType: "text/plain", truncated: false, binary: false },
  { name: "binary preview", text: "00 ff 10", contentType: "application/json", truncated: false, binary: true },
])("preserves $name in Pretty view", async ({ text, contentType, truncated, binary }) => {
  const message = { protocol: "HTTP/1.1", headers: { "Content-Type": [contentType] }, trailers: null, body: { bytes: text.length, captured_bytes: text.length, complete: true } };
  const exchange: Exchange = {
    id: "fallback", session_id: "session", sequence: 1, started_at: "2026-10-20T00:00:00Z", method: "POST", path: "/items", status_code: 200, duration_ms: 1, request_bytes: text.length, response_bytes: text.length, outcome: "completed", host: "localhost", raw_query: "",
    request: message, response: message,
    request_preview: { text, binary, truncated }, response_preview: { text, binary, truncated },
  };
  trafficExchange.mockResolvedValue(exchange);
  render(<TrafficExchange workspaceId="workspace" sessionId="session" record={exchange} />);
  expect((await screen.findByLabelText("Response pretty message")).textContent).toBe(`HTTP/1.1 200\nContent-Type: ${contentType}\n\n${text}`);
  if (truncated) expect(within(screen.getByRole("region", { name: "Response details" })).getByText(/Preview shortened/)).toBeTruthy();
  if (binary) expect(within(screen.getByRole("region", { name: "Response details" })).getByText(/hexadecimal preview/)).toBeTruthy();
});
