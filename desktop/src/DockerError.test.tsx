// @vitest-environment jsdom
import { afterEach, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import DockerError from "./DockerError";

afterEach(cleanup);

it("summarizes and deduplicates missing Compose variables while preserving diagnostics", () => {
  const diagnostic = "docker compose --project-directory /tmp/project -f /tmp/project/compose.yaml config --format json: " +
    ["MINIO_ROOT_PASSWORD", "MINIO_ROOT_USER", "MINIO_ROOT_PASSWORD", "MINIO_ROOT_USER"].map((name) =>
      `error while interpolating services.minio.environment.${name}: required variable ${name} is missing a value: set ${name} in compose.env`
    ).join("\n") + ": exit status 1";
  render(<DockerError error={diagnostic} />);
  expect(screen.getByRole("alert").textContent).toContain("Compose configuration is incomplete");
  expect(screen.getAllByRole("listitem").map((item) => item.textContent)).toEqual(["MINIO_ROOT_PASSWORD", "MINIO_ROOT_USER"]);
  expect(screen.getByText(/Provide non-empty values/)).toBeTruthy();
  const details = screen.getByText("Technical details").closest("details")!;
  expect(details.open).toBe(false);
  expect(details.querySelector("pre")?.textContent).toBe(diagnostic);
  fireEvent.click(screen.getByText("Technical details"));
  expect(details.open).toBe(true);
});

it("keeps unrecognized errors and line breaks visible without guessing a cause", () => {
  const diagnostic = "Cannot connect to the Docker daemon.\nCheck the socket permissions.";
  render(<DockerError error={diagnostic} title="Could not load Compose services" />);
  expect(screen.getByText("Could not load Compose services")).toBeTruthy();
  expect(screen.getByRole("alert").querySelector("pre")?.textContent).toBe(diagnostic);
  expect(screen.queryByRole("list")).toBeNull();
  expect(screen.queryByText("Technical details")).toBeNull();
});
