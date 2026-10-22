import { useCallback, useEffect, useRef, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { LogicalSize, type PhysicalSize } from "@tauri-apps/api/dpi";
import { GripHorizontal, Play, RefreshCw, RotateCw, Square, X } from "lucide-react";
import { api, type DockerOperation, type DockerSnapshot, type Service, type WorkspaceStatus } from "./api";

export const selectedWorkspaceKey = "devorbit.selectedWorkspace";

function message(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function ServiceActions({ name, active, disabled, restartDisabled, onAction }: {
  name: string;
  active: boolean;
  disabled: boolean;
  restartDisabled: boolean;
  onAction: (action: "start" | "stop" | "restart") => void;
}) {
  const action = active ? "Stop" : "Start";
  return <li className="quick-service">
    <span className="quick-service-name" title={name}>{name}</span>
    <button className="icon-button" aria-label={`${action} ${name}`} title={action} disabled={disabled} onClick={() => onAction(active ? "stop" : "start")}>
      {active ? <Square size={16} aria-hidden="true" /> : <Play size={16} aria-hidden="true" />}
    </button>
    <button className="icon-button" aria-label={`Restart ${name}`} title="Restart" disabled={disabled || restartDisabled} onClick={() => onAction("restart")}>
      <RotateCw size={16} aria-hidden="true" />
    </button>
  </li>;
}

function WorkspaceControls({ workspaceId, collapsed, toggling, onToggle }: {
  workspaceId: string | null;
  collapsed: boolean;
  toggling: boolean;
  onToggle: () => Promise<void>;
}) {
  const [services, setServices] = useState<Service[]>([]);
  const [status, setStatus] = useState<WorkspaceStatus | null>(null);
  const [docker, setDocker] = useState<DockerSnapshot | null>(null);
  const [operation, setOperation] = useState<DockerOperation | null>(null);
  const [pending, setPending] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const request = useRef(0);
  const mounted = useRef(false);

  const refresh = useCallback(async () => {
    const version = ++request.current;
    if (!workspaceId) { setLoading(false); return; }
    const results = await Promise.allSettled([
      api.services(workspaceId),
      api.workspaceStatus(workspaceId),
      api.dockerSnapshot(workspaceId, localStorage.getItem(`devorbit.docker.file.${workspaceId}`) || undefined),
    ]);
    if (!mounted.current || version !== request.current) return;
    const [items, current, compose] = results;
    setServices(items.status === "fulfilled" ? items.value : []);
    setStatus(current.status === "fulfilled" ? current.value : null);
    setDocker(compose.status === "fulfilled" ? compose.value : null);
    const failed = results.find((result) => result.status === "rejected");
    if (failed?.status === "rejected") setError(message(failed.reason));
    setLoading(false);
  }, [workspaceId]);

  useEffect(() => {
    mounted.current = true;
    void refresh();
    const timer = window.setInterval(() => { void refresh(); }, 4000);
    return () => { mounted.current = false; ++request.current; window.clearInterval(timer); };
  }, [refresh]);

  useEffect(() => {
    if (!workspaceId || operation?.state !== "running") return;
    let active = true;
    const timer = window.setInterval(() => {
      api.dockerOperation(workspaceId, operation.id).then((latest) => {
        if (!active) return;
        setOperation(latest);
        if (latest.state === "failed") setError(latest.error || "Docker operation failed. Retry the action.");
        if (latest.state !== "running") void refresh();
      }).catch((problem) => { if (active) setError(message(problem)); });
    }, 1000);
    return () => { active = false; window.clearInterval(timer); };
  }, [workspaceId, operation, refresh]);

  async function run(action: () => Promise<unknown>) {
    setPending(true);
    setError("");
    try {
      await action();
    } catch (problem) {
      if (mounted.current) setError(message(problem));
    } finally {
      if (mounted.current) { await refresh(); setPending(false); }
    }
  }

  const busy = loading || pending || operation?.state === "running";
  return <main className={`quick-controls ${collapsed ? "quick-controls-collapsed" : ""}`} aria-label="Quick service controls" aria-busy={loading || pending}>
    <div className="quick-controls-bar" data-tauri-drag-region>
      <button className="icon-button quick-controls-toggle" aria-label={collapsed ? "Expand quick controls" : "Collapse quick controls"} title={collapsed ? "Expand" : "Collapse"} aria-expanded={!collapsed} aria-controls="quick-controls-content" disabled={toggling} onClick={() => onToggle().catch((problem) => setError(message(problem)))}><GripHorizontal size={18} aria-hidden="true" /></button>
      {!collapsed && <>
        <button className="icon-button" aria-label="Refresh services" title="Refresh services" disabled={busy || !workspaceId} onClick={() => { setError(""); setLoading(true); void refresh(); }}><RefreshCw size={15} aria-hidden="true" /></button>
        <button className="icon-button" aria-label="Close quick controls" title="Close" onClick={() => getCurrentWindow().close().catch((problem) => setError(message(problem)))}><X size={17} aria-hidden="true" /></button>
      </>}
    </div>
    <div id="quick-controls-content" className="quick-controls-content" hidden={collapsed}>
    {error && <div className="quick-controls-error" role="alert">{error}</div>}
    <ul className="quick-service-list" aria-label="Services">
      {services.map((service) => {
        const state = status?.services.find((item) => item.service_id === service.id)?.state;
        const active = state === "running" || state === "starting";
        return <ServiceActions key={service.id} name={service.name} active={active}
          disabled={busy || !state || state === "stopping" || (!active && !service.enabled)} restartDisabled={!service.enabled || state === "starting"}
          onAction={(action) => { void run(() => api.serviceAction(service.id, action)); }} />;
      })}
      {docker?.project?.services.map((service) => {
        const active = service.containers.some((container) => ["running", "restarting", "starting"].includes(container.state.toLowerCase()));
        const running = service.containers.some((container) => container.state.toLowerCase() === "running");
        return <ServiceActions key={`docker:${service.name}`} name={service.name} active={active}
          disabled={busy || docker.availability.state !== "ready" || !!docker.error} restartDisabled={!running}
          onAction={(action) => { void run(async () => {
            const latest = await api.dockerAction(workspaceId!, docker.project!.file, action === "start" && service.containers.length === 0 ? "up" : action, service.name);
            if (!mounted.current) return;
            setOperation(latest);
            if (latest.state === "failed") throw new Error(latest.error || "Docker operation failed. Retry the action.");
          }); }} />;
      })}
    </ul>
    </div>
  </main>;
}

export default function QuickControls() {
  const [workspaceId, setWorkspaceId] = useState(() => localStorage.getItem(selectedWorkspaceKey));
  const [collapsed, setCollapsed] = useState(false);
  const [toggling, setToggling] = useState(false);
  const expandedSize = useRef<PhysicalSize | null>(null);
  useEffect(() => {
    const sync = () => setWorkspaceId(localStorage.getItem(selectedWorkspaceKey));
    window.addEventListener("storage", sync);
    return () => window.removeEventListener("storage", sync);
  }, []);

  async function toggle() {
    setToggling(true);
    try {
      const window = getCurrentWindow();
      if (collapsed) {
        await window.setSize(expandedSize.current ?? new LogicalSize(340, 360));
        await window.setMinSize(new LogicalSize(280, 120));
      } else {
        expandedSize.current = await window.innerSize();
        await window.setMinSize(new LogicalSize(48, 44));
        await window.setSize(new LogicalSize(48, 44));
      }
      setCollapsed(!collapsed);
    } finally {
      setToggling(false);
    }
  }

  return <WorkspaceControls key={workspaceId ?? "none"} workspaceId={workspaceId} collapsed={collapsed} toggling={toggling} onToggle={toggle} />;
}
