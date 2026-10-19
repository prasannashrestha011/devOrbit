import { beforeEach, describe, expect, it, vi } from "vitest";

const invoke = vi.hoisted(() => vi.fn());
vi.mock("@tauri-apps/api/core", () => ({ invoke }));

import { api, type ApiError, request } from "./api";

beforeEach(() => invoke.mockReset());

describe("desktop API client", () => {
  it("preserves backend error codes and authentication status", async () => {
    invoke.mockResolvedValue({
      status: 401,
      body: {
        error: { code: "unauthorized", message: "valid bearer token required" },
      },
    });
    await expect(request("GET", "/api/v1/workspaces")).rejects.toMatchObject({
      status: 401,
      code: "unauthorized",
      message: "valid bearer token required",
    });
  });

  it("reports bridge or connection failures as unavailable", async () => {
    invoke.mockImplementationOnce(async () => {
      throw new Error("backend unavailable");
    });
    try {
      await request("GET", "/api/v1/workspaces");
      throw new Error("Expected request to fail");
    } catch (error) {
      expect((error as ApiError).status).toBe(0);
      expect((error as ApiError).code).toBe("unavailable");
    }
  });

  it("retrieves all workspace pages using backend pagination", async () => {
    const first = Array.from({ length: 100 }, (_, index) => ({
      id: String(index),
    }));
    invoke
      .mockResolvedValueOnce({ status: 200, body: first })
      .mockResolvedValueOnce({ status: 200, body: [{ id: "last" }] });
    expect(await api.workspaces()).toHaveLength(101);
    expect(invoke.mock.calls.map((call) => call[1].path)).toEqual([
      "/api/v1/workspaces?limit=100&offset=0",
      "/api/v1/workspaces?limit=100&offset=100",
    ]);
  });
});
