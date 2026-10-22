import { invoke } from "@tauri-apps/api/core";

export type Workspace = {
  id: string;
  name: string;
  description: string;
  root_path: string;
  created_at: string;
  updated_at: string;
};
export type Service = {
  id: string;
  workspace_id: string;
  name: string;
  command: string;
  args: string[];
  working_directory: string;
  git_branch: string;
  env: Record<string, string>;
  enabled: boolean;
  auto_restart: boolean;
  created_at: string;
  updated_at: string;
};
export type ProcessState =
  "stopped" | "starting" | "running" | "stopping" | "exited" | "failed";
export type ProcessStatus = {
  service_id: string;
  execution_id: string;
  pid: number | null;
  state: ProcessState;
  started_at: string | null;
  exited_at: string | null;
  exit_code: number | null;
  last_error: string;
  updated_at: string;
};
export type ShellSession = {
  id: string;
  workspace_id: string;
  shell: string;
  state: ProcessState;
  started_at: string | null;
  exit_code: number | null;
};
export type WorkspaceStatus = {
  workspace_id: string;
  workspace_name: string;
  aggregate_state: "idle" | "transitioning" | "running" | "partial";
  total_services: number;
  enabled_services: number;
  running_services: number;
  stopped_services: number;
  failed_services: number;
  starting_services: number;
  stopping_services: number;
  exited_services: number;
  services: (ProcessStatus & { name: string; enabled: boolean; listening_ports: number[] })[];
};
export type OperationResult = {
  workspace_id: string;
  operation: "start" | "stop" | "restart";
  total: number;
  succeeded: number;
  failed: number;
  skipped: number;
  results: {
    service_id: string;
    service_name: string;
    outcome: string;
    pid: number | null;
    error: string | null;
  }[];
};
export type LogEntry = {
  id: string;
  workspace_id: string;
  service_id: string;
  execution_id: string;
  stream: "stdout" | "stderr" | "terminal";
  timestamp: string;
  sequence: number;
  message: string;
};
export type LogPage = {
  entries: LogEntry[];
  next_cursor: string;
  diagnostics: { dropped_entries: number; storage_errors: number };
};
export type ServiceInput = Pick<Service, "name" | "command" | "args" | "working_directory"> & { revision: string };
export type WorkspaceConfiguration = { content: string; revision: string };
export type EnvFile = { name: string; content: string; revision: string };
export type WorkspaceInput = Pick<
  Workspace,
  "name" | "description"
>;
export type DockerSnapshot = {
  availability: { state: "missing" | "compose_unavailable" | "engine_unavailable" | "ready"; message: string };
  candidates: { file: string; files: string[]; directory: string }[];
  project?: {
    file: string; files: string[]; directory: string; name: string; created: number; running: number; stopped: number;
    services: { name: string; image: string; build: boolean; ports: { target: number; published: string; protocol: string; host?: string }[];
      containers: { id: string; name: string; image: string; state: string; health: string; restart_count: number; health_error?: string; error?: string; ports: { target: number; published: string; protocol: string; host?: string }[] }[] }[];
  };
  error?: string;
};
export type DockerAction = "up" | "start" | "stop" | "restart" | "down" | "build";
export type DockerOperation = { id: string; workspace_id: string; action: DockerAction; state: "running" | "succeeded" | "failed"; error?: string; updated_at: string };
export type DockerTerminal = { id: string; workspace_id: string; container_id: string; container_name: string; shell: string; state: ProcessState };
export type TrafficSession = { id: string; target: { workspace_id: string; kind: "local" | "docker"; service_key: string; port_key: string; file?: string }; upstream: string; proxy_url: string; state: string; started_at: string };
export type TrafficRecord = { id: string; session_id: string; sequence: number; started_at: string; method: string; path: string; status_code: number; duration_ms: number; request_bytes?: number; response_bytes: number; outcome: string; error?: string; capture_incomplete?: boolean; capture_error?: string };
export type TrafficMessage = { protocol: string; status?: string; headers: Record<string, string[]> | null; trailers: Record<string, string[]> | null; transfer_encoding?: string[]; body: { bytes: number; captured_bytes: number; complete: boolean; error?: string } };
export type TrafficPreview = { text: string; binary: boolean; truncated: boolean; error?: string; fields?: { name: string; value: string; filename?: string; content_type?: string }[] };
export type TrafficExchange = TrafficRecord & { host: string; raw_query: string; request: TrafficMessage; response: TrafficMessage; request_preview: TrafficPreview; response_preview: TrafficPreview };
export type TrafficPage = { records: TrafficRecord[]; latest_sequence: number; gap: boolean };

type Reply = { status: number; body: unknown };

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

export async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  let reply: Reply;
  try {
    reply = await invoke<Reply>("api_request", {
      method,
      path,
      body: body ?? null,
    });
  } catch (error) {
    throw new ApiError(0, "unavailable", String(error));
  }
  if (reply.status >= 400) {
    const detail = (
      reply.body as { error?: { code?: string; message?: string } } | null
    )?.error;
    throw new ApiError(
      reply.status,
      detail?.code ?? "request_failed",
      detail?.message ?? `Request failed (${reply.status})`,
    );
  }
  return reply.body as T;
}

async function allPages<T>(path: string): Promise<T[]> {
  const items: T[] = [];
  for (let offset = 0; ; offset += 100) {
    const page = await request<T[]>(
      "GET",
      `${path}?limit=100&offset=${offset}`,
    );
    items.push(...page);
    if (page.length < 100) return items;
  }
}

export const api = {
  workspaces: () => allPages<Workspace>("/api/v1/workspaces"),
  createWorkspace: (body: WorkspaceInput) =>
    request<Workspace>("POST", "/api/v1/workspaces", body),
  updateWorkspace: (id: string, body: WorkspaceInput) =>
    request<Workspace>("PATCH", `/api/v1/workspaces/${id}`, body),
  deleteWorkspace: (id: string) =>
    request<null>("DELETE", `/api/v1/workspaces/${id}`),
  workspaceStatus: (id: string) =>
    request<WorkspaceStatus>("GET", `/api/v1/workspaces/${id}/status`),
  dockerSnapshot: async (id: string, file?: string) => {
    const snapshot = await request<DockerSnapshot>("GET", `/api/v1/workspaces/${id}/docker${file ? `?file=${encodeURIComponent(file)}` : ""}`);
    for (const service of snapshot.project?.services ?? []) service.ports ??= [];
    return snapshot;
  },
  dockerAction: (id: string, file: string, action: DockerAction, service = "") => request<DockerOperation>("POST", `/api/v1/workspaces/${id}/docker/operations`, { file, action, service }),
  dockerOperation: (id: string, operationId: string) => request<DockerOperation>("GET", `/api/v1/workspaces/${id}/docker/operations/${operationId}`),
  cancelDockerOperation: (id: string, operationId: string) => request<DockerOperation>("DELETE", `/api/v1/workspaces/${id}/docker/operations/${operationId}`),
  createDockerTerminal: (id: string, file: string, containerId: string) => request<DockerTerminal>("POST", `/api/v1/workspaces/${id}/docker/terminals`, { file, container_id: containerId }),
  deleteDockerTerminal: (id: string, terminalId: string) => request<null>("DELETE", `/api/v1/workspaces/${id}/docker/terminals/${terminalId}`),
  shellTerminals: (id: string) =>
    request<ShellSession[]>("GET", `/api/v1/workspaces/${id}/terminals`),
  createShellTerminal: (id: string) =>
    request<ShellSession>("POST", `/api/v1/workspaces/${id}/terminals`),
  deleteShellTerminal: (workspaceId: string, sessionId: string) =>
    request<null>("DELETE", `/api/v1/workspaces/${workspaceId}/terminals/${sessionId}`),
  workspaceAction: (id: string, action: "start" | "stop") =>
    request<OperationResult>("POST", `/api/v1/workspaces/${id}/${action}`),
  services: (id: string) =>
    allPages<Service>(`/api/v1/workspaces/${id}/services`),
  configuration: (id: string) => request<WorkspaceConfiguration>("GET", `/api/v1/workspaces/${id}/configuration`),
  validateConfiguration: (id: string, content: string) => request<{ services: Record<string, unknown> }>("POST", `/api/v1/workspaces/${id}/configuration/validate`, { content }),
  saveConfiguration: (id: string, content: string, revision: string) => request<WorkspaceConfiguration>("PUT", `/api/v1/workspaces/${id}/configuration`, { content, revision }),
  createService: (id: string, body: ServiceInput) =>
    request<Service>("POST", `/api/v1/workspaces/${id}/services`, body),
  updateService: (id: string, body: ServiceInput) =>
    request<Service>("PATCH", `/api/v1/services/${id}`, body),
  deleteService: (id: string, revision: string) =>
    request<null>("DELETE", `/api/v1/services/${id}?revision=${encodeURIComponent(revision)}`),
  environmentFiles: (id: string) =>
    request<string[]>("GET", `/api/v1/services/${id}/environment`),
  environmentFile: (id: string, name: string) =>
    request<EnvFile>("GET", `/api/v1/services/${id}/environment/${encodeURIComponent(name)}`),
  saveEnvironmentFile: (id: string, name: string, content: string, revision: string) =>
    request<EnvFile>("PUT", `/api/v1/services/${id}/environment/${encodeURIComponent(name)}`, { content, revision }),
  serviceAction: (id: string, action: "start" | "stop" | "restart") =>
    request<ProcessStatus>("POST", `/api/v1/services/${id}/${action}`),
  logs: (id: string, before?: string) =>
    request<LogPage>(
      "GET",
      `/api/v1/services/${id}/logs?limit=100${before ? `&before=${encodeURIComponent(before)}` : ""}`,
    ),
  trafficSessions: (workspaceId: string) => request<TrafficSession[]>("GET", `/api/v1/workspaces/${workspaceId}/traffic`),
  startTraffic: (workspaceId: string, target: { kind: "local" | "docker"; service_key: string; port_key: string; file?: string }) => request<TrafficSession>("POST", `/api/v1/workspaces/${workspaceId}/traffic`, target),
  stopTraffic: (workspaceId: string, sessionId: string) => request<null>("DELETE", `/api/v1/workspaces/${workspaceId}/traffic/${sessionId}`),
  trafficRecords: (workspaceId: string, sessionId: string, after = 0) => request<TrafficPage>("GET", `/api/v1/workspaces/${workspaceId}/traffic/${sessionId}/records?after=${after}`),
  trafficExchange: (workspaceId: string, sessionId: string, recordId: string) => request<TrafficExchange>("GET", `/api/v1/workspaces/${workspaceId}/traffic/${sessionId}/records/${recordId}`),
  downloadTrafficBody: (workspaceId: string, sessionId: string, recordId: string, side: "request" | "response") => invoke<string | null>("download_traffic_body", { workspaceId, sessionId, recordId, side }),
  clearTraffic: (workspaceId: string, sessionId: string) => request<null>("DELETE", `/api/v1/workspaces/${workspaceId}/traffic/${sessionId}/records`),
};
