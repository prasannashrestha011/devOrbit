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
  services: (ProcessStatus & { name: string; enabled: boolean })[];
};
export type OperationResult = {
  workspace_id: string;
  operation: "start" | "stop";
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
export type ServiceInput = Pick<
  Service,
  "name" | "command" | "args" | "working_directory" | "env" | "enabled"
>;
export type WorkspaceInput = Pick<
  Workspace,
  "name" | "description" | "root_path"
>;

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
  createService: (id: string, body: ServiceInput) =>
    request<Service>("POST", `/api/v1/workspaces/${id}/services`, body),
  updateService: (id: string, body: ServiceInput) =>
    request<Service>("PATCH", `/api/v1/services/${id}`, body),
  deleteService: (id: string) =>
    request<null>("DELETE", `/api/v1/services/${id}`),
  serviceAction: (id: string, action: "start" | "stop" | "restart") =>
    request<ProcessStatus>("POST", `/api/v1/services/${id}/${action}`),
  logs: (id: string, before?: string) =>
    request<LogPage>(
      "GET",
      `/api/v1/services/${id}/logs?limit=100${before ? `&before=${encodeURIComponent(before)}` : ""}`,
    ),
};
